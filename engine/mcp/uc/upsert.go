package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/logger"
)

type UpsertInput struct {
	Project string
	ID      string
	Body    map[string]any
	IfMatch string
}

type UpsertOutput struct {
	Config  map[string]any
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
	log := logger.FromContext(ctx)
	if in == nil {
		return nil, ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return nil, ErrProjectMissing
	}
	mcpID := strings.TrimSpace(in.ID)
	if mcpID == "" {
		return nil, ErrIDMissing
	}
	cfg, err := decodeMCPBody(in.Body, mcpID)
	if err != nil {
		return nil, err
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceMCP, ID: cfg.ID}
	etag, created, err := uc.storeMCP(ctx, key, cfg, in.IfMatch)
	if err != nil {
		return nil, err
	}
	updatedBy := "api"
	if usr, ok := userctx.UserFromContext(ctx); ok && usr != nil {
		updatedBy = usr.ID.String()
	}
	if err := resources.WriteMeta(
		ctx, uc.store, projectID, resources.ResourceMCP, cfg.ID, "api", updatedBy,
	); err != nil {
		log.Error("failed to write mcp meta", "error", err, "mcp", cfg.ID)
		// Best-effort metadata write: do not fail the operation.
	}
	entry, err := core.AsMapDefault(cfg)
	if err != nil {
		return nil, fmt.Errorf("encode mcp config: %w", err)
	}
	return &UpsertOutput{Config: entry, ETag: etag, Created: created}, nil
}

func (uc *Upsert) storeMCP(
	ctx context.Context,
	key resources.ResourceKey,
	cfg *mcp.Config,
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
			return "", false, fmt.Errorf("upsert mcp: %w", err)
		}
		return etag, false, nil
	}
	// Read first to determine if the resource exists so we can
	// accurately report Created vs Updated in the output and audit meta.
	// Our store's Put does not indicate creation status.
	_, _, err := uc.store.Get(ctx, key)
	created := errors.Is(err, resources.ErrNotFound)
	if err != nil && !created {
		return "", false, fmt.Errorf("inspect mcp: %w", err)
	}
	etag, putErr := uc.store.Put(ctx, key, cfg)
	if putErr != nil {
		return "", false, fmt.Errorf("put mcp: %w", putErr)
	}
	return etag, created, nil
}
