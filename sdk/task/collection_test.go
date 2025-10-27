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

func TestCollectionBuilderBuildSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		collection         string
		configure          func(*CollectionBuilder)
		expectedItemVar    string
		expectedCollection string
	}{
		{
			name:               "collection task with default item var",
			collection:         "{{ .input.items }}",
			configure:          func(*CollectionBuilder) {},
			expectedItemVar:    "item",
			expectedCollection: "{{ .input.items }}",
		},
		{
			name:       "collection task with custom item var",
			collection: "\n  {{ range(1, 3) }}  \n",
			configure: func(builder *CollectionBuilder) {
				builder.WithItemVar("user")
			},
			expectedItemVar:    "user",
			expectedCollection: "{{ range(1, 3) }}",
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := NewCollection("iterate").
				WithCollection(tc.collection).
				WithTask("process-item")
			tc.configure(builder)
			cfg, err := builder.Build(t.Context())
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, enginetask.TaskTypeCollection, cfg.Type)
			assert.Equal(t, string(core.ConfigTask), cfg.Resource)
			assert.Equal(t, tc.expectedCollection, cfg.Items)
			assert.Equal(t, tc.expectedItemVar, cfg.ItemVar)
			require.NotNil(t, cfg.Task)
			assert.Equal(t, "process-item", cfg.Task.ID)
			assert.Equal(t, string(core.ConfigTask), cfg.Task.Resource)
		})
	}
}

func TestCollectionBuilderBuildErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		builder func() *CollectionBuilder
		errText string
	}{
		{
			name: "error: missing collection source",
			builder: func() *CollectionBuilder {
				return NewCollection("collect").WithTask("process")
			},
			errText: "collection items cannot be empty",
		},
		{
			name: "error: missing task",
			builder: func() *CollectionBuilder {
				return NewCollection("collect").WithCollection("{{ .items }}")
			},
			errText: "collection task template is required",
		},
		{
			name: "error: empty task id",
			builder: func() *CollectionBuilder {
				return NewCollection("collect").
					WithCollection("{{ .items }}").
					WithTask("   ")
			},
			errText: "task id cannot be empty",
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tc.builder().Build(t.Context())
			require.Error(t, err)
			var buildErr *sdkerrors.BuildError
			require.ErrorAs(t, err, &buildErr)
			assert.ErrorContains(t, buildErr, tc.errText)
		})
	}
}

func TestCollectionBuilderBuildErrorAggregation(t *testing.T) {
	t.Parallel()

	_, err := NewCollection("   ").
		WithCollection("   ").
		WithTask("   ").
		Build(t.Context())
	require.Error(t, err)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotNil(t, buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 3)
	unwrapped := errors.Unwrap(buildErr)
	require.NotNil(t, unwrapped)
}
