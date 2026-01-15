package v2rayxhttp

import (
	"sync"
	"time"
)

// httpSession represents a single xhttp session with upload queue and connection status
type httpSession struct {
	uploadQueue      *uploadQueue
	isFullyConnected chan struct{} // closed when GET request arrives
}

// sessionManager manages HTTP sessions with automatic cleanup
type sessionManager struct {
	sessions sync.Map // map[string]*httpSession
	mu       sync.Mutex
}

func newSessionManager() *sessionManager {
	return &sessionManager{}
}

// getOrCreateSession retrieves an existing session or creates a new one with TTL
func (sm *sessionManager) getOrCreateSession(sessionID string, maxBufferedPosts int) *httpSession {
	// Fast path: check if session already exists
	if sessionAny, ok := sm.sessions.Load(sessionID); ok {
		return sessionAny.(*httpSession)
	}

	// Slow path: create new session with lock
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Double-check after acquiring lock
	if sessionAny, ok := sm.sessions.Load(sessionID); ok {
		return sessionAny.(*httpSession)
	}

	// Create new session
	session := &httpSession{
		uploadQueue:      NewUploadQueue(maxBufferedPosts),
		isFullyConnected: make(chan struct{}),
	}

	sm.sessions.Store(sessionID, session)

	// Start cleanup timer (30 seconds)
	shouldDelete := make(chan struct{})
	go func() {
		time.Sleep(30 * time.Second)
		close(shouldDelete)
	}()

	// Cleanup goroutine: delete session if not fully connected within 30s
	go func() {
		select {
		case <-shouldDelete:
			// Session expired, delete it
			sm.sessions.Delete(sessionID)
			session.uploadQueue.Close()
		case <-session.isFullyConnected:
			// Session fully connected, keep it alive
		}
	}()

	return session
}

// getSession retrieves an existing session
func (sm *sessionManager) getSession(sessionID string) (*httpSession, bool) {
	sessionAny, ok := sm.sessions.Load(sessionID)
	if !ok {
		return nil, false
	}
	return sessionAny.(*httpSession), true
}

// deleteSession removes a session from the manager
func (sm *sessionManager) deleteSession(sessionID string) {
	if sessionAny, ok := sm.sessions.LoadAndDelete(sessionID); ok {
		session := sessionAny.(*httpSession)
		session.uploadQueue.Close()
	}
}
