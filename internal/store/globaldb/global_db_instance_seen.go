package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ agentinstance.SeenStore = (*InstanceRepo)(nil)

func (r *InstanceRepo) LastSeen(ctx context.Context, instance, operator string) (*time.Time, error) {
	if err := r.checkReady(ctx, "read instance seen"); err != nil {
		return nil, err
	}
	if instance == "" || operator == "" {
		return nil, errors.New("instance and operator are required")
	}
	raw, err := r.queries.GetAgentInstanceSeen(ctx, sqlcgen.GetAgentInstanceSeenParams{InstanceID: instance, OperatorID: operator})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, err
	}
	return &at, nil
}

func (r *InstanceRepo) MarkSeen(ctx context.Context, instance, operator string, at time.Time) (time.Time, error) {
	if err := r.checkReady(ctx, "mark instance seen"); err != nil {
		return time.Time{}, err
	}
	if instance == "" || operator == "" || at.IsZero() {
		return time.Time{}, errors.New("instance operator and timestamp are required")
	}
	raw, err := r.queries.MarkAgentInstanceSeen(ctx, sqlcgen.MarkAgentInstanceSeenParams{InstanceID: instance, OperatorID: operator, LastSeenAt: store.FormatTimestamp(at)})
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, raw)
}
