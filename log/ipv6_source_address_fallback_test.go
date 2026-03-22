package log

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type testEventSubscriber struct {
	entries chan LogEntry
}

func (s *testEventSubscriber) HandleEvent(entry LogEntry) {
	s.entries <- entry
}

func TestWarnIPv6SourceAddressFallbackNoCandidate(t *testing.T) {
	entry := captureIPv6SourceAddressFallbackEntry(t, func(logger ContextLogger) {
		WarnIPv6SourceAddressFallback(
			logger,
			context.Background(),
			"2001:db8::/48",
			"hash_5tuple",
			IPv6SourceAddressFallbackReasonNoCandidate,
			netip.Addr{},
			nil,
		)
	})

	if entry.Level != LevelWarn {
		t.Fatalf("expected warning level, got %v", entry.Level)
	}
	if entry.Event == nil {
		t.Fatal("expected structured event")
	}
	if entry.Event.Type != EventTypeIPv6SourceAddressFallback {
		t.Fatalf("expected event type %q, got %q", EventTypeIPv6SourceAddressFallback, entry.Event.Type)
	}
	if entry.Event.Data["action"] != IPv6SourceAddressFallbackActionFallback {
		t.Fatalf("expected action %q, got %#v", IPv6SourceAddressFallbackActionFallback, entry.Event.Data["action"])
	}
	if entry.Event.Data["range"] != "2001:db8::/48" {
		t.Fatalf("expected range to be logged, got %#v", entry.Event.Data["range"])
	}
	if entry.Event.Data["mode"] != "hash_5tuple" {
		t.Fatalf("expected mode to be logged, got %#v", entry.Event.Data["mode"])
	}
	if entry.Event.Data["reason"] != IPv6SourceAddressFallbackReasonNoCandidate {
		t.Fatalf("expected reason to be logged, got %#v", entry.Event.Data["reason"])
	}
	if _, exists := entry.Event.Data["selected_address"]; exists {
		t.Fatal("did not expect selected_address for no_candidate")
	}
	if _, exists := entry.Event.Data["error"]; exists {
		t.Fatal("did not expect error field for no_candidate")
	}
	if !strings.Contains(entry.Message, "ipv6_source_address_range fallback") ||
		!strings.Contains(entry.Message, "range=2001:db8::/48") ||
		!strings.Contains(entry.Message, "mode=hash_5tuple") ||
		!strings.Contains(entry.Message, "reason=no_candidate") {
		t.Fatalf("plain log message missing aligned fields: %q", entry.Message)
	}
	if strings.Contains(entry.Message, "selected_address=") || strings.Contains(entry.Message, "error=") {
		t.Fatalf("plain log message should not imply bind failure: %q", entry.Message)
	}
}

func TestWarnIPv6SourceAddressFallbackBindFailed(t *testing.T) {
	bindErr := errors.New("bind: cannot assign requested address")
	selectedAddress := netip.MustParseAddr("2001:db8::1234")
	entry := captureIPv6SourceAddressFallbackEntry(t, func(logger ContextLogger) {
		WarnIPv6SourceAddressFallback(
			logger,
			context.Background(),
			"2001:db8::/48",
			"random",
			IPv6SourceAddressFallbackReasonBindFailed,
			selectedAddress,
			bindErr,
		)
	})

	if entry.Level != LevelWarn {
		t.Fatalf("expected warning level, got %v", entry.Level)
	}
	if entry.Event == nil {
		t.Fatal("expected structured event")
	}
	if entry.Event.Data["reason"] != IPv6SourceAddressFallbackReasonBindFailed {
		t.Fatalf("expected bind_failed reason, got %#v", entry.Event.Data["reason"])
	}
	if entry.Event.Data["selected_address"] != selectedAddress.String() {
		t.Fatalf("expected selected_address %q, got %#v", selectedAddress, entry.Event.Data["selected_address"])
	}
	if entry.Event.Data["error"] != bindErr.Error() {
		t.Fatalf("expected error %q, got %#v", bindErr.Error(), entry.Event.Data["error"])
	}
	if !strings.Contains(entry.Message, "reason=bind_failed") ||
		!strings.Contains(entry.Message, "selected_address="+selectedAddress.String()) ||
		!strings.Contains(entry.Message, "error="+bindErr.Error()) {
		t.Fatalf("plain log message missing bind failure details: %q", entry.Message)
	}
	if entry.Level == LevelError {
		t.Fatalf("fallback warning must not log at error level: %v", entry.Level)
	}
}

func captureIPv6SourceAddressFallbackEntry(t *testing.T, logFn func(logger ContextLogger)) LogEntry {
	t.Helper()

	bus := NewEventBus()
	t.Cleanup(bus.Close)

	factory := NewMultiOutputFactoryWithBus(context.Background(), nil, Formatter{}, nil, false, bus)
	factory.SetLevel(LevelTrace)

	subscriber := &testEventSubscriber{entries: make(chan LogEntry, 1)}
	subscriptionID := bus.Subscribe(subscriber, EventFilter{
		EventTypes: []EventType{EventTypeIPv6SourceAddressFallback},
		MinLevel:   LevelWarn,
	}, 1)
	t.Cleanup(func() {
		bus.Unsubscribe(subscriptionID)
	})

	logFn(factory.Logger())

	select {
	case entry := <-subscriber.entries:
		return entry
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ipv6 source address fallback log entry")
		return LogEntry{}
	}
}
