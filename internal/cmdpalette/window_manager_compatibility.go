package cmdpalette

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/windowmanager"
)

// Retired palette IDs remain invocable without catalog entries or client dispatch until v0.8.0.
func (s *Service) invokeRetiredWindowManagerCommand(ctx context.Context, request InvokeRequest) (InvokeResult, error) {
	if ctx == nil || request.WorkspaceID == "" {
		return InvokeResult{}, errors.New("cmd palette: invoke context and workspace ID are required")
	}
	if err := request.ProfileLens.Validate(); err != nil {
		return InvokeResult{}, err
	}
	invocationID := strings.TrimSpace(s.newID())
	if invocationID == "" {
		return InvokeResult{}, errors.New("cmd palette: generated invocation ID is empty")
	}
	entry := windowmanager.SurfaceDeprecation(string(request.CommandID))
	if _, warned := s.retiredWarnings.LoadOrStore(entry.Surface, struct{}{}); !warned {
		windowmanager.WarnSurfaceDeprecation(ctx, s.logger, entry, "cmd_palette")
	}
	payload, err := json.Marshal(struct {
		Applied      bool                        `json:"applied"`
		Deprecations []windowmanager.Deprecation `json:"deprecations"`
	}{Deprecations: []windowmanager.Deprecation{entry}})
	if err != nil {
		return InvokeResult{}, err
	}
	execution := ExecutionRequest{
		ProfileLens: request.ProfileLens, WorkspaceID: request.WorkspaceID, InvocationID: invocationID,
		Descriptor: Descriptor{ID: request.CommandID, Source: Source{Kind: SourceKindCore}},
	}
	s.emitInvocation(ctx, execution, "ok", "", s.now().UTC())
	return InvokeResult{ProfileLens: request.ProfileLens, Status: InvokeStatusOK,
		InvocationID: invocationID, Result: payload}, nil
}
