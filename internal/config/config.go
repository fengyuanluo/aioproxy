package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultProxyListen             = "127.0.0.1:1080"
	DefaultAdminListen             = "127.0.0.1:1081"
	DefaultFPLURL                  = "https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/all/data.txt"
	DefaultValidationURL           = "http://www.gstatic.com/generate_204"
	DefaultIPAPICountryURL         = "http://ip-api.com/json/?fields=status,message,country,countryCode,query"
	DefaultSchedulerPolicy         = "random"
	DefaultCredentialUser          = "aio"
	DefaultCredentialPass          = "change-me"
	DefaultSnapshotRetain          = 7
	DefaultFOFABaseURL             = "http://fofa.icu"
	DefaultFOFASize                = 100
	DefaultFOFAFields              = "ip,port,protocol,host"
	DefaultGracefulShutdown        = 15 * time.Second
	ValidationStrategyHTTPStatus   = "http_status"
	ValidationStrategyIPAPICountry = "ip_api_country"
)

type Duration struct{ time.Duration }

func (d Duration) MarshalYAML() (any, error) { return d.Duration.String(), nil }

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode && value.Value == "" {
		d.Duration = 0
		return nil
	}
	var s string
	if err := value.Decode(&s); err == nil {
		if s == "" {
			d.Duration = 0
			return nil
		}
		v, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		d.Duration = v
		return nil
	}
	var n int64
	if err := value.Decode(&n); err == nil {
		d.Duration = time.Duration(n)
		return nil
	}
	return fmt.Errorf("invalid duration")
}

type ByteSize struct{ Bytes int64 }

func (b *ByteSize) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err == nil {
		if s == "" {
			b.Bytes = 0
			return nil
		}
		v, err := ParseByteSize(s)
		if err != nil {
			return err
		}
		b.Bytes = v
		return nil
	}
	var n int64
	if err := value.Decode(&n); err == nil {
		b.Bytes = n
		return nil
	}
	return fmt.Errorf("invalid byte size")
}

func ParseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	mult := int64(1)
	for _, suffix := range []struct {
		s string
		m int64
	}{
		{"KB", 1000}, {"K", 1000}, {"MB", 1000 * 1000}, {"M", 1000 * 1000},
		{"GB", 1000 * 1000 * 1000}, {"G", 1000 * 1000 * 1000},
	} {
		if strings.HasSuffix(s, suffix.s) {
			mult = suffix.m
			s = strings.TrimSpace(strings.TrimSuffix(s, suffix.s))
			break
		}
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("invalid byte size %q", s)
	}
	return int64(f * float64(mult)), nil
}

type Config struct {
	Server         ServerConfig         `yaml:"server"`
	Admin          AdminConfig          `yaml:"admin"`
	Auth           AuthConfig           `yaml:"auth"`
	Scheduler      SchedulerConfig      `yaml:"scheduler"`
	Session        SessionConfig        `yaml:"session"`
	Validation     ValidationConfig     `yaml:"validation"`
	RuntimeFailure RuntimeFailureConfig `yaml:"runtime_failure"`
	Storage        StorageConfig        `yaml:"storage"`
	Logging        LoggingConfig        `yaml:"logging"`
	Lifecycle      LifecycleConfig      `yaml:"lifecycle"`
	Refresh        RefreshConfig        `yaml:"refresh"`
	Plugins        PluginsConfig        `yaml:"plugins"`
}

type ServerConfig struct {
	Listen string `yaml:"listen"`
}

type AdminConfig struct {
	Listen string `yaml:"listen"`
	Token  string `yaml:"token"`
}

type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	seen     bool
}

func (a *AuthConfig) UnmarshalYAML(value *yaml.Node) error {
	type raw AuthConfig
	a.seen = true
	a.Enabled = true
	return value.Decode((*raw)(a))
}

type SchedulerConfig struct {
	Policy string `yaml:"policy"`
}

type SessionConfig struct {
	DefaultTTL Duration `yaml:"default_ttl"`
	MaxTTL     Duration `yaml:"max_ttl"`
}

type ValidationConfig struct {
	Strategy       string   `yaml:"strategy"`
	URL            string   `yaml:"url"`
	SuccessStatus  []int    `yaml:"success_status"`
	Timeout        Duration `yaml:"timeout"`
	Concurrency    int      `yaml:"concurrency"`
	TLSInsecure    bool     `yaml:"tls_insecure"`
	SkipValidation bool     `yaml:"skip_validation"`
}

type RuntimeFailureConfig struct {
	MaxFailures        int      `yaml:"max_failures"`
	EarlyFailureWindow Duration `yaml:"early_failure_window"`
	RetryAttempts      int      `yaml:"retry_attempts"`
}

type StorageConfig struct {
	DataDir           string `yaml:"data_dir"`
	SnapshotRetention int    `yaml:"snapshot_retention"`
}

type LoggingConfig struct {
	File     string             `yaml:"file"`
	Level    string             `yaml:"level"`
	Format   string             `yaml:"format"`
	Rotation LoggingRotationCfg `yaml:"rotation"`
}

type LoggingRotationCfg struct {
	MaxSize    ByteSize `yaml:"max_size"`
	MaxBackups int      `yaml:"max_backups"`
	MaxAgeDays int      `yaml:"max_age_days"`
	Compress   bool     `yaml:"compress"`
}

type LifecycleConfig struct {
	GracePeriod Duration `yaml:"grace_period"`
}

type RefreshConfig struct {
	JitterRatio float64 `yaml:"jitter_ratio"`
}

type PluginsConfig struct {
	FPL     *FPLConfig     `yaml:"fpl"`
	FOFA    *FOFAConfig    `yaml:"fofa"`
	SingBox *SingBoxConfig `yaml:"singbox"`
}

type FPLConfig struct {
	URL             string   `yaml:"url"`
	RefreshInterval Duration `yaml:"refresh_interval"`
}

type FOFAConfig struct {
	BaseURL         string            `yaml:"base_url"`
	Key             string            `yaml:"key"`
	Size            int               `yaml:"size"`
	RefreshInterval Duration          `yaml:"refresh_interval"`
	Queries         []FOFAQueryConfig `yaml:"queries"`
}

type FOFAQueryConfig struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol"`
	Query    string `yaml:"query"`
	Fields   string `yaml:"fields"`
}

type SingBoxConfig struct {
	RefreshInterval Duration              `yaml:"refresh_interval"`
	Sources         []SingBoxSourceConfig `yaml:"sources"`
}

type SingBoxSourceConfig struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	URL  string `yaml:"url"`
	Path string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	c.ApplyDefaults()
	return &c, nil
}

func (c *Config) ApplyDefaults() {
	if c.Server.Listen == "" {
		c.Server.Listen = DefaultProxyListen
	}
	if c.Admin.Listen == "" {
		c.Admin.Listen = DefaultAdminListen
	}
	if !c.Auth.seen {
		c.Auth.Enabled = true
	}
	if c.Auth.Username == "" {
		c.Auth.Username = DefaultCredentialUser
	}
	if c.Auth.Password == "" {
		c.Auth.Password = DefaultCredentialPass
	}
	// bool zero-value cannot distinguish omitted from false; example config sets true explicitly.
	if c.Scheduler.Policy == "" {
		c.Scheduler.Policy = DefaultSchedulerPolicy
	}
	if c.Session.DefaultTTL.Duration == 0 {
		c.Session.DefaultTTL.Duration = 30 * time.Minute
	}
	if c.Session.MaxTTL.Duration == 0 {
		c.Session.MaxTTL.Duration = 24 * time.Hour
	}
	if c.Validation.Strategy == "" {
		c.Validation.Strategy = ValidationStrategyHTTPStatus
	}
	if c.Validation.URL == "" {
		if c.Validation.Strategy == ValidationStrategyIPAPICountry {
			c.Validation.URL = DefaultIPAPICountryURL
		} else {
			c.Validation.URL = DefaultValidationURL
		}
	}
	if len(c.Validation.SuccessStatus) == 0 {
		c.Validation.SuccessStatus = []int{204, 200, 301, 302}
	}
	if c.Validation.Timeout.Duration == 0 {
		c.Validation.Timeout.Duration = 8 * time.Second
	}
	if c.Validation.Concurrency == 0 {
		c.Validation.Concurrency = 100
	}
	if c.RuntimeFailure.MaxFailures == 0 {
		c.RuntimeFailure.MaxFailures = 3
	}
	if c.RuntimeFailure.RetryAttempts == 0 {
		c.RuntimeFailure.RetryAttempts = 3
	}
	if c.RuntimeFailure.EarlyFailureWindow.Duration == 0 {
		c.RuntimeFailure.EarlyFailureWindow.Duration = 3 * time.Second
	}
	if c.Storage.DataDir == "" {
		c.Storage.DataDir = "./data"
	}
	if c.Storage.SnapshotRetention == 0 {
		c.Storage.SnapshotRetention = DefaultSnapshotRetain
	}
	if c.Logging.File == "" {
		c.Logging.File = "./logs/aioproxy.log"
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}
	if c.Logging.Rotation.MaxSize.Bytes == 0 {
		c.Logging.Rotation.MaxSize.Bytes = 100 * 1000 * 1000
	}
	if c.Logging.Rotation.MaxBackups == 0 {
		c.Logging.Rotation.MaxBackups = 7
	}
	if c.Lifecycle.GracePeriod.Duration == 0 {
		c.Lifecycle.GracePeriod.Duration = DefaultGracefulShutdown
	}
	if c.Refresh.JitterRatio == 0 {
		c.Refresh.JitterRatio = 0.1
	}
	if c.Plugins.FPL != nil {
		if c.Plugins.FPL.URL == "" {
			c.Plugins.FPL.URL = DefaultFPLURL
		}
		if c.Plugins.FPL.RefreshInterval.Duration == 0 {
			c.Plugins.FPL.RefreshInterval.Duration = 6 * time.Hour
		}
	}
	if c.Plugins.FOFA != nil {
		if c.Plugins.FOFA.BaseURL == "" {
			c.Plugins.FOFA.BaseURL = DefaultFOFABaseURL
		}
		if c.Plugins.FOFA.Size == 0 {
			c.Plugins.FOFA.Size = DefaultFOFASize
		}
		if c.Plugins.FOFA.RefreshInterval.Duration == 0 {
			c.Plugins.FOFA.RefreshInterval.Duration = 6 * time.Hour
		}
		if len(c.Plugins.FOFA.Queries) == 0 {
			c.Plugins.FOFA.Queries = DefaultFOFAQueries()
		}
	}
	if c.Plugins.SingBox != nil && c.Plugins.SingBox.RefreshInterval.Duration == 0 {
		c.Plugins.SingBox.RefreshInterval.Duration = time.Hour
	}
}

func DefaultFOFAQueries() []FOFAQueryConfig {
	return []FOFAQueryConfig{
		{Name: "socks5-no-auth", Protocol: "socks5", Query: `protocol=="socks5" && banner="Method:No Authentication"`, Fields: DefaultFOFAFields},
		{Name: "http-proxy", Protocol: "http", Query: `banner="Proxy-Authenticate" || banner="Proxy Authentication Required" || banner="Proxy-Agent" || banner="Squid" || banner="tinyproxy" || banner="3proxy"`, Fields: DefaultFOFAFields},
	}
}

type CheckResult struct {
	Errors        []string
	Warnings      []string
	ActivePlugins []string
}

func (r CheckResult) OK() bool { return len(r.Errors) == 0 }

func (c *Config) Check() CheckResult {
	c.ApplyDefaults()
	var r CheckResult
	if err := validListen(c.Server.Listen); err != nil {
		r.Errors = append(r.Errors, "server.listen: "+err.Error())
	}
	if err := validListen(c.Admin.Listen); err != nil {
		r.Errors = append(r.Errors, "admin.listen: "+err.Error())
	} else if !isLoopbackListen(c.Admin.Listen) && c.Admin.Token == "" {
		r.Errors = append(r.Errors, "admin.token is required when admin.listen is not loopback")
	}
	if c.Auth.Enabled {
		if c.Auth.Username == "" || c.Auth.Password == "" {
			r.Errors = append(r.Errors, "auth username/password are required when auth.enabled=true")
		}
		if strings.Contains(c.Auth.Username, "-") {
			r.Errors = append(r.Errors, "auth.username credential part cannot contain '-'")
		}
	}
	if c.Scheduler.Policy != "random" && c.Scheduler.Policy != "round_robin" {
		r.Errors = append(r.Errors, "scheduler.policy must be random or round_robin")
	}
	if c.Session.DefaultTTL.Duration <= 0 || c.Session.MaxTTL.Duration <= 0 {
		r.Errors = append(r.Errors, "session TTLs must be positive")
	}
	if c.Session.DefaultTTL.Duration > c.Session.MaxTTL.Duration {
		r.Warnings = append(r.Warnings, "session.default_ttl exceeds max_ttl; requested session TTLs will be clamped")
	}
	if c.Validation.Timeout.Duration <= 0 {
		r.Errors = append(r.Errors, "validation.timeout must be positive")
	}
	if c.Validation.Strategy != ValidationStrategyHTTPStatus && c.Validation.Strategy != ValidationStrategyIPAPICountry {
		r.Errors = append(r.Errors, "validation.strategy must be http_status or ip_api_country")
	}
	if c.Validation.Concurrency <= 0 {
		r.Errors = append(r.Errors, "validation.concurrency must be positive")
	}
	if c.RuntimeFailure.MaxFailures <= 0 {
		r.Errors = append(r.Errors, "runtime_failure.max_failures must be positive")
	}
	if c.RuntimeFailure.RetryAttempts < 0 {
		r.Errors = append(r.Errors, "runtime_failure.retry_attempts must be zero or positive")
	}
	if strings.EqualFold(c.Logging.Level, "debug") {
		r.Warnings = append(r.Warnings, "debug logging may write secrets and full proxy/subscription material to disk")
	}
	if c.Plugins.FPL != nil {
		r.ActivePlugins = append(r.ActivePlugins, "fpl")
		if c.Plugins.FPL.URL == DefaultFPLURL {
			r.Warnings = append(r.Warnings, "FPL uses built-in default URL")
		}
	}
	if c.Plugins.FOFA != nil {
		if c.Plugins.FOFA.Key == "" {
			r.Warnings = append(r.Warnings, "FOFA config exists but key is empty; FOFA will not activate")
		} else {
			r.ActivePlugins = append(r.ActivePlugins, "fofa")
		}
	}
	if c.Plugins.SingBox != nil {
		if len(c.Plugins.SingBox.Sources) == 0 {
			r.Warnings = append(r.Warnings, "singbox config exists but sources is empty; singbox will not activate")
		} else {
			r.ActivePlugins = append(r.ActivePlugins, "singbox")
			for i, s := range c.Plugins.SingBox.Sources {
				if s.Name == "" {
					r.Errors = append(r.Errors, fmt.Sprintf("plugins.singbox.sources[%d].name is required", i))
				}
				if s.Type != "url" && s.Type != "file" && s.Type != "inline" {
					r.Errors = append(r.Errors, fmt.Sprintf("plugins.singbox.sources[%d].type must be url, file, or inline", i))
				}
				if s.Type == "url" && s.URL == "" {
					r.Errors = append(r.Errors, fmt.Sprintf("plugins.singbox.sources[%d].url is required", i))
				}
				if s.Type == "file" && s.Path == "" {
					r.Errors = append(r.Errors, fmt.Sprintf("plugins.singbox.sources[%d].path is required", i))
				}
			}
		}
	}
	if len(r.ActivePlugins) == 0 {
		r.Warnings = append(r.Warnings, "no active plugins; service can start degraded and proxy requests will fail fast if pool is empty")
	}
	return r
}

func validListen(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if port == "" {
		return errors.New("missing port")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 0 || portNum > 65535 {
		return fmt.Errorf("invalid port %q", port)
	}
	if host == "" {
		return nil
	}
	return nil
}

func isLoopbackListen(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
