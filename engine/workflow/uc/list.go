package uc

import (
	"context"
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
)

type CursorDirection string

const (
	CursorDirectionNone   CursorDirection = ""
	CursorDirectionAfter  CursorDirection = "after"
	CursorDirectionBefore CursorDirection = "before"
)

type ListInput struct {
	Project         string
	Prefix          string
	CursorValue     string
	CursorDirection CursorDirection
	Limit           int
}

type ListItem struct {
	Config *workflow.Config
	ETag   resources.ETag
}

type ListOutput struct {
	Items               []ListItem
	NextCursorValue     string
	NextCursorDirection CursorDirection
	PrevCursorValue     string
	PrevCursorDirection CursorDirection
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
	project := strings.TrimSpace(in.Project)
	if project == "" {
		return nil, ErrProjectMissing
	}
	limit := clampLimit(in.Limit)
	items, err := uc.store.ListWithValues(ctx, project, resources.ResourceWorkflow)
	if err != nil {
		return nil, err
	}
	filtered := filterWorkflows(items, strings.TrimSpace(in.Prefix))
	total := len(filtered)
	start, end := computeWindow(filtered, strings.TrimSpace(in.CursorValue), in.CursorDirection, limit)
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	slice := filtered[start:end]
	list := make([]ListItem, 0, len(slice))
	for i := range slice {
		cfg, decErr := decodeStoredWorkflow(slice[i].Value, slice[i].Key.ID)
		if decErr != nil {
			return nil, decErr
		}
		list = append(list, ListItem{Config: cfg, ETag: slice[i].ETag})
	}
	var nextCursorValue, prevCursorValue string
	var nextCursorDirection, prevCursorDirection CursorDirection
	if len(slice) > 0 && end < total {
		nextCursorValue = slice[len(slice)-1].Key.ID
		nextCursorDirection = CursorDirectionAfter
	}
	if len(slice) > 0 && start > 0 {
		prevCursorValue = slice[0].Key.ID
		prevCursorDirection = CursorDirectionBefore
	}
	return &ListOutput{
		Items:               list,
		NextCursorValue:     nextCursorValue,
		NextCursorDirection: nextCursorDirection,
		PrevCursorValue:     prevCursorValue,
		PrevCursorDirection: prevCursorDirection,
		Total:               total,
	}, nil
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func filterWorkflows(items []resources.StoredItem, prefix string) []resources.StoredItem {
	if prefix == "" {
		return sortWorkflows(items)
	}
	out := make([]resources.StoredItem, 0, len(items))
	for i := range items {
		if strings.HasPrefix(items[i].Key.ID, prefix) {
			out = append(out, items[i])
		}
	}
	return sortWorkflows(out)
}

func sortWorkflows(items []resources.StoredItem) []resources.StoredItem {
	if len(items) <= 1 {
		return items
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key.ID < items[j].Key.ID })
	return items
}

func computeWindow(items []resources.StoredItem, cursorValue string, direction CursorDirection, limit int) (int, int) {
	total := len(items)
	switch direction {
	case CursorDirectionAfter:
		start := searchAfter(items, cursorValue)
		end := start + limit
		if end > total {
			end = total
		}
		return start, end
	case CursorDirectionBefore:
		end := searchBefore(items, cursorValue)
		if end < 0 {
			end = 0
		}
		start := end - limit
		if start < 0 {
			start = 0
		}
		return start, end
	default:
		end := limit
		if end > total {
			end = total
		}
		return 0, end
	}
}

func searchAfter(items []resources.StoredItem, value string) int {
	if value == "" {
		return 0
	}
	return sort.Search(len(items), func(i int) bool {
		return items[i].Key.ID > value
	})
}

func searchBefore(items []resources.StoredItem, value string) int {
	if value == "" {
		return len(items)
	}
	idx := sort.Search(len(items), func(i int) bool {
		return items[i].Key.ID >= value
	})
	return idx
}
