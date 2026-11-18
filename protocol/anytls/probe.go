package anytls

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/tcpinfo"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	anytls "github.com/anytls/sing-anytls"
	"github.com/anytls/sing-anytls/util"
)

// Keys of a probe's measurement, added to the result the server sends and
// to the local result of a client probe. Rates are in bytes per second, like
// statsKeyDeliveryRate.
const (
	probeKeyCongestion    = "cc"
	probeKeyBBRBandwidth  = "bbr_bw_bps"
	probeKeyBBRMinRTT     = "bbr_min_rtt_us"
	probeKeyPeakBandwidth = "peak_bw_bps"
	probeKeySmoothedRTT   = "srtt_us"
	probeKeyMinRTT        = "min_rtt_us"
	probeKeyNetLimited    = "net_limited_us"
	probeKeyAckedBytes    = "acked_bytes"
	probeKeyGoodput       = "goodput_bps"
	probeKeySentBytes     = "sent_bytes"
	probeKeyRetransBytes  = "retrans_bytes"
	probeKeyDSACKDups     = "dsack_dups"
	probeKeyRTOs          = "rtos"
	probeKeyReceived      = "received_bytes"
	probeKeyReceiverRTT   = "rcv_rtt_us"
	probeKeyDrained       = "drained"
)

// probeDrainTimeout bounds how long the sender waits, once it has written its
// last filler, for the socket to deliver it.
const probeDrainTimeout = 3 * time.Second

// probeRounds is how many rounds without 25% growth in the bandwidth estimate
// end a probe, the test BBR uses to leave startup.
const probeRounds = 3

// probeMinRound bounds how often the sender reads TCP_INFO.
const probeMinRound = 5 * time.Millisecond

// probeMinNetLimited is the network limited time below which no goodput is
// reported, since a few packets say little about the path.
const probeMinNetLimited = time.Millisecond

// probeMeasure follows one end's TCP_INFO through a probe.
type probeMeasure struct {
	conn    net.Conn
	sender  bool
	account func(bytes int64)

	start    tcpinfo.Info
	hasStart bool

	nextRound time.Time
	fullBW    uint64
	flat      int
	peakBW    uint64
}

func newProbeMeasure(conn net.Conn, sender bool, account func(bytes int64)) *probeMeasure {
	measure := &probeMeasure{conn: conn, sender: sender, account: account}
	measure.start, measure.hasStart = tcpinfo.Read(conn)
	return measure
}

// Converged reads TCP_INFO about once a round trip and reports whether the
// bandwidth estimate has stopped growing: BBR's own estimate when the sender
// runs BBR, and otherwise the delivery rate of samples the application did
// not limit.
func (m *probeMeasure) Converged() bool {
	if !m.sender || !m.hasStart {
		return false
	}
	now := time.Now()
	if now.Before(m.nextRound) {
		return false
	}
	info, loaded := tcpinfo.Read(m.conn)
	if !loaded {
		return false
	}
	m.nextRound = now.Add(max(info.RTT, probeMinRound))
	var bandwidth uint64
	switch {
	case info.BBR != nil:
		bandwidth = info.BBR.Bandwidth
	case !info.AppLimited:
		bandwidth = info.DeliveryRate
	}
	if bandwidth == 0 {
		return false
	}
	m.peakBW = max(m.peakBW, bandwidth)
	if bandwidth >= m.fullBW+m.fullBW/4 {
		m.fullBW = bandwidth
		m.flat = 0
		return false
	}
	m.flat++
	return m.flat >= probeRounds
}

func (m *probeMeasure) Finish(bytes int64) util.StringMap {
	if m.account != nil && bytes > 0 {
		m.account(bytes)
	}
	if !m.hasStart {
		return nil
	}
	var (
		end     tcpinfo.Info
		loaded  bool
		drained = true
	)
	if m.sender {
		end, loaded, drained = m.drain()
	} else {
		end, loaded = tcpinfo.Read(m.conn)
	}
	if !loaded {
		return nil
	}
	result := util.StringMap{
		probeKeySmoothedRTT: formatMicroseconds(end.RTT),
		probeKeyMinRTT:      formatMicroseconds(end.MinRTT),
	}
	if end.CongestionControl != "" {
		result[probeKeyCongestion] = end.CongestionControl
	}
	if end.BBR != nil {
		result[probeKeyBBRBandwidth] = strconv.FormatUint(end.BBR.Bandwidth, 10)
		result[probeKeyBBRMinRTT] = formatMicroseconds(end.BBR.MinRTT)
		m.peakBW = max(m.peakBW, end.BBR.Bandwidth)
	}
	if !m.sender {
		result[probeKeyReceived] = strconv.FormatUint(end.BytesReceived-m.start.BytesReceived, 10)
		result[probeKeyReceiverRTT] = formatMicroseconds(end.RcvRTT)
		return result
	}
	result[probeKeyDrained] = formatBool(drained)
	if m.peakBW > 0 {
		result[probeKeyPeakBandwidth] = strconv.FormatUint(m.peakBW, 10)
	}
	acked := end.BytesAcked - m.start.BytesAcked
	result[probeKeyAckedBytes] = strconv.FormatUint(acked, 10)
	result[probeKeySentBytes] = strconv.FormatUint(end.BytesSent-m.start.BytesSent, 10)
	if end.HasChrono && m.start.HasChrono {
		limited := end.NetworkLimited() - m.start.NetworkLimited()
		if limited < 0 {
			limited = 0
		}
		result[probeKeyNetLimited] = formatMicroseconds(limited)
		if limited >= probeMinNetLimited {
			result[probeKeyGoodput] = strconv.FormatUint(uint64(float64(acked)/limited.Seconds()), 10)
		}
	}
	if end.HasRetransBytes && m.start.HasRetransBytes {
		result[probeKeyRetransBytes] = strconv.FormatUint(end.BytesRetrans-m.start.BytesRetrans, 10)
		result[probeKeyDSACKDups] = strconv.FormatUint(end.DSACKDups-m.start.DSACKDups, 10)
	}
	if end.HasRTOStats && m.start.HasRTOStats {
		result[probeKeyRTOs] = strconv.FormatUint(end.TotalRTO-m.start.TotalRTO, 10)
	}
	return result
}

// drain waits for the socket to deliver what was written, since the last
// filler written may still be in the send buffer, and returns TCP_INFO once
// it has, or when probeDrainTimeout runs out.
func (m *probeMeasure) drain() (tcpinfo.Info, bool, bool) {
	deadline := time.Now().Add(probeDrainTimeout)
	for {
		info, loaded := tcpinfo.Read(m.conn)
		if !loaded {
			return info, false, false
		}
		if info.Unacked == 0 && info.NotSentBytes == 0 {
			return info, true, true
		}
		if !time.Now().Before(deadline) {
			return info, true, false
		}
		time.Sleep(min(max(info.RTT/4, time.Millisecond), 50*time.Millisecond))
	}
}

func formatMicroseconds(duration time.Duration) string {
	return strconv.FormatInt(duration.Microseconds(), 10)
}

// probeOutboundTag names the outbound probe traffic is accounted to; no such
// outbound exists, and nothing is dialed through it.
const probeOutboundTag = "anytls-probe"

// probeDestination is the destination probe traffic is accounted to.
var probeDestination = M.Socksaddr{Fqdn: "probe.anytls.arpa"}

type probeOutbound struct {
	outbound.Adapter
}

var accountingOutbound adapter.Outbound = &probeOutbound{
	Adapter: outbound.NewAdapter(probeOutboundTag, probeOutboundTag, []string{N.NetworkTCP}, nil),
}

func (*probeOutbound) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	return nil, E.New("anytls probe traffic is not dialed")
}

func (*probeOutbound) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, E.New("anytls probe traffic is not dialed")
}

// probeServerConfig turns the inbound's probe options into the library's
// limits, or nil when probes are disabled.
func (h *Inbound) probeServerConfig(options *option.AnyTLSInboundProbeOptions) *anytls.ProbeConfig {
	config := &anytls.ProbeConfig{Meter: h.probeMeter}
	if options != nil {
		if options.Disabled {
			return nil
		}
		config.MaxBytes = options.MaxBytes
		config.MaxDuration = options.MaxDuration.Build()
		config.Cooldown = options.Cooldown.Build()
		config.MaxConcurrent = options.MaxConcurrent
	}
	return config
}

// probeMeter measures a probe on the server, which sends a download and
// receives an upload, and accounts its filler to the user.
func (h *Inbound) probeMeter(ctx context.Context, conn net.Conn, direction anytls.ProbeDirection) anytls.ProbeRun {
	return newProbeMeasure(conn, direction == anytls.ProbeDownload, func(bytes int64) {
		h.accountProbe(ctx, direction, bytes)
	})
}

// accountProbe passes a probe's filler through the router's connection
// trackers as one connection, so that it counts towards the user's traffic
// and shows up in connection logs like any other.
func (h *Inbound) accountProbe(ctx context.Context, direction anytls.ProbeDirection, bytes int64) {
	var metadata adapter.InboundContext
	if inboundContext := adapter.ContextFrom(ctx); inboundContext != nil {
		metadata = *inboundContext
	}
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	metadata.Network = N.NetworkTCP
	metadata.Destination = probeDestination
	if user, _ := auth.UserFromContext[string](ctx); user != "" {
		metadata.User = user
	}
	h.logger.InfoContext(ctx, "inbound probe ", direction, " of ", bytes, " bytes from ", metadata.Source, " user ", metadata.User)
	if h.trackingRouter == nil {
		return
	}
	conn := h.trackingRouter.TrackConnection(ctx, &probeConn{}, metadata, accountingOutbound)
	defer conn.Close()
	buffer := make([]byte, 64*1024)
	for remaining := bytes; remaining > 0; {
		chunk := buffer[:min(int64(len(buffer)), remaining)]
		var n int
		var err error
		if direction == anytls.ProbeDownload {
			n, err = conn.Write(chunk)
		} else {
			n, err = conn.Read(chunk)
		}
		if err != nil || n <= 0 {
			return
		}
		remaining -= int64(n)
	}
}

// probeConn is the connection probe filler is accounted on: reads return
// zeros as an upload would deliver them, and writes vanish.
type probeConn struct{}

func (*probeConn) Read(b []byte) (int, error)       { clear(b); return len(b), nil }
func (*probeConn) Write(b []byte) (int, error)      { return len(b), nil }
func (*probeConn) Close() error                     { return nil }
func (*probeConn) LocalAddr() net.Addr              { return M.Socksaddr{}.TCPAddr() }
func (*probeConn) RemoteAddr() net.Addr             { return M.Socksaddr{}.TCPAddr() }
func (*probeConn) SetDeadline(time.Time) error      { return nil }
func (*probeConn) SetReadDeadline(time.Time) error  { return nil }
func (*probeConn) SetWriteDeadline(time.Time) error { return nil }

// ProbeOptions asks for a probe of the path to the server.
type ProbeOptions = anytls.ProbeRequest

// Probe fills one session with filler to measure the path to the server in
// one direction, within the limits the server allows. The local end's
// TCP_INFO is measured too, except through a detour, where the socket belongs
// to another transport; options.Meter, when set, replaces that.
func (h *Outbound) Probe(ctx context.Context, options ProbeOptions) (anytls.ProbeResult, error) {
	h.statsAccess.Lock()
	client := h.client
	h.statsAccess.Unlock()
	if client == nil {
		return anytls.ProbeResult{}, E.New("anytls outbound not started")
	}
	if options.Meter == nil && tcpinfo.Supported && !h.localStatsUnavailable {
		options.Meter = func(_ context.Context, conn net.Conn, direction anytls.ProbeDirection) anytls.ProbeRun {
			return newProbeMeasure(conn, direction == anytls.ProbeUpload, nil)
		}
	}
	return client.Probe(ctx, options)
}
