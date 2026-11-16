package hooks

import (
	"bytes"
	"context"

	"errors"
	"fmt"
	"maps"
	"os"

	"slices"

	"time"

	"github.com/compozy/compozy/internal/procutil"
)

func subprocessProcessEnv(env map[string]string) []string {
	return subprocessProcessEnvWith(env, os.LookupEnv)
}

// subprocessProcessEnvWith builds the hook environment from lookup, so tests need no process environment.
func subprocessProcessEnvWith(env map[string]string, lookup func(string) (string, bool)) []string {
	inherited := append(slices.Clone(subprocessEnvAllowlist), procutil.CATrustEnvKeys()...)
	merged := make(map[string]string, len(inherited)+len(env))
	for _, key := range inherited {
		if value, ok := lookup(key); ok {
			merged[key] = value
		}
	}
	maps.Copy(merged, env)

	keys := slices.Sorted(maps.Keys(merged))

	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, key+"="+merged[key])
	}

	return values
}

func subprocessRunError(ctx context.Context, timeout time.Duration, err error, stderr *limitedSubprocessCapture) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("hook timed out after %s: %w", timeout, ctx.Err())
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return fmt.Errorf("hook canceled: %w", ctx.Err())
	}
	if stderr == nil || stderr.Len() == 0 {
		return fmt.Errorf("hook command failed: %w", err)
	}

	return fmt.Errorf("hook command failed: %w (%s)", err, subprocessCaptureSummary(stderr))
}

type limitedSubprocessCapture struct {
	buf       bytes.Buffer
	truncated bool
}

func newLimitedSubprocessCapture() *limitedSubprocessCapture {
	return &limitedSubprocessCapture{}
}

func (c *limitedSubprocessCapture) Write(payload []byte) (int, error) {
	if c == nil {
		return len(payload), nil
	}

	remaining := subprocessCaptureLimitBytes - c.buf.Len()
	switch {
	case remaining <= 0:
		c.truncated = true
	case len(payload) > remaining:
		if _, err := c.buf.Write(payload[:remaining]); err != nil {
			return 0, err
		}
		c.truncated = true
	default:
		if _, err := c.buf.Write(payload); err != nil {
			return 0, err
		}
	}

	return len(payload), nil
}

func (c *limitedSubprocessCapture) String() string {
	if c == nil {
		return ""
	}

	value := c.buf.String()
	if !c.truncated {
		return value
	}

	return value + subprocessCaptureTruncate
}

func (c *limitedSubprocessCapture) Len() int {
	if c == nil {
		return 0
	}

	return c.buf.Len()
}

func (c *limitedSubprocessCapture) Truncated() bool {
	return c != nil && c.truncated
}

func subprocessCaptureSummary(capture *limitedSubprocessCapture) string {
	if capture == nil || capture.Len() == 0 {
		return "redacted output (0 bytes)"
	}
	if capture.Truncated() {
		return fmt.Sprintf("redacted output (%d+ bytes, truncated)", capture.Len())
	}

	return fmt.Sprintf("redacted output (%d bytes)", capture.Len())
}
