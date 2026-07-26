package session

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
)

func TestCrashBundleFileName(t *testing.T) {
	t.Parallel()

	t.Run("ShouldPreserveTimestampWhenSessionIDIsTruncated", func(t *testing.T) {
		t.Parallel()

		first := time.Unix(0, 101).UTC()
		second := time.Unix(0, 202).UTC()
		sessionID := strings.Repeat("session-", 40)

		firstName := crashBundleFileName(sessionID, store.FailureProcess, first)
		secondName := crashBundleFileName(sessionID, store.FailureProcess, second)
		firstToken := strconv.FormatInt(first.UnixNano(), 10)
		secondToken := strconv.FormatInt(second.UnixNano(), 10)

		if firstName == secondName {
			t.Fatalf("crashBundleFileName() reused %q for distinct timestamps", firstName)
		}
		if !strings.Contains(firstName, firstToken) || !strings.Contains(secondName, secondToken) {
			t.Fatalf("crash bundle names = %q, %q; want timestamp suffixes preserved", firstName, secondName)
		}
		if len(strings.TrimSuffix(firstName, ".json")) > crashBundleNameMaxBytes ||
			len(strings.TrimSuffix(secondName, ".json")) > crashBundleNameMaxBytes {
			t.Fatalf("crash bundle names exceed max base bytes: %q, %q", firstName, secondName)
		}
	})
}
