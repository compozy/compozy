package extensionpkg

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const extensionToolProviderOwner = "extensions"

// ExtensionToolRuntime is the live runtime surface needed by extension-host
// tool handles.
type ExtensionToolRuntime interface {
	Get(name string) (*Extension, error)
	toolspkg.ExtensionToolInvoker
}

type extensionScopedToolRuntime interface {
	ExtensionToolRuntime
	GetForInstance(InstanceKey) (*Extension, error)
	ListForWorkspace(string) []ExtensionInfo
	ProvideToolsForInstance(context.Context, InstanceKey) ([]toolspkg.ExtensionToolRuntimeDescriptor, error)
	CallToolForInstance(context.Context, InstanceKey, toolspkg.ExtensionToolCallRequest) (toolspkg.ToolResult, error)
}

// ExtensionToolRuntimeResolver returns the current live extension runtime.
type ExtensionToolRuntimeResolver func() ExtensionToolRuntime

// ExtensionToolProviderOption configures an extension-host tool provider.
type ExtensionToolProviderOption func(*ExtensionToolProvider)

// ExtensionToolProvider lists manifest-authored extension tools and resolves
// executable handles through the live subprocess runtime.
type ExtensionToolProvider struct {
	mu sync.RWMutex
	// manifestRefresh serializes manifest observation through cache publication.
	// The cache mutex stays unheld while this boundary performs filesystem I/O.
	manifestRefresh  chan struct{}
	manifestReadFile extensionManifestReadFile
	registry         *Registry
	runtime          ExtensionToolRuntimeResolver
	logger           *slog.Logger
	source           toolspkg.SourceRef
	cache            extensionToolProviderCache
}

var _ toolspkg.Provider = (*ExtensionToolProvider)(nil)

// WithToolProviderLogger injects the logger used when one invalid extension is
// isolated from the aggregate tool catalog.
func WithToolProviderLogger(logger *slog.Logger) ExtensionToolProviderOption {
	return func(provider *ExtensionToolProvider) {
		if logger != nil {
			provider.logger = logger
		}
	}
}

// NewExtensionToolProvider creates the extension_host provider for the central
// tool registry.
func NewExtensionToolProvider(
	registry *Registry,
	runtime ExtensionToolRuntimeResolver,
	opts ...ExtensionToolProviderOption,
) (*ExtensionToolProvider, error) {
	if registry == nil {
		return nil, toolspkg.NewValidationError(
			"registry",
			toolspkg.ReasonDependencyMissing,
			"extension registry is required",
		)
	}
	provider := &ExtensionToolProvider{
		manifestRefresh:  make(chan struct{}, 1),
		manifestReadFile: os.ReadFile,
		registry:         registry,
		runtime:          runtime,
		logger:           slog.Default(),
		source: toolspkg.SourceRef{
			Kind:  toolspkg.SourceExtension,
			Owner: extensionToolProviderOwner,
		},
	}
	for _, opt := range opts {
		opt(provider)
	}
	if err := provider.source.Validate("source"); err != nil {
		return nil, err
	}
	return provider, nil
}

// ID returns the aggregate extension-provider provenance.
func (p *ExtensionToolProvider) ID() toolspkg.SourceRef {
	if p == nil {
		return toolspkg.SourceRef{}
	}
	return p.source
}

// List returns manifest-authoritative extension-host tool descriptors.
func (p *ExtensionToolProvider) List(ctx context.Context, scope toolspkg.Scope) ([]toolspkg.Descriptor, error) {
	if err := extensionProviderContextErr(ctx); err != nil {
		return nil, err
	}
	manifestTools, err := p.manifestTools(ctx, scope)
	if err != nil {
		return nil, err
	}
	descriptors := make([]toolspkg.Descriptor, 0, len(manifestTools))
	for i := range manifestTools {
		descriptors = append(descriptors, manifestTools[i].descriptor.Tool.Descriptor())
	}
	return descriptors, nil
}

// Resolve returns a handle that reconciles one manifest descriptor against the
// live extension runtime before allowing execution.
func (p *ExtensionToolProvider) Resolve(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.Handle, bool, error) {
	if err := extensionProviderContextErr(ctx); err != nil {
		return nil, false, err
	}
	if err := id.Validate(); err != nil {
		return nil, false, err
	}
	manifestTool, found, err := p.manifestTool(ctx, scope, id)
	if err != nil || !found {
		return nil, found, err
	}
	return &extensionToolHandle{
		manifest: manifestTool,
		runtime:  p.runtime,
		scope:    scope,
	}, true, nil
}

type extensionToolHandle struct {
	manifest extensionManifestTool
	runtime  ExtensionToolRuntimeResolver
	scope    toolspkg.Scope
}

var _ toolspkg.Handle = (*extensionToolHandle)(nil)

func (h *extensionToolHandle) Descriptor() toolspkg.Descriptor {
	if h == nil {
		return toolspkg.Descriptor{}
	}
	return h.manifest.descriptor.Tool.Descriptor()
}

func (h *extensionToolHandle) Availability(ctx context.Context, _ toolspkg.Scope) toolspkg.Availability {
	if h == nil {
		return toolspkg.Unavailable(toolspkg.ReasonBackendNotExecutable)
	}
	state, runtime := h.runtimeState()
	if runtime == nil {
		return ReconcileManifestToolRuntime(&h.manifest.descriptor, nil, state)
	}
	descriptors, err := h.provideTools(ctx, runtime)
	if err != nil {
		availability := ReconcileManifestToolRuntime(&h.manifest.descriptor, nil, state)
		availability.ReasonCodes = appendToolReason(availability.ReasonCodes, toolspkg.ReasonBackendUnhealthy)
		return availability
	}
	runtimeDescriptor, duplicate := runtimeDescriptorForTool(descriptors, h.manifest.descriptor.Tool.ID)
	availability := ReconcileManifestToolRuntime(&h.manifest.descriptor, runtimeDescriptor, state)
	if duplicate {
		availability.ReasonCodes = appendToolReason(
			availability.ReasonCodes,
			toolspkg.ReasonRuntimeDescriptorMismatch,
		)
		availability.Available = false
		availability.Executable = false
	}
	return availability
}

func (h *extensionToolHandle) Call(ctx context.Context, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
	if h == nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			req.ToolID,
			"extension tool handle is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonBackendNotExecutable,
		)
	}
	availability := h.Availability(ctx, h.scope)
	if !availability.Executable {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			h.manifest.descriptor.Tool.ID,
			fmt.Sprintf("tool %q is unavailable", h.manifest.descriptor.Tool.ID),
			toolspkg.ErrToolUnavailable,
			availability.ReasonCodes...,
		)
	}
	runtime := h.runtimeInstance()
	if runtime == nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			h.manifest.descriptor.Tool.ID,
			fmt.Sprintf("tool %q runtime is unavailable", h.manifest.descriptor.Tool.ID),
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonExtensionInactive,
		)
	}
	callRequest := toolspkg.ExtensionToolCallRequest{
		ToolID:    h.manifest.descriptor.Tool.ID,
		Handler:   h.manifest.descriptor.Tool.Backend.Handler,
		SessionID: req.SessionID,
		Input:     cloneRawMessage(req.Input),
	}
	if root := strings.TrimSpace(req.TrustedWorkspaceRoot); root != "" {
		callRequest.TrustedWorkspace = &toolspkg.ExtensionToolWorkspaceScope{
			ID:   strings.TrimSpace(req.WorkspaceID),
			Root: root,
		}
	}
	if scoped, ok := runtime.(extensionScopedToolRuntime); ok {
		return scoped.CallToolForInstance(ctx, h.manifest.key, callRequest)
	}
	return runtime.CallTool(ctx, h.extensionID(), callRequest)
}

func (h *extensionToolHandle) runtimeState() (ExtensionToolRuntimeState, ExtensionToolRuntime) {
	state := ExtensionToolRuntimeState{
		Enabled: h.manifest.info.Enabled,
	}
	runtime := h.runtimeInstance()
	if runtime == nil {
		return state, nil
	}
	var snapshot *Extension
	var err error
	if scoped, ok := runtime.(extensionScopedToolRuntime); ok {
		snapshot, err = scoped.GetForInstance(h.manifest.key)
	} else {
		snapshot, err = runtime.Get(h.extensionID())
	}
	if err != nil || snapshot == nil {
		return state, runtime
	}
	state.Enabled = snapshot.Info.Enabled
	state.Active = snapshot.Status.Active
	state.Healthy = snapshot.Status.Healthy
	if snapshot.InitializeResult != nil {
		state.ProvidedCapabilities = slices.Clone(snapshot.InitializeResult.AcceptedCapabilities.Provides)
	}
	return state, runtime
}

func (h *extensionToolHandle) runtimeInstance() ExtensionToolRuntime {
	if h == nil || h.runtime == nil {
		return nil
	}
	return h.runtime()
}

func (h *extensionToolHandle) extensionID() string {
	return strings.TrimSpace(h.manifest.descriptor.Tool.Backend.ExtensionID)
}

func (h *extensionToolHandle) provideTools(
	ctx context.Context,
	runtime ExtensionToolRuntime,
) ([]toolspkg.ExtensionToolRuntimeDescriptor, error) {
	if scoped, ok := runtime.(extensionScopedToolRuntime); ok {
		return scoped.ProvideToolsForInstance(ctx, h.manifest.key)
	}
	return runtime.ProvideTools(ctx, h.extensionID())
}

func (p *ExtensionToolProvider) runtimeInstance() ExtensionToolRuntime {
	if p == nil || p.runtime == nil {
		return nil
	}
	return p.runtime()
}

func runtimeDescriptorForTool(
	descriptors []toolspkg.ExtensionToolRuntimeDescriptor,
	id toolspkg.ToolID,
) (*toolspkg.ExtensionToolRuntimeDescriptor, bool) {
	var found *toolspkg.ExtensionToolRuntimeDescriptor
	for i := range descriptors {
		if descriptors[i].ID != id {
			continue
		}
		if found != nil {
			return found, true
		}
		descriptor := descriptors[i]
		descriptor.Capabilities = slices.Clone(descriptors[i].Capabilities)
		descriptor.Command = cloneExtensionCommandSpec(descriptors[i].Command)
		found = &descriptor
	}
	return found, false
}

func extensionProviderContextErr(ctx context.Context) error {
	if ctx == nil {
		return ErrContextRequired
	}
	return ctx.Err()
}
