package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/deadentity"
	"github.com/compozy/agh/internal/store"
)

func TestShouldCanonicalizeMCPToolIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		server     string
		tool       string
		want       ToolID
		wantErr    bool
		wantReason ReasonCode
	}{
		{
			name:   "ShouldNormalizeSupportedRawNames",
			server: " Git-Hub ",
			tool:   "Search.Tool",
			want:   "mcp__git_hub__search_tool",
		},
		{
			name:       "ShouldRejectDigitStart",
			server:     "9github",
			tool:       "search",
			wantErr:    true,
			wantReason: ReasonIDInvalidFormat,
		},
		{
			name:       "ShouldRejectReservedSeparatorAfterNormalization",
			server:     "git--hub",
			tool:       "search",
			wantErr:    true,
			wantReason: ReasonIDReservedConflict,
		},
		{
			name:       "ShouldRejectUnsupportedCharacters",
			server:     "git hub",
			tool:       "search",
			wantErr:    true,
			wantReason: ReasonIDInvalidFormat,
		},
		{
			name:       "ShouldRejectOverLengthIDs",
			server:     "github",
			tool:       "tool_" + strings.Repeat("a", 64),
			wantErr:    true,
			wantReason: ReasonIDTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Canonicalize(tt.server, tt.tool)
			if tt.wantErr {
				requireReason(t, err, tt.wantReason)
				return
			}
			if err != nil {
				t.Fatalf("Canonicalize() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Canonicalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldMapMCPAuthStatusToRegistryReasons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status string
		want   ReasonCode
		mapped bool
	}{
		{status: "unconfigured", want: ReasonMCPAuthUnconfigured, mapped: true},
		{status: "needs_login", want: ReasonMCPAuthRequired, mapped: true},
		{status: "expired", want: ReasonMCPAuthExpired, mapped: true},
		{status: "invalid", want: ReasonMCPAuthInvalid, mapped: true},
		{status: "refresh_failed", want: ReasonMCPAuthRefreshFailed, mapped: true},
		{status: "authenticated", mapped: false},
		{status: "", mapped: false},
	}

	for _, tt := range tests {
		name := "Should Map " + tt.status
		if tt.status == "" {
			name = "Should Map Empty Status"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, mapped := MCPAuthStatusReason(MCPAuthStatus{Status: tt.status})
			if mapped != tt.mapped {
				t.Fatalf("MCPAuthStatusReason(%q) mapped = %v, want %v", tt.status, mapped, tt.mapped)
			}
			if got != tt.want {
				t.Fatalf("MCPAuthStatusReason(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestShouldExposeMCPProviderDescriptorsAndPreserveOutputSchema(t *testing.T) {
	t.Run("Should Expose MCP Provider Descriptors And Preserve Output Schema", func(t *testing.T) {
		t.Parallel()

		outputSchema := json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`)
		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:      "lookup",
			Title:        "Lookup",
			FriendlyVerb: "Looking up",
			Preview:      "arg:query",
			Description:  "Lookup data",
			InputSchema:  json.RawMessage(`{"type":"object","properties":{}}`),
			OutputSchema: outputSchema,
			ReadOnly:     true,
		}})
		provider := newTestMCPProvider(t, executor, []SourceRef{{
			Kind:          SourceMCP,
			Owner:         "github",
			RawServerName: "github",
		}})

		descriptors, err := provider.List(context.Background(), Scope{Operator: true})
		if err != nil {
			t.Fatalf("provider.List() error = %v", err)
		}
		if got, want := len(descriptors), 1; got != want {
			t.Fatalf("len(provider.List()) = %d, want %d", got, want)
		}
		descriptor := descriptors[0]
		if got, want := descriptor.ID, ToolID("mcp__github__lookup"); got != want {
			t.Fatalf("descriptor.ID = %q, want %q", got, want)
		}
		if got, want := string(descriptor.OutputSchema), string(outputSchema); got != want {
			t.Fatalf("descriptor.OutputSchema = %s, want %s", got, want)
		}
		if got, want := descriptor.Source.RawToolName, "lookup"; got != want {
			t.Fatalf("descriptor.Source.RawToolName = %q, want %q", got, want)
		}
		presentation := descriptor.Presentation()
		if got, want := presentation.FriendlyVerb, "Looking up"; got != want {
			t.Fatalf("descriptor presentation friendly verb = %q, want %q", got, want)
		}
		if got, want := presentation.Preview, "arg:query"; got != want {
			t.Fatalf("descriptor presentation preview = %q, want %q", got, want)
		}
		if !descriptor.ReadOnly || descriptor.Risk != RiskRead || descriptor.OpenWorld {
			t.Fatalf("descriptor risk flags = %#v, want read-only local-safe flags", descriptor)
		}
	})
}

func TestShouldFailClosedOnMCPSanitizedNameCollisions(t *testing.T) {
	t.Run("Should Fail Closed On MCP Sanitized Name Collisions", func(t *testing.T) {
		t.Parallel()

		executor := newFakeMCPExecutor([]MCPToolDescriptor{
			{
				RawName:     "foo-bar",
				InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			},
			{
				RawName:     "foo.bar",
				InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			},
		})
		provider := newTestMCPProvider(t, executor, []SourceRef{{
			Kind:          SourceMCP,
			Owner:         "github",
			RawServerName: "github",
		}})
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		views, err := registry.OperatorProjection(context.Background(), Scope{Operator: true})
		if err != nil {
			t.Fatalf("registry.OperatorProjection() error = %v", err)
		}
		if got, want := len(views), 1; got != want {
			t.Fatalf("len(views) = %d, want %d", got, want)
		}
		if !views[0].Availability.Conflicted {
			t.Fatalf("views[0].Availability.Conflicted = false, want true")
		}
		requireDecisionReason(t, views[0].Decision, ReasonConflictedSanitizedName)
	})
}

func TestShouldBlockMCPCallsWhenAuthIsRequired(t *testing.T) {
	t.Run("Should Block MCP Calls When Auth Is Required", func(t *testing.T) {
		t.Parallel()

		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:     "lookup",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}})
		executor.status = MCPAuthStatus{Status: "needs_login"}
		provider := newTestMCPProvider(t, executor, []SourceRef{{
			Kind:          SourceMCP,
			Owner:         "github",
			RawServerName: "github",
		}})
		registry := newMCPRegistry(t, provider)

		_, err := registry.Call(context.Background(), Scope{}, CallRequest{
			ToolID: "mcp__github__lookup",
			Input:  json.RawMessage(`{}`),
		})
		requireReason(t, err, ReasonMCPAuthRequired)
		if !errors.Is(err, ErrToolUnavailable) {
			t.Fatalf("registry.Call() error = %v, want ErrToolUnavailable", err)
		}
	})
}

func TestShouldSkipMCPSourceWhenAuthBlocksDiscovery(t *testing.T) {
	t.Run("Should Skip MCP Source When Auth Blocks Discovery", func(t *testing.T) {
		t.Parallel()

		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:     "lookup",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}})
		executor.listErr = NewToolError(
			ErrorCodeUnavailable,
			"mcp__github__lookup",
			"mcp login required",
			ErrToolUnavailable,
			ReasonMCPAuthRequired,
		)
		provider := newTestMCPProvider(t, executor, []SourceRef{{
			Kind:          SourceMCP,
			Owner:         "github",
			RawServerName: "github",
		}})

		descriptors, err := provider.List(context.Background(), Scope{Operator: true})
		if err != nil {
			t.Fatalf("provider.List() error = %v", err)
		}
		if len(descriptors) != 0 {
			t.Fatalf("provider.List() descriptors = %#v, want empty while auth blocks discovery", descriptors)
		}
	})
}

func TestShouldSkipMCPSourceWhenDiscoveryFails(t *testing.T) {
	t.Run("Should Skip MCP Source When Discovery Fails", func(t *testing.T) {
		t.Parallel()

		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:     "lookup",
			InputSchema: json.RawMessage("{\"type\":\"object\",\"properties\":{}}"),
		}})
		executor.listErr = errors.New("transport closed")
		provider := newTestMCPProvider(t, executor, []SourceRef{{
			Kind:          SourceMCP,
			Owner:         "broken",
			RawServerName: "broken",
		}})

		descriptors, err := provider.List(context.Background(), Scope{Operator: true})
		if err != nil {
			t.Fatalf("provider.List() error = %v", err)
		}
		if len(descriptors) != 0 {
			t.Fatalf("provider.List() descriptors = %#v, want empty while source discovery fails", descriptors)
		}
	})
}

func TestShouldCallMCPProviderThroughRegistry(t *testing.T) {
	t.Run("Should Call MCP Provider Through Registry", func(t *testing.T) {
		t.Parallel()

		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:     "lookup",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}})
		executor.status = MCPAuthStatus{Status: "authenticated"}
		provider := newTestMCPProvider(t, executor, []SourceRef{{
			Kind:          SourceMCP,
			Owner:         "github",
			RawServerName: "github",
		}})
		registry := newMCPRegistry(t, provider)

		result, err := registry.Call(context.Background(), Scope{}, CallRequest{
			ToolID: "mcp__github__lookup",
			Input:  json.RawMessage(`{"query":"octo"}`),
		})
		if err != nil {
			t.Fatalf("registry.Call() error = %v", err)
		}
		if got, want := result.Preview, "called lookup"; got != want {
			t.Fatalf("result.Preview = %q, want %q", got, want)
		}
		if got, want := executor.lastCall().RawToolName, "lookup"; got != want {
			t.Fatalf("executor.lastCall().RawToolName = %q, want %q", got, want)
		}
	})
}

func TestShouldRetainDeadMCPDescriptorsUntilOpportunisticRecovery(t *testing.T) {
	t.Run("Should Retain Dead MCP Descriptors Until Opportunistic Recovery", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		clock := &mcpReliabilityTestClock{now: time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)}
		deadStore := newMCPReliabilityStore()
		deadService := deadentity.New(deadStore, deadentity.WithClock(clock.Now))
		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:     "lookup",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}})
		source := SourceRef{
			Kind:          SourceMCP,
			Owner:         "github",
			RawServerName: "github",
			Scope:         mcpSourceScopeWorkspace,
			WorkspaceID:   "ws-dead-mcp",
		}
		scope := Scope{Operator: true, WorkspaceID: source.WorkspaceID}
		provider, err := NewMCPProvider(
			MCPSourceListerFunc(func(context.Context) ([]SourceRef, error) {
				return []SourceRef{source}, nil
			}),
			executor,
			executor,
			WithMCPDeadEntityService(deadService),
		)
		if err != nil {
			t.Fatalf("NewMCPProvider() error = %v", err)
		}
		if descriptors, err := provider.List(ctx, scope); err != nil || len(descriptors) != 1 {
			t.Fatalf("provider.List(initial) = %#v, %v, want one descriptor", descriptors, err)
		}

		executor.setListError(NewToolError(
			ErrorCodeUnavailable,
			"mcp__github__lookup",
			"mcp server configuration is invalid",
			ErrToolUnavailable,
			ReasonMCPUnreachable,
		))
		for attempt := 1; attempt <= deadentity.DefaultPermanentFailureThreshold; attempt++ {
			descriptors, err := provider.List(ctx, scope)
			if err != nil {
				t.Fatalf("provider.List(failure %d) error = %v", attempt, err)
			}
			if len(descriptors) != 1 {
				t.Fatalf("provider.List(failure %d) = %#v, want cached descriptor", attempt, descriptors)
			}
		}
		marked := deadStore.Marked()
		if len(marked) != 1 || marked[0].WorkspaceID != source.WorkspaceID || marked[0].EntityID != "github" {
			t.Fatalf("dead marks = %#v, want workspace-scoped github sidecar", marked)
		}

		handle, found, err := provider.Resolve(ctx, scope, "mcp__github__lookup")
		if err != nil {
			t.Fatalf("provider.Resolve(dead) error = %v", err)
		}
		if !found {
			t.Fatal("provider.Resolve(dead) found = false, want cached handle")
		}
		availability := handle.Availability(ctx, scope)
		if availability.Executable || !hasAnyReason(availability.ReasonCodes, ReasonBackendDead) {
			t.Fatalf("dead handle availability = %#v, want backend_dead", availability)
		}

		clock.Advance(deadentity.DefaultRecoveryInterval)
		executor.setListError(nil)
		descriptors, err := provider.List(ctx, scope)
		if err != nil {
			t.Fatalf("provider.List(recovery) error = %v", err)
		}
		if len(descriptors) != 1 {
			t.Fatalf("provider.List(recovery) = %#v, want rediscovered descriptor", descriptors)
		}
		handle, found, err = provider.Resolve(ctx, scope, "mcp__github__lookup")
		if err != nil || !found {
			t.Fatalf("provider.Resolve(recovered) = %v, %t, %v", handle, found, err)
		}
		if availability = handle.Availability(ctx, scope); !availability.Executable {
			t.Fatalf("recovered handle availability = %#v, want executable", availability)
		}
		if got := deadStore.Clears(); got != 1 {
			t.Fatalf("dead mark clears = %d, want one", got)
		}
	})

	t.Run("Should Never Persist Global MCP Sources", func(t *testing.T) {
		t.Parallel()

		deadStore := newMCPReliabilityStore()
		deadService := deadentity.New(deadStore)
		executor := newFakeMCPExecutor(nil)
		executor.setListError(NewToolError(
			ErrorCodeUnavailable,
			"",
			"mcp server is unavailable",
			ErrToolUnavailable,
			ReasonMCPUnreachable,
		))
		provider, err := NewMCPProvider(
			MCPSourceListerFunc(func(context.Context) ([]SourceRef, error) {
				return []SourceRef{{
					Kind:          SourceMCP,
					Owner:         "global",
					RawServerName: "global",
					Scope:         mcpSourceScopeGlobal,
				}}, nil
			}),
			executor,
			executor,
			WithMCPDeadEntityService(deadService),
		)
		if err != nil {
			t.Fatalf("NewMCPProvider() error = %v", err)
		}
		for attempt := 1; attempt <= deadentity.DefaultPermanentFailureThreshold+1; attempt++ {
			if _, err := provider.List(context.Background(), Scope{Operator: true}); err != nil {
				t.Fatalf("provider.List(global failure %d) error = %v", attempt, err)
			}
		}
		if got := len(deadStore.Marked()); got != 0 {
			t.Fatalf("global source marks = %d, want zero", got)
		}
	})

	t.Run("Should Drop Cached Descriptors After A Workspace Source Is Removed", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		deadService := deadentity.New(newMCPReliabilityStore())
		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:     "lookup",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}})
		source := SourceRef{
			Kind:          SourceMCP,
			Owner:         "removed",
			RawServerName: "removed",
			Scope:         mcpSourceScopeWorkspace,
			WorkspaceID:   "ws-cache-bound",
		}
		scope := Scope{Operator: true, WorkspaceID: source.WorkspaceID}
		active := []SourceRef{source}
		provider, err := NewMCPProvider(
			MCPSourceListerFunc(func(context.Context) ([]SourceRef, error) {
				return append([]SourceRef(nil), active...), nil
			}),
			executor,
			executor,
			WithMCPDeadEntityService(deadService),
		)
		if err != nil {
			t.Fatalf("NewMCPProvider() error = %v", err)
		}
		if descriptors, err := provider.List(ctx, scope); err != nil || len(descriptors) != 1 {
			t.Fatalf("provider.List(initial) = %#v, %v, want one descriptor", descriptors, err)
		}

		active = nil
		if descriptors, err := provider.List(ctx, scope); err != nil || len(descriptors) != 0 {
			t.Fatalf("provider.List(removed) = %#v, %v, want empty", descriptors, err)
		}
		executor.setListError(NewToolError(
			ErrorCodeUnavailable,
			"",
			"removed sidecar is unreachable",
			ErrToolUnavailable,
			ReasonMCPUnreachable,
		))
		active = []SourceRef{source}
		descriptors, err := provider.List(ctx, scope)
		if err != nil {
			t.Fatalf("provider.List(re-added failure) error = %v", err)
		}
		if len(descriptors) != 0 {
			t.Fatalf("provider.List(re-added failure) = %#v, want no stale cached descriptors", descriptors)
		}
	})

	t.Run("Should isolate cached descriptors across workspace projections", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		deadService := deadentity.New(newMCPReliabilityStore())
		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:     "lookup",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}})
		sources := []SourceRef{
			{
				Kind:          SourceMCP,
				Owner:         "github",
				RawServerName: "github",
				Scope:         mcpSourceScopeWorkspace,
				WorkspaceID:   "ws-cache-a",
			},
			{
				Kind:          SourceMCP,
				Owner:         "linear",
				RawServerName: "linear",
				Scope:         mcpSourceScopeWorkspace,
				WorkspaceID:   "ws-cache-b",
			},
		}
		provider, err := NewMCPProvider(
			MCPSourceListerFunc(func(context.Context) ([]SourceRef, error) {
				return append([]SourceRef(nil), sources...), nil
			}),
			executor,
			executor,
			WithMCPDeadEntityService(deadService),
		)
		if err != nil {
			t.Fatalf("NewMCPProvider() error = %v", err)
		}
		for _, workspaceID := range []string{"ws-cache-a", "ws-cache-b"} {
			descriptors, err := provider.List(ctx, Scope{Operator: true, WorkspaceID: workspaceID})
			if err != nil || len(descriptors) != 1 {
				t.Fatalf("provider.List(%s) = %#v, %v, want one descriptor", workspaceID, descriptors, err)
			}
		}

		executor.setListError(NewToolError(
			ErrorCodeUnavailable,
			"mcp__github__lookup",
			"github sidecar is unreachable",
			ErrToolUnavailable,
			ReasonMCPUnreachable,
		))
		descriptors, err := provider.List(ctx, Scope{Operator: true, WorkspaceID: "ws-cache-a"})
		if err != nil {
			t.Fatalf("provider.List(workspace A failure) error = %v", err)
		}
		if len(descriptors) != 1 {
			t.Fatalf("provider.List(workspace A failure) = %#v, want workspace A cached descriptor", descriptors)
		}
	})
}

func TestShouldIsolateMCPProviderRegistryByWorkspace(t *testing.T) {
	t.Run("Should Project Only Global And Current Workspace Sources", func(t *testing.T) {
		t.Parallel()

		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:     "lookup",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}})
		provider := newTestMCPProvider(t, executor, []SourceRef{
			{
				Kind:          SourceMCP,
				Owner:         "github",
				RawServerName: "github",
				Scope:         "global",
			},
			{
				Kind:          SourceMCP,
				Owner:         "linear",
				RawServerName: "linear",
				ResourceID:    "mcp-workspace-a",
				Scope:         "workspace",
				WorkspaceID:   "workspace-a",
			},
			{
				Kind:          SourceMCP,
				Owner:         "slack",
				RawServerName: "slack",
				ResourceID:    "mcp-workspace-b",
				Scope:         "workspace",
				WorkspaceID:   "workspace-b",
			},
		})
		registry := newMCPRegistry(t, provider)

		views, err := registry.OperatorProjection(context.Background(), Scope{
			Operator:    true,
			WorkspaceID: "workspace-a",
		})
		if err != nil {
			t.Fatalf("registry.OperatorProjection() error = %v", err)
		}
		if got, want := len(views), 2; got != want {
			t.Fatalf("len(registry.OperatorProjection()) = %d, want %d (%#v)", got, want, views)
		}
		for _, view := range views {
			if view.Descriptor.Source.WorkspaceID == "workspace-b" {
				t.Fatalf("registry.OperatorProjection() exposed foreign source = %#v", view.Descriptor.Source)
			}
		}
		if executor.listedResource("mcp-workspace-b") {
			t.Fatal("registry.OperatorProjection() probed the foreign workspace source")
		}
	})

	t.Run("Should Prefer The Current Workspace Source Over A Homonymous Global Source", func(t *testing.T) {
		t.Parallel()

		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:     "lookup",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}})
		provider := newTestMCPProvider(t, executor, []SourceRef{
			{
				Kind:          SourceMCP,
				Owner:         "linear",
				RawServerName: "linear",
				Scope:         "global",
			},
			{
				Kind:          SourceMCP,
				Owner:         "linear",
				RawServerName: "linear",
				ResourceID:    "mcp-workspace-a",
				Scope:         "workspace",
				WorkspaceID:   "workspace-a",
			},
		})
		registry := newMCPRegistry(t, provider)

		views, err := registry.OperatorProjection(context.Background(), Scope{
			Operator:    true,
			WorkspaceID: "workspace-a",
		})
		if err != nil {
			t.Fatalf("registry.OperatorProjection() error = %v", err)
		}
		if got, want := len(views), 1; got != want {
			t.Fatalf("len(registry.OperatorProjection()) = %d, want %d (%#v)", got, want, views)
		}
		if got, want := views[0].Descriptor.Source.WorkspaceID, "workspace-a"; got != want {
			t.Fatalf("registry.OperatorProjection() source workspace = %q, want %q", got, want)
		}
		if views[0].Availability.Conflicted {
			t.Fatalf(
				"registry.OperatorProjection() availability = %#v, want non-conflicted workspace override",
				views[0].Availability,
			)
		}
	})

	t.Run("Should Refuse An Authenticated Foreign Source When The Current Source Needs Login", func(t *testing.T) {
		t.Parallel()

		executor := newFakeMCPExecutor([]MCPToolDescriptor{{
			RawName:     "lookup",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}})
		executor.status = MCPAuthStatus{Status: "authenticated"}
		executor.listErrByResource = map[string]error{
			"mcp-workspace-a": NewToolError(
				ErrorCodeUnavailable,
				"mcp__linear__lookup",
				"mcp login required",
				ErrToolUnavailable,
				ReasonMCPAuthRequired,
			),
		}
		provider := newTestMCPProvider(t, executor, []SourceRef{
			{
				Kind:          SourceMCP,
				Owner:         "linear",
				RawServerName: "linear",
				ResourceID:    "mcp-workspace-a",
				Scope:         "workspace",
				WorkspaceID:   "workspace-a",
			},
			{
				Kind:          SourceMCP,
				Owner:         "linear",
				RawServerName: "linear",
				ResourceID:    "mcp-workspace-b",
				Scope:         "workspace",
				WorkspaceID:   "workspace-b",
			},
		})
		registry := newMCPRegistry(t, provider)

		_, err := registry.Call(context.Background(), Scope{WorkspaceID: "workspace-a"}, CallRequest{
			ToolID: "mcp__linear__lookup",
			Input:  json.RawMessage(`{}`),
		})
		requireReason(t, err, ReasonToolUnknown)
		if !errors.Is(err, ErrToolNotFound) {
			t.Fatalf("registry.Call() error = %v, want ErrToolNotFound", err)
		}
		if executor.listedResource("mcp-workspace-b") {
			t.Fatal("registry.Call() probed the authenticated foreign workspace source")
		}
	})
}

type fakeMCPExecutor struct {
	mu                sync.Mutex
	tools             []MCPToolDescriptor
	status            MCPAuthStatus
	listErr           error
	listErrByResource map[string]error
	listedSources     []SourceRef
	calls             []MCPToolCallRequest
}

func newFakeMCPExecutor(tools []MCPToolDescriptor) *fakeMCPExecutor {
	return &fakeMCPExecutor{
		tools:  tools,
		status: MCPAuthStatus{Status: "unconfigured"},
	}
}

func (f *fakeMCPExecutor) ListTools(_ context.Context, source SourceRef) ([]MCPToolDescriptor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.listedSources = append(f.listedSources, source)
	if err := f.listErrByResource[source.ResourceID]; err != nil {
		return nil, err
	}
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]MCPToolDescriptor, len(f.tools))
	for i := range f.tools {
		out[i] = f.tools[i]
		out[i].InputSchema = cloneRawMessage(f.tools[i].InputSchema)
		out[i].OutputSchema = cloneRawMessage(f.tools[i].OutputSchema)
	}
	return out, nil
}

func (f *fakeMCPExecutor) setListError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listErr = err
}

func (f *fakeMCPExecutor) CallTool(
	_ context.Context,
	_ SourceRef,
	req MCPToolCallRequest,
) (ToolResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, req)
	return ToolResult{
		Content: []ToolContent{{
			Type: "text",
			Text: "called " + req.RawToolName,
		}},
		Preview: "called " + req.RawToolName,
	}, nil
}

func (f *fakeMCPExecutor) Status(context.Context, SourceRef) (MCPAuthStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status, nil
}

func (f *fakeMCPExecutor) lastCall() MCPToolCallRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return MCPToolCallRequest{}
	}
	return f.calls[len(f.calls)-1]
}

func (f *fakeMCPExecutor) listedResource(resourceID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, source := range f.listedSources {
		if source.ResourceID == resourceID {
			return true
		}
	}
	return false
}

func newTestMCPProvider(t *testing.T, executor *fakeMCPExecutor, sources []SourceRef) *MCPProvider {
	t.Helper()

	provider, err := NewMCPProvider(
		MCPSourceListerFunc(func(context.Context) ([]SourceRef, error) {
			return sources, nil
		}),
		executor,
		executor,
	)
	if err != nil {
		t.Fatalf("NewMCPProvider() error = %v", err)
	}
	return provider
}

func newMCPRegistry(t *testing.T, provider Provider) *RuntimeRegistry {
	t.Helper()

	registry, err := NewRegistry(
		WithProviders(provider),
		WithPolicyInputs(PolicyInputs{
			SystemPermissionMode: PermissionModeApproveAll,
			ExternalDefault:      ExternalDefaultEnabled,
			ApprovalAvailable:    true,
		}, ToolsetCatalog{}),
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}

type mcpReliabilityStore struct {
	mu       sync.Mutex
	entities map[store.DeadEntityKey]store.DeadEntity
	marked   []store.DeadEntity
	clears   int
}

func newMCPReliabilityStore() *mcpReliabilityStore {
	return &mcpReliabilityStore{entities: make(map[store.DeadEntityKey]store.DeadEntity)}
}

func (s *mcpReliabilityStore) MarkDeadEntity(_ context.Context, entity store.DeadEntity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entities[entity.DeadEntityKey] = entity
	s.marked = append(s.marked, entity)
	return nil
}

func (s *mcpReliabilityStore) ClearDeadEntity(
	_ context.Context,
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clears++
	delete(s.entities, store.DeadEntityKey{WorkspaceID: workspaceID, Kind: kind, EntityID: entityID})
	return nil
}

func (s *mcpReliabilityStore) FindDeadEntity(
	_ context.Context,
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) (store.DeadEntity, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entity, ok := s.entities[store.DeadEntityKey{WorkspaceID: workspaceID, Kind: kind, EntityID: entityID}]
	return entity, ok, nil
}

func (s *mcpReliabilityStore) ListDeadEntities(
	_ context.Context,
	workspaceID string,
) ([]store.DeadEntity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entities := make([]store.DeadEntity, 0)
	for key, entity := range s.entities {
		if key.WorkspaceID == workspaceID {
			entities = append(entities, entity)
		}
	}
	return entities, nil
}

func (s *mcpReliabilityStore) Marked() []store.DeadEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.DeadEntity(nil), s.marked...)
}

func (s *mcpReliabilityStore) Clears() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clears
}

type mcpReliabilityTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *mcpReliabilityTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *mcpReliabilityTestClock) Advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(delta)
}
