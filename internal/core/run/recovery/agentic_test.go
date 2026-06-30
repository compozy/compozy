package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
	"github.com/compozy/compozy/internal/core/run/internal/acpshared"
	"github.com/compozy/compozy/internal/core/workspace"
)

func TestAgenticRemediationBuildsNonRecursiveRecoveryRunConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should build a non-recursive recovery run config", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		runDir := t.TempDir()
		stdoutLog := writeTestFile(t, workspaceRoot, "stdout.log", "go test ./...\n")
		stderrLog := writeTestFile(t, workspaceRoot, "stderr.log", "exit status 1\n")

		var captured model.RuntimeConfig
		strategy := NewAgenticRemediation(WithPreparedPromptExecutor(
			func(
				_ context.Context,
				cfg *model.RuntimeConfig,
				promptText string,
				_ *reusableagents.ExecutionContext,
				_ execpkg.SessionMCPBuilder,
			) (execpkg.PreparedPromptResult, error) {
				captured = *cfg
				if strings.TrimSpace(promptText) == "" {
					t.Fatal("expected recovery prompt")
				}
				return execpkg.PreparedPromptResult{
					RunID:  "recovery-run",
					Output: `{"decision":"reject","reason":"not project-side","changed_files":[]}`,
				}, nil
			},
		))

		verdict, err := strategy.Remediate(context.Background(), RemediationInput{
			Outcome: RunOutcome{
				RunID:        "failed-run",
				Status:       StatusFailed,
				ArtifactsDir: runDir,
				Jobs: []JobOutcome{{
					SafeName: "unit-tests",
					Status:   StatusFailed,
					ExitCode: 1,
					OutLog:   stdoutLog,
					ErrLog:   stderrLog,
					Error:    "tests failed",
				}},
			},
			FailedConfig: &model.RuntimeConfig{
				WorkspaceRoot: workspaceRoot,
				RunID:         "failed-run",
				Mode:          model.ExecutionModePRDTasks,
				Name:          "agentic-recovery",
				PR:            "123",
				Recursive:     true,
				Timeout:       2 * time.Minute,
				IDE:           model.IDEClaude,
				Model:         "failed-model",
			},
			Recovery: workspace.AgentRecoveryConfig{
				IDE:             strPtr(model.IDECodex),
				Model:           strPtr("gpt-5.5"),
				ReasoningEffort: strPtr("high"),
			},
		})
		if err != nil {
			t.Fatalf("Remediate() error = %v", err)
		}
		if verdict.Decision != VerdictReject {
			t.Fatalf("verdict = %#v, want reject", verdict)
		}
		if captured.RecoveryAttempt != 1 {
			t.Fatalf("RecoveryAttempt = %d, want 1", captured.RecoveryAttempt)
		}
		if captured.Recursive {
			t.Fatal("expected recovery run to be non-recursive")
		}
		if captured.RunID != "" {
			t.Fatalf("RunID = %q, want empty so exec allocates a fresh run id", captured.RunID)
		}
		if captured.ParentRunID != "failed-run" {
			t.Fatalf("ParentRunID = %q, want failed run id", captured.ParentRunID)
		}
		if captured.Timeout != 2*time.Minute {
			t.Fatalf("Timeout = %s, want failed-run bounded timeout", captured.Timeout)
		}
		if captured.IDE != model.IDECodex || captured.Model != "gpt-5.5" || captured.ReasoningEffort != "high" {
			t.Fatalf("unexpected recovery runtime: %#v", captured)
		}
		if captured.SystemPrompt == "" {
			t.Fatal("expected recovery system prompt")
		}
	})
}

func TestAgenticRemediationSystemPromptContainsGuidanceAndFailureContext(t *testing.T) {
	t.Parallel()

	t.Run("Should include guidance and failure context in the system prompt", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		stderrLog := writeTestFile(t, workspaceRoot, "stderr.log", "panic: broken invariant\n")
		var systemPrompt string
		strategy := NewAgenticRemediation(WithPreparedPromptExecutor(
			func(
				_ context.Context,
				cfg *model.RuntimeConfig,
				_ string,
				_ *reusableagents.ExecutionContext,
				_ execpkg.SessionMCPBuilder,
			) (execpkg.PreparedPromptResult, error) {
				systemPrompt = cfg.SystemPrompt
				return execpkg.PreparedPromptResult{
					RunID:  "recovery-run",
					Output: `{"decision":"reject","reason":"not enough context","changed_files":[]}`,
				}, nil
			},
		))

		_, err := strategy.Remediate(context.Background(), RemediationInput{
			Outcome: RunOutcome{
				RunID:        "failed-run",
				Status:       StatusFailed,
				ArtifactsDir: t.TempDir(),
				Jobs: []JobOutcome{{
					SafeName: "build",
					Status:   StatusFailed,
					ExitCode: 2,
					ErrLog:   stderrLog,
					Error:    "build failed",
				}},
			},
			FailedConfig: &model.RuntimeConfig{
				WorkspaceRoot: workspaceRoot,
				Mode:          model.ExecutionModeExec,
				Name:          "scope-name",
				ReviewsDir:    ".compozy/tasks/reviews",
			},
		})
		if err != nil {
			t.Fatalf("Remediate() error = %v", err)
		}
		required := []string{
			"Identify the root cause before proposing a fix.",
			"Do not delete failing tests.",
			"Do not skip tests with `.skip`",
			"Do not add lint suppressions",
			"Failure context:",
			"failed_run_id: failed-run",
			"scope_name: scope-name",
			"scope_mode: exec",
			"exit_code: 2",
			"stderr_log:",
			"panic: broken invariant",
		}
		for _, snippet := range required {
			if !strings.Contains(systemPrompt, snippet) {
				t.Fatalf("expected system prompt to contain %q, got:\n%s", snippet, systemPrompt)
			}
		}
	})
}

func TestParseTriageVerdict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		output        string
		wantDecision  VerdictDecision
		wantChanged   []string
		wantReasonSub string
	}{
		{
			name:         "Should parse a fixed verdict JSON payload",
			output:       `{"decision":"fixed","reason":"updated parser","changed_files":["parser.go","parser.go"," tests/parser_test.go "]}`,
			wantDecision: VerdictFixed,
			wantChanged:  []string{"parser.go", "tests/parser_test.go"},
		},
		{
			name:         "Should parse a reject verdict from fenced JSON output",
			output:       "analysis\n```json\n{\"decision\":\"reject\",\"reason\":\"missing credentials\",\"changed_files\":[]}\n```",
			wantDecision: VerdictReject,
		},
		{
			name:          "Should fail safe on malformed JSON output",
			output:        `{"decision":"fixed",`,
			wantDecision:  VerdictReject,
			wantReasonSub: "did not contain",
		},
		{
			name:          "Should fail safe when JSON is missing entirely",
			output:        "I fixed it but forgot JSON",
			wantDecision:  VerdictReject,
			wantReasonSub: "did not contain",
		},
		{
			name:          "Should reject unknown verdict decisions",
			output:        `{"decision":"maybe","reason":"unsure","changed_files":["x.go"]}`,
			wantDecision:  VerdictReject,
			wantReasonSub: "unknown decision",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParseTriageVerdict(tt.output)
			if got.Decision != tt.wantDecision {
				t.Fatalf("Decision = %q, want %q; verdict=%#v", got.Decision, tt.wantDecision, got)
			}
			if tt.wantChanged != nil && !equalStrings(got.ChangedFiles, tt.wantChanged) {
				t.Fatalf("ChangedFiles = %#v, want %#v", got.ChangedFiles, tt.wantChanged)
			}
			if tt.wantReasonSub != "" && !strings.Contains(got.Reason, tt.wantReasonSub) {
				t.Fatalf("Reason = %q, want containing %q", got.Reason, tt.wantReasonSub)
			}
		})
	}
}

func TestAgenticRemediationIntegrationRunsDeterministicAgentAndWritesAudit(t *testing.T) {
	t.Run("Should run a deterministic agent and write recovery audit artifacts", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		t.Setenv("HOME", t.TempDir())
		installRuntimeProbeStub(t, "codex-acp")
		initGitRepo(t, workspaceRoot)

		runArtifacts := model.NewRunArtifactsForRunsDir(t.TempDir(), "failed-run")
		var capturedPrompt string
		restore := acpshared.SwapNewAgentClientForTest(
			func(_ context.Context, _ agent.ClientConfig) (agent.Client, error) {
				return &recoveryFakeACPClient{
					createSessionFn: func(_ context.Context, req agent.SessionRequest) (agent.Session, error) {
						capturedPrompt = string(req.Prompt)
						if err := os.WriteFile(
							filepath.Join(workspaceRoot, "fixed.txt"),
							[]byte("fixed\n"),
							0o600,
						); err != nil {
							t.Fatalf("write deterministic fix: %v", err)
						}
						session := newRecoveryFakeSession("sess-recovery")
						session.updates <- model.SessionUpdate{
							Kind:   model.UpdateKindAgentMessageChunk,
							Status: model.StatusRunning,
							Blocks: []model.ContentBlock{
								textContentBlock(t, `{"decision":"fixed","reason":"added missing file","changed_files":["fixed.txt"]}`),
							},
						}
						go session.finish(nil)
						return session, nil
					},
				}, nil
			},
		)
		t.Cleanup(restore)

		strategy := NewAgenticRemediation()
		verdict, err := strategy.Remediate(context.Background(), RemediationInput{
			Outcome: RunOutcome{
				RunID:        "failed-run",
				Status:       StatusFailed,
				ArtifactsDir: runArtifacts.RunDir,
				Jobs: []JobOutcome{{
					SafeName: "integration",
					Status:   StatusFailed,
					ExitCode: 1,
					Error:    "missing fixed.txt",
				}},
			},
			FailedConfig: &model.RuntimeConfig{
				WorkspaceRoot: workspaceRoot,
				IDE:           model.IDECodex,
				Model:         "gpt-5.5",
				AccessMode:    model.AccessModeDefault,
				Timeout:       time.Minute,
			},
		})
		if err != nil {
			t.Fatalf("Remediate() error = %v", err)
		}
		if verdict.Decision != VerdictFixed || !equalStrings(verdict.ChangedFiles, []string{"fixed.txt"}) {
			t.Fatalf("unexpected verdict: %#v", verdict)
		}
		if !strings.Contains(capturedPrompt, "Do not delete failing tests.") ||
			!strings.Contains(capturedPrompt, "missing fixed.txt") {
			t.Fatalf("expected ACP prompt to contain system prompt and failure context, got:\n%s", capturedPrompt)
		}
		for _, name := range []string{
			recoveryBaselineFileName,
			recoveryFinalFileName,
			recoveryChangedFilesFileName,
			recoveryMetadataFileName,
		} {
			if _, err := os.Stat(filepath.Join(runArtifacts.RecoveryDir, name)); err != nil {
				t.Fatalf("expected recovery artifact %s: %v", name, err)
			}
		}
		payload, err := os.ReadFile(filepath.Join(runArtifacts.RecoveryDir, recoveryChangedFilesFileName))
		if err != nil {
			t.Fatalf("read changed files audit: %v", err)
		}
		if !strings.Contains(string(payload), "fixed.txt") {
			t.Fatalf("expected changed files audit to contain fixed.txt, got:\n%s", string(payload))
		}
	})
}

func writeTestFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func strPtr(value string) *string {
	return &value
}

func equalStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func installRuntimeProbeStub(t *testing.T, command string) {
	t.Helper()
	binDir := t.TempDir()
	scriptPath := filepath.Join(binDir, command)
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write runtime probe stub: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "compozy@example.test")
	runGit(t, root, "config", "user.name", "Compozy Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# fixture\n"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "initial")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func textContentBlock(t *testing.T, text string) model.ContentBlock {
	t.Helper()
	payload, err := json.Marshal(model.TextBlock{
		Type: model.BlockText,
		Text: text,
	})
	if err != nil {
		t.Fatalf("marshal text content block: %v", err)
	}
	return model.ContentBlock{
		Type: model.BlockText,
		Data: payload,
	}
}

type recoveryFakeACPClient struct {
	createSessionFn func(context.Context, agent.SessionRequest) (agent.Session, error)
}

func (c *recoveryFakeACPClient) CreateSession(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
	if c.createSessionFn == nil {
		return nil, errors.New("missing CreateSession fake")
	}
	return c.createSessionFn(ctx, req)
}

func (*recoveryFakeACPClient) ResumeSession(context.Context, agent.ResumeSessionRequest) (agent.Session, error) {
	return nil, errors.New("resume not supported in recovery fake")
}

func (*recoveryFakeACPClient) CancelSession(context.Context, string) error {
	return nil
}

func (*recoveryFakeACPClient) SetSessionModel(context.Context, string, string) error {
	return nil
}

func (*recoveryFakeACPClient) PromptSession(context.Context, agent.PromptSessionRequest) (agent.Session, error) {
	return nil, errors.New("prompt not supported in recovery fake")
}

func (*recoveryFakeACPClient) SupportsLoadSession() bool {
	return false
}

func (*recoveryFakeACPClient) Close() error {
	return nil
}

func (*recoveryFakeACPClient) Kill() error {
	return nil
}

type recoveryFakeSession struct {
	id      string
	updates chan model.SessionUpdate
	done    chan struct{}

	mu       sync.RWMutex
	err      error
	finished bool
}

func newRecoveryFakeSession(id string) *recoveryFakeSession {
	return &recoveryFakeSession{
		id:      id,
		updates: make(chan model.SessionUpdate, 1),
		done:    make(chan struct{}),
	}
}

func (s *recoveryFakeSession) ID() string {
	return s.id
}

func (s *recoveryFakeSession) Identity() agent.SessionIdentity {
	return agent.SessionIdentity{ACPSessionID: s.id}
}

func (s *recoveryFakeSession) Updates() <-chan model.SessionUpdate {
	return s.updates
}

func (s *recoveryFakeSession) Done() <-chan struct{} {
	return s.done
}

func (s *recoveryFakeSession) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

func (*recoveryFakeSession) SlowPublishes() uint64 {
	return 0
}

func (*recoveryFakeSession) DroppedUpdates() uint64 {
	return 0
}

func (s *recoveryFakeSession) finish(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.finished = true
	s.err = err
	close(s.updates)
	close(s.done)
}
