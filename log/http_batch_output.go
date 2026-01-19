package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ Output = (*HTTPBatchOutput)(nil)

// HTTPBatchOutput sends logs to an HTTP endpoint in batches
type HTTPBatchOutput struct {
	config        HTTPBatchConfig
	jsonOutput    *JSONOutput
	buffer        []LogEntry
	bufferMutex   sync.Mutex
	httpClient    *http.Client
	flushTicker   *time.Ticker
	closeChan     chan struct{}
	wg            sync.WaitGroup
	errorLogger   ContextLogger
}

// HTTPBatchConfig holds configuration for HTTP batch output
type HTTPBatchConfig struct {
	URL           string
	JWTToken      string
	BatchSize     int
	FlushInterval time.Duration
	Timeout       time.Duration
	Hostname      string
	Version       string
}

// NewHTTPBatchOutput creates a new HTTP batch output
func NewHTTPBatchOutput(config HTTPBatchConfig, errorLogger ContextLogger) Output {
	output := &HTTPBatchOutput{
		config:      config,
		buffer:      make([]LogEntry, 0, config.BatchSize),
		httpClient:  &http.Client{Timeout: config.Timeout},
		closeChan:   make(chan struct{}),
		errorLogger: errorLogger,
	}

	// Create JSON output for formatting
	output.jsonOutput = NewJSONOutput(nil, "", config.Hostname, config.Version).(*JSONOutput)

	// Start flush loop
	output.flushTicker = time.NewTicker(config.FlushInterval)
	output.wg.Add(1)
	go output.flushLoop()

	return output
}

// Write adds a log entry to the buffer
func (o *HTTPBatchOutput) Write(entry LogEntry) error {
	o.bufferMutex.Lock()
	defer o.bufferMutex.Unlock()

	o.buffer = append(o.buffer, entry)

	// Flush if buffer is full
	if len(o.buffer) >= o.config.BatchSize {
		go o.flush()
	}

	return nil
}

// Close flushes remaining logs and stops the output
func (o *HTTPBatchOutput) Close() error {
	close(o.closeChan)
	o.wg.Wait()

	// Final flush
	o.bufferMutex.Lock()
	defer o.bufferMutex.Unlock()
	if len(o.buffer) > 0 {
		o.sendBatch(o.buffer)
	}

	return nil
}

// flushLoop periodically flushes the buffer
func (o *HTTPBatchOutput) flushLoop() {
	defer o.wg.Done()

	for {
		select {
		case <-o.flushTicker.C:
			o.flush()
		case <-o.closeChan:
			o.flushTicker.Stop()
			return
		}
	}
}

// flush copies the buffer and sends it
func (o *HTTPBatchOutput) flush() {
	o.bufferMutex.Lock()
	if len(o.buffer) == 0 {
		o.bufferMutex.Unlock()
		return
	}

	// Copy buffer and clear
	batch := make([]LogEntry, len(o.buffer))
	copy(batch, o.buffer)
	o.buffer = o.buffer[:0]
	o.bufferMutex.Unlock()

	o.sendBatch(batch)
}

// sendBatch sends a batch of log entries to the HTTP endpoint
func (o *HTTPBatchOutput) sendBatch(batch []LogEntry) {
	// Build JSON array
	var buf bytes.Buffer
	buf.WriteString("[")
	for i, entry := range batch {
		if i > 0 {
			buf.WriteString(",")
		}
		doc := o.jsonOutput.buildJSONDocument(entry)
		data, err := json.Marshal(doc)
		if err != nil {
			// Log error but continue
			if o.errorLogger != nil {
				o.errorLogger.Error("failed to marshal log entry: ", err)
			}
			continue
		}
		buf.Write(data)
	}
	buf.WriteString("]")

	// Create HTTP request
	req, err := http.NewRequestWithContext(context.Background(), "POST", o.config.URL, &buf)
	if err != nil {
		if o.errorLogger != nil {
			o.errorLogger.Error("failed to create HTTP request: ", err)
		}
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.config.JWTToken)

	// Send request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		if o.errorLogger != nil {
			o.errorLogger.Error("failed to send log batch to ", o.config.URL, ": ", err)
		}
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		if o.errorLogger != nil {
			o.errorLogger.Error("log batch HTTP request failed with status ", resp.Status, ": ", string(body))
		}
	}
}

// ParseHTTPBatchConfig parses HTTP batch configuration
func ParseHTTPBatchConfig(config option.LogOutput) (HTTPBatchConfig, error) {
	batchSize := config.BatchSize
	if batchSize == 0 {
		batchSize = 100
	}

	flushInterval, err := time.ParseDuration(config.FlushInterval)
	if err != nil || flushInterval == 0 {
		flushInterval = 5 * time.Second
	}

	timeout, err := time.ParseDuration(config.Timeout)
	if err != nil || timeout == 0 {
		timeout = 10 * time.Second
	}

	return HTTPBatchConfig{
		URL:           config.URL,
		JWTToken:      config.JWTToken,
		BatchSize:     batchSize,
		FlushInterval: flushInterval,
		Timeout:       timeout,
		Hostname:      config.Hostname,
		Version:       config.Version,
	}, nil
}

// CreateHTTPOutput creates an HTTP batch output (used by log.go)
func CreateHTTPOutput(config option.LogOutput, baseTime time.Time) (Output, error) {
	if config.URL == "" {
		return nil, E.New("http output requires url")
	}

	httpConfig, err := ParseHTTPBatchConfig(config)
	if err != nil {
		return nil, E.Cause(err, "parse HTTP config")
	}

	// Create a minimal error logger to stderr to avoid circular logging
	errorFormatter := Formatter{
		BaseTime:         baseTime,
		DisableColors:    false,
		DisableTimestamp: false,
		FullTimestamp:    true,
		TimestampFormat:  "-0700 2006-01-02 15:04:05",
	}
	errorLogger := &stderrLogger{
		formatter: errorFormatter,
		tag:       "http-output",
	}

	return NewHTTPBatchOutput(httpConfig, errorLogger), nil
}

// stderrLogger is a minimal logger that writes errors to stderr
type stderrLogger struct {
	formatter Formatter
	tag       string
}

func (l *stderrLogger) Error(args ...any) {
	l.ErrorContext(context.Background(), args...)
}

func (l *stderrLogger) ErrorContext(ctx context.Context, args ...any) {
	message := fmt.Sprint(args...)
	formatted := l.formatter.Format(ctx, LevelError, l.tag, message, time.Now())
	os.Stderr.Write([]byte(formatted))
}

func (l *stderrLogger) Trace(args ...any)                             {}
func (l *stderrLogger) TraceContext(ctx context.Context, args ...any) {}
func (l *stderrLogger) Debug(args ...any)                             {}
func (l *stderrLogger) DebugContext(ctx context.Context, args ...any) {}
func (l *stderrLogger) Info(args ...any)                              {}
func (l *stderrLogger) InfoContext(ctx context.Context, args ...any)  {}
func (l *stderrLogger) Warn(args ...any)                              {}
func (l *stderrLogger) WarnContext(ctx context.Context, args ...any)  {}
func (l *stderrLogger) Fatal(args ...any)                             {}
func (l *stderrLogger) FatalContext(ctx context.Context, args ...any) {}
func (l *stderrLogger) Panic(args ...any)                             {}
func (l *stderrLogger) PanicContext(ctx context.Context, args ...any) {}
