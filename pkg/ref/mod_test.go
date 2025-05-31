package ref

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

func setupRefTest(t *testing.T) (string, string) {
	t.Helper()

	// Get the directory of this test file
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "failed to get current file path")

	testDir := filepath.Dir(filename)
	fixturesDir := filepath.Join(testDir, "fixtures")
	mainYAML := filepath.Join(fixturesDir, "main.yaml")

	// Verify fixtures exist
	_, err := os.Stat(mainYAML)
	require.NoError(t, err, "fixtures/main.yaml not found")

	return fixturesDir, mainYAML
}

func loadYAMLFile(t *testing.T, path string) Document {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read file %s", path)

	var doc any
	err = yaml.Unmarshal(data, &doc)
	require.NoError(t, err, "failed to parse YAML in %s", path)

	return &simpleDocument{data: doc}
}

// -----------------------------------------------------------------------------
// ParseRef Tests
// -----------------------------------------------------------------------------

func TestParseRef_StringForm(t *testing.T) {
	t.Run("Should parse property reference", func(t *testing.T) {
		node := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "schemas.#(id==\"city_input\")",
		}

		ref, err := ParseRef(node)
		require.NoError(t, err)
		assert.Equal(t, TypeProperty, ref.Type)
		assert.Equal(t, "schemas.#(id==\"city_input\")", ref.Path)
		assert.Equal(t, ModeMerge, ref.Mode)
		assert.Empty(t, ref.File)
	})

	t.Run("Should parse property reference with path", func(t *testing.T) {
		node := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "nested.level1.level2.value",
		}

		ref, err := ParseRef(node)
		require.NoError(t, err)
		assert.Equal(t, TypeProperty, ref.Type)
		assert.Equal(t, "nested.level1.level2.value", ref.Path)
	})

	t.Run("Should parse file reference", func(t *testing.T) {
		node := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "./external.yaml::schemas.#(id==\"user_input\")",
		}

		ref, err := ParseRef(node)
		require.NoError(t, err)
		assert.Equal(t, TypeFile, ref.Type)
		assert.Equal(t, "./external.yaml", ref.File)
		assert.Equal(t, "schemas.#(id==\"user_input\")", ref.Path)
	})

	t.Run("Should parse global reference", func(t *testing.T) {
		node := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "$global::providers.#(id==\"groq_llama\")",
		}

		ref, err := ParseRef(node)
		require.NoError(t, err)
		assert.Equal(t, TypeGlobal, ref.Type)
		assert.Equal(t, "providers.#(id==\"groq_llama\")", ref.Path)
		assert.Empty(t, ref.File)
	})

	t.Run("Should parse reference with mode", func(t *testing.T) {
		node := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "schemas.#(id==\"city_input\")!replace",
		}

		ref, err := ParseRef(node)
		require.NoError(t, err)
		assert.Equal(t, TypeProperty, ref.Type)
		assert.Equal(t, "schemas.#(id==\"city_input\")", ref.Path)
		assert.Equal(t, ModeReplace, ref.Mode)
	})

	t.Run("Should parse empty string as property reference", func(t *testing.T) {
		node := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "",
		}

		ref, err := ParseRef(node)
		require.NoError(t, err)
		assert.Equal(t, TypeProperty, ref.Type)
		assert.Empty(t, ref.Path)
		assert.Equal(t, ModeMerge, ref.Mode)
	})

	t.Run("Should parse URL file reference", func(t *testing.T) {
		node := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "https://example.com/api.yaml::schemas.0",
		}

		ref, err := ParseRef(node)
		require.NoError(t, err)
		assert.Equal(t, TypeFile, ref.Type)
		assert.Equal(t, "https://example.com/api.yaml", ref.File)
		assert.Equal(t, "schemas.0", ref.Path)
	})
}

func TestParseRef_ObjectForm(t *testing.T) {
	t.Run("Should parse property reference object", func(t *testing.T) {
		yamlStr := `
type: property
path: schemas.#(id=="city_input")
mode: merge
`
		var node yaml.Node
		err := yaml.Unmarshal([]byte(yamlStr), &node)
		require.NoError(t, err)

		ref, err := ParseRef(node.Content[0])
		require.NoError(t, err)
		assert.Equal(t, TypeProperty, ref.Type)
		assert.Equal(t, "schemas.#(id==\"city_input\")", ref.Path)
		assert.Equal(t, ModeMerge, ref.Mode)
	})

	t.Run("Should parse file reference object", func(t *testing.T) {
		yamlStr := `
type: file
file: ./external.yaml
path: schemas.0
mode: replace
`
		var node yaml.Node
		err := yaml.Unmarshal([]byte(yamlStr), &node)
		require.NoError(t, err)

		ref, err := ParseRef(node.Content[0])
		require.NoError(t, err)
		assert.Equal(t, TypeFile, ref.Type)
		assert.Equal(t, "./external.yaml", ref.File)
		assert.Equal(t, "schemas.0", ref.Path)
		assert.Equal(t, ModeReplace, ref.Mode)
	})

	t.Run("Should parse global reference object", func(t *testing.T) {
		yamlStr := `
type: global
path: global_providers.#(id=="groq_llama")
`
		var node yaml.Node
		err := yaml.Unmarshal([]byte(yamlStr), &node)
		require.NoError(t, err)

		ref, err := ParseRef(node.Content[0])
		require.NoError(t, err)
		assert.Equal(t, TypeGlobal, ref.Type)
		assert.Equal(t, "global_providers.#(id==\"groq_llama\")", ref.Path)
		assert.Equal(t, ModeMerge, ref.Mode) // default
	})
}

func TestParseRef_Errors(t *testing.T) {
	t.Run("Should error on invalid mode", func(t *testing.T) {
		node := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "schemas.0!invalid",
		}

		_, err := ParseRef(node)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mode: invalid")
	})

	t.Run("Should error on missing type in object form", func(t *testing.T) {
		yamlStr := `
path: schemas.0
mode: merge
`
		var node yaml.Node
		err := yaml.Unmarshal([]byte(yamlStr), &node)
		require.NoError(t, err)

		_, err = ParseRef(node.Content[0])
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "type is required")
	})

	t.Run("Should error on unknown field in object form", func(t *testing.T) {
		yamlStr := `
type: property
path: schemas.0
unknown: value
`
		var node yaml.Node
		err := yaml.Unmarshal([]byte(yamlStr), &node)
		require.NoError(t, err)

		_, err = ParseRef(node.Content[0])
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown field 'unknown'")
	})

	t.Run("Should error on file type without file field", func(t *testing.T) {
		yamlStr := `
type: file
path: schemas.0
`
		var node yaml.Node
		err := yaml.Unmarshal([]byte(yamlStr), &node)
		require.NoError(t, err)

		_, err = ParseRef(node.Content[0])
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file type requires file field")
	})

	t.Run("Should error on property type with empty path", func(t *testing.T) {
		yamlStr := `
type: property
mode: merge
`
		var node yaml.Node
		err := yaml.Unmarshal([]byte(yamlStr), &node)
		require.NoError(t, err)

		_, err = ParseRef(node.Content[0])
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path is required for property type")
	})

	t.Run("Should error on invalid file path", func(t *testing.T) {
		yamlStr := `
type: file
file: invalid_file
path: schemas.0
`
		var node yaml.Node
		err := yaml.Unmarshal([]byte(yamlStr), &node)
		require.NoError(t, err)

		_, err = ParseRef(node.Content[0])
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid file path: invalid_file")
	})
}

// -----------------------------------------------------------------------------
// Path Walking Tests
// -----------------------------------------------------------------------------

func TestWalkGJSONPath(t *testing.T) {
	doc := &simpleDocument{data: map[string]any{
		"schemas": []any{
			map[string]any{
				"id":   "city_input",
				"type": "object",
			},
			map[string]any{
				"id":   "weather_output",
				"type": "object",
			},
		},
		"nested": map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"value": "deep_value",
					"array": []any{
						map[string]any{"name": "first", "value": 1},
						map[string]any{"name": "second", "value": 2},
					},
				},
			},
		},
	}}

	t.Run("Should walk to root with empty path", func(t *testing.T) {
		result, err := doc.Get("")
		require.NoError(t, err)
		assert.Equal(t, doc.data, result)
	})

	t.Run("Should walk to nested value", func(t *testing.T) {
		result, err := doc.Get("nested.level1.level2.value")
		require.NoError(t, err)
		assert.Equal(t, "deep_value", result)
	})

	t.Run("Should walk to array index", func(t *testing.T) {
		result, err := doc.Get("schemas.0.id")
		require.NoError(t, err)
		assert.Equal(t, "city_input", result)
	})

	t.Run("Should walk with array filter", func(t *testing.T) {
		result, err := doc.Get("schemas.#(id==\"weather_output\").type")
		require.NoError(t, err)
		assert.Equal(t, "object", result)
	})

	t.Run("Should walk with nested array filter", func(t *testing.T) {
		result, err := doc.Get("nested.level1.level2.array.#(name==\"first\").value")
		require.NoError(t, err)
		assert.Equal(t, float64(1), result)
	})

	t.Run("Should error on invalid path", func(t *testing.T) {
		_, err := doc.Get("nonexistent.path")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// -----------------------------------------------------------------------------
// Resolution Tests
// -----------------------------------------------------------------------------

func TestResolve_PropertyReference(t *testing.T) {
	fixturesDir, mainPath := setupRefTest(t)
	doc := loadYAMLFile(t, mainPath)

	t.Run("Should resolve simple property reference", func(t *testing.T) {
		ref := &Ref{
			Type: TypeProperty,
			Path: "schemas.#(id==\"city_input\")",
			Mode: ModeMerge,
		}

		ctx := context.Background()
		docData, err := doc.Get("")
		require.NoError(t, err)
		result, err := ref.Resolve(ctx, docData, mainPath, fixturesDir)
		require.NoError(t, err)

		schema, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "city_input", schema["id"])
		assert.Equal(t, "object", schema["type"])
	})

	t.Run("Should resolve nested property reference", func(t *testing.T) {
		ref := &Ref{
			Type: TypeProperty,
			Path: "nested.level1.level2.value",
			Mode: ModeMerge,
		}

		ctx := context.Background()
		docData, err := doc.Get("")
		require.NoError(t, err)
		result, err := ref.Resolve(ctx, docData, mainPath, fixturesDir)
		require.NoError(t, err)
		assert.Equal(t, "deep_value", result)
	})

	t.Run("Should resolve array element", func(t *testing.T) {
		ref := &Ref{
			Type: TypeProperty,
			Path: "test_arrays.base_items.0",
			Mode: ModeMerge,
		}

		ctx := context.Background()
		docData, err := doc.Get("")
		require.NoError(t, err)
		result, err := ref.Resolve(ctx, docData, mainPath, fixturesDir)
		require.NoError(t, err)
		assert.Equal(t, "item1", result)
	})

	t.Run("Should error on nonexistent path", func(t *testing.T) {
		ref := &Ref{
			Type: TypeProperty,
			Path: "nonexistent.path",
			Mode: ModeMerge,
		}

		ctx := context.Background()
		docData, err := doc.Get("")
		require.NoError(t, err)
		_, err = ref.Resolve(ctx, docData, mainPath, fixturesDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestResolve_FileReference(t *testing.T) {
	fixturesDir, mainPath := setupRefTest(t)
	doc := loadYAMLFile(t, mainPath)

	t.Run("Should resolve file reference", func(t *testing.T) {
		ref := &Ref{
			Type: TypeFile,
			File: "./external.yaml",
			Path: "external_schemas.#(id==\"user_input\")",
			Mode: ModeMerge,
		}

		ctx := context.Background()
		docData, err := doc.Get("")
		require.NoError(t, err)
		result, err := ref.Resolve(ctx, docData, mainPath, fixturesDir)
		require.NoError(t, err)

		schema, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user_input", schema["id"])
		assert.Equal(t, "object", schema["type"])
	})

	t.Run("Should error on nonexistent file", func(t *testing.T) {
		ref := &Ref{
			Type: TypeFile,
			File: "./nonexistent.yaml",
			Path: "some.path",
			Mode: ModeMerge,
		}

		ctx := context.Background()
		docData, err := doc.Get("")
		require.NoError(t, err)
		_, err = ref.Resolve(ctx, docData, mainPath, fixturesDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load file")
	})
}

func TestResolve_GlobalReference(t *testing.T) {
	fixturesDir, mainPath := setupRefTest(t)
	doc := loadYAMLFile(t, mainPath)

	t.Run("Should resolve global reference", func(t *testing.T) {
		ref := &Ref{
			Type: TypeGlobal,
			Path: "global_providers.#(id==\"groq_llama\")",
			Mode: ModeMerge,
		}

		ctx := context.Background()
		docData, err := doc.Get("")
		require.NoError(t, err)
		result, err := ref.Resolve(ctx, docData, mainPath, fixturesDir)
		require.NoError(t, err)

		provider, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "groq_llama", provider["id"])
		assert.Equal(t, "groq", provider["provider"])
	})
}

func TestResolve_ChainedFileReferences(t *testing.T) {
	fixturesDir, _ := setupRefTest(t)
	chainedDir := filepath.Join(fixturesDir, "chained")
	file1Path := filepath.Join(chainedDir, "superup", "upup", "up", "doc", "file1.yaml")

	// Load the first file
	file1Doc := loadYAMLFile(t, file1Path)

	t.Run("Should resolve chained YAML file references", func(t *testing.T) {
		// Test automatic resolution of the entire chain
		// file1.yaml -> file2.yaml -> file3.yaml -> file4.yaml

		// Get the raw data first to extract the $ref
		file1Data, err := file1Doc.Get("")
		require.NoError(t, err)
		rootData, ok := file1Data.(map[string]any)
		require.True(t, ok)
		dataSection, ok := rootData["data"].(map[string]any)
		require.True(t, ok)

		// Extract the $ref and resolve it
		refStr, ok := dataSection["$ref"].(string)
		require.True(t, ok)

		ref, err := parseStringRef(refStr)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := ref.Resolve(ctx, file1Data, file1Path, chainedDir)
		require.NoError(t, err)

		// The final result should be the data from file4.yaml after following the entire chain
		finalData, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "file4_value", finalData["value"])

		// There should be no $ref in the final result since file4.yaml has no references
		_, hasRef := finalData["$ref"]
		assert.False(t, hasRef, "Final resolved data should not contain $ref")
	})
}

// -----------------------------------------------------------------------------
// Merge Mode Tests
// -----------------------------------------------------------------------------

func TestApplyMergeMode(t *testing.T) {
	t.Run("Should replace with replace mode", func(t *testing.T) {
		ref := &Ref{Mode: ModeReplace}
		refValue := map[string]any{"a": 1, "b": 2}
		inlineValue := map[string]any{"c": 3}

		result, err := ref.ApplyMergeMode(refValue, inlineValue)
		require.NoError(t, err)
		assert.Equal(t, refValue, result)
	})

	t.Run("Should merge maps with merge mode", func(t *testing.T) {
		ref := &Ref{Mode: ModeMerge}
		refValue := map[string]any{"a": 1, "b": 2}
		inlineValue := map[string]any{"b": 3, "c": 4}

		result, err := ref.ApplyMergeMode(refValue, inlineValue)
		require.NoError(t, err)

		merged, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 1, merged["a"])
		assert.Equal(t, 2, merged["b"]) // ref wins
		assert.Equal(t, 4, merged["c"])
	})

	t.Run("Should merge nested maps with merge mode", func(t *testing.T) {
		ref := &Ref{Mode: ModeMerge}
		refValue := map[string]any{
			"a": 1,
			"nested": map[string]any{
				"x": 10,
				"y": 20,
			},
		}
		inlineValue := map[string]any{
			"nested": map[string]any{
				"y": 30,
				"z": 40,
			},
			"b": 2,
		}

		result, err := ref.ApplyMergeMode(refValue, inlineValue)
		require.NoError(t, err)

		merged, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 1, merged["a"])
		assert.Equal(t, 2, merged["b"])
		nested, ok := merged["nested"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 10, nested["x"])
		assert.Equal(t, 20, nested["y"]) // ref wins
		assert.Equal(t, 40, nested["z"])
	})

	t.Run("Should merge arrays with merge mode", func(t *testing.T) {
		ref := &Ref{Mode: ModeMerge}
		refValue := []any{"a", "b"}
		inlineValue := []any{"c", "d"}

		result, err := ref.ApplyMergeMode(refValue, inlineValue)
		require.NoError(t, err)

		merged, ok := result.([]any)
		require.True(t, ok)
		// Arrays should be merged (union), with inline values first
		assert.Equal(t, []any{"c", "d", "a", "b"}, merged)
	})

	t.Run("Should append arrays with append mode", func(t *testing.T) {
		ref := &Ref{Mode: ModeAppend}
		refValue := []any{"a", "b"}
		inlineValue := []any{"c", "d"}

		result, err := ref.ApplyMergeMode(refValue, inlineValue)
		require.NoError(t, err)

		appended, ok := result.([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"c", "d", "a", "b"}, appended)
	})

	t.Run("Should error on append mode with non-arrays", func(t *testing.T) {
		ref := &Ref{Mode: ModeAppend}
		refValue := map[string]any{"a": 1}
		inlineValue := map[string]any{"b": 2}

		_, err := ref.ApplyMergeMode(refValue, inlineValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "append mode requires both values to be slices")
	})
}

// -----------------------------------------------------------------------------
// Circular Reference Tests
// -----------------------------------------------------------------------------

func TestResolve_CircularReference(t *testing.T) {
	fixturesDir, _ := setupRefTest(t)
	testRefsPath := filepath.Join(fixturesDir, "test_refs.yaml")
	testDoc := loadYAMLFile(t, testRefsPath)

	t.Run("Should detect circular reference", func(t *testing.T) {
		// Get the circular_a reference from test_refs.yaml
		testData, err := testDoc.Get("")
		require.NoError(t, err)
		data, ok := testData.(map[string]any)
		require.True(t, ok)
		circularA, ok := data["circular_a"].(map[string]any)
		require.True(t, ok, "circular_a not found in test document")

		refStr, ok := circularA["$ref"].(string)
		require.True(t, ok, "$ref not found in circular_a")

		ref, err := parseStringRef(refStr)
		require.NoError(t, err)

		// This should detect the circular reference
		ctx := context.Background()
		_, err = ref.Resolve(ctx, testData, testRefsPath, fixturesDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circular reference detected")
	})
}

// -----------------------------------------------------------------------------
// Integration Tests
// -----------------------------------------------------------------------------

func TestResolve_Integration(t *testing.T) {
	fixturesDir, _ := setupRefTest(t)
	testRefsPath := filepath.Join(fixturesDir, "test_refs.yaml")
	testDoc := loadYAMLFile(t, testRefsPath)
	mainPath := filepath.Join(fixturesDir, "main.yaml")
	mainDoc := loadYAMLFile(t, mainPath)

	t.Run("Should resolve property reference from test_refs.yaml", func(t *testing.T) {
		testData, err := testDoc.Get("")
		require.NoError(t, err)
		data, ok := testData.(map[string]any)
		require.True(t, ok)
		refs, ok := data["property_refs"].(map[string]any)
		require.True(t, ok)

		simpleRef, ok := refs["simple_ref"].(map[string]any)["$ref"].(string)
		require.True(t, ok)

		ref, err := parseStringRef(simpleRef)
		require.NoError(t, err)

		ctx := context.Background()
		mainData, err := mainDoc.Get("")
		require.NoError(t, err)
		result, err := ref.Resolve(ctx, mainData, testRefsPath, fixturesDir)
		require.NoError(t, err)

		schema, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "city_input", schema["id"])
	})
}
