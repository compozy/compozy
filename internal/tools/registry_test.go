package tools

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type registryTestProvider struct {
	source      SourceRef
	descriptors []Descriptor
	list        func() []Descriptor
	listScoped  func(Scope) []Descriptor
	handles     map[ToolID]Handle
	resolveErr  map[ToolID]error
}

var _ Provider = registryTestProvider{}

func (p registryTestProvider) ID() SourceRef {
	return p.source
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

	ctx := context.Background()

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
		first.ToolPresentation = NewToolPresentation("Reading skill", "arg:name")
		second := validDescriptor()
		second.ID = "agh__skill_search"
		second.Backend.NativeName = "skill_search"
		second.ToolPresentation = NewToolPresentation("Searching skills", "query")
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
					scoped.ToolPresentation = NewToolPresentation("Global lookup", "arg:query")
					scoped.Source.Scope = "global"
				case "ws-a":
					if removedFromWorkspaceA {
						return nil
					}
					scoped.ToolPresentation = NewToolPresentation("Workspace A lookup", "arg:a")
					scoped.Source.Scope = "workspace"
					scoped.Source.WorkspaceID = "ws-a"
				case "ws-b":
					scoped.ToolPresentation = NewToolPresentation("Workspace B lookup", "arg:b")
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
		descriptor.ToolPresentation = NewToolPresentation("Workspace A secret lookup", "arg:private")
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
		descriptor.ID = ToolID("agh__" + longASCII(65))
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

	t.Run("Should keep operator diagnostics broader than session projection", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		skillView := validDescriptor()
		taskUpdate := validDescriptor()
		taskUpdate.ID = "agh__task_update"
		taskUpdate.ReadOnly = false
		taskUpdate.Risk = RiskMutating
		networkSend := validDescriptor()
		networkSend.ID = "agh__network_send"
		networkSend.ReadOnly = false
		networkSend.OpenWorld = true
		networkSend.Risk = RiskOpenWorld
		conflicted := mcpDescriptor("mcp__github__search", "github", "search")
		conflictedDuplicate := mcpDescriptor("mcp__github__search", "github", "Search")
		provider := providerWithDescriptors(
			SourceRef{Kind: SourceBuiltin, Owner: "daemon"},
			skillView,
			taskUpdate,
			networkSend,
			conflicted,
			conflictedDuplicate,
		)
		provider.handles[networkSend.ID] = &registryTestHandle{
			descriptor: networkSend,
			availability: Availability{
				Registered:  true,
				Enabled:     true,
				ReasonCodes: []ReasonCode{ReasonBackendUnhealthy},
			},
		}
		denyTaskPattern, err := ParseToolPattern("agh__task_*")
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
		requireViewReason(t, operatorViews, networkSend.ID, ReasonBackendUnhealthy)
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
}

func TestRuntimeRegistrySearchGetAndCustomEvaluator(t *testing.T) {
	t.Parallel()

	t.Run("Should search and get through a custom evaluator", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		skillView := validDescriptor()
		skillView.Tags = []string{"skills"}
		taskRead := validDescriptor()
		taskRead.ID = "agh__task_read"
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
}

func TestRuntimeRegistryDynamicPolicyResolver(t *testing.T) {
	ctx := context.Background()
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
	ctx := context.Background()
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

func TestRuntimeRegistryCallDispatchesRegisteredTool(t *testing.T) {
	t.Parallel()

	t.Run("Should call provider handle after policy passes", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		descriptor := validDescriptor()
		called := false
		provider := providerWithDescriptors(SourceRef{Kind: SourceBuiltin, Owner: "daemon"}, descriptor)
		provider.handles[descriptor.ID] = &registryTestHandle{
			descriptor:   descriptor,
			availability: provider.handles[descriptor.ID].Availability(ctx, Scope{}),
			call: func(_ context.Context, req CallRequest) (ToolResult, error) {
				called = true
				if req.ToolID != descriptor.ID {
					t.Fatalf("CallRequest.ToolID = %q, want %q", req.ToolID, descriptor.ID)
				}
				return ToolResult{Content: []ToolContent{{Type: "text", Text: "ok"}}}, nil
			},
		}
		registry, err := NewRegistry(
			WithProviders(provider),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		result, err := registry.Call(ctx, Scope{}, CallRequest{ToolID: descriptor.ID})
		if err != nil {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want nil", err)
		}
		if !called {
			t.Fatal("provider handle was not called")
		}
		if len(result.Content) != 1 || result.Content[0].Text != "ok" {
			t.Fatalf("result = %#v, want ok text content", result)
		}
	})
}

func TestRuntimeRegistryCallReturnsPolicyDenialsBeforeDispatch(t *testing.T) {
	t.Parallel()

	t.Run("Should return denied instead of not found for registered denied tools", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		descriptor := validDescriptor()
		denyPattern, err := ParseToolPattern(descriptor.ID.String())
		if err != nil {
			t.Fatalf("ParseToolPattern() error = %v", err)
		}
		registry, err := NewRegistry(
			WithProviders(providerWithDescriptors(SourceRef{Kind: SourceBuiltin, Owner: "daemon"}, descriptor)),
			WithPolicyInputs(PolicyInputs{
				SystemPermissionMode: PermissionModeApproveAll,
				DenyTools:            []ToolPattern{denyPattern},
			}, ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		_, err = registry.Call(ctx, Scope{}, CallRequest{ToolID: descriptor.ID})
		if !errors.Is(err, ErrToolDenied) {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolDenied", err)
		}
	})
}
