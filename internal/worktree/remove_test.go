package worktree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/redact"
)

// Canonical suite: removal fencing, authoritative safety checks, and boot recovery.
func TestServiceRemoveAndRecover(t *testing.T) {
	t.Parallel()

	t.Run("Should stage checkout bytes and release the repository before collection", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		payload := filepath.Join(f.item.Path, "large-dependency")
		if err := os.WriteFile(payload, []byte("preserved until collection"), 0o600); err != nil {
			t.Fatal(err)
		}
		if refusal, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); err != nil ||
			refusal != nil {
			t.Fatalf("Remove() = %#v, %v", refusal, err)
		}
		staged := filepath.Join(f.service.root, trashDirectory, f.item.ID, "large-dependency")
		if _, err := os.Stat(staged); err != nil {
			t.Fatalf("bytes deleted before collection: %v", err)
		}
		if _, err := os.Stat(f.item.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("checkout still exists: %v", err)
		}
		if f.mustItem(t).State != StateRemoved {
			t.Fatal("removal did not finish")
		}
		release, err := f.service.locks.Acquire(t.Context(), f.commonDir)
		if err != nil {
			t.Fatal(err)
		}
		release()
	})

	t.Run("Should restore changes written while staging without clearing Git registration", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		f.service.renameCheckout = func(from, to string) error {
			if from == f.item.Path {
				if err := os.WriteFile(filepath.Join(from, "changed.txt"), []byte("late work"), 0o600); err != nil {
					return err
				}
				f.dirty.Store(true)
			}
			return os.Rename(from, to)
		}
		refusal, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false)
		if !errors.Is(err, ErrDirtyRequiresForce) || refusal == nil || refusal.Risk.ChangedFiles != 1 {
			t.Fatalf("late-write removal = %#v, %v", refusal, err)
		}
		if f.mustItem(t).State != StateReady || f.removeCalls.Load() != 0 {
			t.Fatal("late-write refusal changed registration or stranded the fence")
		}
		if data, err := os.ReadFile(filepath.Join(f.item.Path, "changed.txt")); err != nil ||
			string(data) != "late work" {
			t.Fatalf("late work = %q, %v", data, err)
		}
	})

	t.Run("Should recheck the removal fence when staging changes during trash discovery", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		f.service.renameCheckout = func(from, to string) error {
			if from != f.item.Path {
				return errors.New("restore busy")
			}
			if err := os.WriteFile(filepath.Join(from, "changed.txt"), []byte("refused work"), 0o600); err != nil {
				return err
			}
			f.dirty.Store(true)
			return os.Rename(from, to)
		}
		f.service.resolver = removalTrashRootsResolver{
			staticWorkspaceResolver: {
				workspaces: map[string]Workspace{f.workspace.ID: f.workspace},
			},
			roots: []string{f.service.root},
			onList: func() {
				if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); !errors.Is(
					err,
					ErrDirtyRequiresForce,
				) {
					t.Fatalf("late refusal = %v", err)
				}
			},
		}
		if err := f.service.collectTrash(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		staged := filepath.Join(f.service.root, trashDirectory, f.item.ID, "changed.txt")
		if data, err := os.ReadFile(staged); err != nil || string(data) != "refused work" {
			t.Fatalf("stale-sweep refused work = %q, %v", data, err)
		}
	})

	for _, recreated := range []bool{false, true} {
		t.Run(
			"Should preserve refused staging across failed recovery with recreated path "+strconv.FormatBool(recreated),
			func(t *testing.T) {
				t.Parallel()
				f := newRemovalTestFixture(t)
				f.service.renameCheckout = func(from, to string) error {
					if from != f.item.Path {
						return errors.New("restore busy")
					}
					if err := os.WriteFile(
						filepath.Join(from, "changed.txt"), []byte("refused work"), 0o600,
					); err != nil {
						return err
					}
					f.dirty.Store(true)
					return os.Rename(from, to)
				}
				refusal, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false)
				if !errors.Is(err, ErrDirtyRequiresForce) || refusal == nil || f.mustItem(t).State != StateRemoving {
					t.Fatalf("failed restore = %#v, %v, state %s", refusal, err, f.mustItem(t).State)
				}
				staged := filepath.Join(f.service.root, trashDirectory, f.item.ID, "changed.txt")
				if recreated {
					if err := os.Mkdir(f.item.Path, 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(f.item.Path, "foreign"), []byte("keep"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); !errors.Is(
					err,
					ErrNotReady,
				) {
					t.Fatalf("retry = %v, want pending recovery", err)
				}
				if err := f.service.RecoverCreations(t.Context()); err != nil {
					t.Fatal(err)
				}
				if err := f.service.collectTrash(t.Context(), nil); err != nil {
					t.Fatal(err)
				}
				if data, err := os.ReadFile(staged); err != nil || string(data) != "refused work" {
					t.Fatalf("refused staged work = %q, %v", data, err)
				}
				if f.mustItem(t).State != StateRemoving || f.removeCalls.Load() != 0 {
					t.Fatal("failed recovery cleared the fence or Git registration")
				}
				if recreated {
					if err := os.Remove(filepath.Join(f.item.Path, "foreign")); err != nil {
						t.Fatal(err)
					}
					if err := os.Remove(f.item.Path); err != nil {
						t.Fatal(err)
					}
				}
				f.service.renameCheckout = os.Rename
				refusal, err = f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false)
				if !errors.Is(err, ErrDirtyRequiresForce) || refusal == nil || f.mustItem(t).State != StateReady {
					t.Fatalf("restored dirty checkout = %#v, %v, state %s", refusal, err, f.mustItem(t).State)
				}
				if data, err := os.ReadFile(filepath.Join(f.item.Path, "changed.txt")); err != nil ||
					string(data) != "refused work" {
					t.Fatalf("restored work = %q, %v", data, err)
				}
			},
		)
	}

	t.Run("Should remove synchronously when the same ID still has staged bytes", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		trash := filepath.Join(f.service.root, trashDirectory)
		if err := prepareOwnedTrash(trash); err != nil {
			t.Fatal(err)
		}
		prior := filepath.Join(trash, f.item.ID)
		if err := os.Mkdir(prior, 0o700); err != nil {
			t.Fatal(err)
		}
		sentinel := filepath.Join(prior, "old-bytes")
		if err := os.WriteFile(sentinel, []byte("prior staged checkout"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); err != nil {
			t.Fatal(err)
		}
		if f.mustItem(t).State != StateRemoved || f.removeCalls.Load() != 1 {
			t.Fatal("staging collision blocked removal")
		}
		if data, err := os.ReadFile(sentinel); err != nil || string(data) != "prior staged checkout" {
			t.Fatalf("prior staged checkout changed: %q, %v", data, err)
		}
	})

	for _, nested := range []bool{false, true} {
		t.Run(
			"Should preserve a registered checkout inside owned trash nested "+strconv.FormatBool(nested),
			func(t *testing.T) {
				t.Parallel()
				f := newRemovalTestFixture(t)
				trash := filepath.Join(f.service.root, trashDirectory)
				if err := prepareOwnedTrash(trash); err != nil {
					t.Fatal(err)
				}
				checkout := filepath.Join(trash, "live-checkout")
				if nested {
					checkout = filepath.Join(checkout, "nested")
				}
				if err := os.MkdirAll(checkout, 0o700); err != nil {
					t.Fatal(err)
				}
				sentinel := filepath.Join(checkout, "uncommitted")
				if err := os.WriteFile(sentinel, []byte("keep live work"), 0o600); err != nil {
					t.Fatal(err)
				}
				live := f.item
				live.ID, live.Name, live.Path = "wt-live", "live-checkout", canonicalComparablePath(checkout)
				if err := f.store.Insert(t.Context(), live); err != nil {
					t.Fatal(err)
				}
				if err := f.service.collectTrash(t.Context(), nil); err != nil {
					t.Fatal(err)
				}
				if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep live work" {
					t.Fatalf("registered checkout changed: %q, %v", data, err)
				}
			},
		)
	}

	t.Run("Should fall back to synchronous removal across filesystem volumes", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		attempted := false
		f.service.renameCheckout = func(string, string) error { attempted = true; return syscall.EXDEV }
		if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); err != nil {
			t.Fatal(err)
		}
		if !attempted || f.mustItem(t).State != StateRemoved || f.removeCalls.Load() != 1 {
			t.Fatal("fallback did not remove the checkout")
		}
		if _, err := os.Stat(f.item.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("fallback left checkout: %v", err)
		}
	})

	t.Run("Should sweep boot leftovers while yielding to pressure and preserving external links", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		outside := filepath.Join(t.TempDir(), "keep")
		if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(f.item.Path, "external")); err != nil {
			t.Fatal(err)
		}
		if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, true); err != nil {
			t.Fatal(err)
		}
		recovered := NewService(f.store, f.runner, WithConfig(config.DefaultWorktreesConfig(), f.service.root))
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		observed := make(chan struct{})
		var once sync.Once
		recovered.StartTrashCollector(ctx, func() bool { once.Do(func() { close(observed) }); return true })
		select {
		case <-observed:
		case <-time.After(5 * time.Second):
			t.Fatal("collector did not sweep on boot")
		}
		staged := filepath.Join(f.service.root, trashDirectory, f.item.ID)
		if _, err := os.Stat(staged); err != nil {
			t.Fatalf("collector ignored pressure: %v", err)
		}
		recovered.Close()
		select {
		case <-recovered.trash.done:
		default:
			t.Fatal("collector goroutine outlived Close")
		}
		if err := recovered.collectTrash(t.Context(), func() bool { return false }); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(staged); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("boot leftovers remain: %v", err)
		}
		if content, err := os.ReadFile(outside); err != nil || string(content) != "keep" {
			t.Fatalf("outside trash changed: %q, %v", content, err)
		}
	})

	for _, directory := range []string{trashDirectory, ".trash", ".Trash"} {
		t.Run("Should preserve unowned trash at "+directory, func(t *testing.T) {
			t.Parallel()
			f := newRemovalTestFixture(t)
			unowned := filepath.Join(f.service.root, directory)
			if err := os.MkdirAll(unowned, 0o700); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(unowned, "keep")
			if err := os.WriteFile(sentinel, []byte("user data"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); err != nil {
				t.Fatal(err)
			}
			if err := f.service.collectTrash(t.Context(), nil); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(sentinel); err != nil || string(data) != "user data" {
				t.Fatalf("unowned data = %q, %v", data, err)
			}
			if directory == trashDirectory {
				if f.removeCalls.Load() != 1 {
					t.Fatal("unowned trash did not use synchronous removal")
				}
				if _, err := os.Lstat(filepath.Join(unowned, trashOwnerFile)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("unowned directory was adopted: %v", err)
				}
			}
		})
	}

	t.Run("Should rediscover configured profile trash without in-memory staging roots", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); err != nil {
			t.Fatal(err)
		}
		resolver := removalTrashRootsResolver{roots: []string{f.service.root}}
		recovered := NewService(f.store, f.runner,
			WithConfig(config.DefaultWorktreesConfig(), t.TempDir()), WithWorkspaceResolver(resolver))
		if err := recovered.collectTrash(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(f.service.root, trashDirectory, f.item.ID)); !errors.Is(
			err,
			os.ErrNotExist,
		) {
			t.Fatalf("profile boot leftovers remain: %v", err)
		}
	})

	t.Run("Should roll back the original fence when the fenced target read fails", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		cause := errors.New("fenced read unavailable")
		f.service.store = &getErrorWorktreeStore{memoryWorktreeStore: f.store, successReads: 2, err: cause}
		if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); !errors.Is(err, cause) {
			t.Fatalf("Remove() error = %v", err)
		}
		if f.mustItem(t).State != StateReady || f.removeCalls.Load() != 0 {
			t.Fatal("failed read stranded the original removal fence")
		}
	})

	t.Run("Should reconcile an abandoned removal on the next removal request", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		if err := f.store.SetState(
			t.Context(), f.workspace.ID, f.item.ID, StateRemoving, f.item.UpdatedAt,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); err != nil {
			t.Fatal(err)
		}
		if f.mustItem(t).State != StateRemoved {
			t.Fatal("abandoned removal needs daemon restart")
		}
	})

	t.Run("Should preview loss without hooks checkout mutations or status writes", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		f.dirty.Store(true)
		f.service.hooks = removalHookDispatcher{
			dispatch: func(HookRequest) (HookVerdict, error) { t.Fatal("preview ran hooks"); return HookVerdict{}, nil },
		}
		risk, refusal, err := f.service.PreviewRemoval(
			t.Context(),
			f.workspace.ID,
			RemovalTarget{Ref: f.item.ID},
			false,
		)
		if !errors.Is(err, ErrDirtyRequiresForce) || refusal == nil || risk.ChangedFiles != 1 {
			t.Fatalf("preview = %#v, %#v, %v", risk, refusal, err)
		}
		if f.mustItem(t).State != StateReady || f.removeCalls.Load() != 0 {
			t.Fatal("preview mutated checkout")
		}
		if status, err := f.store.GetStatus(t.Context(), f.workspace.ID, f.item.ID); err != nil || status != nil {
			t.Fatalf("preview persisted status: %#v, %v", status, err)
		}
	})

	t.Run("Should remove an absent missing registration without filesystem or Git mutations", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		if err := os.RemoveAll(f.item.Path); err != nil {
			t.Fatal(err)
		}
		if err := f.store.SetState(t.Context(), f.workspace.ID, f.item.ID, StateMissing, f.item.UpdatedAt); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, true); err != nil {
				t.Fatal(err)
			}
		}
		if f.mustItem(t).State != StateDismissed || len(f.runner.invocations()) != 0 ||
			f.events.count(EventDismissed) != 1 {
			t.Fatal("missing removal did not settle metadata exactly once")
		}
		if _, err := os.Lstat(f.item.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing path = %v", err)
		}
	})

	t.Run("Should run teardown before removal with the setup environment and redacted output", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		settings := config.DefaultWorktreesConfig()
		settings.TeardownCommand = `test "$PWD" = "$COMPOZY_WORKTREE_PATH" && test "$COMPOZY_WORKTREE_ID" = "wt-remove" && test "$COMPOZY_WORKTREE_BRANCH" = "feature/remove" && printf 'cleaned\n' && printf 'token=super-secret-teardown\n' >&2`
		f.service.getenv = func(string) string { return "/bin/sh" }
		WithConfig(settings, t.TempDir())(f.service)
		if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); err != nil {
			t.Fatal(err)
		}
		if f.events.count(EventTeardownOutput) != 2 || f.events.count(EventTeardownFailed) != 0 ||
			f.mustItem(t).State != StateRemoved {
			t.Fatal("teardown output or removal missing")
		}
		for _, event := range f.events.events {
			if event.Name == EventTeardownOutput && event.RunID != f.item.RunID {
				t.Fatal("missing run attribution")
			}
			if strings.Contains(string(event.Payload), "super-secret-teardown") {
				t.Fatal("secret exposed")
			}
		}
	})
	for _, command := range []string{"printf owner-cleanup", ""} {
		t.Run("Should use owning profile teardown settings including disabled override "+command, func(t *testing.T) {
			t.Parallel()
			f := newRemovalTestFixture(t)
			f.item.ProfileID = "profile-owner"
			f.store.mu.Lock()
			f.store.items[worktreeStoreKey(f.workspace.ID, f.item.ID)] = f.item
			f.store.mu.Unlock()
			workspace := f.workspace
			workspace.Worktrees = config.DefaultWorktreesConfig()
			workspace.Worktrees.TeardownCommand = command
			global := config.DefaultWorktreesConfig()
			global.TeardownCommand = "printf incorrect-global-cleanup"
			WithConfig(global, t.TempDir())(f.service)
			f.service.resolver = staticWorkspaceResolver{workspaces: map[string]Workspace{workspace.ID: f.workspace},
				profileWorkspaces: map[string]map[string]Workspace{"profile-owner": {workspace.ID: workspace}}}
			f.service.getenv = func(string) string { return "/bin/sh" }
			if _, err := f.service.Remove(t.Context(), workspace.ID, f.item.ID, false); err != nil {
				t.Fatal(err)
			}
			want := 0
			if command != "" {
				want = 1
			}
			if f.events.count(EventTeardownOutput) != want {
				t.Fatal("wrong profile teardown output")
			}
			for _, event := range f.events.events {
				if strings.Contains(string(event.Payload), "incorrect-global-cleanup") {
					t.Fatal("used global teardown")
				}
			}
		})
	}

	t.Run("Should redact multiline registered secrets before publishing bounded streams", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		secret := "teardown-first-line\nteardown-second-line"
		t.Cleanup(redact.RegisterDynamicSecret(secret))
		settings := config.DefaultWorktreesConfig()
		settings.TeardownCommand = `printf 'teardown-first-line\nteardown-second-line\n'; printf 'teardown-first-line\nteardown-second-line\n' >&2`
		f.service.getenv = func(string) string { return "/bin/sh" }
		WithConfig(settings, t.TempDir())(f.service)
		if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); err != nil {
			t.Fatal(err)
		}
		if f.events.count(EventTeardownOutput) != 2 {
			t.Fatal("missing output streams")
		}
		for _, event := range f.events.events {
			if strings.Contains(string(event.Payload), "teardown-first-line") ||
				strings.Contains(string(event.Payload), "teardown-second-line") {
				t.Fatal("multiline secret exposed")
			}
		}
	})

	// Invariant: bounded captured output survives event chunk limits without splitting Unicode or exposing secrets.
	// Owner: worktree removal; canonical removal and recovery suite.
	t.Run("Should publish all captured teardown output within event limits", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		secret := "chunk-first-line\nchunk-second-line"
		t.Cleanup(redact.RegisterDynamicSecret(secret))
		prefix := strings.Repeat("é", 2047) + "x"
		output := prefix + secret + strings.Repeat("é", 2500)
		settings := config.DefaultWorktreesConfig()
		settings.TeardownCommand = "printf '%s' '" + output + "'; printf '%s' '" + output + "' >&2"
		WithConfig(settings, t.TempDir())(f.service)
		f.service.getenv = func(string) string { return "/bin/sh" }
		if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); err != nil {
			t.Fatal(err)
		}
		streams := map[string]string{}
		for _, event := range f.events.events {
			if event.Name != EventTeardownOutput {
				continue
			}
			var payload ExitEventPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Chunk) > exitHookOutputLimit {
				t.Fatal("oversized event")
			}
			streams[payload.Stream] += payload.Chunk
		}
		want := redact.String(output)
		if streams[string(GitOutputStdout)] != want || streams[string(GitOutputStderr)] != want {
			t.Fatal("captured output was lost or secret was exposed")
		}
	})

	for _, tc := range []struct {
		name, command, cause string
		timeout              time.Duration
	}{
		{"Should continue removal after teardown failure", "printf failure >&2; exit 7", "exit status 7: failure", time.Second},
		{"Should continue removal after teardown timeout", "sleep 5", "context deadline exceeded", 30 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newRemovalTestFixture(t)
			settings := config.DefaultWorktreesConfig()
			settings.TeardownCommand, settings.TeardownTimeout = tc.command, tc.timeout
			f.service.getenv = func(string) string { return "/bin/sh" }
			WithConfig(settings, t.TempDir())(f.service)
			if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false); err != nil {
				t.Fatal(err)
			}
			if f.events.count(EventTeardownFailed) != 1 || f.mustItem(t).State != StateRemoved {
				t.Fatal("failure must be observable and removal must complete")
			}
			for _, event := range f.events.events {
				if event.Name != EventTeardownFailed {
					continue
				}
				var payload ExitEventPayload
				if err := json.Unmarshal(event.Payload, &payload); err != nil {
					t.Fatal(err)
				}
				if event.RunID != f.item.RunID {
					t.Fatal("missing failure run attribution")
				}
				if !strings.Contains(payload.Message, tc.cause) {
					t.Fatalf("failure cause = %q, want %q", payload.Message, tc.cause)
				}
			}
		})
	}
	t.Run("Should refuse new unpushed commits introduced during teardown", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		settings := config.DefaultWorktreesConfig()
		settings.TeardownCommand = "printf cleanup"
		f.service.getenv = func(string) string { return "/bin/sh" }
		WithConfig(settings, t.TempDir())(f.service)
		f.events.onEvent = func(event LifecycleEvent) {
			if event.Name == EventTeardownOutput {
				f.uniqueCommits.Store(1)
			}
		}
		refusalResult, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, false)
		if !errors.Is(err, ErrUnpushedRequiresForce) || refusalResult == nil ||
			refusalResult.Risk.UnpushedCommits != 1 {
			t.Fatalf("Remove after teardown = (%#v, %v)", refusalResult, err)
		}
		if f.removeCalls.Load() != 0 || f.mustItem(t).State != StateReady {
			t.Fatal("unsafe checkout removed")
		}
	})

	t.Run("Should remove a checkout with unpushed commits without force and keep its branch", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		f.item.Branch = "run/remove"
		f.item.CreatedBranch = true
		f.item.RunNamespace = "run/"
		f.item.CreatedHead = "base-head"
		f.store.mu.Lock()
		f.store.items[worktreeStoreKey(f.workspace.ID, f.item.ID)] = f.item
		f.store.mu.Unlock()
		f.uniqueCommits.Store(2)
		refusalResult, err := f.service.RemoveKeepingBranch(t.Context(), f.workspace.ID, f.item.ID)
		if err != nil || refusalResult != nil || f.mustItem(t).State != StateRemoved {
			t.Fatalf("RemoveKeepingBranch = (%#v, %v), state %s", refusalResult, err, f.mustItem(t).State)
		}
		commands := invocationCommands(f.runner.invocations())
		if containsCommandPrefix(commands, "update-ref -d ") {
			t.Fatal("the kept branch was reclaimed")
		}
		for _, command := range commands {
			if strings.Contains(command, "worktree remove") && strings.Contains(command, "--force") {
				t.Fatalf("checkout removed with force: %s", command)
			}
		}
	})

	t.Run("Should refuse uncommitted files a teardown leaves when keeping the branch", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		settings := config.DefaultWorktreesConfig()
		settings.TeardownCommand = "printf cleanup"
		f.service.getenv = func(string) string { return "/bin/sh" }
		WithConfig(settings, t.TempDir())(f.service)
		f.uniqueCommits.Store(1)
		f.events.onEvent = func(event LifecycleEvent) {
			if event.Name == EventTeardownOutput {
				f.dirty.Store(true)
			}
		}
		refusalResult, err := f.service.RemoveKeepingBranch(t.Context(), f.workspace.ID, f.item.ID)
		if !errors.Is(err, ErrDirtyRequiresForce) || refusalResult == nil {
			t.Fatalf("RemoveKeepingBranch after a dirty teardown = (%#v, %v)", refusalResult, err)
		}
		if f.removeCalls.Load() != 0 || f.mustItem(t).State != StateReady {
			t.Fatal("checkout with uncommitted files removed")
		}
	})

	t.Run("Should refuse to keep-branch remove a checkout another unarchived session binds", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		WithSessionGuard(fixedSessionGuard{bound: true})(f.service)
		refusalResult, err := f.service.RemoveKeepingBranch(t.Context(), f.workspace.ID, f.item.ID)
		if !errors.Is(err, ErrSessionBound) || refusalResult != nil {
			t.Fatalf("RemoveKeepingBranch(bound) = (%#v, %v), want ErrSessionBound", refusalResult, err)
		}
		if f.removeCalls.Load() != 0 || f.mustItem(t).State != StateReady {
			t.Fatal("checkout bound by another session removed")
		}
	})

	t.Run("Should skip teardown when the removal hook denies", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		settings := config.DefaultWorktreesConfig()
		settings.TeardownCommand = "printf should-not-run"
		f.service.getenv = func(string) string { return "/bin/sh" }
		WithConfig(settings, t.TempDir())(f.service)
		WithHooks(removalHookDispatcher{dispatch: func(request HookRequest) (HookVerdict, error) {
			return HookVerdict{Denied: request.Event == EventPreRemove}, nil
		}})(f.service)
		if _, err := f.service.Remove(t.Context(), f.workspace.ID, f.item.ID, true); !errors.Is(err, ErrDeniedByHook) {
			t.Fatalf("Remove = %v", err)
		}
		if f.events.count(EventTeardownOutput) != 0 || f.removeCalls.Load() != 0 {
			t.Fatal("denied removal ran teardown or Git removal")
		}
	})

	t.Run("Should dismiss idempotently and exclude active usage without touching disk", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		settings := config.DefaultWorktreesConfig()
		settings.TeardownCommand = "printf must-not-run"
		WithConfig(settings, t.TempDir())(fixture.service)
		if err := fixture.store.SetState(
			t.Context(),
			fixture.workspace.ID,
			fixture.item.ID,
			StateMissing,
			fixture.item.UpdatedAt,
		); err != nil {
			t.Fatal(err)
		}
		release, acquired := fixture.service.usage.tryAcquireExclusive(
			worktreeUsageKey(fixture.workspace.ID, fixture.item.ID),
		)
		if !acquired {
			t.Fatal("acquire exclusive usage")
		}
		if err := fixture.service.Dismiss(
			t.Context(),
			fixture.workspace.ID,
			fixture.item.ID,
		); !errors.Is(
			err,
			ErrOperationInProgress,
		) {
			t.Fatalf("busy dismissal = %v", err)
		}
		release()
		WithSessionGuard(fixedSessionGuard{active: true})(fixture.service)
		if err := fixture.service.Dismiss(
			t.Context(),
			fixture.workspace.ID,
			fixture.item.ID,
		); !errors.Is(
			err,
			ErrSessionActive,
		) {
			t.Fatalf("active dismissal = %v", err)
		}
		WithSessionGuard(fixedSessionGuard{})(fixture.service)
		for range 2 {
			if err := fixture.service.Dismiss(t.Context(), fixture.workspace.ID, fixture.item.ID); err != nil {
				t.Fatal(err)
			}
		}
		if fixture.events.count(EventDismissed) != 1 || fixture.events.count(EventTeardownOutput) != 0 ||
			len(fixture.runner.invocations()) != 0 {
			t.Fatal("dismissal repeated effects or invoked Git")
		}
		if _, err := os.Stat(fixture.item.Path); err != nil {
			t.Fatalf("checkout changed: %v", err)
		}
	})

	t.Run("Should keep removal idempotent and workspace scoped across terminal states", func(t *testing.T) {
		t.Parallel()
		crossWorkspace := newRemovalTestFixture(t)
		if result, err := crossWorkspace.service.Remove(
			t.Context(), "ws-other", crossWorkspace.item.ID, false,
		); !errors.Is(err, ErrNotFound) || result != nil {
			t.Fatalf("Remove(cross-workspace) = (%#v, %v), want ErrNotFound", result, err)
		}

		removed := newRemovalTestFixture(t)
		if err := removed.store.SetState(
			t.Context(), removed.workspace.ID, removed.item.ID, StateRemoved, removed.item.UpdatedAt,
		); err != nil {
			t.Fatalf("seed removed state: %v", err)
		}
		if result, err := removed.service.Remove(
			t.Context(), removed.workspace.ID, removed.item.ID, false,
		); err != nil || result != nil || len(removed.runner.invocations()) != 0 {
			t.Fatalf(
				"Remove(removed) = (%#v, %v), calls=%#v, want idempotent",
				result,
				err,
				removed.runner.invocations(),
			)
		}

		missing := newRemovalTestFixture(t)
		if err := missing.store.SetState(
			t.Context(), missing.workspace.ID, missing.item.ID, StateMissing, missing.item.UpdatedAt,
		); err != nil {
			t.Fatalf("seed missing state: %v", err)
		}
		if result, err := missing.service.Remove(
			t.Context(), missing.workspace.ID, missing.item.ID, false,
		); !errors.Is(err, ErrNotReady) || result != nil || len(missing.runner.invocations()) != 0 {
			t.Fatalf(
				"Remove(missing) = (%#v, %v), calls=%#v, want ErrNotReady",
				result,
				err,
				missing.runner.invocations(),
			)
		}
	})

	t.Run("Should refuse removal and dismissal when another actor wins the state fence", func(t *testing.T) {
		t.Parallel()
		removal := newRemovalTestFixture(t)
		removal.service.store = &lostStateFenceStore{memoryWorktreeStore: removal.store}
		if result, err := removal.service.Remove(
			t.Context(), removal.workspace.ID, removal.item.ID, false,
		); !errors.Is(err, ErrNotReady) || result != nil || len(removal.runner.invocations()) != 0 {
			t.Fatalf(
				"Remove(lost fence) = (%#v, %v), calls=%#v, want ErrNotReady",
				result,
				err,
				removal.runner.invocations(),
			)
		}

		dismissal := newRemovalTestFixture(t)
		if err := dismissal.store.SetState(
			t.Context(), dismissal.workspace.ID, dismissal.item.ID, StateMissing, dismissal.item.UpdatedAt,
		); err != nil {
			t.Fatalf("seed dismissal target: %v", err)
		}
		dismissal.service.store = &lostStateFenceStore{memoryWorktreeStore: dismissal.store}
		if err := dismissal.service.Dismiss(
			t.Context(),
			dismissal.workspace.ID,
			dismissal.item.ID,
		); !errors.Is(
			err,
			ErrNotReady,
		) {
			t.Fatalf("Dismiss(lost fence) error = %v, want ErrNotReady", err)
		}
	})

	t.Run("Should remove a clean worktree and leave its branch untouched", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		refusalResult, err := fixture.service.Remove(
			t.Context(), fixture.workspace.ID, fixture.item.Name, false,
		)
		if err != nil || refusalResult != nil {
			t.Fatalf("Remove() = %#v, %v, want clean success", refusalResult, err)
		}
		stored, err := fixture.store.Get(t.Context(), fixture.workspace.ID, fixture.item.ID)
		if err != nil || stored.State != StateRemoved {
			t.Fatalf("stored removal = %#v, %v, want removed tombstone", stored, err)
		}
		commands := invocationCommands(fixture.runner.invocations())
		if !containsCommandPrefix(commands, "--git-dir "+fixture.commonDir+" worktree remove ") {
			t.Fatalf("Git calls = %#v, want non-force worktree removal", commands)
		}
		if containsCommandPrefix(commands, "update-ref -d ") {
			t.Fatalf("Git calls = %#v, user branch must survive", commands)
		}
		if got := fixture.events.count(EventRemoved); got != 1 {
			t.Fatalf("removed events = %d, want exactly one", got)
		}
		if err := fixture.service.Dismiss(
			t.Context(), fixture.workspace.ID, fixture.item.Name,
		); err != nil {
			t.Fatalf("Dismiss(name) error = %v", err)
		}
		if got := fixture.mustItem(t).State; got != StateDismissed {
			t.Fatalf("Dismiss(name) state = %q, want dismissed", got)
		}
	})

	t.Run("Should finish the removal record when the request ends while Git deletes", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		ctx, cancel := context.WithCancel(t.Context())
		fixture.onRemove = cancel
		refusalResult, err := fixture.service.Remove(ctx, fixture.workspace.ID, fixture.item.ID, false)
		if err != nil || refusalResult != nil {
			t.Fatalf("Remove(disconnected) = %#v, %v, want completed removal", refusalResult, err)
		}
		if got := fixture.mustItem(t).State; got != StateRemoved {
			t.Fatalf("disconnected removal state = %q, want removed", got)
		}
	})

	t.Run("Should answer a Git removal that stops after deleting .git with the orphan inventory", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		fixture.service.renameCheckout = func(string, string) error { return syscall.EXDEV }
		if err := os.WriteFile(
			filepath.Join(fixture.item.GitDir, "HEAD"),
			[]byte("ref: refs/heads/feature/remove\n"),
			0o600,
		); err != nil {
			t.Fatalf("write admin HEAD: %v", err)
		}
		if err := os.WriteFile(filepath.Join(fixture.item.Path, "left.txt"), []byte("left\n"), 0o600); err != nil {
			t.Fatalf("write leftover: %v", err)
		}
		fixture.removeFails.Store(true)
		fixture.onRemove = func() {
			if err := os.Remove(filepath.Join(fixture.item.Path, ".git")); err != nil {
				t.Errorf("delete .git during removal: %v", err)
			}
		}
		refusalResult, err := fixture.service.Remove(t.Context(), fixture.workspace.ID, fixture.item.ID, false)
		if !errors.Is(err, ErrCheckoutOrphaned) || refusalResult == nil ||
			refusalResult.Code != ErrCheckoutOrphaned.Error() || refusalResult.Risk.LeftoverFiles != 1 {
			t.Fatalf("Remove(interrupted) = %#v, %v, want orphaned refusal with one leftover", refusalResult, err)
		}
		if got := fixture.mustItem(t).State; got != StateReady {
			t.Fatalf("interrupted removal state = %q, want ready", got)
		}
	})

	t.Run("Should refuse dirty state with an inventory and restore the ready fence", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		fixture.dirty.Store(true)
		refusalResult, err := fixture.service.Remove(
			t.Context(), fixture.workspace.ID, fixture.item.ID, false,
		)
		if !errors.Is(err, ErrDirtyRequiresForce) || refusalResult == nil ||
			refusalResult.Risk.ChangedFiles != 1 || refusalResult.Risk.Insertions != 3 ||
			refusalResult.Risk.Deletions != 2 {
			t.Fatalf("Remove(dirty) = %#v, %v, want dirty inventory", refusalResult, err)
		}
		stored, getErr := fixture.store.Get(t.Context(), fixture.workspace.ID, fixture.item.ID)
		if getErr != nil || stored.State != StateReady {
			t.Fatalf("stored refusal = %#v, %v, want ready", stored, getErr)
		}
		if containsCommandPrefix(
			invocationCommands(fixture.runner.invocations()),
			"--git-dir "+fixture.commonDir+" worktree remove ",
		) {
			t.Fatal("dirty refusal invoked destructive Git removal")
		}
	})

	t.Run("Should re-read safety after the hook and refuse a new dirty change", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		fixture.service.hooks = removalHookDispatcher{dispatch: func(request HookRequest) (HookVerdict, error) {
			if request.Event == EventPreRemove {
				fixture.dirty.Store(true)
			}
			return HookVerdict{}, nil
		}}
		refusalResult, err := fixture.service.Remove(
			t.Context(), fixture.workspace.ID, fixture.item.ID, false,
		)
		if !errors.Is(err, ErrDirtyRequiresForce) || refusalResult == nil {
			t.Fatalf("Remove(TOCTOU) = %#v, %v, want dirty refusal", refusalResult, err)
		}
		if containsCommandPrefix(
			invocationCommands(fixture.runner.invocations()),
			"--git-dir "+fixture.commonDir+" worktree remove ",
		) {
			t.Fatal("post-hook dirty state was removed")
		}
	})

	t.Run("Should block an active bound session before mutation", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		WithSessionGuard(fixedSessionGuard{active: true})(fixture.service)
		if _, err := fixture.service.Remove(
			t.Context(), fixture.workspace.ID, fixture.item.ID, true,
		); !errors.Is(err, ErrSessionActive) {
			t.Fatalf("Remove(active session) error = %v, want ErrSessionActive", err)
		}
		stored, err := fixture.store.Get(t.Context(), fixture.workspace.ID, fixture.item.ID)
		if err != nil || stored.State != StateReady {
			t.Fatalf("stored active-session refusal = %#v, %v, want ready", stored, err)
		}
	})

	t.Run("Should block removal while a tool holds a usage lease", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		item, release, err := fixture.service.AcquireUsage(
			t.Context(), fixture.workspace.ID, fixture.item.ID,
		)
		if err != nil {
			t.Fatalf("AcquireUsage() error = %v", err)
		}
		if item == nil || item.ID != fixture.item.ID {
			t.Fatalf("AcquireUsage() item = %#v, want %q", item, fixture.item.ID)
		}
		t.Cleanup(release)

		if _, err := fixture.service.Remove(
			t.Context(), fixture.workspace.ID, fixture.item.ID, true,
		); !errors.Is(err, ErrOperationInProgress) {
			t.Fatalf("Remove(active usage) error = %v, want ErrOperationInProgress", err)
		}
		if got := fixture.mustItem(t).State; got != StateReady {
			t.Fatalf("active usage state = %q, want ready", got)
		}

		release()
		if _, err := fixture.service.Remove(
			t.Context(), fixture.workspace.ID, fixture.item.ID, true,
		); err != nil {
			t.Fatalf("Remove(after release) error = %v", err)
		}
	})

	t.Run("Should preserve the fence when session and status safety checks fail", func(t *testing.T) {
		t.Parallel()
		sessionFailure := newRemovalTestFixture(t)
		WithSessionGuard(fixedSessionGuard{err: errors.New("session store unavailable")})(sessionFailure.service)
		if _, err := sessionFailure.service.Remove(
			t.Context(), sessionFailure.workspace.ID, sessionFailure.item.ID, false,
		); err == nil || !strings.Contains(err.Error(), "inspect active sessions") {
			t.Fatalf("Remove(session check failure) error = %v, want safety failure", err)
		}
		if got := sessionFailure.mustItem(t).State; got != StateReady {
			t.Fatalf("session check failure state = %q, want ready", got)
		}

		statusFailure := newRemovalTestFixture(t)
		baseRun := statusFailure.runner.run
		statusFailure.runner.run = func(call gitInvocation) gitResponse {
			if strings.Join(call.args, " ") == "status --porcelain=v2 --branch -z" {
				return gitResponse{stderr: []byte("cannot read index"), err: errors.New("exit 128")}
			}
			return baseRun(call)
		}
		if _, err := statusFailure.service.Remove(
			t.Context(), statusFailure.workspace.ID, statusFailure.item.ID, false,
		); !errors.Is(err, ErrSafetyCheckFailed) {
			t.Fatalf("Remove(status failure) error = %v, want ErrSafetyCheckFailed", err)
		}
		if got := statusFailure.mustItem(t).State; got != StateReady {
			t.Fatalf("status failure state = %q, want ready", got)
		}
	})

	t.Run("Should refuse unpushed commits unless a remote contains the head", func(t *testing.T) {
		t.Parallel()
		refused := newRemovalTestFixture(t)
		refused.uniqueCommits.Store(2)
		refusalResult, err := refused.service.Remove(
			t.Context(), refused.workspace.ID, refused.item.ID, false,
		)
		if !errors.Is(err, ErrUnpushedRequiresForce) || refusalResult == nil ||
			refusalResult.Risk.UnpushedCommits != 2 {
			t.Fatalf("Remove(unpushed) = (%#v, %v), want unpushed refusal", refusalResult, err)
		}

		contained := newRemovalTestFixture(t)
		contained.uniqueCommits.Store(2)
		contained.remoteContains.Store(true)
		refusalResult, err = contained.service.Remove(
			t.Context(), contained.workspace.ID, contained.item.ID, false,
		)
		if err != nil || refusalResult != nil {
			t.Fatalf("Remove(remote-contained) = (%#v, %v), want success", refusalResult, err)
		}
	})

	t.Run("Should preserve the checkout when patch preservation cannot be verified", func(t *testing.T) {
		t.Parallel()
		for _, oversized := range []bool{false, true} {
			for _, command := range []string{
				"rev-list main..HEAD",
				"for-each-ref --format=%(refname) %(objectname) %(symref) refs/remotes/ refs/heads/",
				"rev-list --cherry-pick --right-only --stdin",
			} {
				t.Run(fmt.Sprintf("Should fail closed for %s oversized=%t", command, oversized), func(t *testing.T) {
					t.Parallel()
					fixture := newRemovalTestFixture(t)
					fixture.uniqueCommits.Store(1)
					baseRun := fixture.runner.run
					fixture.runner.run = func(call gitInvocation) gitResponse {
						if strings.Join(call.args, " ") == command {
							if oversized {
								return gitResponse{stdout: []byte(strings.Repeat("x", maxGitOutputBytes))}
							}
							return gitResponse{stderr: []byte("unreadable refs"), err: errors.New("exit 128")}
						}
						return baseRun(call)
					}
					if _, err := fixture.service.Remove(
						t.Context(),
						fixture.workspace.ID,
						fixture.item.ID,
						false,
					); !errors.Is(
						err,
						ErrSafetyCheckFailed,
					) {
						t.Fatalf("Remove(unverifiable patches) = %v", err)
					}
					if fixture.mustItem(t).State != StateReady || fixture.removeCalls.Load() != 0 {
						t.Fatal("uncertain preservation changed the checkout or removal fence")
					}
				})
			}
		}
	})

	t.Run("Should use force only when explicit and restore ready after Git failure", func(t *testing.T) {
		t.Parallel()
		forced := newRemovalTestFixture(t)
		forced.service.renameCheckout = func(string, string) error { return syscall.EXDEV }
		forced.dirty.Store(true)
		forced.failFirstForcedRemoval.Store(true)
		if refusalResult, err := forced.service.Remove(
			t.Context(), forced.workspace.ID, forced.item.ID, true,
		); err != nil || refusalResult != nil {
			t.Fatalf("Remove(force) = (%#v, %v), want success", refusalResult, err)
		}
		if !containsCommandPrefix(
			invocationCommands(forced.runner.invocations()),
			"--git-dir "+forced.commonDir+" worktree remove --force ",
		) {
			t.Fatal("forced removal omitted the explicit --force flag")
		}
		commands := invocationCommands(forced.runner.invocations())
		if !slices.Contains(
			commands,
			"--git-dir "+forced.commonDir+" worktree remove --force --force "+forced.item.Path,
		) {
			t.Fatalf("forced removal calls = %#v, want Git's double-force locked-worktree retry", commands)
		}

		failed := newRemovalTestFixture(t)
		failed.removeFails.Store(true)
		if _, err := failed.service.Remove(
			t.Context(), failed.workspace.ID, failed.item.ID, false,
		); !errors.Is(err, ErrRemovalFailed) {
			t.Fatalf("Remove(Git failure) error = %v, want ErrRemovalFailed", err)
		}
		stored, err := failed.store.Get(t.Context(), failed.workspace.ID, failed.item.ID)
		if err != nil || stored.State != StateReady {
			t.Fatalf("stored Git failure = %#v, %v, want retriable ready", stored, err)
		}
	})

	t.Run("Should reclaim only an unchanged runtime-owned branch", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		fixture.item.Branch = "run/remove"
		fixture.item.CreatedBranch = true
		fixture.item.RunNamespace = "run/"
		fixture.item.CreatedHead = "base-head"
		fixture.store.mu.Lock()
		fixture.store.items[worktreeStoreKey(fixture.workspace.ID, fixture.item.ID)] = fixture.item
		fixture.store.mu.Unlock()
		if refusalResult, err := fixture.service.Remove(
			t.Context(), fixture.workspace.ID, fixture.item.ID, false,
		); err != nil || refusalResult != nil {
			t.Fatalf("Remove(runtime-owned) = (%#v, %v), want success", refusalResult, err)
		}
		if !containsCommandPrefix(
			invocationCommands(fixture.runner.invocations()),
			"update-ref -d refs/heads/run/remove base-head",
		) {
			t.Fatal("runtime-owned branch was not compare-deleted")
		}
		if got := fixture.events.count(EventBranchReclaimed); got != 1 {
			t.Fatalf("branch reclaimed events = %d, want one", got)
		}
	})

	t.Run("Should recover pending phases from durable ownership checkpoints", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		branchPending := fixture.item
		branchPending.ID, branchPending.Name = "wt-branch", "branch-recovery"
		branchPending.Path = t.TempDir()
		branchPending.State, branchPending.PendingPhase = StatePending, PhaseBranch
		branchPending.CreatedBranch, branchPending.CreatedHead = true, "base-head"
		branchPending.Branch = "run/recover"
		copyPending := fixture.item
		copyPending.ID, copyPending.Name = "wt-copy", "copy-recovery"
		copyPending.Path, copyPending.GitDir = linkTestCheckout(t, fixture.commonDir, "copy-recovery")
		copyPending.State, copyPending.PendingPhase = StatePending, PhaseCopy
		setupPending := fixture.item
		setupPending.ID, setupPending.Name = "wt-setup", "setup-recovery"
		setupPending.Path, setupPending.GitDir = linkTestCheckout(t, fixture.commonDir, "setup-recovery")
		setupPending.State, setupPending.PendingPhase = StatePending, PhaseSetup
		if err := fixture.store.Insert(t.Context(), branchPending); err != nil {
			t.Fatalf("seed branch recovery: %v", err)
		}
		if err := fixture.store.Insert(t.Context(), copyPending); err != nil {
			t.Fatalf("seed copy recovery: %v", err)
		}
		if err := fixture.store.Insert(t.Context(), setupPending); err != nil {
			t.Fatalf("seed setup recovery: %v", err)
		}
		if err := fixture.service.RecoverCreations(t.Context()); err != nil {
			t.Fatalf("RecoverCreations() error = %v", err)
		}
		branchStored, branchErr := fixture.store.Get(t.Context(), fixture.workspace.ID, branchPending.ID)
		copyStored, copyErr := fixture.store.Get(t.Context(), fixture.workspace.ID, copyPending.ID)
		setupStored, setupErr := fixture.store.Get(t.Context(), fixture.workspace.ID, setupPending.ID)
		if branchErr != nil || branchStored.State != StateFailed {
			t.Fatalf("branch recovery = %#v, %v, want failed", branchStored, branchErr)
		}
		if copyErr != nil || copyStored.State != StateReady || copyStored.SetupState != SetupFailed ||
			!strings.Contains(copyStored.SetupError, "daemon restart") {
			t.Fatalf("copy recovery = %#v, %v, want usable setup failure", copyStored, copyErr)
		}
		if setupErr != nil || setupStored.State != StateReady || setupStored.SetupState != SetupFailed ||
			!strings.Contains(setupStored.SetupError, "daemon restart") {
			t.Fatalf("setup recovery = %#v, %v, want usable setup failure", setupStored, setupErr)
		}
		if !containsCommandPrefix(
			invocationCommands(fixture.runner.invocations()),
			"update-ref -d refs/heads/run/recover base-head",
		) {
			t.Fatal("branch-phase recovery did not compare-delete the recorded branch")
		}
		if got := fixture.events.count(EventSetupFailed); got != 2 {
			t.Fatalf("setup failed events = %d, want two", got)
		}
	})

	for _, testCase := range []struct {
		name        string
		checkout    func(t *testing.T, fixture *removalTestFixture) (string, string)
		leftInPlace bool
	}{
		{name: "is gone", checkout: func(t *testing.T, fixture *removalTestFixture) (string, string) {
			return filepath.Join(t.TempDir(), "gone"), fixture.item.GitDir
		}},
		{
			name: "is no longer its linked checkout",
			checkout: func(t *testing.T, fixture *removalTestFixture) (string, string) {
				return t.TempDir(), fixture.item.GitDir
			},
			leftInPlace: true,
		},
		{
			name: "was switched to another branch",
			checkout: func(t *testing.T, fixture *removalTestFixture) (string, string) {
				path, gitDir := linkTestCheckout(t, fixture.commonDir, "switched")
				fixture.switchedCheckout = path
				return path, gitDir
			},
			leftInPlace: true,
		},
	} {
		t.Run("Should fail an interrupted copy phase whose checkout "+testCase.name, func(t *testing.T) {
			t.Parallel()
			fixture := newRemovalTestFixture(t)
			gone := fixture.item
			gone.ID, gone.Name = "wt-gone", "gone-recovery"
			gone.Path, gone.GitDir = testCase.checkout(t, fixture)
			gone.State, gone.PendingPhase = StatePending, PhaseCopy
			if err := fixture.store.Insert(t.Context(), gone); err != nil {
				t.Fatalf("seed interrupted copy: %v", err)
			}
			if err := fixture.service.RecoverCreations(t.Context()); err != nil {
				t.Fatalf("RecoverCreations() error = %v", err)
			}
			stored, err := fixture.store.Get(t.Context(), fixture.workspace.ID, gone.ID)
			if err != nil || stored.State != StateFailed ||
				!strings.Contains(stored.CreateError, "daemon restart") {
				t.Fatalf("interrupted copy = %#v, %v, want failed record instead of a ready one", stored, err)
			}
			if testCase.leftInPlace {
				if _, statErr := os.Stat(gone.Path); statErr != nil ||
					!strings.Contains(stored.CreateRollbackError, "left in place") {
					t.Fatalf(
						"replacement = %v, %q, want it left in place and reported",
						statErr,
						stored.CreateRollbackError,
					)
				}
				for _, command := range invocationCommands(fixture.runner.invocations()) {
					if strings.Contains(command, "worktree remove") {
						t.Fatalf("recovery ran %q against a checkout it does not own", command)
					}
				}
			}
			if got := fixture.events.count(EventSetupFailed); got != 0 {
				t.Fatalf("setup failed events = %d, want none for an unusable checkout", got)
			}
		})
	}

	for _, testCase := range []struct {
		name   string
		appear func(path string) error
	}{
		{name: "directory", appear: func(path string) error { return os.MkdirAll(path, 0o700) }},
		// A dangling symlink is visible to Lstat but not to Stat.
		{name: "dangling symlink", appear: func(path string) error {
			return os.Symlink(filepath.Join(filepath.Dir(path), "missing-target"), path)
		}},
	} {
		t.Run("Should leave a "+testCase.name+" that appears while recovery waits for the lock", func(t *testing.T) {
			t.Parallel()
			fixture := newRemovalTestFixture(t)
			appearing := fixture.item
			appearing.ID, appearing.Name = "wt-appearing", "appearing-recovery"
			appearing.Path = filepath.Join(t.TempDir(), "appearing")
			appearing.State, appearing.PendingPhase = StatePending, PhaseCopy
			if err := fixture.store.Insert(t.Context(), appearing); err != nil {
				t.Fatalf("seed interrupted copy: %v", err)
			}
			release, err := fixture.service.locks.Acquire(t.Context(), fixture.commonDir)
			if err != nil {
				t.Fatalf("hold repository lock: %v", err)
			}
			recovered := make(chan error, 1)
			go func() { recovered <- fixture.service.RecoverCreations(t.Context()) }()
			// Recovery resolves the common dir right before it waits for the lock.
			deadline := time.Now().Add(5 * time.Second)
			for !slices.Contains(invocationCommands(fixture.runner.invocations()),
				"rev-parse --path-format=absolute --git-common-dir") {
				if time.Now().After(deadline) {
					release()
					t.Fatal("recovery never reached the repository lock")
				}
				time.Sleep(5 * time.Millisecond)
			}
			if err := testCase.appear(appearing.Path); err != nil {
				release()
				t.Fatalf("create appearing checkout: %v", err)
			}
			release()
			if err := <-recovered; err != nil {
				t.Fatalf("RecoverCreations() error = %v", err)
			}
			stored, err := fixture.store.Get(t.Context(), fixture.workspace.ID, appearing.ID)
			if err != nil || stored.State != StatePending {
				t.Fatalf("appearing recovery = %#v, %v, want pending for the next boot", stored, err)
			}
			if _, err := os.Lstat(appearing.Path); err != nil {
				t.Fatalf("Lstat(appearing checkout) error = %v, want preserved", err)
			}
			for _, command := range invocationCommands(fixture.runner.invocations()) {
				if strings.Contains(command, "worktree remove") || strings.HasPrefix(command, "update-ref -d ") {
					t.Fatalf("recovery ran %q against a checkout that appeared meanwhile", command)
				}
			}
		})
	}

	t.Run("Should keep an interrupted copy phase pending while Git is unavailable", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		fixture.service.capability = &CapabilityGate{
			cached: true, result: CapabilityDiagnostic{Code: ErrGitUnavailable.Error()}, err: ErrGitUnavailable,
		}
		copyPending := fixture.item
		copyPending.ID, copyPending.Name = "wt-copy", "copy-recovery"
		copyPending.Path, copyPending.GitDir = linkTestCheckout(t, fixture.commonDir, "copy-recovery")
		copyPending.State, copyPending.PendingPhase = StatePending, PhaseCopy
		if err := fixture.store.Insert(t.Context(), copyPending); err != nil {
			t.Fatalf("seed copy recovery: %v", err)
		}
		if err := fixture.service.RecoverCreations(t.Context()); err != nil {
			t.Fatalf("RecoverCreations() error = %v", err)
		}
		stored, err := fixture.store.Get(t.Context(), fixture.workspace.ID, copyPending.ID)
		if err != nil || stored.State != StatePending {
			t.Fatalf("copy recovery without Git = %#v, %v, want pending until its identity can be checked", stored, err)
		}
	})

	t.Run("Should resume a stale removal and fail interrupted exit operations", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		op := ExitOperation{
			ID: "op-interrupted", WorkspaceID: fixture.workspace.ID, WorktreeID: fixture.item.ID,
			Action: "push", State: "running", StartedAt: fixture.item.CreatedAt,
		}
		if err := fixture.store.InsertExitOperation(t.Context(), op); err != nil {
			t.Fatalf("seed running exit operation: %v", err)
		}
		if err := fixture.store.SetState(
			t.Context(), fixture.workspace.ID, fixture.item.ID, StateRemoving, fixture.item.UpdatedAt,
		); err != nil {
			t.Fatalf("seed removing state: %v", err)
		}
		if err := fixture.service.RecoverCreations(t.Context()); err != nil {
			t.Fatalf("RecoverCreations() error = %v", err)
		}
		if got := fixture.mustItem(t).State; got != StateRemoved {
			t.Fatalf("stale removing state = %q, want removed", got)
		}
		fixture.store.mu.Lock()
		recoveredOp := fixture.store.exitOps[op.ID]
		fixture.store.mu.Unlock()
		if recoveredOp.State != "failed" || recoveredOp.FinishedAt == nil {
			t.Fatalf("interrupted exit operation = %#v, want failed with finished_at", recoveredOp)
		}
	})

	t.Run("Should finish an interrupted keep-branch removal without the unpushed-commit refusal", func(t *testing.T) {
		t.Parallel()
		f := newRemovalTestFixture(t)
		f.item.Branch = "run/remove"
		f.item.CreatedBranch = true
		f.item.RunNamespace = "run/"
		f.item.CreatedHead = "base-head"
		f.store.mu.Lock()
		f.store.items[worktreeStoreKey(f.workspace.ID, f.item.ID)] = f.item
		f.store.mu.Unlock()
		f.uniqueCommits.Store(2)
		fenced, err := f.store.FenceRemoval(t.Context(), f.workspace.ID, f.item.ID, true, f.item.UpdatedAt)
		if err != nil || !fenced {
			t.Fatalf("FenceRemoval(keep branch) = %t, %v", fenced, err)
		}
		if err := f.service.RecoverCreations(t.Context()); err != nil {
			t.Fatalf("RecoverCreations() error = %v", err)
		}
		if got := f.mustItem(t).State; got != StateRemoved {
			t.Fatalf("interrupted keep-branch removal = %q, want removed", got)
		}
		if containsCommandPrefix(invocationCommands(f.runner.invocations()), "update-ref -d ") {
			t.Fatal("recovery reclaimed the branch a keep-branch removal keeps")
		}
	})

	t.Run("Should leave worktree rows untouched when the parent workspace root vanished", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		if err := fixture.store.SetState(
			t.Context(), fixture.workspace.ID, fixture.item.ID, StateMissing, fixture.item.UpdatedAt,
		); err != nil {
			t.Fatalf("seed missing row: %v", err)
		}
		vanishedRoot := filepath.Join(t.TempDir(), "vanished")
		fixture.workspace.Root = vanishedRoot
		fixture.service.resolver = staticWorkspaceResolver{workspaces: map[string]Workspace{
			fixture.workspace.ID: fixture.workspace,
		}}
		if err := fixture.service.RecoverCreations(t.Context()); err != nil {
			t.Fatalf("RecoverCreations(vanished workspace) error = %v", err)
		}
		if got := fixture.mustItem(t).State; got != StateMissing {
			t.Fatalf("vanished-workspace row state = %q, want unchanged missing", got)
		}
	})

	t.Run("Should restore only matching missing identities and dismiss tombstones", func(t *testing.T) {
		t.Parallel()
		matching := newRemovalTestFixture(t)
		if err := matching.store.SetState(
			t.Context(), matching.workspace.ID, matching.item.ID, StateMissing, matching.item.UpdatedAt,
		); err != nil {
			t.Fatalf("SetState(missing) error = %v", err)
		}
		if err := matching.service.RecoverCreations(t.Context()); err != nil {
			t.Fatalf("RecoverCreations(matching missing) error = %v", err)
		}
		if got := matching.mustItem(t).State; got != StateReady {
			t.Fatalf("matching missing state = %q, want ready", got)
		}

		dismissed := newRemovalTestFixture(t)
		known := 3
		if err := dismissed.store.SaveStatus(t.Context(), dismissed.workspace.ID, dismissed.item.ID, Status{
			WorktreeID: dismissed.item.ID, DirtyFiles: &known,
		}); err != nil {
			t.Fatalf("seed status history: %v", err)
		}
		op := ExitOperation{
			ID: "op-dismiss-history", WorkspaceID: dismissed.workspace.ID, WorktreeID: dismissed.item.ID,
			Action: "push", State: "completed", StartedAt: dismissed.item.CreatedAt,
		}
		if err := dismissed.store.InsertExitOperation(t.Context(), op); err != nil {
			t.Fatalf("seed operation history: %v", err)
		}
		if err := dismissed.store.SetState(
			t.Context(), dismissed.workspace.ID, dismissed.item.ID, StateMissing, dismissed.item.UpdatedAt,
		); err != nil {
			t.Fatalf("SetState(dismiss target) error = %v", err)
		}
		if err := dismissed.service.Dismiss(
			t.Context(), dismissed.workspace.ID, dismissed.item.ID,
		); err != nil {
			t.Fatalf("Dismiss() error = %v", err)
		}
		if got := dismissed.mustItem(t).State; got != StateDismissed {
			t.Fatalf("dismissed state = %q, want dismissed", got)
		}
		if got := dismissed.events.count(EventDismissed); got != 1 {
			t.Fatalf("dismissed events = %d, want one", got)
		}
		status, statusErr := dismissed.store.GetStatus(t.Context(), dismissed.workspace.ID, dismissed.item.ID)
		if statusErr != nil || status == nil || status.DirtyFiles == nil || *status.DirtyFiles != known {
			t.Fatalf("dismissed status history = %#v, %v, want preserved", status, statusErr)
		}
		dismissed.store.mu.Lock()
		preservedOp, ok := dismissed.store.exitOps[op.ID]
		dismissed.store.mu.Unlock()
		if !ok || preservedOp.WorktreeID != dismissed.item.ID {
			t.Fatalf("dismissed operation history = %#v, want preserved", preservedOp)
		}
	})

	t.Run("Should dismiss only terminal tombstones inside the owning workspace", func(t *testing.T) {
		t.Parallel()
		for _, state := range []State{StateFailed, StateRemoved} {
			t.Run("Should dismiss "+string(state), func(t *testing.T) {
				t.Parallel()
				fixture := newRemovalTestFixture(t)
				if err := fixture.store.SetState(
					t.Context(), fixture.workspace.ID, fixture.item.ID, state, fixture.item.UpdatedAt,
				); err != nil {
					t.Fatalf("seed %s state: %v", state, err)
				}
				if err := fixture.service.Dismiss(
					t.Context(),
					fixture.workspace.ID,
					fixture.item.ID,
				); err != nil {
					t.Fatalf("Dismiss(%s) error = %v", state, err)
				}
				if got := fixture.mustItem(t).State; got != StateDismissed {
					t.Fatalf("Dismiss(%s) state = %q, want dismissed", state, got)
				}
			})
		}

		ready := newRemovalTestFixture(t)
		if err := ready.service.Dismiss(
			t.Context(),
			ready.workspace.ID,
			ready.item.ID,
		); !errors.Is(
			err,
			ErrNotReady,
		) {
			t.Fatalf("Dismiss(ready) error = %v, want ErrNotReady", err)
		}
		if err := ready.service.Dismiss(t.Context(), "ws-other", ready.item.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Dismiss(cross-workspace) error = %v, want ErrNotFound", err)
		}
	})
}

type removalTestFixture struct {
	service                *Service
	store                  *memoryWorktreeStore
	runner                 *recordingGitRunner
	events                 *recordingEventSink
	workspace              Workspace
	item                   Worktree
	commonDir              string
	dirty                  atomicBool
	uniqueCommits          atomicInt
	remoteContains         atomicBool
	removeFails            atomicBool
	failFirstForcedRemoval atomicBool
	removeCalls            atomicInt
	// switchedCheckout names a checkout whose HEAD reads as another branch.
	switchedCheckout string
	// onRemove runs while Git deletes the checkout, e.g. to drop the request.
	onRemove func()
}

type lostStateFenceStore struct {
	*memoryWorktreeStore
}

func (s *lostStateFenceStore) CompareAndSwapState(
	context.Context,
	string,
	string,
	State,
	State,
	time.Time,
) (bool, error) {
	return false, nil
}

func (s *lostStateFenceStore) FenceRemoval(context.Context, string, string, bool, time.Time) (bool, error) {
	return false, nil
}

type atomicBool struct {
	mu    sync.Mutex
	value bool
}

func (b *atomicBool) Store(value bool) {
	b.mu.Lock()
	b.value = value
	b.mu.Unlock()
}

func (b *atomicBool) Load() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.value
}

type atomicInt struct {
	mu    sync.Mutex
	value int
}

func (v *atomicInt) Store(value int) {
	v.mu.Lock()
	v.value = value
	v.mu.Unlock()
}

func (v *atomicInt) Load() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.value
}

// linkTestCheckout writes the .git pointer and admin entry Git keeps for a
// linked checkout, returning the canonical checkout path and admin directory.
func linkTestCheckout(t *testing.T, commonDir, name string) (string, string) {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("canonicalize linked checkout: %v", err)
	}
	adminGitDir := filepath.Join(commonDir, "worktrees", name)
	if err := os.MkdirAll(adminGitDir, 0o700); err != nil {
		t.Fatalf("create linked admin dir: %v", err)
	}
	for file, content := range map[string]string{
		filepath.Join(path, ".git"):             "gitdir: " + adminGitDir + "\n",
		filepath.Join(adminGitDir, "commondir"): "../..\n",
		filepath.Join(adminGitDir, "gitdir"):    filepath.Join(path, ".git") + "\n",
	} {
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatalf("write linked checkout metadata: %v", err)
		}
	}
	return path, adminGitDir
}

func newRemovalTestFixture(t *testing.T) *removalTestFixture {
	t.Helper()
	workspaceRoot := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		t.Fatalf("canonicalize workspace root: %v", err)
	}
	commonDir := filepath.Join(canonicalRoot, ".git")
	pathRaw := t.TempDir()
	path, err := filepath.EvalSymlinks(pathRaw)
	if err != nil {
		t.Fatalf("canonicalize worktree path: %v", err)
	}
	adminGitDir := filepath.Join(commonDir, "worktrees", "removal")
	if err := os.MkdirAll(adminGitDir, 0o700); err != nil {
		t.Fatalf("create worktree admin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: "+adminGitDir+"\n"), 0o600); err != nil {
		t.Fatalf("write worktree pointer: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adminGitDir, "commondir"), []byte("../..\n"), 0o600); err != nil {
		t.Fatalf("write common pointer: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(adminGitDir, "gitdir"),
		[]byte(filepath.Join(path, ".git")+"\n"),
		0o600,
	); err != nil {
		t.Fatalf("write worktree backlink: %v", err)
	}
	workspace := Workspace{ID: "ws-remove", Name: "Remove", Root: canonicalRoot}
	now := time.Date(2026, 8, 12, 16, 0, 0, 0, time.UTC)
	item := Worktree{
		ID: "wt-remove", RunID: "run-remove", WorkspaceID: workspace.ID, Name: "remove", Branch: "feature/remove",
		Path: path, GitDir: adminGitDir, State: StateReady, Origin: OriginManual,
		SetupState: SetupNone, BaseRef: "main", CreatedAt: now, UpdatedAt: now,
	}
	fixture := &removalTestFixture{workspace: workspace, item: item, commonDir: commonDir}
	runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
		command := strings.Join(call.args, " ")
		switch {
		case command == "rev-parse --path-format=absolute --git-common-dir":
			return gitResponse{stdout: []byte(commonDir + "\n")}
		case command == "rev-parse --abbrev-ref HEAD":
			if call.dir == fixture.switchedCheckout {
				return gitResponse{stdout: []byte("someone-else\n")}
			}
			return gitResponse{stdout: []byte(fixture.item.Branch + "\n")}
		case command == "status --porcelain=v2 --branch -z":
			status := "# branch.oid head\x00# branch.head " + fixture.item.Branch + "\x00"
			if fixture.dirty.Load() {
				status += "? changed.txt\x00"
			}
			return gitResponse{stdout: []byte(status)}
		case command == "diff --numstat -z":
			if fixture.dirty.Load() {
				return gitResponse{stdout: []byte("3\t2\tchanged.txt\x00")}
			}
			return gitResponse{}
		case command == "diff --cached --numstat -z":
			return gitResponse{}
		case command == "rev-parse --verify --quiet main^{commit}":
			return gitResponse{stdout: []byte("base\n")}
		case command == "rev-list --count main..HEAD":
			return gitResponse{stdout: []byte(strconv.Itoa(fixture.uniqueCommits.Load()) + "\n")}
		case command == "rev-list main..HEAD" || command == "rev-list --cherry-pick --right-only --stdin":
			var hashes strings.Builder
			for i := range fixture.uniqueCommits.Load() {
				hashes.WriteString(strconv.Itoa(i) + "\n")
			}
			return gitResponse{stdout: []byte(hashes.String())}
		case command == "for-each-ref --format=%(refname) %(objectname) %(symref) refs/remotes/ refs/heads/":
			return gitResponse{}
		case command == "branch -r --contains HEAD":
			if fixture.remoteContains.Load() {
				return gitResponse{stdout: []byte("  origin/feature/remove\n")}
			}
			return gitResponse{}
		case strings.HasPrefix(command, "--git-dir "+commonDir+" worktree remove "):
			fixture.removeCalls.Store(fixture.removeCalls.Load() + 1)
			if fixture.onRemove != nil {
				fixture.onRemove()
			}
			if fixture.removeFails.Load() ||
				(fixture.failFirstForcedRemoval.Load() && fixture.removeCalls.Load() == 1) {
				return gitResponse{stderr: []byte("resource busy"), err: errors.New("exit 1")}
			}
			if err := os.RemoveAll(path); err != nil {
				return gitResponse{err: err}
			}
			if err := os.RemoveAll(adminGitDir); err != nil {
				return gitResponse{err: err}
			}
			return gitResponse{}
		case strings.HasPrefix(command, "update-ref -d "):
			return gitResponse{}
		default:
			return gitResponse{err: errors.New("unexpected removal Git command: " + command)}
		}
	}}
	store := newMemoryWorktreeStore()
	if err := store.Insert(t.Context(), item); err != nil {
		t.Fatalf("seed removal worktree: %v", err)
	}
	events := &recordingEventSink{}
	service := NewService(
		store, runner, WithCapabilityGate(readyCapabilityGate()),
		WithWorkspaceResolver(staticWorkspaceResolver{workspaces: map[string]Workspace{workspace.ID: workspace}}),
		WithConfig(config.DefaultWorktreesConfig(), t.TempDir()), WithEvents(events),
		WithClock(func() time.Time { return now }),
	)
	fixture.service, fixture.store, fixture.runner, fixture.events = service, store, runner, events
	return fixture
}

func (f *removalTestFixture) mustItem(t *testing.T) Worktree {
	t.Helper()
	item, err := f.store.Get(t.Context(), f.workspace.ID, f.item.ID)
	if err != nil || item == nil {
		t.Fatalf("store.Get() = (%#v, %v)", item, err)
	}
	return *item
}

type removalHookDispatcher struct {
	dispatch func(HookRequest) (HookVerdict, error)
}

func (d removalHookDispatcher) DispatchWorktreeHook(
	_ context.Context,
	request HookRequest,
) (HookVerdict, error) {
	return d.dispatch(request)
}

type fixedSessionGuard struct {
	active bool
	bound  bool
	err    error
}

func (g fixedSessionGuard) HasActiveSession(context.Context, string, string) (bool, error) {
	return g.active, g.err
}

func (g fixedSessionGuard) HasUnarchivedSession(context.Context, string, string) (bool, error) {
	return g.bound, g.err
}

type recordingEventSink struct {
	mu      sync.Mutex
	events  []LifecycleEvent
	onEvent func(LifecycleEvent)
}

func (s *recordingEventSink) PublishWorktreeEvent(_ context.Context, event LifecycleEvent) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	if s.onEvent != nil {
		s.onEvent(event)
	}
	return nil
}

func (s *recordingEventSink) count(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, event := range s.events {
		if event.Name == name {
			count++
		}
	}
	return count
}

type removalTrashRootsResolver struct {
	staticWorkspaceResolver
	roots  []string
	err    error
	onList func()
}

func (r removalTrashRootsResolver) ListWorktreeTrashRoots(context.Context) ([]string, error) {
	if r.onList != nil {
		r.onList()
	}
	return r.roots, r.err
}
