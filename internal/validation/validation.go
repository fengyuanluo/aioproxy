package validation

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
	"github.com/aioproxy/aioproxy/internal/proxy"
)

type Validator struct{ cfg config.ValidationConfig }

func New(cfg config.ValidationConfig) *Validator { return &Validator{cfg: cfg} }

func (v *Validator) Validate(ctx context.Context, candidates []core.Candidate, dialers map[string]core.CandidateDialer) []core.Candidate {
	if v.cfg.SkipValidation {
		return candidates
	}
	if len(candidates) == 0 {
		return nil
	}
	workers := v.cfg.Concurrency
	if workers <= 0 {
		workers = 50
	}
	in := make(chan core.Candidate)
	out := make(chan core.Candidate, len(candidates))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range in {
				var dialer core.CandidateDialer
				if dialers != nil {
					dialer = dialers[c.Fingerprint]
				}
				if validated, err := v.ValidateCandidate(ctx, c, dialer); err == nil {
					validated.LastValidation = time.Now()
					out <- validated
				}
			}
		}()
	}
sendLoop:
	for _, c := range candidates {
		select {
		case <-ctx.Done():
			break sendLoop
		case in <- c:
		}
	}
	close(in)
	wg.Wait()
	close(out)
	valid := make([]core.Candidate, 0, len(candidates))
	for c := range out {
		valid = append(valid, c)
	}
	return valid
}

func (v *Validator) ValidateOne(parent context.Context, c core.Candidate, dialer core.CandidateDialer) error {
	_, err := v.ValidateCandidate(parent, c, dialer)
	return err
}

func (v *Validator) ValidateCandidate(parent context.Context, c core.Candidate, dialer core.CandidateDialer) (core.Candidate, error) {
	ctx, cancel := context.WithTimeout(parent, v.cfg.Timeout.Duration)
	defer cancel()
	validationURL := v.cfg.URL
	if validationURL == "" {
		validationURL = config.DefaultValidationURL
	}
	u, err := url.Parse(validationURL)
	if err != nil {
		return c, err
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	target := net.JoinHostPort(u.Hostname(), port)
	conn, err := proxy.DialViaCandidate(ctx, c, target, dialer)
	if err != nil {
		return c, err
	}
	defer conn.Close()
	transport := &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) { return conn, nil }, DisableKeepAlives: true, TLSClientConfig: &tls.Config{InsecureSkipVerify: v.cfg.TLSInsecure}}
	client := &http.Client{Transport: transport, Timeout: v.cfg.Timeout.Duration, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validationURL, nil)
	if err != nil {
		return c, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return c, err
	}
	defer resp.Body.Close()

	switch v.cfg.Strategy {
	case "", config.ValidationStrategyHTTPStatus:
		for _, code := range v.cfg.SuccessStatus {
			if resp.StatusCode == code {
				return c, nil
			}
		}
		return c, fmt.Errorf("validation status %d", resp.StatusCode)
	case config.ValidationStrategyIPAPICountry:
		if resp.StatusCode != http.StatusOK {
			return c, fmt.Errorf("validation status %d", resp.StatusCode)
		}
		var payload struct {
			Status      string `json:"status"`
			Message     string `json:"message"`
			Country     string `json:"country"`
			CountryCode string `json:"countryCode"`
			Query       string `json:"query"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return c, err
		}
		if payload.Status != "success" {
			if payload.Message == "" {
				payload.Message = "ip-api status not success"
			}
			return c, errors.New(payload.Message)
		}
		if strings.TrimSpace(payload.CountryCode) == "" {
			return c, fmt.Errorf("ip-api countryCode missing")
		}
		if c.Metadata == nil {
			c.Metadata = map[string]string{}
		}
		c.Metadata["country_code"] = strings.ToUpper(strings.TrimSpace(payload.CountryCode))
		if strings.TrimSpace(payload.Country) != "" {
			c.Metadata["country"] = strings.TrimSpace(payload.Country)
		}
		if strings.TrimSpace(payload.Query) != "" {
			c.Metadata["query_ip"] = strings.TrimSpace(payload.Query)
		}
		return c, nil
	default:
		return c, fmt.Errorf("unsupported validation strategy %s", v.cfg.Strategy)
	}
}
