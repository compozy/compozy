package repo

import (
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/postgres"
	sqli "github.com/compozy/compozy/engine/infra/sqlite"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Provider returns repositories backed by the selected driver. For now, Task
// and Workflow repos are Postgres-backed; Auth can switch by mode.
type Provider struct {
	mode string
	pg   *pgxpool.Pool
	sq   *sqli.Store
}

// NewProvider constructs a Provider. Pass whichever driver is initialized for the
// current mode. Drivers not used can be nil.
func NewProvider(mode string, pg *pgxpool.Pool, sq *sqli.Store) *Provider {
	return &Provider{mode: mode, pg: pg, sq: sq}
}

// NewAuthRepo returns an auth repository based on mode: standalone uses SQLite
// when available; otherwise Postgres.
func (p *Provider) NewAuthRepo() uc.Repository {
	if p.mode == "standalone" && p.sq != nil {
		return sqli.NewAuthRepo(p.sq.DB())
	}
	return postgres.NewAuthRepo(p.pg)
}

// NewTaskRepo returns a task repository.
func (p *Provider) NewTaskRepo() task.Repository { return postgres.NewTaskRepo(p.pg) }

// NewWorkflowRepo returns a workflow repository.
func (p *Provider) NewWorkflowRepo() workflow.Repository { return postgres.NewWorkflowRepo(p.pg) }
