//go:build mage

// Suite: Go test shard census
// Invariant: Census weights load deterministically with positive defaults, the LPT partition
// covers every item exactly once with input-determined assignment, and census updates fold
// gotestsum events into stable rounded weights while removing deleted source entries.
// Boundary IN: Census JSON bytes, shard item lists, and gotestsum test2json events.
// Boundary OUT: Shard invocation building in gotest_lane.go.

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadGoTestCensus(t *testing.T) {
	t.Parallel()

	writeCensus := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "census.json")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write census fixture: %v", err)
		}
		return path
	}

	t.Run("Should return no census for a missing file", func(t *testing.T) {
		t.Parallel()
		census, err := loadGoTestCensus(filepath.Join(t.TempDir(), "absent.json"))
		if err != nil {
			t.Fatalf("loadGoTestCensus() error = %v", err)
		}
		if census != nil {
			t.Fatalf("loadGoTestCensus() = %+v, want nil for a missing file", census)
		}
	})

	t.Run("Should reject invalid JSON", func(t *testing.T) {
		t.Parallel()
		_, err := loadGoTestCensus(writeCensus(t, "{not json"))
		if err == nil || !strings.Contains(err.Error(), "decode go test census") {
			t.Fatalf("loadGoTestCensus() error = %v, want decode failure", err)
		}
	})

	t.Run("Should reject non-positive defaults", func(t *testing.T) {
		t.Parallel()
		_, err := loadGoTestCensus(writeCensus(
			t,
			`{"defaultPackageSeconds": 0, "defaultSplitTestSeconds": 2, "packages": {}, "splitTests": {}}`,
		))
		if err == nil || !strings.Contains(err.Error(), "must be positive") {
			t.Fatalf("loadGoTestCensus() error = %v, want positive-defaults failure", err)
		}
	})

	t.Run("Should resolve known weights and fall back to defaults", func(t *testing.T) {
		t.Parallel()
		census, err := loadGoTestCensus(writeCensus(t, `{
			"defaultPackageSeconds": 2.5,
			"defaultSplitTestSeconds": 7.5,
			"packages": {"example.com/pkg": 42.5},
			"splitTests": {"TestKnown": 12.25}
		}`))
		if err != nil {
			t.Fatalf("loadGoTestCensus() error = %v", err)
		}
		if got := census.packageWeight("example.com/pkg"); got != 42.5 {
			t.Fatalf("packageWeight(known) = %v, want 42.5", got)
		}
		if got := census.packageWeight("example.com/unknown"); got != 2.5 {
			t.Fatalf("packageWeight(unknown) = %v, want default 2.5", got)
		}
		if got := census.splitTestWeight("TestKnown"); got != 12.25 {
			t.Fatalf("splitTestWeight(known) = %v, want 12.25", got)
		}
		if got := census.splitTestWeight("TestUnknown"); got != 7.5 {
			t.Fatalf("splitTestWeight(unknown) = %v, want default 7.5", got)
		}
	})

	t.Run("Should keep uniform weights without a census", func(t *testing.T) {
		t.Parallel()
		var census *goTestCensus
		if got := census.packageWeight("example.com/pkg"); got != 1 {
			t.Fatalf("nil census packageWeight() = %v, want 1", got)
		}
		if got := census.splitTestWeight("TestAny"); got != 1 {
			t.Fatalf("nil census splitTestWeight() = %v, want 1", got)
		}
	})
}

func TestMissingPackageCount(t *testing.T) {
	t.Parallel()

	t.Run("Should count only packages without a census entry", func(t *testing.T) {
		t.Parallel()
		census := &goTestCensus{
			DefaultPackageSeconds:   1,
			DefaultSplitTestSeconds: 1,
			Packages:                map[string]float64{"example.com/known": 3},
		}
		got := census.missingPackageCount([]string{"example.com/known", "example.com/new", goSplitTestPackage})
		if got != 1 {
			t.Fatalf("missingPackageCount() = %d, want 1 (split package and known entries excluded)", got)
		}
	})

	t.Run("Should report nothing without a census", func(t *testing.T) {
		t.Parallel()
		var census *goTestCensus
		if got := census.missingPackageCount([]string{"example.com/any"}); got != 0 {
			t.Fatalf("nil census missingPackageCount() = %d, want 0", got)
		}
	})
}

func TestPartitionGoShardItems(t *testing.T) {
	t.Parallel()

	t.Run("Should place every item in exactly one bin", func(t *testing.T) {
		t.Parallel()
		items := []goShardItem{
			{name: "a", weight: 3},
			{name: "b", weight: 1},
			{name: "c", weight: 4},
			{name: "d", weight: 1},
			{name: "e", weight: 5},
		}
		bins := partitionGoShardItems(items, 3)
		if len(bins) != 3 {
			t.Fatalf("partitionGoShardItems() bins = %d, want 3", len(bins))
		}
		counts := make(map[string]int, len(items))
		for _, bin := range bins {
			for _, item := range bin {
				counts[item.name]++
			}
		}
		for _, item := range items {
			if counts[item.name] != 1 {
				t.Fatalf("item %q assigned %d times, want exactly once", item.name, counts[item.name])
			}
		}
	})

	t.Run("Should balance weighted items with the LPT rule", func(t *testing.T) {
		t.Parallel()
		items := []goShardItem{
			{name: "heavy", weight: 10},
			{name: "medium", weight: 9},
			{name: "light-one", weight: 1},
			{name: "light-two", weight: 1},
			{name: "light-three", weight: 1},
		}
		bins := partitionGoShardItems(items, 2)
		loads := make([]float64, len(bins))
		for index, bin := range bins {
			for _, item := range bin {
				loads[index] += item.weight
			}
		}
		if loads[0] != 11 || loads[1] != 11 {
			t.Fatalf("bin loads = %v, want [11 11]", loads)
		}
	})

	t.Run("Should produce identical bins for identical inputs", func(t *testing.T) {
		t.Parallel()
		items := []goShardItem{
			{name: "b", weight: 2},
			{name: "a", weight: 2},
			{name: "c", weight: 7},
			{name: "d", weight: 3, splitTest: true},
		}
		first := partitionGoShardItems(items, 2)
		second := partitionGoShardItems(slices.Clone(items), 2)
		if !slices.EqualFunc(first, second, slices.Equal) {
			t.Fatalf("partitionGoShardItems() diverged: %v vs %v", first, second)
		}
	})
}

func TestGoTestObservation(t *testing.T) {
	t.Parallel()

	t.Run("Should record package and split-test durations from events", func(t *testing.T) {
		t.Parallel()
		observed := newGoTestObservation()
		observed.recordEvent(gotestsumEvent{Action: "pass", Package: "example.com/pkg", Elapsed: 4.2})
		observed.recordEvent(gotestsumEvent{Action: "fail", Package: "example.com/pkg", Elapsed: 6.8})
		observed.recordEvent(
			gotestsumEvent{Action: "pass", Package: goSplitTestPackage, Test: "TestSplit", Elapsed: 3.5},
		)
		if got := observed.packages["example.com/pkg"]; got != 6.8 {
			t.Fatalf("package duration = %v, want max 6.8", got)
		}
		if got := observed.splitTests["TestSplit"]; got != 3.5 {
			t.Fatalf("split test duration = %v, want 3.5", got)
		}
	})

	t.Run("Should ignore cached results, subtests, and per-test regular events", func(t *testing.T) {
		t.Parallel()
		observed := newGoTestObservation()
		observed.recordEvent(gotestsumEvent{Action: "pass", Package: "example.com/pkg", Elapsed: 0})
		observed.recordEvent(gotestsumEvent{Action: "output", Package: "example.com/pkg", Elapsed: 9})
		observed.recordEvent(
			gotestsumEvent{Action: "pass", Package: goSplitTestPackage, Test: "TestSplit/sub", Elapsed: 2},
		)
		observed.recordEvent(
			gotestsumEvent{Action: "pass", Package: "example.com/pkg", Test: "TestRegular", Elapsed: 2},
		)
		if len(observed.packages) != 0 || len(observed.splitTests) != 0 {
			t.Fatalf("observation recorded ignored events: %+v / %+v", observed.packages, observed.splitTests)
		}
	})

	t.Run("Should fold observations into the census with median defaults", func(t *testing.T) {
		t.Parallel()
		census := &goTestCensus{
			DefaultPackageSeconds:   1,
			DefaultSplitTestSeconds: 1,
			Packages:                map[string]float64{"example.com/kept": 2},
		}
		observed := newGoTestObservation()
		observed.packages["example.com/pkg"] = 10.456
		observed.splitTests["TestSplit"] = 4.004
		observed.applyTo(census)
		if got := census.Packages["example.com/pkg"]; got != 10.46 {
			t.Fatalf("folded package weight = %v, want rounded 10.46", got)
		}
		if got := census.Packages["example.com/kept"]; got != 2 {
			t.Fatalf("unobserved package weight = %v, want kept 2", got)
		}
		if got := census.SplitTests["TestSplit"]; got != 4 {
			t.Fatalf("folded split-test weight = %v, want rounded 4", got)
		}
		if census.DefaultPackageSeconds != 10.46 || census.DefaultSplitTestSeconds != 4 {
			t.Fatalf(
				"defaults = (%v, %v), want medians (10.46, 4)",
				census.DefaultPackageSeconds,
				census.DefaultSplitTestSeconds,
			)
		}
	})
}

func TestCensusPruneMissingSource(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should preserve existing source across build tags and discard removed packages and tests",
		func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeTestFile(t, root, "internal/kept/platform_windows.go", "//go:build windows\n\npackage kept\n")
			writeTestFile(t, root, "internal/store/globaldb/kept_test.go",
				"package globaldb\nimport \"testing\"\nfunc TestKept(t *testing.T) {}\n")
			census := &goTestCensus{
				DefaultPackageSeconds:   1,
				DefaultSplitTestSeconds: 1,
				Packages: map[string]float64{
					compozyModulePath + "internal/kept":    2,
					compozyModulePath + "internal/removed": 99,
				},
				SplitTests: map[string]float64{"TestKept": 3, "TestRemoved": 99},
			}
			if err := census.pruneMissingSource(root); err != nil {
				t.Fatalf("pruneMissingSource() error = %v", err)
			}
			if len(census.Packages) != 1 || census.Packages[compozyModulePath+"internal/kept"] != 2 {
				t.Fatalf("package weights = %v, want only existing source weight 2", census.Packages)
			}
			if len(census.SplitTests) != 1 || census.SplitTests["TestKept"] != 3 {
				t.Fatalf("split weights = %v, want only existing test weight 3", census.SplitTests)
			}
			if census.DefaultPackageSeconds != 2 || census.DefaultSplitTestSeconds != 3 {
				t.Fatalf("defaults = (%v, %v), want retained medians (2, 3)",
					census.DefaultPackageSeconds, census.DefaultSplitTestSeconds)
			}
		},
	)
}

// TestCensusUpdateBootstrap owns the integration-first persisted census contract.
func TestCensusUpdateBootstrap(t *testing.T) {
	t.Run("Should bootstrap integration and preserve census on source parsing failure", func(t *testing.T) {
		root := t.TempDir()
		t.Chdir(root)
		t.Setenv("COMPOZY_CENSUS_LANE", "integration")
		t.Setenv("CENSUS_RUN_ID", "trusted-run")
		writeTestFile(t, root, "magefiles/.keep", "")
		writeTestFile(
			t, root, "receipts/gotest-shard0-0.json",
			`{"Action":"pass","Package":"example/package","Elapsed":2}`,
		)
		if err := TestCensusUpdate("receipts"); err != nil {
			t.Fatal(err)
		}
		census, err := loadGoTestCensus(goTestCensusPath)
		if err != nil {
			t.Fatal(err)
		}
		if census.Integration == nil || census.Integration.SourceRun != "trusted-run" {
			t.Fatalf("integration provenance missing: %+v", census)
		}
		if census.DefaultPackageSeconds <= 0 || census.DefaultSplitTestSeconds <= 0 {
			t.Fatalf("root defaults invalid: %+v", census)
		}
		before, err := os.ReadFile(goTestCensusPath)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, root, "internal/daemon/broken_test.go", "package daemon\nfunc broken(")
		writeTestFile(
			t, root, "receipts/gotest-shard0-0.json",
			`{"Action":"pass","Package":"github.com/compozy/compozy/internal/daemon",`+
				`"Test":"TestObserved","Elapsed":3}`,
		)
		t.Setenv("CENSUS_RUN_ID", "must-not-persist")
		if err := TestCensusUpdate("receipts"); err == nil ||
			!strings.Contains(err.Error(), "parse "+filepath.Join("internal", "daemon", "broken_test.go")+":") {
			t.Fatalf("expected broken source parsing error, got %v", err)
		}
		after, err := os.ReadFile(goTestCensusPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("failed refresh changed persisted census or provenance")
		}
	})
}

func TestIntegrationObservationIdentity(t *testing.T) {
	t.Parallel()
	t.Run("Should keep equal test names in different packages separate", func(t *testing.T) {
		t.Parallel()
		observed := newGoTestObservation()
		observed.lane = "integration"
		for index, pkg := range []string{
			goSplitTestPackage,
			compozyModulePath + "internal/daemon",
			compozyModulePath + "internal/store",
			compozyModulePath + "internal/session",
		} {
			observed.recordEvent(
				gotestsumEvent{Action: "pass", Package: pkg, Test: "TestSame", Elapsed: float64(index + 1)},
			)
			if got := observed.splitTests[pkg+"#TestSame"]; got != float64(index+1) {
				t.Fatalf("%s weight = %v", pkg, got)
			}
		}
		if len(observed.splitTests) != 4 {
			t.Fatalf("weights = %v", observed.splitTests)
		}
	})
	t.Run("Should retain failed shard and split-test timings", func(t *testing.T) {
		t.Parallel()
		observed := newGoTestObservation()
		observed.lane = "integration"
		observed.recordEvent(gotestsumEvent{Action: "fail", Package: goSplitTestPackage, Elapsed: 12})
		observed.recordEvent(gotestsumEvent{Action: "fail", Package: goSplitTestPackage, Test: "TestSame", Elapsed: 5})
		if observed.packages[goSplitTestPackage] != 12 || observed.splitTests[goSplitTestPackage+"#TestSame"] != 5 {
			t.Fatalf("failed shard timings lost: %+v / %+v", observed.packages, observed.splitTests)
		}
	})
}

// Invariant: only fail-then-pass receipts count as flakes, persistent failures
// stay failures, and duplicate shard downloads do not inflate the rolling ledger.
// Owner: Mage census; canonical suite: gotest_census_test.go.
func TestFlakeReceipts(t *testing.T) {
	t.Parallel()
	t.Run("Should report recovered Go tests and reject a skipped retry", func(t *testing.T) {
		t.Parallel()
		input := `{"Package":"p","Test":"TestFlaky/sub","Action":"fail"}
{"Package":"p","Test":"TestFlaky/sub","Action":"pass"}
{"Package":"p","Test":"TestBroken","Action":"fail"}
{"Package":"p","Test":"TestBroken","Action":"fail"}
{"Package":"p","Test":"TestSkipped","Action":"fail"}
{"Package":"p","Test":"TestSkipped","Action":"skip"}
{"Package":"other","Test":"TestFlaky/sub","Action":"pass"}
{"Package":"p","Action":"fail"}`
		got, err := readGoFlakes(strings.NewReader(input))
		if err == nil || !strings.Contains(err.Error(), "TestSkipped ended with skip after failure") {
			t.Fatalf("skipped retry was not rejected: %v", err)
		}
		if len(got) != 1 || got[0].Test != "p#TestFlaky/sub" || got[0].Attempt != 2 {
			t.Fatalf("flakes = %+v", got)
		}
	})
	t.Run("Should identify the malformed Go receipt line", func(t *testing.T) {
		t.Parallel()
		if _, err := readGoFlakes(strings.NewReader("{}\nbroken JSON")); err == nil ||
			!strings.Contains(err.Error(), "decode gotest event at line 2") {
			t.Fatalf("expected malformed event at line 2, got %v", err)
		}
	})
	t.Run(
		"Should reject a failed leaf missing from the retry without reporting its parent as recovered",
		func(t *testing.T) {
			t.Parallel()
			input := `{"Package":"p","Test":"TestParent/leaf","Action":"fail"}
{"Package":"p","Test":"TestParent","Action":"fail"}
{"Package":"p","Test":"TestParent","Action":"pass"}
{"Package":"p","Test":"TestRecovered","Action":"fail"}
{"Package":"p","Test":"TestRecovered","Action":"pass"}`
			got, err := readGoFlakes(strings.NewReader(input))
			if err == nil ||
				!strings.Contains(err.Error(), "p#TestParent/leaf ended with fail after failure; retry must pass") {
				t.Fatalf("expected missing leaf rejection, got %v", err)
			}
			if len(got) != 1 || got[0].Test != "p#TestRecovered" || got[0].Attempt != 2 {
				t.Fatalf("genuine recovery lost or parent misclassified: %+v", got)
			}
		},
	)
	t.Run("Should attribute a recovered nested subtest once without conflating other packages", func(t *testing.T) {
		t.Parallel()
		var input strings.Builder
		for _, test := range []string{"TestParent", "TestParent/sub", "TestParent/sub/leaf", "TestParent/other"} {
			for _, action := range []string{"fail", "pass"} {
				fmt.Fprintf(&input, `{"Package":"p","Test":%q,"Action":%q}`+"\n", test, action)
			}
		}
		input.WriteString(`{"Package":"other","Test":"TestParent","Action":"fail"}` + "\n")
		input.WriteString(`{"Package":"other","Test":"TestParent","Action":"pass"}` + "\n")
		got, err := readGoFlakes(strings.NewReader(input.String()))
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, len(got))
		for _, receipt := range got {
			names = append(names, receipt.Test)
		}
		slices.Sort(names)
		want := []string{"other#TestParent", "p#TestParent/other", "p#TestParent/sub/leaf"}
		if !slices.Equal(names, want) {
			t.Fatalf("recovered tests=%v want=%v", names, want)
		}
	})
	t.Run("Should classify Playwright retries by outcome rather than attempt count", func(t *testing.T) {
		t.Parallel()
		var report playwrightCensus
		input := `{"suites":[{"suites":[{"specs":[{"file":"os.spec.ts","title":"tabs","tests":[
{"expectedStatus":"passed","projectName":"chromium","results":[{"status":"timedOut","retry":0},{"status":"passed","retry":1}]},
{"expectedStatus":"passed","results":[{"status":"failed","retry":0},{"status":"failed","retry":1}]},
{"expectedStatus":"failed","results":[{"status":"passed","retry":0},{"status":"failed","retry":1}]},
{"expectedStatus":"passed","results":[{"status":"passed","retry":0}]}
]}]}]}]}`
		if err := json.Unmarshal([]byte(input), &report); err != nil {
			t.Fatal(err)
		}
		got := report.flakes()
		if len(got) != 1 || got[0].Attempt != 2 || got[0].Test != "os.spec.ts#tabs [chromium]" {
			t.Fatalf("flakes = %+v", got)
		}
	})
	t.Run("Should deduplicate the same receipt while retaining distinct runs and jobs", func(t *testing.T) {
		t.Parallel()
		receipt := flakeReceipt{
			Suite:      "go",
			Test:       "p#TestFlaky",
			Job:        "shard-1",
			Run:        "42",
			SHA:        "abc",
			Attempt:    2,
			RunAttempt: "1",
		}
		other := receipt
		other.Job = "shard-2"
		next := receipt
		next.Run = "43"
		body := renderFlakeCensus([]flakeReceipt{receipt, receipt, other, next}, "compozy/compozy")
		if !strings.Contains(body, "| go / p#TestFlaky | 3 |") || !strings.Contains(body, "actions/runs/43") {
			t.Fatal(body)
		}
	})
}

// Invariant: five complete, unretried E2E-023 samples are required; the 50 ms
// assertion moves to their medians without discarding a measured outlier.
func TestPerfCensusSamples(t *testing.T) {
	t.Parallel()
	t.Run("Should retain all five measurements and reject missing or retried samples", func(t *testing.T) {
		t.Parallel()
		sample := base64.StdEncoding.EncodeToString([]byte(`{"worstSwitchFrameMs":0,"worstPeerConvergenceTaskMs":73}`))
		testJSON := `{"expectedStatus":"passed","results":[{"status":"passed","retry":0,"attachments":[{"name":"perf-envelope","body":"` + sample + `"}]}]}`
		decode := func(input string) playwrightCensus {
			var report playwrightCensus
			if err := json.Unmarshal([]byte(input), &report); err != nil {
				t.Fatal(err)
			}
			return report
		}
		input := `{"suites":[{"specs":[{"title":"E2E-023: the 11-tab envelope @perf","tests":[` + strings.Join(
			[]string{testJSON, testJSON, testJSON, testJSON, testJSON},
			",",
		) + `]}]}]}`
		samples, err := decode(input).perfSamples()
		if err != nil || len(samples) != 5 || samples[0].Peer != 73 {
			t.Fatalf("samples=%+v err=%v", samples, err)
		}
		for _, tc := range []struct {
			name      string
			input     string
			wantError string
		}{
			{"Should reject a retried sample", strings.Replace(input, `"retry":0`, `"retry":1`, 1), "without retries"},
			{"Should reject a failed sample", strings.Replace(input, `"status":"passed"`, `"status":"failed"`, 1), "must pass its functional assertions"},
			{"Should reject a missing envelope", strings.Replace(input, `"perf-envelope"`, `"missing"`, 1), "missing performance envelope"},
			{"Should reject an incomplete sample count", strings.Replace(input, `,"tests":[`+testJSON+`,`, `,"tests":[`, 1), "got 4 performance samples, require 5"},
			{"Should reject missing measurements", strings.Replace(input, sample, base64.StdEncoding.EncodeToString([]byte(`{}`)), 1), "missing performance measurements"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := decode(
					tc.input,
				).perfSamples(); err == nil ||
					!strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected %q, got %v", tc.wantError, err)
				}
			})
		}
	})
}

func TestPerfCensusJudgment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		values    []float64
		wantError string
	}{
		{"Should tolerate an isolated spike while retaining the 50ms median budget", []float64{0, 50, 0, 73, 0}, ""},
		{"Should fail a majority breach of the unchanged budget", []float64{0, 51, 51, 73, 0}, "median switch=51ms peer=51ms exceeds 50ms"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report := playwrightCensus{
				Suites: []playwrightCensusSuite{
					{Specs: []playwrightCensusSpec{{Title: "E2E-023: the 11-tab envelope @perf"}}},
				},
			}
			for _, value := range tc.values {
				envelope, err := json.Marshal(perfCensusEnvelope{Switch: value, Peer: value})
				if err != nil {
					t.Fatal(err)
				}
				report.Suites[0].Specs[0].Tests = append(report.Suites[0].Specs[0].Tests, playwrightCensusTest{
					ExpectedStatus: "passed",
					Results: []playwrightCensusResult{
						{
							Status: "passed",
							Attachments: []playwrightCensusAttachment{
								{Name: "perf-envelope", Body: base64.StdEncoding.EncodeToString(envelope)},
							},
						},
					},
				})
			}
			data, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "playwright-perf.json")
			if err := os.WriteFile(file, data, 0o600); err != nil {
				t.Fatal(err)
			}
			err = TestPerfCensus(file)
			if tc.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("judgment=%v wantError=%q", err, tc.wantError)
			}
		})
	}
}
