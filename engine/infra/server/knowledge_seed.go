package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge/uc"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

func seedKnowledgeDefinitions(
	ctx context.Context,
	store resources.ResourceStore,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) error {
	if store == nil || projectConfig == nil {
		return nil
	}
	providers := collectKnowledgeProviders(workflows)
	refs := projectConfig.AggregatedKnowledgeBases(providers...)
	if len(refs) == 0 {
		return nil
	}
	return upsertKnowledgeBases(ctx, store, projectConfig.Name, refs)
}

// collectKnowledgeProviders collects non-nil workflows as knowledge providers.
func collectKnowledgeProviders(workflows []*workflow.Config) []project.KnowledgeBaseProvider {
	providers := make([]project.KnowledgeBaseProvider, 0, len(workflows))
	for _, wf := range workflows {
		if wf != nil {
			providers = append(providers, wf)
		}
	}
	return providers
}

// upsertKnowledgeBases seeds missing knowledge bases into the resource store.
func upsertKnowledgeBases(
	ctx context.Context,
	store resources.ResourceStore,
	projectName string,
	refs []project.KnowledgeBaseRef,
) error {
	upsert := uc.NewUpsert(store)
	projectID := strings.TrimSpace(projectName)
	if projectID == "" {
		return fmt.Errorf("project name cannot be empty or whitespace-only")
	}
	var errs error
	for i := range refs {
		if err := ensureKnowledgeBase(ctx, store, upsert, projectID, &refs[i]); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

// ensureKnowledgeBase creates a knowledge base definition when missing.
func ensureKnowledgeBase(
	ctx context.Context,
	store resources.ResourceStore,
	upsert *uc.Upsert,
	projectID string,
	ref *project.KnowledgeBaseRef,
) error {
	if ref == nil {
		logger.FromContext(ctx).Debug("Skipping nil knowledge base ref")
		return nil
	}
	id := strings.TrimSpace(ref.Base.ID)
	if id == "" {
		logger.FromContext(ctx).Debug("Skipping knowledge base with empty ID", "origin", ref.Origin)
		return nil
	}
	key := resources.ResourceKey{
		Project: projectID,
		Type:    resources.ResourceKnowledgeBase,
		ID:      id,
	}
	if _, _, err := store.Get(ctx, key); err == nil {
		return nil
	} else if !errors.Is(err, resources.ErrNotFound) {
		return fmt.Errorf("seed knowledge base %q: %w", id, err)
	}
	body, err := core.AsMapDefault(ref.Base)
	if err != nil {
		return fmt.Errorf("seed knowledge base %q: encode: %w", id, err)
	}
	if _, err := upsert.Execute(ctx, &uc.UpsertInput{Project: projectID, ID: id, Body: body}); err != nil {
		if errors.Is(err, uc.ErrAlreadyExists) {
			return nil
		}
		return fmt.Errorf("seed knowledge base %q: %w", id, err)
	}
	logger.FromContext(ctx).Debug("Seeded knowledge base definition", "kb_id", id, "origin", ref.Origin)
	return nil
}
