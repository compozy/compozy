package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ store.NetworkAvailabilityStore = (*NetworkRepo)(nil)

// GetNetworkAvailability returns the persisted network admission state.
func (g *NetworkRepo) GetNetworkAvailability(ctx context.Context) (store.NetworkAvailability, error) {
	if err := g.checkReady(ctx, "get network availability"); err != nil {
		return store.NetworkAvailability{}, err
	}
	return getNetworkAvailabilityWithExecutor(ctx, g.db)
}

func getNetworkAvailabilityWithExecutor(
	ctx context.Context,
	exec networkSQLExecutor,
) (store.NetworkAvailability, error) {
	row, err := sqlcgen.New(exec).GetNetworkAvailability(ctx)
	if err != nil {
		return store.NetworkAvailability{}, fmt.Errorf("store: get network availability: %w", err)
	}
	return networkAvailabilityFromGenerated(row.Enabled, row.Epoch, row.UpdatedAt, row.UpdatedBy)
}

// SetNetworkAvailability changes availability inside the same immediate-write domain used by admission.
func (g *NetworkRepo) SetNetworkAvailability(
	ctx context.Context,
	enabled bool,
	updatedBy string,
) (availability store.NetworkAvailability, err error) {
	if err := g.checkReady(ctx, "set network availability"); err != nil {
		return store.NetworkAvailability{}, err
	}
	actor := strings.TrimSpace(updatedBy)
	if actor == "" {
		return store.NetworkAvailability{}, fmt.Errorf("store: network availability updated_by is required")
	}
	updatedAt := g.now().UTC()
	err = g.withNetworkImmediateTransaction(ctx, "set network availability", func(exec networkSQLExecutor) error {
		previous, readErr := getNetworkAvailabilityWithExecutor(ctx, exec)
		if readErr != nil {
			return readErr
		}
		row, updateErr := sqlcgen.New(exec).UpdateNetworkAvailability(
			ctx,
			sqlcgen.UpdateNetworkAvailabilityParams{
				Enabled:   networkAvailabilityBoolInt64(enabled),
				UpdatedAt: store.FormatTimestamp(updatedAt),
				UpdatedBy: actor,
			},
		)
		if updateErr != nil {
			return fmt.Errorf("store: update network availability: %w", updateErr)
		}
		availability, updateErr = networkAvailabilityFromGenerated(
			row.Enabled,
			row.Epoch,
			row.UpdatedAt,
			row.UpdatedBy,
		)
		if updateErr != nil {
			return updateErr
		}
		if previous.Enabled == availability.Enabled {
			return nil
		}
		return appendNetworkAvailabilityEventSummary(ctx, exec, previous, availability)
	})
	if err != nil {
		return store.NetworkAvailability{}, err
	}
	return availability, nil
}

func networkAvailabilityFromGenerated(
	enabled int64,
	epoch int64,
	updatedAt string,
	updatedBy string,
) (store.NetworkAvailability, error) {
	parsedUpdatedAt, err := store.ParseTimestamp(updatedAt)
	if err != nil {
		return store.NetworkAvailability{}, fmt.Errorf("store: parse network availability updated_at: %w", err)
	}
	if enabled != 0 && enabled != 1 {
		return store.NetworkAvailability{}, fmt.Errorf(
			"store: network availability enabled must be 0 or 1: %d",
			enabled,
		)
	}
	actor := strings.TrimSpace(updatedBy)
	if actor == "" {
		return store.NetworkAvailability{}, fmt.Errorf(
			"store: network availability updated_by is required",
		)
	}
	return store.NetworkAvailability{
		Enabled:   enabled == 1,
		Epoch:     epoch,
		UpdatedAt: parsedUpdatedAt,
		UpdatedBy: actor,
	}, nil
}

func networkAvailabilityBoolInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
