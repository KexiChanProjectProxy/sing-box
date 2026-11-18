package anytls

import (
	"context"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/tcpinfo"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	anytls "github.com/anytls/sing-anytls"
	"github.com/anytls/sing-anytls/padding"
	"github.com/stretchr/testify/require"
)

// recordingTracker stands in for the router, counting what passes through
// TrackConnection the way the traffic trackers do.
type recordingTracker struct {
	access   sync.Mutex
	metadata []adapter.InboundContext
	outbound []string
	upload   atomic.Int64
	download atomic.Int64
}

func (r *recordingTracker) TrackConnection(_ context.Context, conn net.Conn, metadata adapter.InboundContext, outbound adapter.Outbound) net.Conn {
	r.access.Lock()
	r.metadata = append(r.metadata, metadata)
	r.outbound = append(r.outbound, outbound.Tag())
	r.access.Unlock()
	return bufio.NewInt64CounterConn(conn, []*atomic.Int64{&r.upload}, []*atomic.Int64{&r.download})
}

type closingHandler struct{}

func (closingHandler) NewConnectionEx(_ context.Context, conn net.Conn, _ M.Socksaddr, _ M.Socksaddr, _ N.CloseHandlerFunc) {
	conn.Close()
}

// probePair connects an anytls client to a server over loopback TCP, so that
// both ends have real TCP_INFO.
func probePair(t *testing.T, options *option.AnyTLSInboundProbeOptions) (*anytls.Client, *recordingTracker) {
	t.Helper()
	tracker := &recordingTracker{}
	h := &Inbound{
		Adapter:        inbound.NewAdapter(C.TypeAnyTLS, "anytls-in"),
		logger:         log.NewNOPFactory().Logger(),
		trackingRouter: tracker,
	}
	service, err := anytls.NewService(anytls.ServiceConfig{
		Users:         []anytls.User{{Name: "alice", Password: "secret"}},
		PaddingScheme: padding.DefaultPaddingScheme,
		Handler:       closingHandler{},
		Logger:        h.logger,
		Probe:         h.probeServerConfig(options),
	})
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				metadata := adapter.InboundContext{Source: M.SocksaddrFromNet(conn.RemoteAddr())}
				ctx := adapter.WithContext(context.Background(), &metadata)
				_ = service.NewConnection(ctx, conn, metadata.Source, nil)
				conn.Close()
			}()
		}
	}()
	client, err := anytls.NewClient(context.Background(), anytls.ClientConfig{
		Password: "secret",
		Logger:   log.NewNOPFactory().Logger(),
		DialOut: func(ctx context.Context) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "tcp", listener.Addr().String())
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	return client, tracker
}

func localMeter(_ context.Context, conn net.Conn, direction anytls.ProbeDirection) anytls.ProbeRun {
	return newProbeMeasure(conn, direction == anytls.ProbeUpload, nil)
}

func probeContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestProbeDownloadMeasuresAndAccounts(t *testing.T) {
	t.Parallel()
	client, tracker := probePair(t, &option.AnyTLSInboundProbeOptions{MaxBytes: 4 << 20})
	result, err := client.Probe(probeContext(t), anytls.ProbeRequest{Direction: anytls.ProbeDownload, Meter: localMeter})
	require.NoError(t, err)
	t.Logf("download: %d bytes in %v (%s), server %v, local %v", result.Bytes, result.Duration, result.Reason, result.Server, result.Local)
	require.Positive(t, result.Bytes)
	require.Contains(t, []string{"bytes", "converged"}, result.Reason)
	require.Equal(t, strconv.FormatInt(result.Bytes, 10), result.Server["bytes"], "every byte the server sent arrived")

	// The filler is accounted to the user as one connection.
	require.EqualValues(t, result.Bytes, tracker.download.Load())
	require.Zero(t, tracker.upload.Load())
	require.Len(t, tracker.metadata, 1)
	require.Equal(t, "alice", tracker.metadata[0].User)
	require.Equal(t, "anytls-in", tracker.metadata[0].Inbound)
	require.Equal(t, probeDestination, tracker.metadata[0].Destination)
	require.True(t, tracker.metadata[0].Source.IsValid())
	require.Equal(t, probeOutboundTag, tracker.outbound[0])

	if !tcpinfo.Supported {
		return
	}
	for _, key := range []string{probeKeyAckedBytes, probeKeySentBytes, probeKeyNetLimited, probeKeySmoothedRTT, probeKeyMinRTT, probeKeyCongestion} {
		require.Contains(t, result.Server, key)
	}
	sent, err := strconv.ParseInt(result.Server[probeKeySentBytes], 10, 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, sent, result.Bytes, "TCP carried at least the filler")
	require.Equal(t, "1", result.Server[probeKeyDrained])
	acked, err := strconv.ParseInt(result.Server[probeKeyAckedBytes], 10, 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, acked, result.Bytes, "the result waits for the filler to be acknowledged")
	require.Contains(t, result.Local, probeKeyReceived)
	received, err := strconv.ParseInt(result.Local[probeKeyReceived], 10, 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, received, result.Bytes)
}

func TestProbeUploadMeasuresAndAccounts(t *testing.T) {
	t.Parallel()
	client, tracker := probePair(t, &option.AnyTLSInboundProbeOptions{MaxBytes: 2 << 20})
	result, err := client.Probe(probeContext(t), anytls.ProbeRequest{Direction: anytls.ProbeUpload, Meter: localMeter})
	require.NoError(t, err)
	t.Logf("upload: %d bytes in %v (%s), server %v, local %v", result.Bytes, result.Duration, result.Reason, result.Server, result.Local)
	serverBytes, err := strconv.ParseInt(result.Server["bytes"], 10, 64)
	require.NoError(t, err)
	require.Positive(t, serverBytes)
	require.EqualValues(t, serverBytes, tracker.upload.Load())
	require.Zero(t, tracker.download.Load())
	if !tcpinfo.Supported {
		return
	}
	require.Contains(t, result.Server, probeKeyReceived)
	require.Contains(t, result.Local, probeKeyAckedBytes)
}

func TestProbeDisabled(t *testing.T) {
	t.Parallel()
	client, _ := probePair(t, &option.AnyTLSInboundProbeOptions{Disabled: true})
	_, err := client.Probe(probeContext(t), anytls.ProbeRequest{})
	require.ErrorIs(t, err, anytls.ErrProbeUnsupported)
}

func TestProbeServerConfigDefaults(t *testing.T) {
	t.Parallel()
	h := &Inbound{}
	config := h.probeServerConfig(nil)
	require.NotNil(t, config, "probes are on by default")
	require.NotNil(t, config.Meter)
	require.Zero(t, config.MaxBytes, "the library fills in its defaults")
	require.Nil(t, h.probeServerConfig(&option.AnyTLSInboundProbeOptions{Disabled: true}))
}

func TestServerStatsReportsDetail(t *testing.T) {
	t.Parallel()
	if !tcpinfo.Supported {
		t.Skip("TCP_INFO is Linux only")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			defer conn.Close()
			buffer := make([]byte, 64*1024)
			for {
				if _, readErr := conn.Read(buffer); readErr != nil {
					return
				}
			}
		}
	}()
	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write(make([]byte, 1<<20))
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	stats := serverStats(conn)
	for _, key := range []string{statsKeyRTT, probeKeyMinRTT, statsKeyAppLimited, statsKeyBytesAcked, statsKeyBusy, probeKeyCongestion} {
		require.Contains(t, stats, key)
	}
	// The report still parses for clients that only know the original keys.
	_, ok := parseServerStats(1, stats)
	require.True(t, ok)
}

func TestServerReportsKeepTheLatestPerSession(t *testing.T) {
	t.Parallel()
	var reports serverReports
	now := time.Now()
	reports.store(2, map[string]string{"rtt_us": "1"}, now)
	reports.store(1, map[string]string{"rtt_us": "2"}, now)
	reports.store(2, map[string]string{"rtt_us": "3"}, now.Add(time.Second))
	list := reports.list(now.Add(time.Second))
	require.Len(t, list, 2)
	require.EqualValues(t, 1, list[0].Seq)
	require.Equal(t, "3", list[1].Stats["rtt_us"])
	// A session that stopped reporting is dropped.
	require.Len(t, reports.list(now.Add(serverReportTTL+500*time.Millisecond)), 1)
}
