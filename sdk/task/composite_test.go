package task

import (
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompositeBuilderBuildSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		builder *CompositeBuilder
		expect  func(t *testing.T, cfg *enginetask.Config)
	}{
		{
			name: "Composite task with workflow reference",
			builder: NewComposite("compose-user").
				WithWorkflow("user-onboarding"),
			expect: func(t *testing.T, cfg *enginetask.Config) {
				t.Helper()
				assert.Equal(t, enginetask.TaskTypeComposite, cfg.Type)
				assert.Equal(t, "user-onboarding", cfg.Action)
				assert.Equal(t, string(core.ConfigTask), cfg.Resource)
				assert.Nil(t, cfg.With)
			},
		},
		{
			name: "Composite task with input mapping",
			builder: NewComposite("compose-report").
				WithWorkflow("report-workflow").
				WithInput(map[string]string{"user": "{{ .workflow.input.user }}"}),
			expect: func(t *testing.T, cfg *enginetask.Config) {
				t.Helper()
				require.NotNil(t, cfg.With)
				assert.Equal(t, "{{ .workflow.input.user }}", (*cfg.With)["user"])
			},
		},
		{
			name: "Input mapping with templates",
			builder: NewComposite("compose-invoice").
				WithWorkflow("invoice-generator").
				WithInput(map[string]string{"amount": "{{ printf \"%.2f\" .workflow.input.total }}"}),
			expect: func(t *testing.T, cfg *enginetask.Config) {
				t.Helper()
				require.NotNil(t, cfg.With)
				assert.Equal(t, "{{ printf \"%.2f\" .workflow.input.total }}", (*cfg.With)["amount"])
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := tc.builder.Build(t.Context())
			require.NoError(t, err)
			require.NotNil(t, cfg)
			tc.expect(t, cfg)
		})
	}
}

func TestCompositeBuilderRequiresWorkflowID(t *testing.T) {
	t.Parallel()

	_, err := NewComposite("missing-workflow").Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.ErrorContains(t, buildErr, "workflow id is required")
}

func TestCompositeBuilderRejectsEmptyWorkflowID(t *testing.T) {
	t.Parallel()

	_, err := NewComposite("empty-workflow").
		WithWorkflow("   ").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.ErrorContains(t, buildErr, "workflow id cannot be empty")
}

func TestCompositeBuilderAggregatesErrors(t *testing.T) {
	t.Parallel()

	_, err := NewComposite("   ").
		WithWorkflow("   ").
		WithInput(map[string]string{"": "value"}).
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotNil(t, buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 2)

	unwrapped := errors.Unwrap(buildErr)
	require.NotNil(t, unwrapped)
}
