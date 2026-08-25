//go:build mage

// Suite: Go test shard census
// Invariant: Census weights load deterministically with positive defaults, the LPT partition
// covers every item exactly once with input-determined assignment, and census updates fold
// gotestsum events into stable rounded weights.
// Boundary IN: Census JSON bytes, shard item lists, and gotestsum test2json events.
// Boundary OUT: Shard invocation building in gotest_lane.go.

package main

import (
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
		observed.recordEvent(gotestsumEvent{Action: "pass", Package: goSplitTestPackage, Test: "TestSplit", Elapsed: 3.5})
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
		observed.recordEvent(gotestsumEvent{Action: "pass", Package: goSplitTestPackage, Test: "TestSplit/sub", Elapsed: 2})
		observed.recordEvent(gotestsumEvent{Action: "pass", Package: "example.com/pkg", Test: "TestRegular", Elapsed: 2})
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
