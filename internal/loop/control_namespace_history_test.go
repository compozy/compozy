package loop

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
)

// Suite: generation history projection
// Invariant: a namespace exposes only the durable previous and best generation context.
// Boundary IN: pure history projection and its reader-backed record assembly.
// Boundary OUT: coordinator wiring and persistence queries, owned by their canonical suites.

func TestGenerationHistoryProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should leave initial generation history and namespace roots empty", testGenerationHistoryInitialGeneration)
	t.Run("Should project every previous node and verdict", testGenerationHistoryPreviousGeneration)
	t.Run("Should scope fan-out history to the current item", testGenerationHistoryFanOutScope)
	t.Run("Should project best outputs without status", testGenerationHistoryBestGeneration)
	t.Run("Should project scalar best output", testGenerationHistoryScalarBestOutput)
	t.Run("Should include the contract verdict in route causes", testGenerationHistoryContractVerdict)
	t.Run("Should reject a non-positive generation", testGenerationHistoryNonPositiveGeneration)
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
	namespace, err := runtimeNamespaceWithHistory(
		Run{},
		1,
		dsl.Graph{},
		controlTopology{},
		nil,
		history,
		"",
		0,
	)
	if err != nil {
		t.Fatalf("runtimeNamespaceWithHistory() error = %v", err)
	}
	for _, key := range []string{"previous", "best"} {
		value, ok := namespace[key].(map[string]any)
		if !ok {
			t.Fatalf("namespace[%q] = %#v, want empty object", key, namespace[key])
		}
		if len(value) != 0 {
			t.Fatalf("namespace[%q] = %#v, want empty object", key, value)
		}
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
	nodes := namespace[namespaceNodesKey].(map[string]any)
	draft := nodes["draft"].(map[string]any)[namespaceOutputKey].(map[string]any)
	if got, want := draft["item"], "first"; got != want {
		t.Fatalf("scoped previous draft item = %v, want %q", got, want)
	}
	verdicts := namespace["verdicts"].(map[string]any)
	quality := verdicts["quality"].(map[string]any)
	if got, want := quality["outcome"], gate.VerdictOutcomeRejected; got != want {
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
	bestOutput := history.Best.Nodes["draft"][0].Output.(map[string]any)
	if got, want := bestOutput["summary"], "best draft"; got != want {
		t.Fatalf("best draft output = %v, want %q", got, want)
	}
	namespace := history.bestNamespace(controlTopology{}, "", 0)
	node, ok := namespace["nodes"].(map[string]any)["draft"].(map[string]any)
	if !ok {
		t.Fatalf("best namespace node = %#v, want object", namespace["nodes"])
	}
	if _, exists := node["status"]; exists {
		t.Fatalf("best namespace node = %#v, status must be absent", node)
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

type generationHistoryReaderStub struct {
	outputs     map[int][]GenerationOutput
	verdicts    map[int][]gate.VerdictRecord
	routeCauses map[int][]gate.VerdictRecord
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
