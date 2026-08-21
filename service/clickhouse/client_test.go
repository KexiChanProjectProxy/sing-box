package clickhouse

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	R "github.com/sagernet/sing-box/route/rule"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/stretchr/testify/require"
)

type fakeBatch struct {
	rows [][]any
	sent bool
	err  error
}

func (b *fakeBatch) Append(v ...any) error {
	if b.err != nil {
		return b.err
	}
	b.rows = append(b.rows, append([]any(nil), v...))
	return nil
}

func (b *fakeBatch) Send() error {
	if b.err != nil {
		return b.err
	}
	b.sent = true
	return nil
}

func (b *fakeBatch) Close() error { return nil }

type fakeConn struct {
	query string
	batch *fakeBatch
	err   error
}

func (c *fakeConn) PrepareBatch(ctx context.Context, query string) (preparedBatch, error) {
	c.query = query
	if c.err != nil {
		return nil, c.err
	}
	return c.batch, nil
}

func (c *fakeConn) Close() error { return nil }

func TestPushUsesPrepareBatchAppendSend(t *testing.T) {
	batch := &fakeBatch{}
	service := &Service{
		logger:      log.NewNOPFactory().Logger(),
		insertQuery: "INSERT INTO `logs`.`sessions` (" + insertColumns + ")",
		conn:        &fakeConn{batch: batch},
	}
	end := time.Date(2026, 8, 22, 0, 21, 28, 917000000, time.UTC)
	err := service.push([]sessionEvent{{
		Node:   "gw-01",
		ID:     "sess-1",
		Start:  end.Add(-time.Second),
		End:    end,
		Action: actionAllow,
		Source: sessionAddr{IP: "10.10.1.23", Port: 1},
	}})
	require.NoError(t, err)
	require.True(t, batch.sent)
	require.Len(t, batch.rows, 1)
	require.Equal(t, "gw-01", batch.rows[0][0])
	require.Equal(t, "sess-1", batch.rows[0][1])
}

func TestPushDropsOnPrepareError(t *testing.T) {
	service := &Service{
		logger:      log.NewNOPFactory().Logger(),
		insertQuery: "INSERT INTO `sessions` (" + insertColumns + ")",
		conn:        &fakeConn{err: E.New("table missing")},
	}
	err := service.push([]sessionEvent{{ID: "sess-1"}})
	require.Error(t, err)
}

func TestRoutedConnectionEmitsOnClose(t *testing.T) {
	service := &Service{
		logger: log.NewNOPFactory().Logger(),
		queue:  make(chan sessionEvent, 1),
		node:   "gw-01",
	}
	service.ctx, service.cancel = context.WithCancel(context.Background())
	left, right := net.Pipe()
	defer right.Close()
	wrapped := service.RoutedConnection(service.ctx, left, adapter.InboundContext{
		Inbound:     "mixed-in",
		InboundType: C.TypeMixed,
		Network:     N.NetworkTCP,
		Source:      M.SocksaddrFrom(netip.MustParseAddr("10.0.0.1"), 1234),
		Destination: M.SocksaddrFrom(netip.MustParseAddr("1.1.1.1"), 443),
		User:        "alice",
	}, nil, nil)
	require.NoError(t, wrapped.Close())
	select {
	case event := <-service.queue:
		require.Equal(t, actionAllow, event.Action)
		require.Equal(t, "gw-01", event.Node)
		require.Equal(t, "alice", event.User)
		require.Equal(t, "mixed-in", event.Inbound)
		require.Equal(t, "final", event.Rule)
		require.Equal(t, "10.0.0.1", event.Source.IP)
		require.Equal(t, "1.1.1.1", event.Destination.IP)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for session event")
	}
}

func TestRejectedConnectionEmitsReject(t *testing.T) {
	service := &Service{
		logger: log.NewNOPFactory().Logger(),
		queue:  make(chan sessionEvent, 1),
		node:   "gw-01",
	}
	service.ctx, service.cancel = context.WithCancel(context.Background())
	service.RejectedConnection(service.ctx, adapter.InboundContext{
		Network:     N.NetworkTCP,
		Source:      M.SocksaddrFrom(netip.MustParseAddr("10.0.0.2"), 9),
		Destination: M.ParseSocksaddrHostPort("blocked.example", 443),
	}, stubRule{
		name:   "ads",
		action: &R.RuleActionReject{Method: C.RuleActionRejectMethodDrop},
	})
	select {
	case event := <-service.queue:
		require.Equal(t, actionReject, event.Action)
		require.Equal(t, closeDrop, event.Close)
		require.Equal(t, "ads", event.Rule)
		require.Equal(t, "blocked.example", event.Destination.Domain)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reject event")
	}
}

func TestSkipDNSDoesNotWrap(t *testing.T) {
	service := &Service{queue: make(chan sessionEvent, 1)}
	service.ctx, service.cancel = context.WithCancel(context.Background())
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	wrapped := service.RoutedConnection(service.ctx, left, adapter.InboundContext{
		Protocol: C.ProtocolDNS,
	}, nil, nil)
	require.Equal(t, left, wrapped)
	service.RejectedConnection(service.ctx, adapter.InboundContext{Protocol: C.ProtocolDNS}, nil)
	select {
	case <-service.queue:
		t.Fatal("dns session should be skipped")
	default:
	}
}

func TestEnqueueDropsWhenQueueFull(t *testing.T) {
	service := &Service{
		logger: log.NewNOPFactory().Logger(),
		queue:  make(chan sessionEvent),
	}
	service.ctx, service.cancel = context.WithCancel(context.Background())
	service.enqueue(sessionEvent{ID: "lost"})
	require.Equal(t, uint64(1), service.dropped.Load())
	select {
	case <-service.queue:
		t.Fatal("full queue must drop")
	default:
	}
}
