package compozy

import (
	"context"
	"errors"
	"fmt"

	appconfig "github.com/compozy/compozy/pkg/config"

	"github.com/compozy/compozy/engine/resources"
)

type modeCleanup func(context.Context) error

type modeRuntimeState struct {
	resourceStore resources.ResourceStore
	cleanups      []modeCleanup
}

func (s *modeRuntimeState) addCleanup(fn modeCleanup) {
	if s == nil || fn == nil {
		return
	}
	s.cleanups = append(s.cleanups, fn)
}

func (s *modeRuntimeState) cleanup(ctx context.Context) error {
	if s == nil {
		return nil
	}
	var errs []error
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		fn := s.cleanups[i]
		if fn == nil {
			continue
		}
		if err := fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	s.cleanups = nil
	return errors.Join(errs...)
}

func (s *modeRuntimeState) cleanupOnError(ctx context.Context) {
	_ = s.cleanup(ctx)
}

func (e *Engine) bootstrapMode(ctx context.Context, cfg *appconfig.Config) (*modeRuntimeState, error) {
	if e == nil {
		return nil, fmt.Errorf("engine is nil")
	}
	switch e.mode {
	case ModeStandalone:
		return e.bootstrapStandalone(ctx, cfg)
	case ModeDistributed:
		return e.bootstrapDistributed(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported engine mode %q", e.mode)
	}
}
