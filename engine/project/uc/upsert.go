package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

type UpsertInput struct {
	Project string
	Body    map[string]any
	IfMatch string
}

type UpsertOutput struct {
	Config  *project.Config
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
	cfg, err := decodeStoredProject(in.Body, projectID)
	if err != nil {
		return nil, ErrInvalidInput
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceProject, ID: projectID}
	etag, created, err := uc.storeProject(ctx, key, cfg, in.IfMatch)
	if err != nil {
		return nil, err
	}
	updatedBy := "api"
	if usr, ok := userctx.UserFromContext(ctx); ok && usr != nil {
		updatedBy = usr.ID.String()
	}
	if err := resources.WriteMeta(
		ctx,
		uc.store,
		projectID,
		resources.ResourceProject,
		projectID,
		"api",
		updatedBy,
	); err != nil {
		log.Error("failed to write project meta", "error", err, "project", projectID)
		return nil, fmt.Errorf("write project meta: %w", err)
	}
	return &UpsertOutput{Config: cfg, ETag: etag, Created: created}, nil
}

func (uc *Upsert) storeProject(
	ctx context.Context,
	key resources.ResourceKey,
	cfg *project.Config,
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
			return "", false, fmt.Errorf("upsert project: %w", err)
		}
		return etag, false, nil
	}
	_, _, err := uc.store.Get(ctx, key)
	created := errors.Is(err, resources.ErrNotFound)
	if err != nil && !created {
		return "", false, fmt.Errorf("inspect project: %w", err)
	}
	etag, putErr := uc.store.Put(ctx, key, cfg)
	if putErr != nil {
		return "", false, fmt.Errorf("put project: %w", putErr)
	}
	return etag, created, nil
}
