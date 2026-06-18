package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/aioproxy/aioproxy/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

func New(cfg config.LoggingConfig) (*slog.Logger, io.Closer, error) {
	if cfg.File == "" {
		cfg.File = "./logs/aioproxy.log"
	}
	if err := os.MkdirAll(filepath.Dir(cfg.File), 0o755); err != nil {
		return nil, nil, err
	}
	lj := &lumberjack.Logger{
		Filename:   cfg.File,
		MaxSize:    int(max(1, cfg.Rotation.MaxSize.Bytes/(1000*1000))),
		MaxBackups: cfg.Rotation.MaxBackups,
		MaxAge:     cfg.Rotation.MaxAgeDays,
		Compress:   cfg.Rotation.Compress,
	}
	level := new(slog.LevelVar)
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn", "warning":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "json":
		h = slog.NewJSONHandler(lj, opts)
	case "text", "":
		h = slog.NewTextHandler(lj, opts)
	default:
		return nil, nil, fmt.Errorf("unsupported log format %q", cfg.Format)
	}
	return slog.New(h), lj, nil
}
