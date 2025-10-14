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
	providers := make([]project.KnowledgeBaseProvider, 0, len(workflows))
	for _, wf := range workflows {
		if wf != nil {
			providers = append(providers, wf)
		}
	}
	refs := projectConfig.AggregatedKnowledgeBases(providers...)
	if len(refs) == 0 {
		return nil
	}
	upsert := uc.NewUpsert(store)
	projectID := strings.TrimSpace(projectConfig.Name)
	for i := range refs {
		ref := refs[i]
		id := strings.TrimSpace(ref.Base.ID)
		if id == "" {
			continue
		}
		key := resources.ResourceKey{
			Project: projectID,
			Type:    resources.ResourceKnowledgeBase,
			ID:      id,
		}
		if _, _, err := store.Get(ctx, key); err == nil {
			continue
		} else if !errors.Is(err, resources.ErrNotFound) {
			return fmt.Errorf("seed knowledge base %q: %w", id, err)
		}
		body, err := core.AsMapDefault(ref.Base)
		if err != nil {
			return fmt.Errorf("seed knowledge base %q: encode: %w", id, err)
		}
		if _, err := upsert.Execute(ctx, &uc.UpsertInput{Project: projectID, ID: id, Body: body}); err != nil {
			if !errors.Is(err, uc.ErrAlreadyExists) {
				return fmt.Errorf("seed knowledge base %q: %w", id, err)
			}
			continue
		}
		logger.FromContext(ctx).Debug("Seeded knowledge base definition", "kb_id", id, "origin", ref.Origin)
	}
	return nil
}
