package transport

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// SessionManager manages client sessions
type SessionManager struct {
	sessions      map[string]*Session
	mu            sync.RWMutex
	cleanupTicker *time.Ticker
	timeout       time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager(timeout time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions:      make(map[string]*Session),
		timeout:       timeout,
		cleanupTicker: time.NewTicker(5 * time.Minute),
	}
	
	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()
	
	return sm
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(transport string) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}
	
	now := time.Now().Unix()
	session := &Session{
		ID:           sessionID,
		Transport:    transport,
		CreatedAt:    now,
		LastActivity: now,
		Metadata:     make(map[string]interface{}),
	}
	
	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()
	
	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*Session, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	session, exists := sm.sessions[sessionID]
	if exists {
		// Update last activity atomically while holding the lock
		session.LastActivity = time.Now().Unix()
	}
	
	return session, exists
}

// RemoveSession removes a session
func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.mu.Lock()
	delete(sm.sessions, sessionID)
	sm.mu.Unlock()
}

// cleanupExpiredSessions removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	for range sm.cleanupTicker.C {
		now := time.Now().Unix()
		expiredSessions := []string{}
		
		sm.mu.RLock()
		for id, session := range sm.sessions {
			if now-session.LastActivity > int64(sm.timeout.Seconds()) {
				expiredSessions = append(expiredSessions, id)
			}
		}
		sm.mu.RUnlock()
		
		// Remove expired sessions
		for _, id := range expiredSessions {
			sm.RemoveSession(id)
		}
	}
}

// Stop stops the session manager
func (sm *SessionManager) Stop() {
	sm.cleanupTicker.Stop()
}

// generateSessionID generates a random session ID
func generateSessionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// SessionStore provides thread-safe storage for session-specific data
type SessionStore struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

// NewSessionStore creates a new session store
func NewSessionStore() *SessionStore {
	return &SessionStore{
		data: make(map[string]interface{}),
	}
}

// Set stores a value in the session
func (s *SessionStore) Set(key string, value interface{}) {
	s.mu.Lock()
	s.data[key] = value
	s.mu.Unlock()
}

// Get retrieves a value from the session
func (s *SessionStore) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	value, exists := s.data[key]
	s.mu.RUnlock()
	return value, exists
}

// Delete removes a value from the session
func (s *SessionStore) Delete(key string) {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
}

// Clear removes all values from the session
func (s *SessionStore) Clear() {
	s.mu.Lock()
	s.data = make(map[string]interface{})
	s.mu.Unlock()
}