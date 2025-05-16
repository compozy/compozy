package common

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Parse(t *testing.T) {
	t.Run("Should parse valid agent id reference", func(t *testing.T) {
		input := "agent(id=my-agent)"
		want := &PackageRef{
			Component: ComponentAgent,
			Type: &RefType{
				Type:  "id",
				Value: "my-agent",
			},
		}

		got, err := Parse(NewPackageRefConfig(input))
		require.NoError(t, err)
		assert.Equal(t, want.Component, got.Component)
		assert.Equal(t, want.Type.Type, got.Type.Type)
		assert.Equal(t, want.Type.Value, got.Type.Value)
	})

	t.Run("Should parse valid workflow dep reference", func(t *testing.T) {
		input := "workflow(dep=compozy/workflows:flow@v1.0.0)"
		want := &PackageRef{
			Component: ComponentWorkflow,
			Type: &RefType{
				Type:  "dep",
				Value: "compozy/workflows:flow@v1.0.0",
			},
		}

		got, err := Parse(NewPackageRefConfig(input))
		require.NoError(t, err)
		assert.Equal(t, want.Component, got.Component)
		assert.Equal(t, want.Type.Type, got.Type.Type)
		assert.Equal(t, want.Type.Value, got.Type.Value)
	})

	t.Run("Should return error for invalid format - missing type=value", func(t *testing.T) {
		input := "agent()"
		_, err := Parse(NewPackageRefConfig(input))
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should return error for invalid format - empty value", func(t *testing.T) {
		input := "agent(id=)"
		_, err := Parse(NewPackageRefConfig(input))
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should return error for invalid component", func(t *testing.T) {
		input := "invalid(id=test)"
		_, err := Parse(NewPackageRefConfig(input))
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should return error for invalid type", func(t *testing.T) {
		input := "agent(bad=test)"
		_, err := Parse(NewPackageRefConfig(input))
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})
}

func Test_Value(t *testing.T) {
	t.Run("Should return correct value for agent id", func(t *testing.T) {
		input := "agent(id=my-agent)"
		want := "my-agent"

		pkgRef, err := Parse(NewPackageRefConfig(input))
		require.NoError(t, err)
		assert.Equal(t, want, pkgRef.Value())
	})

	t.Run("Should return correct value for tool file", func(t *testing.T) {
		input := "tool(file=./tool.yaml)"
		want := "./tool.yaml"

		pkgRef, err := Parse(NewPackageRefConfig(input))
		require.NoError(t, err)
		assert.Equal(t, want, pkgRef.Value())
	})

	t.Run("Should return correct value for workflow dep", func(t *testing.T) {
		input := "workflow(dep=compozy/workflows:flow@v1.0.0)"
		want := "compozy/workflows:flow@v1.0.0"

		pkgRef, err := Parse(NewPackageRefConfig(input))
		require.NoError(t, err)
		assert.Equal(t, want, pkgRef.Value())
	})
}

func Test_SerializeDeserialize(t *testing.T) {
	t.Run("Should correctly serialize and deserialize agent id reference", func(t *testing.T) {
		input := "agent(id=my-agent)"
		pkgRef, err := Parse(NewPackageRefConfig(input))
		require.NoError(t, err)

		jsonData, err := json.Marshal(pkgRef)
		require.NoError(t, err)

		var got PackageRef
		err = json.Unmarshal(jsonData, &got)
		require.NoError(t, err)

		assert.Equal(t, pkgRef.Component, got.Component)
		assert.Equal(t, pkgRef.Type.Type, got.Type.Type)
		assert.Equal(t, pkgRef.Type.Value, got.Type.Value)
	})

	t.Run("Should correctly serialize and deserialize workflow dep reference", func(t *testing.T) {
		input := "workflow(dep=compozy/workflows:flow@v1.0.0)"
		pkgRef, err := Parse(NewPackageRefConfig(input))
		require.NoError(t, err)

		jsonData, err := json.Marshal(pkgRef)
		require.NoError(t, err)

		var got PackageRef
		err = json.Unmarshal(jsonData, &got)
		require.NoError(t, err)

		assert.Equal(t, pkgRef.Component, got.Component)
		assert.Equal(t, pkgRef.Type.Type, got.Type.Type)
		assert.Equal(t, pkgRef.Type.Value, got.Type.Value)
	})
}

func Test_PackageRefConfig(t *testing.T) {
	t.Run("Should create valid agent id config", func(t *testing.T) {
		input := "agent(id=my-agent)"
		want := "my-agent"

		config := NewPackageRefConfig(input)
		pkgRef, err := config.IntoRef()
		require.NoError(t, err)
		assert.Equal(t, want, pkgRef.Value())
	})

	t.Run("Should create valid workflow dep config", func(t *testing.T) {
		input := "workflow(dep=compozy/workflows:flow@v1.0.0)"
		want := "compozy/workflows:flow@v1.0.0"

		config := NewPackageRefConfig(input)
		pkgRef, err := config.IntoRef()
		require.NoError(t, err)
		assert.Equal(t, want, pkgRef.Value())
	})
}

func Test_ComponentMethods(t *testing.T) {
	t.Run("Should correctly identify agent component", func(t *testing.T) {
		component := ComponentAgent
		assert.True(t, component.IsAgent())
		assert.False(t, component.IsMcp())
		assert.False(t, component.IsTool())
		assert.False(t, component.IsTask())
		assert.False(t, component.IsWorkflow())
	})

	t.Run("Should correctly identify mcp component", func(t *testing.T) {
		component := ComponentMcp
		assert.False(t, component.IsAgent())
		assert.True(t, component.IsMcp())
		assert.False(t, component.IsTool())
		assert.False(t, component.IsTask())
		assert.False(t, component.IsWorkflow())
	})

	t.Run("Should correctly identify tool component", func(t *testing.T) {
		component := ComponentTool
		assert.False(t, component.IsAgent())
		assert.False(t, component.IsMcp())
		assert.True(t, component.IsTool())
		assert.False(t, component.IsTask())
		assert.False(t, component.IsWorkflow())
	})

	t.Run("Should correctly identify task component", func(t *testing.T) {
		component := ComponentTask
		assert.False(t, component.IsAgent())
		assert.False(t, component.IsMcp())
		assert.False(t, component.IsTool())
		assert.True(t, component.IsTask())
		assert.False(t, component.IsWorkflow())
	})

	t.Run("Should correctly identify workflow component", func(t *testing.T) {
		component := ComponentWorkflow
		assert.False(t, component.IsAgent())
		assert.False(t, component.IsMcp())
		assert.False(t, component.IsTool())
		assert.False(t, component.IsTask())
		assert.True(t, component.IsWorkflow())
	})
}

func Test_ParseEdgeCases(t *testing.T) {
	t.Run("Should return error for whitespace-only value", func(t *testing.T) {
		input := "agent(id= )"
		_, err := Parse(NewPackageRefConfig(input))
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should parse complex version string", func(t *testing.T) {
		input := "workflow(dep=owner/repo:pkg@ver-with-dashes)"
		_, err := Parse(NewPackageRefConfig(input))
		assert.NoError(t, err)
	})

	t.Run("Should return error for empty repo", func(t *testing.T) {
		input := "workflow(dep=owner/)"
		_, err := Parse(NewPackageRefConfig(input))
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should return error for empty owner", func(t *testing.T) {
		input := "workflow(dep=/repo)"
		_, err := Parse(NewPackageRefConfig(input))
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should parse missing package name", func(t *testing.T) {
		input := "workflow(dep=owner/repo@v1.0.0)"
		_, err := Parse(NewPackageRefConfig(input))
		assert.NoError(t, err)
	})

	t.Run("Should parse missing version", func(t *testing.T) {
		input := "workflow(dep=owner/repo:pkg)"
		_, err := Parse(NewPackageRefConfig(input))
		assert.NoError(t, err)
	})
}

func Test_ValidateFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "package-ref-test")
	require.NoError(t, err)
	defer func() {
		err := os.RemoveAll(tmpDir)
		assert.NoError(t, err, "Failed to remove temp dir")
	}()

	// Create a valid YAML file
	validYaml := filepath.Join(tmpDir, "valid.yaml")
	err = os.WriteFile(validYaml, []byte("test: data"), 0644)
	require.NoError(t, err)

	// Create an invalid extension file
	invalidExt := filepath.Join(tmpDir, "invalid.txt")
	err = os.WriteFile(invalidExt, []byte("test data"), 0644)
	require.NoError(t, err)

	t.Run("Should validate yaml file", func(t *testing.T) {
		ref := &RefType{
			Type:  "file",
			Value: "valid.yaml",
		}
		err := ref.Validate(validYaml)
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid extension", func(t *testing.T) {
		ref := &RefType{
			Type:  "file",
			Value: "invalid.txt",
		}
		err := ref.Validate(invalidExt)
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should return error for non-existent file", func(t *testing.T) {
		ref := &RefType{
			Type:  "file",
			Value: "nonexistent.yaml",
		}
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.yaml")
		err := ref.Validate(nonExistentPath)
		assert.Error(t, err)
	})
}

func Test_DeserializeInvalid(t *testing.T) {
	t.Run("Should return error for invalid component", func(t *testing.T) {
		input := `{"component":"invalid","type":"id=test"}`
		var got PackageRef
		err := json.Unmarshal([]byte(input), &got)
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should return error for invalid type format", func(t *testing.T) {
		input := `{"component":"agent","type":"invalid"}`
		var got PackageRef
		err := json.Unmarshal([]byte(input), &got)
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should return error for missing type field", func(t *testing.T) {
		input := `{"component":"agent"}`
		var got PackageRef
		err := json.Unmarshal([]byte(input), &got)
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should return error for missing component field", func(t *testing.T) {
		input := `{"type":"id=test"}`
		var got PackageRef
		err := json.Unmarshal([]byte(input), &got)
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})
}

func Test_PackageRefConfigInvalid(t *testing.T) {
	t.Run("Should return error for invalid format", func(t *testing.T) {
		input := "invalid(format)"
		config := NewPackageRefConfig(input)
		_, err := config.IntoRef()
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should return error for empty string", func(t *testing.T) {
		input := ""
		config := NewPackageRefConfig(input)
		_, err := config.IntoRef()
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Should return error for missing type=value", func(t *testing.T) {
		input := "agent()"
		config := NewPackageRefConfig(input)
		_, err := config.IntoRef()
		assert.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})
}
