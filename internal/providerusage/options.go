package providerusage

import (
	"log/slog"
	"time"

	"github.com/jonboulle/clockwork"
)

type Registry map[string]Collector

type Metrics interface {
	Collect(provider string, status Status, duration time.Duration)
}

type Options struct {
	Resolver        TargetResolver
	Collectors      Registry
	Clock           clockwork.Clock
	RefreshInterval time.Duration
	Logger          *slog.Logger
	Metrics         Metrics
	ProviderOrder   []string
	// Disabled defaults to false; Start owns scheduling for the supplied profiles.
	Disabled bool
}
