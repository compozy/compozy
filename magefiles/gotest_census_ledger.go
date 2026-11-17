//go:build mage

package main

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
)

// TestFlakeCensus renders a rolling seven-day census for one tracking issue.
// Repeated artifact downloads and manual job reruns cannot double-count a receipt.
func TestFlakeCensus(receiptDir, output string) error {
	flakes, err := loadFlakeReceipts(receiptDir)
	if err != nil {
		return err
	}
	body := renderFlakeCensus(flakes, os.Getenv("GITHUB_REPOSITORY"))
	return os.WriteFile(output, []byte(body), 0o644)
}

func renderFlakeCensus(flakes []flakeReceipt, repository string) string {
	type group struct {
		tests map[string]bool
		last  flakeReceipt
	}
	groups := map[string]*group{}
	for _, flake := range flakes {
		key := flake.Suite + " / " + flake.Test
		if groups[key] == nil {
			groups[key] = &group{tests: map[string]bool{}}
		}
		item := groups[key]
		item.tests[flakeIdentity(flake)] = true
		if item.last.Run == "" || len(flake.Run) > len(item.last.Run) ||
			(len(flake.Run) == len(item.last.Run) && flake.Run > item.last.Run) {
			item.last = flake
		}
	}
	names := slices.Sorted(maps.Keys(groups))
	var body strings.Builder
	body.WriteString(
		`<!-- compozy-flake-census -->
# CI flake ledger

Rolling seven-day census of tests that failed initially and passed the single retry.
Second failures still fail the job. Counts deduplicate suite/test/job/run/SHA/test
attempt/workflow attempt; this is not a regression pass rate. Only same-repository
CI/Release structured artifacts are read, never logs.

Refs #1547 and #1736 for historical investigation.

| Test | Recovered occurrences | Latest evidence |
| --- | ---: | --- |
`,
	)
	for _, name := range names {
		item := groups[name]
		// Escape untrusted test titles as text, including table/control delimiters.
		title := flakeMarkdown(name)
		fmt.Fprintf(
			&body,
			"| %s | %d | [run %s](https://github.com/%s/actions/runs/%s), SHA `%s`, job `%s`, attempt %d |\n",
			title,
			len(item.tests),
			item.last.Run,
			repository,
			item.last.Run,
			flakeMarkdown(item.last.SHA),
			flakeMarkdown(item.last.Job),
			item.last.Attempt,
		)
	}
	if len(names) == 0 {
		body.WriteString("| No recovered flakes observed | 0 | — |\n")
	}
	body.WriteString(
		`
Acceptance: investigate recurrent tests in their owning suites without raising
budgets, skipping assertions or adding job retries. Artifacts retain individual
receipts; this issue is updated in place by the scheduled Go test census.
`,
	)
	return body.String()
}

func flakeMarkdown(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "|", "&#124;", "\n", " ", "\r", " ", "`", "&#96;").
		Replace(value)
}
