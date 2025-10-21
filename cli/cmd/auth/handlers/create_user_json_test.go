package handlers_test

import (
	"io"
	"os"
	"testing"

	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/cmd/auth/handlers"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = original
	})
	t.Cleanup(func() {
		_ = r.Close()
	})
	run()
	require.NoError(t, w.Close())
	output, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(output)
}

func TestCreateUserJSONReturnsHandledError(t *testing.T) {
	t.Parallel()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cobraCmd := &cobra.Command{}
	cobraCmd.Flags().String("email", "", "")
	cobraCmd.Flags().String("name", "", "")
	cobraCmd.Flags().String("role", "", "")
	cobraCmd.SetContext(ctx)
	require.NoError(t, cobraCmd.Flags().Set("email", "user@example.com"))
	require.NoError(t, cobraCmd.Flags().Set("name", "Example User"))
	require.NoError(t, cobraCmd.Flags().Set("role", "invalid"))

	executor := &cmd.CommandExecutor{}
	var observedErr error
	output := captureStdout(t, func() {
		observedErr = handlers.CreateUserJSON(ctx, cobraCmd, executor, nil)
	})
	require.NotEmpty(t, output)
	require.Error(t, observedErr)
	require.True(t, helpers.IsJSONHandledError(observedErr))

	propagatedErr := cmd.HandleCommonErrors(observedErr, models.ModeJSON)
	require.Error(t, propagatedErr)
	require.True(t, helpers.IsJSONHandledError(propagatedErr))
	require.Same(t, observedErr, propagatedErr)
}
