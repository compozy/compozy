package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	memoryrecall "github.com/compozy/agh/internal/memory/recall"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestServiceConstructionDefaults(t *testing.T) {
	t.Parallel()

	service := NewService()

	if service.minHours != defaultMinHours {
		t.Fatalf("minHours = %v, want %v", service.minHours, defaultMinHours)
	}
	if service.minSessions != defaultMinSessions {
		t.Fatalf("minSessions = %d, want %d", service.minSessions, defaultMinSessions)
	}
	if service.logger == nil {
		t.Fatal("logger = nil, want non-nil")
	}
	if service.goal != defaultGoal {
		t.Fatalf("goal = %q, want %q", service.goal, defaultGoal)
	}
	if service.prompt != ConsolidationPrompt() {
		t.Fatal("prompt does not match embedded consolidation prompt")
	}
	if service.lock == nil {
		t.Fatal("lock = nil, want non-nil")
	}
}

func TestServiceConstructionOverridesDefaults(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := newOpenTestStore(t, filepath.Join(t.TempDir(), "memory"))

	service := NewService(
		WithMemoryStore(store),
		WithSessionsDir("/tmp/sessions"),
		WithLockPath("/tmp/lock"),
		WithMinHours(12),
		WithMinSessions(5),
		WithLogger(logger),
		withGoal("custom-goal"),
	)

	if service.memStore != store {
		t.Fatal("memStore was not applied")
	}
	if service.sessionsDir != "/tmp/sessions" {
		t.Fatalf("sessionsDir = %q, want /tmp/sessions", service.sessionsDir)
	}
	if service.lockPath != "/tmp/lock" {
		t.Fatalf("lockPath = %q, want /tmp/lock", service.lockPath)
	}
	if service.minHours != 12 {
		t.Fatalf("minHours = %v, want 12", service.minHours)
	}
	if service.minSessions != 5 {
		t.Fatalf("minSessions = %d, want 5", service.minSessions)
	}
	if service.logger != logger {
		t.Fatal("logger override was not applied")
	}
	if service.goal != "custom-goal" {
		t.Fatalf("goal = %q, want custom-goal", service.goal)
	}
}

func TestServiceShouldRunTimeGateFails(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Round(0)
	lock := &stubLock{lastConsolidatedAt: now.Add(-time.Hour)}
	sessionsScanned := 0
	service := NewService(
		withLock(lock),
		WithMinHours(24),
		WithMinSessions(3),
		withNow(func() time.Time { return now }),
		withSessionCounter(func(time.Time) (int, error) {
			sessionsScanned++
			return 10, nil
		}),
	)

	ok, err := service.ShouldRun()
	if err != nil {
		t.Fatalf("ShouldRun() error = %v", err)
	}
	if ok {
		t.Fatal("ShouldRun() = true, want false")
	}
	if sessionsScanned != 0 {
		t.Fatalf("session scans = %d, want 0", sessionsScanned)
	}
	if lock.tryAcquireCalls != 0 {
		t.Fatalf("lock acquisitions = %d, want 0", lock.tryAcquireCalls)
	}
}

func TestServiceShouldRunSessionGateFails(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Round(0)
	lock := &stubLock{lastConsolidatedAt: now.Add(-48 * time.Hour)}
	service := NewService(
		withLock(lock),
		WithMinHours(24),
		WithMinSessions(3),
		withNow(func() time.Time { return now }),
		withSessionCounter(func(time.Time) (int, error) {
			return 2, nil
		}),
	)

	ok, err := service.ShouldRun()
	if err != nil {
		t.Fatalf("ShouldRun() error = %v", err)
	}
	if ok {
		t.Fatal("ShouldRun() = true, want false")
	}
	if lock.tryAcquireCalls != 0 {
		t.Fatalf("lock acquisitions = %d, want 0", lock.tryAcquireCalls)
	}
}

func TestServiceShouldRunAllGatesPassWithoutLockSideEffects(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Round(0)
	lock := &stubLock{lastConsolidatedAt: now.Add(-48 * time.Hour)}
	service := NewService(
		withLock(lock),
		WithMinHours(24),
		WithMinSessions(3),
		withNow(func() time.Time { return now }),
		withSessionCounter(func(time.Time) (int, error) {
			return 3, nil
		}),
	)

	ok, err := service.ShouldRun()
	if err != nil {
		t.Fatalf("ShouldRun() error = %v", err)
	}
	if !ok {
		t.Fatal("ShouldRun() = false, want true")
	}
	if lock.tryAcquireCalls != 0 {
		t.Fatalf("lock acquisitions = %d, want 0", lock.tryAcquireCalls)
	}
	if service.pending {
		t.Fatal("service.pending = true, want false")
	}
	if !service.priorMtime.IsZero() {
		t.Fatalf("service.priorMtime = %v, want zero", service.priorMtime)
	}
}

func TestServiceShouldRunEvaluatesGatesInOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should time gate short-circuits session and lock", func(t *testing.T) {
		now := time.Now().UTC().Round(0)
		lock := &stubLock{lastConsolidatedAt: now.Add(-time.Hour)}
		sequence := make([]string, 0, 2)
		service := NewService(
			withLock(lock),
			WithMinHours(24),
			WithMinSessions(3),
			withNow(func() time.Time { return now }),
			withSessionCounter(func(time.Time) (int, error) {
				sequence = append(sequence, "sessions")
				return 10, nil
			}),
		)

		ok, err := service.ShouldRun()
		if err != nil {
			t.Fatalf("ShouldRun() error = %v", err)
		}
		if ok {
			t.Fatal("ShouldRun() = true, want false")
		}
		if got := strings.Join(sequence, ","); got != "" {
			t.Fatalf("sequence = %q, want empty", got)
		}
		if lock.tryAcquireCalls != 0 {
			t.Fatalf("lock acquisitions = %d, want 0", lock.tryAcquireCalls)
		}
	})

	t.Run("Should session gate runs after time gate without touching lock", func(t *testing.T) {
		now := time.Now().UTC().Round(0)
		sequence := make([]string, 0, 1)
		lock := &stubLock{
			lastConsolidatedAt: now.Add(-48 * time.Hour),
		}
		service := NewService(
			withLock(lock),
			WithMinHours(24),
			WithMinSessions(1),
			withNow(func() time.Time { return now }),
			withSessionCounter(func(time.Time) (int, error) {
				sequence = append(sequence, "sessions")
				return 1, nil
			}),
		)

		ok, err := service.ShouldRun()
		if err != nil {
			t.Fatalf("ShouldRun() error = %v", err)
		}
		if !ok {
			t.Fatal("ShouldRun() = false, want true")
		}
		if got := strings.Join(sequence, ","); got != "sessions" {
			t.Fatalf("sequence = %q, want sessions", got)
		}
		if lock.tryAcquireCalls != 0 {
			t.Fatalf("lock acquisitions = %d, want 0", lock.tryAcquireCalls)
		}
	})
}

func TestServiceRunCallsSessionSpawnerWithGoalPromptAndWorkspaceID(t *testing.T) {
	t.Parallel()

	prior := time.Now().UTC().Add(-24 * time.Hour).Round(0)
	lock := &stubLock{
		tryAcquireFn: func() (time.Time, bool, error) {
			return prior, true, nil
		},
	}
	globalMemoryDir := filepath.Join(t.TempDir(), "memory")
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	workspaceID := "ws-dream"
	service := NewService(
		withLock(lock),
		withGoal("custom-goal"),
		WithMemoryStore(newOpenTestStore(t, globalMemoryDir)),
		WithWorkspaceResolver(&fakeDreamWorkspaceResolver{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      workspaceID,
					RootDir: workspaceRoot,
				},
			},
		}),
	)

	var gotGoal string
	var gotPrompt string
	var gotWorkspace string
	var gotCutoff time.Time
	err := service.Run(testutil.Context(t), func(
		_ context.Context,
		goal string,
		prompt string,
		workspace string,
		lastConsolidatedAt time.Time,
	) error {
		gotGoal = goal
		gotPrompt = prompt
		gotWorkspace = workspace
		gotCutoff = lastConsolidatedAt
		return nil
	}, workspaceID)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotGoal != "custom-goal" {
		t.Fatalf("goal = %q, want custom-goal", gotGoal)
	}
	if gotPrompt != ConsolidationPrompt() {
		t.Fatal("prompt passed to session spawner did not match embedded prompt")
	}
	if gotWorkspace != workspaceID {
		t.Fatalf("workspace = %q, want %q", gotWorkspace, workspaceID)
	}
	if !gotCutoff.Equal(prior) {
		t.Fatalf("last consolidated at = %v, want %v", gotCutoff, prior)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, ".agh", "memory")); err != nil {
		t.Fatalf("workspace memory dir stat error = %v", err)
	}
	if lock.tryAcquireCalls != 1 {
		t.Fatalf("lock acquisitions = %d, want 1", lock.tryAcquireCalls)
	}
	if lock.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", lock.releaseCalls)
	}
}

func TestServiceRunDreamSignalGateBlocksWhenNoUnpromotedSignals(t *testing.T) {
	t.Parallel()

	t.Run("Should stamp anti-thrash lock and skip spawn when signal gate is empty", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		env := newDreamSeedEnv(t, now)
		lock := &stubLock{
			releaseAt: now,
			tryAcquireFn: func() (time.Time, bool, error) {
				return now.Add(-48 * time.Hour), true, nil
			},
		}
		service := NewService(
			WithMemoryStore(env.baseStore),
			WithMinHours(24),
			WithMinSessions(0),
			WithDreamGateConfig(DreamGateConfig{MinCandidates: 1, MinRecallCount: 2, MinScore: 0.75}),
			withLock(lock),
			withNow(func() time.Time { return now }),
		)

		spawnCalls := 0
		err := service.Run(testutil.Context(t), func(context.Context, string, string, string, time.Time) error {
			spawnCalls++
			return nil
		}, "")

		if !errors.Is(err, ErrDreamGateNotSatisfied) {
			t.Fatalf("Run() error = %v, want ErrDreamGateNotSatisfied", err)
		}
		if spawnCalls != 0 {
			t.Fatalf("spawn calls = %d, want 0", spawnCalls)
		}
		if lock.releaseCalls != 1 {
			t.Fatalf("release calls = %d, want 1 anti-thrash stamp", lock.releaseCalls)
		}
		if len(lock.rollbackCalls) != 0 {
			t.Fatalf("rollback calls = %d, want 0", len(lock.rollbackCalls))
		}

		shouldRun, err := service.ShouldRun()
		if err != nil {
			t.Fatalf("ShouldRun(during cooldown) error = %v", err)
		}
		if shouldRun {
			t.Fatal("ShouldRun(during cooldown) = true, want false after sub-threshold run")
		}
		now = now.Add(24 * time.Hour)
		shouldRun, err = service.ShouldRun()
		if err != nil {
			t.Fatalf("ShouldRun(after cooldown) error = %v", err)
		}
		if !shouldRun {
			t.Fatal("ShouldRun(after cooldown) = false, want true")
		}
	})
}

func TestServiceRunDreamSignalGatePromotesEligibleSignals(t *testing.T) {
	t.Parallel()

	t.Run("Should create system artifact, curated promotion, and idempotent signal marks", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		env := newDreamSeedEnv(t, now)
		seedDreamRecallSignals(t, env.workspaceStore, memcontract.ScopeWorkspace, env.workspaceID, 5, now)
		lock := &stubLock{
			tryAcquireFn: func() (time.Time, bool, error) {
				return now.Add(-48 * time.Hour), true, nil
			},
		}
		service := NewService(
			WithMemoryStore(env.baseStore),
			WithWorkspaceResolver(&fakeDreamWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      env.workspaceID,
						RootDir: env.workspaceRoot,
					},
				},
			}),
			WithMinHours(0),
			WithMinSessions(0),
			WithDreamGateConfig(DreamGateConfig{MinCandidates: 5, MinRecallCount: 2, MinScore: 0.75}),
			withLock(lock),
			withNow(func() time.Time { return now }),
		)

		gotWorkspace := ""
		err := service.Run(testutil.Context(t), func(_ context.Context, _, _, workspace string, _ time.Time) error {
			gotWorkspace = workspace
			return nil
		}, env.workspaceID)

		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if gotWorkspace != env.workspaceID {
			t.Fatalf("spawn workspace = %q, want %q", gotWorkspace, env.workspaceID)
		}
		assertDreamPromotedCount(t, env.workspaceStore, 5)
		assertDreamConsolidationStatus(t, env.workspaceStore, "completed", 5)
		assertDreamEventCount(t, env.workspaceStore, memoryEventDreamPromoted, 1)
		assertFileExists(
			t,
			filepath.Join(env.workspaceRoot, ".agh", "memory", "_system", "dreaming", "20260505-dreaming-curator.md"),
		)
		assertFileExists(t, filepath.Join(env.workspaceRoot, ".agh", "memory", "project_dreaming_20260505.md"))
		if lock.releaseCalls != 1 {
			t.Fatalf("release calls = %d, want 1", lock.releaseCalls)
		}
		if len(lock.rollbackCalls) != 0 {
			t.Fatalf("rollback calls = %d, want 0", len(lock.rollbackCalls))
		}
	})
}

func TestServiceRunSkipsWorkspaceDisabledDreamWithoutPromotion(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back the provisional run without promotion", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		env := newDreamSeedEnv(t, now)
		seedDreamRecallSignals(t, env.workspaceStore, memcontract.ScopeWorkspace, env.workspaceID, 5, now)
		prior := now.Add(-48 * time.Hour)
		lock := &stubLock{
			tryAcquireFn: func() (time.Time, bool, error) {
				return prior, true, nil
			},
		}
		service := NewService(
			WithMemoryStore(env.baseStore),
			WithWorkspaceResolver(&fakeDreamWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: env.workspaceID, RootDir: env.workspaceRoot},
				},
			}),
			WithMinHours(0),
			WithMinSessions(0),
			WithDreamGateConfig(DreamGateConfig{MinCandidates: 5, MinRecallCount: 2, MinScore: 0.75}),
			withLock(lock),
			withNow(func() time.Time { return now }),
		)

		err := service.Run(testutil.Context(t), func(context.Context, string, string, string, time.Time) error {
			return ErrDreamRoleDisabled
		}, env.workspaceID)
		if !errors.Is(err, ErrDreamRoleDisabled) {
			t.Fatalf("Run() error = %v, want ErrDreamRoleDisabled", err)
		}
		assertDreamPromotedCount(t, env.workspaceStore, 0)
		assertDreamConsolidationCount(t, env.workspaceStore, 0)
		assertDreamEventCount(t, env.workspaceStore, memoryEventDreamStarted, 0)
		if lock.releaseCalls != 0 || len(lock.rollbackCalls) != 1 || !lock.rollbackCalls[0].Equal(prior) {
			t.Fatalf("lock release/rollback = %d/%v, want 0/[prior]", lock.releaseCalls, lock.rollbackCalls)
		}
	})

	t.Run("Should surface rollback failure instead of reporting a clean skip", func(t *testing.T) {
		t.Parallel()

		rollbackErr := errors.New("rollback disabled dream run")
		service := NewService(withLock(&stubLock{rollbackErr: rollbackErr}))
		err := service.Run(testutil.Context(t), func(context.Context, string, string, string, time.Time) error {
			return ErrDreamRoleDisabled
		}, "")
		if !errors.Is(err, rollbackErr) {
			t.Fatalf("Run() error = %v, want rollback failure", err)
		}
		if errors.Is(err, ErrDreamRoleDisabled) {
			t.Fatalf("Run() error = %v, must not suppress cleanup failure as disabled-role skip", err)
		}
	})
}

func TestServiceRunDreamSignalGateUsesStableWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should promote workspace signals when registration id differs from stable identity", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		env := newDreamSeedEnv(t, now)
		seedDreamRecallSignals(t, env.workspaceStore, memcontract.ScopeWorkspace, env.workspaceID, 5, now)
		registrationID := "ws-registration"
		lock := &stubLock{
			tryAcquireFn: func() (time.Time, bool, error) {
				return now.Add(-48 * time.Hour), true, nil
			},
		}
		service := NewService(
			WithMemoryStore(env.baseStore),
			WithWorkspaceResolver(&fakeDreamWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      registrationID,
						RootDir: env.workspaceRoot,
					},
					WorkspaceID: env.workspaceID,
				},
			}),
			WithMinHours(0),
			WithMinSessions(0),
			WithDreamGateConfig(DreamGateConfig{MinCandidates: 5, MinRecallCount: 2, MinScore: 0.75}),
			withLock(lock),
			withNow(func() time.Time { return now }),
		)

		gotWorkspace := ""
		err := service.Run(testutil.Context(t), func(_ context.Context, _, _, workspace string, _ time.Time) error {
			gotWorkspace = workspace
			return nil
		}, registrationID)

		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if gotWorkspace != env.workspaceID {
			t.Fatalf("spawn workspace = %q, want stable identity %q", gotWorkspace, env.workspaceID)
		}
		assertDreamPromotedCount(t, env.workspaceStore, 5)
		assertDreamConsolidationStatus(t, env.workspaceStore, "completed", 5)
		assertDreamEventCount(t, env.workspaceStore, memoryEventDreamPromoted, 1)
	})
}

func TestServiceRunDreamFailureWritesDLQAndDoesNotMarkPromoted(t *testing.T) {
	t.Parallel()

	t.Run("Should write _system failure and leave recall signals unpromoted", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		env := newDreamSeedEnv(t, now)
		seedDreamRecallSignals(t, env.workspaceStore, memcontract.ScopeWorkspace, env.workspaceID, 5, now)
		spawnErr := errors.New("dreaming curator failed")
		lock := &stubLock{
			tryAcquireFn: func() (time.Time, bool, error) {
				return now.Add(-48 * time.Hour), true, nil
			},
		}
		service := NewService(
			WithMemoryStore(env.baseStore),
			WithWorkspaceResolver(&fakeDreamWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      env.workspaceID,
						RootDir: env.workspaceRoot,
					},
				},
			}),
			WithMinHours(0),
			WithMinSessions(0),
			WithDreamGateConfig(DreamGateConfig{MinCandidates: 5, MinRecallCount: 2, MinScore: 0.75}),
			withLock(lock),
			withNow(func() time.Time { return now }),
		)

		err := service.Run(testutil.Context(t), func(context.Context, string, string, string, time.Time) error {
			return spawnErr
		}, env.workspaceID)

		if !errors.Is(err, spawnErr) {
			t.Fatalf("Run() error = %v, want spawnErr", err)
		}
		assertDreamPromotedCount(t, env.workspaceStore, 0)
		candidates, candidateErr := env.workspaceStore.dreamCandidates(
			testutil.Context(t),
			env.workspaceID,
			DreamGateConfig{MinCandidates: 5, MinRecallCount: 2, MinScore: 0.75},
			now,
		)
		if candidateErr != nil {
			t.Fatalf("dreamCandidates(after provider failure) error = %v", candidateErr)
		}
		if len(candidates) != 5 {
			t.Fatalf("dream candidates after provider failure = %d, want 5 intact", len(candidates))
		}
		assertDreamConsolidationStatus(t, env.workspaceStore, "failed", 0)
		assertDreamEventCount(t, env.workspaceStore, memoryEventDreamFailed, 1)
		failures := globDreamFailures(t, env.workspaceRoot)
		if len(failures) != 1 {
			t.Fatalf("dream failure files = %v, want one file", failures)
		}
		if lock.releaseCalls != 0 {
			t.Fatalf("release calls = %d, want 0", lock.releaseCalls)
		}
		if len(lock.rollbackCalls) != 1 {
			t.Fatalf("rollback calls = %d, want 1", len(lock.rollbackCalls))
		}
	})
}

func TestStoreMarkDreamPromotedIsIdempotent(t *testing.T) {
	t.Parallel()

	t.Run("Should only stamp unpromoted signals once per run", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		env := newDreamSeedEnv(t, now)
		seedDreamRecallSignals(t, env.workspaceStore, memcontract.ScopeWorkspace, env.workspaceID, 2, now)
		candidates, err := env.workspaceStore.dreamCandidates(
			testutil.Context(t),
			env.workspaceID,
			DreamGateConfig{MinCandidates: 2, MinRecallCount: 2, MinScore: 0.75},
			now,
		)
		if err != nil {
			t.Fatalf("dreamCandidates() error = %v", err)
		}
		if len(candidates) != 2 {
			t.Fatalf("dream candidates = %d, want 2", len(candidates))
		}

		first, err := env.workspaceStore.markDreamPromoted(testutil.Context(t), candidates, "dream-run", now)
		if err != nil {
			t.Fatalf("markDreamPromoted(first) error = %v", err)
		}
		second, err := env.workspaceStore.markDreamPromoted(
			testutil.Context(t),
			candidates,
			"dream-run",
			now.Add(time.Hour),
		)
		if err != nil {
			t.Fatalf("markDreamPromoted(second) error = %v", err)
		}

		if first != 2 || second != 0 {
			t.Fatalf("promoted counts = %d/%d, want 2/0", first, second)
		}
		assertPromotionRunID(t, env.workspaceStore, "dream-run")
	})
}

func TestDreamPromotionScoreUsesSignalWeights(t *testing.T) {
	t.Parallel()

	t.Run("Should rank fresh high-recall candidates above stale low-recall candidates", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		config := normalizeDreamGateConfig(DreamGateConfig{MinRecallCount: 2})
		fresh := DreamCandidate{
			RecallCount:        2,
			RecallScore:        0.95,
			LastRecalledAt:     now.Add(-time.Hour),
			FreshnessStartedAt: now.Add(-2 * time.Hour),
		}
		stale := DreamCandidate{
			RecallCount:        1,
			RecallScore:        0.25,
			LastRecalledAt:     now.Add(-45 * 24 * time.Hour),
			FreshnessStartedAt: now.Add(-45 * 24 * time.Hour),
		}

		freshScore := dreamPromotionScore(fresh, config, now)
		staleScore := dreamPromotionScore(stale, config, now)

		if freshScore <= staleScore {
			t.Fatalf("fresh score %.3f <= stale score %.3f, want fresh higher", freshScore, staleScore)
		}
		if freshScore < config.MinScore {
			t.Fatalf("fresh score %.3f < threshold %.3f", freshScore, config.MinScore)
		}
	})
}

func TestDreamSystemPathValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unsafe _system path segments", func(t *testing.T) {
		t.Parallel()

		store := newOpenTestStore(t, filepath.Join(t.TempDir(), "memory"))
		if _, err := store.dreamSystemPath(memcontract.ScopeGlobal, "dreaming", "../bad.json"); err == nil {
			t.Fatal("dreamSystemPath() error = nil, want unsafe segment error")
		}
	})

	t.Run("Should build scoped _system paths without prompt-facing filenames", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "memory")
		store := newOpenTestStore(t, root)
		path, err := store.dreamSystemPath(memcontract.ScopeGlobal, "dream", "failures", "run.json")
		if err != nil {
			t.Fatalf("dreamSystemPath() error = %v", err)
		}
		want := filepath.Join(root, "_system", "dream", "failures", "run.json")
		if path != want {
			t.Fatalf("dream system path = %q, want %q", path, want)
		}
	})
}

func TestServiceRunRequiresWorkspaceResolverForExplicitWorkspace(t *testing.T) {
	t.Parallel()

	lock := &stubLock{
		tryAcquireFn: func() (time.Time, bool, error) {
			return time.Time{}, true, nil
		},
	}
	service := NewService(
		withLock(lock),
		WithMemoryStore(newOpenTestStore(t, filepath.Join(t.TempDir(), "memory"))),
	)

	err := service.Run(
		testutil.Context(t),
		func(context.Context, string, string, string, time.Time) error { return nil },
		"ws-missing",
	)
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "workspace resolver is required") {
		t.Fatalf("Run() error = %v, want workspace resolver error", err)
	}
	if len(lock.rollbackCalls) != 1 {
		t.Fatalf("rollback calls = %d, want 1", len(lock.rollbackCalls))
	}
}

func TestServiceRunResolvesWorkspaceRefBeforeSpawn(t *testing.T) {
	t.Parallel()

	lock := &stubLock{
		tryAcquireFn: func() (time.Time, bool, error) {
			return time.Time{}, true, nil
		},
	}
	resolver := fakeDreamWorkspaceResolver{
		resolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      "ws-resolved",
				RootDir: filepath.Join(t.TempDir(), "workspace"),
			},
		},
	}
	service := NewService(
		withLock(lock),
		WithWorkspaceResolver(&resolver),
	)

	var gotWorkspace string
	err := service.Run(testutil.Context(t), func(_ context.Context, _, _, workspace string, _ time.Time) error {
		gotWorkspace = workspace
		return nil
	}, "workspace-alias")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotWorkspace != "ws-resolved" {
		t.Fatalf("workspace = %q, want ws-resolved", gotWorkspace)
	}
	if got, want := resolver.resolveCalls, 1; got != want {
		t.Fatalf("resolver Resolve() calls = %d, want %d", got, want)
	}
	if got, want := resolver.lastArg, "workspace-alias"; got != want {
		t.Fatalf("resolver Resolve() arg = %q, want %q", got, want)
	}
}

func TestServiceRunWrapsWorkspaceResolveErrors(t *testing.T) {
	t.Parallel()

	lock := &stubLock{
		tryAcquireFn: func() (time.Time, bool, error) {
			return time.Time{}, true, nil
		},
	}
	resolveErr := errors.New("lookup failed")
	service := NewService(
		withLock(lock),
		WithWorkspaceResolver(&fakeDreamWorkspaceResolver{err: resolveErr}),
	)

	err := service.Run(
		testutil.Context(t),
		func(context.Context, string, string, string, time.Time) error { return nil },
		"workspace-alias",
	)
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !errors.Is(err, resolveErr) {
		t.Fatalf("Run() error = %v, want wrapped resolve error", err)
	}
	if !strings.Contains(err.Error(), `resolve workspace "workspace-alias"`) {
		t.Fatalf("Run() error = %v, want resolve workspace context", err)
	}
}

func TestServiceRunWrapsWorkspaceEnsureDirsErrors(t *testing.T) {
	t.Parallel()

	lock := &stubLock{
		tryAcquireFn: func() (time.Time, bool, error) {
			return time.Time{}, true, nil
		},
	}
	rootDir := filepath.Join(t.TempDir(), "workspace")
	service := NewService(
		withLock(lock),
		WithWorkspaceResolver(&fakeDreamWorkspaceResolver{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      "ws-resolved",
					RootDir: rootDir,
				},
			},
		}),
		WithMemoryStore(newOpenTestStore(t, "")),
	)

	err := service.Run(
		testutil.Context(t),
		func(context.Context, string, string, string, time.Time) error { return nil },
		"workspace-alias",
	)
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `ensure workspace memory dirs for "`) {
		t.Fatalf("Run() error = %v, want ensure dirs context", err)
	}
	if !strings.Contains(err.Error(), rootDir) {
		t.Fatalf("Run() error = %v, want workspace root in wrapped error", err)
	}
}

func TestServiceRunRollsBackLockOnSessionSpawnerFailure(t *testing.T) {
	t.Parallel()

	prior := time.Now().UTC().Add(-24 * time.Hour).Round(0)
	lock := &stubLock{
		tryAcquireFn: func() (time.Time, bool, error) {
			return prior, true, nil
		},
	}
	service := NewService(withLock(lock))

	err := service.Run(testutil.Context(t), func(context.Context, string, string, string, time.Time) error {
		return errors.New("boom")
	}, "")
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Run() error = %v, want wrapped spawner failure", err)
	}
	if len(lock.rollbackCalls) != 1 {
		t.Fatalf("rollback calls = %d, want 1", len(lock.rollbackCalls))
	}
	if !lock.rollbackCalls[0].Equal(prior) {
		t.Fatalf("rollback prior mtime = %v, want %v", lock.rollbackCalls[0], prior)
	}
}

func TestServiceRunReturnsJoinedSpawnAndRollbackErrors(t *testing.T) {
	t.Parallel()

	prior := time.Now().UTC().Add(-24 * time.Hour).Round(0)
	spawnErr := errors.New("spawn failed")
	rollbackErr := errors.New("rollback failed")
	lock := &stubLock{
		tryAcquireFn: func() (time.Time, bool, error) {
			return prior, true, nil
		},
		rollbackErr: rollbackErr,
	}
	service := NewService(withLock(lock))

	err := service.Run(testutil.Context(t), func(context.Context, string, string, string, time.Time) error {
		return spawnErr
	}, "")
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !errors.Is(err, spawnErr) {
		t.Fatalf("Run() error = %v, want errors.Is(spawnErr)", err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("Run() error = %v, want errors.Is(rollbackErr)", err)
	}
}

func TestServiceRunReturnsErrLockUnavailableWhenBusy(t *testing.T) {
	t.Parallel()

	lock := &stubLock{
		tryAcquireFn: func() (time.Time, bool, error) {
			return time.Time{}, false, nil
		},
	}
	service := NewService(withLock(lock))

	err := service.Run(
		testutil.Context(t),
		func(context.Context, string, string, string, time.Time) error { return nil },
		"",
	)
	if !errors.Is(err, ErrLockUnavailable) {
		t.Fatalf("Run() error = %v, want ErrLockUnavailable", err)
	}
}

func TestServiceRunValidatesInputs(t *testing.T) {
	t.Parallel()

	service := NewService(withLock(&stubLock{
		tryAcquireFn: func() (time.Time, bool, error) {
			return time.Time{}, true, nil
		},
	}))

	nilContext := func() context.Context { return nil }

	if err := service.Run(
		nilContext(),
		func(context.Context, string, string, string, time.Time) error { return nil },
		"",
	); err == nil {
		t.Fatal("Run(nil context, spawner) error = nil, want non-nil")
	}
	if err := service.Run(testutil.Context(t), nil, ""); err == nil {
		t.Fatal("Run(ctx, nil) error = nil, want non-nil")
	}
}

func TestServiceOptionsClampNegativeThresholds(t *testing.T) {
	t.Parallel()

	service := NewService(
		WithMinHours(-5),
		WithMinSessions(-3),
	)

	if service.minHours != 0 {
		t.Fatalf("minHours = %v, want 0", service.minHours)
	}
	if service.minSessions != 0 {
		t.Fatalf("minSessions = %d, want 0", service.minSessions)
	}
}

func TestServiceValidateRequiresDependencies(t *testing.T) {
	t.Parallel()

	if err := (*Service)(nil).validate(); err == nil {
		t.Fatal("validate(nil) error = nil, want non-nil")
	}

	service := &Service{}
	if err := service.validate(); err == nil {
		t.Fatal("validate(empty service) error = nil, want non-nil")
	}

	service = NewService(withLock(&stubLock{}))
	service.logger = nil
	if err := service.validate(); err == nil {
		t.Fatal("validate(nil logger) error = nil, want non-nil")
	}
}

func TestServiceScanCompletedSessionsSinceFiltersByStoppedAt(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Round(0)
	root := t.TempDir()
	since := now.Add(-48 * time.Hour)
	writeSessionMeta(t, root, "equal", persistedSessionMetadata{StoppedAt: new(since)})
	writeSessionMeta(t, root, "after", persistedSessionMetadata{StoppedAt: new(since.Add(time.Minute))})
	writeSessionMeta(t, root, "before", persistedSessionMetadata{StoppedAt: new(since.Add(-time.Minute))})
	writeMalformedSessionMeta(t, root, "malformed")

	service := NewService(
		WithSessionsDir(root),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)

	count, err := service.scanCompletedSessionsSince(since)
	if err != nil {
		t.Fatalf("scanCompletedSessionsSince() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("scanCompletedSessionsSince() = %d, want 2", count)
	}
}

func TestServiceScanCompletedSessionsSinceIgnoresIncompleteSessions(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Round(0)
	root := t.TempDir()
	writeSessionMeta(t, root, "complete", persistedSessionMetadata{StoppedAt: new(now)})
	writeSessionMeta(t, root, "incomplete", persistedSessionMetadata{})

	service := NewService(
		WithSessionsDir(root),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)

	count, err := service.scanCompletedSessionsSince(time.Time{})
	if err != nil {
		t.Fatalf("scanCompletedSessionsSince() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("scanCompletedSessionsSince() = %d, want 1", count)
	}
}

func TestServiceScanCompletedSessionsSinceFallsBackToStoppedStateUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Round(0)
	root := t.TempDir()
	writeSessionMeta(t, root, "stopped", persistedSessionMetadata{
		State:     "stopped",
		UpdatedAt: now,
	})
	writeSessionMeta(t, root, "active", persistedSessionMetadata{
		State:     "active",
		UpdatedAt: now,
	})

	service := NewService(
		WithSessionsDir(root),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)

	count, err := service.scanCompletedSessionsSince(time.Time{})
	if err != nil {
		t.Fatalf("scanCompletedSessionsSince() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("scanCompletedSessionsSince() = %d, want 1", count)
	}
}

func TestServiceScanCompletedSessionsSinceValidatesDirectoryAndMissingPath(t *testing.T) {
	t.Parallel()

	service := NewService(WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))

	if _, err := service.scanCompletedSessionsSince(time.Time{}); err == nil {
		t.Fatal("scanCompletedSessionsSince() error = nil, want non-nil")
	}

	service.sessionsDir = filepath.Join(t.TempDir(), "missing")
	count, err := service.scanCompletedSessionsSince(time.Time{})
	if err != nil {
		t.Fatalf("scanCompletedSessionsSince() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("scanCompletedSessionsSince() = %d, want 0", count)
	}
}

func TestServiceScanCompletedSessionsSinceFailsWhenPathIsFile(t *testing.T) {
	t.Parallel()

	service := NewService(WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	path := filepath.Join(t.TempDir(), "sessions-file")
	if err := os.WriteFile(path, []byte("x"), filePerm); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	service.sessionsDir = path

	if _, err := service.scanCompletedSessionsSince(time.Time{}); err == nil {
		t.Fatal("scanCompletedSessionsSince() error = nil, want non-nil")
	}
}

func TestServiceTimeGatePassesForZeroTimestampAndZeroThreshold(t *testing.T) {
	t.Parallel()

	service := NewService(WithMinHours(0))
	if !service.timeGatePasses(time.Time{}) {
		t.Fatal("timeGatePasses(zero) = false, want true")
	}
	if !service.timeGatePasses(time.Now().UTC()) {
		t.Fatal("timeGatePasses(now) with zero threshold = false, want true")
	}
}

func TestServiceAcquireLockReturnsFalseWhenPending(t *testing.T) {
	t.Parallel()

	service := NewService(withLock(&stubLock{}))
	service.pending = true

	_, ok, err := service.acquireLock()
	if err != nil {
		t.Fatalf("acquireLock() error = %v", err)
	}
	if ok {
		t.Fatal("acquireLock() ok = true, want false")
	}
}

func TestServiceCompleteRunReturnsReleaseError(t *testing.T) {
	t.Parallel()

	releaseErr := errors.New("release failed")
	service := NewService(withLock(&stubLock{releaseErr: releaseErr}))
	service.pending = true

	err := service.completeRun(true, time.Time{})
	if !errors.Is(err, releaseErr) {
		t.Fatalf("completeRun(true) error = %v, want release error", err)
	}
	if service.pending {
		t.Fatal("service.pending = true, want false")
	}
}

func TestServiceRunSerializesConcurrentCalls(t *testing.T) {
	t.Parallel()

	lock := &stubLock{
		tryAcquireFn: func() (time.Time, bool, error) {
			return time.Time{}, true, nil
		},
	}
	service := NewService(withLock(lock))

	var active atomic.Int32
	var maxActive atomic.Int32
	started := make(chan struct{}, 2)
	releaseFirst := make(chan struct{})
	errCh := make(chan error, 2)
	var first sync.Once

	spawner := func(context.Context, string, string, string, time.Time) error {
		current := active.Add(1)
		for {
			previous := maxActive.Load()
			if current <= previous || maxActive.CompareAndSwap(previous, current) {
				break
			}
		}

		started <- struct{}{}
		shouldBlock := false
		first.Do(func() { shouldBlock = true })
		if shouldBlock {
			<-releaseFirst
		}

		active.Add(-1)
		return nil
	}

	go func() {
		errCh <- service.Run(testutil.Context(t), spawner, "")
	}()
	<-started

	go func() {
		errCh <- service.Run(testutil.Context(t), spawner, "")
	}()

	select {
	case <-started:
		t.Fatal("second spawner started before first run finished")
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseFirst)
	<-started

	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	}
	if got := maxActive.Load(); got != 1 {
		t.Fatalf("max concurrent spawners = %d, want 1", got)
	}
}

type stubLock struct {
	lastConsolidatedAt time.Time
	tryAcquireFn       func() (time.Time, bool, error)
	releaseAt          time.Time
	releaseErr         error
	rollbackErr        error

	tryAcquireCalls int
	releaseCalls    int
	rollbackCalls   []time.Time
}

func (s *stubLock) LastConsolidatedAt() (time.Time, error) {
	return s.lastConsolidatedAt, nil
}

func (s *stubLock) TryAcquire() (time.Time, bool, error) {
	s.tryAcquireCalls++
	if s.tryAcquireFn != nil {
		return s.tryAcquireFn()
	}
	return time.Time{}, true, nil
}

func (s *stubLock) Release() error {
	s.releaseCalls++
	if !s.releaseAt.IsZero() {
		s.lastConsolidatedAt = s.releaseAt
	}
	return s.releaseErr
}

func (s *stubLock) Rollback(priorMtime time.Time) error {
	s.rollbackCalls = append(s.rollbackCalls, priorMtime)
	return s.rollbackErr
}

func writeSessionMeta(t *testing.T, sessionsDir string, sessionID string, meta persistedSessionMetadata) {
	t.Helper()

	sessionDir := filepath.Join(sessionsDir, sessionID)
	if err := os.MkdirAll(sessionDir, dirPerm); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	payload, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(sessionDir, "meta.json"), payload, filePerm); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}

func writeMalformedSessionMeta(t *testing.T, sessionsDir string, sessionID string) {
	t.Helper()

	sessionDir := filepath.Join(sessionsDir, sessionID)
	if err := os.MkdirAll(sessionDir, dirPerm); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "meta.json"), []byte("{bad json"), filePerm); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}

type dreamSeedEnv struct {
	baseStore      *Store
	workspaceStore *Store
	workspaceRoot  string
	workspaceID    string
}

func newDreamSeedEnv(t *testing.T, _ time.Time) *dreamSeedEnv {
	t.Helper()

	baseDir := t.TempDir()
	workspaceRoot := filepath.Join(baseDir, "workspace")
	baseStore := newOpenTestStore(t,
		filepath.Join(baseDir, "global", "memory"),
		WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
	)
	workspaceStore := baseStore.ForWorkspace(workspaceRoot)
	if err := workspaceStore.EnsureDirs(); err != nil {
		t.Fatalf("Store.EnsureDirs() error = %v", err)
	}
	identity, err := workspacepkg.EnsureIdentity(testutil.Context(t), workspaceRoot)
	if err != nil {
		t.Fatalf("EnsureIdentity() error = %v", err)
	}
	return &dreamSeedEnv{
		baseStore:      baseStore,
		workspaceStore: workspaceStore,
		workspaceRoot:  workspaceRoot,
		workspaceID:    identity.WorkspaceID,
	}
}

func seedDreamRecallSignals(
	t *testing.T,
	store *Store,
	scope memcontract.Scope,
	workspaceID string,
	count int,
	now time.Time,
) {
	t.Helper()

	for idx := range count {
		filename := fmt.Sprintf("project_signal_%02d.md", idx)
		content := fmt.Sprintf("Recurring operator preference %02d needs durable recall.\n", idx)
		if err := store.Write(scope, filename, mustMemoryContent(t, testMemoryMeta{
			Name:        fmt.Sprintf("Signal %02d", idx),
			Description: "Dreaming signal fixture",
			Type:        memcontract.TypeProject,
		}, content)); err != nil {
			t.Fatalf("Store.Write(%q) error = %v", filename, err)
		}
		chunkID := dreamChunkIDForFilename(t, store, filename)
		for seq := range 2 {
			if err := store.RecordRecall(testutil.Context(t), []memoryrecall.Signal{{
				ChunkID:     chunkID,
				WorkspaceID: workspaceID,
				SurfaceID:   fmt.Sprintf("surface-%02d-%02d", idx, seq),
				Score:       0.98,
				SurfacedAt:  now.Add(time.Duration(seq) * time.Minute),
				SessionID:   fmt.Sprintf("session-%02d", seq),
			}}); err != nil {
				t.Fatalf("RecordRecall(%q) error = %v", chunkID, err)
			}
		}
	}
}

func dreamChunkIDForFilename(t *testing.T, store *Store, filename string) string {
	t.Helper()

	db, err := store.catalog.ensureDB(testutil.Context(t))
	if err != nil {
		t.Fatalf("ensureDB() error = %v", err)
	}
	var chunkID string
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT c.id
		 FROM memory_chunks c
		 JOIN memory_catalog_entries e ON e.id = c.file_id
		 WHERE e.filename = ?`,
		filename,
	).Scan(&chunkID); err != nil {
		t.Fatalf("query chunk id for %q error = %v", filename, err)
	}
	return chunkID
}

func assertDreamPromotedCount(t *testing.T, store *Store, want int) {
	t.Helper()

	db, err := store.catalog.ensureDB(testutil.Context(t))
	if err != nil {
		t.Fatalf("ensureDB() error = %v", err)
	}
	var got int
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT COUNT(*) FROM memory_recall_signals WHERE promoted_at IS NOT NULL`,
	).Scan(&got); err != nil {
		t.Fatalf("query promoted count error = %v", err)
	}
	if got != want {
		t.Fatalf("promoted signal count = %d, want %d", got, want)
	}
}

func assertDreamConsolidationStatus(t *testing.T, store *Store, wantStatus string, wantPromoted int) {
	t.Helper()

	db, err := store.catalog.ensureDB(testutil.Context(t))
	if err != nil {
		t.Fatalf("ensureDB() error = %v", err)
	}
	var status string
	var promoted int
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT status, promoted_count FROM memory_consolidations ORDER BY started_at DESC LIMIT 1`,
	).Scan(&status, &promoted); err != nil {
		t.Fatalf("query dream consolidation status error = %v", err)
	}
	if status != wantStatus || promoted != wantPromoted {
		t.Fatalf("dream consolidation = %s/%d, want %s/%d", status, promoted, wantStatus, wantPromoted)
	}
}

func assertDreamConsolidationCount(t *testing.T, store *Store, want int) {
	t.Helper()

	db, err := store.catalog.ensureDB(testutil.Context(t))
	if err != nil {
		t.Fatalf("ensureDB() error = %v", err)
	}
	var got int
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT COUNT(*) FROM memory_consolidations`,
	).Scan(&got); err != nil {
		t.Fatalf("query dream consolidation count error = %v", err)
	}
	if got != want {
		t.Fatalf("dream consolidation count = %d, want %d", got, want)
	}
}

func assertDreamEventCount(t *testing.T, store *Store, op string, want int) {
	t.Helper()

	db, err := store.catalog.ensureDB(testutil.Context(t))
	if err != nil {
		t.Fatalf("ensureDB() error = %v", err)
	}
	var got int
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT COUNT(*) FROM memory_events WHERE op = ?`,
		op,
	).Scan(&got); err != nil {
		t.Fatalf("query dream event count error = %v", err)
	}
	if got != want {
		t.Fatalf("dream event count for %s = %d, want %d", op, got, want)
	}
}

func assertPromotionRunID(t *testing.T, store *Store, runID string) {
	t.Helper()

	db, err := store.catalog.ensureDB(testutil.Context(t))
	if err != nil {
		t.Fatalf("ensureDB() error = %v", err)
	}
	var got int
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT COUNT(*) FROM memory_recall_signals WHERE promotion_run_id = ?`,
		runID,
	).Scan(&got); err != nil {
		t.Fatalf("query promotion run id error = %v", err)
	}
	if got == 0 {
		t.Fatalf("promotion_run_id %q count = 0, want promoted rows", runID)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
	}
}

func globDreamFailures(t *testing.T, workspaceRoot string) []string {
	t.Helper()

	matches, err := filepath.Glob(
		filepath.Join(workspaceRoot, ".agh", "memory", "_system", "dream", "failures", "*.json"),
	)
	if err != nil {
		t.Fatalf("filepath.Glob() error = %v", err)
	}
	return matches
}

type fakeDreamWorkspaceResolver struct {
	resolved workspacepkg.ResolvedWorkspace
	err      error

	resolveCalls int
	lastArg      string
}

func (r *fakeDreamWorkspaceResolver) Resolve(_ context.Context, arg string) (workspacepkg.ResolvedWorkspace, error) {
	if r.err != nil {
		return workspacepkg.ResolvedWorkspace{}, r.err
	}
	r.resolveCalls++
	r.lastArg = strings.TrimSpace(arg)
	return r.resolved, nil
}

func (r *fakeDreamWorkspaceResolver) ResolveOrRegister(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	if r.err != nil {
		return workspacepkg.ResolvedWorkspace{}, r.err
	}
	return r.resolved, nil
}
