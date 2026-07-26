package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// NativeAvailabilityFunc computes runtime availability for one daemon-compiled tool.
type NativeAvailabilityFunc func(ctx context.Context, scope Scope) Availability

// NativeTool binds one descriptor to an in-process daemon handler.
type NativeTool struct {
	Descriptor   Descriptor
	Call         NativeToolFunc
	Availability NativeAvailabilityFunc
}

// NativeProvider serves daemon-compiled native_go tools.
type NativeProvider struct {
	source SourceRef
	tools  map[ToolID]*NativeTool
	ids    []ToolID
}

var _ Provider = (*NativeProvider)(nil)

// NewNativeProvider validates and indexes native tools for one source.
func NewNativeProvider(source SourceRef, nativeTools ...NativeTool) (*NativeProvider, error) {
	if err := source.Validate("source"); err != nil {
		return nil, err
	}
	provider := &NativeProvider{
		source: source,
		tools:  make(map[ToolID]*NativeTool, len(nativeTools)),
		ids:    make([]ToolID, 0, len(nativeTools)),
	}
	for i := range nativeTools {
		nativeTool := &nativeTools[i]
		descriptor, err := DescriptorWithSchemaDigests(nativeTool.Descriptor)
		if err != nil {
			return nil, wrapField(err, fmt.Sprintf("tools[%d].descriptor", i))
		}
		nativeTool.Descriptor = descriptor
		if err := validateNativeTool(source, nativeTool); err != nil {
			return nil, wrapField(err, fmt.Sprintf("tools[%d]", i))
		}
		id := nativeTool.Descriptor.ID
		if _, ok := provider.tools[id]; ok {
			return nil, NewValidationError(
				fmt.Sprintf("tools[%d].descriptor.id", i),
				ReasonConflictedID,
				fmt.Sprintf("duplicate native tool %q", id),
			)
		}
		provider.tools[id] = cloneNativeTool(nativeTool)
		provider.ids = append(provider.ids, id)
	}
	slices.Sort(provider.ids)
	return provider, nil
}

// ID returns the provider provenance.
func (p *NativeProvider) ID() SourceRef {
	if p == nil {
		return SourceRef{}
	}
	return p.source
}

// List returns deterministic native descriptors.
func (p *NativeProvider) List(_ context.Context, _ Scope) ([]Descriptor, error) {
	if p == nil {
		return nil, NewValidationError("provider", ReasonDependencyMissing, "native provider is required")
	}
	descriptors := make([]Descriptor, 0, len(p.ids))
	for _, id := range p.ids {
		descriptors = append(descriptors, cloneDescriptor(p.tools[id].Descriptor))
	}
	return descriptors, nil
}

// Resolve returns the executable handle for one native tool.
func (p *NativeProvider) Resolve(_ context.Context, scope Scope, id ToolID) (Handle, bool, error) {
	if p == nil {
		return nil, false, NewValidationError("provider", ReasonDependencyMissing, "native provider is required")
	}
	nativeTool, ok := p.tools[id]
	if !ok {
		return nil, false, nil
	}
	return &nativeHandle{
		descriptor:   cloneDescriptor(nativeTool.Descriptor),
		call:         nativeTool.Call,
		availability: nativeTool.Availability,
		scope:        scope,
	}, true, nil
}

type nativeHandle struct {
	descriptor   Descriptor
	call         NativeToolFunc
	availability NativeAvailabilityFunc
	scope        Scope
}

var _ Handle = (*nativeHandle)(nil)

func (h *nativeHandle) Descriptor() Descriptor {
	if h == nil {
		return Descriptor{}
	}
	return cloneDescriptor(h.descriptor)
}

func (h *nativeHandle) Availability(ctx context.Context, scope Scope) Availability {
	if h == nil || h.call == nil {
		return Availability{
			Enabled:     true,
			ReasonCodes: []ReasonCode{ReasonBackendNotExecutable},
		}
	}
	if h.availability != nil {
		return h.availability(ctx, scope)
	}
	return Available()
}

func (h *nativeHandle) Call(ctx context.Context, req CallRequest) (ToolResult, error) {
	if h == nil || h.call == nil {
		var id ToolID
		if h != nil {
			id = h.descriptor.ID
		}
		return ToolResult{}, NewToolError(
			ErrorCodeUnavailable,
			id,
			fmt.Sprintf("tool %q native handler is missing", id),
			ErrToolUnavailable,
			ReasonBackendNotExecutable,
		)
	}
	var err error
	req, err = normalizeCallRequest(h.scope, req)
	if err != nil {
		return ToolResult{}, err
	}
	scope := h.scope
	scope.WorkspaceID = req.WorkspaceID
	scope.SessionID = req.SessionID
	scope.AgentName = req.AgentName
	if actorKind := strings.TrimSpace(req.ActorKind); actorKind != "" {
		scope.ActorKind = actorKind
	}
	return h.call(ctx, scope, req)
}

// Available returns the default executable availability for healthy native tools.
func Available() Availability {
	return Availability{
		Enabled:    true,
		Available:  true,
		Authorized: true,
		Executable: true,
	}
}

// Unavailable returns a deterministic unavailable state for missing native dependencies.
func Unavailable(reason ReasonCode) Availability {
	if reason == "" {
		reason = ReasonDependencyMissing
	}
	return Availability{
		Enabled:     true,
		ReasonCodes: []ReasonCode{reason},
	}
}

func validateNativeTool(source SourceRef, nativeTool *NativeTool) error {
	if nativeTool == nil {
		return NewValidationError("tool", ReasonDependencyMissing, "native tool is required")
	}
	if nativeTool.Call == nil {
		return NewValidationError("call", ReasonHandlerMissing, "native tool handler is required")
	}
	descriptor := nativeTool.Descriptor
	if err := descriptor.Validate(); err != nil {
		return err
	}
	if descriptor.Backend.Kind != BackendNativeGo {
		return NewValidationError(
			"descriptor.backend.kind",
			ReasonBackendNotExecutable,
			"native tool must use native_go",
		)
	}
	if descriptor.Source != source {
		return NewValidationError(
			"descriptor.source",
			ReasonSourceDisabled,
			"native tool source must match provider source",
		)
	}
	if strings.TrimSpace(descriptor.Backend.NativeName) == "" {
		return NewValidationError("descriptor.backend.native_name", ReasonHandlerMissing, "native_name is required")
	}
	return nil
}

func cloneNativeTool(src *NativeTool) *NativeTool {
	if src == nil {
		return nil
	}
	return &NativeTool{
		Descriptor:   cloneDescriptor(src.Descriptor),
		Call:         src.Call,
		Availability: src.Availability,
	}
}
