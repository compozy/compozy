package main

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/update"
)

func TestWaitForUpdateResult(t *testing.T) {
	t.Parallel()

	t.Run("Should return a ready release", func(t *testing.T) {
		t.Parallel()

		result := make(chan *update.ReleaseInfo, 1)
		want := &update.ReleaseInfo{Version: "v1.2.3"}
		result <- want
		close(result)

		if got := waitForUpdateResult(result); got != want {
			t.Fatalf("waitForUpdateResult() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should return nil for a closed channel", func(t *testing.T) {
		t.Parallel()

		result := make(chan *update.ReleaseInfo)
		close(result)

		if got := waitForUpdateResult(result); got != nil {
			t.Fatalf("waitForUpdateResult() = %#v, want nil", got)
		}
	})

	t.Run("Should return nil when the update check does not finish quickly", func(t *testing.T) {
		t.Parallel()

		result := make(chan *update.ReleaseInfo)
		done := make(chan *update.ReleaseInfo, 1)

		go func() {
			done <- waitForUpdateResult(result)
		}()

		select {
		case got := <-done:
			if got != nil {
				t.Fatalf("waitForUpdateResult() = %#v, want nil", got)
			}
		case <-time.After(2 * updateResultWaitTimeout):
			t.Fatalf("waitForUpdateResult did not return within %s", 2*updateResultWaitTimeout)
		}
	})
}
