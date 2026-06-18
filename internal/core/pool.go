package core

import (
	"math/rand"
	"sync"
	"time"
)

type Pool struct {
	mu        sync.RWMutex
	items     map[string]Candidate
	dialers   map[string]CandidateDialer
	order     []string
	rr        int
	updatedAt time.Time
}

func NewPool() *Pool {
	return &Pool{items: map[string]Candidate{}, dialers: map[string]CandidateDialer{}, updatedAt: time.Now()}
}

func (p *Pool) Replace(candidates []Candidate) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = map[string]Candidate{}
	p.dialers = map[string]CandidateDialer{}
	p.order = nil
	p.rr = 0
	for _, c := range candidates {
		c.Normalize()
		if _, exists := p.items[c.Fingerprint]; !exists {
			p.order = append(p.order, c.Fingerprint)
		}
		p.items[c.Fingerprint] = c
	}
	p.updatedAt = time.Now()
}

func (p *Pool) AddValidated(candidates []Candidate, dialers map[string]CandidateDialer) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	added := 0
	for _, c := range candidates {
		c.Normalize()
		c.Status = StatusAvailable
		c.FailureCount = 0
		c.LastError = ""
		if existing, ok := p.items[c.Fingerprint]; ok {
			if !existing.CreatedAt.IsZero() {
				c.CreatedAt = existing.CreatedAt
			}
		} else {
			p.order = append(p.order, c.Fingerprint)
		}
		c.UpdatedAt = time.Now()
		p.items[c.Fingerprint] = c
		if dialers != nil {
			if d := dialers[c.Fingerprint]; d != nil {
				p.dialers[c.Fingerprint] = d
			}
		}
		added++
	}
	p.updatedAt = time.Now()
	return added
}

func (p *Pool) RegisterDialer(fingerprint string, dialer CandidateDialer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dialers[fingerprint] = dialer
}

func (p *Pool) Dialer(fingerprint string) CandidateDialer {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.dialers[fingerprint]
}

func (p *Pool) List() []Candidate {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Candidate, 0, len(p.items))
	for _, fp := range p.order {
		if c, ok := p.items[fp]; ok {
			out = append(out, c)
		}
	}
	return out
}

func (p *Pool) Available() []Candidate {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Candidate, 0, len(p.items))
	for _, fp := range p.order {
		if c, ok := p.items[fp]; ok && c.Status == StatusAvailable {
			out = append(out, c)
		}
	}
	return out
}

func (p *Pool) IsAvailable(fingerprint string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	c, ok := p.items[fingerprint]
	return ok && c.Status == StatusAvailable
}

func (p *Pool) Get(fingerprint string) (Candidate, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	c, ok := p.items[fingerprint]
	return c, ok
}

func (p *Pool) GetAvailable(fingerprint string) (Candidate, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	c, ok := p.items[fingerprint]
	return c, ok && c.Status == StatusAvailable
}

func (p *Pool) Count() (total, available int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	total = len(p.items)
	for _, c := range p.items {
		if c.Status == StatusAvailable {
			available++
		}
	}
	return
}

func (p *Pool) Pick(policy string) (Candidate, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if policy == "round_robin" {
		for attempts := 0; attempts < len(p.order); attempts++ {
			fp := p.order[p.rr%len(p.order)]
			p.rr++
			if c, ok := p.items[fp]; ok && c.Status == StatusAvailable {
				return c, true
			}
		}
		return Candidate{}, false
	}
	available := make([]Candidate, 0, len(p.items))
	for _, fp := range p.order {
		if c, ok := p.items[fp]; ok && c.Status == StatusAvailable {
			available = append(available, c)
		}
	}
	if len(available) == 0 {
		return Candidate{}, false
	}
	return available[rand.Intn(len(available))], true
}

func (p *Pool) MarkFailure(fingerprint, reason string, maxFailures int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.items[fingerprint]
	if !ok {
		return
	}
	c.FailureCount++
	c.LastError = reason
	c.UpdatedAt = time.Now()
	if c.FailureCount >= maxFailures {
		c.Status = StatusUnavailable
	}
	p.items[fingerprint] = c
	p.updatedAt = time.Now()
}

func (p *Pool) MarkSuccess(fingerprint string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.items[fingerprint]
	if !ok {
		return
	}
	if c.Status == StatusAvailable && c.FailureCount > 0 {
		c.FailureCount = 0
		c.LastError = ""
		c.UpdatedAt = time.Now()
		p.items[fingerprint] = c
		p.updatedAt = time.Now()
	}
}
