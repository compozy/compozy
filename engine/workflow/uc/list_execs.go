package uc

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

type ListExecutions struct {
	repo workflow.Repository
}

func NewListExecutionsUC(repo workflow.Repository) *ListExecutions {
	return &ListExecutions{
		repo: repo,
	}
}

func (uc *ListExecutions) Execute(ctx context.Context) ([]core.ExecutionMap, error) {
	return uc.repo.ListExecutionsMap(ctx)
}
