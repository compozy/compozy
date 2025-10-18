package uc

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
	"github.com/compozy/compozy/engine/task"
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
		ids, err := uc.workflowTools(ctx, projectID, workflowID)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			filterIDs[id] = struct{}{}
		}
	}
	items, err := uc.store.ListWithValues(ctx, projectID, resources.ResourceTool)
	if err != nil {
		return nil, err
	}
	filtered := resourceutil.FilterStoredItems(items, strings.TrimSpace(in.Prefix))
	if len(filterIDs) > 0 {
		filtered = filterToolsBySet(filtered, filterIDs)
	}
	window, nextValue, nextDir, prevValue, prevDir := resourceutil.ApplyCursorWindow(
		filtered,
		strings.TrimSpace(in.CursorValue),
		in.CursorDirection,
		limit,
	)
	payload := make([]map[string]any, 0, len(window))
	for i := range window {
		cfg, err := decodeStoredTool(window[i].Value, window[i].Key.ID)
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

func (uc *List) workflowTools(ctx context.Context, project string, workflowID string) ([]string, error) {
	key := resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: workflowID}
	value, _, err := uc.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, err
	}
	wf, err := resourceutil.DecodeStoredWorkflow(value, workflowID)
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for i := range wf.Tools {
		id := strings.TrimSpace(wf.Tools[i].ID)
		if id != "" {
			set[id] = struct{}{}
		}
	}
	for i := range wf.Tasks {
		collectTaskTools(&wf.Tasks[i], set)
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids, nil
}

func collectTaskTools(cfg *task.Config, dst map[string]struct{}) {
	if cfg == nil {
		return
	}
	if cfg.Tool != nil {
		id := strings.TrimSpace(cfg.Tool.ID)
		if id != "" {
			dst[id] = struct{}{}
		}
	}
}

func filterToolsBySet(items []resources.StoredItem, allow map[string]struct{}) []resources.StoredItem {
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

// unified workflow decoding is provided by resourceutil.DecodeStoredWorkflow
