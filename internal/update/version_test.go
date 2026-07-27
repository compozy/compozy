package update

import "testing"

func TestReleaseTrackForVersion(t *testing.T) {
	t.Run("Should classify stable and beta versions", func(t *testing.T) {
		t.Parallel()

		for raw, want := range map[string]releaseTrack{
			"v0.3.0":        releaseTrackStable,
			"v0.3.0-beta.1": releaseTrackBeta,
		} {
			got, _, err := releaseTrackForVersion(raw)
			if err != nil {
				t.Fatalf("releaseTrackForVersion(%q) error = %v", raw, err)
			}
			if got != want {
				t.Fatalf("releaseTrackForVersion(%q) = %q, want %q", raw, got, want)
			}
		}
	})

	t.Run("Should reject unsupported prerelease tracks", func(t *testing.T) {
		t.Parallel()

		if _, _, err := releaseTrackForVersion("v0.3.0-rc.1"); err == nil {
			t.Fatal("releaseTrackForVersion(rc) error = nil, want unsupported-track error")
		}
	})

	t.Run("Should accept only beta releases on the current major minor line", func(t *testing.T) {
		t.Parallel()

		_, current, err := releaseTrackForVersion("v0.3.0-beta.1")
		if err != nil {
			t.Fatalf("releaseTrackForVersion(current) error = %v", err)
		}
		for candidate, want := range map[string]bool{
			"v0.3.0-beta.2": true,
			"v0.3.1-beta.1": true,
			"v0.4.0-beta.1": false,
			"v0.3.0-rc.1":   false,
			"v0.3.0":        false,
		} {
			parsed, parseErr := parseVersion(candidate)
			if parseErr != nil {
				t.Fatalf("parseVersion(%q) error = %v", candidate, parseErr)
			}
			if got := isBetaVersionForLine(parsed, current); got != want {
				t.Fatalf("isBetaVersionForLine(%q) = %t, want %t", candidate, got, want)
			}
		}
	})
}

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

	t.Run("Should treat dirty git-describe builds as development builds", func(t *testing.T) {
		t.Parallel()

		if !isDevVersion("v0.3.0-16-gb2ad2446-dirty") {
			t.Fatal("isDevVersion(dirty git-describe) = false, want true")
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
