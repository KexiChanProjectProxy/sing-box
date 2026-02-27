package log

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// WebhookSubscriber batches structured log events and POSTs them as JSON
type WebhookSubscriber struct {
	url           string
	headers       map[string]string
	batchSize     int
	flushInterval time.Duration
	timeout       time.Duration

	mu     sync.Mutex
	batch  []map[string]interface{}
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// WebhookSubscriberConfig configures a WebhookSubscriber
type WebhookSubscriberConfig struct {
	URL           string
	Headers       map[string]string
	BatchSize     int
	FlushInterval time.Duration
	Timeout       time.Duration
}

// NewWebhookSubscriber creates a WebhookSubscriber and starts its flush goroutine
func NewWebhookSubscriber(cfg WebhookSubscriberConfig) *WebhookSubscriber {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}

	w := &WebhookSubscriber{
		url:           cfg.URL,
		headers:       cfg.Headers,
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		timeout:       cfg.Timeout,
		stopCh:        make(chan struct{}),
	}

	w.wg.Add(1)
	go w.flushLoop()
	return w
}

// HandleEvent implements EventSubscriber
func (w *WebhookSubscriber) HandleEvent(entry LogEntry) {
	if entry.Event == nil {
		return
	}

	doc := w.buildDoc(entry)

	w.mu.Lock()
	w.batch = append(w.batch, doc)
	flush := len(w.batch) >= w.batchSize
	w.mu.Unlock()

	if flush {
		go w.flush()
	}
}

func (w *WebhookSubscriber) buildDoc(entry LogEntry) map[string]interface{} {
	doc := map[string]interface{}{
		"@timestamp": entry.Timestamp.UTC().Format(time.RFC3339Nano),
		"level":      FormatLevel(entry.Level),
		"message":    entry.Message,
	}
	if entry.Tag != "" {
		doc["tag"] = entry.Tag
	}
	if entry.ConnectionID != 0 {
		doc["connection_id"] = entry.ConnectionID
	}
	if entry.Event != nil {
		event := map[string]interface{}{
			"type": string(entry.Event.Type),
		}
		for k, v := range entry.Event.Data {
			event[k] = v
		}
		doc["event"] = event
	}
	return doc
}

func (w *WebhookSubscriber) flushLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.flush()
		case <-w.stopCh:
			w.flush()
			return
		}
	}
}

func (w *WebhookSubscriber) flush() {
	w.mu.Lock()
	if len(w.batch) == 0 {
		w.mu.Unlock()
		return
	}
	batch := w.batch
	w.batch = nil
	w.mu.Unlock()

	payload, err := json.Marshal(batch)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// Close flushes remaining events and stops the flush goroutine
func (w *WebhookSubscriber) Close() {
	close(w.stopCh)
	w.wg.Wait()
}
