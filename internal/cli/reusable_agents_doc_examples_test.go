package cli

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	coreRun "github.com/compozy/compozy/internal/core/run"
)

func TestDocumentedAgentsInspectExampleMatchesReviewerFixture(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	copyCLIDocumentedAgentFixture(t, workspaceRoot, "reviewer")
	withWorkingDir(t, workspaceRoot)

	output, err := executeRootCommand("agents", "inspect", "reviewer")
	if err != nil {
		t.Fatalf("execute documented agents inspect example: %v\noutput:\n%s", err, output)
	}

	for _, want := range []string{
		"Agent: reviewer",
		"Status: valid",
		"Source: workspace",
		"Title: Reviewer",
		"Description: Reviews implementation plans and diffs before code lands.",
		"Runtime defaults: ide=codex model=gpt-5.4 reasoning=high access=default",
		"MCP servers: none",
		"Validation: OK",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected inspect output to contain %q\noutput:\n%s", want, output)
		}
	}
}

func TestDocumentedExecAgentExampleWorksWithReviewerFixture(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	copyCLIDocumentedAgentFixture(t, workspaceRoot, "reviewer")
	withWorkingDir(t, workspaceRoot)

	var capturedPrompt string
	restore := coreRun.SwapNewAgentClientForTest(
		func(_ context.Context, _ agent.ClientConfig) (agent.Client, error) {
			return &cliCapturingACPClient{
				createSessionFn: func(_ context.Context, req agent.SessionRequest) (agent.Session, error) {
					capturedPrompt = string(req.Prompt)
					return newCLIACPTestSession(
						"sess-reviewer",
						agent.SessionIdentity{ACPSessionID: "sess-reviewer"},
						[]model.SessionUpdate{
							{
								Kind: model.UpdateKindAgentMessageChunk,
								Blocks: []model.ContentBlock{
									mustCLIContentBlock(t, model.TextBlock{Text: "reviewer ready"}),
								},
								Status: model.StatusRunning,
							},
						},
						nil,
					), nil
				},
			}, nil
		},
	)
	defer restore()

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--agent",
		"reviewer",
		"Review the staged changes",
	)
	if err != nil {
		t.Fatalf("execute documented exec --agent example: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for documented exec example, got %q", stderr)
	}
	if !strings.Contains(stdout, "reviewer ready") {
		t.Fatalf("expected documented exec example output, got %q", stdout)
	}
	for _, want := range []string{
		"<agent_metadata>",
		"name: reviewer",
		"Review the user's request, inspect the relevant diff or files",
	} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("expected captured reviewer prompt to contain %q\nprompt:\n%s", want, capturedPrompt)
		}
	}
}

func copyCLIDocumentedAgentFixture(t *testing.T, workspaceRoot, name string) {
	t.Helper()

	sourceRoot := mustCLIRepoRootPath(t, "docs", "examples", "agents", name)
	targetRoot := filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", name)

	if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetRoot, relPath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, content, 0o600)
	}); err != nil {
		t.Fatalf("copy documented CLI agent fixture %q: %v", name, err)
	}
}
