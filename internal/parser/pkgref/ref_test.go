package pkgref

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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

		got, err := Parse(input)
		if err != nil {
			t.Errorf("Parse() error = %v, want nil", err)
			return
		}

		if got.Component != want.Component {
			t.Errorf("Parse() component = %v, want %v", got.Component, want.Component)
		}
		if got.Type.Type != want.Type.Type {
			t.Errorf("Parse() type = %v, want %v", got.Type.Type, want.Type.Type)
		}
		if got.Type.Value != want.Type.Value {
			t.Errorf("Parse() value = %v, want %v", got.Type.Value, want.Type.Value)
		}
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

		got, err := Parse(input)
		if err != nil {
			t.Errorf("Parse() error = %v, want nil", err)
			return
		}

		if got.Component != want.Component {
			t.Errorf("Parse() component = %v, want %v", got.Component, want.Component)
		}
		if got.Type.Type != want.Type.Type {
			t.Errorf("Parse() type = %v, want %v", got.Type.Type, want.Type.Type)
		}
		if got.Type.Value != want.Type.Value {
			t.Errorf("Parse() value = %v, want %v", got.Type.Value, want.Type.Value)
		}
	})

	t.Run("Should return error for invalid format - missing type=value", func(t *testing.T) {
		input := "agent()"
		_, err := Parse(input)
		if err == nil {
			t.Error("Parse() error = nil, want error")
		}
		if err.Error() == "" {
			t.Error("Parse() error message is empty")
		}
	})

	t.Run("Should return error for invalid format - empty value", func(t *testing.T) {
		input := "agent(id=)"
		_, err := Parse(input)
		if err == nil {
			t.Error("Parse() error = nil, want error")
		}
		if err.Error() == "" {
			t.Error("Parse() error message is empty")
		}
	})

	t.Run("Should return error for invalid component", func(t *testing.T) {
		input := "invalid(id=test)"
		_, err := Parse(input)
		if err == nil {
			t.Error("Parse() error = nil, want error")
		}
		if err.Error() == "" {
			t.Error("Parse() error message is empty")
		}
	})

	t.Run("Should return error for invalid type", func(t *testing.T) {
		input := "agent(bad=test)"
		_, err := Parse(input)
		if err == nil {
			t.Error("Parse() error = nil, want error")
		}
		if err.Error() == "" {
			t.Error("Parse() error message is empty")
		}
	})
}

func Test_Value(t *testing.T) {
	t.Run("Should return correct value for agent id", func(t *testing.T) {
		input := "agent(id=my-agent)"
		want := "my-agent"

		pkgRef, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if got := pkgRef.Value(); got != want {
			t.Errorf("Value() = %v, want %v", got, want)
		}
	})

	t.Run("Should return correct value for tool file", func(t *testing.T) {
		input := "tool(file=./tool.yaml)"
		want := "./tool.yaml"

		pkgRef, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if got := pkgRef.Value(); got != want {
			t.Errorf("Value() = %v, want %v", got, want)
		}
	})

	t.Run("Should return correct value for workflow dep", func(t *testing.T) {
		input := "workflow(dep=compozy/workflows:flow@v1.0.0)"
		want := "compozy/workflows:flow@v1.0.0"

		pkgRef, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if got := pkgRef.Value(); got != want {
			t.Errorf("Value() = %v, want %v", got, want)
		}
	})
}

func Test_SerializeDeserialize(t *testing.T) {
	t.Run("Should correctly serialize and deserialize agent id reference", func(t *testing.T) {
		input := "agent(id=my-agent)"
		pkgRef, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		jsonData, err := json.Marshal(pkgRef)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		var got PackageRef
		if err := json.Unmarshal(jsonData, &got); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if pkgRef.Component != got.Component {
			t.Errorf("Component = %v, want %v", got.Component, pkgRef.Component)
		}
		if pkgRef.Type.Type != got.Type.Type {
			t.Errorf("Type.Type = %v, want %v", got.Type.Type, pkgRef.Type.Type)
		}
		if pkgRef.Type.Value != got.Type.Value {
			t.Errorf("Type.Value = %v, want %v", got.Type.Value, pkgRef.Type.Value)
		}
	})

	t.Run("Should correctly serialize and deserialize workflow dep reference", func(t *testing.T) {
		input := "workflow(dep=compozy/workflows:flow@v1.0.0)"
		pkgRef, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		jsonData, err := json.Marshal(pkgRef)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		var got PackageRef
		if err := json.Unmarshal(jsonData, &got); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if pkgRef.Component != got.Component {
			t.Errorf("Component = %v, want %v", got.Component, pkgRef.Component)
		}
		if pkgRef.Type.Type != got.Type.Type {
			t.Errorf("Type.Type = %v, want %v", got.Type.Type, pkgRef.Type.Type)
		}
		if pkgRef.Type.Value != got.Type.Value {
			t.Errorf("Type.Value = %v, want %v", got.Type.Value, pkgRef.Type.Value)
		}
	})
}

func Test_PackageRefConfig(t *testing.T) {
	t.Run("Should create valid agent id config", func(t *testing.T) {
		input := "agent(id=my-agent)"
		want := "my-agent"

		config := NewPackageRefConfig(input)
		pkgRef, err := config.IntoRef()
		if err != nil {
			t.Fatalf("IntoRef() error = %v", err)
		}

		if got := pkgRef.Value(); got != want {
			t.Errorf("Value() = %v, want %v", got, want)
		}
	})

	t.Run("Should create valid workflow dep config", func(t *testing.T) {
		input := "workflow(dep=compozy/workflows:flow@v1.0.0)"
		want := "compozy/workflows:flow@v1.0.0"

		config := NewPackageRefConfig(input)
		pkgRef, err := config.IntoRef()
		if err != nil {
			t.Fatalf("IntoRef() error = %v", err)
		}

		if got := pkgRef.Value(); got != want {
			t.Errorf("Value() = %v, want %v", got, want)
		}
	})
}

func Test_ComponentMethods(t *testing.T) {
	t.Run("Should correctly identify agent component", func(t *testing.T) {
		component := ComponentAgent
		if !component.IsAgent() {
			t.Error("IsAgent() = false, want true")
		}
		if component.IsMcp() {
			t.Error("IsMcp() = true, want false")
		}
		if component.IsTool() {
			t.Error("IsTool() = true, want false")
		}
		if component.IsTask() {
			t.Error("IsTask() = true, want false")
		}
		if component.IsWorkflow() {
			t.Error("IsWorkflow() = true, want false")
		}
	})

	t.Run("Should correctly identify mcp component", func(t *testing.T) {
		component := ComponentMcp
		if component.IsAgent() {
			t.Error("IsAgent() = true, want false")
		}
		if !component.IsMcp() {
			t.Error("IsMcp() = false, want true")
		}
		if component.IsTool() {
			t.Error("IsTool() = true, want false")
		}
		if component.IsTask() {
			t.Error("IsTask() = true, want false")
		}
		if component.IsWorkflow() {
			t.Error("IsWorkflow() = true, want false")
		}
	})

	t.Run("Should correctly identify tool component", func(t *testing.T) {
		component := ComponentTool
		if component.IsAgent() {
			t.Error("IsAgent() = true, want false")
		}
		if component.IsMcp() {
			t.Error("IsMcp() = true, want false")
		}
		if !component.IsTool() {
			t.Error("IsTool() = false, want true")
		}
		if component.IsTask() {
			t.Error("IsTask() = true, want false")
		}
		if component.IsWorkflow() {
			t.Error("IsWorkflow() = true, want false")
		}
	})

	t.Run("Should correctly identify task component", func(t *testing.T) {
		component := ComponentTask
		if component.IsAgent() {
			t.Error("IsAgent() = true, want false")
		}
		if component.IsMcp() {
			t.Error("IsMcp() = true, want false")
		}
		if component.IsTool() {
			t.Error("IsTool() = true, want false")
		}
		if !component.IsTask() {
			t.Error("IsTask() = false, want true")
		}
		if component.IsWorkflow() {
			t.Error("IsWorkflow() = true, want false")
		}
	})

	t.Run("Should correctly identify workflow component", func(t *testing.T) {
		component := ComponentWorkflow
		if component.IsAgent() {
			t.Error("IsAgent() = true, want false")
		}
		if component.IsMcp() {
			t.Error("IsMcp() = true, want false")
		}
		if component.IsTool() {
			t.Error("IsTool() = true, want false")
		}
		if component.IsTask() {
			t.Error("IsTask() = true, want false")
		}
		if !component.IsWorkflow() {
			t.Error("IsWorkflow() = false, want true")
		}
	})
}

func Test_ParseEdgeCases(t *testing.T) {
	t.Run("Should return error for whitespace-only value", func(t *testing.T) {
		input := "agent(id= )"
		_, err := Parse(input)
		if err == nil {
			t.Error("Parse() error = nil, want error")
		}
		if err.Error() == "" {
			t.Error("Parse() error message is empty")
		}
	})

	t.Run("Should parse complex version string", func(t *testing.T) {
		input := "workflow(dep=owner/repo:pkg@ver-with-dashes)"
		_, err := Parse(input)
		if err != nil {
			t.Errorf("Parse() error = %v, want nil", err)
		}
	})

	t.Run("Should return error for empty repo", func(t *testing.T) {
		input := "workflow(dep=owner/)"
		_, err := Parse(input)
		if err == nil {
			t.Error("Parse() error = nil, want error")
		}
		if err.Error() == "" {
			t.Error("Parse() error message is empty")
		}
	})

	t.Run("Should return error for empty owner", func(t *testing.T) {
		input := "workflow(dep=/repo)"
		_, err := Parse(input)
		if err == nil {
			t.Error("Parse() error = nil, want error")
		}
		if err.Error() == "" {
			t.Error("Parse() error message is empty")
		}
	})

	t.Run("Should parse missing package name", func(t *testing.T) {
		input := "workflow(dep=owner/repo@v1.0.0)"
		_, err := Parse(input)
		if err != nil {
			t.Errorf("Parse() error = %v, want nil", err)
		}
	})

	t.Run("Should parse missing version", func(t *testing.T) {
		input := "workflow(dep=owner/repo:pkg)"
		_, err := Parse(input)
		if err != nil {
			t.Errorf("Parse() error = %v, want nil", err)
		}
	})
}

func Test_ValidateFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "package-ref-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create a valid YAML file
	validYaml := filepath.Join(tmpDir, "valid.yaml")
	if err := os.WriteFile(validYaml, []byte("test: data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create an invalid extension file
	invalidExt := filepath.Join(tmpDir, "invalid.txt")
	if err := os.WriteFile(invalidExt, []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Run("Should validate yaml file", func(t *testing.T) {
		ref := &RefType{
			Type:  "file",
			Value: "valid.yaml",
		}
		err := ref.Validate(validYaml)
		if err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("Should return error for invalid extension", func(t *testing.T) {
		ref := &RefType{
			Type:  "file",
			Value: "invalid.txt",
		}
		err := ref.Validate(invalidExt)
		if err == nil {
			t.Error("Validate() error = nil, want error")
		}
		if err.Error() == "" {
			t.Error("Validate() error message is empty")
		}
	})

	t.Run("Should return error for non-existent file", func(t *testing.T) {
		ref := &RefType{
			Type:  "file",
			Value: "nonexistent.yaml",
		}
		err := ref.Validate(validYaml)
		if err == nil {
			t.Error("Validate() error = nil, want error")
		}
		if err.Error() == "" {
			t.Error("Validate() error message is empty")
		}
	})
}

func Test_DeserializeInvalid(t *testing.T) {
	t.Run("Should return error for invalid component", func(t *testing.T) {
		input := `{"component":"invalid","type":"id=test"}`
		var got PackageRef
		err := json.Unmarshal([]byte(input), &got)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
		if err.Error() == "" {
			t.Error("Unmarshal() error message is empty")
		}
	})

	t.Run("Should return error for invalid type format", func(t *testing.T) {
		input := `{"component":"agent","type":"invalid"}`
		var got PackageRef
		err := json.Unmarshal([]byte(input), &got)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
		if err.Error() == "" {
			t.Error("Unmarshal() error message is empty")
		}
	})

	t.Run("Should return error for missing type field", func(t *testing.T) {
		input := `{"component":"agent"}`
		var got PackageRef
		err := json.Unmarshal([]byte(input), &got)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
		if err.Error() == "" {
			t.Error("Unmarshal() error message is empty")
		}
	})

	t.Run("Should return error for missing component field", func(t *testing.T) {
		input := `{"type":"id=test"}`
		var got PackageRef
		err := json.Unmarshal([]byte(input), &got)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
		if err.Error() == "" {
			t.Error("Unmarshal() error message is empty")
		}
	})
}

func Test_PackageRefConfigInvalid(t *testing.T) {
	t.Run("Should return error for invalid format", func(t *testing.T) {
		input := "invalid(format)"
		config := NewPackageRefConfig(input)
		_, err := config.IntoRef()
		if err == nil {
			t.Error("Expected error for invalid config, got nil")
		}
		if err.Error() == "" {
			t.Error("IntoRef() error message is empty")
		}
	})

	t.Run("Should return error for empty string", func(t *testing.T) {
		input := ""
		config := NewPackageRefConfig(input)
		_, err := config.IntoRef()
		if err == nil {
			t.Error("Expected error for invalid config, got nil")
		}
		if err.Error() == "" {
			t.Error("IntoRef() error message is empty")
		}
	})

	t.Run("Should return error for missing type=value", func(t *testing.T) {
		input := "agent()"
		config := NewPackageRefConfig(input)
		_, err := config.IntoRef()
		if err == nil {
			t.Error("Expected error for invalid config, got nil")
		}
		if err.Error() == "" {
			t.Error("IntoRef() error message is empty")
		}
	})
}
