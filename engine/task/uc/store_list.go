package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resourceutil"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
)

type ListInput struct {
	Project         string
	Prefix          string
	CursorValue     string
	CursorDirection resourceutil.CursorDirection
	Limit           int
	WorkflowID      string
}

type ListOutput struct {
	Items               []map[string]any
	NextCursorValue     string
	NextCursorDirection resourceutil.CursorDirection
	PrevCursorValue     string
	PrevCursorDirection resourceutil.CursorDirection
	Total               int
}

type List struct {
	store resources.ResourceStore
}

func NewList(store resources.ResourceStore) *List {
	return &List{store: store}
}

func (uc *List) Execute(ctx context.Context, in *ListInput) (*ListOutput, error) {
	_ = config.FromContext(ctx)
	if in == nil {
		return nil, ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return nil, ErrProjectMissing
	}
	limit := resourceutil.ClampLimit(in.Limit)
	filterIDs := map[string]struct{}{}
	if workflowID := strings.TrimSpace(in.WorkflowID); workflowID != "" {
		ids, err := uc.workflowTasks(ctx, projectID, workflowID)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			filterIDs[id] = struct{}{}
		}
	}
	items, err := uc.store.ListWithValues(ctx, projectID, resources.ResourceTask)
	if err != nil {
		return nil, err
	}
	filtered := resourceutil.FilterStoredItems(items, strings.TrimSpace(in.Prefix))
	if len(filterIDs) > 0 {
		filtered = filterTasksBySet(filtered, filterIDs)
	}
	window, nextValue, nextDir, prevValue, prevDir := resourceutil.ApplyCursorWindow(
		filtered,
		strings.TrimSpace(in.CursorValue),
		in.CursorDirection,
		limit,
	)
	payload := make([]map[string]any, 0, len(window))
	for i := range window {
		cfg, err := decodeStoredTask(window[i].Value, window[i].Key.ID)
		if err != nil {
			return nil, err
		}
		entry, err := cfg.AsMap()
		if err != nil {
			return nil, err
		}
		entry["_etag"] = string(window[i].ETag)
		payload = append(payload, entry)
	}
	return &ListOutput{
		Items:               payload,
		NextCursorValue:     nextValue,
		NextCursorDirection: nextDir,
		PrevCursorValue:     prevValue,
		PrevCursorDirection: prevDir,
		Total:               len(filtered),
	}, nil
}

func (uc *List) workflowTasks(ctx context.Context, project string, workflowID string) ([]string, error) {
	key := resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: workflowID}
	value, _, err := uc.store.Get(ctx, key)
	if err != nil {
		if err == resources.ErrNotFound {
			return nil, ErrWorkflowNotFound
		}
		return nil, err
	}
	wf, err := decodeWorkflow(value, workflowID)
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for i := range wf.Tasks {
		id := strings.TrimSpace(wf.Tasks[i].ID)
		if id != "" {
			set[id] = struct{}{}
		}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids, nil
}

func filterTasksBySet(items []resources.StoredItem, allow map[string]struct{}) []resources.StoredItem {
	if len(allow) == 0 {
		return items
	}
	out := make([]resources.StoredItem, 0, len(items))
	for i := range items {
		if _, ok := allow[items[i].Key.ID]; ok {
			out = append(out, items[i])
		}
	}
	return out
}

func decodeWorkflow(value any, id string) (*workflow.Config, error) {
	switch v := value.(type) {
	case *workflow.Config:
		if strings.TrimSpace(v.ID) == "" {
			v.ID = id
		}
		return v, nil
	case workflow.Config:
		clone := v
		if strings.TrimSpace(clone.ID) == "" {
			clone.ID = id
		}
		return &clone, nil
	case map[string]any:
		cfg := &workflow.Config{}
		if err := cfg.FromMap(v); err != nil {
			return nil, fmt.Errorf("decode workflow: %w", err)
		}
		if strings.TrimSpace(cfg.ID) == "" {
			cfg.ID = id
		}
		return cfg, nil
	default:
		return nil, fmt.Errorf("decode workflow: unsupported type %T", value)
	}
}
