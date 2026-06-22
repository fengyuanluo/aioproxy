package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

const (
	ProtocolHTTP    = "http"
	ProtocolSOCKS5  = "socks5"
	ProtocolSingBox = "singbox"

	StatusAvailable   = "available"
	StatusUnavailable = "unavailable"
)

type Candidate struct {
	ID             string            `json:"id"`
	Fingerprint    string            `json:"fingerprint"`
	Protocol       string            `json:"protocol"`
	Host           string            `json:"host,omitempty"`
	Port           int               `json:"port,omitempty"`
	Username       string            `json:"username,omitempty"`
	Password       string            `json:"password,omitempty"`
	Source         string            `json:"source"`
	Name           string            `json:"name,omitempty"`
	Status         string            `json:"status"`
	FailureCount   int               `json:"failure_count"`
	LastValidation time.Time         `json:"last_validation,omitempty"`
	LastError      string            `json:"last_error,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func (c Candidate) MatchesRoute(plugin, region string) bool {
	if plugin != "" && !strings.EqualFold(c.Source, plugin) {
		return false
	}
	if region != "" && !strings.EqualFold(strings.TrimSpace(c.Metadata["country_code"]), region) {
		return false
	}
	return true
}

func (c *Candidate) Normalize() {
	c.Protocol = strings.ToLower(strings.TrimSpace(c.Protocol))
	c.Host = strings.TrimSpace(c.Host)
	c.Source = strings.TrimSpace(c.Source)
	if c.Metadata == nil {
		c.Metadata = map[string]string{}
	}
	if c.Status == "" {
		c.Status = StatusAvailable
	}
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	c.Fingerprint = c.CanonicalFingerprint()
	if c.ID == "" {
		c.ID = c.Fingerprint[:16]
	}
}

func (c Candidate) CanonicalFingerprint() string {
	parts := []string{strings.ToLower(c.Protocol), strings.ToLower(c.Host), fmt.Sprint(c.Port), c.Username, c.Password}
	if c.Protocol == ProtocolSingBox {
		keys := make([]string, 0, len(c.Metadata))
		for k := range c.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k == "tag" || k == "node_id" || k == "config_hash" || k == "raw_hash" {
				parts = append(parts, k+"="+c.Metadata[k])
			}
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (c Candidate) Address() string {
	if c.Host == "" || c.Port == 0 {
		return ""
	}
	return net.JoinHostPort(c.Host, fmt.Sprint(c.Port))
}

type CandidateDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type ImportReport struct {
	Plugin      string            `json:"plugin"`
	Source      string            `json:"source"`
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  time.Time         `json:"finished_at"`
	Total       int               `json:"total"`
	Imported    int               `json:"imported"`
	Validated   int               `json:"validated"`
	Skipped     int               `json:"skipped"`
	SkipReasons map[string]int    `json:"skip_reasons,omitempty"`
	Error       string            `json:"error,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func (r *ImportReport) AddSkip(reason string) {
	r.Skipped++
	if r.SkipReasons == nil {
		r.SkipReasons = map[string]int{}
	}
	r.SkipReasons[reason]++
}

type PluginResult struct {
	Candidates []Candidate
	Dialers    map[string]CandidateDialer
	Reports    []ImportReport
}

type PluginStatus struct {
	Name        string         `json:"name"`
	Active      bool           `json:"active"`
	Degraded    bool           `json:"degraded"`
	LastRefresh time.Time      `json:"last_refresh,omitempty"`
	LastError   string         `json:"last_error,omitempty"`
	Reports     []ImportReport `json:"reports,omitempty"`
}
