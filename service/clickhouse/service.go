package clickhouse

import (
	"context"
	stdTLS "crypto/tls"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.ClickHouseServiceOptions](registry, C.TypeClickHouse, NewService)
}

var (
	_ adapter.Service                 = (*Service)(nil)
	_ adapter.ConnectionTracker       = (*Service)(nil)
	_ adapter.ConnectionRejectHandler = (*Service)(nil)
)

type Service struct {
	boxService.Adapter
	ctx         context.Context
	cancel      context.CancelFunc
	logger      log.StructuredLogger
	options     option.ClickHouseServiceOptions
	outbound    adapter.OutboundManager
	node        string
	server      string
	protocol    string
	insertQuery string
	maxEntries  int
	maxWait     time.Duration
	queue       chan sessionEvent
	dropped     atomic.Uint64
	conn        batchConn
	wg          sync.WaitGroup
}

func NewService(ctx context.Context, logger log.StructuredLogger, tag string, options option.ClickHouseServiceOptions) (adapter.Service, error) {
	protocol, err := parseProtocol(options.Protocol)
	if err != nil {
		return nil, err
	}
	tlsEnabled := options.TLS != nil && options.TLS.Enabled
	server, err := parseServer(options.Server, options.ServerPort, protocol, tlsEnabled)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.Table) == "" {
		return nil, E.New("missing table")
	}
	insertQuery, err := buildInsertQuery(options.Database, options.Table)
	if err != nil {
		return nil, err
	}
	router := service.FromContext[adapter.Router](ctx)
	if router == nil {
		return nil, E.New("missing router")
	}
	outbound := service.FromContext[adapter.OutboundManager](ctx)
	if outbound == nil {
		return nil, E.New("missing outbound manager")
	}
	node := tag
	if node == "" {
		node = "sing-box"
	}
	maxEntries := defaultMaxEntries
	maxWait := defaultMaxWait
	if options.Batch != nil {
		if options.Batch.MaxEntries > 0 {
			maxEntries = options.Batch.MaxEntries
		}
		if options.Batch.MaxWait > 0 {
			maxWait = time.Duration(options.Batch.MaxWait)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{
		Adapter:     boxService.NewAdapter(C.TypeClickHouse, tag),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
		options:     options,
		outbound:    outbound,
		node:        node,
		server:      server,
		protocol:    protocol,
		insertQuery: insertQuery,
		maxEntries:  maxEntries,
		maxWait:     maxWait,
		queue:       make(chan sessionEvent, defaultQueueSize),
	}
	router.AppendTracker(s)
	return s, nil
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	conn, err := s.openConn()
	if err != nil {
		return err
	}
	s.conn = conn
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.loopPush()
	}()
	s.logger.InfoEvent("clickhouse.started", "clickhouse access log started",
		log.String("server", s.server),
		log.String("protocol", s.protocol),
		log.String("table", s.options.Table),
		log.String("node", s.node),
	)
	return nil
}

func (s *Service) openConn() (batchConn, error) {
	serviceDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:        s.ctx,
		Options:        option.DialerOptions{Detour: s.options.Detour},
		RemoteIsDomain: true,
	})
	if err != nil {
		return nil, E.Cause(err, "create clickhouse dialer")
	}
	tlsOptions := common.PtrValueOrDefault(s.options.TLS)
	var tlsConfig tls.Config
	if tlsOptions.Enabled {
		serverName, _, splitErr := net.SplitHostPort(s.server)
		if splitErr != nil {
			serverName = s.server
		}
		tlsConfig, err = tls.NewClient(s.ctx, s.logger, serverName, tlsOptions)
		if err != nil {
			return nil, E.Cause(err, "create clickhouse tls")
		}
	}
	dialPlain := func(ctx context.Context, addr string) (net.Conn, error) {
		return serviceDialer.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddr(addr))
	}
	chOptions := &ch.Options{
		Addr: []string{s.server},
		Auth: ch.Auth{
			Database: s.options.Database,
			Username: s.options.Username,
			Password: s.options.Password,
		},
		DialContext:     dialPlain,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     pushTimeout,
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Hour,
	}
	if s.protocol == protocolHTTP {
		chOptions.Protocol = ch.HTTP
		if tlsConfig != nil {
			chOptions.TLS = &stdTLS.Config{}
			chOptions.TransportFunc = func(transport *http.Transport) (http.RoundTripper, error) {
				transport.DialTLSContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
					conn, dialErr := dialPlain(ctx, addr)
					if dialErr != nil {
						return nil, dialErr
					}
					tlsConn, handshakeErr := tls.ClientHandshake(ctx, conn, tlsConfig)
					if handshakeErr != nil {
						conn.Close()
						return nil, handshakeErr
					}
					return tlsConn, nil
				}
				return transport, nil
			}
		}
	} else {
		chOptions.Compression = &ch.Compression{Method: ch.CompressionLZ4}
		if tlsConfig != nil {
			chOptions.DialContext = func(ctx context.Context, addr string) (net.Conn, error) {
				conn, dialErr := dialPlain(ctx, addr)
				if dialErr != nil {
					return nil, dialErr
				}
				tlsConn, handshakeErr := tls.ClientHandshake(ctx, conn, tlsConfig)
				if handshakeErr != nil {
					conn.Close()
					return nil, handshakeErr
				}
				return tlsConn, nil
			}
		}
	}
	conn, err := ch.Open(chOptions)
	if err != nil {
		return nil, E.Cause(err, "open clickhouse")
	}
	return nativeConn{Conn: conn}, nil
}

type nativeConn struct {
	ch.Conn
}

func (c nativeConn) PrepareBatch(ctx context.Context, query string) (preparedBatch, error) {
	return c.Conn.PrepareBatch(ctx, query)
}

func (s *Service) Close() error {
	s.cancel()
	s.wg.Wait()
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if dropped := s.dropped.Load(); dropped > 0 {
		s.logger.WarnEvent("clickhouse.dropped", "dropped session logs", log.Uint64("count", dropped))
	}
	return nil
}
