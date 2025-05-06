package task

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/transition"
	"github.com/compozy/compozy/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMode is used to skip file existence checks in tests
var TestMode bool

func TestMain(m *testing.M) {
	// Set test mode
	TestMode = true
	// Run tests
	m.Run()
}

func TestLoadTask(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		wantErr  bool
		validate func(*testing.T, *TaskConfig)
	}{
		{
			name:    "basic task",
			fixture: "basic_task.yaml",
			validate: func(t *testing.T, config *TaskConfig) {
				TestMode = true // Skip file existence check for valid test
				defer func() { TestMode = false }()

				require.NotNil(t, config.ID)
				require.NotNil(t, config.Type)
				require.NotNil(t, config.Action)
				require.NotNil(t, config.InputSchema)
				require.NotNil(t, config.OutputSchema)
				require.NotNil(t, config.Env)
				require.NotNil(t, config.With)
				require.NotNil(t, config.OnSuccess)
				require.NotNil(t, config.OnError)

				assert.Equal(t, TaskID("code-format"), *config.ID)
				assert.Equal(t, TaskTypeBasic, config.Type)
				assert.Equal(t, "format-code", string(*config.Action))

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

				// Validate transitions
				assert.Equal(t, "next-task", *config.OnSuccess.Next)
				assert.Equal(t, "retry-task", *config.OnError.Next)
			},
		},
		{
			name:    "decision task",
			fixture: "decision_task.yaml",
			validate: func(t *testing.T, config *TaskConfig) {
				TestMode = true // Skip file existence check for valid test
				defer func() { TestMode = false }()

				require.NotNil(t, config.ID)
				require.NotNil(t, config.Type)
				require.NotEmpty(t, config.Condition)
				require.NotNil(t, config.Routes)
				require.NotNil(t, config.InputSchema)
				require.NotNil(t, config.OutputSchema)
				require.NotNil(t, config.Env)
				require.NotNil(t, config.With)
				require.NotNil(t, config.OnError)

				assert.Equal(t, TaskID("code-review"), *config.ID)
				assert.Equal(t, TaskTypeDecision, config.Type)
				assert.Equal(t, "review_score", string(config.Condition))
				assert.Equal(t, 3, len(config.Routes))

				// Validate routes
				assert.Equal(t, "deploy", string(config.Routes["approved"]))
				assert.Equal(t, "update-code", string(config.Routes["needs_changes"]))
				assert.Equal(t, "notify-team", string(config.Routes["rejected"]))

				// Validate input schema
				schema := config.InputSchema
				assert.Equal(t, "object", schema.Type)
				require.NotNil(t, schema.Properties)
				assert.Contains(t, schema.Properties, "code")
				assert.Contains(t, schema.Properties, "review_score")
				require.NotNil(t, schema.Required)
				assert.Contains(t, schema.Required, "code")
				assert.Contains(t, schema.Required, "review_score")

				// Validate output schema
				outSchema := config.OutputSchema
				assert.Equal(t, "object", outSchema.Type)
				require.NotNil(t, outSchema.Properties)
				assert.Contains(t, outSchema.Properties, "status")
				assert.Contains(t, outSchema.Properties, "comments")
				require.NotNil(t, outSchema.Required)
				assert.Contains(t, outSchema.Required, "status")

				// Validate env and with
				assert.Equal(t, "0.8", config.Env["REVIEW_THRESHOLD"])
				assert.Equal(t, 0.7, (*config.With)["min_score"])
				assert.Equal(t, 10, (*config.With)["max_comments"])

				// Validate error transition
				assert.Equal(t, "retry-task", *config.OnError.Next)
			},
		},
		{
			name:    "invalid task",
			fixture: "invalid_task.yaml",
			wantErr: true,
			validate: func(t *testing.T, config *TaskConfig) {
				TestMode = false // Enable file existence check for invalid test
				err := config.Validate()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "Basic task configuration is required for basic task type")
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

func TestTaskConfigValidation(t *testing.T) {
	taskID := TaskID("test-task")
	tests := []struct {
		name    string
		config  *TaskConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Basic Task",
			config: &TaskConfig{
				ID:     &taskID,
				Type:   TaskTypeBasic,
				Action: func() *agent.ActionID { a := agent.ActionID("test-action"); return &a }(),
				cwd:    common.NewCWD("/test/path"),
			},
			wantErr: false,
		},
		{
			name: "Valid Decision Task",
			config: &TaskConfig{
				ID:        &taskID,
				Type:      TaskTypeDecision,
				Condition: "test-condition",
				Routes: map[TaskRoute]TaskRoute{
					"route1": "next1",
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: false,
		},
		{
			name: "Missing CWD",
			config: &TaskConfig{
				ID: &taskID,
			},
			wantErr: true,
			errMsg:  "Current working directory is required for test-task",
		},
		{
			name: "Invalid Package Reference",
			config: &TaskConfig{
				ID:  &taskID,
				Use: pkgref.NewPackageRefConfig("invalid"),
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Invalid package reference",
		},
		{
			name: "Invalid Task Type",
			config: &TaskConfig{
				ID:   &taskID,
				Type: "invalid",
				cwd:  common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Invalid task type",
		},
		{
			name: "Basic Task Missing Configuration",
			config: &TaskConfig{
				ID:   &taskID,
				Type: TaskTypeBasic,
				cwd:  common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Basic task configuration is required for basic task type",
		},
		{
			name: "Decision Task Missing Configuration",
			config: &TaskConfig{
				ID:   &taskID,
				Type: TaskTypeDecision,
				cwd:  common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Decision task configuration is required for decision task type",
		},
		{
			name: "Decision Task Missing Routes",
			config: &TaskConfig{
				ID:        &taskID,
				Type:      TaskTypeDecision,
				Condition: "test-condition",
				Routes:    map[TaskRoute]TaskRoute{},
				cwd:       common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Decision task must have at least one route",
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

func TestTaskConfigCWD(t *testing.T) {
	config := &TaskConfig{}
	assert.Empty(t, config.GetCWD())

	config.SetCWD("/test/path")
	assert.Equal(t, "/test/path", config.GetCWD())

	config.SetCWD("/new/path")
	assert.Equal(t, "/new/path", config.GetCWD())
}

func TestTaskConfigMerge(t *testing.T) {
	next1 := "next1"
	next2 := "next2"
	base := &TaskConfig{
		Env: common.EnvMap{
			"KEY1": "value1",
		},
		With: &common.WithParams{
			"param1": "value1",
		},
		OnSuccess: &transition.SuccessTransitionConfig{
			Next: &next1,
		},
		OnError: &transition.ErrorTransitionConfig{
			Next: &next1,
		},
	}

	other := &TaskConfig{
		Env: common.EnvMap{
			"KEY2": "value2",
		},
		With: &common.WithParams{
			"param2": "value2",
		},
		OnSuccess: &transition.SuccessTransitionConfig{
			Next: &next2,
		},
		OnError: &transition.ErrorTransitionConfig{
			Next: &next2,
		},
	}

	err := base.Merge(other)
	require.NoError(t, err)

	// Check merged values
	assert.Equal(t, "value1", base.Env["KEY1"])
	assert.Equal(t, "value2", base.Env["KEY2"])
	assert.Equal(t, "value1", (*base.With)["param1"])
	assert.Equal(t, "value2", (*base.With)["param2"])
	assert.Equal(t, "next2", *base.OnSuccess.Next)
	assert.Equal(t, "next2", *base.OnError.Next)
}
