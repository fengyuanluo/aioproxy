package core

import (
	"strings"
	"sync"
	"time"
)

type SessionManager struct {
	mu         sync.Mutex
	bindings   map[string]sessionBinding
	defaultTTL time.Duration
	maxTTL     time.Duration
}

type sessionBinding struct {
	Fingerprint string
	ExpiresAt   time.Time
	TTL         time.Duration
}

type SessionInfo struct {
	Credential string
	SessionID  string
	TTL        time.Duration
}

func NewSessionManager(defaultTTL, maxTTL time.Duration) *SessionManager {
	return &SessionManager{bindings: map[string]sessionBinding{}, defaultTTL: defaultTTL, maxTTL: maxTTL}
}

func ParseSessionUsername(username, credential string, defaultTTL, maxTTL time.Duration) (SessionInfo, bool) {
	if username == credential {
		return SessionInfo{Credential: credential}, true
	}
	prefix := credential + "-"
	if !strings.HasPrefix(username, prefix) {
		return SessionInfo{}, false
	}
	rest := strings.TrimPrefix(username, prefix)
	if rest == "" {
		return SessionInfo{Credential: credential}, true
	}
	ttl := defaultTTL
	sessionID := rest
	if i := strings.LastIndex(rest, "-"); i > 0 && i < len(rest)-1 {
		if parsed, err := time.ParseDuration(rest[i+1:]); err == nil {
			sessionID = rest[:i]
			ttl = parsed
		}
	}
	if ttl <= 0 {
		ttl = defaultTTL
	}
	if maxTTL > 0 && ttl > maxTTL {
		ttl = maxTTL
	}
	return SessionInfo{Credential: credential, SessionID: sessionID, TTL: ttl}, true
}

func (m *SessionManager) Pick(info SessionInfo, pool *Pool, policy string) (Candidate, bool) {
	if info.SessionID == "" {
		return pool.Pick(policy)
	}
	now := time.Now()
	m.mu.Lock()
	if b, ok := m.bindings[info.SessionID]; ok && now.Before(b.ExpiresAt) && pool.IsAvailable(b.Fingerprint) {
		b.ExpiresAt = now.Add(b.TTL)
		m.bindings[info.SessionID] = b
		m.mu.Unlock()
		return pool.Get(b.Fingerprint)
	}
	m.mu.Unlock()
	c, ok := pool.Pick(policy)
	if !ok {
		return Candidate{}, false
	}
	ttl := info.TTL
	if ttl <= 0 {
		ttl = m.defaultTTL
	}
	if m.maxTTL > 0 && ttl > m.maxTTL {
		ttl = m.maxTTL
	}
	m.mu.Lock()
	m.bindings[info.SessionID] = sessionBinding{Fingerprint: c.Fingerprint, ExpiresAt: now.Add(ttl), TTL: ttl}
	m.mu.Unlock()
	return c, true
}

func (m *SessionManager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, v := range m.bindings {
		if now.After(v.ExpiresAt) {
			delete(m.bindings, k)
		}
	}
	return len(m.bindings)
}
