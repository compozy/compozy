//go:build integration

package pluginsource

import (
	"errors"
	"os"
	"testing"
)

// Public acquisition is opt-in through the dedicated CI workflow, never the unit lane.
func TestPublicGitHubMarketplace(t *testing.T) {
	t.Parallel()
	if os.Getenv("COMPOZY_TEST_PUBLIC_MARKETPLACE") != "1" {
		t.Skip("public acquisition runs only in its opt-in CI lane")
	}
	t.Run("Should acquire the official public marketplace and its pinned snapshot", func(t *testing.T) {
		t.Parallel()
		source, err := NewGitHubSource("github:anthropics/claude-plugins-official")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := source.Close(); err != nil {
				t.Error(err)
			}
		})
		document, err := source.Fetch(t.Context())
		if err != nil {
			if failure, ok := errors.AsType[*SourceError](err); ok {
				t.Fatalf("public document acquisition: %v; cause: %v", err, failure.Cause)
			}
			t.Fatal(err)
		}
		if len(document.Plugins) == 0 {
			t.Fatal("public marketplace returned no plugins")
		}
		t.Logf("Fetched %d plugins from %s", len(document.Plugins), document.ResolvedRef)
		snapshot, err := source.OpenSnapshot(t.Context(), document, t.TempDir())
		if err != nil {
			if failure, ok := errors.AsType[*SourceError](err); ok {
				t.Fatalf("public snapshot acquisition: %v; cause: %v", err, failure.Cause)
			}
			t.Fatal(err)
		}
		if err := snapshot.Close(); err != nil {
			t.Fatal(err)
		}
	})
}
