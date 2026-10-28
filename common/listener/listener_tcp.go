package listener

import (
	"net"
	"net/netip"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/redir"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"github.com/database64128/tfo-go/v2"
)

func (l *Listener) ListenTCP() (net.Listener, error) {
	//nolint:staticcheck
	if l.listenOptions.ProxyProtocol || l.listenOptions.ProxyProtocolAcceptNoHeader {
		return nil, E.New("Proxy Protocol is deprecated and removed in sing-box 1.6.0")
	}
	if l.listenOptions.ListenUnix != "" {
		return l.listenUnix()
	}
	var err error
	bindAddr := M.SocksaddrFrom(l.listenOptions.Listen.Build(netip.AddrFrom4([4]byte{127, 0, 0, 1})), l.listenOptions.ListenPort)
	var listenConfig net.ListenConfig
	if l.listenOptions.BindInterface != "" {
		listenConfig.Control = control.Append(listenConfig.Control, control.BindToInterface(service.FromContext[adapter.NetworkManager](l.ctx).InterfaceFinder(), l.listenOptions.BindInterface, -1))
	}
	if l.listenOptions.RoutingMark != 0 {
		listenConfig.Control = control.Append(listenConfig.Control, control.RoutingMark(uint32(l.listenOptions.RoutingMark)))
	}
	if l.listenOptions.ReuseAddr {
		listenConfig.Control = control.Append(listenConfig.Control, control.ReuseAddr())
	}
	if l.listenOptions.DisableTCPKeepAlive {
		listenConfig.KeepAlive = -1
		listenConfig.KeepAliveConfig.Enable = false
	} else {
		keepIdle := time.Duration(l.listenOptions.TCPKeepAlive)
		if keepIdle == 0 {
			keepIdle = C.TCPKeepAliveInitial
		}
		keepInterval := time.Duration(l.listenOptions.TCPKeepAliveInterval)
		if keepInterval == 0 {
			keepInterval = C.TCPKeepAliveInterval
		}
		listenConfig.KeepAliveConfig = net.KeepAliveConfig{
			Enable:   true,
			Idle:     keepIdle,
			Interval: keepInterval,
		}
	}
	if l.listenOptions.TCPMultiPath {
		listenConfig.SetMultipathTCP(true)
	}
	if l.tproxy {
		listenConfig.Control = control.Append(listenConfig.Control, func(network, address string, conn syscall.RawConn) error {
			return control.Raw(conn, func(fd uintptr) error {
				return redir.TProxy(fd, !strings.HasSuffix(network, "4"), false)
			})
		})
	}
	tcpListener, err := ListenNetworkNamespace[net.Listener](l.ctx, l.listenOptions.NetNs, func() (net.Listener, error) {
		if l.listenOptions.TCPFastOpen {
			var tfoConfig tfo.ListenConfig
			tfoConfig.ListenConfig = listenConfig
			return tfoConfig.Listen(l.ctx, M.NetworkFromNetAddr(N.NetworkTCP, bindAddr.Addr), bindAddr.String())
		} else {
			return listenConfig.Listen(l.ctx, M.NetworkFromNetAddr(N.NetworkTCP, bindAddr.Addr), bindAddr.String())
		}
	})
	if err != nil {
		return nil, err
	}
	l.logger.InfoEvent("listener.started", "server started", log.String("listen", tcpListener.Addr().String()), log.String("network", "tcp"))

	l.tcpListener = tcpListener
	return tcpListener, err
}

func (l *Listener) loopTCPIn() {
	tcpListener := l.tcpListener
	var (
		metadata    adapter.InboundContext
		acceptDelay time.Duration
	)
	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			if l.shutdown.Load() && E.IsClosed(err) {
				return
			}
			//nolint:staticcheck
			if netError, isNetError := err.(net.Error); isNetError && netError.Temporary() {
				if acceptDelay == 0 {
					acceptDelay = 5 * time.Millisecond
				} else {
					acceptDelay = min(2*acceptDelay, time.Second)
				}
				l.logger.ErrorEvent("listener.error", "listener error", log.Err(err), log.Duration("retry_in", acceptDelay))
				time.Sleep(acceptDelay)
				continue
			}
			l.tcpListener.Close()
			l.logger.ErrorEvent("listener.closed", "listener closed", log.Err(err))
			return
		}
		acceptDelay = 0
		//nolint:staticcheck
		metadata.InboundDetour = l.listenOptions.Detour
		ctx := log.ContextWithNewID(l.ctx)
		if l.listenOptions.ListenUnix != "" {
			// A Unix socket peer has no network address.
			metadata.Source = M.Socksaddr{}
			metadata.OriginDestination = M.Socksaddr{}
			l.logger.InfoEventContext(ctx, "inbound.accepted", "inbound accepted", log.String("listen", l.listenOptions.ListenUnix), log.String("network", C.NetworkUnix))
		} else {
			metadata.Source = M.SocksaddrFromNet(conn.RemoteAddr()).Unwrap()
			metadata.OriginDestination = M.SocksaddrFromNet(conn.LocalAddr()).Unwrap()
			l.logger.InfoEventContext(ctx, "inbound.accepted", "inbound accepted", log.Addr("source", metadata.Source))
		}

		go l.connHandler.NewConnection(ctx, conn, metadata, nil)
	}
}

func (l *Listener) listenUnix() (net.Listener, error) {
	path := l.listenOptions.ListenUnix
	if l.listenOptions.Listen != nil || l.listenOptions.ListenPort != 0 {
		return nil, E.New("listen_unix is mutually exclusive with listen and listen_port")
	}
	if l.tproxy {
		return nil, E.New("listen_unix is not supported by tproxy")
	}
	if l.listenOptions.TCPFastOpen {
		return nil, E.New("tcp_fast_open is not supported with listen_unix")
	}
	if l.listenOptions.TCPMultiPath {
		return nil, E.New("tcp_multi_path is not supported with listen_unix")
	}
	if l.listenOptions.BindInterface != "" {
		return nil, E.New("bind_interface is not supported with listen_unix")
	}
	if l.listenOptions.RoutingMark != 0 {
		return nil, E.New("routing_mark is not supported with listen_unix")
	}
	if l.listenOptions.ReuseAddr {
		return nil, E.New("reuse_addr is not supported with listen_unix")
	}
	unixListener, err := ListenNetworkNamespace[net.Listener](l.ctx, l.listenOptions.NetNs, func() (net.Listener, error) {
		info, err := os.Lstat(path)
		if err == nil {
			if info.Mode()&os.ModeSocket == 0 {
				return nil, E.New("listen_unix: ", path, " exists and is not a socket")
			}
			err = os.Remove(path)
			if err != nil {
				return nil, E.Cause(err, "remove stale socket ", path)
			}
		} else if !os.IsNotExist(err) {
			return nil, E.Cause(err, "stat ", path)
		}
		var listenConfig net.ListenConfig
		return listenConfig.Listen(l.ctx, C.NetworkUnix, path)
	})
	if err != nil {
		return nil, err
	}
	l.logger.InfoEvent("listener.started", "server started", log.String("listen", path), log.String("network", C.NetworkUnix))
	l.tcpListener = unixListener
	return unixListener, nil
}
