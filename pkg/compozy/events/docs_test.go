package events

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEventsDocumentationEnumeratesAllPublicKinds(t *testing.T) {
	t.Parallel()

	content := readEventsDocumentation(t)
	for _, kind := range []EventKind{
		EventKindRunQueued,
		EventKindRunStarted,
		EventKindRunCrashed,
		EventKindRunCompleted,
		EventKindRunFailed,
		EventKindRunCancelled,
		EventKindJobQueued,
		EventKindJobStarted,
		EventKindJobAttemptStarted,
		EventKindJobAttemptFinished,
		EventKindJobRetryScheduled,
		EventKindJobCompleted,
		EventKindJobFailed,
		EventKindJobCancelled,
		EventKindSessionStarted,
		EventKindSessionUpdate,
		EventKindSessionCompleted,
		EventKindSessionFailed,
		EventKindReusableAgentLifecycle,
		EventKindToolCallStarted,
		EventKindToolCallUpdated,
		EventKindToolCallFailed,
		EventKindUsageUpdated,
		EventKindUsageAggregated,
		EventKindTaskFileUpdated,
		EventKindTaskMetadataRefreshed,
		EventKindTaskMemoryUpdated,
		EventKindArtifactUpdated,
		EventKindExtensionLoaded,
		EventKindExtensionReady,
		EventKindExtensionFailed,
		EventKindExtensionEvent,
		EventKindReviewStatusFinalized,
		EventKindReviewRoundRefreshed,
		EventKindReviewIssueResolved,
		EventKindProviderCallStarted,
		EventKindProviderCallCompleted,
		EventKindProviderCallFailed,
		EventKindShutdownRequested,
		EventKindShutdownDraining,
		EventKindShutdownTerminated,
	} {
		want := "`" + string(kind) + "`"
		if !strings.Contains(content, want) {
			t.Fatalf("expected docs/events.md to mention %s", kind)
		}
	}
}

func readEventsDocumentation(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve docs test file path")
	}
	docsPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "docs", "events.md")
	content, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read %s: %v", docsPath, err)
	}
	return string(content)
}
