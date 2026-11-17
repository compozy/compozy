//go:build mage

package main

import (
	"bufio"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// flakeReceipt records a recovered test, not a successful whole-job rerun.
// Attempt is one-based; RunAttempt distinguishes manual GitHub job reruns.
type flakeReceipt struct {
	Suite      string `json:"suite"`
	Test       string `json:"test"`
	Job        string `json:"job"`
	Run        string `json:"run"`
	SHA        string `json:"sha"`
	Attempt    int    `json:"attempt"`
	RunAttempt string `json:"run_attempt"`
}

type playwrightCensus struct {
	Suites []playwrightCensusSuite `json:"suites"`
	Errors []jsontext.Value        `json:"errors"`
}

type playwrightCensusSuite struct {
	Suites []playwrightCensusSuite `json:"suites"`
	Specs  []playwrightCensusSpec  `json:"specs"`
}

type playwrightCensusSpec struct {
	Title string                 `json:"title"`
	File  string                 `json:"file"`
	Tests []playwrightCensusTest `json:"tests"`
}

type playwrightCensusTest struct {
	ProjectName    string                   `json:"projectName"`
	ExpectedStatus string                   `json:"expectedStatus"`
	Results        []playwrightCensusResult `json:"results"`
}

type playwrightCensusResult struct {
	Status      string                       `json:"status"`
	Retry       int                          `json:"retry"`
	Attachments []playwrightCensusAttachment `json:"attachments"`
}

type playwrightCensusAttachment struct {
	Name string `json:"name"`
	Body string `json:"body"`
	Path string `json:"path"`
}

func (report playwrightCensus) specs() []playwrightCensusSpec {
	var specs []playwrightCensusSpec
	var visit func([]playwrightCensusSuite)
	visit = func(suites []playwrightCensusSuite) {
		for _, suite := range suites {
			specs = append(specs, suite.Specs...)
			visit(suite.Suites)
		}
	}
	visit(report.Suites)
	return specs
}

// TestFlakes converts the existing test receipts to a small, log-free JSON artifact.
// It never changes the test runner's exit status: workflows run it with always().
func TestFlakes(receiptDir, output string) error {
	flakes := []flakeReceipt{}
	err := filepath.WalkDir(receiptDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		var found []flakeReceipt
		var err error
		switch {
		case strings.HasPrefix(entry.Name(), "gotest-") && strings.HasSuffix(path, ".json"):
			found, err = readGoFlakeFile(path)
		case strings.HasPrefix(entry.Name(), "playwright") && strings.HasSuffix(path, ".json"):
			var report playwrightCensus
			report, err = readPlaywrightCensus(path)
			if err == nil {
				found = report.flakes()
			}
		default:
			return nil
		}
		if err != nil {
			return fmt.Errorf("read test receipt %s: %w", path, err)
		}
		flakes = append(flakes, found...)
		return nil
	})
	if err != nil {
		return err
	}
	for index := range flakes {
		flakes[index].Job = os.Getenv("COMPOZY_FLAKE_JOB")
		flakes[index].Run = os.Getenv("GITHUB_RUN_ID")
		flakes[index].SHA = os.Getenv("GITHUB_SHA")
		flakes[index].RunAttempt = os.Getenv("GITHUB_RUN_ATTEMPT")
		fmt.Printf(
			"::warning title=FLAKY::%s passed on attempt %d\n",
			escapeFlakeAnnotation(flakes[index].Test),
			flakes[index].Attempt,
		)
	}
	return writeFlakeReceipts(output, flakes)
}

func readGoFlakeFile(path string) (flakes []flakeReceipt, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	return readGoFlakes(file)
}

func readGoFlakes(reader io.Reader) ([]flakeReceipt, error) {
	type outcome struct {
		failed  bool
		last    string
		attempt int
	}
	outcomes := map[string]outcome{}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var event gotestsumEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, err
		}
		if event.Test == "" || (event.Action != "pass" && event.Action != "fail" && event.Action != "skip") {
			continue
		}
		key := event.Package + "#" + event.Test
		result := outcomes[key]
		result.attempt++
		result.failed = result.failed || event.Action == "fail"
		result.last = event.Action
		outcomes[key] = result
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flakes := []flakeReceipt{}
	for name, result := range outcomes {
		if result.failed && result.last == "pass" {
			flakes = append(flakes, flakeReceipt{Suite: "go", Test: name, Attempt: result.attempt})
		}
	}
	return flakes, nil
}

func readPlaywrightCensus(path string) (playwrightCensus, error) {
	var report playwrightCensus
	data, err := os.ReadFile(path)
	if err != nil {
		return report, err
	}
	err = json.Unmarshal(data, &report)
	return report, err
}

func (report playwrightCensus) flakes() []flakeReceipt {
	flakes := []flakeReceipt{}
	for _, spec := range report.specs() {
		for _, test := range spec.Tests {
			// Expected failures and skipped tests are not recovered flakes.
			if test.ExpectedStatus != "passed" || len(test.Results) < 2 {
				continue
			}
			last := test.Results[len(test.Results)-1]
			failed := false
			for _, result := range test.Results[:len(test.Results)-1] {
				failed = failed || result.Status == "failed" || result.Status == "timedOut" ||
					result.Status == "interrupted"
			}
			if failed && last.Status == "passed" && last.Retry > 0 {
				flakes = append(flakes, flakeReceipt{
					Suite:   "playwright",
					Test:    spec.File + "#" + spec.Title + " [" + test.ProjectName + "]",
					Attempt: last.Retry + 1,
				})
			}
		}
	}
	return flakes
}

func writeFlakeReceipts(output string, flakes []flakeReceipt) error {
	slices.SortFunc(flakes, func(a, b flakeReceipt) int {
		return strings.Compare(flakeIdentity(a), flakeIdentity(b))
	})
	flakes = slices.CompactFunc(flakes, func(a, b flakeReceipt) bool { return flakeIdentity(a) == flakeIdentity(b) })
	data, err := json.Marshal(flakes, jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(output, append(data, '\n'), 0o644); err != nil {
		return err
	}
	if path := os.Getenv("GITHUB_OUTPUT"); path != "" {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := fmt.Fprintf(file, "count=%d\n", len(flakes))
		return errors.Join(writeErr, file.Close())
	}
	return nil
}

func flakeIdentity(receipt flakeReceipt) string {
	return fmt.Sprintf(
		"%q/%q/%q/%q/%q/%q/%d",
		receipt.Suite,
		receipt.Test,
		receipt.Job,
		receipt.Run,
		receipt.RunAttempt,
		receipt.SHA,
		receipt.Attempt,
	)
}

func escapeFlakeAnnotation(value string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(value)
}

// TestFlakeMerge collects shard artifacts once per workflow run. The scheduled
// census downloads only this small, consistently named artifact (never logs).
func TestFlakeMerge(receiptDir, output string) error {
	flakes, err := loadFlakeReceipts(receiptDir)
	if err != nil {
		return err
	}
	return writeFlakeReceipts(output, flakes)
}

func loadFlakeReceipts(dir string) ([]flakeReceipt, error) {
	flakes := []flakeReceipt{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "flakes.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var receipts []flakeReceipt
		if err := json.Unmarshal(data, &receipts); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
		flakes = append(flakes, receipts...)
		return nil
	})
	return flakes, err
}
