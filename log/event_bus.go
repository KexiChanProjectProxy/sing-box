package log

import (
	"sync"
	"sync/atomic"
)

// EventSubscriber is the interface for receiving log events
type EventSubscriber interface {
	HandleEvent(entry LogEntry)
}

// EventFilter specifies which events a subscriber receives
type EventFilter struct {
	// EventTypes filters by event type. Empty slice means all types.
	EventTypes []EventType
	// MinLevel filters by minimum log level. Zero value means all levels.
	MinLevel Level
}

// matches returns true if the entry passes the filter
func (f *EventFilter) matches(entry LogEntry) bool {
	if f.MinLevel != 0 && entry.Level > f.MinLevel {
		return false
	}
	if len(f.EventTypes) == 0 {
		return true
	}
	if entry.Event == nil {
		return false
	}
	for _, t := range f.EventTypes {
		if entry.Event.Type == t {
			return true
		}
	}
	return false
}

type subscription struct {
	id     int
	sub    EventSubscriber
	filter EventFilter
	ch     chan LogEntry
	done   chan struct{}
}

// EventBus dispatches log events to subscribers asynchronously.
// Publishing never blocks the caller: events are dropped for slow subscribers.
type EventBus struct {
	mu     sync.RWMutex
	subs   map[int]*subscription
	nextID int
	closed atomic.Bool
}

// NewEventBus creates a new EventBus
func NewEventBus() *EventBus {
	return &EventBus{
		subs: make(map[int]*subscription),
	}
}

// Subscribe registers a subscriber and returns its ID.
// bufferSize is the channel buffer; use 0 for the default (256).
func (b *EventBus) Subscribe(sub EventSubscriber, filter EventFilter, bufferSize int) int {
	if bufferSize <= 0 {
		bufferSize = 256
	}

	b.mu.Lock()
	id := b.nextID
	b.nextID++
	s := &subscription{
		id:     id,
		sub:    sub,
		filter: filter,
		ch:     make(chan LogEntry, bufferSize),
		done:   make(chan struct{}),
	}
	b.subs[id] = s
	b.mu.Unlock()

	go s.dispatch()
	return id
}

// Unsubscribe removes a subscriber by ID
func (b *EventBus) Unsubscribe(id int) {
	b.mu.Lock()
	s, ok := b.subs[id]
	if ok {
		delete(b.subs, id)
	}
	b.mu.Unlock()

	if ok {
		close(s.done)
	}
}

// Publish dispatches an entry to all matching subscribers (non-blocking)
func (b *EventBus) Publish(entry LogEntry) {
	if b.closed.Load() {
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, s := range b.subs {
		if s.filter.matches(entry) {
			select {
			case s.ch <- entry:
			default:
				// Drop: subscriber too slow; proxy must not block
			}
		}
	}
}

// Close shuts down all subscriber goroutines
func (b *EventBus) Close() {
	if !b.closed.CompareAndSwap(false, true) {
		return
	}

	b.mu.Lock()
	subs := make([]*subscription, 0, len(b.subs))
	for _, s := range b.subs {
		subs = append(subs, s)
	}
	b.subs = make(map[int]*subscription)
	b.mu.Unlock()

	for _, s := range subs {
		close(s.done)
	}
}

func (s *subscription) dispatch() {
	for {
		select {
		case entry, ok := <-s.ch:
			if !ok {
				return
			}
			s.sub.HandleEvent(entry)
		case <-s.done:
			// Drain remaining entries
			for {
				select {
				case entry := <-s.ch:
					s.sub.HandleEvent(entry)
				default:
					return
				}
			}
		}
	}
}
