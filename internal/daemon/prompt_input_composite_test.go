package daemon

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/session"
)

func TestPromptInputCompositeOrdersEnabledAugmentersByDescriptorOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should order enabled augmenters by descriptor order", func(t *testing.T) {
		t.Parallel()

		resolver := &staticPromptInputAugmenterResolver{
			resolved: ResolvedHarnessContext{
				Policy: ResolvedHarnessPolicy{
					EnableAugmenters: []HarnessAugmenter{"later", "earlier"},
				},
			},
		}

		callOrder := make([]string, 0, 2)
		augmenter, err := newPromptInputCompositeAugmenter(
			discardLogger(),
			resolver,
			nil,
			promptInputAugmenterDescriptor{
				Name:   "later",
				Order:  200,
				Budget: 32,
				Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
					callOrder = append(callOrder, "later")
					return message + "\nSECOND", nil
				},
			},
			promptInputAugmenterDescriptor{
				Name:   "earlier",
				Order:  100,
				Budget: 32,
				Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
					callOrder = append(callOrder, "earlier")
					return message + "\nFIRST", nil
				},
			},
		)
		if err != nil {
			t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
		}

		got, err := augmenter(context.Background(), newPromptInputTestSession(""), "base")
		if err != nil {
			t.Fatalf("Augment() error = %v", err)
		}
		if got != "base\nFIRST\nSECOND" {
			t.Fatalf("Augment() = %q, want ordered output", got)
		}
		if got, want := strings.Join(callOrder, ","), "earlier,later"; got != want {
			t.Fatalf("call order = %q, want %q", got, want)
		}
	})
}

func TestNormalizePromptInputAugmenterDescriptorsValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		descriptors []promptInputAugmenterDescriptor
		wantErr     string
	}{
		{
			name: "missing name",
			descriptors: []promptInputAugmenterDescriptor{{
				Budget:    1,
				Augmenter: func(context.Context, *session.Session, string) (string, error) { return "", nil },
			}},
			wantErr: "name is required",
		},
		{
			name: "duplicate names",
			descriptors: []promptInputAugmenterDescriptor{
				{
					Name:      "dup",
					Budget:    1,
					Augmenter: func(context.Context, *session.Session, string) (string, error) { return "", nil },
				},
				{
					Name:      "dup",
					Budget:    1,
					Augmenter: func(context.Context, *session.Session, string) (string, error) { return "", nil },
				},
			},
			wantErr: `duplicate prompt input augmenter descriptor "dup"`,
		},
		{
			name: "missing augmenter",
			descriptors: []promptInputAugmenterDescriptor{{
				Name:   "missing",
				Budget: 1,
			}},
			wantErr: `prompt input augmenter "missing" is missing an augmenter`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizePromptInputAugmenterDescriptors(tt.descriptors)
			if err == nil {
				t.Fatal("normalizePromptInputAugmenterDescriptors() error = nil, want validation failure")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("normalizePromptInputAugmenterDescriptors() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizePromptInputAugmenterBudgetBehaviorDefaultsToTrim(t *testing.T) {
	t.Parallel()

	t.Run("Should default unknown budget behavior to trim", func(t *testing.T) {
		t.Parallel()

		if got := normalizePromptInputAugmenterBudgetBehavior(""); got != promptInputAugmenterBudgetBehaviorTrim {
			t.Fatalf("normalizePromptInputAugmenterBudgetBehavior(empty) = %q, want trim", got)
		}
		if got := normalizePromptInputAugmenterBudgetBehavior(
			"unknown",
		); got != promptInputAugmenterBudgetBehaviorTrim {
			t.Fatalf("normalizePromptInputAugmenterBudgetBehavior(unknown) = %q, want trim", got)
		}
		if got := normalizePromptInputAugmenterBudgetBehavior(
			promptInputAugmenterBudgetBehaviorOmit,
		); got != promptInputAugmenterBudgetBehaviorOmit {
			t.Fatalf("normalizePromptInputAugmenterBudgetBehavior(omit) = %q, want omit", got)
		}
	})
}

func TestPromptInputCompositeAppliesAggregateBudgetPolicies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		suffixBehavior promptInputAugmenterBudgetBehavior
		want           string
	}{
		{
			name:           "trim later output to remaining aggregate budget",
			suffixBehavior: promptInputAugmenterBudgetBehaviorTrim,
			want:           "AAbaseB",
		},
		{
			name:           "omit later output when aggregate budget is exhausted",
			suffixBehavior: promptInputAugmenterBudgetBehaviorOmit,
			want:           "AAbase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolver := &staticPromptInputAugmenterResolver{
				resolved: ResolvedHarnessContext{
					Policy: ResolvedHarnessPolicy{
						EnableAugmenters: []HarnessAugmenter{"prefix", "suffix"},
					},
				},
			}

			augmenter, err := newPromptInputCompositeAugmenter(
				discardLogger(),
				resolver,
				nil,
				promptInputAugmenterDescriptor{
					Name:   "prefix",
					Order:  100,
					Budget: 2,
					Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
						return "AA" + message, nil
					},
				},
				promptInputAugmenterDescriptor{
					Name:           "suffix",
					Order:          200,
					Budget:         1,
					BudgetBehavior: tt.suffixBehavior,
					Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
						return message + "BBBB", nil
					},
				},
			)
			if err != nil {
				t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
			}

			got, err := augmenter(context.Background(), newPromptInputTestSession(""), "base")
			if err != nil {
				t.Fatalf("Augment() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Augment() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPromptInputCompositePreservesCurrentMessageForOverBudgetRewrite(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve current message for over-budget rewrite", func(t *testing.T) {
		t.Parallel()

		resolver := &staticPromptInputAugmenterResolver{
			resolved: ResolvedHarnessContext{
				Policy: ResolvedHarnessPolicy{
					EnableAugmenters: []HarnessAugmenter{"rewrite"},
				},
			},
		}

		augmenter, err := newPromptInputCompositeAugmenter(
			discardLogger(),
			resolver,
			nil,
			promptInputAugmenterDescriptor{
				Name:           "rewrite",
				Order:          100,
				Budget:         1,
				BudgetBehavior: promptInputAugmenterBudgetBehaviorTrim,
				Augmenter: func(_ context.Context, _ *session.Session, _ string) (string, error) {
					return "rewritten message", nil
				},
			},
		)
		if err != nil {
			t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
		}

		got, err := augmenter(context.Background(), newPromptInputTestSession(""), "base")
		if err != nil {
			t.Fatalf("Augment() error = %v", err)
		}
		if got != "base" {
			t.Fatalf("Augment() = %q, want original message preserved for over-budget rewrite", got)
		}
	})
}

func TestPromptInputCompositeLoggerForSessionFallsBackSafely(t *testing.T) {
	t.Parallel()

	t.Run("Should fall back safely when session logger is unavailable", func(t *testing.T) {
		t.Parallel()

		composite := &promptInputComposite{}
		if got := composite.loggerForSession(nil); got == nil {
			t.Fatal("loggerForSession(nil) = nil, want default logger")
		}
		if got := composite.loggerForSession(&session.Session{}); got == nil {
			t.Fatal("loggerForSession(session without info) = nil, want default logger")
		}
	})
}

func TestPromptInputCompositeCriticalFailureStopsPipeline(t *testing.T) {
	t.Parallel()

	t.Run("Should stop pipeline on critical failure", func(t *testing.T) {
		t.Parallel()

		resolver := &staticPromptInputAugmenterResolver{
			resolved: ResolvedHarnessContext{
				Policy: ResolvedHarnessPolicy{
					EnableAugmenters: []HarnessAugmenter{"prefix", "boom", "after"},
				},
			},
		}

		afterCalls := 0
		augmenter, err := newPromptInputCompositeAugmenter(
			discardLogger(),
			resolver,
			nil,
			promptInputAugmenterDescriptor{
				Name:   "prefix",
				Order:  100,
				Budget: 32,
				Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
					return "AA" + message, nil
				},
			},
			promptInputAugmenterDescriptor{
				Name:     "boom",
				Order:    200,
				Critical: true,
				Budget:   32,
				Augmenter: func(_ context.Context, _ *session.Session, _ string) (string, error) {
					return "", errors.New("boom")
				},
			},
			promptInputAugmenterDescriptor{
				Name:   "after",
				Order:  300,
				Budget: 32,
				Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
					afterCalls++
					return message + " after", nil
				},
			},
		)
		if err != nil {
			t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
		}

		if _, err := augmenter(context.Background(), newPromptInputTestSession(""), "base"); err == nil {
			t.Fatal("Augment() error = nil, want critical failure")
		} else if !strings.Contains(err.Error(), `prompt augmenter "boom"`) {
			t.Fatalf("Augment() error = %v, want wrapped augmenter name", err)
		}
		if afterCalls != 0 {
			t.Fatalf("afterCalls = %d, want 0 after critical failure", afterCalls)
		}
	})
}

func TestPromptInputCompositeContextCancellationStopsPipeline(t *testing.T) {
	t.Parallel()

	t.Run("Should stop pipeline on context cancellation", func(t *testing.T) {
		t.Parallel()

		resolver := &staticPromptInputAugmenterResolver{
			resolved: ResolvedHarnessContext{
				Policy: ResolvedHarnessPolicy{
					EnableAugmenters: []HarnessAugmenter{"warn"},
				},
			},
		}

		augmenter, err := newPromptInputCompositeAugmenter(
			discardLogger(),
			resolver,
			nil,
			promptInputAugmenterDescriptor{
				Name:   "warn",
				Order:  100,
				Budget: 32,
				Augmenter: func(_ context.Context, _ *session.Session, _ string) (string, error) {
					return "", context.Canceled
				},
			},
		)
		if err != nil {
			t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
		}

		if _, err := augmenter(
			context.Background(),
			newPromptInputTestSession(""),
			"base",
		); !errors.Is(
			err,
			context.Canceled,
		) {
			t.Fatalf("Augment() error = %v, want context.Canceled", err)
		}
	})
}

func TestPromptInputCompositeNoncriticalFailureWarnsAndContinues(t *testing.T) {
	t.Parallel()

	t.Run("Should warn and continue after noncritical failure", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
		resolver := &staticPromptInputAugmenterResolver{
			resolved: ResolvedHarnessContext{
				Policy: ResolvedHarnessPolicy{
					EnableAugmenters: []HarnessAugmenter{"warn", "suffix"},
				},
			},
		}

		augmenter, err := newPromptInputCompositeAugmenter(
			logger,
			resolver,
			nil,
			promptInputAugmenterDescriptor{
				Name:   "warn",
				Order:  100,
				Budget: 32,
				Augmenter: func(_ context.Context, _ *session.Session, _ string) (string, error) {
					return "", errors.New("temporary failure")
				},
			},
			promptInputAugmenterDescriptor{
				Name:   "suffix",
				Order:  200,
				Budget: 32,
				Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
					return message + " ok", nil
				},
			},
		)
		if err != nil {
			t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
		}

		got, err := augmenter(context.Background(), newPromptInputTestSession(""), "base")
		if err != nil {
			t.Fatalf("Augment() error = %v, want warning-only continuation", err)
		}
		if got != "base ok" {
			t.Fatalf("Augment() = %q, want suffix to run after warning", got)
		}
		if !strings.Contains(logs.String(), "noncritical prompt augmenter failed") {
			t.Fatalf("logs = %q, want noncritical warning message", logs.String())
		}
		if !strings.Contains(logs.String(), "augmenter=warn") {
			t.Fatalf("logs = %q, want augmenter name", logs.String())
		}
	})
}

func TestPromptInputCompositeBlankOutputPreservesLastValidMessage(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve last valid message for blank output", func(t *testing.T) {
		t.Parallel()

		resolver := &staticPromptInputAugmenterResolver{
			resolved: ResolvedHarnessContext{
				Policy: ResolvedHarnessPolicy{
					EnableAugmenters: []HarnessAugmenter{"blank", "suffix"},
				},
			},
		}

		augmenter, err := newPromptInputCompositeAugmenter(
			discardLogger(),
			resolver,
			nil,
			promptInputAugmenterDescriptor{
				Name:   "blank",
				Order:  100,
				Budget: 32,
				Augmenter: func(_ context.Context, _ *session.Session, _ string) (string, error) {
					return "   \n\t", nil
				},
			},
			promptInputAugmenterDescriptor{
				Name:   "suffix",
				Order:  200,
				Budget: 32,
				Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
					return message + "\nkept", nil
				},
			},
		)
		if err != nil {
			t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
		}

		got, err := augmenter(context.Background(), newPromptInputTestSession(""), "base")
		if err != nil {
			t.Fatalf("Augment() error = %v", err)
		}
		if got != "base\nkept" {
			t.Fatalf("Augment() = %q, want blank augmenter to leave prior message intact", got)
		}
	})
}

func TestPromptInputCompositeIncludesDurableMemoryRecall(t *testing.T) {
	t.Parallel()

	t.Run("Should include durable memory recall", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		globalDir := filepath.Join(baseDir, "global")
		workspaceRoot := filepath.Join(baseDir, "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", workspaceRoot, err)
		}

		store := memory.NewStore(
			globalDir,
			memory.WithCatalogDatabasePath(filepath.Join(baseDir, "catalog.db")),
		)
		openDaemonMemoryCatalog(t, store)
		workspaceStore := store.ForWorkspace(workspaceRoot)
		if err := workspaceStore.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		if err := workspaceStore.Write(memcontract.ScopeWorkspace, "auth.md", []byte(`---
name: Auth
description: Auth migration notes
type: project
---
Remember auth migration sessions and workspace-scoped handling.
`)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}

		resolver := &staticPromptInputAugmenterResolver{
			resolved: ResolvedHarnessContext{
				Policy: ResolvedHarnessPolicy{
					EnableAugmenters: []HarnessAugmenter{HarnessAugmenterDurableMemory},
				},
			},
		}
		augmenter, err := newPromptInputCompositeAugmenter(
			discardLogger(),
			resolver,
			nil,
			defaultPromptInputAugmenterDescriptors(memory.NewRecallAugmenter(store), nil)...,
		)
		if err != nil {
			t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
		}

		got, err := augmenter(
			context.Background(),
			newPromptInputTestSession(workspaceRoot),
			"auth migration sessions",
		)
		if err != nil {
			t.Fatalf("Augment() error = %v", err)
		}
		if !strings.Contains(got, "Relevant durable memory for this turn:") {
			t.Fatalf("Augment() = %q, want durable memory recall block", got)
		}
		if !strings.Contains(got, "Auth") {
			t.Fatalf("Augment() = %q, want recalled memory metadata", got)
		}
		if !strings.Contains(got, "</turn-recall>\n\n<user-message>\nauth migration sessions\n</user-message>") {
			t.Fatalf("Augment() = %q, want preserved user message suffix", got)
		}
		if strings.Contains(got, "User message:") {
			t.Fatalf("Augment() = %q, want no legacy user message marker", got)
		}
	})
}

func TestPromptInputCompositeOmitsOverBudgetDurableMemoryRecall(t *testing.T) {
	t.Parallel()

	t.Run("Should omit over-budget durable memory recall", func(t *testing.T) {
		t.Parallel()

		resolver := &staticPromptInputAugmenterResolver{
			resolved: ResolvedHarnessContext{
				Policy: ResolvedHarnessPolicy{
					EnableAugmenters: []HarnessAugmenter{HarnessAugmenterDurableMemory},
				},
			},
		}
		oversizedRecall := func(_ context.Context, _ *session.Session, message string) (string, error) {
			return strings.Join([]string{
				"<turn-recall>",
				strings.Repeat("x", memory.RecallAugmenterBudget+32),
				"</turn-recall>",
				"",
				"<user-message>",
				message,
				"</user-message>",
			}, "\n"), nil
		}
		augmenter, err := newPromptInputCompositeAugmenter(
			discardLogger(),
			resolver,
			nil,
			defaultPromptInputAugmenterDescriptors(oversizedRecall, nil)...,
		)
		if err != nil {
			t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
		}

		got, err := augmenter(context.Background(), newPromptInputTestSession(""), "hello")
		if err != nil {
			t.Fatalf("Augment() error = %v", err)
		}
		if got != "hello" {
			t.Fatalf("Augment() = %q, want original message after over-budget recall omission", got)
		}
		if strings.Contains(got, "<turn-recall>") || strings.Contains(got, "<user-message>") {
			t.Fatalf("Augment() = %q, want no partially emitted recall wrappers", got)
		}
	})
}

func TestPromptInputContributionRunes(t *testing.T) {
	t.Parallel()

	t.Run("Should count prompt input contribution runes", func(t *testing.T) {
		t.Parallel()

		if got := promptInputContributionRunes("base", "AAbaseZZ"); got != 4 {
			t.Fatalf("promptInputContributionRunes(wrapped) = %d, want 4", got)
		}
		if got := promptInputContributionRunes("base", "rewritten"); got != len("rewritten")-len("base") {
			t.Fatalf("promptInputContributionRunes(rewrite) = %d, want positive rewrite delta", got)
		}
		if got := promptInputContributionRunes("base", "x"); got != 0 {
			t.Fatalf("promptInputContributionRunes(shorter rewrite) = %d, want 0", got)
		}
	})
}

type staticPromptInputAugmenterResolver struct {
	resolved ResolvedHarnessContext
	err      error
}

func (r *staticPromptInputAugmenterResolver) ResolvePrompt(
	_ *session.Info,
	_ session.TurnSource,
	_ acp.PromptMeta,
) (ResolvedHarnessContext, error) {
	if r == nil {
		return ResolvedHarnessContext{}, nil
	}
	return r.resolved, r.err
}

func newPromptInputTestSession(workspaceRoot string) *session.Session {
	return &session.Session{
		ID:        "sess-1",
		AgentName: "coder",
		Type:      session.SessionTypeUser,
		Workspace: workspaceRoot,
	}
}
