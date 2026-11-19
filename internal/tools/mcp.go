package tools

import (
	"cmp"
	"context"
	"encoding/base32"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	mcpNamespace                   = "mcp"
	mcpRawToolSegment              = "raw"
	mcpProviderOwner               = "mcp"
	mcpAuthStatusUnconfigured      = "unconfigured"
	mcpAuthStatusNeedsLogin        = "needs_login"
	mcpAuthStatusAuthenticated     = "authenticated"
	mcpAuthStatusExpired           = "expired"
	mcpAuthStatusInvalid           = "invalid"
	mcpAuthStatusRefreshFailed     = "refresh_failed"
	mcpSourceScopeUser             = "user"
	mcpSourceScopeProfile          = "profile"
	mcpSourceScopeWorkspace        = "workspace"
	mcpSourceScopeWorkspaceProfile = "workspace_profile"
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
	discoveryMu          sync.Mutex
	discoveryGenerations map[SourceRef]mcpDiscoveryGeneration
	discovered           sync.Map // source identity -> last discovered descriptors
	discoveryFailures    sync.Map // source identity -> failed latest discovery
	catalog              *mcpCatalogRefresh

	consent         func(context.Context, SourceRef, string) (bool, error)
	discoveryBudget time.Duration
	source          SourceRef
	sources         MCPSourceLister
	exec            MCPCallExecutor
	auth            MCPAuthStatusProvider
	reliability     *mcpReliability
}

var _ Provider = (*MCPProvider)(nil)
var _ ProjectionGenerationProvider = (*MCPProvider)(nil)

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
	descriptors, _, err := p.listProjection(ctx, scope)
	return descriptors, err
}

// MCPSourcesForScope selects the sources one scope projects, keeping the most specific definition per server.
func MCPSourcesForScope(sources []SourceRef, scope Scope) []SourceRef {
	profileID := strings.TrimSpace(scope.ProfileID)
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	bestRank := make(map[string]int)
	for _, source := range sources {
		rank, visible := mcpSourceProjectionRank(source, profileID, workspaceID)
		if !visible {
			continue
		}
		name := mcpSourceServerName(source)
		if current, exists := bestRank[name]; !exists || rank > current {
			bestRank[name] = rank
		}
	}

	projected := make([]SourceRef, 0, len(sources))
	for _, source := range sources {
		rank, visible := mcpSourceProjectionRank(source, profileID, workspaceID)
		if !visible || bestRank[mcpSourceServerName(source)] != rank {
			continue
		}
		source.ProfileID = profileID
		projected = append(projected, source)
	}
	return projected
}

func mcpSourceProjectionRank(source SourceRef, profileID, workspaceID string) (int, bool) {
	sourceProfileID := strings.TrimSpace(source.ProfileID)
	sourceWorkspaceID := strings.TrimSpace(source.WorkspaceID)
	switch strings.TrimSpace(source.Scope) {
	case "", mcpSourceScopeUser:
		return 0, sourceProfileID == "" && sourceWorkspaceID == ""
	case mcpSourceScopeProfile:
		return 1, profileID != "" && sourceProfileID == profileID && sourceWorkspaceID == ""
	case mcpSourceScopeWorkspace:
		return 2, workspaceID != "" && sourceWorkspaceID == workspaceID && sourceProfileID == ""
	case mcpSourceScopeWorkspaceProfile:
		return 3, profileID != "" && workspaceID != "" &&
			sourceProfileID == profileID && sourceWorkspaceID == workspaceID
	default:
		return 0, false
	}
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
			provider:    p,
		}, true, nil
	}
	return nil, false, nil
}

type mcpHandle struct {
	provider    *MCPProvider
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

func (h *mcpHandle) Availability(ctx context.Context, scope Scope) Availability {
	if h == nil || isNilInterface(h.exec) {
		return Unavailable(ReasonBackendNotExecutable)
	}
	if !isNilInterface(h.auth) {
		status, err := h.auth.Status(ctx, h.descriptor.Source, "")
		if err != nil {
			return Unavailable(ReasonBackendUnhealthy)
		}
		if reason, known := MCPAuthStatusReason(status); known && reason != ReasonMCPAuthUnconfigured {
			return Availability{
				Enabled: true, Available: true, Authorized: false, Executable: false,
				ReasonCodes: []ReasonCode{reason},
			}
		}
	}
	if h.reliability != nil && h.reliability.dead(ctx, h.descriptor.Source, scope.ProfileID) {
		return Unavailable(ReasonBackendDead)
	}
	if h.provider != nil {
		source := h.descriptor.Source
		source.RawToolName = ""
		source.Owner = mcpSourceServerName(source)
		if _, failed := h.provider.discoveryFailures.Load(source); failed {
			return Unavailable(ReasonBackendUnhealthy)
		}
	}
	return Available()
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
	legacyTool, legacyErr := canonicalMCPSegment("mcp.tool", rawTool)
	if legacyErr == nil {
		return validateMCPToolID(mcpNamespace + "__" + server + "__" + legacyTool)
	}
	if strings.TrimSpace(rawTool) == "" || !utf8.ValidString(rawTool) || strings.ContainsRune(rawTool, 0) {
		return "", legacyErr
	}
	// A fourth segment keeps encoded names disjoint from every previously assigned tool ID.
	prefix := mcpNamespace + "__" + server + "__" + mcpRawToolSegment + "__x"
	if len(prefix)+mcpRawToolEncoding.EncodedLen(len(rawTool)) > maxSegmentedIDLength {
		return "", mcpToolIDTooLong()
	}
	return validateMCPToolID(prefix + mcpRawToolEncoding.EncodeToString([]byte(rawTool)))
}

var mcpRawToolEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

func mcpToolIDTooLong() error {
	return NewValidationError("tool_id", ReasonIDTooLong, "mcp tool id exceeds 64 characters")
}

func validateMCPToolID(value string) (ToolID, error) {
	id := ToolID(value)
	if len(value) > maxSegmentedIDLength {
		return "", mcpToolIDTooLong()
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
		if status.TokenPresent && status.Refreshable {
			return "", false
		}
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
	rawTool := cmp.Or(desc.Source.RawToolName, desc.RawName)
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
	owner, err := MCPToolOwner(id)
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
	if mcpSource.ProfileID == "" {
		mcpSource.ProfileID = source.ProfileID
	}
	if mcpSource.Scope == "" {
		mcpSource.Scope = source.Scope
	}

	// destructiveHint wins over readOnlyHint: the tool is a write and must index as one.
	readOnly := desc.ReadOnly && !desc.Destructive
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
		ID:      id,
		Backend: BackendRef{Kind: BackendMCP, MCPServer: owner, MCPTool: rawTool},
		ToolPresentation: NewToolPresentation(
			displayTitle,
			strings.TrimSpace(desc.FriendlyVerb),
			strings.TrimSpace(desc.Preview),
		),
		Description:                 strings.TrimSpace(desc.Description),
		InputSchema:                 cloneRawMessage(desc.InputSchema),
		OutputSchema:                cloneRawMessage(desc.OutputSchema),
		Source:                      mcpSource,
		Visibility:                  VisibilityModel,
		Risk:                        risk,
		ReadOnly:                    readOnly,
		Destructive:                 desc.Destructive,
		OpenWorld:                   openWorld,
		ConcurrencySafe:             readOnly,
		mcpReadOnlyOverride:         desc.ReadOnlyOverride,
		mcpReadOnlyOperatorOverride: desc.ReadOnlyOverrideOrigin == MCPReadOnlyOverrideOperatorConfig,
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

// MCPToolOwner returns the canonical server segment of an MCP tool ID.
func MCPToolOwner(id ToolID) (string, error) {
	segments, err := id.Segments()
	if err != nil {
		return "", err
	}
	legacy := len(segments) == 3
	encoded := len(segments) == 4 && segments[2] == mcpRawToolSegment &&
		len(segments[3]) > 1 && strings.HasPrefix(segments[3], "x")
	if segments[0] != mcpNamespace || (!legacy && !encoded) {
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

func applyMCPReadAuthority(d *Descriptor, overrides map[string]bool) {
	approved := overrides[string(d.ID)]
	if !d.mcpReadOnlyOverride || (!d.mcpReadOnlyOperatorOverride && !approved) ||
		d.Backend.Kind != BackendMCP ||
		d.Destructive {
		return
	}
	d.ReadOnly, d.ConcurrencySafe, d.OpenWorld, d.Risk = true, true, false, RiskRead
}
