package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// RegistryOption configures a runtime registry.
type RegistryOption func(*RuntimeRegistry)

// RuntimeRegistry indexes providers and produces scoped projections.
type RuntimeRegistry struct {
	providers             []Provider
	descriptorMetadata    descriptorMetadataIndex
	evaluator             PolicyEvaluator
	policyInputs          PolicyInputs
	policyResolver        PolicyInputResolver
	toolsets              ToolsetCatalog
	hooks                 HookRunner
	approvalBridge        ApprovalBridge
	processor             ResultProcessor
	events                ToolEventSink
	defaultMaxResultBytes int64
	sensitiveFields       []string
	usePolicyInput        bool
}

var _ Registry = (*RuntimeRegistry)(nil)

// NewRegistry validates providers and returns a deterministic registry.
func NewRegistry(opts ...RegistryOption) (*RuntimeRegistry, error) {
	registry := &RuntimeRegistry{
		policyInputs: DefaultPolicyInputs(),
	}
	for _, opt := range opts {
		opt(registry)
	}
	for i, provider := range registry.providers {
		if err := ValidateProvider(provider); err != nil {
			return nil, wrapField(err, fmt.Sprintf("providers[%d]", i))
		}
	}
	slices.SortFunc(registry.providers, func(a Provider, b Provider) int {
		return strings.Compare(sourceKey(a.ID()), sourceKey(b.ID()))
	})
	return registry, nil
}

// WithProviders registers provider sources for indexing.
func WithProviders(providers ...Provider) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.providers = append(registry.providers, providers...)
	}
}

// WithPolicyEvaluator injects a custom evaluator for tests or composition roots.
func WithPolicyEvaluator(evaluator PolicyEvaluator) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.evaluator = evaluator
		registry.policyResolver = nil
		registry.usePolicyInput = false
	}
}

// WithPolicyInputs configures the default effective policy evaluator.
func WithPolicyInputs(inputs PolicyInputs, toolsets ToolsetCatalog) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.policyInputs = clonePolicyInputs(inputs)
		registry.policyResolver = NewStaticPolicyInputResolver(inputs)
		registry.toolsets = toolsets
		registry.usePolicyInput = true
	}
}

// WithPolicyInputResolver configures a scope-aware effective policy evaluator.
func WithPolicyInputResolver(resolver PolicyInputResolver, toolsets ToolsetCatalog) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.policyResolver = resolver
		registry.toolsets = toolsets
		registry.usePolicyInput = true
	}
}

// WithHookRunner wires registry-owned call hooks into dispatch.
func WithHookRunner(hooks HookRunner) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.hooks = hooks
	}
}

// WithApprovalBridge wires approval-required calls into a session permission path.
func WithApprovalBridge(bridge ApprovalBridge) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.approvalBridge = bridge
	}
}

// WithResultProcessor wires result redaction, offload, and budget enforcement.
func WithResultProcessor(processor ResultProcessor) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.processor = processor
	}
}

// WithDefaultMaxResultBytes sets the fallback result budget for silent descriptors.
func WithDefaultMaxResultBytes(maxBytes int64) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.defaultMaxResultBytes = maxBytes
	}
}

// WithSensitiveResultFields configures extra field names redacted from results.
func WithSensitiveResultFields(fields ...string) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.sensitiveFields = append(registry.sensitiveFields, fields...)
	}
}

// WithToolEventSink wires structured dispatch events into observability.
func WithToolEventSink(events ToolEventSink) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.events = events
	}
}

// List returns an operator or session projection based on Scope.Operator.
func (r *RuntimeRegistry) List(ctx context.Context, scope Scope) ([]ToolView, error) {
	if scope.Operator {
		return r.OperatorProjection(ctx, scope)
	}
	return r.SessionProjection(ctx, scope)
}

// Search filters the scoped projection by descriptor text and provenance.
func (r *RuntimeRegistry) Search(ctx context.Context, scope Scope, q SearchQuery) ([]ToolView, error) {
	views, err := r.List(ctx, scope)
	if err != nil {
		return nil, err
	}
	needle := strings.TrimSpace(strings.ToLower(q.Query))
	if needle == "" {
		return limitViews(views, q.Limit), nil
	}
	filtered := make([]ToolView, 0, len(views))
	for i := range views {
		if toolViewMatches(&views[i], needle) {
			filtered = append(filtered, views[i])
		}
	}
	return limitViews(filtered, q.Limit), nil
}

// Get returns one tool from the scoped projection.
func (r *RuntimeRegistry) Get(ctx context.Context, scope Scope, id ToolID) (ToolView, error) {
	if err := id.Validate(); err != nil {
		return ToolView{}, err
	}
	views, err := r.List(ctx, scope)
	if err != nil {
		return ToolView{}, err
	}
	for i := range views {
		if views[i].Descriptor.ID == id {
			return views[i], nil
		}
	}
	return ToolView{}, NewToolError(
		ErrorCodeNotFound,
		id,
		fmt.Sprintf("tool %q not found", id),
		ErrToolNotFound,
		ReasonToolUnknown,
	)
}

// DiagnosticProjection returns all indexed tools with effective diagnostics for the supplied scope.
func (r *RuntimeRegistry) DiagnosticProjection(ctx context.Context, scope Scope) ([]ToolView, error) {
	index, err := r.buildIndex(ctx, scope)
	if err != nil {
		return nil, err
	}
	evaluator, err := r.evaluatorFor(ctx, scope, index.ids())
	if err != nil {
		return nil, err
	}
	views := make([]ToolView, 0, len(index.entries))
	for _, entry := range index.entries {
		view, err := r.viewFor(ctx, scope, evaluator, entry)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// DiagnosticSearch filters the diagnostic projection without hiding denied tools.
func (r *RuntimeRegistry) DiagnosticSearch(ctx context.Context, scope Scope, q SearchQuery) ([]ToolView, error) {
	views, err := r.DiagnosticProjection(ctx, scope)
	if err != nil {
		return nil, err
	}
	needle := strings.TrimSpace(strings.ToLower(q.Query))
	if needle == "" {
		return limitViews(views, q.Limit), nil
	}
	filtered := make([]ToolView, 0, len(views))
	for i := range views {
		if toolViewMatches(&views[i], needle) {
			filtered = append(filtered, views[i])
		}
	}
	return limitViews(filtered, q.Limit), nil
}

// DiagnosticGet returns one diagnostic projection row even when the tool is not callable.
func (r *RuntimeRegistry) DiagnosticGet(ctx context.Context, scope Scope, id ToolID) (ToolView, error) {
	if err := id.Validate(); err != nil {
		return ToolView{}, err
	}
	views, err := r.DiagnosticProjection(ctx, scope)
	if err != nil {
		return ToolView{}, err
	}
	for i := range views {
		if views[i].Descriptor.ID == id {
			return views[i], nil
		}
	}
	return ToolView{}, NewToolError(
		ErrorCodeNotFound,
		id,
		fmt.Sprintf("tool %q not found", id),
		ErrToolNotFound,
		ReasonToolUnknown,
	)
}

// Call runs the central provider-agnostic registry dispatch pipeline.
func (r *RuntimeRegistry) Call(ctx context.Context, scope Scope, req CallRequest) (ToolResult, error) {
	return r.dispatch(ctx, scope, req)
}

// ListToolsets returns named toolsets with expansion diagnostics for the supplied scope.
func (r *RuntimeRegistry) ListToolsets(ctx context.Context, scope Scope) ([]ToolsetView, error) {
	toolsets := r.toolsets.List()
	views := make([]ToolsetView, 0, len(toolsets))
	for _, toolset := range toolsets {
		view, err := r.toolsetView(ctx, scope, toolset)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// GetToolset returns one named toolset with expansion diagnostics for the supplied scope.
func (r *RuntimeRegistry) GetToolset(ctx context.Context, scope Scope, id ToolsetID) (ToolsetView, error) {
	if err := id.Validate(); err != nil {
		return ToolsetView{}, err
	}
	toolset, ok := r.toolsets.Get(id)
	if !ok {
		return ToolsetView{}, NewToolError(
			ErrorCodeNotFound,
			ToolID(id),
			fmt.Sprintf("toolset %q not found", id),
			ErrToolNotFound,
			ReasonToolsetUnknown,
		)
	}
	return r.toolsetView(ctx, scope, toolset)
}

// OperatorProjection returns all registered tools with diagnostics.
func (r *RuntimeRegistry) OperatorProjection(ctx context.Context, scope Scope) ([]ToolView, error) {
	views, err := r.DiagnosticProjection(ctx, scope)
	if err != nil {
		return nil, err
	}
	for i := range views {
		views[i].Decision.VisibleToOperator = true
	}
	return views, nil
}

// SessionProjection returns only callable tools for the effective session.
func (r *RuntimeRegistry) SessionProjection(ctx context.Context, scope Scope) ([]ToolView, error) {
	index, err := r.buildIndex(ctx, scope)
	if err != nil {
		return nil, err
	}
	evaluator, err := r.evaluatorFor(ctx, scope, index.ids())
	if err != nil {
		return nil, err
	}
	views := make([]ToolView, 0, len(index.entries))
	for _, entry := range index.entries {
		view, err := r.viewFor(ctx, scope, evaluator, entry)
		if err != nil {
			return nil, err
		}
		if view.Decision.VisibleToSession && view.Decision.Callable {
			views = append(views, view)
		}
	}
	return views, nil
}

func (r *RuntimeRegistry) viewFor(
	ctx context.Context,
	scope Scope,
	evaluator PolicyEvaluator,
	entry *registryEntry,
) (ToolView, error) {
	availability := r.availabilityFor(ctx, scope, entry)
	decision, err := evaluator.Evaluate(ctx, scope, entry.descriptor)
	if err != nil {
		return ToolView{}, err
	}
	decision = applyAvailabilityDecision(decision, availability)
	return ToolView{
		Descriptor:   cloneDescriptor(entry.descriptor),
		Availability: availability,
		Decision:     decision,
	}, nil
}

func (r *RuntimeRegistry) availabilityFor(ctx context.Context, scope Scope, entry *registryEntry) Availability {
	if len(entry.conflicts) > 0 {
		return Availability{
			Registered:  true,
			Enabled:     true,
			Conflicted:  true,
			ReasonCodes: append([]ReasonCode(nil), entry.conflicts...),
		}
	}
	handle, ok, err := entry.provider.Resolve(ctx, scope, entry.descriptor.ID)
	if err != nil {
		return Availability{
			Registered:  true,
			Enabled:     true,
			ReasonCodes: []ReasonCode{ReasonBackendUnhealthy},
		}
	}
	if !ok || isNilInterface(handle) {
		return Availability{
			Registered:  true,
			Enabled:     true,
			ReasonCodes: []ReasonCode{ReasonBackendNotExecutable},
		}
	}
	availability := handle.Availability(ctx, scope)
	availability.Registered = true
	if err := availability.Validate(); err != nil {
		reason, found := ReasonOf(err)
		if !found {
			reason = ReasonBackendUnhealthy
		}
		return Availability{
			Registered:  true,
			Enabled:     availability.Enabled,
			Available:   availability.Available,
			Authorized:  availability.Authorized,
			Executable:  false,
			Conflicted:  availability.Conflicted,
			ReasonCodes: appendReason(availability.ReasonCodes, reason),
		}
	}
	return availability
}

func appendReason(reasons []ReasonCode, reason ReasonCode) []ReasonCode {
	if reason == "" || slices.Contains(reasons, reason) {
		return reasons
	}
	return append(reasons, reason)
}

func denialErrorForView(view *ToolView) error {
	id := view.Descriptor.ID
	reasons := view.Decision.ReasonCodes
	if slices.Contains(reasons, ReasonApprovalRequired) {
		return NewToolError(
			ErrorCodeApprovalRequired,
			id,
			fmt.Sprintf("tool %q requires approval", id),
			ErrToolApprovalRequired,
			reasons...,
		)
	}
	if view.Availability.Conflicted {
		return NewToolError(
			ErrorCodeConflict,
			id,
			fmt.Sprintf("tool %q is conflicted", id),
			ErrToolConflict,
			reasons...)
	}
	if !view.Availability.Executable {
		return NewToolError(
			ErrorCodeUnavailable,
			id,
			fmt.Sprintf("tool %q is unavailable", id),
			ErrToolUnavailable,
			reasons...,
		)
	}
	return NewToolError(ErrorCodeDenied, id, fmt.Sprintf("tool %q is denied", id), ErrToolDenied, reasons...)
}

func cloneDescriptor(src Descriptor) Descriptor {
	cloned := src
	cloned.ToolPresentation = CloneToolPresentation(src.ToolPresentation)
	cloned.InputSchema = cloneRawMessage(src.InputSchema)
	cloned.OutputSchema = cloneRawMessage(src.OutputSchema)
	cloned.Toolsets = append([]ToolsetID(nil), src.Toolsets...)
	cloned.Tags = append([]string(nil), src.Tags...)
	cloned.SearchHints = append([]string(nil), src.SearchHints...)
	return cloned
}

func cloneToolView(src *ToolView) ToolView {
	if src == nil {
		return ToolView{}
	}
	cloned := *src
	cloned.Descriptor = cloneDescriptor(src.Descriptor)
	cloned.Availability.ReasonCodes = append([]ReasonCode(nil), src.Availability.ReasonCodes...)
	cloned.Decision.ReasonCodes = append([]ReasonCode(nil), src.Decision.ReasonCodes...)
	return cloned
}

func limitViews(views []ToolView, limit int) []ToolView {
	if limit <= 0 || limit >= len(views) {
		return views
	}
	return views[:limit]
}

func toolViewMatches(view *ToolView, needle string) bool {
	d := &view.Descriptor
	values := []string{
		d.ID.String(),
		d.DisplayTitle,
		d.Description,
		d.Source.Owner,
		d.Source.RawServerName,
		d.Source.RawToolName,
	}
	values = append(values, d.Tags...)
	values = append(values, d.SearchHints...)
	for _, toolset := range d.Toolsets {
		values = append(values, toolset.String())
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}
