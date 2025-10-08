package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resourceutil"
	"github.com/compozy/compozy/pkg/logger"
)

type DeleteInput struct {
	Project string
	ID      string
}

type Delete struct {
	store resources.ResourceStore
}

func NewDelete(store resources.ResourceStore) *Delete {
	return &Delete{store: store}
}

func (uc *Delete) Execute(ctx context.Context, in *DeleteInput) error {
	if in == nil {
		return ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return ErrProjectMissing
	}
	kbID := strings.TrimSpace(in.ID)
	if kbID == "" {
		return ErrIDMissing
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceKnowledgeBase, ID: kbID}
	triple, err := loadKnowledgeTriple(ctx, uc.store, projectID, kbID)
	if err != nil {
		return err
	}
	kb, emb, vec, err := normalizeKnowledgeTriple(ctx, triple)
	if err != nil {
		return err
	}
	_ = emb
	conflicts, err := uc.collectConflicts(ctx, projectID, kbID)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return resourceutil.ConflictError{Details: conflicts}
	}
	storeCfg, err := toVectorStoreConfig(projectID, vec)
	if err != nil {
		return err
	}
	vectorStore, err := vectordb.New(ctx, storeCfg)
	if err != nil {
		return fmt.Errorf("init vector store: %w", err)
	}
	log := logger.FromContext(ctx)
	defer func() {
		if cerr := vectorStore.Close(ctx); cerr != nil {
			log.Warn("failed to close vector store", "kb_id", kb.ID, "error", cerr)
		}
	}()
	if err := uc.store.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete knowledge base %q: %w", kbID, err)
	}
	filter := vectordb.Filter{Metadata: map[string]string{"knowledge_base_id": kb.ID}}
	if err := vectorStore.Delete(ctx, filter); err != nil {
		if restoreErr := uc.restoreKnowledgeBase(ctx, key, kb); restoreErr != nil {
			if !errors.Is(restoreErr, resources.ErrETagMismatch) {
				log.Error(
					"failed to restore knowledge base after vector cleanup failure",
					"kb_id",
					kb.ID,
					"error",
					restoreErr,
				)
			}
		}
		return fmt.Errorf("cleanup knowledge vectors: %w", err)
	}
	return nil
}

func (uc *Delete) collectConflicts(
	ctx context.Context,
	projectID string,
	kbID string,
) ([]resourceutil.ReferenceDetail, error) {
	conflicts := make([]resourceutil.ReferenceDetail, 0, 4)
	projectRef, err := resourceutil.ProjectReferencesKnowledgeBase(ctx, uc.store, projectID, kbID)
	if err != nil {
		return nil, err
	}
	if projectRef {
		conflicts = append(conflicts, resourceutil.ReferenceDetail{Resource: "project", IDs: []string{projectID}})
	}
	if wfRefs, err := resourceutil.WorkflowsReferencingKnowledgeBase(ctx, uc.store, projectID, kbID); err != nil {
		return nil, err
	} else if len(wfRefs) > 0 {
		conflicts = append(conflicts, resourceutil.ReferenceDetail{Resource: "workflows", IDs: wfRefs})
	}
	if agRefs, err := resourceutil.AgentsReferencingKnowledgeBase(ctx, uc.store, projectID, kbID); err != nil {
		return nil, err
	} else if len(agRefs) > 0 {
		conflicts = append(conflicts, resourceutil.ReferenceDetail{Resource: "agents", IDs: agRefs})
	}
	if taskRefs, err := resourceutil.TasksReferencingKnowledgeBase(ctx, uc.store, projectID, kbID); err != nil {
		return nil, err
	} else if len(taskRefs) > 0 {
		conflicts = append(conflicts, resourceutil.ReferenceDetail{Resource: "tasks", IDs: taskRefs})
	}
	return conflicts, nil
}

func (uc *Delete) restoreKnowledgeBase(
	ctx context.Context,
	key resources.ResourceKey,
	cfg *knowledge.BaseConfig,
) error {
	if cfg == nil {
		return nil
	}
	_, err := uc.store.PutIfMatch(ctx, key, cfg, "")
	return err
}
