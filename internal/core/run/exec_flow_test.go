package run

import (
	"testing"
	"time"
)

func TestFormatExecRunIDIncludesNanoseconds(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 5, 12, 34, 56, 123456789, time.UTC)

	if got := formatExecRunID(now); got != "exec-20260405-123456-123456789" {
		t.Fatalf("unexpected exec run id: %q", got)
	}
}
