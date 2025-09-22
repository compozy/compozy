package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resourceutil"
	"github.com/compozy/compozy/engine/workflow"
)

type ListInput struct {
	Project         string
	Prefix          string
	CursorValue     string
	CursorDirection resourceutil.CursorDirection
	Limit           int
}

type ListItem struct {
	Config *workflow.Config
	ETag   resources.ETag
}

type ListOutput struct {
	Items               []ListItem
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
	project := strings.TrimSpace(in.Project)
	if project == "" {
		return nil, ErrProjectMissing
	}
	limit := resourceutil.ClampLimit(in.Limit)
	items, err := uc.store.ListWithValues(ctx, project, resources.ResourceWorkflow)
	if err != nil {
		return nil, fmt.Errorf("list workflows for project %q: %w", project, err)
	}
	filtered := resourceutil.FilterStoredItems(items, strings.TrimSpace(in.Prefix))
	slice, nextValue, nextDir, prevValue, prevDir := resourceutil.ApplyCursorWindow(
		filtered,
		strings.TrimSpace(in.CursorValue),
		in.CursorDirection,
		limit,
	)
	list := make([]ListItem, 0, len(slice))
	for i := range slice {
		cfg, decErr := decodeStoredWorkflow(slice[i].Value, slice[i].Key.ID)
		if decErr != nil {
			return nil, fmt.Errorf("decode workflow %q: %w", slice[i].Key.ID, decErr)
		}
		list = append(list, ListItem{Config: cfg, ETag: slice[i].ETag})
	}
	return &ListOutput{
		Items:               list,
		NextCursorValue:     nextValue,
		NextCursorDirection: nextDir,
		PrevCursorValue:     prevValue,
		PrevCursorDirection: prevDir,
		Total:               len(filtered),
	}, nil
}
