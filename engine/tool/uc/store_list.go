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
	projectID, limit, err := validateListInput(in)
	if err != nil {
		return nil, err
	}
	workflowFilter, err := uc.resolveWorkflowFilter(ctx, projectID, strings.TrimSpace(in.WorkflowID))
	if err != nil {
		return nil, err
	}
	items, err := uc.store.ListWithValues(ctx, projectID, resources.ResourceTool)
	if err != nil {
		return nil, err
	}
	filtered := uc.applyToolFilters(items, strings.TrimSpace(in.Prefix), workflowFilter)
	window, nextValue, nextDir, prevValue, prevDir := resourceutil.ApplyCursorWindow(
		filtered,
		strings.TrimSpace(in.CursorValue),
		in.CursorDirection,
		limit,
	)
	payload, err := uc.buildToolPayload(window)
	if err != nil {
		return nil, err
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

// validateListInput normalizes list parameters and validates invariants
func validateListInput(in *ListInput) (string, int, error) {
	if in == nil {
		return "", 0, ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return "", 0, ErrProjectMissing
	}
	return projectID, resourceutil.ClampLimit(in.Limit), nil
}

// resolveWorkflowFilter loads workflow tools when filtering by workflow ID
func (uc *List) resolveWorkflowFilter(
	ctx context.Context,
	projectID string,
	workflowID string,
) (map[string]struct{}, error) {
	if workflowID == "" {
		return nil, nil
	}
	ids, err := uc.workflowTools(ctx, projectID, workflowID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}

// applyToolFilters applies prefix and workflow-based filters to the tool list
func (uc *List) applyToolFilters(
	items []resources.StoredItem,
	prefix string,
	workflowFilter map[string]struct{},
) []resources.StoredItem {
	filtered := resourceutil.FilterStoredItems(items, prefix)
	if len(workflowFilter) == 0 {
		return filtered
	}
	return filterToolsBySet(filtered, workflowFilter)
}

// buildToolPayload converts stored items into API payload objects
func (uc *List) buildToolPayload(window []resources.StoredItem) ([]map[string]any, error) {
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
	return payload, nil
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
