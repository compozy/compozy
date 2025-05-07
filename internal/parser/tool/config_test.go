package tool

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Set test mode
	TestMode = true
	// Run tests
	m.Run()
}

func TestLoadTool(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		wantErr  bool
		validate func(*testing.T, *ToolConfig)
	}{
		{
			name:    "basic tool",
			fixture: "basic_tool.yaml",
			validate: func(t *testing.T, config *ToolConfig) {
				TestMode = true // Skip file existence check for valid test
				defer func() { TestMode = false }()

				require.NotNil(t, config.ID)
				require.NotNil(t, config.Description)
				require.NotNil(t, config.Execute)
				require.NotNil(t, config.InputSchema)
				require.NotNil(t, config.OutputSchema)
				require.NotNil(t, config.Env)
				require.NotNil(t, config.With)

				assert.Equal(t, ToolID("code-formatter"), *config.ID)
				assert.Equal(t, ToolDescription("A tool for formatting code"), *config.Description)
				assert.Equal(t, ToolExecute("./format.ts"), *config.Execute)
				assert.True(t, config.Execute.IsTypeScript())

				// Validate input schema
				schema := config.InputSchema.Schema
				assert.Equal(t, "object", schema.GetType())
				require.NotNil(t, schema.GetProperties())
				assert.Contains(t, schema.GetProperties(), "code")
				assert.Contains(t, schema.GetProperties(), "language")
				if required, ok := schema["required"].([]string); ok && len(required) > 0 {
					assert.Contains(t, required, "code")
				}

				// Validate output schema
				outSchema := config.OutputSchema.Schema
				assert.Equal(t, "object", outSchema.GetType())
				require.NotNil(t, outSchema.GetProperties())
				assert.Contains(t, outSchema.GetProperties(), "formatted_code")
				if required, ok := outSchema["required"].([]string); ok && len(required) > 0 {
					assert.Contains(t, required, "formatted_code")
				}

				// Validate env and with
				assert.Equal(t, "1.0.0", config.Env["FORMATTER_VERSION"])
				assert.Equal(t, 2, (*config.With)["indent_size"])
				assert.Equal(t, false, (*config.With)["use_tabs"])
			},
		},
		{
			name:    "package tool",
			fixture: "package_tool.yaml",
			validate: func(t *testing.T, config *ToolConfig) {
				TestMode = true // Skip file existence check for valid test
				defer func() { TestMode = false }()

				require.NotNil(t, config.ID)
				require.NotNil(t, config.Description)
				require.NotNil(t, config.InputSchema)
				require.NotNil(t, config.OutputSchema)
				require.NotNil(t, config.Env)
				require.NotNil(t, config.With)

				assert.Equal(t, ToolID("code-linter"), *config.ID)
				assert.Equal(t, ToolDescription("A tool for linting code"), *config.Description)

				// Validate input schema
				schema := config.InputSchema.Schema
				assert.Equal(t, "object", schema.GetType())
				require.NotNil(t, schema.GetProperties())
				assert.Contains(t, schema.GetProperties(), "code")
				assert.Contains(t, schema.GetProperties(), "language")
				if required, ok := schema["required"].([]string); ok && len(required) > 0 {
					assert.Contains(t, required, "code")
				}

				// Validate output schema
				outSchema := config.OutputSchema.Schema
				assert.Equal(t, "object", outSchema.GetType())
				require.NotNil(t, outSchema.GetProperties())
				assert.Contains(t, outSchema.GetProperties(), "issues")
				issues := outSchema.GetProperties()["issues"]
				assert.Equal(t, "array", issues.GetType())

				// Get the items from the schema
				if items, ok := (*issues)["items"].(map[string]any); ok {
					// Check properties directly from the items map
					if itemType, ok := items["type"].(string); ok {
						assert.Equal(t, "object", itemType)
					}

					if itemProps, ok := items["properties"].(map[string]any); ok {
						assert.Contains(t, itemProps, "line")
						assert.Contains(t, itemProps, "message")
						assert.Contains(t, itemProps, "severity")
					} else {
						t.Error("Item properties not found or not a map")
					}
				} else {
					t.Error("Items not found or not a map")
				}

				if required, ok := outSchema["required"].([]string); ok && len(required) > 0 {
					assert.Contains(t, required, "issues")
				}

				// Validate env and with
				assert.Equal(t, "8.0.0", config.Env["ESLINT_VERSION"])
				assert.Equal(t, 10, (*config.With)["max_warnings"])
				assert.Equal(t, true, (*config.With)["fix"])
			},
		},
		{
			name:    "invalid tool",
			fixture: "invalid_tool.yaml",
			wantErr: true,
			validate: func(t *testing.T, config *ToolConfig) {
				TestMode = false // Enable file existence check for invalid test
				err := config.Validate()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid tool execute path")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the test directory path
			_, filename, _, ok := runtime.Caller(0)
			require.True(t, ok)
			testDir := filepath.Dir(filename)

			// Setup test fixture using utils
			dstPath := utils.SetupFixture(t, testDir, tt.fixture)

			// Run the test
			config, err := Load(dstPath)
			if err != nil {
				if tt.wantErr {
					if tt.validate != nil {
						tt.validate(t, config)
					}
					return
				}
				require.NoError(t, err)
			}
			require.NotNil(t, config)

			// Validate the config
			err = config.Validate()
			if err != nil {
				if tt.wantErr {
					if tt.validate != nil {
						tt.validate(t, config)
					}
					return
				}
				require.NoError(t, err)
			}

			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestToolConfigValidation(t *testing.T) {
	toolID := ToolID("test-tool")
	tests := []struct {
		name    string
		config  *ToolConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Config",
			config: &ToolConfig{
				ID:  &toolID,
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: false,
		},
		{
			name: "Missing CWD",
			config: &ToolConfig{
				ID: func() *ToolID { id := ToolID("test-tool"); return &id }(),
			},
			wantErr: true,
			errMsg:  "current working directory is required for test-tool",
		},
		{
			name: "Invalid Package Reference",
			config: &ToolConfig{
				ID:  &toolID,
				Use: pkgref.NewPackageRefConfig("invalid"),
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Invalid package reference",
		},
		{
			name: "Invalid Execute Path",
			config: &ToolConfig{
				ID:      func() *ToolID { id := ToolID("test-tool"); return &id }(),
				Execute: func() *ToolExecute { e := ToolExecute("./nonexistent.ts"); return &e }(),
				cwd:     common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "invalid tool execute path: /test/path/nonexistent.ts",
		},
		{
			name: "Input Schema Not Allowed with ID Reference",
			config: &ToolConfig{
				ID:  &toolID,
				Use: pkgref.NewPackageRefConfig("tool(id=test-tool)"),
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
					},
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Input schema not allowed for reference type id",
		},
		{
			name: "Output Schema Not Allowed with File Reference",
			config: &ToolConfig{
				ID:  &toolID,
				Use: pkgref.NewPackageRefConfig("tool(file=basic_tool.yaml)"),
				OutputSchema: &schema.OutputSchema{
					Schema: schema.Schema{
						"type": "object",
					},
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Output schema not allowed for reference type file",
		},
		{
			name: "Both Schemas Not Allowed with Dep Reference",
			config: &ToolConfig{
				ID:  &toolID,
				Use: pkgref.NewPackageRefConfig("tool(dep=compozy/tools:test-tool)"),
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
					},
				},
				OutputSchema: &schema.OutputSchema{
					Schema: schema.Schema{
						"type": "object",
					},
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Input schema not allowed for reference type dep",
		},
		{
			name: "Valid With Params",
			config: &ToolConfig{
				ID: &toolID,
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type": "string",
							},
						},
					},
				},
				With: &common.WithParams{
					"name": "test",
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: false,
		},
		{
			name: "Invalid With Params",
			config: &ToolConfig{
				ID:      func() *ToolID { id := ToolID("test-tool"); return &id }(),
				Execute: func() *ToolExecute { e := ToolExecute("./test.ts"); return &e }(),
				cwd:     common.NewCWD("/test/path"),
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type": "string",
							},
						},
						"required": []string{"name"},
					},
				},
				With: &common.WithParams{
					"age": 42,
				},
			},
			wantErr: true,
			errMsg:  "with parameters invalid for test-tool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disable test mode for validation tests
			TestMode = false
			defer func() { TestMode = true }()

			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestToolConfigCWD(t *testing.T) {
	config := &ToolConfig{}

	// Test setting CWD
	config.SetCWD("/test/path")
	assert.Equal(t, "/test/path", config.GetCWD())

	// Test updating CWD
	config.SetCWD("/new/path")
	assert.Equal(t, "/new/path", config.GetCWD())
}

func TestToolConfigMerge(t *testing.T) {
	baseConfig := &ToolConfig{
		Env: common.EnvMap{
			"KEY1": "value1",
		},
		With: &common.WithParams{},
	}

	otherConfig := &ToolConfig{
		Env: common.EnvMap{
			"KEY2": "value2",
		},
		With: &common.WithParams{},
	}

	err := baseConfig.Merge(otherConfig)
	require.NoError(t, err)

	// Check that base config has both env variables
	assert.Equal(t, "value1", baseConfig.Env["KEY1"])
	assert.Equal(t, "value2", baseConfig.Env["KEY2"])
}

func TestToolExecuteIsTypeScript(t *testing.T) {
	tests := []struct {
		name     string
		execute  ToolExecute
		expected bool
	}{
		{
			name:     "TypeScript file",
			execute:  "./script.ts",
			expected: true,
		},
		{
			name:     "JavaScript file",
			execute:  "./script.js",
			expected: false,
		},
		{
			name:     "Python file",
			execute:  "./script.py",
			expected: false,
		},
		{
			name:     "No extension",
			execute:  "./script",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.execute.IsTypeScript())
		})
	}
}
