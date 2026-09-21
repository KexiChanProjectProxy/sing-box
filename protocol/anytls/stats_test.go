package anytls

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tcpinfo"
	"github.com/sagernet/sing-box/common/transportstats"

	"github.com/anytls/sing-anytls/util"
	"github.com/stretchr/testify/require"
)

func TestParseServerStatsRoundTrip(t *testing.T) {
	t.Parallel()
	report := util.StringMap{
		statsKeyRTT:          "25000",
		statsKeyRTTVar:       "4000",
		statsKeySegmentsOut:  "1000",
		statsKeyRetransmits:  "30",
		statsKeyDeliveryRate: "12500000",
	}
	sample, ok := parseServerStats(7, report)
	require.True(t, ok)
	require.EqualValues(t, 7, sample.Key)
	require.Equal(t, 25*time.Millisecond, sample.RTT)
	require.Equal(t, 4*time.Millisecond, sample.RTTVar)
	require.EqualValues(t, 1000, sample.Sent)
	require.EqualValues(t, 30, sample.Lost)
	require.InDelta(t, 100, sample.DeliveryRate, 1e-9, "12.5 MB/s is 100 Mbps")
	require.Zero(t, sample.BytesSent, "per session reports must not feed byte based throughput")
}

func TestParseServerStatsRejectsIncompleteReports(t *testing.T) {
	t.Parallel()
	complete := util.StringMap{
		statsKeyRTT: "1", statsKeyRTTVar: "1", statsKeySegmentsOut: "1",
		statsKeyRetransmits: "0", statsKeyDeliveryRate: "0",
	}
	for key := range complete {
		report := util.StringMap{}
		for k, v := range complete {
			if k != key {
				report[k] = v
			}
		}
		_, ok := parseServerStats(1, report)
		require.False(t, ok, "missing %s", key)
	}
	bad := util.StringMap{}
	for k, v := range complete {
		bad[k] = v
	}
	bad[statsKeyRTT] = "-5"
	_, ok := parseServerStats(1, bad)
	require.False(t, ok)
}

// A report produced by serverStats must parse on the other end.
func TestServerStatsParsesOnClient(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	}()
	client, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer client.Close()
	go func() { _, _ = io.Copy(io.Discard, client) }()
	server := <-accepted
	defer server.Close()
	_, err = server.Write(make([]byte, 64*1024))
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)

	report := serverStats(server)
	if !tcpinfo.Supported {
		require.Nil(t, report)
		return
	}
	require.NotNil(t, report)
	sample, ok := parseServerStats(1, report)
	require.True(t, ok)
	require.NotZero(t, sample.Sent)
}

func TestConnTrackerOffDoesNotWrap(t *testing.T) {
	t.Parallel()
	var tracker connTracker
	left, right := net.Pipe()
	defer right.Close()
	require.Same(t, left, tracker.track(left))
	left.Close()
}

func TestConnTrackerUntracksOnClose(t *testing.T) {
	t.Parallel()
	var tracker connTracker
	tracker.enabled.Store(true)
	left, right := net.Pipe()
	defer right.Close()
	wrapped := tracker.track(left)
	tracker.access.Lock()
	require.Len(t, tracker.conns, 1)
	tracker.access.Unlock()
	require.NoError(t, wrapped.Close())
	_ = wrapped.Close()
	tracker.access.Lock()
	require.Empty(t, tracker.conns, "a closed connection must stop being sampled")
	tracker.access.Unlock()
}

func TestConnTrackerSamplesSkipNonTCP(t *testing.T) {
	t.Parallel()
	var tracker connTracker
	tracker.enabled.Store(true)
	left, right := net.Pipe()
	defer right.Close()
	wrapped := tracker.track(left)
	defer wrapped.Close()
	require.Empty(t, tracker.samples(), "a connection without TCP_INFO yields no sample")
}

func TestLocalStatsDisabledThroughDetour(t *testing.T) {
	t.Parallel()
	outbound := &Outbound{stats: transportstats.NewCollector(), localStatsUnavailable: true}
	outbound.EnableTransportStats(adapter.TransportStatsClient)
	require.False(t, outbound.conns.enabled.Load(), "connections through a detour must not be sampled")
	require.Nil(t, outbound.TransportStats(adapter.TransportStatsClient))
}
