package extensionpkg

import (
	"context"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

type extensionProjectionRuntime struct {
	state       ExtensionToolRuntimeState
	descriptors map[toolspkg.ToolID]*toolspkg.ExtensionToolRuntimeDescriptor
	duplicates  map[toolspkg.ToolID]bool
	err         error
}

// ProjectionAvailability shares live observations only within one projection request.
func (p *ExtensionToolProvider) ProjectionAvailability(
	ctx context.Context,
	scope toolspkg.Scope,
	ids []toolspkg.ToolID,
) map[toolspkg.ToolID]toolspkg.Availability {
	availability := make(map[toolspkg.ToolID]toolspkg.Availability, len(ids))
	if len(ids) == 0 {
		return availability
	}
	manifestTools, err := p.manifestTools(ctx, scope)
	if err != nil {
		toolspkg.LogBackendFailure(ctx, ids[0], "tool_projection_manifest_read", err)
		for _, id := range ids {
			availability[id] = toolspkg.Unavailable(toolspkg.ReasonBackendUnhealthy)
		}
		return availability
	}
	requested := make(map[toolspkg.ToolID]struct{}, len(ids))
	for _, id := range ids {
		requested[id] = struct{}{}
	}
	snapshots := make(map[InstanceKey]extensionProjectionRuntime)
	for i := range manifestTools {
		manifest := &manifestTools[i]
		id := manifest.descriptor.Tool.ID
		if _, wanted := requested[id]; !wanted {
			continue
		}
		if _, resolved := availability[id]; resolved {
			continue
		}
		snapshot, loaded := snapshots[manifest.key]
		if !loaded {
			handle := extensionToolHandle{manifest: *manifest, runtime: p.runtime, scope: scope}
			snapshot = handle.projectionRuntime(ctx)
			snapshots[manifest.key] = snapshot
		}
		availability[id] = snapshot.availability(&manifest.descriptor)
	}
	return availability
}

func (h *extensionToolHandle) projectionRuntime(ctx context.Context) extensionProjectionRuntime {
	state, runtime := h.runtimeState()
	snapshot := extensionProjectionRuntime{state: state}
	if runtime == nil {
		return snapshot
	}
	descriptors, err := h.provideTools(ctx, runtime)
	snapshot.err = err
	if err != nil {
		return snapshot
	}
	snapshot.descriptors = make(map[toolspkg.ToolID]*toolspkg.ExtensionToolRuntimeDescriptor, len(descriptors))
	snapshot.duplicates = make(map[toolspkg.ToolID]bool)
	for i := range descriptors {
		id := descriptors[i].ID
		if _, exists := snapshot.descriptors[id]; exists {
			snapshot.duplicates[id] = true
			continue
		}
		snapshot.descriptors[id] = &descriptors[i]
	}
	return snapshot
}

func (s extensionProjectionRuntime) availability(manifest *ManifestToolDescriptor) toolspkg.Availability {
	id := manifest.Tool.ID
	availability := ReconcileManifestToolRuntime(manifest, s.descriptors[id], s.state)
	if s.err != nil {
		availability.ReasonCodes = appendToolReason(availability.ReasonCodes, toolspkg.ReasonBackendUnhealthy)
	}
	if s.duplicates[id] {
		availability.ReasonCodes = appendToolReason(availability.ReasonCodes, toolspkg.ReasonRuntimeDescriptorMismatch)
		availability.Available = false
		availability.Executable = false
	}
	return availability
}
