package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

const (
	mcpNamespace               = "mcp"
	mcpProviderOwner           = "mcp"
	mcpAuthStatusUnconfigured  = "unconfigured"
	mcpAuthStatusNeedsLogin    = "needs_login"
	mcpAuthStatusAuthenticated = "authenticated"
	mcpAuthStatusExpired       = "expired"
	mcpAuthStatusInvalid       = "invalid"
	mcpAuthStatusRefreshFailed = "refresh_failed"
	mcpSourceScopeGlobal       = "global"
	mcpSourceScopeWorkspace    = "workspace"
)

// MCPSourceLister returns configured MCP server sources for dynamic discovery.
type MCPSourceLister interface {
	ListMCPSources(ctx context.Context) ([]SourceRef, error)
}

// MCPSourceListerFunc adapts a function into an MCP source lister.
type MCPSourceListerFunc func(context.Context) ([]SourceRef, error)

// ListMCPSources returns configured MCP server sources.
func (f MCPSourceListerFunc) ListMCPSources(ctx context.Context) ([]SourceRef, error) {
	if f == nil {
		return nil, nil
	}
	return f(ctx)
}

// MCPProvider adapts daemon-owned MCP discovery and calls into registry descriptors.
type MCPProvider struct {
	source      SourceRef
	sources     MCPSourceLister
	exec        MCPCallExecutor
	auth        MCPAuthStatusProvider
	reliability *mcpReliability
}

var _ Provider = (*MCPProvider)(nil)

// NewMCPProvider creates a registry provider for daemon-owned MCP call-through.
func NewMCPProvider(
	sources MCPSourceLister,
	exec MCPCallExecutor,
	auth MCPAuthStatusProvider,
	opts ...MCPProviderOption,
) (*MCPProvider, error) {
	if isNilInterface(exec) {
		return nil, NewValidationError(
			"executor",
			ReasonDependencyMissing,
			"mcp executor is required",
		)
	}
	if isNilInterface(sources) {
		return nil, NewValidationError(
			"sources",
			ReasonDependencyMissing,
			"mcp source lister is required",
		)
	}
	provider := &MCPProvider{
		source: SourceRef{
			Kind:  SourceDynamic,
			Owner: mcpProviderOwner,
		},
		sources: sources,
		exec:    exec,
		auth:    auth,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(provider)
		}
	}
	if err := provider.source.Validate("source"); err != nil {
		return nil, err
	}
	return provider, nil
}

// ID returns aggregate provider provenance.
func (p *MCPProvider) ID() SourceRef {
	if p == nil {
		return SourceRef{}
	}
	return p.source
}

// List discovers configured MCP tools and normalizes them into registry descriptors.
func (p *MCPProvider) List(ctx context.Context, scope Scope) ([]Descriptor, error) {
	if p == nil || isNilInterface(p.exec) || isNilInterface(p.sources) {
		return nil, NewValidationError(
			"provider",
			ReasonDependencyMissing,
			"mcp provider is required",
		)
	}
	if err := contextErr(ctx, ""); err != nil {
		return nil, err
	}
	sources, err := p.sources.ListMCPSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("tools: list mcp sources: %w", err)
	}
	sources = mcpSourcesForScope(sources, scope)
	if p.reliability != nil {
		p.reliability.retainActiveSources(scope, sources)
	}
	descriptors := make([]Descriptor, 0)
	for _, source := range sources {
		sourceDescriptors, err := p.listSourceDescriptors(ctx, source)
		if err != nil {
			return nil, err
		}
		descriptors = append(descriptors, sourceDescriptors...)
	}
	slices.SortFunc(descriptors, func(left, right Descriptor) int {
		return strings.Compare(left.ID.String(), right.ID.String())
	})
	return descriptors, nil
}

func mcpSourcesForScope(sources []SourceRef, scope Scope) []SourceRef {
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	workspaceSources := make([]SourceRef, 0)
	workspaceServerNames := make(map[string]struct{})
	if workspaceID != "" {
		for _, source := range sources {
			if strings.TrimSpace(source.Scope) != mcpSourceScopeWorkspace ||
				strings.TrimSpace(source.WorkspaceID) != workspaceID {
				continue
			}
			workspaceSources = append(workspaceSources, source)
			workspaceServerNames[mcpSourceServerName(source)] = struct{}{}
		}
	}

	projected := make([]SourceRef, 0, len(sources))
	for _, source := range sources {
		sourceScope := strings.TrimSpace(source.Scope)
		if (sourceScope != "" && sourceScope != mcpSourceScopeGlobal) ||
			strings.TrimSpace(source.WorkspaceID) != "" {
			continue
		}
		if _, shadowed := workspaceServerNames[mcpSourceServerName(source)]; shadowed {
			continue
		}
		projected = append(projected, source)
	}
	return append(projected, workspaceSources...)
}

func mcpSourceServerName(source SourceRef) string {
	return strings.TrimSpace(firstNonEmpty(source.RawServerName, source.Owner))
}

// Resolve returns a handle for one discovered MCP tool.
func (p *MCPProvider) Resolve(ctx context.Context, scope Scope, id ToolID) (Handle, bool, error) {
	if err := id.Validate(); err != nil {
		return nil, false, err
	}
	descriptors, err := p.List(ctx, scope)
	if err != nil {
		return nil, false, err
	}
	for i := range descriptors {
		if descriptors[i].ID != id {
			continue
		}
		return &mcpHandle{
			descriptor:  cloneDescriptor(descriptors[i]),
			exec:        p.exec,
			auth:        p.auth,
			scope:       scope,
			reliability: p.reliability,
		}, true, nil
	}
	return nil, false, nil
}

type mcpHandle struct {
	descriptor  Descriptor
	exec        MCPCallExecutor
	auth        MCPAuthStatusProvider
	scope       Scope
	reliability *mcpReliability
}

var _ Handle = (*mcpHandle)(nil)

func (h *mcpHandle) Descriptor() Descriptor {
	if h == nil {
		return Descriptor{}
	}
	return cloneDescriptor(h.descriptor)
}

func (h *mcpHandle) Availability(ctx context.Context, _ Scope) Availability {
	if h == nil || isNilInterface(h.exec) {
		return Unavailable(ReasonBackendNotExecutable)
	}
	if h.reliability != nil && h.reliability.dead(ctx, h.descriptor.Source) {
		return Unavailable(ReasonBackendDead)
	}
	if isNilInterface(h.auth) {
		return Available()
	}
	status, err := h.auth.Status(ctx, h.descriptor.Source)
	if err != nil {
		return Unavailable(ReasonBackendUnhealthy)
	}
	reason, ok := MCPAuthStatusReason(status)
	if !ok || reason == ReasonMCPAuthUnconfigured {
		return Available()
	}
	return Availability{
		Enabled:     true,
		Available:   true,
		Authorized:  false,
		Executable:  false,
		ReasonCodes: []ReasonCode{reason},
	}
}

func (h *mcpHandle) Call(ctx context.Context, req CallRequest) (ToolResult, error) {
	if h == nil || isNilInterface(h.exec) {
		return ToolResult{}, NewToolError(
			ErrorCodeUnavailable,
			req.ToolID,
			"mcp tool handle is unavailable",
			ErrToolUnavailable,
			ReasonBackendNotExecutable,
		)
	}
	availability := h.Availability(ctx, h.scope)
	if !availability.Executable {
		return ToolResult{}, NewToolError(
			ErrorCodeUnavailable,
			h.descriptor.ID,
			fmt.Sprintf("tool %q is unavailable", h.descriptor.ID),
			ErrToolUnavailable,
			availability.ReasonCodes...,
		)
	}
	return h.exec.CallTool(ctx, h.descriptor.Source, MCPToolCallRequest{
		ToolID:      h.descriptor.ID,
		RawToolName: h.descriptor.Source.RawToolName,
		Input:       cloneRawMessage(req.Input),
	})
}

// Canonicalize normalizes one raw MCP server/tool pair into the canonical registry ToolID.
func Canonicalize(rawServer, rawTool string) (ToolID, error) {
	server, err := canonicalMCPSegment("mcp.server", rawServer)
	if err != nil {
		return "", err
	}
	tool, err := canonicalMCPSegment("mcp.tool", rawTool)
	if err != nil {
		return "", err
	}
	id := ToolID(mcpNamespace + "__" + server + "__" + tool)
	if len(id) > maxSegmentedIDLength {
		return "", NewValidationError(
			"tool_id",
			ReasonIDTooLong,
			"mcp tool id exceeds 64 characters",
		)
	}
	if err := id.Validate(); err != nil {
		return "", err
	}
	return id, nil
}

// MCPAuthStatusReason maps redacted MCP auth status to registry reason codes.
func MCPAuthStatusReason(status MCPAuthStatus) (ReasonCode, bool) {
	switch strings.TrimSpace(status.Status) {
	case mcpAuthStatusUnconfigured:
		return ReasonMCPAuthUnconfigured, true
	case mcpAuthStatusNeedsLogin:
		return ReasonMCPAuthRequired, true
	case mcpAuthStatusExpired:
		return ReasonMCPAuthExpired, true
	case mcpAuthStatusInvalid:
		return ReasonMCPAuthInvalid, true
	case mcpAuthStatusRefreshFailed:
		return ReasonMCPAuthRefreshFailed, true
	case "", mcpAuthStatusAuthenticated:
		return "", false
	default:
		return "", false
	}
}

func mcpRegistryDescriptor(source SourceRef, desc MCPToolDescriptor) (Descriptor, error) {
	rawServer := firstNonEmpty(desc.Source.RawServerName, source.RawServerName, source.Owner)
	rawTool := firstNonEmpty(desc.Source.RawToolName, desc.RawName)
	id, err := Canonicalize(rawServer, rawTool)
	if err != nil {
		return Descriptor{}, err
	}
	if desc.ID != "" && desc.ID != id {
		return Descriptor{}, NewValidationError(
			"id",
			ReasonConflictedID,
			"mcp descriptor id does not match canonical raw server/tool names",
		)
	}
	owner, err := mcpOwnerFromID(id)
	if err != nil {
		return Descriptor{}, err
	}
	mcpSource := desc.Source
	mcpSource.Kind = SourceMCP
	mcpSource.Owner = owner
	mcpSource.RawServerName = rawServer
	mcpSource.RawToolName = rawTool
	if mcpSource.ResourceID == "" {
		mcpSource.ResourceID = source.ResourceID
	}
	if mcpSource.ResourceVersion == "" {
		mcpSource.ResourceVersion = source.ResourceVersion
	}
	if mcpSource.WorkspaceID == "" {
		mcpSource.WorkspaceID = source.WorkspaceID
	}
	if mcpSource.Scope == "" {
		mcpSource.Scope = source.Scope
	}

	readOnly := desc.ReadOnly
	risk := RiskOpenWorld
	openWorld := true
	if readOnly {
		risk = RiskRead
		openWorld = false
	}
	displayTitle := strings.TrimSpace(desc.Title)
	if displayTitle == "" {
		displayTitle = rawTool
	}
	descriptor := Descriptor{
		ID:           id,
		Backend:      BackendRef{Kind: BackendMCP, MCPServer: owner, MCPTool: rawTool},
		DisplayTitle: displayTitle,
		ToolPresentation: NewToolPresentation(
			strings.TrimSpace(desc.FriendlyVerb),
			strings.TrimSpace(desc.Preview),
		),
		Description:     strings.TrimSpace(desc.Description),
		InputSchema:     cloneRawMessage(desc.InputSchema),
		OutputSchema:    cloneRawMessage(desc.OutputSchema),
		Source:          mcpSource,
		Visibility:      VisibilityModel,
		Risk:            risk,
		ReadOnly:        readOnly,
		Destructive:     false,
		OpenWorld:       openWorld,
		ConcurrencySafe: readOnly,
	}
	return DescriptorWithSchemaDigests(descriptor)
}

func canonicalMCPSegment(field string, raw string) (string, error) {
	trimmed := strings.Trim(raw, " \t\n\r\v\f")
	if trimmed == "" {
		return "", NewValidationError(field, ReasonIDEmptySegment, "mcp name segment is required")
	}
	var builder strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '_':
			builder.WriteRune(r)
		case r == '-' || r == '.':
			builder.WriteRune('_')
		default:
			return "", NewValidationError(
				field,
				ReasonIDInvalidFormat,
				"mcp name contains an unsupported character",
			)
		}
	}
	segment := builder.String()
	switch {
	case segment == "":
		return "", NewValidationError(field, ReasonIDEmptySegment, "mcp name segment is required")
	case segment[0] < 'a' || segment[0] > 'z':
		return "", NewValidationError(
			field,
			ReasonIDInvalidFormat,
			"segment must start with a lowercase letter",
		)
	case strings.HasPrefix(segment, "_") || strings.HasSuffix(segment, "_"):
		return "", NewValidationError(
			field,
			ReasonIDReservedConflict,
			"segment uses reserved underscore boundary",
		)
	case strings.Contains(segment, "__"):
		return "", NewValidationError(
			field,
			ReasonIDReservedConflict,
			"segment contains reserved separator",
		)
	default:
		return segment, nil
	}
}

func mcpOwnerFromID(id ToolID) (string, error) {
	segments, err := id.Segments()
	if err != nil {
		return "", err
	}
	if len(segments) != 3 || segments[0] != mcpNamespace {
		return "", NewValidationError(
			"tool_id",
			ReasonIDInvalidFormat,
			"mcp tool id must have mcp server and tool segments",
		)
	}
	return segments[1], nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
