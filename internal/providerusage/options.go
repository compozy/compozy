package providerusage

import (
	"context"
	"github.com/jonboulle/clockwork"
	"log/slog"
	"time"
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

type Manager struct{ options Options }

func NewService(options Options) *Manager {
	if options.Clock == nil {
		options.Clock = clockwork.NewRealClock()
	}
	if options.RefreshInterval == 0 {
		options.RefreshInterval = 5 * time.Minute
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	return &Manager{options: options}
}

var _ Service = (*Manager)(nil)

func (s *Manager) Envelope(_ context.Context, profile string) (Envelope, error) {
	return Envelope{ProfileID: profile, Enabled: !s.options.Disabled, GeneratedAt: s.options.Clock.Now(), Targets: []Snapshot{}}, nil
}
func (s *Manager) Refresh(ctx context.Context, req RefreshRequest) (Envelope, error) {
	return s.Envelope(ctx, req.ProfileID)
}
func (s *Manager) Subscribe(_ string) (<-chan Revision, func()) {
	c := make(chan Revision)
	return c, func() {}
}
