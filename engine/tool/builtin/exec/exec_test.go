package exec

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testProgramSource = `package main
import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)
var sleepMs = flag.Int("sleep_ms", 0, "")
var stdoutSize = flag.Int("stdout_size", 0, "")
var stderrSize = flag.Int("stderr_size", 0, "")
var exitCode = flag.Int("exit_code", 0, "")
func main() {
	flag.Parse()
	if *sleepMs > 0 {
		time.Sleep(time.Duration(*sleepMs) * time.Millisecond)
	}
	if size := *stdoutSize; size > 0 {
		fmt.Println(strings.Repeat("A", size))
	} else {
		fmt.Println(strings.Join(flag.Args(), " "))
	}
	if size := *stderrSize; size > 0 {
		fmt.Fprintln(os.Stderr, strings.Repeat("B", size))
	}
	if val := os.Getenv("EXEC_ENV_TEST"); val != "" {
		fmt.Println(val)
	}
	os.Exit(*exitCode)
}
`

func buildTestBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "tool.go")
	require.NoError(t, os.WriteFile(sourcePath, []byte(testProgramSource), 0o644))
	binaryPath := filepath.Join(dir, "tool")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", binaryPath, sourcePath)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build test binary: %v\n%s", err, string(output))
	}
	return binaryPath
}

func newTestContext(
	t *testing.T,
	allowlist []config.NativeExecCommandConfig,
	configure func(*config.Config),
) context.Context {
	t.Helper()
	ctx := t.Context()
	ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
	manager := config.NewManager(t.Context(), config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := manager.Get()
	cfg.Runtime.NativeTools.Exec.Allowlist = allowlist
	if configure != nil {
		configure(cfg)
	}
	return config.ContextWithManager(ctx, manager)
}

func invoke(ctx context.Context, t *testing.T, payload map[string]any) (core.Output, *core.Error) {
	t.Helper()
	handler := Definition().Handler
	output, err := handler(ctx, payload)
	if err == nil {
		return output, nil
	}
	var coreErr *core.Error
	if errors.As(err, &coreErr) {
		return nil, coreErr
	}
	t.Fatalf("expected core.Error, got %v", err)
	return nil, nil
}

func TestExecHandler(t *testing.T) {
	binary := buildTestBinary(t)

	t.Run("Should execute allowlisted command", func(t *testing.T) {
		ctx := newTestContext(t, []config.NativeExecCommandConfig{{
			Path:            binary,
			Description:     "test binary",
			MaxArgs:         4,
			AllowAdditional: true,
		}}, nil)
		payload := map[string]any{
			"command": binary,
			"args":    []string{"alpha", "beta"},
		}
		output, coreErr := invoke(ctx, t, payload)
		require.Nil(t, coreErr)
		stdout := output["stdout"].(string)
		assert.Contains(t, stdout, "alpha beta")
		assert.GreaterOrEqual(t, output["duration_ms"].(int64), int64(0))
		exitCode, ok := output["exit_code"].(int)
		require.True(t, ok)
		assert.Equal(t, 0, exitCode)
		assert.Equal(t, true, output["success"])
	})

	t.Run("Should reject non allowlisted command", func(t *testing.T) {
		ctx := newTestContext(t, nil, nil)
		_, coreErr := invoke(ctx, t, map[string]any{"command": binary})
		require.NotNil(t, coreErr)
		assert.Equal(t, builtin.CodeCommandNotAllowed, coreErr.Code)
	})

	t.Run("Should enforce argument constraints", func(t *testing.T) {
		ctx := newTestContext(t, []config.NativeExecCommandConfig{{
			Path:    binary,
			MaxArgs: 2,
			Arguments: []config.NativeExecArgumentConfig{{
				Index: 0,
				Enum:  []string{"safe"},
			}},
		}}, nil)
		_, coreErr := invoke(ctx, t, map[string]any{
			"command": binary,
			"args":    []string{"unsafe"},
		})
		require.NotNil(t, coreErr)
		assert.Equal(t, builtin.CodeInvalidArgument, coreErr.Code)
	})

	t.Run("Should honor timeout override", func(t *testing.T) {
		ctx := newTestContext(t, []config.NativeExecCommandConfig{{
			Path: binary,
		}}, nil)
		output, coreErr := invoke(ctx, t, map[string]any{
			"command":    binary,
			"args":       []string{"-sleep_ms", "200"},
			"timeout_ms": 50,
		})
		require.Nil(t, coreErr)
		assert.False(t, output["success"].(bool))
		assert.True(t, output["timed_out"].(bool))
	})

	t.Run("Should truncate stderr", func(t *testing.T) {
		ctx := newTestContext(t, []config.NativeExecCommandConfig{{
			Path: binary,
		}}, func(cfg *config.Config) {
			cfg.Runtime.NativeTools.Exec.MaxStderrBytes = 8
		})
		payload := map[string]any{
			"command": binary,
			"args":    []string{"--stderr_size", "100"},
		}
		output, coreErr := invoke(ctx, t, payload)
		require.Nil(t, coreErr)
		assert.True(t, output["stderr_truncated"].(bool))
	})

	t.Run("Should merge environment variables", func(t *testing.T) {
		ctx := newTestContext(t, []config.NativeExecCommandConfig{{
			Path: binary,
		}}, nil)
		payload := map[string]any{
			"command": binary,
			"env": map[string]string{
				"EXEC_ENV_TEST": "EXECVALUE",
			},
		}
		output, coreErr := invoke(ctx, t, payload)
		require.Nil(t, coreErr)
		stdout := output["stdout"].(string)
		assert.Contains(t, stdout, "EXECVALUE")
	})
}
