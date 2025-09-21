package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

type UpsertInput struct {
	Project string
	ID      string
	Body    map[string]any
	IfMatch string
}

type UpsertOutput struct {
	Task    map[string]any
	ETag    resources.ETag
	Created bool
}

type Upsert struct {
	store resources.ResourceStore
}

func NewUpsert(store resources.ResourceStore) *Upsert {
	return &Upsert{store: store}
}

func (uc *Upsert) Execute(ctx context.Context, in *UpsertInput) (*UpsertOutput, error) {
	_ = config.FromContext(ctx)
	log := logger.FromContext(ctx)
	if in == nil {
		return nil, ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return nil, ErrProjectMissing
	}
	taskID := strings.TrimSpace(in.ID)
	if taskID == "" {
		return nil, ErrIDMissing
	}
	cfg, err := decodeTaskBody(in.Body, taskID)
	if err != nil {
		return nil, err
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceTask, ID: taskID}
	etag, created, err := uc.storeTask(ctx, key, cfg, in.IfMatch)
	if err != nil {
		return nil, err
	}
	updatedBy := "api"
	if usr, ok := userctx.UserFromContext(ctx); ok && usr != nil {
		updatedBy = usr.ID.String()
	}
	if err := resources.WriteMeta(ctx, uc.store, projectID, resources.ResourceTask, taskID, "api", updatedBy); err != nil {
		log.Error("failed to write task meta", "error", err, "task", taskID)
		return nil, fmt.Errorf("write task meta: %w", err)
	}
	entry, err := cfg.AsMap()
	if err != nil {
		return nil, err
	}
	return &UpsertOutput{Task: entry, ETag: etag, Created: created}, nil
}

func (uc *Upsert) storeTask(
	ctx context.Context,
	key resources.ResourceKey,
	cfg *task.Config,
	ifMatch string,
) (resources.ETag, bool, error) {
	trimmed := strings.TrimSpace(ifMatch)
	if trimmed != "" {
		etag, err := uc.store.PutIfMatch(ctx, key, cfg, resources.ETag(trimmed))
		switch {
		case errors.Is(err, resources.ErrETagMismatch):
			return "", false, ErrETagMismatch
		case errors.Is(err, resources.ErrNotFound):
			return "", false, ErrStaleIfMatch
		case err != nil:
			return "", false, fmt.Errorf("upsert task: %w", err)
		}
		return etag, false, nil
	}
	_, _, err := uc.store.Get(ctx, key)
	created := errors.Is(err, resources.ErrNotFound)
	if err != nil && !created {
		return "", false, fmt.Errorf("inspect task: %w", err)
	}
	etag, putErr := uc.store.Put(ctx, key, cfg)
	if putErr != nil {
		return "", false, fmt.Errorf("put task: %w", putErr)
	}
	return etag, created, nil
}
