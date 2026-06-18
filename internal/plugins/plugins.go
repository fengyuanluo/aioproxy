package plugins

import (
	"context"
	"time"

	"github.com/aioproxy/aioproxy/internal/core"
)

type Plugin interface {
	Name() string
	Active() bool
	RefreshInterval() time.Duration
	Refresh(ctx context.Context) core.PluginResult
}
