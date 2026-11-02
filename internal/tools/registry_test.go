package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
)

type registryTestProvider struct {
	source      SourceRef
	descriptors []Descriptor
	list        func() []Descriptor
	listScoped  func(Scope) []Descriptor
	handles     map[ToolID]Handle
	resolveErr  map[ToolID]error
	deferred    bool
}

var _ Provider = registryTestProvider{}

type projectionAvailabilityTestProvider struct {
	registryTestProvider
	availability func(context.Context, Scope, []ToolID) map[ToolID]Availability
}

var _ ProjectionAvailabilityProvider = (*projectionAvailabilityTestProvider)(nil)

func (p *projectionAvailabilityTestProvider) ProjectionAvailability(
	ctx context.Context, scope Scope, ids []ToolID,
) map[ToolID]Availability {
	return p.availability(ctx, scope, ids)
}

func (p registryTestProvider) ID() SourceRef {
	return p.source
}

func (p registryTestProvider) DeferInitialDiscovery() bool {
	return p.deferred
}

func (p registryTestProvider) List(_ context.Context, scope Scope) ([]Descriptor, error) {
	if p.listScoped != nil {
		return append([]Descriptor(nil), p.listScoped(scope)...), nil
	}
	if p.list != nil {
		return append([]Descriptor(nil), p.list()...), nil
	}
	return append([]Descriptor(nil), p.descriptors...), nil
}

func (p registryTestProvider) Resolve(_ context.Context, _ Scope, id ToolID) (Handle, bool, error) {
	if err, ok := p.resolveErr[id]; ok {
		return nil, false, err
	}
	handle, ok := p.handles[id]
	return handle, ok, nil
}

type registryTestHandle struct {
	descriptor   Descriptor
	availability Availability
	result       ToolResult
	callErr      error
	call         func(context.Context, CallRequest) (ToolResult, error)
}

var _ Handle = (*registryTestHandle)(nil)

func (h *registryTestHandle) Descriptor() Descriptor {
	return h.descriptor
}

func (h *registryTestHandle) Availability(_ context.Context, _ Scope) Availability {
	return h.availability
}

func (h *registryTestHandle) Call(ctx context.Context, req CallRequest) (ToolResult, error) {
	if h.call != nil {
		return h.call(ctx, req)
	}
	if h.callErr != nil {
		return ToolResult{}, h.callErr
	}
	if len(h.result.Content) > 0 || len(h.result.Structured) > 0 || h.result.Preview != "" {
		return h.result, nil
	}
	return ToolResult{Content: []ToolContent{{Type: "text", Text: "ok"}}}, nil
}

type staticPolicyEvaluator struct {
	decision EffectiveToolDecision
}

var _ PolicyEvaluator = staticPolicyEvaluator{}

func (e staticPolicyEvaluator) Evaluate(_ context.Context, _ Scope, _ Descriptor) (EffectiveToolDecision, error) {
	return e.decision, nil
}

func TestRuntimeRegistryIndexingAndCollisions(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("Should reject invalid descriptors introduced by a dynamic provider reindex", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		provider := registryTestProvider{
			source: descriptor.Source,
			list: func() []Descriptor {
				return []Descriptor{descriptor}
			},
		}
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		if _, err := registry.DiagnosticProjection(t.Context(), Scope{}); err != nil {
			t.Fatalf("DiagnosticProjection(initial) error = %v", err)
		}
		descriptor.InputSchema = json.RawMessage(`{"type":false}`)
		_, err = registry.DiagnosticProjection(t.Context(), Scope{})
		requireReason(t, err, ReasonSchemaInvalid)
	})

	t.Run("Should recompute native conflicts and ordering when dynamic providers change", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		provider, err := NewNativeProvider(descriptor.Source, NativeTool{
			Descriptor: descriptor,
			Call: func(context.Context, Scope, CallRequest) (ToolResult, error) {
				return ToolResult{}, nil
			},
		})
		if err != nil {
			t.Fatalf("NewNativeProvider() error = %v", err)
		}
		first := descriptorWithID(ToolIDTaskRead, "Read Task")
		last := descriptorWithID(ToolIDToolList, "List Tools")
		descriptors := []Descriptor{last, descriptor, first}
		dynamic := providerWithDescriptors(descriptor.Source, descriptors...)
		dynamic.list = func() []Descriptor { return descriptors }
		registry, err := NewRegistry(WithProviders(provider, dynamic))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		views, err := registry.OperatorProjection(t.Context(), Scope{Operator: true})
		if err != nil {
			t.Fatalf("OperatorProjection(conflicted) error = %v", err)
		}
		requireToolIDs(t, views, descriptor.ID, first.ID, last.ID)
		requireViewReason(t, views, descriptor.ID, ReasonConflictedID)
		descriptors = []Descriptor{last, first}
		views, err = registry.SessionProjection(t.Context(), Scope{})
		if err != nil {
			t.Fatalf("SessionProjection(recovered) error = %v", err)
		}
		ids := make([]ToolID, len(views))
		for i := range views {
			ids[i] = views[i].Descriptor.ID
		}
		if want := []ToolID{descriptor.ID, first.ID, last.ID}; !slices.Equal(ids, want) {
			t.Fatalf("SessionProjection(recovered) IDs = %#v, want ordered IDs %#v", ids, want)
		}
	})

	t.Run("Should mark duplicate canonical IDs as conflicted and hide from session projection", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		provider := providerWithDescriptors(SourceRef{Kind: SourceBuiltin, Owner: "daemon"}, descriptor, descriptor)
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		operatorViews, err := registry.OperatorProjection(ctx, Scope{Operator: true})
		if err != nil {
			t.Fatalf("OperatorProjection() error = %v", err)
		}
		if got, want := len(operatorViews), 1; got != want {
			t.Fatalf("len(operatorViews) = %d, want %d", got, want)
		}
		if !operatorViews[0].Availability.Conflicted {
			t.Fatalf("operator view availability = %#v, want conflicted", operatorViews[0].Availability)
		}
		requireDecisionReason(t, operatorViews[0].Decision, ReasonConflictedID)
		sessionViews, err := registry.SessionProjection(ctx, Scope{})
		if err != nil {
			t.Fatalf("SessionProjection() error = %v", err)
		}
		if len(sessionViews) != 0 {
			t.Fatalf("sessionViews = %#v, want empty", sessionViews)
		}
	})

	t.Run("Should replace descriptor metadata after a provider reindex", func(t *testing.T) {
		t.Parallel()

		first := validDescriptor()
		first.ToolPresentation = NewToolPresentation(first.DisplayTitle, "Reading skill", "arg:name")
		second := validDescriptor()
		second.ID = "compozy__skill_search"
		second.Backend.NativeName = "skill_search"
		second.ToolPresentation = NewToolPresentation(second.DisplayTitle, "Searching skills", "query")
		descriptors := []Descriptor{first, second}
		provider := registryTestProvider{
			source: first.Source,
			list: func() []Descriptor {
				return descriptors
			},
		}
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		if _, err := registry.DiagnosticProjection(ctx, Scope{Operator: true}); err != nil {
			t.Fatalf("DiagnosticProjection(initial) error = %v", err)
		}
		metadata, ok := registry.LookupToolMetadata("", second.ID.String())
		if !ok {
			t.Fatal("LookupToolMetadata(initial) ok = false, want true")
		}
		secondPresentation := second.Presentation()
		if got, want := metadata.FriendlyVerb, secondPresentation.FriendlyVerb; got != want {
			t.Fatalf("LookupToolMetadata(initial).FriendlyVerb = %q, want %q", got, want)
		}

		descriptors = []Descriptor{first}
		if _, err := registry.DiagnosticProjection(ctx, Scope{Operator: true}); err != nil {
			t.Fatalf("DiagnosticProjection(reindex) error = %v", err)
		}
		if metadata, ok := registry.LookupToolMetadata("", second.ID.String()); ok {
			t.Fatalf("LookupToolMetadata(removed) = %#v, true; want zero, false", metadata)
		}
	})

	t.Run("Should isolate descriptor metadata across concurrent workspace reindexes", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		removedFromWorkspaceA := false
		provider := registryTestProvider{
			source: descriptor.Source,
			listScoped: func(scope Scope) []Descriptor {
				scoped := cloneDescriptor(descriptor)
				switch scope.WorkspaceID {
				case "":
					scoped.ToolPresentation = NewToolPresentation(scoped.DisplayTitle, "Global lookup", "arg:query")
					scoped.Source.Scope = "global"
				case "ws-a":
					if removedFromWorkspaceA {
						return nil
					}
					scoped.ToolPresentation = NewToolPresentation(scoped.DisplayTitle, "Workspace A lookup", "arg:a")
					scoped.Source.Scope = "workspace"
					scoped.Source.WorkspaceID = "ws-a"
				case "ws-b":
					scoped.ToolPresentation = NewToolPresentation(scoped.DisplayTitle, "Workspace B lookup", "arg:b")
					scoped.Source.Scope = "workspace"
					scoped.Source.WorkspaceID = "ws-b"
				default:
					return nil
				}
				return []Descriptor{scoped}
			},
		}
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		if _, err := registry.DiagnosticProjection(ctx, Scope{Operator: true}); err != nil {
			t.Fatalf("DiagnosticProjection(global) error = %v", err)
		}

		type projectionResult struct {
			workspaceID string
			err         error
		}
		start := make(chan struct{})
		results := make(chan projectionResult, 2)
		for _, scope := range []Scope{{WorkspaceID: "ws-a"}, {WorkspaceID: "ws-b"}} {
			go func() {
				<-start
				_, projectionErr := registry.DiagnosticProjection(ctx, scope)
				results <- projectionResult{workspaceID: scope.WorkspaceID, err: projectionErr}
			}()
		}
		close(start)
		for range 2 {
			result := <-results
			if result.err != nil {
				t.Fatalf("DiagnosticProjection(%s) error = %v", result.workspaceID, result.err)
			}
		}

		for _, expected := range []struct {
			workspaceID  string
			friendlyVerb string
		}{
			{workspaceID: "ws-a", friendlyVerb: "Workspace A lookup"},
			{workspaceID: "ws-b", friendlyVerb: "Workspace B lookup"},
			{workspaceID: "ws-c", friendlyVerb: "Global lookup"},
		} {
			metadata, ok := registry.LookupToolMetadata(expected.workspaceID, descriptor.ID.String())
			if !ok {
				t.Fatalf("LookupToolMetadata(%s) ok = false, want true", expected.workspaceID)
			}
			if got := metadata.FriendlyVerb; got != expected.friendlyVerb {
				t.Fatalf(
					"LookupToolMetadata(%s).FriendlyVerb = %q, want %q",
					expected.workspaceID,
					got,
					expected.friendlyVerb,
				)
			}
		}

		removedFromWorkspaceA = true
		if _, err := registry.DiagnosticProjection(ctx, Scope{WorkspaceID: "ws-a"}); err != nil {
			t.Fatalf("DiagnosticProjection(ws-a removal) error = %v", err)
		}
		if metadata, ok := registry.LookupToolMetadata("ws-a", descriptor.ID.String()); ok {
			t.Fatalf("LookupToolMetadata(ws-a removed) = %#v, true; want zero, false", metadata)
		}
		metadata, ok := registry.LookupToolMetadata("ws-b", descriptor.ID.String())
		if !ok || metadata.FriendlyVerb != "Workspace B lookup" {
			t.Fatalf("LookupToolMetadata(ws-b after ws-a removal) = %#v, %v", metadata, ok)
		}
	})

	t.Run("Should exclude workspace metadata from the global fallback", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		descriptor.ToolPresentation = NewToolPresentation(
			descriptor.DisplayTitle,
			"Workspace A secret lookup",
			"arg:private",
		)
		descriptor.Source.Scope = "workspace"
		descriptor.Source.WorkspaceID = "ws-a"
		registry, err := NewRegistry(WithProviders(registryTestProvider{
			source:      descriptor.Source,
			descriptors: []Descriptor{descriptor},
		}))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		if _, err := registry.DiagnosticProjection(ctx, Scope{Operator: true}); err != nil {
			t.Fatalf("DiagnosticProjection(global) error = %v", err)
		}
		for _, workspaceID := range []string{"ws-a", "ws-b"} {
			if metadata, ok := registry.LookupToolMetadata(workspaceID, descriptor.ID.String()); ok {
				t.Fatalf(
					"LookupToolMetadata(%s before scoped index) = %#v, true; want zero, false",
					workspaceID,
					metadata,
				)
			}
		}

		if _, err := registry.DiagnosticProjection(ctx, Scope{WorkspaceID: "ws-a"}); err != nil {
			t.Fatalf("DiagnosticProjection(ws-a) error = %v", err)
		}
		metadata, ok := registry.LookupToolMetadata("ws-a", descriptor.ID.String())
		if !ok || metadata.FriendlyVerb != "Workspace A secret lookup" {
			t.Fatalf("LookupToolMetadata(ws-a after scoped index) = %#v, %v", metadata, ok)
		}
		if metadata, ok := registry.LookupToolMetadata("ws-b", descriptor.ID.String()); ok {
			t.Fatalf("LookupToolMetadata(ws-b) = %#v, true; want zero, false", metadata)
		}
	})

	t.Run("Should mark sanitized external name collisions with specific reason code", func(t *testing.T) {
		t.Parallel()

		first := mcpDescriptor("mcp__github__create_issue", "github", "create issue")
		second := mcpDescriptor("mcp__github__create_issue", "github", "create-issue")
		provider := providerWithDescriptors(first.Source, first, second)
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		views, err := registry.OperatorProjection(ctx, Scope{Operator: true})
		if err != nil {
			t.Fatalf("OperatorProjection() error = %v", err)
		}
		if got, want := len(views), 1; got != want {
			t.Fatalf("len(views) = %d, want %d", got, want)
		}
		if !slices.Contains(views[0].Availability.ReasonCodes, ReasonConflictedSanitizedName) {
			t.Fatalf("availability reasons = %#v, want sanitized collision", views[0].Availability.ReasonCodes)
		}
	})

	t.Run("Should fail closed on over length canonical IDs", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		descriptor.ID = ToolID("compozy__" + longASCII(65))
		provider := providerWithDescriptors(SourceRef{Kind: SourceBuiltin, Owner: "daemon"}, descriptor)
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		_, err = registry.OperatorProjection(ctx, Scope{Operator: true})
		requireReason(t, err, ReasonIDTooLong)
	})
}

func TestRuntimeRegistryProjections(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should batch only allowed tools by provider instance and refresh availability on every projection",
		func(t *testing.T) {
			t.Parallel()

			alpha := descriptorWithID(ToolIDTaskRead, "Read task")
			beta := descriptorWithID(ToolIDToolList, "List tools")
			hidden := validDescriptor()
			firstCalls, secondCalls := 0, 0
			available := false
			first := &projectionAvailabilityTestProvider{
				registryTestProvider: providerWithDescriptors(alpha.Source, alpha, hidden),
				availability: func(_ context.Context, scope Scope, ids []ToolID) map[ToolID]Availability {
					firstCalls++
					if scope.SessionID != "session-batch" || !slices.Equal(ids, []ToolID{alpha.ID}) {
						t.Fatalf("first provider batch scope = %#v, IDs = %#v", scope, ids)
					}
					return map[ToolID]Availability{alpha.ID: Available()}
				},
			}
			second := &projectionAvailabilityTestProvider{
				registryTestProvider: providerWithDescriptors(alpha.Source, beta),
				availability: func(_ context.Context, _ Scope, ids []ToolID) map[ToolID]Availability {
					secondCalls++
					if !slices.Equal(ids, []ToolID{beta.ID}) {
						t.Fatalf("second provider batch IDs = %#v", ids)
					}
					if !available {
						return nil
					}
					return map[ToolID]Availability{beta.ID: Available()}
				},
			}
			inputs := DefaultPolicyInputs()
			inputs.Session = SessionToolPolicy{Enforced: true, Tools: []ToolID{alpha.ID, beta.ID}}
			registry, err := NewRegistry(WithProviders(first, second), WithPolicyInputs(inputs, ToolsetCatalog{}))
			if err != nil {
				t.Fatal(err)
			}
			scope := Scope{SessionID: "session-batch"}
			views, err := registry.SessionProjection(t.Context(), scope)
			if err != nil {
				t.Fatal(err)
			}
			requireToolIDs(t, views, alpha.ID)
			available = true
			views, err = registry.SessionProjection(t.Context(), scope)
			if err != nil {
				t.Fatal(err)
			}
			requireToolIDs(t, views, alpha.ID, beta.ID)
			if firstCalls != 2 || secondCalls != 2 {
				t.Fatalf("provider batch calls = %d, %d; want 2 each", firstCalls, secondCalls)
			}
			views, err = registry.OperatorProjection(t.Context(), Scope{Operator: true})
			if err != nil {
				t.Fatal(err)
			}
			requireToolIDs(t, views, alpha.ID, beta.ID, hidden.ID)
			if firstCalls != 2 || secondCalls != 2 {
				t.Fatal("operator diagnostics used session-only availability batches")
			}
		},
	)

	t.Run("Should avoid probing denied backends while retaining operator diagnostics", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		probes := 0
		provider, err := NewNativeProvider(descriptor.Source, NativeTool{
			Descriptor: descriptor,
			Availability: func(context.Context, Scope) Availability {
				probes++
				return Unavailable(ReasonBackendUnhealthy)
			},
			Call: func(context.Context, Scope, CallRequest) (ToolResult, error) {
				return ToolResult{}, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		inputs := DefaultPolicyInputs()
		inputs.Session.Enforced = true
		registry, err := NewRegistry(WithProviders(provider), WithPolicyInputs(inputs, ToolsetCatalog{}))
		if err != nil {
			t.Fatal(err)
		}
		views, err := registry.SessionProjection(t.Context(), Scope{})
		if err != nil || len(views) != 0 || probes != 0 {
			t.Fatalf("denied session projection = %#v, probes = %d, error = %v", views, probes, err)
		}
		views, err = registry.OperatorProjection(t.Context(), Scope{Operator: true})
		if err != nil || probes != 1 {
			t.Fatalf("operator projection probes = %d, error = %v", probes, err)
		}
		requireViewReason(t, views, descriptor.ID, ReasonSessionDenied)
		requireViewReason(t, views, descriptor.ID, ReasonBackendUnhealthy)
	})

	t.Run("Should keep operator diagnostics broader than session projection", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		skillView := validDescriptor()
		taskUpdate := validDescriptor()
		taskUpdate.ID = "compozy__task_update"
		taskUpdate.ReadOnly = false
		taskUpdate.Risk = RiskMutating
		terminalExec := validDescriptor()
		terminalExec.ID = ToolIDTerminalExec
		terminalExec.ReadOnly = false
		terminalExec.OpenWorld = true
		terminalExec.Risk = RiskOpenWorld
		conflicted := mcpDescriptor("mcp__github__search", "github", "search")
		conflictedDuplicate := mcpDescriptor("mcp__github__search", "github", "Search")
		provider := providerWithDescriptors(
			SourceRef{Kind: SourceBuiltin, Owner: "daemon"},
			skillView,
			taskUpdate,
			terminalExec,
			conflicted,
			conflictedDuplicate,
		)
		provider.handles[terminalExec.ID] = &registryTestHandle{
			descriptor: terminalExec,
			availability: Availability{
				Registered:  true,
				Enabled:     true,
				ReasonCodes: []ReasonCode{ReasonBackendUnhealthy},
			},
		}
		denyTaskPattern, err := ParseToolPattern("compozy__task_*")
		if err != nil {
			t.Fatalf("ParseToolPattern() error = %v", err)
		}
		registry, err := NewRegistry(
			WithProviders(provider),
			WithPolicyInputs(PolicyInputs{
				SystemPermissionMode: PermissionModeApproveAll,
				ExternalDefault:      ExternalDefaultEnabled,
				DenyTools:            []ToolPattern{denyTaskPattern},
			}, ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		operatorViews, err := registry.OperatorProjection(ctx, Scope{Operator: true})
		if err != nil {
			t.Fatalf("OperatorProjection() error = %v", err)
		}
		if got, want := len(operatorViews), 4; got != want {
			t.Fatalf("len(operatorViews) = %d, want %d", got, want)
		}
		requireViewReason(t, operatorViews, taskUpdate.ID, ReasonPolicyDenied)
		requireViewReason(t, operatorViews, terminalExec.ID, ReasonBackendUnhealthy)
		requireViewReason(t, operatorViews, conflicted.ID, ReasonConflictedSanitizedName)

		sessionViews, err := registry.SessionProjection(ctx, Scope{})
		if err != nil {
			t.Fatalf("SessionProjection() error = %v", err)
		}
		if got, want := len(sessionViews), 1; got != want {
			t.Fatalf("len(sessionViews) = %d, want %d: %#v", got, want, sessionViews)
		}
		if sessionViews[0].Descriptor.ID != skillView.ID {
			t.Fatalf("session tool = %q, want %q", sessionViews[0].Descriptor.ID, skillView.ID)
		}
	})

	t.Run("Should defer remote discovery from the bootstrap projection", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		local := descriptorWithID(ToolIDTaskRunClaimNext, "Claim Task Run", ToolsetIDAutonomy)
		secondLocal := descriptorWithID(ToolIDTaskRead, "Read Task", ToolsetIDTasks)
		remote := mcpDescriptor("mcp__github__search", "github", "search")
		localProvider := providerWithDescriptors(
			SourceRef{Kind: SourceBuiltin, Owner: "daemon"},
			local,
			secondLocal,
		)
		remoteProvider := providerWithDescriptors(SourceRef{Kind: SourceDynamic, Owner: "mcp"}, remote)
		remoteListCalls := 0
		remoteProvider.deferred = true
		remoteProvider.list = func() []Descriptor {
			remoteListCalls++
			return []Descriptor{remote}
		}
		registry, err := NewRegistry(
			WithProviders(localProvider, remoteProvider),
			WithPolicyInputs(PolicyInputs{
				SystemPermissionMode: PermissionModeApproveAll,
				ExternalDefault:      ExternalDefaultEnabled,
			}, ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		bootstrap, err := registry.BootstrapSessionProjection(ctx, Scope{})
		if err != nil {
			t.Fatalf("BootstrapSessionProjection() error = %v", err)
		}
		requireToolIDs(t, bootstrap, local.ID, secondLocal.ID)
		if remoteListCalls != 0 {
			t.Fatalf("deferred provider list calls = %d, want 0 during bootstrap", remoteListCalls)
		}
		localHandle, ok := localProvider.handles[secondLocal.ID].(*registryTestHandle)
		if !ok {
			t.Fatalf("local provider handle = %T, want *registryTestHandle", localProvider.handles[secondLocal.ID])
		}
		localHandle.result = ToolResult{Content: []ToolContent{{Type: "text", Text: "read"}}}
		result, err := registry.BootstrapSessionCall(ctx, Scope{}, CallRequest{ToolID: secondLocal.ID})
		if err != nil {
			t.Fatalf("BootstrapSessionCall() error = %v", err)
		}
		if len(result.Content) != 1 || result.Content[0].Text != "read" {
			t.Fatalf("BootstrapSessionCall() result = %#v, want read text", result)
		}
		if got, want := remoteListCalls, 1; got != want {
			t.Fatalf("deferred provider list calls = %d, want %d during bootstrap call", got, want)
		}

		full, err := registry.SessionProjection(ctx, Scope{})
		if err != nil {
			t.Fatalf("SessionProjection() error = %v", err)
		}
		requireToolIDs(t, full, local.ID, secondLocal.ID, remote.ID)
		if remoteListCalls == 0 {
			t.Fatal("deferred provider list calls = 0, want discovery during full projection")
		}
	})

	t.Run("Should discover a deferred tool required by the agent policy during bootstrap", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		local := descriptorWithID(ToolIDTaskRead, "Read Task", ToolsetIDTasks)
		remote := mcpDescriptor("mcp__github__search", "github", "search")
		remotePattern, err := ParseToolPattern(remote.ID.String())
		if err != nil {
			t.Fatalf("ParseToolPattern() error = %v", err)
		}
		remoteProvider := providerWithDescriptors(SourceRef{Kind: SourceDynamic, Owner: "mcp"}, remote)
		remoteProvider.deferred = true
		remoteListCalls := 0
		remoteProvider.list = func() []Descriptor {
			remoteListCalls++
			return []Descriptor{remote}
		}
		registry, err := NewRegistry(
			WithProviders(
				providerWithDescriptors(SourceRef{Kind: SourceBuiltin, Owner: "daemon"}, local),
				remoteProvider,
			),
			WithPolicyInputs(PolicyInputs{
				SystemPermissionMode: PermissionModeApproveAll,
				ExternalDefault:      ExternalDefaultEnabled,
				Agent: AgentToolPolicy{
					Tools: []ToolPattern{remotePattern},
				},
			}, ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		bootstrap, err := registry.BootstrapSessionProjection(ctx, Scope{})
		if err != nil {
			t.Fatalf("BootstrapSessionProjection() error = %v", err)
		}
		requireToolIDs(t, bootstrap, remote.ID)
		if got, want := remoteListCalls, 1; got != want {
			t.Fatalf("deferred provider list calls = %d, want %d during required bootstrap", got, want)
		}
	})

	t.Run("Should retain deferred descriptor metadata after a bootstrap projection", func(t *testing.T) {
		t.Parallel()

		local := descriptorWithID(ToolIDTaskRead, "Read Task", ToolsetIDTasks)
		remote := mcpDescriptor("mcp__github__search", "github", "search")
		remote.ToolPresentation = NewToolPresentation(remote.DisplayTitle, "Searching GitHub", "query")
		remoteProvider := providerWithDescriptors(SourceRef{Kind: SourceDynamic, Owner: "mcp"}, remote)
		remoteProvider.deferred = true
		registry, err := NewRegistry(
			WithProviders(
				providerWithDescriptors(SourceRef{Kind: SourceBuiltin, Owner: "daemon"}, local),
				remoteProvider,
			),
			WithPolicyInputs(PolicyInputs{
				SystemPermissionMode: PermissionModeApproveAll,
				ExternalDefault:      ExternalDefaultEnabled,
			}, ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		if _, err := registry.SessionProjection(t.Context(), Scope{}); err != nil {
			t.Fatalf("SessionProjection() error = %v", err)
		}
		if _, ok := registry.LookupToolMetadata("", remote.ID.String()); !ok {
			t.Fatal("LookupToolMetadata(remote before bootstrap) ok = false, want true")
		}
		if _, err := registry.BootstrapSessionProjection(t.Context(), Scope{}); err != nil {
			t.Fatalf("BootstrapSessionProjection() error = %v", err)
		}
		if _, ok := registry.LookupToolMetadata("", remote.ID.String()); !ok {
			t.Fatal("LookupToolMetadata(remote after bootstrap) ok = false, want true")
		}
	})

	t.Run("Should reject a bootstrap call when deferred discovery conflicts with its tool ID", func(t *testing.T) {
		t.Parallel()

		local := descriptorWithID(ToolIDTaskRead, "Read Task", ToolsetIDTasks)
		localCalled := false
		localProvider := providerWithDescriptors(SourceRef{Kind: SourceBuiltin, Owner: "daemon"}, local)
		localProvider.handles[local.ID] = &registryTestHandle{
			descriptor:   local,
			availability: availableDispatchHandle(),
			call: func(context.Context, CallRequest) (ToolResult, error) {
				localCalled = true
				return ToolResult{Content: []ToolContent{{Type: "text", Text: "unexpected"}}}, nil
			},
		}
		remote := mcpDescriptor(local.ID, "github", "read")
		remoteProvider := providerWithDescriptors(SourceRef{Kind: SourceDynamic, Owner: "mcp"}, remote)
		remoteProvider.deferred = true
		remoteListCalls := 0
		remoteProvider.list = func() []Descriptor {
			remoteListCalls++
			return []Descriptor{remote}
		}
		registry, err := NewRegistry(
			WithProviders(localProvider, remoteProvider),
			WithPolicyInputs(PolicyInputs{
				SystemPermissionMode: PermissionModeApproveAll,
				ExternalDefault:      ExternalDefaultEnabled,
			}, ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		_, err = registry.BootstrapSessionCall(t.Context(), Scope{}, CallRequest{ToolID: local.ID})
		if !errors.Is(err, ErrToolConflict) {
			t.Fatalf("BootstrapSessionCall() error = %v, want ErrToolConflict", err)
		}
		if localCalled {
			t.Fatal("local provider handle was called for conflicted tool")
		}
		if got, want := remoteListCalls, 1; got != want {
			t.Fatalf("deferred provider list calls = %d, want %d for authoritative conflict detection", got, want)
		}
	})
}

func TestRuntimeRegistrySearchGetAndCustomEvaluator(t *testing.T) {
	t.Parallel()

	t.Run("Should search and get through a custom evaluator", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		skillView := validDescriptor()
		skillView.Tags = []string{"skills"}
		taskRead := validDescriptor()
		taskRead.ID = "compozy__task_read"
		taskRead.DisplayTitle = "Read Task"
		taskRead.Description = "Read task details"
		taskRead.Tags = []string{"tasks"}
		provider := providerWithDescriptors(SourceRef{Kind: SourceBuiltin, Owner: "daemon"}, skillView, taskRead)
		registry, err := NewRegistry(
			WithProviders(provider),
			WithPolicyEvaluator(staticPolicyEvaluator{decision: EffectiveToolDecision{
				VisibleToOperator:    true,
				VisibleToSession:     true,
				Callable:             true,
				SystemPermissionMode: string(PermissionModeApproveAll),
			}}),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		operatorList, err := registry.List(ctx, Scope{Operator: true})
		if err != nil {
			t.Fatalf("List(operator) error = %v", err)
		}
		if got, want := len(operatorList), 2; got != want {
			t.Fatalf("len(operatorList) = %d, want %d", got, want)
		}
		searchResults, err := registry.Search(ctx, Scope{}, SearchQuery{Query: "task", Limit: 1})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if got, want := len(searchResults), 1; got != want {
			t.Fatalf("len(searchResults) = %d, want %d", got, want)
		}
		if searchResults[0].Descriptor.ID != taskRead.ID {
			t.Fatalf("search result = %q, want %q", searchResults[0].Descriptor.ID, taskRead.ID)
		}
		got, err := registry.Get(ctx, Scope{}, skillView.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.Descriptor.ID != skillView.ID {
			t.Fatalf("Get() = %q, want %q", got.Descriptor.ID, skillView.ID)
		}
	})

	t.Run("Should point-evaluate DiagnosticGet with the projection row's conflicts and denials", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		allowed := validDescriptor()
		denied := validDescriptor()
		denied.ID = "compozy__task_read"
		conflicted := validDescriptor()
		conflicted.ID = "compozy__task_update"
		builtin := SourceRef{Kind: SourceBuiltin, Owner: "daemon"}
		first := providerWithDescriptors(builtin, allowed, denied, conflicted)
		second := providerWithDescriptors(SourceRef{Kind: SourceBuiltin, Owner: "other"}, conflicted)
		evaluator := &countingPolicyEvaluator{deny: denied.ID}
		registry, err := NewRegistry(WithProviders(first, second), WithPolicyEvaluator(evaluator))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		projection, err := registry.DiagnosticProjection(ctx, Scope{})
		if err != nil {
			t.Fatalf("DiagnosticProjection() error = %v", err)
		}
		for i := range projection {
			want := projection[i]
			before := evaluator.calls.Load()
			got, err := registry.DiagnosticGet(ctx, Scope{}, want.Descriptor.ID)
			if err != nil {
				t.Fatalf("DiagnosticGet(%s) error = %v", want.Descriptor.ID, err)
			}
			if calls := evaluator.calls.Load() - before; calls != 1 {
				t.Fatalf("DiagnosticGet(%s) policy evaluations = %d, want 1", want.Descriptor.ID, calls)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("DiagnosticGet(%s) = %#v, want projection row %#v", want.Descriptor.ID, got, want)
			}
		}
		requireViewReason(t, projection, denied.ID, ReasonPolicyDenied)
		conflictView, err := registry.DiagnosticGet(ctx, Scope{}, conflicted.ID)
		if err != nil || !conflictView.Availability.Conflicted {
			t.Fatalf("DiagnosticGet(conflicted) = %#v, %v; want conflicted availability", conflictView, err)
		}
		_, err = registry.DiagnosticGet(ctx, Scope{}, "compozy__missing")
		requireReason(t, err, ReasonToolUnknown)
	})
}

type countingPolicyEvaluator struct {
	deny  ToolID
	calls atomic.Int64
}

func (e *countingPolicyEvaluator) Evaluate(_ context.Context, _ Scope, d Descriptor) (EffectiveToolDecision, error) {
	e.calls.Add(1)
	if d.ID == e.deny {
		return EffectiveToolDecision{
			VisibleToOperator: true,
			ReasonCodes:       []ReasonCode{ReasonPolicyDenied},
		}, nil
	}
	return EffectiveToolDecision{
		VisibleToOperator:    true,
		VisibleToSession:     true,
		Callable:             true,
		SystemPermissionMode: string(PermissionModeApproveAll),
	}, nil
}

func TestRuntimeRegistryDynamicPolicyResolver(t *testing.T) {
	ctx := t.Context()
	toolList := descriptorWithID(ToolIDToolList, "Tool List", ToolsetIDBootstrap)
	toolInfo := descriptorWithID(ToolIDToolInfo, "Tool Info", ToolsetIDBootstrap)
	skillView := descriptorWithID(ToolIDSkillView, "Skill View", ToolsetIDCatalog)
	taskRead := descriptorWithID(ToolIDTaskRead, "Task Read", ToolsetIDTasks)
	provider := providerWithDescriptors(
		SourceRef{Kind: SourceBuiltin, Owner: "daemon"},
		toolList,
		toolInfo,
		skillView,
		taskRead,
	)
	catalog := defaultDiscoveryTestCatalog(t)
	resolver := &mutablePolicyInputResolver{
		inputs: PolicyInputs{
			SystemPermissionMode: PermissionModeApproveAll,
			ApprovalAvailable:    true,
		},
	}
	registry, err := NewRegistry(
		WithProviders(provider),
		WithPolicyInputResolver(resolver, catalog),
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	sessionScope := Scope{SessionID: "sess-1", AgentName: "coder"}

	t.Run("Should project full catalog until agent policy narrows the allowlist", func(t *testing.T) {
		resolver.inputs = PolicyInputs{
			SystemPermissionMode: PermissionModeApproveAll,
			ApprovalAvailable:    true,
		}
		views, err := registry.SessionProjection(ctx, sessionScope)
		if err != nil {
			t.Fatalf("SessionProjection(full default) error = %v", err)
		}
		requireToolIDs(t, views, ToolIDToolInfo, ToolIDToolList, ToolIDSkillView, ToolIDTaskRead)

		operatorViews, err := registry.OperatorProjection(ctx, Scope{
			SessionID: "sess-1",
			AgentName: "coder",
			Operator:  true,
		})
		if err != nil {
			t.Fatalf("OperatorProjection(scoped diagnostics) error = %v", err)
		}
		requireToolIDs(t, operatorViews, ToolIDToolInfo, ToolIDToolList, ToolIDSkillView, ToolIDTaskRead)

		resolver.inputs.Agent.Toolsets = []ToolsetID{ToolsetIDTasks}
		views, err = registry.SessionProjection(ctx, sessionScope)
		if err != nil {
			t.Fatalf("SessionProjection(narrowed agent) error = %v", err)
		}
		requireToolIDs(t, views, ToolIDTaskRead)
		requireNoToolID(t, views, ToolIDSkillView)
		requireNoToolID(t, views, ToolIDToolList)

		diagnostic, err := registry.DiagnosticGet(ctx, sessionScope, ToolIDToolInfo)
		if err != nil {
			t.Fatalf("DiagnosticGet(narrowed agent) error = %v", err)
		}
		if diagnostic.Decision.Callable {
			t.Fatalf("DiagnosticGet(%s).Callable = true, want denied diagnostic view", ToolIDToolInfo)
		}
		requireDecisionReason(t, diagnostic.Decision, ReasonPolicyDenied)
		diagnostics, err := registry.DiagnosticSearch(ctx, sessionScope, SearchQuery{Query: "Tool Info"})
		if err != nil {
			t.Fatalf("DiagnosticSearch(narrowed agent) error = %v", err)
		}
		requireToolIDs(t, diagnostics, ToolIDToolInfo)
	})

	t.Run("Should let explicit denies and session lineage override full default projection", func(t *testing.T) {
		denySkill, err := ParseToolPattern(ToolIDSkillView.String())
		if err != nil {
			t.Fatalf("ParseToolPattern(skill_view) error = %v", err)
		}
		resolver.inputs = PolicyInputs{
			SystemPermissionMode: PermissionModeApproveAll,
			ApprovalAvailable:    true,
			Agent: AgentToolPolicy{
				DenyTools: []ToolPattern{denySkill},
			},
		}
		views, err := registry.SessionProjection(ctx, sessionScope)
		if err != nil {
			t.Fatalf("SessionProjection(denied full default) error = %v", err)
		}
		requireToolIDs(t, views, ToolIDToolInfo, ToolIDToolList, ToolIDTaskRead)
		requireNoToolID(t, views, ToolIDSkillView)

		resolver.inputs = PolicyInputs{
			SystemPermissionMode: PermissionModeApproveAll,
			ApprovalAvailable:    true,
			Session: SessionToolPolicy{
				Enforced: true,
				Tools:    []ToolID{ToolIDToolInfo},
			},
		}
		views, err = registry.SessionProjection(ctx, sessionScope)
		if err != nil {
			t.Fatalf("SessionProjection(session lineage) error = %v", err)
		}
		requireToolIDs(t, views, ToolIDToolInfo)
		requireNoToolID(t, views, ToolIDToolList)
		requireNoToolID(t, views, ToolIDSkillView)
	})
}

func TestRuntimeRegistryResolverRevalidatesProjectionAndDispatch(t *testing.T) {
	ctx := t.Context()
	descriptor := descriptorWithID(ToolIDSkillView, "Skill View", ToolsetIDCatalog)
	handle := &registryTestHandle{
		descriptor: descriptor,
		availability: Availability{
			Registered:  true,
			Enabled:     true,
			Available:   true,
			Authorized:  true,
			Executable:  true,
			ReasonCodes: nil,
		},
	}
	callCount := 0
	handle.call = func(context.Context, CallRequest) (ToolResult, error) {
		callCount++
		return ToolResult{Content: []ToolContent{{Type: "text", Text: "ok"}}}, nil
	}
	provider := registryTestProvider{
		source:      SourceRef{Kind: SourceBuiltin, Owner: "daemon"},
		descriptors: []Descriptor{descriptor},
		handles: map[ToolID]Handle{
			descriptor.ID: handle,
		},
		resolveErr: map[ToolID]error{},
	}
	resolver := &mutablePolicyInputResolver{inputs: PolicyInputs{
		SystemPermissionMode: PermissionModeApproveAll,
		ApprovalAvailable:    true,
	}}
	registry, err := NewRegistry(
		WithProviders(provider),
		WithPolicyInputResolver(resolver, defaultDiscoveryTestCatalog(t)),
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	views, err := registry.List(ctx, Scope{})
	if err != nil {
		t.Fatalf("List(initial) error = %v", err)
	}
	requireToolIDs(t, views, descriptor.ID)

	denySkill, err := ParseToolPattern(descriptor.ID.String())
	if err != nil {
		t.Fatalf("ParseToolPattern() error = %v", err)
	}
	resolver.inputs.DenyTools = []ToolPattern{denySkill}
	searchResults, err := registry.Search(ctx, Scope{}, SearchQuery{Query: "skill"})
	if err != nil {
		t.Fatalf("Search(after deny) error = %v", err)
	}
	if len(searchResults) != 0 {
		t.Fatalf("Search(after deny) = %#v, want no session-visible results", searchResults)
	}
	_, err = registry.Get(ctx, Scope{}, descriptor.ID)
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("Get(after deny) error = %v, want ErrToolNotFound", err)
	}
	_, err = registry.Call(ctx, Scope{}, CallRequest{ToolID: descriptor.ID})
	if !errors.Is(err, ErrToolDenied) {
		t.Fatalf("Call(after deny) error = %v, want ErrToolDenied", err)
	}
	if callCount != 0 {
		t.Fatalf("call count after denied dispatch = %d, want 0", callCount)
	}

	resolver.inputs.DenyTools = nil
	handle.availability = Availability{
		Registered:  true,
		Enabled:     true,
		Available:   false,
		Authorized:  true,
		Executable:  false,
		ReasonCodes: []ReasonCode{ReasonBackendUnhealthy},
	}
	operatorViews, err := registry.OperatorProjection(ctx, Scope{Operator: true})
	if err != nil {
		t.Fatalf("OperatorProjection(unhealthy) error = %v", err)
	}
	requireViewReason(t, operatorViews, descriptor.ID, ReasonBackendUnhealthy)

	handle.availability = Availability{
		Registered:  true,
		Enabled:     true,
		Available:   true,
		Authorized:  true,
		Executable:  true,
		ReasonCodes: nil,
	}
	_, err = registry.Call(ctx, Scope{}, CallRequest{ToolID: descriptor.ID})
	if err != nil {
		t.Fatalf("Call(after policy and availability recover) error = %v", err)
	}
	if callCount != 1 {
		t.Fatalf("call count after recovered dispatch = %d, want 1", callCount)
	}
	if resolver.resolveCalls < 4 {
		t.Fatalf("resolver calls = %d, want recomputation across list/projection/call paths", resolver.resolveCalls)
	}
}

func providerWithDescriptors(source SourceRef, descriptors ...Descriptor) registryTestProvider {
	handles := make(map[ToolID]Handle, len(descriptors))
	for _, descriptor := range descriptors {
		handles[descriptor.ID] = &registryTestHandle{
			descriptor: descriptor,
			availability: Availability{
				Registered:  true,
				Enabled:     true,
				Available:   true,
				Authorized:  true,
				Executable:  true,
				ReasonCodes: nil,
			},
		}
	}
	return registryTestProvider{
		source:      source,
		descriptors: descriptors,
		handles:     handles,
		resolveErr:  map[ToolID]error{},
	}
}

func mcpDescriptor(id ToolID, owner string, rawName string) Descriptor {
	descriptor := validDescriptor()
	descriptor.ID = id
	descriptor.DisplayTitle = rawName
	descriptor.Backend = BackendRef{
		Kind:      BackendMCP,
		MCPServer: owner,
		MCPTool:   rawName,
	}
	descriptor.Source = SourceRef{
		Kind:          SourceMCP,
		Owner:         owner,
		RawServerName: owner,
		RawToolName:   rawName,
	}
	descriptor.Visibility = VisibilityModel
	descriptor.Risk = RiskRead
	descriptor.ReadOnly = true
	descriptor.Destructive = false
	descriptor.OpenWorld = false
	return descriptor
}

func requireViewReason(t *testing.T, views []ToolView, id ToolID, reason ReasonCode) {
	t.Helper()

	for i := range views {
		if views[i].Descriptor.ID != id {
			continue
		}
		requireDecisionReason(t, views[i].Decision, reason)
		return
	}
	t.Fatalf("view %q not found in %#v", id, views)
}

type mutablePolicyInputResolver struct {
	inputs       PolicyInputs
	resolveCalls int
}

var _ PolicyInputResolver = (*mutablePolicyInputResolver)(nil)

func (r *mutablePolicyInputResolver) Resolve(_ context.Context, _ Scope) (PolicyInputs, error) {
	r.resolveCalls++
	return clonePolicyInputs(r.inputs), nil
}

func descriptorWithID(id ToolID, title string, toolsets ...ToolsetID) Descriptor {
	descriptor := validDescriptor()
	descriptor.ID = id
	descriptor.DisplayTitle = title
	descriptor.Description = title
	descriptor.Toolsets = append([]ToolsetID(nil), toolsets...)
	return descriptor
}

func defaultDiscoveryTestCatalog(t *testing.T) ToolsetCatalog {
	t.Helper()

	catalog, err := NewToolsetCatalog(
		Toolset{
			ID: ToolsetIDBootstrap,
			Tools: []string{
				ToolIDToolInfo.String(),
				ToolIDToolList.String(),
			},
		},
		Toolset{
			ID:       ToolsetIDCatalog,
			Tools:    []string{ToolIDSkillView.String()},
			Toolsets: []ToolsetID{ToolsetIDBootstrap},
		},
		Toolset{
			ID:    ToolsetIDTasks,
			Tools: []string{ToolIDTaskRead.String()},
		},
	)
	if err != nil {
		t.Fatalf("NewToolsetCatalog() error = %v", err)
	}
	return catalog
}

func requireToolIDs(t *testing.T, views []ToolView, ids ...ToolID) {
	t.Helper()

	got := make(map[ToolID]struct{}, len(views))
	for i := range views {
		got[views[i].Descriptor.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := got[id]; !ok {
			t.Fatalf("tool %q not found in projection %#v", id, views)
		}
	}
	if len(got) != len(ids) {
		t.Fatalf("projection ids = %#v, want exactly %#v", sortedToolIDsFromSet(got), ids)
	}
}

func requireNoToolID(t *testing.T, views []ToolView, id ToolID) {
	t.Helper()

	for i := range views {
		if views[i].Descriptor.ID == id {
			t.Fatalf("projection contains %q: %#v", id, views)
		}
	}
}

func longASCII(length int) string {
	return string(makeFilledBytes(length, 'a'))
}

func makeFilledBytes(length int, value byte) []byte {
	if length < 0 {
		return nil
	}
	data := make([]byte, length)
	for i := range data {
		data[i] = value
	}
	return data
}

func TestRuntimeRegistryToolsets(t *testing.T) {
	t.Parallel()

	t.Run("Should expand a scoped provider snapshot consistently and refresh the next request", func(t *testing.T) {
		t.Parallel()
		catalog, err := NewToolsetCatalog(
			Toolset{ID: "compozy__first", Tools: []string{"compozy__*"}},
			Toolset{ID: "compozy__second", Tools: []string{"compozy__*"}},
		)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		provider := registryTestProvider{
			source: SourceRef{Kind: SourceBuiltin, Owner: "daemon"},
			listScoped: func(scope Scope) []Descriptor {
				if scope.WorkspaceID != "workspace-a" || scope.ProfileID != "profile-a" {
					t.Fatalf("provider scope = %#v, want the request workspace and profile", scope)
				}
				calls++
				id := ToolIDSkillView
				if calls > 1 {
					id = ToolIDTaskRead
				}
				return []Descriptor{descriptorWithID(id, "Current tool")}
			},
		}
		registry, err := NewRegistry(WithProviders(provider), WithPolicyInputs(DefaultPolicyInputs(), catalog))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []ToolID{ToolIDSkillView, ToolIDTaskRead} {
			views, err := registry.ListToolsets(t.Context(), Scope{WorkspaceID: "workspace-a", ProfileID: "profile-a"})
			if err != nil {
				t.Fatal(err)
			}
			if len(views) != 2 {
				t.Fatalf("ListToolsets() = %#v, want both toolsets", views)
			}
			for _, view := range views {
				if !slices.Equal(view.ExpandedTools, []ToolID{want}) || len(view.ReasonCodes) != 0 {
					t.Fatalf("toolset view = %#v, want current snapshot tool %q", view, want)
				}
			}
		}
	})

	t.Run("Should skip provider discovery when there are no toolsets", func(t *testing.T) {
		t.Parallel()
		provider := registryTestProvider{
			source: SourceRef{Kind: SourceBuiltin, Owner: "daemon"},
			list: func() []Descriptor {
				t.Fatal("empty toolset list should not discover providers")
				return nil
			},
		}
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatal(err)
		}
		views, err := registry.ListToolsets(t.Context(), Scope{})
		if err != nil || len(views) != 0 {
			t.Fatalf("ListToolsets() = %#v, %v; want an empty list", views, err)
		}
	})

	t.Run("Should preserve expansion diagnostics and get one fresh toolset", func(t *testing.T) {
		t.Parallel()
		catalog, err := NewToolsetCatalog(Toolset{ID: "compozy__missing", Tools: []string{string(ToolIDSkillView)}})
		if err != nil {
			t.Fatal(err)
		}
		provider := registryTestProvider{source: SourceRef{Kind: SourceBuiltin, Owner: "daemon"}}
		registry, err := NewRegistry(WithProviders(provider), WithPolicyInputs(DefaultPolicyInputs(), catalog))
		if err != nil {
			t.Fatal(err)
		}
		views, err := registry.ListToolsets(t.Context(), Scope{})
		if err != nil || len(views) != 1 {
			t.Fatalf("ListToolsets() = %#v, %v", views, err)
		}
		view, err := registry.GetToolset(t.Context(), Scope{}, "compozy__missing")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(view, views[0]) || !slices.Equal(view.ReasonCodes, []ReasonCode{ReasonToolUnknown}) {
			t.Fatalf("GetToolset() = %#v, want matching unknown-tool diagnostic %#v", view, views[0])
		}
	})
}

func BenchmarkRegistryListToolsets(b *testing.B) {
	for _, count := range []int{1, 16, 32} {
		b.Run(fmt.Sprintf("toolsets_%d", count), func(b *testing.B) {
			sets := make([]Toolset, count)
			for index := range count {
				sets[index] = Toolset{
					ID:    ToolsetID(fmt.Sprintf("compozy__set_%d", index)),
					Tools: []string{"compozy__*"},
				}
			}
			catalog, err := NewToolsetCatalog(sets...)
			if err != nil {
				b.Fatal(err)
			}
			descriptors := make([]Descriptor, 128)
			for index := range descriptors {
				descriptors[index] = descriptorWithID(ToolID(fmt.Sprintf("compozy__tool_%d", index)), "Tool")
			}
			provider := registryTestProvider{
				source:      SourceRef{Kind: SourceBuiltin, Owner: "daemon"},
				descriptors: descriptors,
			}
			registry, err := NewRegistry(WithProviders(provider), WithPolicyInputs(DefaultPolicyInputs(), catalog))
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				views, err := registry.ListToolsets(b.Context(), Scope{})
				if err != nil || len(views) != count {
					b.Fatalf("ListToolsets() count=%d error=%v", len(views), err)
				}
			}
		})
	}
}
