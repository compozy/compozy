package loop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/loop/dsl"
)

func TestFileImportOutputRefShouldRequireExplicitJSONOrTextParse(t *testing.T) {
	t.Parallel()

	t.Run("Should import JSON files", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "payload.json")
		if err := os.WriteFile(path, []byte(`{"name":"demo","items":[1,2]}`), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}

		ref, err := fileImportOutputRef(dsl.FileParseJSON, path)
		if err != nil {
			t.Fatalf("fileImportOutputRef(json) error = %v", err)
		}
		var payload struct {
			Name  string `json:"name"`
			Items []int  `json:"items"`
		}
		if err := json.Unmarshal([]byte(ref), &payload); err != nil {
			t.Fatalf("Unmarshal(json file-import output) error = %v", err)
		}
		if payload.Name != "demo" || len(payload.Items) != 2 || payload.Items[1] != 2 {
			t.Fatalf("json payload = %#v, want decoded source content", payload)
		}
	})

	t.Run("Should import text files", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "payload.txt")
		if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}

		ref, err := fileImportOutputRef(dsl.FileParseText, path)
		if err != nil {
			t.Fatalf("fileImportOutputRef(text) error = %v", err)
		}
		if got, want := ref, `{"text":"hello\n"}`; got != want {
			t.Fatalf("fileImportOutputRef(text) = %q, want %q", got, want)
		}
	})

	t.Run("Should reject unreadable JSON files", func(t *testing.T) {
		t.Parallel()

		_, err := fileImportOutputRef(dsl.FileParseJSON, filepath.Join(t.TempDir(), "missing.json"))
		if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "read file-import") {
			t.Fatalf("fileImportOutputRef(json missing) error = %v, want read failure", err)
		}
	})

	t.Run("Should reject malformed JSON files", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "payload.json")
		if err := os.WriteFile(path, []byte(`{"broken":`), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}

		_, err := fileImportOutputRef(dsl.FileParseJSON, path)
		if err == nil || !strings.Contains(err.Error(), "decode file-import JSON") {
			t.Fatalf("fileImportOutputRef(json malformed) error = %v, want decode failure", err)
		}
	})

	t.Run("Should reject unreadable text files", func(t *testing.T) {
		t.Parallel()

		_, err := fileImportOutputRef(dsl.FileParseText, filepath.Join(t.TempDir(), "missing.txt"))
		if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "read file-import") {
			t.Fatalf("fileImportOutputRef(text missing) error = %v, want read failure", err)
		}
	})

	for _, tc := range []struct {
		name  string
		parse dsl.FileParseKind
	}{
		{name: "Should reject empty parse", parse: ""},
		{name: "Should reject legacy task parse", parse: "md" + "_tasks"},
		{name: "Should reject unknown parse", parse: "yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := fileImportOutputRef(tc.parse, "unused")
			requireFileImportParseRequiredError(t, err)
		})
	}
}

func TestEvaluateFileImportNodeShouldRequireExplicitParse(t *testing.T) {
	t.Parallel()

	t.Run("Should import files into a succeeded generation output", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "payload.json")
		if err := os.WriteFile(path, []byte(`{"items":["task_01"]}`), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
		node := fileImportNodeForTest(path, dsl.FileParseJSON)
		graph := dsl.Graph{Nodes: []dsl.Node{node}}

		output, err := evaluateFileImportNode(
			Run{},
			1,
			&ResolvedDefinition{Definition: dsl.Definition{Graph: graph}},
			newControlTopology(graph),
			GenerationOutput{Generation: 1, NodeID: string(node.ID)},
			node,
			nil,
		)
		if err != nil {
			t.Fatalf("evaluateFileImportNode() error = %v", err)
		}
		if got, want := output.Status, generationOutputSucceeded; got != want {
			t.Fatalf("output status = %q, want %q", got, want)
		}
		var payload struct {
			Items []string `json:"items"`
		}
		if err := json.Unmarshal([]byte(output.OutputRef), &payload); err != nil {
			t.Fatalf("Unmarshal(output ref) error = %v", err)
		}
		if len(payload.Items) != 1 || payload.Items[0] != "task_01" {
			t.Fatalf("output payload = %#v, want task_01 item", payload)
		}
	})

	t.Run("Should reject missing patterns", func(t *testing.T) {
		t.Parallel()

		node := fileImportNodeForTest(" ", dsl.FileParseJSON)
		graph := dsl.Graph{Nodes: []dsl.Node{node}}

		_, err := evaluateFileImportNode(
			Run{},
			1,
			&ResolvedDefinition{Definition: dsl.Definition{Graph: graph}},
			newControlTopology(graph),
			GenerationOutput{Generation: 1, NodeID: string(node.ID)},
			node,
			nil,
		)
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "file-import pattern is required") {
			t.Fatalf("evaluateFileImportNode() error = %v, want pattern validation", err)
		}
	})

	t.Run("Should reject missing parse at runtime", func(t *testing.T) {
		t.Parallel()

		node := fileImportNodeForTest("unused", "")
		graph := dsl.Graph{Nodes: []dsl.Node{node}}

		_, err := evaluateFileImportNode(
			Run{},
			1,
			&ResolvedDefinition{Definition: dsl.Definition{Graph: graph}},
			newControlTopology(graph),
			GenerationOutput{Generation: 1, NodeID: string(node.ID)},
			node,
			nil,
		)
		requireFileImportParseRequiredError(t, err)
	})
}

func fileImportNodeForTest(pattern string, parse dsl.FileParseKind) dsl.Node {
	return dsl.Node{
		ID:      "load",
		Class:   dsl.NodeClassSource,
		Kind:    string(dsl.SourceFileImport),
		Pattern: pattern,
		Parse:   parse,
	}
}

func requireFileImportParseRequiredError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("fileImportOutputRef() error = %v, want ErrValidation", err)
	}
	want := ErrValidation.Error() + ": " + fileImportParseRequiredMessage
	if got := err.Error(); got != want {
		t.Fatalf("fileImportOutputRef() error = %q, want %q", got, want)
	}
	if strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("fileImportOutputRef() error = %q, want required parse message", err)
	}
}
