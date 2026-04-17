package extension_test

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	extension "github.com/compozy/compozy/sdk/extension"
	exttesting "github.com/compozy/compozy/sdk/extension/testing"
)

func TestTypedHookRegistrationCoversAllPublicHookBuilders(t *testing.T) {
	t.Parallel()

	ext := extension.New("sdk-ext", "1.0.0")

	ext.OnPlanPreDiscover(zeroMutableHandler[extension.PlanPreDiscoverPayload, extension.ExtraSourcesPatch]())
	ext.OnPlanPostDiscover(zeroMutableHandler[extension.PlanPostDiscoverPayload, extension.EntriesPatch]())
	ext.OnPlanPreGroup(zeroMutableHandler[extension.PlanPreGroupPayload, extension.EntriesPatch]())
	ext.OnPlanPostGroup(zeroMutableHandler[extension.PlanPostGroupPayload, extension.GroupsPatch]())
	ext.OnPlanPrePrepareJobs(zeroMutableHandler[extension.PlanPrePrepareJobsPayload, extension.GroupsPatch]())
	ext.OnPlanPreResolveTaskRuntime(
		zeroMutableHandler[extension.PlanPreResolveTaskRuntimePayload, extension.TaskRuntimePatch](),
	)
	ext.OnPlanPostPrepareJobs(zeroMutableHandler[extension.PlanPostPrepareJobsPayload, extension.JobsPatch]())
	ext.OnPromptPreBuild(zeroMutableHandler[extension.PromptPreBuildPayload, extension.BatchParamsPatch]())
	ext.OnPromptPostBuild(zeroMutableHandler[extension.PromptPostBuildPayload, extension.PromptTextPatch]())
	ext.OnPromptPreSystem(zeroMutableHandler[extension.PromptPreSystemPayload, extension.SystemAddendumPatch]())
	ext.OnAgentPreSessionCreate(
		zeroMutableHandler[extension.AgentPreSessionCreatePayload, extension.SessionRequestPatch](),
	)
	ext.OnAgentPostSessionCreate(zeroObserverHandler[extension.AgentPostSessionCreatePayload]())
	ext.OnAgentPreSessionResume(
		zeroMutableHandler[extension.AgentPreSessionResumePayload, extension.ResumeSessionRequestPatch](),
	)
	ext.OnAgentOnSessionUpdate(zeroObserverHandler[extension.AgentOnSessionUpdatePayload]())
	ext.OnAgentPostSessionEnd(zeroObserverHandler[extension.AgentPostSessionEndPayload]())
	ext.OnJobPreExecute(zeroMutableHandler[extension.JobPreExecutePayload, extension.JobPatch]())
	ext.OnJobPostExecute(zeroObserverHandler[extension.JobPostExecutePayload]())
	ext.OnJobPreRetry(zeroMutableHandler[extension.JobPreRetryPayload, extension.RetryDecisionPatch]())
	ext.OnRunPreStart(zeroMutableHandler[extension.RunPreStartPayload, extension.RuntimeConfigPatch]())
	ext.OnRunPostStart(zeroObserverHandler[extension.RunPostStartPayload]())
	ext.OnRunPreShutdown(zeroObserverHandler[extension.RunPreShutdownPayload]())
	ext.OnRunPostShutdown(zeroObserverHandler[extension.RunPostShutdownPayload]())
	ext.OnReviewPreFetch(zeroMutableHandler[extension.ReviewPreFetchPayload, extension.FetchConfigPatch]())
	ext.OnReviewPostFetch(zeroMutableHandler[extension.ReviewPostFetchPayload, extension.IssuesPatch]())
	ext.OnReviewPreBatch(zeroMutableHandler[extension.ReviewPreBatchPayload, extension.GroupsPatch]())
	ext.OnReviewPostFix(zeroObserverHandler[extension.ReviewPostFixPayload]())
	ext.OnReviewPreResolve(zeroMutableHandler[extension.ReviewPreResolvePayload, extension.ResolveDecisionPatch]())
	ext.OnArtifactPreWrite(zeroMutableHandler[extension.ArtifactPreWritePayload, extension.ArtifactWritePatch]())
	ext.OnArtifactPostWrite(zeroObserverHandler[extension.ArtifactPostWritePayload]())
	ext.OnEvent(func(context.Context, extension.Event) error { return nil }, extension.EventKind("run.completed"))

	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		GrantedCapabilities: allGrantedCapabilities(),
	})
	defer cancel()
	harness.HandleHostMethod("host.events.subscribe", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.EventSubscribeResult{SubscriptionID: "sub-1"}, nil
	})

	response, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    "sdk-ext",
		Version: "1.0.0",
		Source:  "workspace",
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if got, want := response.AcceptedCapabilities, expectedRequiredCapabilities(); len(got) != len(want) {
		t.Fatalf("accepted capability count = %d, want %d", len(got), len(want))
	} else {
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("accepted capabilities = %#v, want %#v", got, want)
			}
		}
	}

	if got, want := response.SupportedHookEvents, expectedHookEvents(); len(got) != len(want) {
		t.Fatalf("supported hook count = %d, want %d", len(got), len(want))
	} else {
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("supported hooks = %#v, want %#v", got, want)
			}
		}
	}

	shutdownHarness(ctx, t, harness, errCh)
}

func TestHostAPIAllMethodsRoundTrip(t *testing.T) {
	t.Parallel()

	ext := extension.New("sdk-ext", "1.0.0").WithCapabilities(
		extension.CapabilityEventsRead,
		extension.CapabilityEventsPublish,
		extension.CapabilityTasksRead,
		extension.CapabilityTasksCreate,
		extension.CapabilityRunsStart,
		extension.CapabilityArtifactsRead,
		extension.CapabilityArtifactsWrite,
		extension.CapabilityMemoryRead,
		extension.CapabilityMemoryWrite,
	)

	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		GrantedCapabilities: allGrantedCapabilities(),
	})
	defer cancel()

	harness.HandleHostMethod("host.events.subscribe", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.EventSubscribeResult{SubscriptionID: "sub-1"}, nil
	})
	harness.HandleHostMethod("host.events.publish", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.EventPublishResult{Seq: 7}, nil
	})
	harness.HandleHostMethod("host.tasks.list", func(_ context.Context, _ json.RawMessage) (any, error) {
		return []extension.Task{{Workflow: "demo", Number: 1}}, nil
	})
	harness.HandleHostMethod("host.tasks.get", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.Task{Workflow: "demo", Number: 2, Title: "Get"}, nil
	})
	harness.HandleHostMethod("host.tasks.create", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.Task{Workflow: "demo", Number: 3, Title: "Create"}, nil
	})
	harness.HandleHostMethod("host.runs.start", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.RunHandle{RunID: "run-123", ParentRunID: "run-parent"}, nil
	})
	harness.HandleHostMethod("host.artifacts.read", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.ArtifactReadResult{Path: "artifact.txt", Content: []byte("hello")}, nil
	})
	harness.HandleHostMethod("host.artifacts.write", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.ArtifactWriteResult{Path: "artifact.txt", BytesWritten: 5}, nil
	})
	harness.HandleHostMethod("host.prompts.render", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.PromptRenderResult{Rendered: "rendered"}, nil
	})
	harness.HandleHostMethod("host.memory.read", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.MemoryReadResult{Path: "memory.md", Exists: true, Content: "hello"}, nil
	})
	harness.HandleHostMethod("host.memory.write", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.MemoryWriteResult{Path: "memory.md", BytesWritten: 5}, nil
	})

	if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    "sdk-ext",
		Version: "1.0.0",
		Source:  "workspace",
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	subscription, err := ext.Host().Events.Subscribe(
		ctx,
		extension.EventSubscribeRequest{Kinds: []extension.EventKind{"run.completed"}},
	)
	if err != nil || subscription.SubscriptionID != "sub-1" {
		t.Fatalf("Events.Subscribe() = %#v, %v", subscription, err)
	}
	publish, err := ext.Host().Events.Publish(ctx, extension.EventPublishRequest{Kind: "extension.custom"})
	if err != nil || publish.Seq != 7 {
		t.Fatalf("Events.Publish() = %#v, %v", publish, err)
	}
	list, err := ext.Host().Tasks.List(ctx, extension.TaskListRequest{Workflow: "demo"})
	if err != nil || len(list) != 1 || list[0].Number != 1 {
		t.Fatalf("Tasks.List() = %#v, %v", list, err)
	}
	get, err := ext.Host().Tasks.Get(ctx, extension.TaskGetRequest{Workflow: "demo", Number: 2})
	if err != nil || get.Number != 2 {
		t.Fatalf("Tasks.Get() = %#v, %v", get, err)
	}
	create, err := ext.Host().Tasks.Create(ctx, extension.TaskCreateRequest{Workflow: "demo", Title: "Create"})
	if err != nil || create.Number != 3 {
		t.Fatalf("Tasks.Create() = %#v, %v", create, err)
	}
	runHandle, err := ext.Host().Runs.Start(ctx, extension.RunStartRequest{Runtime: extension.RunConfig{Name: "child"}})
	if err != nil || runHandle.RunID != "run-123" {
		t.Fatalf("Runs.Start() = %#v, %v", runHandle, err)
	}
	artifact, err := ext.Host().Artifacts.Read(ctx, extension.ArtifactReadRequest{Path: "artifact.txt"})
	if err != nil || string(artifact.Content) != "hello" {
		t.Fatalf("Artifacts.Read() = %#v, %v", artifact, err)
	}
	writeResult, err := ext.Host().Artifacts.Write(
		ctx,
		extension.ArtifactWriteRequest{Path: "artifact.txt", Content: []byte("hello")},
	)
	if err != nil || writeResult.BytesWritten != 5 {
		t.Fatalf("Artifacts.Write() = %#v, %v", writeResult, err)
	}
	rendered, err := ext.Host().Prompts.Render(ctx, extension.PromptRenderRequest{Template: "tpl"})
	if err != nil || rendered.Rendered != "rendered" {
		t.Fatalf("Prompts.Render() = %#v, %v", rendered, err)
	}
	memory, err := ext.Host().Memory.Read(ctx, extension.MemoryReadRequest{Workflow: "demo"})
	if err != nil || !memory.Exists {
		t.Fatalf("Memory.Read() = %#v, %v", memory, err)
	}
	writeMemory, err := ext.Host().Memory.Write(ctx, extension.MemoryWriteRequest{Workflow: "demo", Content: "hello"})
	if err != nil || writeMemory.BytesWritten != 5 {
		t.Fatalf("Memory.Write() = %#v, %v", writeMemory, err)
	}

	shutdownHarness(ctx, t, harness, errCh)
}

func zeroMutableHandler[Payload any, Patch any]() func(context.Context, extension.HookContext, Payload) (Patch, error) {
	return func(context.Context, extension.HookContext, Payload) (Patch, error) {
		var zero Patch
		return zero, nil
	}
}

func zeroObserverHandler[Payload any]() func(context.Context, extension.HookContext, Payload) error {
	return func(context.Context, extension.HookContext, Payload) error { return nil }
}

func allGrantedCapabilities() []extension.Capability {
	return []extension.Capability{
		extension.CapabilityAgentMutate,
		extension.CapabilityArtifactsRead,
		extension.CapabilityArtifactsWrite,
		extension.CapabilityEventsPublish,
		extension.CapabilityEventsRead,
		extension.CapabilityJobMutate,
		extension.CapabilityMemoryRead,
		extension.CapabilityMemoryWrite,
		extension.CapabilityPlanMutate,
		extension.CapabilityPromptMutate,
		extension.CapabilityReviewMutate,
		extension.CapabilityRunsStart,
		extension.CapabilityRunMutate,
		extension.CapabilityTasksCreate,
		extension.CapabilityTasksRead,
	}
}

func expectedRequiredCapabilities() []extension.Capability {
	values := []extension.Capability{
		extension.CapabilityAgentMutate,
		extension.CapabilityArtifactsWrite,
		extension.CapabilityEventsRead,
		extension.CapabilityJobMutate,
		extension.CapabilityPlanMutate,
		extension.CapabilityPromptMutate,
		extension.CapabilityReviewMutate,
		extension.CapabilityRunMutate,
	}
	sort.Slice(values, func(i, j int) bool { return string(values[i]) < string(values[j]) })
	return values
}

func expectedHookEvents() []extension.HookName {
	values := []extension.HookName{
		extension.HookPlanPreDiscover,
		extension.HookPlanPostDiscover,
		extension.HookPlanPreGroup,
		extension.HookPlanPostGroup,
		extension.HookPlanPrePrepareJobs,
		extension.HookPlanPreResolveTaskRuntime,
		extension.HookPlanPostPrepareJobs,
		extension.HookPromptPreBuild,
		extension.HookPromptPostBuild,
		extension.HookPromptPreSystem,
		extension.HookAgentPreSessionCreate,
		extension.HookAgentPostSessionCreate,
		extension.HookAgentPreSessionResume,
		extension.HookAgentOnSessionUpdate,
		extension.HookAgentPostSessionEnd,
		extension.HookJobPreExecute,
		extension.HookJobPostExecute,
		extension.HookJobPreRetry,
		extension.HookRunPreStart,
		extension.HookRunPostStart,
		extension.HookRunPreShutdown,
		extension.HookRunPostShutdown,
		extension.HookReviewPreFetch,
		extension.HookReviewPostFetch,
		extension.HookReviewPreBatch,
		extension.HookReviewPostFix,
		extension.HookReviewPreResolve,
		extension.HookArtifactPreWrite,
		extension.HookArtifactPostWrite,
	}
	sort.Slice(values, func(i, j int) bool { return string(values[i]) < string(values[j]) })
	return values
}
