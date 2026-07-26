//go:build mage

// Suite: Hermetic Go test lane policy
// Invariant: Unit-test shards partition every package once and reject ambiguous configuration.
// Boundary IN: Mage package selection, concurrency policy, and environment scrubbing.
// Boundary OUT: GitHub Actions runner orchestration in .github/workflows/ci.yml.

package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestGoUnitTestPackageLimitFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		effectiveCPU int
		parallelism  int
		want         int
	}{
		{name: "Should keep one group on four CPUs", effectiveCPU: 4, parallelism: 4, want: 1},
		{name: "Should keep one group on eight CPUs", effectiveCPU: 8, parallelism: 4, want: 1},
		{name: "Should use the partial second group on twelve CPUs", effectiveCPU: 12, parallelism: 4, want: 2},
		{name: "Should allow two groups on sixteen CPUs", effectiveCPU: 16, parallelism: 4, want: 2},
		{name: "Should allow four groups on thirty two CPUs", effectiveCPU: 32, parallelism: 4, want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := goUnitTestPackageLimitFor(tt.effectiveCPU, tt.parallelism); got != tt.want {
				t.Fatalf(
					"goUnitTestPackageLimitFor(%d, %d) = %d, want %d",
					tt.effectiveCPU,
					tt.parallelism,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestParseGoTestShard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		indexRaw  string
		totalRaw  string
		want      goTestShard
		wantOn    bool
		wantError string
	}{
		{name: "Should disable sharding when both values are unset"},
		{
			name:     "Should parse a valid zero-based shard",
			indexRaw: "1",
			totalRaw: "2",
			want:     goTestShard{index: 1, total: 2},
			wantOn:   true,
		},
		{
			name:      "Should reject a missing shard total",
			indexRaw:  "0",
			wantError: "must be set together",
		},
		{
			name:      "Should reject a negative shard index",
			indexRaw:  "-1",
			totalRaw:  "2",
			wantError: "non-negative integer",
		},
		{
			name:      "Should reject a shard index outside the total",
			indexRaw:  "2",
			totalRaw:  "2",
			wantError: "must be less than",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, enabled, err := parseGoTestShard(tt.indexRaw, tt.totalRaw)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("parseGoTestShard(%q, %q) error = %v, want containing %q", tt.indexRaw, tt.totalRaw, err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGoTestShard(%q, %q) error = %v", tt.indexRaw, tt.totalRaw, err)
			}
			if enabled != tt.wantOn || got != tt.want {
				t.Fatalf(
					"parseGoTestShard(%q, %q) = (%+v, %t), want (%+v, %t)",
					tt.indexRaw,
					tt.totalRaw,
					got,
					enabled,
					tt.want,
					tt.wantOn,
				)
			}
		})
	}
}

func TestShardGoTestPackages(t *testing.T) {
	t.Parallel()

	t.Run("Should partition the sorted package list exactly once", func(t *testing.T) {
		t.Parallel()
		packages := []string{"package/d", "package/a", "package/e", "package/b", "package/c"}
		first := shardGoTestPackages(packages, goTestShard{index: 0, total: 2})
		second := shardGoTestPackages(packages, goTestShard{index: 1, total: 2})

		if !slices.Equal(first, []string{"package/a", "package/c", "package/e"}) {
			t.Fatalf("first shard = %v, want [package/a package/c package/e]", first)
		}
		if !slices.Equal(second, []string{"package/b", "package/d"}) {
			t.Fatalf("second shard = %v, want [package/b package/d]", second)
		}
		counts := make(map[string]int, len(packages))
		for _, packagePath := range append(first, second...) {
			counts[packagePath]++
		}
		for _, packagePath := range packages {
			if counts[packagePath] != 1 {
				t.Fatalf(
					"package %q appears %d times across shards, want exactly once",
					packagePath,
					counts[packagePath],
				)
			}
		}
	})
}

func TestHermeticGoTestEnvFromBase(t *testing.T) {
	t.Parallel()

	t.Run("Should scrub ambient runtime-state vars and keep everything else", func(t *testing.T) {
		t.Parallel()
		base := []string{
			"AGH_HOME=/tmp/stale-qa-lab/runtime",
			"AGH_HTTP_PORT=54321",
			"AGH_TEST_DAEMON_BIN=/tmp/daemon",
			"AGH_LIVE_DISCOVERY_HELPER=/tmp/helper",
			"PROVIDER_HOME=/tmp/stale-provider",
			"PATH=/usr/bin",
		}
		var log bytes.Buffer
		got := hermeticGoTestEnvFromBase(base, nil, &log)
		for _, banned := range []string{"AGH_HOME=/tmp/stale-qa-lab/runtime", "AGH_HTTP_PORT=54321", "PROVIDER_HOME=/tmp/stale-provider"} {
			if slices.Contains(got, banned) {
				t.Fatalf("hermeticGoTestEnvFromBase() kept ambient %q; env = %v", banned, got)
			}
		}
		for _, kept := range []string{"AGH_TEST_DAEMON_BIN=/tmp/daemon", "AGH_LIVE_DISCOVERY_HELPER=/tmp/helper", "PATH=/usr/bin"} {
			if !slices.Contains(got, kept) {
				t.Fatalf("hermeticGoTestEnvFromBase() dropped %q; env = %v", kept, got)
			}
		}
		if !bytes.Contains(log.Bytes(), []byte("AGH_HOME")) {
			t.Fatalf("scrub note missing dropped var name; log = %q", log.String())
		}
	})

	t.Run("Should let explicit lane overrides win over the scrub", func(t *testing.T) {
		t.Parallel()
		base := []string{"AGH_WEB_DIST_DIR=/tmp/stale-dist"}
		got := hermeticGoTestEnvFromBase(
			base,
			map[string]string{"AGH_WEB_DIST_DIR": "/lane/dist", "CGO_ENABLED": "1"},
			nil,
		)
		if !slices.Contains(got, "AGH_WEB_DIST_DIR=/lane/dist") {
			t.Fatalf("lane override missing; env = %v", got)
		}
		if !slices.Contains(got, "CGO_ENABLED=1") {
			t.Fatalf("added override missing; env = %v", got)
		}
		if slices.Contains(got, "AGH_WEB_DIST_DIR=/tmp/stale-dist") {
			t.Fatalf("stale ambient value survived; env = %v", got)
		}
	})
}

func TestMergeEnvOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Should replace existing entries and append new ones", func(t *testing.T) {
		t.Parallel()
		got := mergeEnvOverrides([]string{"A=1", "B=2"}, map[string]string{"B": "3", "C": "4"})
		for _, want := range []string{"A=1", "B=3", "C=4"} {
			if !slices.Contains(got, want) {
				t.Fatalf("mergeEnvOverrides() = %v, want to contain %q", got, want)
			}
		}
		if slices.Contains(got, "B=2") {
			t.Fatalf("mergeEnvOverrides() kept stale entry; got %v", got)
		}
	})

	t.Run("Should return a copy when no overrides are given", func(t *testing.T) {
		t.Parallel()
		base := []string{"A=1"}
		got := mergeEnvOverrides(base, nil)
		if len(got) != 1 || got[0] != "A=1" {
			t.Fatalf("mergeEnvOverrides() = %v, want [A=1]", got)
		}
	})
}
