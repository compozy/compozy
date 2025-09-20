package uc

import (
	"context"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

type ListInput struct {
	Project string
	Type    resources.ResourceType
	Prefix  string
}

type ListOutput struct {
	Keys []string
}

type ListResources struct {
	store resources.ResourceStore
}

func NewListResources(store resources.ResourceStore) *ListResources {
	return &ListResources{store: store}
}

func (uc *ListResources) Execute(ctx context.Context, in *ListInput) (*ListOutput, error) {
	_ = config.FromContext(ctx)
	_ = logger.FromContext(ctx)
	keys, err := uc.store.List(ctx, in.Project, in.Type)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(keys))
	p := strings.TrimSpace(in.Prefix)
	for i := range keys {
		id := keys[i].ID
		if p == "" || strings.HasPrefix(id, p) {
			out = append(out, id)
		}
	}
	return &ListOutput{Keys: out}, nil
}
