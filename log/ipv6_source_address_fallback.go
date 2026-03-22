package log

import (
	"context"
	"net/netip"
)

const (
	EventTypeIPv6SourceAddressFallback EventType = "ipv6_source_address_fallback"

	IPv6SourceAddressFallbackActionFallback = "fallback"

	IPv6SourceAddressFallbackReasonNoCandidate = "no_candidate"
	IPv6SourceAddressFallbackReasonBindFailed  = "bind_failed"
)

type IPv6SourceAddressFallbackEvent struct {
	Action          string `json:"action"`
	Range           string `json:"range,omitempty"`
	Mode            string `json:"mode,omitempty"`
	Reason          string `json:"reason,omitempty"`
	SelectedAddress string `json:"selected_address,omitempty"`
	Error           string `json:"error,omitempty"`
}

func NewIPv6SourceAddressFallbackEvent(addressRange, mode, reason string) *IPv6SourceAddressFallbackEvent {
	return &IPv6SourceAddressFallbackEvent{
		Action: IPv6SourceAddressFallbackActionFallback,
		Range:  addressRange,
		Mode:   mode,
		Reason: reason,
	}
}

func (e *IPv6SourceAddressFallbackEvent) WithSelectedAddress(address netip.Addr) *IPv6SourceAddressFallbackEvent {
	if address.IsValid() {
		e.SelectedAddress = address.String()
	}
	return e
}

func (e *IPv6SourceAddressFallbackEvent) WithError(err error) *IPv6SourceAddressFallbackEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *IPv6SourceAddressFallbackEvent) ToMap() map[string]any {
	m := map[string]any{
		"action": e.Action,
	}
	if e.Range != "" {
		m["range"] = e.Range
	}
	if e.Mode != "" {
		m["mode"] = e.Mode
	}
	if e.Reason != "" {
		m["reason"] = e.Reason
	}
	if e.SelectedAddress != "" {
		m["selected_address"] = e.SelectedAddress
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *IPv6SourceAddressFallbackEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{
		Type: EventTypeIPv6SourceAddressFallback,
		Data: e.ToMap(),
	}
}

func WithIPv6SourceAddressFallbackEvent(logger ContextLogger, ctx context.Context, level Level, event *IPv6SourceAddressFallbackEvent, args ...any) {
	if ml, ok := logger.(*multiOutputLogger); ok {
		ml.LogWithEvent(ctx, level, event.ToStructuredEvent(), args)
	} else {
		logWithLevel(logger, ctx, level, args)
	}
}

func WarnIPv6SourceAddressFallback(logger ContextLogger, ctx context.Context, addressRange, mode, reason string, selectedAddress netip.Addr, err error) {
	event := NewIPv6SourceAddressFallbackEvent(addressRange, mode, reason).
		WithSelectedAddress(selectedAddress).
		WithError(err)

	args := []any{
		"ipv6_source_address_range fallback (range=", addressRange,
		", mode=", mode,
		", reason=", reason,
	}
	if selectedAddress.IsValid() {
		args = append(args, ", selected_address=", selectedAddress.String())
	}
	if err != nil {
		args = append(args, ", error=", err.Error())
	}
	args = append(args, ")")

	WithIPv6SourceAddressFallbackEvent(logger, ctx, LevelWarn, event, args...)
}
