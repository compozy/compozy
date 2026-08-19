package loop

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/loop/gate"
)

// Suite: generation history projection
// Invariant: history references always resolve through a complete authored shape.
// Boundary IN: pure history projection and its reader-backed record assembly.
// Boundary OUT: coordinator wiring and persistence queries, owned by their canonical suites.

func TestGenerationHistoryProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should seed initial generation history with schema-shaped zeros", testGenerationHistoryInitialGeneration)
	t.Run("Should project every previous node and verdict", testGenerationHistoryPreviousGeneration)
	t.Run("Should keep sparse later generations schema-total", testGenerationHistorySparseGeneration)
	t.Run("Should expose only escalated classified failures to repair context", testGenerationHistoryRepairFailure)
	t.Run("Should scope fan-out history to the current item", testGenerationHistoryFanOutScope)
	t.Run("Should project best outputs without status", testGenerationHistoryBestGeneration)
	t.Run("Should reject invalid best-generation state", testGenerationHistoryBestValidation)
	t.Run("Should project scalar best output", testGenerationHistoryScalarBestOutput)
	t.Run("Should hydrate externalized previous and best outputs", testGenerationHistoryExternalizedOutputs)
	t.Run("Should authorize every externalized output through its owning cell", testGenerationHistoryOutputOwnership)
	t.Run("Should include the contract verdict in route causes", testGenerationHistoryContractVerdict)
	t.Run(
		"Should preserve terminal node output or absence without authoring errors",
		testGenerationHistoryTerminalNodes,
	)
	t.Run("Should reject a non-positive generation", testGenerationHistoryNonPositiveGeneration)
}

func testGenerationHistoryExternalizedOutputs(t *testing.T) {
	t.Parallel()

	previousPayload := json.RawMessage(`{"summary":"previous"}`)
	bestPayload := json.RawMessage(`{"summary":"best"}`)
	previousRef := OutputRefForPayload(previousPayload)
	bestRef := OutputRefForPayload(bestPayload)
	bestGeneration := int64(1)
	bestScore := 0.91
	run := Run{
		ID: "run-history-payloads", WorkspaceID: "ws-history-payloads",
		BestGeneration: &bestGeneration, BestScore: &bestScore,
	}
	reader := generationHistoryReaderStub{
		outputs: map[int][]GenerationOutput{
			1: {{Generation: 1, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: bestRef}},
			2: {{Generation: 2, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: previousRef}},
		},
		payloads: map[GenerationOutputPayloadKey]json.RawMessage{
			{WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1, NodeID: "draft", OutputRef: bestRef}:     bestPayload,
			{WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 2, NodeID: "draft", OutputRef: previousRef}: previousPayload,
		},
	}

	history, err := ReadGenerationHistory(context.Background(), reader, run, 3)
	if err != nil {
		t.Fatalf("ReadGenerationHistory() error = %v", err)
	}
	previous, ok := history.Previous.Nodes["draft"][0].Output.(map[string]any)
	if !ok || previous["summary"] != "previous" {
		t.Fatalf("previous output = %#v, want hydrated payload", history.Previous.Nodes["draft"][0].Output)
	}
	best, ok := history.Best.Nodes["draft"][0].Output.(map[string]any)
	if !ok || best["summary"] != "best" {
		t.Fatalf("best output = %#v, want hydrated payload", history.Best.Nodes["draft"][0].Output)
	}
	if got := reader.outputs[2][0].OutputRef; got != previousRef {
		t.Fatalf("previous stored output_ref = %q, want %q", got, previousRef)
	}
	if got := reader.outputs[1][0].OutputRef; got != bestRef {
		t.Fatalf("best stored output_ref = %q, want %q", got, bestRef)
	}
}

func testGenerationHistoryOutputOwnership(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{"summary":"shared"}`)
	ref := OutputRefForPayload(payload)
	run := Run{ID: "run-output-ownership", WorkspaceID: "ws-output-ownership"}
	reader := generationHistoryReaderStub{
		outputs: map[int][]GenerationOutput{
			2: {
				{Generation: 2, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: ref},
				{Generation: 2, NodeID: "review", Status: generationOutputSucceeded, OutputRef: ref},
			},
		},
		payloads: map[GenerationOutputPayloadKey]json.RawMessage{
			{WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 2, NodeID: "draft", OutputRef: ref}: payload,
		},
	}

	_, err := ReadGenerationHistory(context.Background(), reader, run, 3)
	if !errors.Is(err, ErrOutputRefNotFound) {
		t.Fatalf("ReadGenerationHistory() error = %v, want %v", err, ErrOutputRefNotFound)
	}
}

func testGenerationHistoryRepairFailure(t *testing.T) {
	t.Parallel()
	failureClass := FailurePayloadDeclared
	reader := generationHistoryReaderStub{
		outputs: map[int][]GenerationOutput{2: {
			{NodeID: "fetch", Status: generationOutputFailed},
			{NodeID: "optional", Status: generationOutputSucceeded, OutputRef: failureAbsorbedOutputRef},
		}},
		attempts: []NodeAttempt{
			{Generation: 2, NodeID: "fetch", Attempt: 1, FailureClass: &failureClass,
				FailureCode: "tool_failed", Cause: "request failed", Hint: "use a smaller batch",
				Disposition: AttemptEscalated},
			{Generation: 2, NodeID: "optional", Attempt: 1, FailureClass: &failureClass,
				FailureCode: "optional_failed", Cause: "optional request failed", Hint: "ignore",
				Disposition: AttemptAbsorbed},
		},
	}
	history, err := ReadGenerationHistory(
		context.Background(), reader, Run{ID: "run-repair", WorkspaceID: "workspace-1"}, 3,
	)
	if err != nil {
		t.Fatalf("ReadGenerationHistory() error = %v", err)
	}
	fetch := history.Previous.Nodes["fetch"][0]
	if fetch.Failure == nil || fetch.Failure.Class != failureClass || fetch.Failure.Code != "tool_failed" ||
		fetch.Failure.Hint != "use a smaller batch" || fetch.Disposition != AttemptEscalated {
		t.Fatalf("fetch repair failure = %#v, want classified escalated attempt", fetch)
	}
	if optional := history.Previous.Nodes["optional"][0]; optional.Failure != nil || optional.Disposition != "" {
		t.Fatalf("optional repair failure = %#v, want absorbed failure omitted", optional)
	}
	namespace := history.previousNamespace(
		newControlTopology(dsl.Graph{Nodes: []dsl.Node{
			{ID: "fetch", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
			{ID: "optional", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
		}}),
		"fetch",
		0,
	)
	nodes := namespace[namespaceNodesKey].(map[string]any)
	fetchNamespace := nodes["fetch"].(map[string]any)
	projected, ok := fetchNamespace[namespaceFailureKey].(ClassifiedFailure)
	if !ok || projected.Cause != "request failed" || fetchNamespace[namespaceDispositionKey] != AttemptEscalated {
		t.Fatalf("fetch namespace = %#v, want classified repair context", fetchNamespace)
	}
	optionalNamespace := nodes["optional"].(map[string]any)
	if _, exists := optionalNamespace[namespaceFailureKey]; exists {
		t.Fatalf("optional namespace = %#v, want no absorbed repair context", optionalNamespace)
	}
}

func testGenerationHistoryInitialGeneration(t *testing.T) {
	t.Parallel()

	history, err := ReadGenerationHistory(context.Background(), nil, Run{}, 1)
	if err != nil {
		t.Fatalf("ReadGenerationHistory() error = %v", err)
	}
	if history.Previous != nil {
		t.Fatalf("history.Previous = %#v, want nil", history.Previous)
	}
	if history.Best != nil {
		t.Fatalf("history.Best = %#v, want nil", history.Best)
	}
	definition := dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{
		{
			ID: "draft", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{"output_schema": map[string]any{
				"type": "object", "properties": map[string]any{
					"summary": map[string]any{"type": "string"},
					"counts": map[string]any{"type": "object", "properties": map[string]any{
						"done": map[string]any{"type": "integer"},
					}},
				},
			}},
		},
		{ID: "quality", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate)},
	}}}
	namespace, err := runtimeNamespaceWithHistory(
		Run{},
		1,
		definition.Graph,
		newDefinitionControlTopology(definition, nil),
		nil,
		history,
		"",
		0,
	)
	if err != nil {
		t.Fatalf("runtimeNamespaceWithHistory() error = %v", err)
	}
	previous, ok := namespace["previous"].(map[string]any)
	if !ok {
		t.Fatalf("namespace[previous] = %#v, want absent-data object", namespace["previous"])
	}
	previousNodes, ok := previous["nodes"].(map[string]any)
	if !ok {
		t.Fatalf("namespace[previous].nodes = %#v, want node map", previous["nodes"])
	}
	draft := previousNodes["draft"].(map[string]any)
	draftOutput := draft[namespaceOutputKey].(map[string]any)
	counts := draftOutput["counts"].(map[string]any)
	if draft[namespaceStatusKey] != "" || draftOutput["summary"] != "" || counts["done"] != json.Number("0") {
		t.Fatalf("namespace[previous].nodes.draft = %#v, want schema-shaped zeros", draft)
	}
	verdict := previous[namespaceVerdictsKey].(map[string]any)["quality"].(map[string]any)
	if verdict[namespaceOutcomeKey] != "" || !reflect.DeepEqual(verdict[namespaceBlockingIssuesKey], []any{}) {
		t.Fatalf("namespace[previous].verdicts.quality = %#v, want empty verdict shape", verdict)
	}
	rendered, err := refs.RenderTemplateString(
		"history",
		"{{ .previous.nodes.draft.output.summary }}|{{ .previous.verdicts.quality.blocking_issues }}",
		namespace,
	)
	if err != nil {
		t.Fatalf("RenderTemplateString() error = %v", err)
	}
	if rendered != "|[]" {
		t.Fatalf("RenderTemplateString() = %q, want %q", rendered, "|[]")
	}
	best, ok := namespace["best"].(map[string]any)
	if !ok || best[metadataGenerationKey] != int64(0) {
		t.Fatalf("namespace[best] = %#v, want generation-zero object", namespace["best"])
	}
}

func testGenerationHistoryPreviousGeneration(t *testing.T) {
	t.Parallel()

	qualityScore := 0.81
	reader := generationHistoryReaderStub{
		outputs: map[int][]GenerationOutput{
			2: {
				{NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"summary":"ready"}`},
				{NodeID: "review", Status: generationOutputFailed, OutputRef: `{"reason":"missing citation"}`},
			},
		},
		verdicts: map[int][]gate.VerdictRecord{
			2: {
				{
					GateID:         "quality",
					Outcome:        gate.VerdictOutcomeRejected,
					Score:          &qualityScore,
					BlockingIssues: []byte(`[{"id":"citation"}]`),
					Criteria:       []byte(`[{"id":"quality","passed":false}]`),
				},
				{
					GateID:         "security",
					Outcome:        gate.VerdictOutcomeApproved,
					BlockingIssues: []byte(`[]`),
					Criteria:       []byte(`[{"id":"security","passed":true}]`),
				},
			},
		},
		routeCauses: map[int][]gate.VerdictRecord{
			2: {
				{GateID: "security"},
				{GateID: "quality"},
			},
		},
	}

	history, err := ReadGenerationHistory(
		context.Background(),
		reader,
		Run{ID: "run-1", WorkspaceID: "workspace-1"},
		3,
	)
	if err != nil {
		t.Fatalf("ReadGenerationHistory() error = %v", err)
	}
	if history.Previous == nil {
		t.Fatal("history.Previous = nil, want generation projection")
	}
	if got, want := history.Previous.Generation, int64(2); got != want {
		t.Fatalf("history.Previous.Generation = %d, want %d", got, want)
	}
	if got, want := history.Previous.Nodes["draft"][0].Status, generationOutputSucceeded; got != want {
		t.Fatalf("draft status = %q, want %q", got, want)
	}
	reviewOutput, ok := history.Previous.Nodes["review"][0].Output.(map[string]any)
	if !ok {
		t.Fatalf("review output = %#v, want object", history.Previous.Nodes["review"][0].Output)
	}
	if got, want := reviewOutput["reason"], "missing citation"; got != want {
		t.Fatalf("review output reason = %v, want %q", got, want)
	}
	if got, want := len(history.Previous.Verdicts), 2; got != want {
		t.Fatalf("history.Previous.Verdicts length = %d, want %d", got, want)
	}
	if got, want := history.Previous.Verdicts["quality"][0].Outcome, gate.VerdictOutcomeRejected; got != want {
		t.Fatalf("quality verdict outcome = %q, want %q", got, want)
	}
	if history.Previous.Verdicts["quality"][0].Score == nil {
		t.Fatal("quality verdict score = nil, want score")
	}
	if got, want := *history.Previous.Verdicts["quality"][0].Score, qualityScore; got != want {
		t.Fatalf("quality verdict score = %v, want %v", got, want)
	}
	wantCauses := []GateInstanceProjection{{GateID: "security"}, {GateID: "quality"}}
	if got := history.Previous.RouteCauses; !reflect.DeepEqual(got, wantCauses) {
		t.Fatalf("history.Previous.RouteCauses = %#v, want %#v", got, wantCauses)
	}
}

func testGenerationHistorySparseGeneration(t *testing.T) {
	t.Parallel()

	definition := dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{
		{
			ID: "draft", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
			Produces: dsl.Schema{"summary": "string"},
		},
		{
			ID: "repair", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
			Produces: dsl.Schema{"summary": "string"},
		},
		{ID: "quality", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate)},
	}}}
	history := GenerationHistory{Previous: &PreviousGeneration{
		Generation: 2,
		Nodes: map[string]map[int]NodeProjection{
			"draft": {0: {Status: generationOutputSucceeded, Output: map[string]any{"summary": "ready"}}},
		},
		Verdicts: map[string]map[int]VerdictProjection{},
	}}
	namespace, err := runtimeNamespaceWithHistory(
		Run{},
		3,
		definition.Graph,
		newDefinitionControlTopology(definition, nil),
		nil,
		history,
		"repair",
		0,
	)
	if err != nil {
		t.Fatalf("runtimeNamespaceWithHistory() error = %v", err)
	}
	rendered, err := refs.RenderTemplateString(
		"reattempt-history",
		"{{ .previous.nodes.repair.output.summary }}|{{ .previous.verdicts.quality.blocking_issues }}",
		namespace,
	)
	if err != nil {
		t.Fatalf("RenderTemplateString() error = %v", err)
	}
	if rendered != "|[]" {
		t.Fatalf("RenderTemplateString() = %q, want %q", rendered, "|[]")
	}
}

func testGenerationHistoryFanOutScope(t *testing.T) {
	t.Parallel()

	history, err := ProjectGenerationHistory(
		2,
		Run{},
		[]GenerationOutput{
			{NodeID: "draft", ItemIndex: 0, Status: generationOutputSucceeded, OutputRef: `{"item":"first"}`},
			{NodeID: "draft", ItemIndex: 1, Status: generationOutputSucceeded, OutputRef: `{"item":"second"}`},
			{NodeID: "quality", ItemIndex: 0, Status: generationOutputFailed, OutputRef: `{"item":"first"}`},
			{NodeID: "quality", ItemIndex: 1, Status: generationOutputSucceeded, OutputRef: `{"item":"second"}`},
		},
		[]gate.VerdictRecord{
			{
				GateID: "quality", ItemIndex: 0, Outcome: gate.VerdictOutcomeRejected,
				BlockingIssues: []byte(`[]`), Criteria: []byte(`[]`),
			},
			{
				GateID: "quality", ItemIndex: 1, Outcome: gate.VerdictOutcomeApproved,
				BlockingIssues: []byte(`[]`), Criteria: []byte(`[]`),
			},
		},
		[]gate.VerdictRecord{{GateID: "quality", ItemIndex: 0}},
		nil,
	)
	if err != nil {
		t.Fatalf("ProjectGenerationHistory() error = %v", err)
	}
	graph := dsl.Graph{
		Nodes: []dsl.Node{
			{ID: "fan", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut)},
			{ID: "draft", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
			{ID: "quality", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate)},
			{ID: "collect", Class: dsl.NodeClassControl, Kind: string(dsl.ControlCollect)},
		},
		Edges: []dsl.Edge{{From: "fan", To: "draft"}, {From: "draft", To: "quality"}, {From: "quality", To: "collect"}},
	}
	namespace := history.previousNamespace(newControlTopology(graph), "quality", 0)
	nodes, ok := namespace[namespaceNodesKey].(map[string]any)
	if !ok {
		t.Fatalf("previous namespace nodes = %#v, want object", namespace[namespaceNodesKey])
	}
	draftNode, ok := nodes["draft"].(map[string]any)
	if !ok {
		t.Fatalf("previous draft node = %#v, want object", nodes["draft"])
	}
	draft, ok := draftNode[namespaceOutputKey].(map[string]any)
	if !ok {
		t.Fatalf("previous draft output = %#v, want object", draftNode[namespaceOutputKey])
	}
	if got, want := draft["item"], "first"; got != want {
		t.Fatalf("scoped previous draft item = %v, want %q", got, want)
	}
	verdicts, ok := namespace[namespaceVerdictsKey].(map[string]any)
	if !ok {
		t.Fatalf("previous namespace verdicts = %#v, want object", namespace[namespaceVerdictsKey])
	}
	quality, ok := verdicts["quality"].(map[string]any)
	if !ok {
		t.Fatalf("previous quality verdict = %#v, want object", verdicts["quality"])
	}
	if got, want := quality[namespaceOutcomeKey], gate.VerdictOutcomeRejected; got != want {
		t.Fatalf("scoped previous verdict = %v, want %q", got, want)
	}
}

func testGenerationHistoryBestGeneration(t *testing.T) {
	t.Parallel()

	bestGeneration := int64(1)
	bestScore := 0.91
	reader := generationHistoryReaderStub{
		outputs: map[int][]GenerationOutput{
			1: {{NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"summary":"best draft"}`}},
			2: {{NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"summary":"latest draft"}`}},
		},
		verdicts:    map[int][]gate.VerdictRecord{2: {}},
		routeCauses: map[int][]gate.VerdictRecord{2: {}},
	}

	history, err := ReadGenerationHistory(
		context.Background(),
		reader,
		Run{
			ID:             "run-1",
			WorkspaceID:    "workspace-1",
			BestGeneration: &bestGeneration,
			BestScore:      &bestScore,
		},
		3,
	)
	if err != nil {
		t.Fatalf("ReadGenerationHistory() error = %v", err)
	}
	if history.Best == nil {
		t.Fatal("history.Best = nil, want best generation projection")
	}
	if got, want := history.Best.Generation, bestGeneration; got != want {
		t.Fatalf("history.Best.Generation = %d, want %d", got, want)
	}
	if got, want := history.Best.Score, bestScore; got != want {
		t.Fatalf("history.Best.Score = %v, want %v", got, want)
	}
	bestNodes := history.Best.Nodes["draft"]
	if len(bestNodes) == 0 {
		t.Fatalf("best draft outputs = %#v, want one output", bestNodes)
	}
	bestOutput, ok := bestNodes[0].Output.(map[string]any)
	if !ok {
		t.Fatalf("best draft output = %#v, want object", bestNodes[0].Output)
	}
	if got, want := bestOutput["summary"], "best draft"; got != want {
		t.Fatalf("best draft output = %v, want %q", got, want)
	}
	namespace := history.bestNamespace(controlTopology{}, "", 0)
	namespaceNodes, ok := namespace[namespaceNodesKey].(map[string]any)
	if !ok {
		t.Fatalf("best namespace nodes = %#v, want object", namespace[namespaceNodesKey])
	}
	node, ok := namespaceNodes["draft"].(map[string]any)
	if !ok {
		t.Fatalf("best namespace node = %#v, want object", namespaceNodes["draft"])
	}
	if _, exists := node[namespaceStatusKey]; exists {
		t.Fatalf("best namespace node = %#v, status must be absent", node)
	}
}

func testGenerationHistoryBestValidation(t *testing.T) {
	t.Parallel()

	validGeneration := int64(1)
	invalidGeneration := int64(0)
	validScore := 0.8
	nonFiniteScore := math.Inf(1)
	tests := []struct {
		name string
		run  Run
	}{
		{
			name: "Should reject best generation without best score",
			run:  Run{BestGeneration: &validGeneration},
		},
		{
			name: "Should reject best generation below one",
			run:  Run{BestGeneration: &invalidGeneration, BestScore: &validScore},
		},
		{
			name: "Should reject a non-finite best score",
			run:  Run{BestGeneration: &validGeneration, BestScore: &nonFiniteScore},
		},
	}
	for index := range tests {
		tt := &tests[index]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ProjectGenerationHistory(2, tt.run, nil, nil, nil, nil)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("ProjectGenerationHistory() error = %v, want ErrValidation", err)
			}
		})
	}
}

func testGenerationHistoryScalarBestOutput(t *testing.T) {
	t.Parallel()

	bestGeneration := int64(1)
	bestScore := 0.5
	history, err := ProjectGenerationHistory(
		2,
		Run{BestGeneration: &bestGeneration, BestScore: &bestScore},
		nil,
		nil,
		nil,
		[]GenerationOutput{{NodeID: "score", OutputRef: `0.5`}},
	)
	if err != nil {
		t.Fatalf("ProjectGenerationHistory() error = %v", err)
	}
	if got, want := history.Best.Nodes["score"][0].Output, json.Number("0.5"); got != want {
		t.Fatalf("best scalar output = %#v, want %#v", got, want)
	}
}

func testGenerationHistoryContractVerdict(t *testing.T) {
	t.Parallel()

	reader := generationHistoryReaderStub{
		outputs: map[int][]GenerationOutput{
			2: {{NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"summary":"retry"}`}},
		},
		verdicts: map[int][]gate.VerdictRecord{
			2: {{
				GateID:         "contract",
				Outcome:        gate.VerdictOutcomeRejected,
				BlockingIssues: []byte(`[]`),
				Criteria:       []byte(`[]`),
			}},
		},
		routeCauses: map[int][]gate.VerdictRecord{2: {{GateID: "contract"}}},
	}

	history, err := ReadGenerationHistory(
		context.Background(),
		reader,
		Run{ID: "run-1", WorkspaceID: "workspace-1"},
		3,
	)
	if err != nil {
		t.Fatalf("ReadGenerationHistory() error = %v", err)
	}
	if _, ok := history.Previous.Verdicts["contract"]; !ok {
		t.Fatalf("history.Previous.Verdicts = %#v, want contract verdict", history.Previous.Verdicts)
	}
	wantCauses := []GateInstanceProjection{{GateID: "contract"}}
	if got := history.Previous.RouteCauses; !reflect.DeepEqual(got, wantCauses) {
		t.Fatalf("history.Previous.RouteCauses = %#v, want %#v", got, wantCauses)
	}
}

func testGenerationHistoryNonPositiveGeneration(t *testing.T) {
	t.Parallel()

	_, err := ReadGenerationHistory(context.Background(), generationHistoryReaderStub{}, Run{}, 0)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ReadGenerationHistory() error = %v, want ErrValidation", err)
	}
}

func testGenerationHistoryTerminalNodes(t *testing.T) {
	t.Parallel()

	history, err := ProjectGenerationHistory(
		2,
		Run{},
		[]GenerationOutput{
			{NodeID: "quarantined", Status: "quarantined", OutputRef: `{"last":"known"}`},
			{NodeID: "canceled", Status: "canceled"},
		},
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("ProjectGenerationHistory() error = %v", err)
	}
	quarantined, ok := history.Previous.Nodes["quarantined"][0].Output.(map[string]any)
	if !ok || quarantined["last"] != "known" {
		t.Fatalf("quarantined output = %#v, want last recorded output", history.Previous.Nodes["quarantined"][0])
	}
	if got := history.Previous.Nodes["canceled"][0].Output; got != nil {
		t.Fatalf("canceled output = %#v, want absent", got)
	}
}

type generationHistoryReaderStub struct {
	outputs     map[int][]GenerationOutput
	payloads    map[GenerationOutputPayloadKey]json.RawMessage
	verdicts    map[int][]gate.VerdictRecord
	routeCauses map[int][]gate.VerdictRecord
	attempts    []NodeAttempt
}

func (r generationHistoryReaderStub) GetGenerationOutputPayload(
	_ context.Context,
	key GenerationOutputPayloadKey,
) (json.RawMessage, error) {
	payload, ok := r.payloads[key]
	if !ok {
		return nil, ErrOutputRefNotFound
	}
	return cloneRawMessage(payload), nil
}

func (r generationHistoryReaderStub) ListNodeAttempts(
	_ context.Context,
	_ WorkspaceID,
	_ RunID,
) ([]NodeAttempt, error) {
	return append([]NodeAttempt(nil), r.attempts...), nil
}

func (r generationHistoryReaderStub) ListGenerationOutputs(
	_ context.Context,
	_ WorkspaceID,
	_ RunID,
	generation int,
) ([]GenerationOutput, error) {
	return append([]GenerationOutput(nil), r.outputs[generation]...), nil
}

func (r generationHistoryReaderStub) ListGateVerdicts(
	_ context.Context,
	_ string,
	_ string,
	generation int64,
) ([]gate.VerdictRecord, error) {
	return append([]gate.VerdictRecord(nil), r.verdicts[int(generation)]...), nil
}

func (r generationHistoryReaderStub) ListRouteCausingVerdicts(
	_ context.Context,
	_ string,
	_ string,
	generation int64,
) ([]gate.VerdictRecord, error) {
	return append([]gate.VerdictRecord(nil), r.routeCauses[int(generation)]...), nil
}
