package tool

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/parser/common"
	"github.com/compozy/compozy/parser/package_ref"
	"github.com/compozy/compozy/parser/testutils"
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
				schema := config.InputSchema
				assert.Equal(t, "object", schema.Type)
				require.NotNil(t, schema.Properties)
				assert.Contains(t, schema.Properties, "code")
				assert.Contains(t, schema.Properties, "language")
				require.NotNil(t, schema.Required)
				assert.Contains(t, schema.Required, "code")

				// Validate output schema
				outSchema := config.OutputSchema
				assert.Equal(t, "object", outSchema.Type)
				require.NotNil(t, outSchema.Properties)
				assert.Contains(t, outSchema.Properties, "formatted_code")
				require.NotNil(t, outSchema.Required)
				assert.Contains(t, outSchema.Required, "formatted_code")

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
				require.NotNil(t, config.Use)
				require.NotNil(t, config.InputSchema)
				require.NotNil(t, config.OutputSchema)
				require.NotNil(t, config.Env)
				require.NotNil(t, config.With)

				assert.Equal(t, ToolID("code-linter"), *config.ID)
				assert.Equal(t, ToolDescription("A tool for linting code"), *config.Description)
				assert.Equal(t, "tool(id=eslint)", string(*config.Use))

				// Validate input schema
				schema := config.InputSchema
				assert.Equal(t, "object", schema.Type)
				require.NotNil(t, schema.Properties)
				assert.Contains(t, schema.Properties, "code")
				assert.Contains(t, schema.Properties, "language")
				require.NotNil(t, schema.Required)
				assert.Contains(t, schema.Required, "code")

				// Validate output schema
				outSchema := config.OutputSchema
				assert.Equal(t, "object", outSchema.Type)
				require.NotNil(t, outSchema.Properties)
				assert.Contains(t, outSchema.Properties, "issues")
				issues := outSchema.Properties["issues"].(map[string]interface{})
				assert.Equal(t, "array", issues["type"])
				items := issues["items"].(map[string]interface{})
				assert.Equal(t, "object", items["type"])
				itemProps := items["properties"].(map[string]interface{})
				assert.Contains(t, itemProps, "line")
				assert.Contains(t, itemProps, "message")
				assert.Contains(t, itemProps, "severity")
				require.NotNil(t, outSchema.Required)
				assert.Contains(t, outSchema.Required, "issues")

				// Validate package reference
				require.NotNil(t, config.Use)
				ref, err := config.Use.IntoRef()
				require.NoError(t, err)
				assert.Equal(t, package_ref.ComponentTool, ref.Component)
				assert.Equal(t, "id", ref.Type.Type)
				assert.Equal(t, "eslint", ref.Type.Value)

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
				assert.Contains(t, err.Error(), "Invalid tool execute path")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the test directory path
			_, filename, _, ok := runtime.Caller(0)
			require.True(t, ok)
			testDir := filepath.Dir(filename)

			// Setup test fixture using testutils
			dstPath := testutils.SetupFixture(t, testDir, tt.fixture)

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
				ID: &toolID,
			},
			wantErr: true,
			errMsg:  "Missing file path for tool",
		},
		{
			name: "Invalid Package Reference",
			config: &ToolConfig{
				ID:  &toolID,
				Use: package_ref.NewPackageRefConfig("invalid"),
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Invalid package reference",
		},
		{
			name: "Invalid Execute Path",
			config: &ToolConfig{
				ID:      &toolID,
				Execute: func() *ToolExecute { e := ToolExecute("./nonexistent.ts"); return &e }(),
				cwd:     common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Invalid tool execute path",
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
