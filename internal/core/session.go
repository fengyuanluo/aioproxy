package core

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type SessionManager struct {
	mu         sync.Mutex
	bindings   map[string]sessionBinding
	defaultTTL time.Duration
	maxTTL     time.Duration
	lastSweep  time.Time
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
	Plugin     string
	Region     string
}

func NewSessionManager(defaultTTL, maxTTL time.Duration) *SessionManager {
	return &SessionManager{bindings: map[string]sessionBinding{}, defaultTTL: defaultTTL, maxTTL: maxTTL}
}

func ParseSessionUsername(username, credential string, defaultTTL, maxTTL time.Duration) (SessionInfo, bool) {
	if strings.Contains(username, "~") {
		return ParseStructuredUsername(username, credential, defaultTTL, maxTTL)
	}
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

func ParseStructuredUsername(username, credential string, defaultTTL, maxTTL time.Duration) (SessionInfo, bool) {
	parts := strings.Split(username, "~")
	if len(parts) == 0 || parts[0] != credential {
		return SessionInfo{}, false
	}
	info := SessionInfo{Credential: credential}
	seen := map[string]bool{}
	var ttlRaw string
	for _, part := range parts[1:] {
		if part == "" {
			return SessionInfo{}, false
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return SessionInfo{}, false
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])
		if key == "" || val == "" || seen[key] {
			return SessionInfo{}, false
		}
		seen[key] = true
		switch key {
		case "plugin":
			info.Plugin = strings.ToLower(val)
		case "region":
			info.Region = strings.ToUpper(val)
		case "session":
			info.SessionID = val
		case "ttl":
			ttlRaw = val
		default:
			return SessionInfo{}, false
		}
	}
	if ttlRaw != "" {
		if info.SessionID == "" {
			return SessionInfo{}, false
		}
		ttl, err := time.ParseDuration(ttlRaw)
		if err != nil {
			return SessionInfo{}, false
		}
		info.TTL = ttl
	}
	if info.TTL <= 0 && info.SessionID != "" {
		info.TTL = defaultTTL
	}
	if info.TTL > 0 && maxTTL > 0 && info.TTL > maxTTL {
		info.TTL = maxTTL
	}
	return info, true
}

func (i SessionInfo) BindingKey() string {
	if i.SessionID == "" {
		return ""
	}
	return fmt.Sprintf("plugin=%s|region=%s|session=%s", strings.ToLower(i.Plugin), strings.ToUpper(i.Region), i.SessionID)
}

func (m *SessionManager) Pick(info SessionInfo, pool *Pool, policy string) (Candidate, bool) {
	match := func(c Candidate) bool { return c.MatchesRoute(info.Plugin, info.Region) }
	if info.SessionID == "" {
		return pool.PickMatching(policy, match)
	}
	bindingKey := info.BindingKey()
	now := time.Now()
	m.mu.Lock()
	m.sweepExpiredLocked(now, time.Minute)
	if b, ok := m.bindings[bindingKey]; ok && now.Before(b.ExpiresAt) {
		c, available := pool.GetAvailable(b.Fingerprint)
		if !available || !match(c) {
			delete(m.bindings, bindingKey)
		} else {
			b.ExpiresAt = now.Add(b.TTL)
			m.bindings[bindingKey] = b
			m.mu.Unlock()
			return c, true
		}
	}
	c, ok := pool.PickMatching(policy, match)
	if !ok {
		m.mu.Unlock()
		return Candidate{}, false
	}
	ttl := info.TTL
	if ttl <= 0 {
		ttl = m.defaultTTL
	}
	if m.maxTTL > 0 && ttl > m.maxTTL {
		ttl = m.maxTTL
	}
	m.bindings[bindingKey] = sessionBinding{Fingerprint: c.Fingerprint, ExpiresAt: now.Add(ttl), TTL: ttl}
	m.mu.Unlock()
	return c, true
}

func (m *SessionManager) sweepExpiredLocked(now time.Time, interval time.Duration) {
	if interval > 0 && !m.lastSweep.IsZero() && now.Sub(m.lastSweep) < interval {
		return
	}
	for k, v := range m.bindings {
		if !now.Before(v.ExpiresAt) {
			delete(m.bindings, k)
		}
	}
	m.lastSweep = now
}

func (m *SessionManager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepExpiredLocked(time.Now(), 0)
	return len(m.bindings)
}
