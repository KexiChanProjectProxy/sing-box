package anytls

import (
	"context"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tcpinfo"
	"github.com/sagernet/sing-box/common/transportstats"

	"github.com/anytls/sing-anytls/util"
)

// Keys of the statistics a server reports. Both ends of this package agree on
// them; the anytls library only carries the map.
const (
	statsKeyRTT          = "rtt_us"
	statsKeyRTTVar       = "rttvar_us"
	statsKeySegmentsOut  = "segs_out"
	statsKeyRetransmits  = "retrans"
	statsKeyDeliveryRate = "delivery_rate_bps"
)

// serverStats reports the server's TCP_INFO for a connection. It returns nil
// where TCP_INFO cannot be read, which makes the server send nothing.
func serverStats(conn net.Conn) util.StringMap {
	info, loaded := tcpinfo.Read(conn)
	if !loaded {
		return nil
	}
	return util.StringMap{
		statsKeyRTT:          strconv.FormatInt(info.RTT.Microseconds(), 10),
		statsKeyRTTVar:       strconv.FormatInt(info.RTTVar.Microseconds(), 10),
		statsKeySegmentsOut:  strconv.FormatUint(info.DataSegmentsOut, 10),
		statsKeyRetransmits:  strconv.FormatUint(info.Retransmits, 10),
		statsKeyDeliveryRate: strconv.FormatUint(info.DeliveryRate, 10),
	}
}

// parseServerStats turns a server report into a sample keyed by session. It
// returns false for a report missing any key it needs.
func parseServerStats(seq uint64, stats util.StringMap) (transportstats.Sample, bool) {
	rtt, err := strconv.ParseInt(stats[statsKeyRTT], 10, 64)
	if err != nil || rtt < 0 {
		return transportstats.Sample{}, false
	}
	rttVar, err := strconv.ParseInt(stats[statsKeyRTTVar], 10, 64)
	if err != nil || rttVar < 0 {
		return transportstats.Sample{}, false
	}
	segmentsOut, err := strconv.ParseUint(stats[statsKeySegmentsOut], 10, 64)
	if err != nil {
		return transportstats.Sample{}, false
	}
	retransmits, err := strconv.ParseUint(stats[statsKeyRetransmits], 10, 64)
	if err != nil {
		return transportstats.Sample{}, false
	}
	deliveryRate, err := strconv.ParseUint(stats[statsKeyDeliveryRate], 10, 64)
	if err != nil {
		return transportstats.Sample{}, false
	}
	return tcpSample(seq, tcpinfo.Info{
		RTT:             time.Duration(rtt) * time.Microsecond,
		RTTVar:          time.Duration(rttVar) * time.Microsecond,
		DataSegmentsOut: segmentsOut,
		Retransmits:     retransmits,
		DeliveryRate:    deliveryRate,
	}), true
}

// tcpSample converts TCP_INFO into a sample. The delivery rate comes from the
// kernel directly, so byte counts are left out: every report of a session
// arrives on its own, and a byte based throughput would need a common clock.
func tcpSample(key uint64, info tcpinfo.Info) transportstats.Sample {
	return transportstats.Sample{
		Key:          key,
		Sent:         info.DataSegmentsOut,
		Lost:         info.Retransmits,
		RTT:          info.RTT,
		RTTVar:       info.RTTVar,
		DeliveryRate: float64(info.DeliveryRate) * 8 / 1e6,
	}
}

// connTracker remembers the connections an outbound has open to its server, so
// that their TCP_INFO can be sampled.
type connTracker struct {
	enabled atomic.Bool
	nextID  atomic.Uint64
	access  sync.Mutex
	conns   map[uint64]net.Conn
}

// track registers conn and returns a wrapper that unregisters it on close. It
// returns conn unchanged while tracking is off.
func (t *connTracker) track(conn net.Conn) net.Conn {
	if !t.enabled.Load() {
		return conn
	}
	id := t.nextID.Add(1)
	t.access.Lock()
	if t.conns == nil {
		t.conns = make(map[uint64]net.Conn)
	}
	t.conns[id] = conn
	t.access.Unlock()
	return &trackedConn{Conn: conn, tracker: t, id: id}
}

func (t *connTracker) untrack(id uint64) {
	t.access.Lock()
	delete(t.conns, id)
	t.access.Unlock()
}

// samples reads TCP_INFO from every tracked connection.
func (t *connTracker) samples() []transportstats.Sample {
	t.access.Lock()
	conns := make(map[uint64]net.Conn, len(t.conns))
	for id, conn := range t.conns {
		conns[id] = conn
	}
	t.access.Unlock()
	samples := make([]transportstats.Sample, 0, len(conns))
	for id, conn := range conns {
		if info, loaded := tcpinfo.Read(conn); loaded {
			samples = append(samples, tcpSample(id, info))
		}
	}
	return samples
}

type trackedConn struct {
	net.Conn
	tracker   *connTracker
	id        uint64
	closeOnce sync.Once
}

func (c *trackedConn) Close() error {
	c.closeOnce.Do(func() {
		c.tracker.untrack(c.id)
	})
	return c.Conn.Close()
}

func (c *trackedConn) Upstream() any {
	return c.Conn
}

func (c *trackedConn) ReaderReplaceable() bool {
	return false
}

func (c *trackedConn) WriterReplaceable() bool {
	return false
}

var _ adapter.OutboundWithTransportStats = (*Outbound)(nil)

// EnableTransportStats starts sampling the connections to the server.
//
// The client direction is this side's TCP_INFO for its own connections, as
// the sender of upstream traffic. It needs Linux, and is unavailable through a
// detour, where the socket under the connection belongs to another transport.
//
// The server direction is the server's TCP_INFO reported back over the
// protocol, as the sender of downstream traffic. It needs a server that
// supports reporting and runs on Linux, and applies to sessions opened after
// it is enabled.
func (h *Outbound) EnableTransportStats(direction adapter.TransportStatsDirection) {
	switch direction {
	case adapter.TransportStatsClient:
		if !tcpinfo.Supported || h.localStatsUnavailable {
			return
		}
		h.conns.enabled.Store(true)
		h.stats.Enable(transportstats.DirectionClient)
		h.statsStart.Do(func() {
			ctx, cancel := context.WithCancel(h.ctx)
			h.statsAccess.Lock()
			h.statsCancel = cancel
			h.statsAccess.Unlock()
			go transportstats.Run(ctx, func(context.Context) {
				h.stats.Record(transportstats.DirectionClient, h.conns.samples(), true)
			})
		})
	case adapter.TransportStatsServer:
		h.stats.Enable(transportstats.DirectionServer)
		h.serverStatsRequested.Store(true)
		h.applyServerStatsHandler()
	}
}

// applyServerStatsHandler installs the report handler once the library client
// exists. It is called both when the direction is enabled and when the client
// is created, whichever comes last.
func (h *Outbound) applyServerStatsHandler() {
	if !h.serverStatsRequested.Load() {
		return
	}
	h.statsAccess.Lock()
	client := h.client
	h.statsAccess.Unlock()
	if client == nil {
		return
	}
	client.SetStatsHandler(func(seq uint64, stats util.StringMap) {
		if sample, loaded := parseServerStats(seq, stats); loaded {
			h.stats.Record(transportstats.DirectionServer, []transportstats.Sample{sample}, false)
		}
	})
}

func (h *Outbound) TransportStats(direction adapter.TransportStatsDirection) adapter.TransportStatsReader {
	recorder := h.stats.Recorder(transportstats.Direction(direction))
	if recorder == nil {
		return nil
	}
	return recorder
}
