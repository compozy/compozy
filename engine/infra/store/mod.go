package store

import (
	"github.com/compozy/compozy/engine/workflow"
)

func NewWorkflowRepository() (workflow.Repository, error) {
	repo, err := NewMemDBRepository()
	if err != nil {
		return nil, err
	}
	return repo, nil
}
