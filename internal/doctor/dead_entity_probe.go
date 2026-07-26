package doctor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const deadEntityProbeIDPrefix = "runtime.dead_entities."

// DeadEntitySource is the read-only durable reliability surface required by doctor.
type DeadEntitySource interface {
	List(ctx context.Context, workspaceID string) ([]store.DeadEntity, error)
}

// DeadEntityWorkspaceSource lists the workspace authorities doctor must inspect.
type DeadEntityWorkspaceSource interface {
	List(context.Context) ([]workspacepkg.Workspace, error)
}

// DeadEntityProbe projects one runtime family without mutating its recovery state.
type DeadEntityProbe struct {
	Source     DeadEntitySource
	Workspaces DeadEntityWorkspaceSource
	Kind       store.DeadEntityKind
}

var _ Probe = (*DeadEntityProbe)(nil)

func (p *DeadEntityProbe) ID() string {
	spec, ok := deadEntityDiagnosticSpec(p.Kind)
	if !ok {
		return deadEntityProbeIDPrefix + "invalid"
	}
	return deadEntityProbeIDPrefix + spec.id
}

func (p *DeadEntityProbe) Category() string {
	spec, ok := deadEntityDiagnosticSpec(p.Kind)
	if !ok {
		return ""
	}
	return spec.category
}

func (p *DeadEntityProbe) Run(
	ctx context.Context,
	_ *ProbeEnv,
) ([]contract.DiagnosticItem, error) {
	if p == nil || p.Source == nil {
		return nil, errors.New("doctor: dead-entity source is required")
	}
	if p.Workspaces == nil {
		return nil, errors.New("doctor: dead-entity workspace source is required")
	}
	spec, ok := deadEntityDiagnosticSpec(p.Kind)
	if !ok {
		return nil, fmt.Errorf("doctor: unsupported dead-entity kind %q", p.Kind)
	}
	workspaces, err := p.Workspaces.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("doctor: list dead-entity workspaces: %w", err)
	}
	sort.Slice(workspaces, func(left int, right int) bool {
		return workspaces[left].ID < workspaces[right].ID
	})

	items := make([]contract.DiagnosticItem, 0)
	for _, workspace := range workspaces {
		entities, err := p.Source.List(ctx, workspace.ID)
		if err != nil {
			return nil, fmt.Errorf("doctor: list dead entities for workspace %q: %w", workspace.ID, err)
		}
		for _, entity := range entities {
			if entity.Kind != p.Kind {
				continue
			}
			items = append(items, deadEntityDiagnosticItem(spec, workspace, entity))
		}
	}
	return items, nil
}

type deadEntitySpec struct {
	id       string
	category string
	code     string
	title    string
}

func deadEntityDiagnosticSpec(kind store.DeadEntityKind) (deadEntitySpec, bool) {
	switch kind {
	case store.DeadEntityKindMCPSidecar:
		return deadEntitySpec{
			id:       "mcp",
			category: contract.CategoryMCP,
			code:     contract.CodeMCPServerUnavailable,
			title:    "MCP server is marked dead",
		}, true
	case store.DeadEntityKindBridge:
		return deadEntitySpec{
			id:       "bridge",
			category: contract.CategoryBridge,
			code:     contract.CodeBridgeHealthUnavailable,
			title:    "Bridge runtime is marked dead",
		}, true
	case store.DeadEntityKindExtension:
		return deadEntitySpec{
			id:       "extension",
			category: contract.CategoryExtension,
			code:     contract.CodeExtensionRuntimeUnavailable,
			title:    "Extension runtime is marked dead",
		}, true
	default:
		return deadEntitySpec{}, false
	}
}

func deadEntityDiagnosticItem(
	spec deadEntitySpec,
	workspace workspacepkg.Workspace,
	entity store.DeadEntity,
) contract.DiagnosticItem {
	markedAt := ""
	if !entity.MarkedAt.IsZero() {
		markedAt = entity.MarkedAt.UTC().Format(time.RFC3339Nano)
	}
	return diagnostics.NewItem(
		"doctor.dead_entity."+spec.id+"."+diagnosticIDPart(workspace.ID)+"."+diagnosticIDPart(entity.EntityID),
		spec.code,
		spec.category,
		spec.title,
		"AGH is suppressing ordinary attempts and will admit a bounded recovery probe when the retry window opens.",
		contract.SeverityError,
		contract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			"workspace_id":   strings.TrimSpace(workspace.ID),
			"workspace_name": strings.TrimSpace(workspace.Name),
			"kind":           entity.Kind,
			"entity_id":      strings.TrimSpace(entity.EntityID),
			"reason":         strings.TrimSpace(entity.Reason),
			"marked_at":      markedAt,
		}),
	)
}
