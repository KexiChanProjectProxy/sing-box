package clickhouse

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"

	"github.com/gofrs/uuid/v5"
)

func (s *Service) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	if skipSession(metadata, matchOutbound) {
		return conn
	}
	upload := new(atomic.Int64)
	download := new(atomic.Int64)
	tracker := &connTracker{
		ExtendedConn: bufio.NewCounterConn(conn, []N.CountFunc{func(n int64) {
			upload.Add(n)
		}}, []N.CountFunc{func(n int64) {
			download.Add(n)
		}}),
		service:  s,
		snapshot: s.newSnapshot(metadata, matchedRule, matchOutbound, actionAllow, ""),
		upload:   upload,
		download: download,
		start:    time.Now(),
	}
	return tracker
}

func (s *Service) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	if skipSession(metadata, matchOutbound) {
		return conn
	}
	upload := new(atomic.Int64)
	download := new(atomic.Int64)
	return &packetConnTracker{
		PacketConn: bufio.NewCounterPacketConn(conn, []N.CountFunc{func(n int64) {
			upload.Add(n)
		}}, []N.CountFunc{func(n int64) {
			download.Add(n)
		}}),
		service:  s,
		snapshot: s.newSnapshot(metadata, matchedRule, matchOutbound, actionAllow, ""),
		upload:   upload,
		download: download,
		start:    time.Now(),
	}
}

func (s *Service) RoutedFlow(ctx context.Context, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) tun.FlowTracker {
	if skipSession(metadata, matchOutbound) {
		return nil
	}
	return &flowTracker{
		service:  s,
		snapshot: s.newSnapshot(metadata, matchedRule, matchOutbound, actionAllow, ""),
	}
}

func (s *Service) RejectedConnection(ctx context.Context, metadata adapter.InboundContext, matchedRule adapter.Rule) {
	if skipSession(metadata, nil) {
		return
	}
	now := time.Now()
	s.emit(s.newSnapshot(metadata, matchedRule, nil, actionReject, rejectCloseReason(matchedRule)), now, now, 0, 0)
}

func (s *Service) newSnapshot(metadata adapter.InboundContext, matchedRule adapter.Rule, outbound adapter.Outbound, action string, closeReason string) sessionSnapshot {
	tag, outboundType, chain := resolveChain(s.outbound, outbound)
	return sessionSnapshot{
		metadata:     metadata,
		rule:         matchedRule,
		outboundTag:  tag,
		outboundType: outboundType,
		chain:        chain,
		action:       action,
		close:        closeReason,
	}
}

func (s *Service) emit(snapshot sessionSnapshot, start time.Time, end time.Time, upload int64, download int64) {
	id, err := uuid.NewV4()
	if err != nil {
		return
	}
	snapshot.start = start
	snapshot.end = end
	snapshot.upload = upload
	snapshot.download = download
	s.enqueue(buildSessionEvent(id.String(), s.node, snapshot))
}

type connTracker struct {
	N.ExtendedConn
	service  *Service
	snapshot sessionSnapshot
	upload   *atomic.Int64
	download *atomic.Int64
	start    time.Time
	close    sync.Once
}

func (t *connTracker) Close() error {
	t.close.Do(func() {
		start := t.start
		if start.IsZero() {
			start = time.Now()
		}
		t.service.emit(t.snapshot, start, time.Now(), t.upload.Load(), t.download.Load())
	})
	return t.ExtendedConn.Close()
}

func (t *connTracker) Upstream() any {
	return t.ExtendedConn
}

func (t *connTracker) ReaderReplaceable() bool {
	return true
}

func (t *connTracker) WriterReplaceable() bool {
	return true
}

type packetConnTracker struct {
	N.PacketConn
	service  *Service
	snapshot sessionSnapshot
	upload   *atomic.Int64
	download *atomic.Int64
	start    time.Time
	close    sync.Once
}

func (t *packetConnTracker) Close() error {
	t.close.Do(func() {
		start := t.start
		if start.IsZero() {
			start = time.Now()
		}
		t.service.emit(t.snapshot, start, time.Now(), t.upload.Load(), t.download.Load())
	})
	return t.PacketConn.Close()
}

func (t *packetConnTracker) Upstream() any {
	return t.PacketConn
}

func (t *packetConnTracker) ReaderReplaceable() bool {
	return true
}

func (t *packetConnTracker) WriterReplaceable() bool {
	return true
}

type flowTracker struct {
	service  *Service
	snapshot sessionSnapshot
	upload   atomic.Int64
	download atomic.Int64
	start    time.Time
	close    sync.Once
}

func (t *flowTracker) AttachFlow(handle tun.FlowHandle) {
	t.start = time.Now()
}

func (t *flowTracker) CountForward(n int) {
	t.upload.Add(int64(n))
}

func (t *flowTracker) CountReverse(n int) {
	t.download.Add(int64(n))
}

func (t *flowTracker) FlowEstablished() {}

func (t *flowTracker) CloseFlow(reason tun.FlowCloseReason) {
	t.close.Do(func() {
		start := t.start
		end := time.Now()
		if start.IsZero() {
			start = end
		}
		snapshot := t.snapshot
		snapshot.close = flowCloseReason(reason)
		t.service.emit(snapshot, start, end, t.upload.Load(), t.download.Load())
	})
}
