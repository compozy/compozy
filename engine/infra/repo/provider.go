package repo

import (
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Provider exposes repositories required by the application, backed by a
// specific driver (PostgreSQL for now). It intentionally returns interfaces
// rather than driver-specific types.
type Provider struct {
	pool *pgxpool.Pool
}

func NewProvider(pool *pgxpool.Pool) *Provider { return &Provider{pool: pool} }

// NewAuthRepo returns an auth repository.
func (p *Provider) NewAuthRepo() uc.Repository { return postgres.NewAuthRepo(p.pool) }

// NewTaskRepo returns a task repository.
func (p *Provider) NewTaskRepo() task.Repository { return postgres.NewTaskRepo(p.pool) }

// NewWorkflowRepo returns a workflow repository.
func (p *Provider) NewWorkflowRepo() workflow.Repository { return postgres.NewWorkflowRepo(p.pool) }
