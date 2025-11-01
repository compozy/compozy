package testutil

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestContextProvidesLoggerAndConfig(t *testing.T) {
	t.Parallel()
	t.Run("Should provide logger and config", func(t *testing.T) {
		t.Parallel()
		ctx := NewTestContext(t)
		require.NotNil(t, ctx.Done(), "expected context with cancellation support")
		require.NotNil(t, logger.FromContext(ctx), "expected logger in context")
		require.NotNil(t, config.FromContext(ctx), "expected configuration in context")
	})
}

func TestRequireNoError(t *testing.T) {
	t.Run("Should succeed when error is nil", func(t *testing.T) {
		RequireNoError(t, nil)
	})
	t.Run("Should report failure when error is present", func(t *testing.T) {
		prev := reportFailure
		called := false
		var message string
		reportFailure = func(_ *testing.T, format string, args ...any) {
			called = true
			message = fmt.Sprintf(format, args...)
		}
		t.Cleanup(func() {
			reportFailure = prev
		})
		RequireNoError(t, fmt.Errorf("boom"))
		require.True(t, called, "expected failure handler to be invoked")
		assert.Contains(t, message, "unexpected error")
	})
}

func TestRequireValidationError(t *testing.T) {
	t.Run("Should verify build error contains target", func(t *testing.T) {
		inner := fmt.Errorf("invalid value for field")
		be := &sdkerrors.BuildError{Errors: []error{inner}}
		RequireValidationError(t, be, "field")
	})
	t.Run("Should report failure when validation error missing", func(t *testing.T) {
		prev := reportFailure
		called := false
		var message string
		reportFailure = func(_ *testing.T, format string, args ...any) {
			called = true
			message = fmt.Sprintf(format, args...)
		}
		t.Cleanup(func() {
			reportFailure = prev
		})
		RequireValidationError(t, nil, "")
		require.True(t, called, "expected validation failure handler to run")
		assert.Contains(t, message, "expected validation error")
	})
}

func TestAssertBuildError(t *testing.T) {
	t.Parallel()
	t.Run("Should assert expected substrings", func(t *testing.T) {
		t.Parallel()
		be := &sdkerrors.BuildError{Errors: []error{fmt.Errorf("missing id"), fmt.Errorf("invalid name")}}
		AssertBuildError(t, be, []string{"missing", "invalid"})
	})
}

func TestNewTestModelDefaults(t *testing.T) {
	t.Parallel()
	t.Run("Should set defaults for provider and model", func(t *testing.T) {
		t.Parallel()
		model := NewTestModel("", "")
		require.Equal(t, enginecore.ProviderOpenAI, model.Provider)
		require.NotEmpty(t, model.Model)
		assert.Contains(t, model.APIKey, "TEST_API_KEY")
	})
}

func TestNewTestAgent(t *testing.T) {
	t.Parallel()
	t.Run("Should build agent with defaults", func(t *testing.T) {
		t.Parallel()
		agent := NewTestAgent("example-agent")
		require.Equal(t, "example-agent", agent.ID)
		require.NotEmpty(t, strings.TrimSpace(agent.Instructions))
		require.NotEmpty(t, agent.Model.Config.Model)
	})
}

func TestNewTestWorkflow(t *testing.T) {
	t.Parallel()
	t.Run("Should create workflow with defaults", func(t *testing.T) {
		t.Parallel()
		wf := NewTestWorkflow("workflow")
		require.Equal(t, "workflow", wf.ID)
		require.Len(t, wf.Agents, 1)
		require.Len(t, wf.Tasks, 1)
		require.NotNil(t, wf.Tasks[0].Agent)
		require.NotEmpty(t, wf.Tasks[0].Agent.ID)
		ctx := NewTestContext(t)
		require.NoError(t, wf.Validate(ctx))
		assert.Equal(t, wf.Agents[0].ID, wf.Tasks[0].Agent.ID)
	})
}

func TestRunTableTests(t *testing.T) {
	t.Parallel()
	t.Run("Should execute builders and validators", func(t *testing.T) {
		t.Parallel()
		executions := make([]string, 0, 2)
		table := []TableTest{
			{
				Name: "ok",
				BuildFunc: func(ctx context.Context) (any, error) {
					if logger.FromContext(ctx) == nil {
						return nil, errors.New("logger missing")
					}
					executions = append(executions, "ok")
					return "value", nil
				},
				Validate: func(t *testing.T, v any) {
					t.Helper()
					require.IsType(t, "", v)
					assert.Equal(t, "value", v)
				},
			},
			{
				Name:        "err",
				WantErr:     true,
				ErrContains: "boom",
				BuildFunc: func(_ context.Context) (any, error) {
					executions = append(executions, "err")
					return nil, fmt.Errorf("boom failure")
				},
			},
		}
		RunTableTests(t, table)
		require.Len(t, executions, 2)
	})
}

func TestAssertConfigEqual(t *testing.T) {
	t.Parallel()
	t.Run("Should compare maps without diff", func(t *testing.T) {
		t.Parallel()
		want := map[string]any{"k": "v"}
		got := map[string]any{"k": "v"}
		AssertConfigEqual(t, want, got)
	})
}
