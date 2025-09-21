package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
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
	Config  *workflow.Config
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
	if in == nil || strings.TrimSpace(in.ID) == "" {
		return nil, ErrInvalidInput
	}
	project := strings.TrimSpace(in.Project)
	if project == "" {
		return nil, ErrProjectMissing
	}
	cfg, err := decodeWorkflowBody(in.Body, in.ID)
	if err != nil {
		return nil, ErrInvalidInput
	}
	key := resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: cfg.ID}
	etag, created, err := uc.storeWorkflow(ctx, key, cfg, in.IfMatch)
	if err != nil {
		return nil, err
	}
	updatedBy := "api"
	if usr, ok := userctx.UserFromContext(ctx); ok && usr != nil {
		updatedBy = usr.ID.String()
	}
	metaErr := resources.WriteMeta(ctx, uc.store, project, resources.ResourceWorkflow, cfg.ID, "api", updatedBy)
	if metaErr != nil {
		log.Error("failed to write workflow meta", "error", metaErr, "workflow_id", cfg.ID)
		return nil, fmt.Errorf("write workflow meta: %w", metaErr)
	}
	return &UpsertOutput{Config: cfg, ETag: etag, Created: created}, nil
}

func (uc *Upsert) storeWorkflow(
	ctx context.Context,
	key resources.ResourceKey,
	cfg *workflow.Config,
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
			return "", false, fmt.Errorf("put workflow %s: %w", cfg.ID, err)
		}
		return etag, false, nil
	}
	_, _, err := uc.store.Get(ctx, key)
	created := errors.Is(err, resources.ErrNotFound)
	if err != nil && !created {
		return "", false, fmt.Errorf("inspect workflow %s: %w", cfg.ID, err)
	}
	etag, putErr := uc.store.Put(ctx, key, cfg)
	if putErr != nil {
		return "", false, fmt.Errorf("put workflow %s: %w", cfg.ID, putErr)
	}
	return etag, created, nil
}
