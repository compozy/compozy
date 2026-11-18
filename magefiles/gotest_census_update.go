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
// record while their source exists, and defaults become the medians of the merged maps.
func TestCensusUpdate(jsonDir string) error {
	census, err := loadGoTestCensus(goTestCensusPath)
	if err != nil {
		return err
	}
	if census == nil {
		census = &goTestCensus{DefaultPackageSeconds: 1, DefaultSplitTestSeconds: 1}
	}
	var files []string
	err = filepath.WalkDir(jsonDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("find gotestsum JSON files in %q: %w", jsonDir, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no gotestsum JSON files found in %q", jsonDir)
	}
	slices.Sort(files)
	target := census
	lane := os.Getenv("COMPOZY_CENSUS_LANE")
	if lane != "" && lane != "unit" && lane != "integration" {
		return fmt.Errorf("unknown census lane %q", lane)
	}
	if lane == "integration" {
		if census.Integration == nil {
			census.Integration = &goTestCensus{}
		}
		target = census.Integration
	}
	observed := newGoTestObservation()
	observed.lane = lane
	for _, file := range files {
		if err := observed.readGotestsumJSONFile(file); err != nil {
			return err
		}
	}
	observed.applyTo(target)
	target.SourceRun = os.Getenv("CENSUS_RUN_ID")
	if err := target.pruneMissingSource("."); err != nil {
		return err
	}
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
		len(target.Packages),
		len(target.SplitTests),
	)
	return nil
}

type goTestObservation struct {
	lane       string
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
	if o.lane == "integration" && event.Test != "" {
		if !strings.Contains(event.Test, "/") {
			key := event.Package + "#" + event.Test
			o.splitTests[key] = max(o.splitTests[key], event.Elapsed)
		}
		return
	}
	if event.Package == goSplitTestPackage && o.lane != "integration" {
		if event.Test != "" && !strings.Contains(event.Test, "/") {
			o.splitTests[event.Test] = max(o.splitTests[event.Test], event.Elapsed)
		}
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

// pruneMissingSource checks source files across build tags so platform-specific
// packages retain their measured weights when the updater runs on another OS.
func (c *goTestCensus) pruneMissingSource(root string) error {
	for packagePath := range c.Packages {
		relative, local := strings.CutPrefix(packagePath, compozyModulePath)
		if !local {
			continue
		}
		files, err := filepath.Glob(filepath.Join(root, relative, "*.go"))
		if err != nil {
			return fmt.Errorf("find census package sources %q: %w", packagePath, err)
		}
		if len(files) == 0 {
			delete(c.Packages, packagePath)
		}
	}
	// Parse each package once, even when every integration test has a weight.
	sourceTests := make(map[string]map[string]struct{})
	for key := range c.SplitTests {
		packagePath, testName, qualified := strings.Cut(key, "#")
		if !qualified {
			packagePath, testName = goSplitTestPackage, key
		}
		tests, loaded := sourceTests[packagePath]
		if !loaded {
			files, err := filepath.Glob(
				filepath.Join(root, strings.TrimPrefix(packagePath, compozyModulePath), "*_test.go"),
			)
			if err != nil {
				return fmt.Errorf("find census sources %q: %w", packagePath, err)
			}
			tests = make(map[string]struct{})
			for _, file := range files {
				names, err := topLevelTestsInFile(file)
				if err != nil {
					return err
				}
				for _, name := range names {
					tests[name] = struct{}{}
				}
			}
			sourceTests[packagePath] = tests
		}
		if _, exists := tests[testName]; !exists {
			delete(c.SplitTests, key)
		}
	}
	c.DefaultPackageSeconds = censusMedian(c.Packages, c.DefaultPackageSeconds)
	c.DefaultSplitTestSeconds = censusMedian(c.SplitTests, c.DefaultSplitTestSeconds)
	return nil
}
