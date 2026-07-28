package workref

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

type constructorCase struct {
	name      string
	id        string
	value     string
	wantID    string
	wantValue string
}

var constructorCases = []constructorCase{
	{
		name:      "Should trim leading and trailing whitespace",
		id:        "  workspace-01\t",
		value:     "\n /tmp/demo \t",
		wantID:    "workspace-01",
		wantValue: "/tmp/demo",
	},
	{
		name:      "Should preserve interior whitespace",
		id:        "workspace 01",
		value:     "/tmp/demo folder",
		wantID:    "workspace 01",
		wantValue: "/tmp/demo folder",
	},
	{
		name:      "Should collapse whitespace-only values to empty strings",
		id:        " \n\t ",
		value:     "\t ",
		wantID:    "",
		wantValue: "",
	},
}

func runConstructorCases[T comparable](
	t *testing.T,
	constructor func(string, string) T,
	want func(string, string) T,
) {
	t.Helper()

	for _, tc := range constructorCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := constructor(tc.id, tc.value)
			expected := want(tc.wantID, tc.wantValue)
			if got != expected {
				t.Fatalf("constructor(%q, %q) = %#v, want %#v", tc.id, tc.value, got, expected)
			}
		})
	}
}

func TestConstructors(t *testing.T) {
	t.Parallel()

	t.Run("Should construct PathRef with NewPath", func(t *testing.T) {
		t.Parallel()

		runConstructorCases(t, NewPath, func(id string, value string) PathRef {
			return PathRef{
				WorkspaceID:   id,
				WorkspacePath: value,
			}
		})
	})

	t.Run("Should construct RootRef with NewRoot", func(t *testing.T) {
		t.Parallel()

		runConstructorCases(t, NewRoot, func(id string, value string) RootRef {
			return RootRef{
				WorkspaceID: id,
				Workspace:   value,
			}
		})
	})
}

func TestSerializationContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve PathRef JSON and YAML field names", func(t *testing.T) {
		t.Parallel()

		ref := PathRef{
			WorkspaceID:   "workspace-01",
			WorkspacePath: "/tmp/demo",
		}
		assertJSONContract(t, ref, "{\"workspace_id\":\"workspace-01\",\"workspace_path\":\"/tmp/demo\"}")
		assertYAMLContract(t, ref, "workspace_id: workspace-01\nworkspace_path: /tmp/demo")
	})

	t.Run("Should preserve RootRef JSON and YAML field names", func(t *testing.T) {
		t.Parallel()

		ref := RootRef{
			WorkspaceID: "workspace-01",
			Workspace:   "/tmp/root",
		}
		assertJSONContract(t, ref, "{\"workspace_id\":\"workspace-01\",\"workspace\":\"/tmp/root\"}")
		assertYAMLContract(t, ref, "workspace_id: workspace-01\nworkspace: /tmp/root")
	})
}

func assertJSONContract[T comparable](t *testing.T, value T, want string) {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%#v) error = %v", value, err)
	}
	if string(encoded) != want {
		t.Fatalf("json.Marshal(%#v) = %s, want %s", value, encoded, want)
	}

	var decoded T
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", encoded, err)
	}
	if decoded != value {
		t.Fatalf("json round-trip = %#v, want %#v", decoded, value)
	}
}

func assertYAMLContract[T comparable](t *testing.T, value T, want string) {
	t.Helper()

	encoded, err := yaml.Marshal(value)
	if err != nil {
		t.Fatalf("yaml.Marshal(%#v) error = %v", value, err)
	}
	if got := strings.TrimSpace(string(encoded)); got != want {
		t.Fatalf("yaml.Marshal(%#v) = %q, want %q", value, got, want)
	}

	var decoded T
	if err := yaml.UnmarshalWithOptions(encoded, &decoded, yaml.Strict()); err != nil {
		t.Fatalf("yaml.Unmarshal(%s) error = %v", encoded, err)
	}
	if decoded != value {
		t.Fatalf("yaml round-trip = %#v, want %#v", decoded, value)
	}
}
