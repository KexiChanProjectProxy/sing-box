package log

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/observable"
)

var _ ObservableFactory = (*multiOutputFactory)(nil)

// multiOutputFactory implements a factory that writes to multiple outputs
type multiOutputFactory struct {
	ctx               context.Context
	outputs           []Output
	platformWriter    PlatformWriter
	platformFormatter Formatter
	needObservable    bool
	level             Level
	subscriber        *observable.Subscriber[Entry]
	observer          *observable.Observer[Entry]
}

// NewMultiOutputFactory creates a new multi-output factory
func NewMultiOutputFactory(
	ctx context.Context,
	outputs []Output,
	platformFormatter Formatter,
	platformWriter PlatformWriter,
	needObservable bool,
) ObservableFactory {
	factory := &multiOutputFactory{
		ctx:               ctx,
		outputs:           outputs,
		platformFormatter: platformFormatter,
		platformWriter:    platformWriter,
		needObservable:    needObservable,
		level:             LevelTrace,
		subscriber:        observable.NewSubscriber[Entry](128),
	}
	if needObservable {
		factory.observer = observable.NewObserver[Entry](factory.subscriber, 64)
	}
	return factory
}

// Start initializes all outputs
func (f *multiOutputFactory) Start() error {
	for _, output := range f.outputs {
		if starter, ok := output.(interface{ Start() error }); ok {
			if err := starter.Start(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close closes all outputs
func (f *multiOutputFactory) Close() error {
	var errors []error
	for _, output := range f.outputs {
		if err := output.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if err := f.subscriber.Close(); err != nil {
		errors = append(errors, err)
	}
	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}

// Level returns the current log level
func (f *multiOutputFactory) Level() Level {
	return f.level
}

// SetLevel sets the log level
func (f *multiOutputFactory) SetLevel(level Level) {
	f.level = level
}

// Logger returns a logger without a tag
func (f *multiOutputFactory) Logger() ContextLogger {
	return f.NewLogger("")
}

// NewLogger returns a logger with a tag
func (f *multiOutputFactory) NewLogger(tag string) ContextLogger {
	return &multiOutputLogger{
		factory: f,
		tag:     tag,
	}
}

// Subscribe subscribes to log entries (for observable pattern)
func (f *multiOutputFactory) Subscribe() (subscription observable.Subscription[Entry], done <-chan struct{}, err error) {
	if f.observer == nil {
		return nil, nil, common.ErrNotInitialized
	}
	return f.observer.Subscribe()
}

// UnSubscribe unsubscribes from log entries
func (f *multiOutputFactory) UnSubscribe(sub observable.Subscription[Entry]) {
	if f.observer != nil {
		f.observer.UnSubscribe(sub)
	}
}

// multiOutputLogger implements ContextLogger for the multi-output factory
type multiOutputLogger struct {
	factory *multiOutputFactory
	tag     string
}

// Log logs a message with the given level
func (l *multiOutputLogger) Log(ctx context.Context, level Level, args []any) {
	l.LogWithEvent(ctx, level, nil, args)
}

// LogWithEvent logs a message with the given level and structured event
func (l *multiOutputLogger) LogWithEvent(ctx context.Context, level Level, event *StructuredEvent, args []any) {
	// Apply level override from context
	level = OverrideLevelFromContext(level, ctx)
	if level > l.factory.level {
		return
	}

	nowTime := time.Now()
	message := F.ToString(args...)

	// Build log entry with metadata
	entry := l.buildLogEntry(ctx, level, message, nowTime)

	// Add structured event if provided
	if event != nil {
		entry.Event = event
	}

	// Write to all outputs (non-blocking)
	for _, output := range l.factory.outputs {
		go output.Write(entry)
	}

	// Emit to observable if needed
	if l.factory.needObservable {
		l.factory.subscriber.Emit(Entry{level, message})
	}

	// Write to platform writer if needed
	if l.factory.platformWriter != nil {
		platformMessage := l.factory.platformFormatter.Format(ctx, level, l.tag, message, nowTime)
		l.factory.platformWriter.WriteMessage(level, platformMessage)
	}
}

// buildLogEntry builds a LogEntry from context and message
func (l *multiOutputLogger) buildLogEntry(ctx context.Context, level Level, message string, timestamp time.Time) LogEntry {
	entry := LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
		Tag:       l.tag,
		Metadata:  make(map[string]interface{}),
	}

	// Extract connection ID
	if id, hasId := IDFromContext(ctx); hasId {
		entry.ConnectionID = id.ID
		entry.ConnectionDuration = time.Since(id.CreatedAt)
	}

	// Extract InboundContext metadata
	if metadata := adapter.ContextFrom(ctx); metadata != nil {
		l.extractInboundMetadata(entry.Metadata, metadata)
	}

	return entry
}

// extractInboundMetadata extracts metadata from InboundContext
func (l *multiOutputLogger) extractInboundMetadata(dest map[string]interface{}, metadata *adapter.InboundContext) {
	// Network type
	if metadata.Network != "" {
		dest["network"] = metadata.Network
	}

	// Source
	if metadata.Source.IsValid() {
		dest["source_ip"] = metadata.Source.Addr.String()
		dest["source_port"] = metadata.Source.Port
	}

	// Destination
	if metadata.Destination.IsValid() {
		dest["dest_ip"] = metadata.Destination.Addr.String()
		dest["dest_port"] = metadata.Destination.Port
		if metadata.Destination.IsFqdn() {
			dest["dest_domain"] = metadata.Destination.Fqdn
		}
	}

	// Destination addresses (resolved IPs)
	if len(metadata.DestinationAddresses) > 0 {
		addresses := make([]string, len(metadata.DestinationAddresses))
		for i, addr := range metadata.DestinationAddresses {
			addresses[i] = addr.String()
		}
		dest["dest_addresses"] = addresses
	}

	// Original destination (if different)
	if metadata.OriginDestination.IsValid() {
		dest["origin_dest_ip"] = metadata.OriginDestination.Addr.String()
		dest["origin_dest_port"] = metadata.OriginDestination.Port
	}

	// Inbound
	if metadata.Inbound != "" {
		dest["inbound_tag"] = metadata.Inbound
	}
	if metadata.InboundType != "" {
		dest["inbound_type"] = metadata.InboundType
	}
	if metadata.User != "" {
		dest["user"] = metadata.User
	}

	// Outbound
	if metadata.Outbound != "" {
		dest["outbound_tag"] = metadata.Outbound
	}

	// Protocol
	if metadata.Protocol != "" {
		dest["protocol"] = metadata.Protocol
	}
	if metadata.Domain != "" && metadata.Domain != metadata.Destination.Fqdn {
		// Include domain if different from destination
		dest["domain"] = metadata.Domain
	}
	if metadata.Client != "" {
		dest["tls_client"] = metadata.Client
	}
	if len(metadata.SnifferNames) > 0 {
		dest["sniffer_names"] = metadata.SnifferNames
	}
	if metadata.SniffError != nil {
		dest["sniff_error"] = metadata.SniffError.Error()
	}

	// Process info
	if metadata.ProcessInfo != nil {
		processInfo := make(map[string]interface{})
		if metadata.ProcessInfo.ProcessID != 0 {
			processInfo["id"] = metadata.ProcessInfo.ProcessID
		}
		if metadata.ProcessInfo.ProcessPath != "" {
			processInfo["path"] = metadata.ProcessInfo.ProcessPath
		}
		if metadata.ProcessInfo.PackageName != "" {
			processInfo["package"] = metadata.ProcessInfo.PackageName
		}
		if metadata.ProcessInfo.User != "" {
			processInfo["user"] = metadata.ProcessInfo.User
		}
		if metadata.ProcessInfo.UserId != 0 {
			processInfo["user_id"] = metadata.ProcessInfo.UserId
		}
		if len(processInfo) > 0 {
			dest["process"] = processInfo
		}
	}

	// GeoIP
	if metadata.SourceGeoIPCode != "" {
		dest["source_geoip"] = metadata.SourceGeoIPCode
	}
	if metadata.GeoIPCode != "" {
		dest["dest_geoip"] = metadata.GeoIPCode
	}

	// Routing
	if metadata.MatchedRuleSet != "" {
		dest["matched_ruleset"] = metadata.MatchedRuleSet
	}

	// DNS
	if metadata.QueryType != 0 {
		dest["dns_query_type"] = metadata.QueryType
	}
	if metadata.FakeIP {
		dest["fake_ip"] = true
	}

	// TLS
	if metadata.TLSFragment {
		dest["tls_fragment"] = true
	}
	if metadata.TLSRecordFragment {
		dest["tls_record_fragment"] = true
	}
	if metadata.TLSFragmentFallbackDelay > 0 {
		dest["tls_fragment_fallback_delay_ms"] = metadata.TLSFragmentFallbackDelay.Milliseconds()
	}

	// Network strategy
	if metadata.NetworkStrategy != nil {
		dest["network_strategy"] = metadata.NetworkStrategy.String()
	}
	if len(metadata.NetworkType) > 0 {
		networkTypes := make([]string, len(metadata.NetworkType))
		for i, nt := range metadata.NetworkType {
			networkTypes[i] = nt.String()
		}
		dest["network_type"] = networkTypes
	}
	if len(metadata.FallbackNetworkType) > 0 {
		fallbackTypes := make([]string, len(metadata.FallbackNetworkType))
		for i, nt := range metadata.FallbackNetworkType {
			fallbackTypes[i] = nt.String()
		}
		dest["fallback_network_type"] = fallbackTypes
	}
	if metadata.FallbackDelay > 0 {
		dest["fallback_delay_ms"] = metadata.FallbackDelay.Milliseconds()
	}
}

// Convenience methods

func (l *multiOutputLogger) Trace(args ...any) {
	l.TraceContext(context.Background(), args...)
}

func (l *multiOutputLogger) Debug(args ...any) {
	l.DebugContext(context.Background(), args...)
}

func (l *multiOutputLogger) Info(args ...any) {
	l.InfoContext(context.Background(), args...)
}

func (l *multiOutputLogger) Warn(args ...any) {
	l.WarnContext(context.Background(), args...)
}

func (l *multiOutputLogger) Error(args ...any) {
	l.ErrorContext(context.Background(), args...)
}

func (l *multiOutputLogger) Fatal(args ...any) {
	l.FatalContext(context.Background(), args...)
}

func (l *multiOutputLogger) Panic(args ...any) {
	l.PanicContext(context.Background(), args...)
}

func (l *multiOutputLogger) TraceContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelTrace, args)
}

func (l *multiOutputLogger) DebugContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelDebug, args)
}

func (l *multiOutputLogger) InfoContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelInfo, args)
}

func (l *multiOutputLogger) WarnContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelWarn, args)
}

func (l *multiOutputLogger) ErrorContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelError, args)
}

func (l *multiOutputLogger) FatalContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelFatal, args)
}

func (l *multiOutputLogger) PanicContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelPanic, args)
}
