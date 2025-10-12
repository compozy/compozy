package uc

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resourceutil"
)

type ListInput struct {
	Project         string
	Prefix          string
	CursorValue     string
	CursorDirection resourceutil.CursorDirection
	Limit           int
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
	keys, err := uc.store.List(ctx, projectID, resources.ResourceKnowledgeBase)
	if err != nil {
		return nil, fmt.Errorf("list knowledge base keys: %w", err)
	}
	prefix := strings.TrimSpace(in.Prefix)
	ids := filterKnowledgeBaseIDs(keys, prefix)
	sort.Strings(ids)
	windowIDs, nextValue, nextDir, prevValue, prevDir := applyCursorWindowIDs(
		ids,
		strings.TrimSpace(in.CursorValue),
		in.CursorDirection,
		limit,
	)
	payload, err := buildKnowledgeBasePayload(ctx, uc.store, projectID, windowIDs)
	if err != nil {
		return nil, err
	}
	return &ListOutput{
		Items:               payload,
		NextCursorValue:     nextValue,
		NextCursorDirection: nextDir,
		PrevCursorValue:     prevValue,
		PrevCursorDirection: prevDir,
		Total:               len(ids),
	}, nil
}

func filterKnowledgeBaseIDs(keys []resources.ResourceKey, prefix string) []string {
	ids := make([]string, 0, len(keys))
	for i := range keys {
		id := strings.TrimSpace(keys[i].ID)
		if id == "" {
			continue
		}
		if prefix == "" || strings.HasPrefix(id, prefix) {
			ids = append(ids, id)
		}
	}
	return ids
}

func applyCursorWindowIDs(
	ids []string,
	cursorValue string,
	cursorDirection resourceutil.CursorDirection,
	limit int,
) ([]string, string, resourceutil.CursorDirection, string, resourceutil.CursorDirection) {
	return resourceutil.ApplyCursorWindowIDs(ids, cursorValue, cursorDirection, limit)
}

func buildKnowledgeBasePayload(
	ctx context.Context,
	store resources.ResourceStore,
	projectID string,
	windowIDs []string,
) ([]map[string]any, error) {
	payload := make([]map[string]any, 0, len(windowIDs))
	for _, id := range windowIDs {
		key := resources.ResourceKey{Project: projectID, Type: resources.ResourceKnowledgeBase, ID: id}
		val, etag, err := store.Get(ctx, key)
		if err != nil {
			if errors.Is(err, resources.ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("load knowledge base %q: %w", id, err)
		}
		cfg, err := decodeStoredKnowledgeBase(val, id)
		if err != nil {
			return nil, err
		}
		entry, err := core.AsMapDefault(cfg)
		if err != nil {
			return nil, err
		}
		entry["_etag"] = string(etag)
		payload = append(payload, entry)
	}
	return payload, nil
}
