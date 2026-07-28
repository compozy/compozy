//go:build !windows

package procutil

import (
	"strings"
	"testing"
)

func TestStartedAtEnv(t *testing.T) {
	t.Run("ShouldForceTheCLocaleForPSOutput", func(t *testing.T) {
		t.Setenv("LC_ALL", "pt_BR.UTF-8")

		var localeEntries []string
		for _, entry := range startedAtEnv() {
			if strings.HasPrefix(entry, "LC_ALL=") {
				localeEntries = append(localeEntries, entry)
			}
		}
		if len(localeEntries) != 1 {
			t.Fatalf("startedAtEnv() LC_ALL entries = %#v, want exactly one", localeEntries)
		}
		if got, want := localeEntries[0], "LC_ALL=C"; got != want {
			t.Fatalf("startedAtEnv() LC_ALL = %q, want %q", got, want)
		}
	})
}
