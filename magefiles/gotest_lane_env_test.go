//go:build mage

// Suite: Go test lane environment overrides
// Invariant: An explicit valid package cap wins and malformed caps fall back safely.
// Boundary IN: Process-environment parsing for the Mage unit-test lane.
// Boundary OUT: Pure capacity and shard selection in gotest_lane_test.go.

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// not parallel: t.Setenv mutates the process environment for each subtest.
func TestGoUnitTestPackageLimit(t *testing.T) {
	defaultLimit := goUnitTestPackageLimitFor(runtime.GOMAXPROCS(0), goUnitTestParallelism)
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "Should default to the combined lane capacity when unset",
			value: "",
			want:  strconv.Itoa(defaultLimit),
		},
		{name: "Should honor a valid override", value: "2", want: "2"},
		{name: "Should ignore a non-numeric override", value: "many", want: strconv.Itoa(defaultLimit)},
		{name: "Should ignore a non-positive override", value: "0", want: strconv.Itoa(defaultLimit)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(goTestPackageLimitEnvVar, tc.value)
			if got := goUnitTestPackageLimit(); got != tc.want {
				t.Fatalf("goUnitTestPackageLimit() with %q = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

// Invariant: CI retries only failed tests once and caps the rerun set at five;
// local invocations remain single-attempt. Owner: Mage test lane policy.
func TestGoTestRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, ci string
		want     []string
	}{
		{"Should leave local execution single-attempt", "", []string{"--format", "pkgname"}},
		{"Should expose one bounded test retry in CI", "true", []string{"--format", "pkgname", "--rerun-fails=1", "--rerun-fails-max-failures=5"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CI", tc.ci)
			if got := gotestsumRunnerArgs(); !slices.Equal(got, tc.want) {
				t.Fatalf("args=%v want=%v", got, tc.want)
			}
			// Package selection belongs to gotestsum so retries can select a
			// failed test without rerunning the original package list.
			args := gotestsumJSONArgs("receipt.json", "example.com/a", "example.com/b")
			want := append(slices.Clone(tc.want), "--jsonfile", "receipt.json",
				"--packages", "example.com/a example.com/b", "--")
			if !slices.Equal(args, want) {
				t.Fatalf("runner package selection=%v want=%v", args, want)
			}
		})
	}
}

// not parallel: report provenance is read from process environment via t.Setenv.

func TestFlakeReport(t *testing.T) {
	t.Run("Should retain recoveries and provenance even when another retry skips", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("COMPOZY_FLAKE_JOB", "integration-other-0")
		t.Setenv("GITHUB_RUN_ID", "42")
		t.Setenv("GITHUB_SHA", "merge-sha")
		t.Setenv("COMPOZY_FLAKE_SHA", "abc")
		t.Setenv("GITHUB_RUN_ATTEMPT", "1")
		t.Setenv("GITHUB_OUTPUT", filepath.Join(root, "output"))
		if err := os.WriteFile(filepath.Join(root, "output"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		writeTestFile(
			t,
			root,
			"gotest-shard0.json",
			`{"Package":"p","Test":"TestRecovered","Action":"fail"}`+"\n"+`{"Package":"p","Test":"TestRecovered","Action":"pass"}`+"\n",
		)
		target := filepath.Join(root, "receipt", "flakes.json")
		writeTestFile(t, root, "gotest-skipped.json",
			`{"Package":"p","Test":"TestSkipped","Action":"fail"}`+"\n"+
				`{"Package":"p","Test":"TestSkipped","Action":"skip"}`+"\n")
		if err := TestFlakes(root, target); err == nil || !strings.Contains(err.Error(), "retry must pass") {
			t.Fatalf("skipped retry should reject the report: %v", err)
		}
		receipts, err := loadFlakeReceipts(filepath.Dir(target))
		if err != nil {
			t.Fatal(err)
		}
		want := flakeReceipt{
			Suite:      "go",
			Test:       "p#TestRecovered",
			Job:        "integration-other-0",
			Run:        "42",
			SHA:        "abc",
			Attempt:    2,
			RunAttempt: "1",
		}
		if len(receipts) != 1 || receipts[0] != want {
			t.Fatalf("receipts=%+v", receipts)
		}
		output, err := os.ReadFile(filepath.Join(root, "output"))
		if err != nil {
			t.Fatal(err)
		}
		if string(output) != "count=1\n" {
			t.Fatalf("output=%s", output)
		}
	})
	t.Run(
		"Should reject a dynamically skipped Playwright retry while retaining recovered receipts",
		func(t *testing.T) {
			t.Setenv("GITHUB_OUTPUT", "")
			root := t.TempDir()
			writeTestFile(t, root, "playwright.json", `{"suites":[{"specs":[
{"file":"os.spec.ts","title":"skipped retry","tests":[{"expectedStatus":"skipped","results":[{"status":"failed","retry":0},{"status":"skipped","retry":1}]}]},
{"file":"os.spec.ts","title":"recovered","tests":[{"expectedStatus":"passed","results":[{"status":"failed","retry":0},{"status":"passed","retry":1}]}]},
{"file":"os.spec.ts","title":"expected failure","tests":[{"expectedStatus":"failed","results":[{"status":"failed","retry":0}]}]},
{"file":"os.spec.ts","title":"ordinary skip","tests":[{"expectedStatus":"skipped","results":[{"status":"skipped","retry":0}]}]}
]}]}`)
			target := filepath.Join(root, "output", "flakes.json")
			if err := TestFlakes(root, target); err == nil ||
				!strings.Contains(
					err.Error(),
					"os.spec.ts#skipped retry [] ended with skipped after failure; retry must pass",
				) {
				t.Fatalf("expected skipped retry rejection, got %v", err)
			}
			receipts, err := loadFlakeReceipts(filepath.Dir(target))
			if err != nil {
				t.Fatal(err)
			}
			if len(receipts) != 1 || receipts[0].Test != "os.spec.ts#recovered []" || receipts[0].Attempt != 2 {
				t.Fatalf("genuine recovery lost or skipped test misclassified: %+v", receipts)
			}
		},
	)
}

// Invariant: receipt paths remain rooted in the checkout even when the runner
// changes into sdk/go. Owner: Mage lane environment; canonical suite: this file.
func TestGoTestReceiptDirectory(t *testing.T) {
	t.Run("Should resolve a relative receipt directory before running another module", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv(goTestJSONFileDirEnvVar, "receipts")
		dir, err := ensureGoTestJSONFileDir()
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(dir) {
			t.Fatalf("receipt directory must be absolute: %q", dir)
		}
		if _, err := os.Stat(dir); err != nil {
			t.Fatal(err)
		}
	})
}
