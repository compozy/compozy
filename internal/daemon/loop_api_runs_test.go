package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/session"
	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// Invariant: request transports expose preview/provenance fields and never private blob refs.
// The daemon Loop API suite owns the public request payload builder.
func TestLoopRequestPayloadShouldExposeOnlyPublicRequestState(t *testing.T) {
	t.Parallel()

	answeredAt := time.Date(2026, time.August, 16, 14, 11, 3, 0, time.UTC)
	payload := loopRequestPayload(looppkg.Request{
		LoopRunID: "run-1", LoopName: "rollout", Generation: 2, NodeID: "select", ItemIndex: 3,
		Kind: looppkg.RequestKindAsk, State: looppkg.RequestStateAnswered,
		Prompt: "Choose an environment", Context: json.RawMessage(`{"truncated":true,"byte_size":20000}`),
		Expect: json.RawMessage(`{"environment":"string"}`), Decisions: []string{"respond"},
		Agents: dsl.ResponderAgentsAllow, AnsweredDecision: "respond",
		ActorKind: "human", ActorID: "operator:pedro", OpenedAt: answeredAt.Add(-time.Minute),
		ResolvedAt: &answeredAt,
	})
	response := contract.LoopRequestsResponse{
		Items:      []contract.LoopRequestPayload{payload},
		Aggregates: contract.LoopRequestAggregates{Pending: 4},
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	encoded := string(raw)
	for _, fragment := range []string{`"pending":4`, `"answered_at":"2026-08-16T14:11:03Z"`, `"agents":"allow"`} {
		if !strings.Contains(encoded, fragment) {
			t.Fatalf("request payload = %s, want %s", encoded, fragment)
		}
	}
	for _, privateField := range []string{"context_ref", "proposed_ref", "answered_payload_ref"} {
		if strings.Contains(encoded, privateField) {
			t.Fatalf("request payload leaked private field %q: %s", privateField, encoded)
		}
	}
}

// Invariant: amendment projections redact secrets, inline only bounded values, and never expose blob refs.
// The daemon Loop API suite owns the shared HTTP, UDS, CLI, and native-tool amendment projection.
func TestLoopNodeAmendmentPayloadShouldBeBoundedAndRedacted(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.August, 16, 15, 30, 0, 0, time.UTC)
	payload, err := loopNodeAmendmentPayload(looppkg.NodeAmendment{
		LoopRunID: "run-1", Generation: 2, NodeID: "repair", ItemIndex: 3, Sequence: 1,
		OriginalRef: "private-original-ref", AmendedRef: "private-amended-ref",
		Original: json.RawMessage(`{"api_token":"secret-value","value":"before"}`),
		Amended:  json.RawMessage(`{"value":"after"}`), ActorKind: "human", ActorID: "operator:one",
		CreatedAt: createdAt,
	})
	if err != nil {
		t.Fatalf("loopNodeAmendmentPayload() error = %v", err)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(amendment) error = %v", err)
	}
	if strings.Contains(string(encoded), "secret-value") || strings.Contains(string(encoded), "private-") ||
		!strings.Contains(string(encoded), `"api_token":"[REDACTED]"`) {
		t.Fatalf("amendment projection = %s, want redacted values without refs", encoded)
	}

	large := json.RawMessage(`{"value":"` + strings.Repeat("x", amendmentInlineLimitBytes) + `"}`)
	bounded, err := loopNodeAmendmentPayload(looppkg.NodeAmendment{
		LoopRunID: "run-1", Generation: 2, NodeID: "repair", Sequence: 2,
		Original: large, Amended: large, CreatedAt: createdAt,
	})
	if err != nil {
		t.Fatalf("loopNodeAmendmentPayload(large) error = %v", err)
	}
	if bounded.Original != nil || bounded.Amended != nil || bounded.OriginalSummary == nil ||
		bounded.AmendedSummary == nil || bounded.OriginalSummary.ByteSize <= amendmentInlineLimitBytes ||
		bounded.OriginalSummary.ContentHash == "" {
		t.Fatalf("bounded amendment projection = %#v", bounded)
	}
}

// Invariant: agents in a run starter's durable spawn chain cannot act as that run's responder.
// The daemon Loop API suite owns the shared responder trust boundary used by approve and respond.
func TestDaemonLoopResponderPolicyShouldEvaluateDurableSpawnChains(t *testing.T) {
	t.Parallel()

	policy := daemonLoopResponderPolicy{
		runs: responderRunReaderStub{run: looppkg.Run{
			ID: "run-1", WorkspaceID: "ws-1",
			StartedBy: task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: "starter"},
		}},
		sessions: responderSessionReaderStub{sessions: map[string]*session.Info{
			"starter":    responderSessionInfo("starter", "ws-1", ""),
			"child":      responderSessionInfo("child", "ws-1", "starter"),
			"grandchild": responderSessionInfo("grandchild", "ws-1", "child"),
			"unrelated":  responderSessionInfo("unrelated", "ws-1", ""),
			"stale":      responderSessionInfo("different", "ws-1", ""),
		}},
	}
	for _, tt := range []struct {
		name     string
		actor    task.ActorContext
		wantDeny bool
	}{
		{name: "Should deny the direct starter", actor: responderActorForTest(task.ActorKindAgentSession, "starter", "ws-1"), wantDeny: true},
		{name: "Should deny a transitively spawned child", actor: responderActorForTest(task.ActorKindAgentSession, "grandchild", "ws-1"), wantDeny: true},
		{name: "Should allow an unrelated agent", actor: responderActorForTest(task.ActorKindAgentSession, "unrelated", "ws-1")},
		{name: "Should allow a human operator", actor: responderActorForTest(task.ActorKindHuman, "operator:pedro", "ws-1")},
		{name: "Should deny a cross-workspace actor", actor: responderActorForTest(task.ActorKindAgentSession, "unrelated", "ws-other"), wantDeny: true},
		{name: "Should fail closed on missing lineage", actor: responderActorForTest(task.ActorKindAgentSession, "missing", "ws-1"), wantDeny: true},
		{name: "Should fail closed on stale lineage", actor: responderActorForTest(task.ActorKindAgentSession, "stale", "ws-1"), wantDeny: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			denied, err := policy.DeniesSelfOperation(t.Context(), "ws-1", "run-1", tt.actor)
			if err != nil {
				t.Fatalf("DeniesSelfOperation() error = %v", err)
			}
			if denied != tt.wantDeny {
				t.Fatalf("DeniesSelfOperation() = %v, want %v", denied, tt.wantDeny)
			}
		})
	}
}

type responderRunReaderStub struct {
	run looppkg.Run
	err error
}

func (s responderRunReaderStub) GetLoopRun(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RunID,
) (looppkg.Run, error) {
	return s.run, s.err
}

type responderSessionReaderStub struct {
	sessions map[string]*session.Info
}

func (s responderSessionReaderStub) Status(_ context.Context, sessionID string) (*session.Info, error) {
	info, ok := s.sessions[sessionID]
	if !ok {
		return nil, errors.New("session not found")
	}
	return info, nil
}

func responderSessionInfo(id, workspaceID, parentID string) *session.Info {
	return &session.Info{
		ID: id, WorkspaceID: workspaceID,
		Lineage: &storepkg.SessionLineage{ParentSessionID: parentID},
	}
}

func responderActorForTest(kind task.ActorKind, id, workspaceID string) task.ActorContext {
	return task.ActorContext{
		Actor: task.ActorIdentity{Kind: kind, Ref: id},
		Scope: task.CallerScope{WorkspaceID: workspaceID},
	}
}

func TestDaemonLoopAPIServiceShouldBuildRunWebURLFromEffectiveConfig(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(homePaths, "", compozyconfig.WriteScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
	}
	if _, err := compozyconfig.EditConfigOverlay(
		homePaths,
		"",
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			if err := editor.SetValue([]string{"http", "host"}, "127.0.0.1"); err != nil {
				return err
			}
			return editor.SetValue([]string{"http", "port"}, 43127)
		},
	); err != nil {
		t.Fatalf("EditConfigOverlay() error = %v", err)
	}

	service := &daemonLoopAPIService{homePaths: homePaths}
	endpoint, err := service.resolveLoopRunWebEndpoint(t.Context(), "ws-1")
	if err != nil {
		t.Fatalf("resolveLoopRunWebEndpoint() error = %v", err)
	}
	got := endpoint.runURL("run-123")
	const want = "http://127.0.0.1:43127/loop-runs/run-123"
	if got != want {
		t.Fatalf("loopRunWebEndpoint.runURL() = %q, want %q", got, want)
	}
}

func TestDaemonLoopAPIServiceShouldResolveRunWebEndpointBeforeStarting(t *testing.T) {
	t.Parallel()

	errResolve := errors.New("resolve effective config")
	startCalled := false
	aggregate := &loopApprovalAggregateStub{startFn: func(
		context.Context,
		looppkg.WorkspaceID,
		string,
		looppkg.Inputs,
		task.ActorContext,
	) (*looppkg.Run, error) {
		startCalled = true
		return nil, errors.New("unexpected Start call")
	}}
	service := &daemonLoopAPIService{
		aggregate:         aggregate,
		workspaceResolver: &loopAPIWorkspaceResolverErrorStub{err: errResolve},
	}

	_, err := service.RunLoop(
		t.Context(),
		"ws-1",
		"delivery",
		contract.RunLoopRequest{},
		dsl.StartHTTP,
		task.ActorContext{},
		false,
	)
	if !errors.Is(err, errResolve) {
		t.Fatalf("RunLoop() error = %v, want effective-config resolution error", err)
	}
	if startCalled {
		t.Fatal("RunLoop() called Start before resolving the run web endpoint")
	}
}

func TestDaemonLoopAPIServiceShouldAssembleGenerationDetailFromLineage(t *testing.T) {
	t.Parallel()

	score := 0.72
	rank := 0
	routeAt := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	storedOutputRef := looppkg.OutputRefForPayload([]byte(`{"value":"best"}`))
	persistence := &loopRunHistoryPersistenceStub{
		lineage: []looppkg.LoopGeneration{
			{
				RunID: "run-lineage", Generation: 1, ParentGeneration: 0,
				Origin: looppkg.OriginInitial,
			},
			{
				RunID: "run-lineage", Generation: 3, ParentGeneration: 1,
				Origin: looppkg.OriginRatchetRestore,
			},
		},
		outputs: map[int][]looppkg.GenerationOutput{
			1: {{Generation: 1, NodeID: "draft", Status: "succeeded", OutputRef: storedOutputRef}},
			3: {{Generation: 3, NodeID: "draft", Status: "pending"}},
		},
		verdicts: map[int64][]gate.VerdictRecord{
			3: {
				{
					RunID: "run-lineage", Generation: 3, GateID: "quality", ItemIndex: 2,
					Outcome: gate.VerdictOutcomeRejected, Score: &score, RouteCauseRank: &rank,
					BlockingIssues: []byte(`[{"id":"citation","note":"missing source"}]`),
					Criteria: []byte(
						`[{"id":"quality","type":"agent","outcome":"rejected",` +
							`"passed":false,"score":0.72,"evidence":{"source":"judge"}}]`,
					),
				},
				{
					RunID: "run-lineage", Generation: 3, GateID: "quality", ItemIndex: 3,
					Outcome:        gate.VerdictOutcomeApproved,
					BlockingIssues: []byte(`[]`),
					Criteria:       []byte(`[]`),
				},
			},
		},
		routeCauses: map[int64][]looppkg.RouteCause{
			3: {{
				Generation: 3, NodeID: "router", ItemIndex: 2, Route: "revise",
				Cause: "matched_when", MatchedWhen: "outputs.score < 0.8", At: routeAt,
			}},
		},
	}
	service := &daemonLoopAPIService{persistence: persistence}
	run := looppkg.Run{ID: "run-lineage", WorkspaceID: "ws-lineage", Generation: 3}

	generations, err := service.loopGenerations(t.Context(), run)
	if err != nil {
		t.Fatalf("loopGenerations() error = %v", err)
	}
	if len(generations) != 2 || generations[0].Generation != 1 || generations[1].Generation != 3 {
		t.Fatalf("loopGenerations() = %#v, want exact lineage rows 1 and 3", generations)
	}
	if got := generations[0].Outputs[0].OutputRef; got != storedOutputRef {
		t.Fatalf("generation output_ref = %q, want stored ref %q", got, storedOutputRef)
	}
	if generations[1].ParentGeneration != 1 ||
		generations[1].Origin != contract.LoopGenerationOriginRatchetRestore {
		t.Fatalf("generation 3 provenance = %#v, want ratchet restore from 1", generations[1])
	}
	if len(generations[1].Verdicts) != 2 || generations[1].Verdicts[0].GateID != "quality" ||
		generations[1].Verdicts[0].ItemIndex != 2 ||
		generations[1].Verdicts[0].Score == nil || *generations[1].Verdicts[0].Score != score ||
		generations[1].Verdicts[0].RouteCauseRank == nil || *generations[1].Verdicts[0].RouteCauseRank != rank ||
		len(generations[1].Verdicts[0].BlockingIssues) != 1 ||
		generations[1].Verdicts[0].BlockingIssues[0].ID != "citation" ||
		len(generations[1].Verdicts[0].Criteria) != 1 ||
		generations[1].Verdicts[0].Criteria[0].ID != "quality" ||
		generations[1].Verdicts[0].Criteria[0].Score == nil ||
		*generations[1].Verdicts[0].Criteria[0].Score != score {
		t.Fatalf("generation 3 verdicts = %#v, want exact diagnostics/score/rank", generations[1].Verdicts)
	}
	if generations[1].Verdicts[1].GateID != "quality" || generations[1].Verdicts[1].ItemIndex != 3 {
		t.Fatalf("generation 3 fan-out verdicts = %#v, want separate item indexes 2 and 3", generations[1].Verdicts)
	}
	if len(generations[1].RouteCauses) != 1 || generations[1].RouteCauses[0].NodeID != "router" ||
		generations[1].RouteCauses[0].ItemIndex != 2 || generations[1].RouteCauses[0].Route != "revise" ||
		generations[1].RouteCauses[0].Cause != "matched_when" ||
		generations[1].RouteCauses[0].MatchedWhen != "outputs.score < 0.8" ||
		!generations[1].RouteCauses[0].At.Equal(routeAt) {
		t.Fatalf("generation 3 route causes = %#v, want exact durable route decision", generations[1].RouteCauses)
	}
	if len(persistence.outputCalls) != 2 || persistence.outputCalls[0] != 1 || persistence.outputCalls[1] != 3 {
		t.Fatalf("ListGenerationOutputs calls = %#v, want lineage generations only", persistence.outputCalls)
	}
	for _, workspaceID := range persistence.workspaceCalls {
		if workspaceID != "ws-lineage" {
			t.Fatalf("workspace-scoped history call = %q, want ws-lineage", workspaceID)
		}
	}
}

func TestDaemonLoopAPIServiceShouldWrapGenerationHistoryErrors(t *testing.T) {
	t.Parallel()

	errPersistence := errors.New("history unavailable")
	run := looppkg.Run{ID: "run-errors", WorkspaceID: "ws-errors"}
	tests := []struct {
		name        string
		persistence *loopRunHistoryPersistenceStub
		wantContext string
	}{
		{
			name:        "Should identify the run when lineage loading fails",
			persistence: &loopRunHistoryPersistenceStub{lineageErr: errPersistence},
			wantContext: "list generations for loop run run-errors",
		},
		{
			name: "Should identify the run and generation when output loading fails",
			persistence: &loopRunHistoryPersistenceStub{
				lineage: []looppkg.LoopGeneration{{Generation: 4}}, outputErr: errPersistence,
			},
			wantContext: "list outputs for loop run run-errors generation 4",
		},
		{
			name: "Should identify the run and generation when verdict loading fails",
			persistence: &loopRunHistoryPersistenceStub{
				lineage: []looppkg.LoopGeneration{{Generation: 4}}, verdictErr: errPersistence,
			},
			wantContext: "list gate verdicts for loop run run-errors generation 4",
		},
		{
			name: "Should identify the run and generation when route cause loading fails",
			persistence: &loopRunHistoryPersistenceStub{
				lineage: []looppkg.LoopGeneration{{Generation: 4}}, routeCauseErr: errPersistence,
			},
			wantContext: "list route causes for loop run run-errors generation 4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &daemonLoopAPIService{persistence: tt.persistence}
			_, err := service.loopGenerations(t.Context(), run)
			if !errors.Is(err, errPersistence) {
				t.Fatalf("loopGenerations() error = %v, want wrapped persistence error", err)
			}
			if !strings.Contains(err.Error(), tt.wantContext) {
				t.Fatalf("loopGenerations() error = %q, want context %q", err, tt.wantContext)
			}
		})
	}
}

func TestDaemonLoopAPIServiceAnnotationsRequireDefinition(t *testing.T) {
	t.Parallel()

	t.Run("Should reject reads for a missing definition", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		service := &daemonLoopAPIService{
			catalog:     newResourceCatalog(looppkg.CloneResourceSpec),
			persistence: db,
		}

		_, err := service.GetLoopAnnotations(t.Context(), "ws-missing", "missing-loop")
		if !errors.Is(err, looppkg.ErrDefinitionNotFound) {
			t.Fatalf("GetLoopAnnotations(missing definition) error = %v, want ErrDefinitionNotFound", err)
		}
	})

	t.Run("Should reject writes for a missing definition without persisting them", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		service := &daemonLoopAPIService{
			catalog:     newResourceCatalog(looppkg.CloneResourceSpec),
			persistence: db,
		}
		request := contract.PutLoopAnnotationsRequest{Annotations: []contract.LoopAnnotationPayload{{
			NodeID: "ghost",
			X:      12,
			Y:      34,
		}}}

		_, err := service.PutLoopAnnotations(t.Context(), "ws-missing", "missing-loop", request)
		if !errors.Is(err, looppkg.ErrDefinitionNotFound) {
			t.Fatalf("PutLoopAnnotations(missing definition) error = %v, want ErrDefinitionNotFound", err)
		}
		annotations, listErr := db.ListLoopUIAnnotations(t.Context(), "ws-missing", "missing-loop")
		if listErr != nil {
			t.Fatalf("ListLoopUIAnnotations(after rejected write) error = %v", listErr)
		}
		if len(annotations) != 0 {
			t.Fatalf("annotations after rejected write = %#v, want empty", annotations)
		}
	})
}

func TestDaemonLoopAPIServiceShouldManageScopedInputDefaultsWithoutCollapsingPresence(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	workspaceRoot := t.TempDir()
	resolver := loopCatalogWorkspaceResolverForTest(
		t,
		"ws-input-defaults",
		workspaceRoot,
		time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	)
	service := &daemonLoopAPIService{homePaths: homePaths, workspaceResolver: resolver}

	global, err := service.PutLoopInputDefault(
		t.Context(),
		"ws-input-defaults",
		"review-and-fix",
		"auto_commit",
		contract.PutLoopInputDefaultRequest{Scope: contract.LoopInputDefaultsScopeGlobal, Value: false},
	)
	if err != nil {
		t.Fatalf("PutLoopInputDefault(global) error = %v", err)
	}
	if !global.Present || global.Value != false {
		t.Fatalf("global input default = %#v, want present explicit false", global)
	}

	workspace, err := service.PutLoopInputDefaults(
		t.Context(),
		"ws-input-defaults",
		"review-and-fix",
		contract.PutLoopInputDefaultsRequest{
			Scope: contract.LoopInputDefaultsScopeWorkspace,
			Values: map[string]any{
				"auto_commit": true,
				"retries":     float64(0),
				"reviewer":    "",
			},
		},
	)
	if err != nil {
		t.Fatalf("PutLoopInputDefaults(workspace) error = %v", err)
	}
	if workspace.Values["auto_commit"] != true {
		t.Fatalf("workspace auto_commit = %#v, want true", workspace.Values["auto_commit"])
	}
	switch retries := workspace.Values["retries"].(type) {
	case int64:
		if retries != 0 {
			t.Fatalf("workspace retries = %d, want zero", retries)
		}
	case float64:
		if retries != 0 {
			t.Fatalf("workspace retries = %f, want zero", retries)
		}
	default:
		t.Fatalf("workspace retries = %#v (%T), want numeric zero", retries, retries)
	}
	if reviewer, present := workspace.Values["reviewer"]; !present || reviewer != "" {
		t.Fatalf("workspace reviewer = %#v/%v, want present empty string", reviewer, present)
	}

	globalLayer, err := service.GetLoopInputDefaults(
		t.Context(),
		"ws-input-defaults",
		"review-and-fix",
		contract.LoopInputDefaultsScopeGlobal,
	)
	if err != nil {
		t.Fatalf("GetLoopInputDefaults(global) error = %v", err)
	}
	if value, present := globalLayer.Values["auto_commit"]; !present || value != false {
		t.Fatalf("global layer after workspace override = %#v, want explicit false preserved", globalLayer.Values)
	}

	deleted, err := service.DeleteLoopInputDefault(
		t.Context(),
		"ws-input-defaults",
		"review-and-fix",
		"auto_commit",
		contract.LoopInputDefaultsScopeWorkspace,
	)
	if err != nil {
		t.Fatalf("DeleteLoopInputDefault(workspace) error = %v", err)
	}
	if !deleted.Deleted {
		t.Fatalf("DeleteLoopInputDefault(workspace) = %#v, want deleted", deleted)
	}
	missing, err := service.GetLoopInputDefault(
		t.Context(),
		"ws-input-defaults",
		"review-and-fix",
		"auto_commit",
		contract.LoopInputDefaultsScopeWorkspace,
	)
	if err != nil {
		t.Fatalf("GetLoopInputDefault(workspace after delete) error = %v", err)
	}
	if missing.Present {
		t.Fatalf("workspace input default after delete = %#v, want absent", missing)
	}
}

func TestDaemonLoopAPIServiceApproveLoopRun(t *testing.T) {
	t.Parallel()

	t.Run("Should deny approval by the Loop starter agent session", func(t *testing.T) {
		t.Parallel()

		aggregate := &loopApprovalAggregateStub{
			run: loopApprovalRun("sess-author"),
			approveFn: func(
				context.Context,
				looppkg.WorkspaceID,
				looppkg.RunID,
				looppkg.NodeID,
				looppkg.GateDecision,
				task.ActorContext,
			) error {
				t.Fatal("Approve() should not be called for self-approval")
				return nil
			},
		}
		service := &daemonLoopAPIService{aggregate: aggregate}
		actor := mustLoopApprovalActor(t, "sess-author")

		err := service.ApproveLoopRun(
			t.Context(),
			"ws-1",
			"run-1",
			contract.ApproveLoopRunRequest{GateID: "human", Decision: contract.LoopGateDecisionApprove},
			actor,
		)
		if !errors.Is(err, task.ErrPermissionDenied) {
			t.Fatalf("ApproveLoopRun() error = %v, want ErrPermissionDenied", err)
		}
	})

	t.Run("Should delegate approval for a different agent session", func(t *testing.T) {
		t.Parallel()

		approveCalled := false
		aggregate := &loopApprovalAggregateStub{
			run: loopApprovalRun("sess-author"),
			approveFn: func(
				_ context.Context,
				ws looppkg.WorkspaceID,
				runID looppkg.RunID,
				gateID looppkg.NodeID,
				decision looppkg.GateDecision,
				approveActor task.ActorContext,
			) error {
				approveCalled = true
				if ws != looppkg.WorkspaceID("ws-1") ||
					runID != looppkg.RunID("run-1") ||
					gateID != looppkg.NodeID("human") ||
					decision != looppkg.GateDecisionApprove {
					t.Fatalf("Approve() = %s/%s/%s/%s", ws, runID, gateID, decision)
				}
				if approveActor.Actor.Ref != "sess-reviewer" {
					t.Fatalf("Approve() actor = %#v, want sess-reviewer", approveActor.Actor)
				}
				return nil
			},
		}
		service := &daemonLoopAPIService{aggregate: aggregate}
		actor := mustLoopApprovalActor(t, "sess-reviewer")

		if err := service.ApproveLoopRun(
			t.Context(),
			"ws-1",
			"run-1",
			contract.ApproveLoopRunRequest{GateID: "human", Decision: contract.LoopGateDecisionApprove},
			actor,
		); err != nil {
			t.Fatalf("ApproveLoopRun() error = %v", err)
		}
		if !approveCalled {
			t.Fatal("Approve() was not called")
		}
	})
}

func loopApprovalRun(starterSession string) *looppkg.Run {
	return &looppkg.Run{
		ID:          looppkg.RunID("run-1"),
		WorkspaceID: looppkg.WorkspaceID("ws-1"),
		Status:      looppkg.StatusNeedsApproval,
		StartedBy: task.ActorIdentity{
			Kind: task.ActorKindAgentSession,
			Ref:  starterSession,
		},
	}
}

func mustLoopApprovalActor(t *testing.T, sessionID string) task.ActorContext {
	t.Helper()
	actor, err := task.DeriveAgentSessionActorContextForOrigin(
		sessionID,
		"ws-1",
		task.OriginKindUDS,
		"loop_approve",
	)
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContextForOrigin() error = %v", err)
	}
	return actor
}

type loopRunHistoryPersistenceStub struct {
	loopAPIPersistence
	lineage        []looppkg.LoopGeneration
	outputs        map[int][]looppkg.GenerationOutput
	verdicts       map[int64][]gate.VerdictRecord
	routeCauses    map[int64][]looppkg.RouteCause
	outputCalls    []int
	workspaceCalls []string
	lineageErr     error
	outputErr      error
	verdictErr     error
	routeCauseErr  error
}

func (s *loopRunHistoryPersistenceStub) ListGenerations(
	_ context.Context,
	workspaceID string,
	_ string,
) ([]looppkg.LoopGeneration, error) {
	s.workspaceCalls = append(s.workspaceCalls, workspaceID)
	if s.lineageErr != nil {
		return nil, s.lineageErr
	}
	return append([]looppkg.LoopGeneration(nil), s.lineage...), nil
}

func (s *loopRunHistoryPersistenceStub) ListGenerationOutputs(
	_ context.Context,
	workspaceID looppkg.WorkspaceID,
	_ looppkg.RunID,
	generation int,
) ([]looppkg.GenerationOutput, error) {
	s.workspaceCalls = append(s.workspaceCalls, string(workspaceID))
	s.outputCalls = append(s.outputCalls, generation)
	if s.outputErr != nil {
		return nil, s.outputErr
	}
	return append([]looppkg.GenerationOutput(nil), s.outputs[generation]...), nil
}

func (s *loopRunHistoryPersistenceStub) ListGateVerdicts(
	_ context.Context,
	workspaceID string,
	_ string,
	generation int64,
) ([]gate.VerdictRecord, error) {
	s.workspaceCalls = append(s.workspaceCalls, workspaceID)
	if s.verdictErr != nil {
		return nil, s.verdictErr
	}
	return append([]gate.VerdictRecord(nil), s.verdicts[generation]...), nil
}

func (s *loopRunHistoryPersistenceStub) ListRouteCauses(
	_ context.Context,
	workspaceID looppkg.WorkspaceID,
	_ looppkg.RunID,
	generation int64,
) ([]looppkg.RouteCause, error) {
	s.workspaceCalls = append(s.workspaceCalls, string(workspaceID))
	if s.routeCauseErr != nil {
		return nil, s.routeCauseErr
	}
	return append([]looppkg.RouteCause(nil), s.routeCauses[generation]...), nil
}

type loopApprovalAggregateStub struct {
	run     *looppkg.Run
	startFn func(
		context.Context,
		looppkg.WorkspaceID,
		string,
		looppkg.Inputs,
		task.ActorContext,
	) (*looppkg.Run, error)
	approveFn func(
		context.Context,
		looppkg.WorkspaceID,
		looppkg.RunID,
		looppkg.NodeID,
		looppkg.GateDecision,
		task.ActorContext,
	) error
}

func (s *loopApprovalAggregateStub) Start(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	name string,
	inputs looppkg.Inputs,
	actor task.ActorContext,
) (*looppkg.Run, error) {
	if s.startFn != nil {
		return s.startFn(ctx, ws, name, inputs, actor)
	}
	return nil, errors.New("unexpected Start call")
}

type loopAPIWorkspaceResolverErrorStub struct {
	err error
}

func (s *loopAPIWorkspaceResolverErrorStub) Resolve(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return workspacepkg.ResolvedWorkspace{}, s.err
}

func (s *loopAPIWorkspaceResolverErrorStub) ResolveOrRegister(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return workspacepkg.ResolvedWorkspace{}, s.err
}

func (s *loopAPIWorkspaceResolverErrorStub) Get(
	context.Context,
	string,
) (workspacepkg.Workspace, error) {
	return workspacepkg.Workspace{}, s.err
}

func (s *loopApprovalAggregateStub) StartInline(
	context.Context,
	looppkg.WorkspaceID,
	dsl.Definition,
	looppkg.Inputs,
	looppkg.RunOrigin,
	task.ActorContext,
) (*looppkg.Run, error) {
	return nil, errors.New("unexpected StartInline call")
}

func (s *loopApprovalAggregateStub) ReplaceInline(
	context.Context,
	looppkg.RunID,
	looppkg.WorkspaceID,
	dsl.Definition,
	looppkg.Inputs,
	looppkg.RunOrigin,
	task.ActorContext,
) (looppkg.InlineReplaceResult, error) {
	return looppkg.InlineReplaceResult{}, errors.New("unexpected ReplaceInline call")
}

func (s *loopApprovalAggregateStub) ClearInlineGoal(
	context.Context,
	looppkg.WorkspaceID,
	string,
	task.ActorContext,
) error {
	return errors.New("unexpected ClearInlineGoal call")
}

func (s *loopApprovalAggregateStub) DryRun(
	context.Context,
	looppkg.WorkspaceID,
	string,
	looppkg.Inputs,
) (*looppkg.PlanPreview, error) {
	return nil, errors.New("unexpected DryRun call")
}

func (s *loopApprovalAggregateStub) CancelRun(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RunID,
	string,
	task.ActorContext,
) error {
	return errors.New("unexpected CancelRun call")
}

func (s *loopApprovalAggregateStub) KillRun(
	context.Context, looppkg.WorkspaceID, looppkg.RunID, string, task.ActorContext,
) error {
	return errors.New("unexpected KillRun call")
}

func (s *loopApprovalAggregateStub) CancelNode(
	context.Context, looppkg.WorkspaceID, looppkg.RunID, looppkg.NodeID, *int, string, task.ActorContext,
) error {
	return errors.New("unexpected CancelNode call")
}

func (s *loopApprovalAggregateStub) KillNode(
	context.Context, looppkg.WorkspaceID, looppkg.RunID, looppkg.NodeID, *int, string, task.ActorContext,
) error {
	return errors.New("unexpected KillNode call")
}

func (s *loopApprovalAggregateStub) Pause(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RunID,
	task.ActorContext,
) error {
	return errors.New("unexpected Pause call")
}

func (s *loopApprovalAggregateStub) Resume(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RunID,
	task.ActorContext,
) error {
	return errors.New("unexpected Resume call")
}

func (s *loopApprovalAggregateStub) Approve(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	runID looppkg.RunID,
	gateID looppkg.NodeID,
	decision looppkg.GateDecision,
	actor task.ActorContext,
) error {
	if s.approveFn == nil {
		return errors.New("unexpected Approve call")
	}
	return s.approveFn(ctx, ws, runID, gateID, decision, actor)
}

func (s *loopApprovalAggregateStub) ListRequests(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RequestQuery,
) (looppkg.RequestPage, error) {
	return looppkg.RequestPage{}, errors.New("unexpected ListRequests call")
}

func (s *loopApprovalAggregateStub) GetRequest(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RequestRef,
) (looppkg.RequestDetail, error) {
	return looppkg.RequestDetail{}, errors.New("unexpected GetRequest call")
}

func (s *loopApprovalAggregateStub) Respond(
	context.Context,
	looppkg.RespondInput,
) (looppkg.RespondResult, error) {
	return looppkg.RespondResult{}, errors.New("unexpected Respond call")
}

func (s *loopApprovalAggregateStub) AmendNodeOutput(
	context.Context,
	looppkg.AmendInput,
) (looppkg.NodeAmendment, error) {
	return looppkg.NodeAmendment{}, errors.New("unexpected AmendNodeOutput call")
}

func (s *loopApprovalAggregateStub) Configure(
	context.Context,
	looppkg.WorkspaceID,
	string,
	looppkg.LoopConfig,
) error {
	return errors.New("unexpected Configure call")
}

func (s *loopApprovalAggregateStub) GetConfig(
	context.Context,
	looppkg.WorkspaceID,
	string,
) (*looppkg.LoopConfig, error) {
	return nil, errors.New("unexpected GetConfig call")
}

func (s *loopApprovalAggregateStub) GetConfigSnapshot(
	context.Context,
	looppkg.WorkspaceID,
	string,
) (looppkg.ConfigSnapshot, error) {
	return looppkg.ConfigSnapshot{}, errors.New("unexpected GetConfigSnapshot call")
}

func (s *loopApprovalAggregateStub) Get(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RunID,
) (*looppkg.Run, error) {
	if s.run == nil {
		return nil, looppkg.ErrRunNotFound
	}
	run := *s.run
	return &run, nil
}

func (s *loopApprovalAggregateStub) Transition(
	context.Context,
	looppkg.RunID,
	looppkg.Status,
	looppkg.TransitionCause,
) error {
	return errors.New("unexpected Transition call")
}
