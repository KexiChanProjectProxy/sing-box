package route

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ tun.FlowTracker = (*flowLogger)(nil)
	_ tun.FlowTracker = (multiFlowTracker)(nil)
)

type flowLogger struct {
	ctx         context.Context
	logger      log.StructuredLogger
	network     string
	source      string
	destination string
	outbound    adapter.Outbound
	createdAt   time.Time
	upload      atomic.Int64
	download    atomic.Int64
}

func newFlowLogger(ctx context.Context, logger log.StructuredLogger, metadata adapter.InboundContext, outbound adapter.Outbound) *flowLogger {
	var source, destination string
	if metadata.Network == N.NetworkICMP {
		source = metadata.Source.AddrString()
		destination = metadata.Destination.AddrString()
	} else {
		source = metadata.Source.String()
		destination = metadata.Destination.String()
	}
	return &flowLogger{
		ctx:         ctx,
		logger:      logger,
		network:     metadata.Network,
		source:      source,
		destination: destination,
		outbound:    outbound,
	}
}

func (l *flowLogger) AttachFlow(handle tun.FlowHandle) {
	l.createdAt = time.Now()
}

func (l *flowLogger) CountForward(n int) {
	l.upload.Add(int64(n))
}

func (l *flowLogger) CountReverse(n int) {
	l.download.Add(int64(n))
}

func (l *flowLogger) FlowEstablished() {
}

func (l *flowLogger) CloseFlow(reason tun.FlowCloseReason) {
	l.logger.DebugEventContext(l.ctx, "connection.closed", "connection closed",
		log.String("reason", reason.String()),
		log.Uint64("upload", uint64(l.upload.Load())),
		log.Uint64("download", uint64(l.download.Load())),
	)
}

type multiFlowTracker []tun.FlowTracker

func (t multiFlowTracker) AttachFlow(handle tun.FlowHandle) {
	for _, tracker := range t {
		tracker.AttachFlow(handle)
	}
}

func (t multiFlowTracker) CountForward(n int) {
	for _, tracker := range t {
		tracker.CountForward(n)
	}
}

func (t multiFlowTracker) CountReverse(n int) {
	for _, tracker := range t {
		tracker.CountReverse(n)
	}
}

func (t multiFlowTracker) FlowEstablished() {
	for _, tracker := range t {
		tracker.FlowEstablished()
	}
}

func (t multiFlowTracker) CloseFlow(reason tun.FlowCloseReason) {
	for _, tracker := range t {
		tracker.CloseFlow(reason)
	}
}
