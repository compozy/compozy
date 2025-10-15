package repo

import (
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Provider returns repositories backed by Postgres only.
type Provider struct {
	pg *pgxpool.Pool
}

// NewProvider constructs a Postgres-only Provider.
func NewProvider(pg *pgxpool.Pool) *Provider {
	return &Provider{pg: pg}
}

// NewAuthRepo returns an auth repository backed by Postgres.
func (p *Provider) NewAuthRepo() uc.Repository {
	return postgres.NewAuthRepo(p.pg)
}

// NewTaskRepo returns a task repository backed by Postgres.
func (p *Provider) NewTaskRepo() task.Repository {
	return postgres.NewTaskRepo(p.pg)
}

// NewWorkflowRepo returns a workflow repository backed by Postgres.
func (p *Provider) NewWorkflowRepo() workflow.Repository {
	return postgres.NewWorkflowRepo(p.pg)
}

// NewUsageRepo returns an LLM usage repository backed by Postgres.
// Callers use this to persist execution token summaries.
func (p *Provider) NewUsageRepo() usage.Repository {
	return postgres.NewUsageRepo(p.pg)
}
