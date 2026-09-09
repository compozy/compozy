package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/memory"
	extractorpkg "github.com/compozy/compozy/internal/memory/extractor"
)

// daemonMemoryExtractorEvents routes delayed telemetry by the source session's owner.
type daemonMemoryExtractorEvents struct {
	stores         memory.RecallStoreResolver
	sessionProfile func(context.Context, string) (string, error)
}

func (s *daemonMemoryExtractorEvents) RecordExtractorEvent(ctx context.Context, event extractorpkg.Event) error {
	profileID := event.Turn.ProfileID
	if profileID == "" {
		var err error
		profileID, err = s.sessionProfile(ctx, event.Normalize(nil).SessionID)
		if err != nil {
			return err
		}
	}
	if profileID == "" {
		return errors.New("daemon: extractor event profile is required")
	}
	store, err := s.stores(ctx, profileID)
	if err != nil {
		return err
	}
	return store.RecordExtractorEvent(ctx, event)
}
