package project

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	engineproject "github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// Builder constructs engine project configurations using a fluent API while
// accumulating validation errors until Build is invoked.
type Builder struct {
	config    *engineproject.Config
	errors    []error
	workflows []*workflow.Config
	agents    []*agent.Config
}

// New creates a project builder with the provided project name.
func New(name string) *Builder {
	return &Builder{
		config: &engineproject.Config{
			Name:   strings.TrimSpace(name),
			Models: make([]*core.ProviderConfig, 0),
		},
		errors:    make([]error, 0),
		workflows: make([]*workflow.Config, 0),
		agents:    make([]*agent.Config, 0),
	}
}

// WithVersion configures the semantic version associated with the project.
func (b *Builder) WithVersion(version string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(version)
	b.config.Version = trimmed
	return b
}

// WithDescription sets a human-readable description for the project.
func (b *Builder) WithDescription(desc string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(desc)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("description cannot be empty"))
		return b
	}
	b.config.Description = trimmed
	return b
}

// WithAuthor configures the author metadata for the project.
func (b *Builder) WithAuthor(name, email, org string) *Builder {
	if b == nil {
		return nil
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		b.errors = append(b.errors, fmt.Errorf("author name cannot be empty"))
	}
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		b.errors = append(b.errors, fmt.Errorf("author email cannot be empty"))
	} else if _, err := mail.ParseAddress(trimmedEmail); err != nil {
		b.errors = append(b.errors, fmt.Errorf("author email must be valid: %w", err))
	}
	trimmedOrg := strings.TrimSpace(org)
	b.config.Author = core.Author{
		Name:         trimmedName,
		Email:        trimmedEmail,
		Organization: trimmedOrg,
	}
	return b
}

// AddModel registers a model provider configuration with the project.
func (b *Builder) AddModel(model *core.ProviderConfig) *Builder {
	if b == nil {
		return nil
	}
	if model == nil {
		b.errors = append(b.errors, fmt.Errorf("model cannot be nil"))
		return b
	}
	b.config.Models = append(b.config.Models, model)
	return b
}

// AddWorkflow registers a workflow configuration with the project builder.
func (b *Builder) AddWorkflow(wf *workflow.Config) *Builder {
	if b == nil {
		return nil
	}
	if wf == nil {
		b.errors = append(b.errors, fmt.Errorf("workflow cannot be nil"))
		return b
	}
	b.workflows = append(b.workflows, wf)
	return b
}

// AddAgent registers an agent configuration with the project builder.
func (b *Builder) AddAgent(agentCfg *agent.Config) *Builder {
	if b == nil {
		return nil
	}
	if agentCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("agent cannot be nil"))
		return b
	}
	b.agents = append(b.agents, agentCfg)
	return b
}

// Build validates the accumulated configuration and returns a project config.
func (b *Builder) Build(ctx context.Context) (*engineproject.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("project builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}

	log := logger.FromContext(ctx)
	log.Debug("building project configuration", "project", b.config.Name, "workflows", len(b.workflows))

	collected := make([]error, 0, len(b.errors)+4)
	collected = append(collected, b.errors...)

	if err := validate.ValidateRequired(ctx, "project name", b.config.Name); err != nil {
		collected = append(collected, err)
	} else if err := validate.ValidateID(ctx, b.config.Name); err != nil {
		collected = append(collected, fmt.Errorf("project name must be alphanumeric or hyphenated: %w", err))
	}

	if version := strings.TrimSpace(b.config.Version); version != "" {
		if _, err := semver.NewVersion(version); err != nil {
			collected = append(collected, fmt.Errorf("version must be valid semver: %w", err))
		}
	}

	if len(b.workflows) == 0 {
		collected = append(collected, fmt.Errorf("at least one workflow must be registered"))
	}

	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone project config: %w", err)
	}
	return cloned, nil
}
