package heartbeat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/resources"
)

const (
	// ResourceKind is the canonical desired-state kind for package-owned HEARTBEAT.md content.
	ResourceKind resources.ResourceKind = "agent.heartbeat"

	resourceMaxBytes = 512 << 10
)

// ResourceSpec stores read-only packaged HEARTBEAT.md content for one resource-backed agent.
type ResourceSpec struct {
	AgentName       string `json:"agent_name"`
	AgentResourceID string `json:"agent_resource_id"`
	SourcePath      string `json:"source_path"`
	Body            string `json:"body"`
}

// NewResourceCodec builds the typed codec for agent.heartbeat records.
func NewResourceCodec() (resources.KindCodec[ResourceSpec], error) {
	return resources.NewJSONCodec(ResourceKind, resourceMaxBytes, validateResourceSpec)
}

func validateResourceSpec(
	ctx context.Context,
	scope resources.ResourceScope,
	spec ResourceSpec,
) (ResourceSpec, error) {
	normalizedScope := scope.Normalize()
	if err := normalizedScope.Validate("scope"); err != nil {
		return ResourceSpec{}, err
	}
	next := ResourceSpec{
		AgentName:       strings.TrimSpace(spec.AgentName),
		AgentResourceID: strings.TrimSpace(spec.AgentResourceID),
		SourcePath:      strings.TrimSpace(spec.SourcePath),
		Body:            spec.Body,
	}
	if next.AgentName == "" {
		return ResourceSpec{}, fmt.Errorf("%w: agent_name is required", resources.ErrValidation)
	}
	if next.AgentResourceID == "" {
		return ResourceSpec{}, fmt.Errorf("%w: agent_resource_id is required", resources.ErrValidation)
	}
	if next.SourcePath == "" {
		return ResourceSpec{}, fmt.Errorf("%w: source_path is required", resources.ErrValidation)
	}
	if strings.TrimSpace(next.Body) == "" {
		return ResourceSpec{}, fmt.Errorf("%w: body is required", resources.ErrValidation)
	}
	if _, err := Parse(ctx, ParseRequest{
		SourcePath: next.SourcePath,
		Content:    []byte(next.Body),
		Config:     aghconfig.DefaultHeartbeatConfig(),
	}); err != nil {
		return ResourceSpec{}, errors.Join(resources.ErrValidation, err)
	}
	return next, nil
}
