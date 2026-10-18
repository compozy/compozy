package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) EventMetadata(
	ctx context.Context,
	id string,
	query store.EventQuery,
) (events []store.EventMetadata, err error) {
	target := strings.TrimSpace(id)
	for attempt := range 2 {
		recorder, cleanup, openErr := m.openQueryRecorder(ctx, target)
		if openErr != nil {
			return nil, openErr
		}
		reader, ok := recorder.(store.EventMetadataReader)
		if !ok {
			return nil, errors.Join(errors.New("session: recorder has no event metadata projection"), cleanup())
		}
		events, err = reader.QueryEventMetadata(ctx, query)
		cleanupErr := cleanup()
		if err == nil {
			if cleanupErr != nil {
				return nil, cleanupErr
			}
			return events, nil
		}
		if cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
		if errors.Is(err, store.ErrClosed) && attempt == 0 {
			if _, waitErr := m.waitForSessionFinalization(ctx, target); waitErr != nil {
				return nil, fmt.Errorf(
					"session: wait for finalization for %q after closed recorder: %w", target, waitErr,
				)
			}
			continue
		}
		return nil, fmt.Errorf("session: query event metadata for %q: %w", target, err)
	}
	return nil, fmt.Errorf("session: query event metadata for %q: %w", target, err)
}
