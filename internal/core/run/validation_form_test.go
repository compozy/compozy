package run

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/tasks"

	tea "charm.land/bubbletea/v2"
)

func TestValidationFormContinueKeyQuitsWithContinuedDecision(t *testing.T) {
	t.Parallel()

	model := newValidationFormModel(testValidationReport(), testValidationRegistry(t), &bytes.Buffer{}, nil)

	next, cmd := model.Update(keyText("c"))
	if cmd == nil {
		t.Fatal("expected continue key to return quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected quit command, got %T", cmd())
	}

	typed := next.(*validationFormModel)
	if got := typed.decision; got != PreflightContinued {
		t.Fatalf("expected continued decision, got %q", got)
	}
}

func TestValidationFormAbortKeysQuitWithAbortedDecision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "a", key: keyText("a")},
		{name: "esc", key: keyText("esc")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			model := newValidationFormModel(testValidationReport(), testValidationRegistry(t), &bytes.Buffer{}, nil)
			next, cmd := model.Update(tc.key)
			if cmd == nil {
				t.Fatal("expected abort key to return quit command")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("expected quit command, got %T", cmd())
			}

			typed := next.(*validationFormModel)
			if got := typed.decision; got != PreflightAborted {
				t.Fatalf("expected aborted decision, got %q", got)
			}
		})
	}
}

func TestValidationFormCopyPromptDefersClipboardWriteUntilProgramExit(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	model := newValidationFormModel(testValidationReport(), testValidationRegistry(t), &stderr, nil)

	next, cmd := model.Update(keyText("p"))
	if cmd == nil {
		t.Fatal("expected copy key to return quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected quit command, got %T", cmd())
	}

	typed := next.(*validationFormModel)
	if got := typed.decision; got != PreflightAborted {
		t.Fatalf("expected copy action to abort, got %q", got)
	}
	if !typed.shouldCopyFixPrompt {
		t.Fatal("expected copy action to defer clipboard copy until the program exits")
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected no clipboard feedback during the live Bubble Tea update, got %q", got)
	}
}

func TestValidationFormViewRendersOffendingFilesAndIssues(t *testing.T) {
	t.Parallel()

	model := newValidationFormModel(testValidationReport(), testValidationRegistry(t), &bytes.Buffer{}, nil)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := model.View().Content
	for _, want := range []string{
		"Task Metadata Validation Required",
		"/tmp/tasks/task_01.md",
		"title is required",
		"/tmp/tasks/task_02.md",
		"type \"\" must be one of:",
		"Continue anyway",
		"Copy fix prompt",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q\nview:\n%s", want, view)
		}
	}
}

func TestValidationFormBodyOwnsSurfaceBackground(t *testing.T) {
	t.Parallel()

	model := newValidationFormModel(testValidationReport(), testValidationRegistry(t), &bytes.Buffer{}, nil)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	assertRenderedCellsUseBackground(t, model.renderBody(model.contentWidth()), colorBgSurface)
}

func TestValidationFormScrollKeysMoveIssueViewport(t *testing.T) {
	t.Parallel()

	model := newValidationFormModel(testScrollableValidationReport(12), testValidationRegistry(t), &bytes.Buffer{}, nil)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 18})
	if model.issueViewport.TotalLineCount() <= model.issueViewport.Height() {
		t.Fatalf(
			"expected long validation report to exceed viewport height, got total=%d height=%d",
			model.issueViewport.TotalLineCount(),
			model.issueViewport.Height(),
		)
	}

	next, _ := model.Update(keyText("end"))
	typed := next.(*validationFormModel)
	if got := typed.issueViewport.YOffset(); got == 0 {
		t.Fatal("expected end key to move the issue viewport")
	}
	view := typed.View().Content
	if !strings.Contains(view, "/tmp/tasks/task_12.md") {
		t.Fatalf("expected scrolled view to include the final issue, got:\n%s", view)
	}
	if strings.Contains(view, "/tmp/tasks/task_01.md") {
		t.Fatalf("expected scrolled view to move past the first issue, got:\n%s", view)
	}

	next, _ = typed.Update(keyText("home"))
	typed = next.(*validationFormModel)
	if got := typed.issueViewport.YOffset(); got != 0 {
		t.Fatalf("expected home key to restore the viewport to the top, got offset %d", got)
	}
}

func TestValidationFormInitAndHelpers(t *testing.T) {
	t.Parallel()

	model := newValidationFormModel(testValidationReport(), testValidationRegistry(t), nil, nil)
	if cmd := model.Init(); cmd != nil {
		t.Fatalf("expected nil init command, got %v", cmd)
	}

	if got := clampInt(5, 10, 20); got != 10 {
		t.Fatalf("expected lower clamp, got %d", got)
	}
	if got := clampInt(25, 10, 20); got != 20 {
		t.Fatalf("expected upper clamp, got %d", got)
	}
	if got := clampInt(15, 10, 20); got != 15 {
		t.Fatalf("expected unclamped value, got %d", got)
	}

	model.fixPrompt = ""
	if err := model.copyFixPrompt(); err != nil {
		t.Fatalf("expected empty clipboard copy to be ignored, got %v", err)
	}
}

func testValidationRegistry(t *testing.T) *tasks.TypeRegistry {
	t.Helper()

	registry, err := tasks.NewRegistry(nil)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	return registry
}

func testValidationReport() tasks.Report {
	return tasks.Report{
		TasksDir: "/tmp/tasks",
		Scanned:  2,
		Issues: []tasks.Issue{
			{
				Path:    "/tmp/tasks/task_01.md",
				Field:   "title",
				Message: "title is required",
			},
			{
				Path:    "/tmp/tasks/task_02.md",
				Field:   "type",
				Message: `type "" must be one of: backend, bugfix, chore, docs, frontend, infra, refactor, test`,
			},
		},
	}
}

func testScrollableValidationReport(total int) tasks.Report {
	issues := make([]tasks.Issue, 0, total)
	for i := 1; i <= total; i++ {
		issues = append(issues, tasks.Issue{
			Path:    fmt.Sprintf("/tmp/tasks/task_%02d.md", i),
			Field:   "title",
			Message: fmt.Sprintf("title %d is required", i),
		})
	}
	return tasks.Report{
		TasksDir: "/tmp/tasks",
		Scanned:  total,
		Issues:   issues,
	}
}
