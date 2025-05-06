package package_ref

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *PackageRef
		wantErr bool
	}{
		{
			name:  "valid agent id reference",
			input: "agent(id=my-agent)",
			want: &PackageRef{
				Component: ComponentAgent,
				Type: &RefType{
					Type:  "id",
					Value: "my-agent",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid workflow dep reference",
			input: "workflow(dep=compozy/workflows:flow@v1.0.0)",
			want: &PackageRef{
				Component: ComponentWorkflow,
				Type: &RefType{
					Type:  "dep",
					Value: "compozy/workflows:flow@v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid format - missing type=value",
			input:   "agent()",
			wantErr: true,
		},
		{
			name:    "invalid format - empty value",
			input:   "agent(id=)",
			wantErr: true,
		},
		{
			name:    "invalid component",
			input:   "invalid(id=test)",
			wantErr: true,
		},
		{
			name:    "invalid type",
			input:   "agent(bad=test)",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.wantErr {
				// Check for structured error type
				if _, ok := err.(*PackageRefError); !ok {
					t.Errorf("Parse() error type = %T, want *PackageRefError", err)
				}
			}
			if !tt.wantErr {
				if got.Component != tt.want.Component {
					t.Errorf("Parse() component = %v, want %v", got.Component, tt.want.Component)
				}
				if got.Type.Type != tt.want.Type.Type {
					t.Errorf("Parse() type = %v, want %v", got.Type.Type, tt.want.Type.Type)
				}
				if got.Type.Value != tt.want.Type.Value {
					t.Errorf("Parse() value = %v, want %v", got.Type.Value, tt.want.Type.Value)
				}
			}
		})
	}
}

func TestValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "agent id value",
			input: "agent(id=my-agent)",
			want:  "my-agent",
		},
		{
			name:  "tool file value",
			input: "tool(file=./tool.yaml)",
			want:  "./tool.yaml",
		},
		{
			name:  "workflow dep value",
			input: "workflow(dep=compozy/workflows:flow@v1.0.0)",
			want:  "compozy/workflows:flow@v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgRef, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got := pkgRef.Value(); got != tt.want {
				t.Errorf("Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeDeserialize(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "agent id reference",
			input: "agent(id=my-agent)",
		},
		{
			name:  "workflow dep reference",
			input: "workflow(dep=compozy/workflows:flow@v1.0.0)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the input
			pkgRef, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			// Serialize to JSON
			jsonData, err := json.Marshal(pkgRef)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// Deserialize from JSON
			var got PackageRef
			if err := json.Unmarshal(jsonData, &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			// Compare the original and deserialized values
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
}

func TestPackageRefConfig(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "agent id config",
			input: "agent(id=my-agent)",
			want:  "my-agent",
		},
		{
			name:  "workflow dep config",
			input: "workflow(dep=compozy/workflows:flow@v1.0.0)",
			want:  "compozy/workflows:flow@v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config
			config := NewPackageRefConfig(tt.input)

			// Convert to ref
			pkgRef, err := config.IntoRef()
			if err != nil {
				t.Fatalf("IntoRef() error = %v", err)
			}

			// Check value
			if got := pkgRef.Value(); got != tt.want {
				t.Errorf("Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComponentMethods(t *testing.T) {
	tests := []struct {
		name       string
		component  Component
		isAgent    bool
		isMcp      bool
		isTool     bool
		isTask     bool
		isWorkflow bool
	}{
		{
			name:       "agent component",
			component:  ComponentAgent,
			isAgent:    true,
			isMcp:      false,
			isTool:     false,
			isTask:     false,
			isWorkflow: false,
		},
		{
			name:       "mcp component",
			component:  ComponentMcp,
			isAgent:    false,
			isMcp:      true,
			isTool:     false,
			isTask:     false,
			isWorkflow: false,
		},
		{
			name:       "tool component",
			component:  ComponentTool,
			isAgent:    false,
			isMcp:      false,
			isTool:     true,
			isTask:     false,
			isWorkflow: false,
		},
		{
			name:       "task component",
			component:  ComponentTask,
			isAgent:    false,
			isMcp:      false,
			isTool:     false,
			isTask:     true,
			isWorkflow: false,
		},
		{
			name:       "workflow component",
			component:  ComponentWorkflow,
			isAgent:    false,
			isMcp:      false,
			isTool:     false,
			isTask:     false,
			isWorkflow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.component.IsAgent(); got != tt.isAgent {
				t.Errorf("IsAgent() = %v, want %v", got, tt.isAgent)
			}
			if got := tt.component.IsMcp(); got != tt.isMcp {
				t.Errorf("IsMcp() = %v, want %v", got, tt.isMcp)
			}
			if got := tt.component.IsTool(); got != tt.isTool {
				t.Errorf("IsTool() = %v, want %v", got, tt.isTool)
			}
			if got := tt.component.IsTask(); got != tt.isTask {
				t.Errorf("IsTask() = %v, want %v", got, tt.isTask)
			}
			if got := tt.component.IsWorkflow(); got != tt.isWorkflow {
				t.Errorf("IsWorkflow() = %v, want %v", got, tt.isWorkflow)
			}
		})
	}
}

func TestParseEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "whitespace-only value",
			input:   "agent(id= )",
			wantErr: true,
		},
		{
			name:    "complex version string",
			input:   "workflow(dep=owner/repo:pkg@ver-with-dashes)",
			wantErr: false,
		},
		{
			name:    "empty repo",
			input:   "workflow(dep=owner/)",
			wantErr: true,
		},
		{
			name:    "empty owner",
			input:   "workflow(dep=/repo)",
			wantErr: true,
		},
		{
			name:    "missing package name",
			input:   "workflow(dep=owner/repo@v1.0.0)",
			wantErr: false,
		},
		{
			name:    "missing version",
			input:   "workflow(dep=owner/repo:pkg)",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*PackageRefError); !ok {
					t.Errorf("Parse() error type = %T, want *PackageRefError", err)
				}
			}
		})
	}
}

func TestValidateFile(t *testing.T) {
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

	tests := []struct {
		name     string
		filePath string
		refPath  string
		wantErr  bool
	}{
		{
			name:     "valid yaml file",
			filePath: validYaml,
			refPath:  "valid.yaml",
			wantErr:  false,
		},
		{
			name:     "invalid extension",
			filePath: invalidExt,
			refPath:  "invalid.txt",
			wantErr:  true,
		},
		{
			name:     "non-existent file",
			filePath: validYaml,
			refPath:  "nonexistent.yaml",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := &RefType{
				Type:  "file",
				Value: tt.refPath,
			}
			err := ref.Validate(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*PackageRefError); !ok {
					t.Errorf("Validate() error type = %T, want *PackageRefError", err)
				}
			}
		})
	}
}

func TestDeserializeInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid component",
			input: `{"component":"invalid","type":"id=test"}`,
		},
		{
			name:  "invalid type format",
			input: `{"component":"agent","type":"invalid"}`,
		},
		{
			name:  "missing type field",
			input: `{"component":"agent"}`,
		},
		{
			name:  "missing component field",
			input: `{"type":"id=test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got PackageRef
			err := json.Unmarshal([]byte(tt.input), &got)
			if err == nil {
				t.Error("Expected error for invalid JSON, got nil")
			} else {
				if _, ok := err.(*PackageRefError); !ok {
					t.Errorf("Unmarshal() error type = %T, want *PackageRefError", err)
				}
			}
		})
	}
}

func TestPackageRefConfigInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid format",
			input: "invalid(format)",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "missing type=value",
			input: "agent()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewPackageRefConfig(tt.input)
			_, err := config.IntoRef()
			if err == nil {
				t.Error("Expected error for invalid config, got nil")
			} else {
				if _, ok := err.(*PackageRefError); !ok {
					t.Errorf("IntoRef() error type = %T, want *PackageRefError", err)
				}
			}
		})
	}
}
