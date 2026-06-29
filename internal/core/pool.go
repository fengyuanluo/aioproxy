package core

import (
	"math/rand"
	"sort"
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

func (p *Pool) ReplaceValidatedMatching(candidates []Candidate, dialers map[string]CandidateDialer, replace func(Candidate) bool) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	normalized := make([]Candidate, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		c.Normalize()
		c.Status = StatusAvailable
		c.FailureCount = 0
		c.LastError = ""
		if existing, ok := p.items[c.Fingerprint]; ok {
			if !existing.CreatedAt.IsZero() {
				c.CreatedAt = existing.CreatedAt
			}
		}
		c.UpdatedAt = time.Now()
		normalized = append(normalized, c)
		seen[c.Fingerprint] = struct{}{}
	}
	if replace != nil {
		nextOrder := p.order[:0]
		for _, fp := range p.order {
			c, ok := p.items[fp]
			if !ok {
				continue
			}
			if replace(c) {
				if _, keep := seen[fp]; !keep {
					delete(p.items, fp)
					if d := p.dialers[fp]; d != nil {
						if resetter, ok := d.(interface{ ResetIdleCache() }); ok {
							resetter.ResetIdleCache()
						}
						delete(p.dialers, fp)
					}
					continue
				}
			}
			nextOrder = append(nextOrder, fp)
		}
		p.order = nextOrder
	}
	for _, c := range normalized {
		if _, exists := p.items[c.Fingerprint]; !exists {
			p.order = append(p.order, c.Fingerprint)
		}
		p.items[c.Fingerprint] = c
		if dialers != nil {
			if d := dialers[c.Fingerprint]; d != nil {
				p.dialers[c.Fingerprint] = d
			}
		}
	}
	p.updatedAt = time.Now()
	return len(normalized)
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

func (p *Pool) ForEach(fn func(Candidate) bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, fp := range p.order {
		c, ok := p.items[fp]
		if !ok {
			continue
		}
		if !fn(c) {
			return
		}
	}
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
	return p.PickMatching(policy, func(Candidate) bool { return true })
}

func (p *Pool) PickMatching(policy string, match func(Candidate) bool) (Candidate, bool) {
	return p.PickMatchingExcluding(policy, match, nil)
}

func (p *Pool) PickMatchingPercentExcluding(policy string, match func(Candidate) bool, exclude map[string]struct{}, percent int) (Candidate, bool) {
	if percent >= 100 {
		return p.PickMatchingExcluding(policy, match, exclude)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	eligible := make([]Candidate, 0, len(p.order))
	for _, fp := range p.order {
		if _, skip := exclude[fp]; skip {
			continue
		}
		if c, ok := p.items[fp]; ok && c.Status == StatusAvailable && match(c) {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return Candidate{}, false
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		li := eligible[i].LastValidationLatency
		lj := eligible[j].LastValidationLatency
		switch {
		case li <= 0 && lj <= 0:
			return false
		case li <= 0:
			return false
		case lj <= 0:
			return true
		default:
			return li < lj
		}
	})
	keep := len(eligible) * percent / 100
	if keep < 1 {
		keep = 1
	}
	if keep > len(eligible) {
		keep = len(eligible)
	}
	eligible = eligible[:keep]
	if policy == "round_robin" {
		idx := p.rr % len(eligible)
		p.rr++
		return eligible[idx], true
	}
	return eligible[rand.Intn(len(eligible))], true
}

func (p *Pool) PickMatchingExcluding(policy string, match func(Candidate) bool, exclude map[string]struct{}) (Candidate, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if policy == "round_robin" {
		for attempts := 0; attempts < len(p.order); attempts++ {
			fp := p.order[p.rr%len(p.order)]
			p.rr++
			if _, skip := exclude[fp]; skip {
				continue
			}
			if c, ok := p.items[fp]; ok && c.Status == StatusAvailable && match(c) {
				return c, true
			}
		}
		return Candidate{}, false
	}
	if len(p.order) == 0 {
		return Candidate{}, false
	}
	start := rand.Intn(len(p.order))
	for attempts := 0; attempts < len(p.order); attempts++ {
		fp := p.order[(start+attempts)%len(p.order)]
		if _, skip := exclude[fp]; skip {
			continue
		}
		if c, ok := p.items[fp]; ok && c.Status == StatusAvailable && match(c) {
			return c, true
		}
	}
	return Candidate{}, false
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

func (p *Pool) MatchingAvailable(match func(Candidate) bool, exclude map[string]struct{}) []Candidate {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Candidate, 0, len(p.items))
	for _, fp := range p.order {
		if _, skip := exclude[fp]; skip {
			continue
		}
		if c, ok := p.items[fp]; ok && c.Status == StatusAvailable && match(c) {
			out = append(out, c)
		}
	}
	return out
}
