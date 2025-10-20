package uc

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
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
	projectID, prefix, limit, err := uc.validateListRequest(in)
	if err != nil {
		return nil, err
	}

	filterIDs, err := uc.collectWorkflowFilters(ctx, projectID, in.WorkflowID)
	if err != nil {
		return nil, err
	}

	items, err := uc.store.ListWithValues(ctx, projectID, resources.ResourceTask)
	if err != nil {
		return nil, err
	}

	filtered := uc.applyTaskFilters(items, prefix, filterIDs)
	window, nextValue, nextDir, prevValue, prevDir := resourceutil.ApplyCursorWindow(
		filtered,
		strings.TrimSpace(in.CursorValue),
		in.CursorDirection,
		limit,
	)

	payload, err := uc.buildListPayload(window)
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

// validateListRequest sanitizes the list input and returns core parameters.
func (uc *List) validateListRequest(in *ListInput) (string, string, int, error) {
	if in == nil {
		return "", "", 0, ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return "", "", 0, ErrProjectMissing
	}
	return projectID, strings.TrimSpace(in.Prefix), resourceutil.ClampLimit(in.Limit), nil
}

// collectWorkflowFilters retrieves workflow task IDs used to filter the result set.
func (uc *List) collectWorkflowFilters(
	ctx context.Context,
	projectID string,
	workflowID string,
) (map[string]struct{}, error) {
	trimmedWorkflowID := strings.TrimSpace(workflowID)
	if trimmedWorkflowID == "" {
		return nil, nil
	}
	ids, err := uc.workflowTasks(ctx, projectID, trimmedWorkflowID)
	if err != nil {
		return nil, err
	}
	filterIDs := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		filterIDs[id] = struct{}{}
	}
	return filterIDs, nil
}

// applyTaskFilters applies prefix and workflow filters to the raw item list.
func (uc *List) applyTaskFilters(
	items []resources.StoredItem,
	prefix string,
	filterIDs map[string]struct{},
) []resources.StoredItem {
	filtered := resourceutil.FilterStoredItems(items, prefix)
	if len(filterIDs) > 0 {
		filtered = filterTasksBySet(filtered, filterIDs)
	}
	return filtered
}

// buildListPayload decodes stored tasks into response payload entries.
func (uc *List) buildListPayload(window []resources.StoredItem) ([]map[string]any, error) {
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
	return payload, nil
}

func (uc *List) workflowTasks(ctx context.Context, project string, workflowID string) ([]string, error) {
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

// unified workflow decoding is provided by resourceutil.DecodeStoredWorkflow
