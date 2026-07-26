package update

import "testing"

func TestCompareVersionsTreatsGitDescribeBuildAsBaseVersion(t *testing.T) {
	t.Run("Should compare git-describe builds using the base tagged version", func(t *testing.T) {
		t.Parallel()

		got, err := compareVersions("v1.2.3-4-gabcdef1", "v1.2.4")
		if err != nil {
			t.Fatalf("compareVersions() error = %v", err)
		}
		if got >= 0 {
			t.Fatalf("compareVersions() = %d, want negative value", got)
		}
	})
}

func TestTrimGitDescribeSuffixLeavesTaggedVersionUntouched(t *testing.T) {
	t.Run("Should leave tagged versions untouched when no git-describe suffix exists", func(t *testing.T) {
		t.Parallel()

		if got := trimGitDescribeSuffix("v1.2.3"); got != "v1.2.3" {
			t.Fatalf("trimGitDescribeSuffix() = %q, want %q", got, "v1.2.3")
		}
	})
}

func TestIsDevVersionClassifiesUntaggedBuilds(t *testing.T) {
	t.Run("Should treat bare git hashes as development builds", func(t *testing.T) {
		t.Parallel()

		for _, version := range []string{"dev", "25bd6116", "25bd6116-dirty"} {
			if !isDevVersion(version) {
				t.Fatalf("isDevVersion(%q) = false, want true", version)
			}
		}
	})

	t.Run("Should keep tagged git-describe builds comparable", func(t *testing.T) {
		t.Parallel()

		for _, version := range []string{"v1.2.3", "v1.2.3-4-gabcdef1"} {
			if isDevVersion(version) {
				t.Fatalf("isDevVersion(%q) = true, want false", version)
			}
		}
	})
}
