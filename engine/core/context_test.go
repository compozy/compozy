package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ProjectNameContext(t *testing.T) {
	t.Run("Should set and get project name from context", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithProjectName(ctx, "compozy")
		name, err := GetProjectName(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "compozy", name)
	})
	t.Run("Should error when project name not present", func(t *testing.T) {
		_, err := GetProjectName(context.Background())
		assert.ErrorContains(t, err, "project name not found")
	})
}
