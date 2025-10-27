package main

import (
	"sort"
	"testing"
)

func TestExtractTaskNumber(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"_task_1.md", 1},
		{"_task_10.md", 10},
		{"_task_2.md", 2},
		{"_task_100.md", 100},
		{"invalid.md", 0},
		{"_task_abc.md", 0},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := extractTaskNumber(tt.filename)
			if result != tt.expected {
				t.Errorf("extractTaskNumber(%q) = %d, want %d", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestTaskSorting(t *testing.T) {
	// Simulating the filenames in wrong (lexicographic) order
	names := []string{
		"_task_1.md",
		"_task_10.md",
		"_task_11.md",
		"_task_2.md",
		"_task_3.md",
		"_task_9.md",
	}

	// Apply numeric sorting
	sort.Slice(names, func(i, j int) bool {
		numI := extractTaskNumber(names[i])
		numJ := extractTaskNumber(names[j])
		return numI < numJ
	})

	// Expected correct order
	expected := []string{
		"_task_1.md",
		"_task_2.md",
		"_task_3.md",
		"_task_9.md",
		"_task_10.md",
		"_task_11.md",
	}

	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Position %d: got %q, want %q", i, name, expected[i])
		}
	}
}

func TestFlattenAndSortIssues(t *testing.T) {
	// Create test issues in unsorted order
	groups := map[string][]issueEntry{
		"group1": {
			{name: "_task_10.md", codeFile: "_task_10"},
			{name: "_task_2.md", codeFile: "_task_2"},
		},
		"group2": {
			{name: "_task_1.md", codeFile: "_task_1"},
			{name: "_task_11.md", codeFile: "_task_11"},
		},
	}

	t.Run("PRD tasks mode uses numeric sorting", func(t *testing.T) {
		result := flattenAndSortIssues(groups, ExecutionModePRDTasks)
		expected := []string{"_task_1.md", "_task_2.md", "_task_10.md", "_task_11.md"}

		if len(result) != len(expected) {
			t.Fatalf("Expected %d issues, got %d", len(expected), len(result))
		}

		for i, issue := range result {
			if issue.name != expected[i] {
				t.Errorf("Position %d: got %q, want %q", i, issue.name, expected[i])
			}
		}
	})

	t.Run("PR Review mode uses lexicographic sorting", func(t *testing.T) {
		// Use different naming that would sort differently with numeric vs lexicographic
		crGroups := map[string][]issueEntry{
			"group1": {
				{name: "issue_b.md", codeFile: "file_b"},
				{name: "issue_a.md", codeFile: "file_a"},
			},
		}

		result := flattenAndSortIssues(crGroups, ExecutionModePRReview)
		expected := []string{"issue_a.md", "issue_b.md"}

		if len(result) != len(expected) {
			t.Fatalf("Expected %d issues, got %d", len(expected), len(result))
		}

		for i, issue := range result {
			if issue.name != expected[i] {
				t.Errorf("Position %d: got %q, want %q", i, issue.name, expected[i])
			}
		}
	})
}
