package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestMCPServeCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should require an explicit workspace", func(t *testing.T) {
		t.Parallel()

		cmd := newMCPServeCommand(commandDeps{runMCPServe: func(context.Context, mcpServeOptions) error {
			t.Fatal("runMCPServe called without required workspace")
			return nil
		}})
		cmd.SetArgs(nil)
		if err := cmd.ExecuteContext(t.Context()); err == nil || !strings.Contains(err.Error(), "workspace") {
			t.Fatalf("ExecuteContext() error = %v, want required workspace", err)
		}
	})

	t.Run("Should forward transport options and command streams", func(t *testing.T) {
		t.Parallel()

		var captured mcpServeOptions
		deps := commandDeps{runMCPServe: func(_ context.Context, opts mcpServeOptions) error {
			captured = opts
			return nil
		}}
		cmd := newMCPServeCommand(deps)
		stdin := strings.NewReader("input")
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetIn(stdin)
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{
			"--workspace", "alpha",
			"--transport", "http",
			"--listen", "127.0.0.1:3131",
			"--token-env", "CUSTOM_TOKEN",
		})
		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("ExecuteContext() error = %v", err)
		}
		if captured.Workspace != "alpha" || captured.Transport != "http" ||
			captured.Listen != "127.0.0.1:3131" || captured.TokenEnv != "CUSTOM_TOKEN" {
			t.Fatalf("captured options = %#v, want command flags", captured)
		}
		if captured.Stdin != stdin || captured.Stdout != stdout || captured.Stderr != stderr {
			t.Fatalf("captured streams = %#v, want command streams", captured)
		}
		if cmd.Flags().Lookup("token") != nil {
			t.Fatal("serve command exposes a token value flag")
		}
	})

	t.Run("Should validate HTTP listener before daemon discovery", func(t *testing.T) {
		t.Parallel()

		deps := commandDeps{}.withDefaults()
		err := runMCPServe(t.Context(), deps, mcpServeOptions{
			Workspace: "alpha",
			Transport: mcpServeTransportHTTP,
		})
		if err == nil || !strings.Contains(err.Error(), "--listen is required") {
			t.Fatalf("runMCPServe() error = %v, want required listener", err)
		}
	})

	t.Run("Should reject listen on stdio before daemon discovery", func(t *testing.T) {
		t.Parallel()

		deps := commandDeps{}.withDefaults()
		err := runMCPServe(t.Context(), deps, mcpServeOptions{
			Workspace: "alpha",
			Transport: mcpServeTransportStdio,
			Listen:    "127.0.0.1:3131",
		})
		if err == nil || !strings.Contains(err.Error(), "only valid") {
			t.Fatalf("runMCPServe() error = %v, want stdio listener rejection", err)
		}
	})
}
