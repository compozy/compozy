//go:build mage

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// TestCensusUpdate folds gotestsum JSON files (`--jsonfile`, one test2json
// event per line) from jsonDir into the committed shard census. Values observed
// in the input replace existing entries; entries not observed keep their
// record, and the defaults become the medians of the merged maps.
func TestCensusUpdate(jsonDir string) error {
	census, err := loadGoTestCensus(goTestCensusPath)
	if err != nil {
		return err
	}
	if census == nil {
		census = &goTestCensus{}
	}
	files, err := filepath.Glob(filepath.Join(jsonDir, "*.json"))
	if err != nil {
		return fmt.Errorf("glob gotestsum JSON files in %q: %w", jsonDir, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no gotestsum JSON files found in %q", jsonDir)
	}
	slices.Sort(files)
	observed := newGoTestObservation()
	for _, file := range files {
		if err := observed.readGotestsumJSONFile(file); err != nil {
			return err
		}
	}
	observed.applyTo(census)
	data, err := json.MarshalIndent(census, "", "  ")
	if err != nil {
		return fmt.Errorf("encode go test census: %w", err)
	}
	if err := os.WriteFile(goTestCensusPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write go test census %q: %w", goTestCensusPath, err)
	}
	fmt.Printf(
		"Updated %s from %d files: %d packages, %d split tests\n",
		goTestCensusPath,
		len(files),
		len(census.Packages),
		len(census.SplitTests),
	)
	return nil
}

type goTestObservation struct {
	packages   map[string]float64
	splitTests map[string]float64
}

type gotestsumEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
}

func newGoTestObservation() *goTestObservation {
	return &goTestObservation{
		packages:   make(map[string]float64),
		splitTests: make(map[string]float64),
	}
}

func (o *goTestObservation) readGotestsumJSONFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open gotestsum JSON file %q: %w", path, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		event := gotestsumEvent{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return fmt.Errorf("decode gotestsum event in %q: %w", path, err)
		}
		o.recordEvent(event)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan gotestsum JSON file %q: %w", path, err)
	}
	return nil
}

// Zero-elapsed results are cached replays and carry no duration signal.
func (o *goTestObservation) recordEvent(event gotestsumEvent) {
	if event.Action != "pass" && event.Action != "fail" {
		return
	}
	if event.Elapsed <= 0 {
		return
	}
	if event.Package == goSplitTestPackage {
		if event.Test == "" || strings.Contains(event.Test, "/") {
			return
		}
		o.splitTests[event.Test] = max(o.splitTests[event.Test], event.Elapsed)
		return
	}
	if event.Test != "" {
		return
	}
	o.packages[event.Package] = max(o.packages[event.Package], event.Elapsed)
}

func (o *goTestObservation) applyTo(census *goTestCensus) {
	if census.Packages == nil {
		census.Packages = make(map[string]float64, len(o.packages))
	}
	if census.SplitTests == nil {
		census.SplitTests = make(map[string]float64, len(o.splitTests))
	}
	for name, seconds := range o.packages {
		census.Packages[name] = roundCensusSeconds(seconds)
	}
	for name, seconds := range o.splitTests {
		census.SplitTests[name] = roundCensusSeconds(seconds)
	}
	census.DefaultPackageSeconds = censusMedian(census.Packages, census.DefaultPackageSeconds)
	census.DefaultSplitTestSeconds = censusMedian(census.SplitTests, census.DefaultSplitTestSeconds)
}

func censusMedian(values map[string]float64, fallback float64) float64 {
	if len(values) == 0 {
		return max(fallback, goTestCensusMinimumWeight)
	}
	ordered := make([]float64, 0, len(values))
	for _, value := range values {
		ordered = append(ordered, value)
	}
	slices.Sort(ordered)
	return max(roundCensusSeconds(ordered[len(ordered)/2]), goTestCensusMinimumWeight)
}

func roundCensusSeconds(seconds float64) float64 {
	return math.Round(seconds*100) / 100
}
