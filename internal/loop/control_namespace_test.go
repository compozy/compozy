package loop

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
)

func TestCollectionItemsShouldResolveFiniteFanOutArrays(t *testing.T) {
	t.Parallel()

	t.Run("Should decode JSON string collections", func(t *testing.T) {
		t.Parallel()

		items, err := collectionItems(`[{"id":"A"},{"id":"B"}]`)
		if err != nil {
			t.Fatalf("collectionItems() error = %v", err)
		}
		if got, want := len(items), 2; got != want {
			t.Fatalf("items len = %d, want %d", got, want)
		}
		first, ok := items[0].(map[string]any)
		if !ok {
			t.Fatalf("first item = %#v, want object", items[0])
		}
		if got, want := first["id"], "A"; got != want {
			t.Fatalf("first id = %v, want %q", got, want)
		}
	})

	t.Run("Should decode raw JSON collections", func(t *testing.T) {
		t.Parallel()

		items, err := collectionItems(json.RawMessage(`[1,2,3]`))
		if err != nil {
			t.Fatalf("collectionItems() error = %v", err)
		}
		if got, want := len(items), 3; got != want {
			t.Fatalf("items len = %d, want %d", got, want)
		}
	})

	t.Run("Should copy typed slice collections", func(t *testing.T) {
		t.Parallel()

		items, err := collectionItems([]string{"A", "B"})
		if err != nil {
			t.Fatalf("collectionItems() error = %v", err)
		}
		if got, want := len(items), 2; got != want {
			t.Fatalf("items len = %d, want %d", got, want)
		}
		if got, want := items[1], "B"; got != want {
			t.Fatalf("second item = %v, want %q", got, want)
		}
	})

	t.Run("Should reject non array collections", func(t *testing.T) {
		t.Parallel()

		_, err := collectionItems(`{"id":"A"}`)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("collectionItems() error = %v, want ErrValidation", err)
		}
	})
}

func TestResolveFanOutCollectionShouldRenderTemplateCollections(t *testing.T) {
	t.Parallel()

	t.Run("Should render template collections", func(t *testing.T) {
		t.Parallel()

		node := dsl.Node{
			ID:         "fan",
			Collection: `{{ json .nodes.load.output.items }}`,
		}
		namespace := map[string]any{
			"nodes": map[string]any{
				"load": map[string]any{
					"output": map[string]any{
						"items": []any{"A", "B"},
					},
				},
			},
		}

		items, err := resolveFanOutCollection(&ResolvedDefinition{}, node, namespace)
		if err != nil {
			t.Fatalf("resolveFanOutCollection() error = %v", err)
		}
		if got, want := len(items), 2; got != want {
			t.Fatalf("items len = %d, want %d", got, want)
		}
		if got, want := items[0], "A"; got != want {
			t.Fatalf("first item = %v, want %q", got, want)
		}
	})
}

func TestRuntimeNamespaceShouldScopeFanOutItemsByBatchSize(t *testing.T) {
	t.Parallel()

	graph := dsl.Graph{
		Nodes: []dsl.Node{
			{ID: "fan", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut)},
			{ID: "body", Class: dsl.NodeClassAction, Kind: "run-agent"},
		},
		Edges: []dsl.Edge{{From: "fan", To: "body"}},
	}
	topology := newControlTopology(graph)
	fanOutOutputs := func(t *testing.T, batchSize int, items []any) []GenerationOutput {
		t.Helper()
		materialization, terminal := buildFanOutMaterialization(dsl.Node{ID: "fan", BatchSize: batchSize}, items, 1)
		if terminal != nil {
			t.Fatalf("buildFanOutMaterialization() terminal = %#v, want nil", terminal)
		}
		ref, err := fanOutMaterializationRef(materialization)
		if err != nil {
			t.Fatalf("fanOutMaterializationRef() error = %v", err)
		}
		return []GenerationOutput{{NodeID: "fan", ItemIndex: 0, Status: "succeeded", OutputRef: ref}}
	}

	t.Run("Should expose the element itself when batch_size is 1", func(t *testing.T) {
		t.Parallel()

		outputs := fanOutOutputs(t, 1, []any{
			map[string]any{"title": "Task A"},
			map[string]any{"title": "Task B"},
		})
		namespace, err := runtimeNamespace(Run{}, 1, graph, topology, outputs, "body", 1)
		if err != nil {
			t.Fatalf("runtimeNamespace() error = %v", err)
		}
		item, ok := namespace["item"].(map[string]any)
		if !ok {
			t.Fatalf("namespace item = %#v, want the fanned element object", namespace["item"])
		}
		if got, want := item["title"], "Task B"; got != want {
			t.Fatalf("item title = %v, want %q", got, want)
		}
		rendered, err := refs.RenderTemplateString("body.prompt", "{{ .item.title }}", namespace)
		if err != nil {
			t.Fatalf("RenderTemplateString() error = %v", err)
		}
		if got, want := rendered, "Task B"; got != want {
			t.Fatalf("rendered prompt = %q, want %q", got, want)
		}
	})

	t.Run("Should expose the chunk slice when batch_size is greater than 1", func(t *testing.T) {
		t.Parallel()

		outputs := fanOutOutputs(t, 2, []any{
			map[string]any{"id": "A"},
			map[string]any{"id": "B"},
			map[string]any{"id": "C"},
		})
		namespace, err := runtimeNamespace(Run{}, 1, graph, topology, outputs, "body", 1)
		if err != nil {
			t.Fatalf("runtimeNamespace() error = %v", err)
		}
		chunk, ok := namespace["item"].([]any)
		if !ok {
			t.Fatalf("namespace item = %#v, want chunk slice even for a short final chunk", namespace["item"])
		}
		if got, want := len(chunk), 1; got != want {
			t.Fatalf("chunk len = %d, want %d", got, want)
		}
		rendered, err := refs.RenderTemplateString(
			"body.prompt",
			"{{ len .item }}:{{ range .item }}{{ .id }}{{ end }}",
			namespace,
		)
		if err != nil {
			t.Fatalf("RenderTemplateString() error = %v", err)
		}
		if got, want := rendered, "1:C"; got != want {
			t.Fatalf("rendered prompt = %q, want %q", got, want)
		}
	})
}

func TestOutputValueShouldPreserveExactJSONNumbers(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve large JSON numbers as json.Number", func(t *testing.T) {
		t.Parallel()

		value := outputValue(`{"id":9007199254740993}`)
		object, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("outputValue() = %#v, want object", value)
		}
		number, ok := object["id"].(json.Number)
		if !ok {
			t.Fatalf("outputValue().id = %#v, want json.Number", object["id"])
		}
		if got, want := number.String(), "9007199254740993"; got != want {
			t.Fatalf("outputValue().id = %q, want %q", got, want)
		}
	})
}
