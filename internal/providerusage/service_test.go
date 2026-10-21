package providerusage

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

type serviceFixture struct {
	service *Manager
	catalog *TargetCatalog
	clock   *clockwork.FakeClock
	target  Target
}

func usageFixture(t *testing.T, handler http.HandlerFunc) serviceFixture {
	t.Helper()
	clock := fixtureClock(t)
	target := codexTarget(t, "account-a", clock.Now().Add(24*time.Hour))
	server := fixtureServer(t, handler)
	catalog := &TargetCatalog{}
	catalog.Replace("personal", []Target{target})
	service := NewService(
		Options{
			Resolver: catalog,
			Collectors: DefaultRegistry(
				RegistryOptions{BaseURLs: map[string]string{codexProviderID: server.URL}, Now: clock.Now},
			),
			Clock: clock,
		},
	)
	t.Cleanup(func() {
		if err := service.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return serviceFixture{service, catalog, clock, target}
}
func refreshFixture(t *testing.T, f serviceFixture, intent Intent) Envelope {
	t.Helper()
	env, err := f.service.Refresh(
		t.Context(),
		RefreshRequest{Wait: 15 * time.Second, ProfileID: "personal", Intent: intent},
	)
	requireNoError(t, err)
	return env
}
func writeFixture(w http.ResponseWriter, t *testing.T, body string) {
	t.Helper()
	if _, err := w.Write([]byte(body)); err != nil {
		t.Error(err)
	}
}

func TestUsageService(t *testing.T) {
	t.Run("Should advertise the daemon host Keychain capability even when disabled", func(t *testing.T) {
		t.Parallel()
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) { writeFixture(w, t, codexHTTPFixture) })
		for _, enabled := range []bool{true, false} {
			requireNoError(t, f.service.SetEnabled(t.Context(), enabled))
			env, err := f.service.Envelope(t.Context(), "personal")
			requireNoError(t, err)
			requireEqual(t, runtime.GOOS == "darwin", env.KeychainSupported)
		}
	})

	t.Run("Should coalesce concurrent flights and enforce success based freshness floors", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		entered := make(chan struct{})
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) == 1 {
				close(entered)
				<-release
			}
			writeFixture(w, t, codexHTTPFixture)
		})
		results := make(chan Envelope, 2)
		for range 2 {
			go func() {
				env, err := f.service.Refresh(
					t.Context(),
					RefreshRequest{Wait: 15 * time.Second, ProfileID: "personal"},
				)
				if err != nil {
					t.Error(err)
				}
				results <- env
			}()
		}
		<-entered
		once.Do(func() { close(release) })
		a, b := <-results, <-results
		requireEqual(t, StatusOK, a.Targets[0].Status)
		requireEqual(t, StatusOK, b.Targets[0].Status)
		requireEqual(t, int32(1), calls.Load())
		f.clock.Advance(10 * time.Second)
		refreshFixture(t, f, IntentExplicit)
		requireEqual(t, int32(1), calls.Load())
		f.clock.Advance(80 * time.Second)
		_, err := f.service.Refresh(
			t.Context(),
			RefreshRequest{Wait: 15 * time.Second, ProfileID: "personal", MaxAge: 2 * time.Minute},
		)
		requireNoError(t, err)
		requireEqual(t, int32(1), calls.Load())
		f.clock.Advance(31 * time.Second)
		refreshFixture(t, f, IntentBackground)
		requireEqual(t, int32(2), calls.Load())
	})
	t.Run("Should retain same account stale values without restamping and drop replaced accounts", func(t *testing.T) {
		t.Parallel()
		var failure atomic.Bool
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			if failure.Load() {
				w.WriteHeader(503)
			}
			writeFixture(w, t, codexHTTPFixture)
		})
		first := refreshFixture(t, f, IntentBackground)
		f.clock.Advance(time.Minute)
		failure.Store(true)
		stale := refreshFixture(t, f, IntentBackground)
		requireEqual(t, StatusStale, stale.Targets[0].Status)
		requireEqual(t, *first.Targets[0].MeasuredAt, *stale.Targets[0].MeasuredAt)
		requireEqual(t, 40.0, *stale.Targets[0].Windows[0].UsedPercent)
		requireNoError(
			t,
			os.WriteFile(
				filepath.Join(codexHome(f.target), "auth.json"),
				[]byte(fixtureAuth(t, "account-b", f.clock.Now().Add(time.Hour))),
				0o600,
			),
		)
		changed := refreshFixture(t, f, IntentBackground)
		requireEqual(t, StatusError, changed.Targets[0].Status)
		requireEqual(t, 0, len(changed.Targets[0].Windows))
		requireEqual(t, (*Account)(nil), changed.Targets[0].Account)
	})
	t.Run("Should age only beyond three intervals copy reads and isolate profiles", func(t *testing.T) {
		t.Parallel()
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) { writeFixture(w, t, codexHTTPFixture) })
		work, err := f.service.Envelope(t.Context(), "work")
		requireNoError(t, err)
		cold, err := f.service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		requireEqual(t, StatusRefreshing, cold.Targets[0].Status)
		first := refreshFixture(t, f, IntentBackground)
		*first.Targets[0].Windows[0].UsedPercent = 99
		first.Targets[0].Account.Plan = "changed"
		f.clock.Advance(15 * time.Minute)
		at, err := f.service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		requireEqual(t, StatusOK, at.Targets[0].Status)
		requireEqual(t, 40.0, *at.Targets[0].Windows[0].UsedPercent)
		requireEqual(t, "pro", at.Targets[0].Account.Plan)
		f.clock.Advance(time.Nanosecond)
		aged, err := f.service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		requireEqual(t, StatusStale, aged.Targets[0].Status)
		after, err := f.service.Envelope(t.Context(), "work")
		requireNoError(t, err)
		requireEqual(t, work.Revision, after.Revision)
	})
	t.Run("Should discard in flight ownership changes before publishing", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		entered := make(chan struct{})
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		f := usageFixture(t, func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				close(entered)
				<-release
				writeFixture(w, t, codexHTTPFixture)
				return
			}
			requireEqual(t, "account-b", r.Header.Get("ChatGPT-Account-Id"))
			writeFixture(w, t, strings.ReplaceAll(codexHTTPFixture, `"used_percent":40`, `"used_percent":84`))
		})
		done := make(chan Envelope, 1)
		go func() {
			env, err := f.service.Refresh(
				t.Context(),
				RefreshRequest{Wait: 15 * time.Second, ProfileID: "personal"},
			)
			if err != nil {
				t.Error(err)
			}
			done <- env
		}()
		<-entered
		requireNoError(
			t,
			os.WriteFile(
				filepath.Join(codexHome(f.target), "auth.json"),
				[]byte(fixtureAuth(t, "account-b", f.clock.Now().Add(time.Hour))),
				0o600,
			),
		)
		once.Do(func() { close(release) })
		env := <-done
		requireEqual(t, int32(2), calls.Load())
		requireEqual(t, 84.0, *env.Targets[0].Windows[0].UsedPercent)
	})
	t.Run("Should honor identity cooldowns and permit only one explicit bypass until success", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(429)
		})
		refreshFixture(t, f, IntentBackground)
		refreshFixture(t, f, IntentBackground)
		requireEqual(t, int32(1), calls.Load())
		refreshFixture(t, f, IntentExplicit)
		refreshFixture(t, f, IntentExplicit)
		requireEqual(t, int32(2), calls.Load())
		requireNoError(
			t,
			os.WriteFile(
				filepath.Join(codexHome(f.target), "auth.json"),
				[]byte(fixtureAuth(t, "account-b", f.clock.Now().Add(time.Hour))),
				0o600,
			),
		)
		refreshFixture(t, f, IntentBackground)
		requireEqual(t, int32(3), calls.Load())
	})
	t.Run("Should publish late after caller cancellation and coalesce revision notifications", func(t *testing.T) {
		t.Parallel()
		entered := make(chan struct{})
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			close(entered)
			<-release
			writeFixture(w, t, codexHTTPFixture)
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan struct{})
		revisions, unsubscribe := f.service.Subscribe("personal")
		defer unsubscribe()
		go func() {
			_, err := f.service.Refresh(ctx, RefreshRequest{Wait: 15 * time.Second, ProfileID: "personal"})
			if err != context.Canceled {
				t.Errorf("expected cancellation, got %v", err)
			}
			close(done)
		}()
		<-entered
		cancel()
		<-done
		waiting, err := f.service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		requireEqual(t, true, waiting.Targets[0].Refreshing)
		once.Do(func() { close(release) })
		for {
			select {
			case revision := <-revisions:
				env, err := f.service.Envelope(t.Context(), "personal")
				requireNoError(t, err)
				if !env.Targets[0].Refreshing && env.Targets[0].Status == StatusOK {
					if revision.Revision <= waiting.Revision {
						continue
					}
					return
				}
			case <-t.Context().Done():
				t.Fatal("completion not published")
			}
		}
	})
	t.Run("Should cancel collection and clear snapshots on disable without late publication", func(t *testing.T) {
		t.Parallel()
		entered := make(chan struct{})
		f := usageFixture(t, func(_ http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() })
		done := make(chan struct{})
		go func() {
			if _, err := f.service.Refresh(
				t.Context(),
				RefreshRequest{Wait: 15 * time.Second, ProfileID: "personal"},
			); err != nil {
				t.Error(err)
			}
			close(done)
		}()
		<-entered
		requireNoError(t, f.service.SetEnabled(t.Context(), false))
		<-done
		env, err := f.service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		requireEqual(t, false, env.Enabled)
		requireEqual(t, 0, len(env.Targets))
	})
	t.Run("Should omit secrets identities and response bodies from envelopes and logs", func(t *testing.T) {
		t.Parallel()
		var logs bytes.Buffer
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(503)
			writeFixture(w, t, `never-redeem-this-secret account-a raw-private-body`)
		})
		f.service.options.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
		env := refreshFixture(t, f, IntentBackground)
		data, err := json.Marshal(env)
		requireNoError(t, err)
		for _, secret := range []string{"never-redeem-this-secret", "account-a", "raw-private-body", "fixture."} {
			if strings.Contains(string(data)+logs.String(), secret) {
				t.Fatalf("leaked %q", secret)
			}
		}
	})
}

func TestTraySignal(t *testing.T) {
	t.Run(
		"Should select highest known usage break ties by reset and preserve exhausted stale values",
		func(t *testing.T) {
			t.Parallel()
			now := fixtureClock(t).Now()
			later := now.Add(time.Hour)
			rows := []Snapshot{
				{
					ProviderID:   "codex",
					ProviderName: "Codex",
					Status:       StatusOK,
					Windows:      []Window{{ID: "weekly", Label: "Weekly", UsedPercent: new(72.0), ResetsAt: &later}},
				},
				{
					ProviderID:   "claude",
					ProviderName: "Claude",
					Status:       StatusStale,
					Windows: []Window{
						{ID: "weekly_opus", Label: "Weekly · Opus", UsedPercent: new(88.0), ResetsAt: &now},
					},
				},
			}
			tray := SelectTray(rows)
			requireEqual(t, "claude", tray.ProviderID)
			requireEqual(t, SeverityWarning, tray.Severity)
			requireEqual(t, true, tray.Stale)
			rows[1].Windows[0].UsedPercent = new(72.0)
			requireEqual(t, "claude", SelectTray(rows).ProviderID)
			rows[1].Windows[0].UsedPercent = new(112.0)
			requireEqual(t, SeverityExhausted, SelectTray(rows).Severity)
			rows[0].Windows[0].UsedPercent = nil
			rows[1].Windows[0].UsedPercent = nil
			requireEqual(t, (*TraySignal)(nil), SelectTray(rows))
		},
	)
}

func TestUsageScheduling(t *testing.T) {
	t.Run("Should apply interval bounds and recompute snapshot freshness", func(t *testing.T) {
		t.Parallel()
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) { writeFixture(w, t, codexHTTPFixture) })
		refreshFixture(t, f, IntentBackground)
		f.clock.Advance(4 * time.Minute)
		f.service.SetRefreshInterval(time.Minute)
		env, err := f.service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		requireEqual(t, StatusStale, env.Targets[0].Status)
		requireEqual(t, f.clock.Now().Add(time.Minute), *env.Targets[0].NextRefreshAt)
		f.service.SetRefreshInterval(time.Hour)
		env, err = f.service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		requireEqual(t, StatusOK, env.Targets[0].Status)
		requireEqual(t, f.clock.Now().Add(time.Hour), *env.Targets[0].NextRefreshAt)
		revision := env.Revision
		for _, invalid := range []time.Duration{0, -time.Second, time.Minute - time.Nanosecond, time.Hour + time.Nanosecond} {
			f.service.SetRefreshInterval(invalid)
			env, err = f.service.Envelope(t.Context(), "personal")
			requireNoError(t, err)
			requireEqual(t, revision, env.Revision)
			requireEqual(t, f.clock.Now().Add(time.Hour), *env.Targets[0].NextRefreshAt)
		}
	})
	t.Run("Should return immediately for zero wait while collection continues", func(t *testing.T) {
		t.Parallel()
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			close(entered)
			<-release
			writeFixture(w, t, codexHTTPFixture)
		})
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		env, err := f.service.Refresh(ctx, RefreshRequest{ProfileID: "personal"})
		requireNoError(t, err)
		requireEqual(t, true, env.Targets[0].Refreshing)
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("collection did not start")
		}
		once.Do(func() { close(release) })
		requireEqual(t, StatusOK, refreshFixture(t, f, IntentBackground).Targets[0].Status)
	})

	t.Run("Should run one explicit recovery after a joined stale rate limit", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		entered := make(chan struct{})
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) == 2 {
				close(entered)
				<-release
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			writeFixture(w, t, codexHTTPFixture)
		})
		refreshFixture(t, f, IntentBackground)
		f.clock.Advance(time.Minute)
		results := make(chan Envelope, 2)
		refresh := func(intent Intent) {
			env, err := f.service.Refresh(
				t.Context(),
				RefreshRequest{Wait: 15 * time.Second, ProfileID: "personal", Intent: intent},
			)
			if err != nil {
				t.Error(err)
			}
			results <- env
		}
		go refresh(IntentBackground)
		<-entered
		go refresh(IntentExplicit)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		requireNoError(t, f.clock.BlockUntilContext(ctx, 3))
		once.Do(func() { close(release) })
		first, second := <-results, <-results
		requireEqual(t, StatusOK, first.Targets[0].Status)
		requireEqual(t, StatusOK, second.Targets[0].Status)
		requireEqual(t, int32(3), calls.Load())
	})
	t.Run("Should start within startup jitter and collect again on interval ticks", func(t *testing.T) {
		t.Parallel()
		requests := make(chan struct{}, 4)
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			writeFixture(w, t, codexHTTPFixture)
			requests <- struct{}{}
		})
		requireNoError(t, f.service.Start(t.Context(), []string{"personal"}))
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		requireNoError(t, f.clock.BlockUntilContext(ctx, 1))
		f.clock.Advance(10 * time.Second)
		select {
		case <-requests:
		case <-ctx.Done():
			t.Fatal("startup collection missing")
		}
		refreshFixture(t, f, IntentBackground)
		f.service.SetRefreshInterval(time.Minute)
		f.clock.Advance(time.Minute)
		select {
		case <-requests:
		case <-ctx.Done():
			t.Fatal("interval collection missing")
		}
	})
	t.Run("Should stop the managed app server within five seconds", func(t *testing.T) {
		t.Parallel()
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			t.Error("unexpected fallback after shutdown")
			w.WriteHeader(500)
		})
		f.target.Route.NativeCommand = fakeCodex(t)
		requireNoError(t, os.WriteFile(filepath.Join(codexHome(f.target), "block"), nil, 0o600))
		f.catalog.Replace("personal", []Target{f.target})
		done := make(chan struct{})
		go func() {
			if _, err := f.service.Refresh(
				t.Context(),
				RefreshRequest{Wait: 15 * time.Second, ProfileID: "personal"},
			); err != nil {
				t.Error(err)
			}
			close(done)
		}()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		timer := time.NewTicker(time.Millisecond)
		defer timer.Stop()
		for {
			if _, err := os.Stat(filepath.Join(codexHome(f.target), "spawned")); err == nil {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("fixture did not start")
			case <-timer.C:
			}
		}
		requireNoError(t, f.service.Shutdown(ctx))
		<-done
	})
}

func TestUsageFenceAndWait(t *testing.T) {
	t.Run("Should discard in flight route replacement even when the account stays the same", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		entered := make(chan struct{})
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) == 1 {
				close(entered)
				<-release
				writeFixture(w, t, codexHTTPFixture)
				return
			}
			writeFixture(w, t, strings.ReplaceAll(codexHTTPFixture, `"used_percent":40`, `"used_percent":84`))
		})
		done := make(chan Envelope, 1)
		go func() {
			env, err := f.service.Refresh(
				t.Context(),
				RefreshRequest{Wait: 15 * time.Second, ProfileID: "personal"},
			)
			if err != nil {
				t.Error(err)
			}
			done <- env
		}()
		<-entered
		f.target.RouteFingerprint = "replacement-route"
		f.catalog.Replace("personal", []Target{f.target})
		once.Do(func() { close(release) })
		env := <-done
		requireEqual(t, int32(2), calls.Load())
		requireEqual(t, 84.0, *env.Targets[0].Windows[0].UsedPercent)
	})
	t.Run("Should drop last good values when digest changes without an account identifier", func(t *testing.T) {
		t.Parallel()
		var fail atomic.Bool
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			if fail.Load() {
				w.WriteHeader(503)
			}
			writeFixture(w, t, codexHTTPFixture)
		})
		path := filepath.Join(codexHome(f.target), "auth.json")
		requireNoError(t, os.WriteFile(path, []byte(fixtureAuth(t, "", f.clock.Now().Add(time.Hour))), 0o600))
		refreshFixture(t, f, IntentBackground)
		fail.Store(true)
		requireNoError(t, os.WriteFile(path, []byte(fixtureAuth(t, "", f.clock.Now().Add(2*time.Hour))), 0o600))
		env := refreshFixture(t, f, IntentBackground)
		requireEqual(t, StatusError, env.Targets[0].Status)
		requireEqual(t, 0, len(env.Targets[0].Windows))
	})
	t.Run("Should return at the caller wait deadline without canceling the collection", func(t *testing.T) {
		t.Parallel()
		entered := make(chan struct{})
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			close(entered)
			<-release
			writeFixture(w, t, codexHTTPFixture)
		})
		done := make(chan Envelope, 1)
		go func() {
			env, err := f.service.Refresh(
				t.Context(),
				RefreshRequest{ProfileID: "personal", Wait: time.Second},
			)
			if err != nil {
				t.Error(err)
			}
			done <- env
		}()
		<-entered
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		requireNoError(t, f.clock.BlockUntilContext(ctx, 2))
		f.clock.Advance(time.Second)
		env := <-done
		requireEqual(t, true, env.Targets[0].Refreshing)
		once.Do(func() { close(release) })
		completed := refreshFixture(t, f, IntentBackground)
		requireEqual(t, StatusOK, completed.Targets[0].Status)
	})
}

func TestUsageCollectionDeadline(t *testing.T) {
	t.Run("Should publish a typed failure when the daemon owned collection deadline expires", func(t *testing.T) {
		t.Parallel()
		entered := make(chan struct{})
		f := usageFixture(t, func(_ http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() })
		revisions, unsubscribe := f.service.Subscribe("personal")
		defer unsubscribe()
		done := make(chan struct{})
		go func() {
			if _, err := f.service.Refresh(
				t.Context(),
				RefreshRequest{Wait: 15 * time.Second, ProfileID: "personal"},
			); err != nil {
				t.Error(err)
			}
			close(done)
		}()
		<-entered
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		requireNoError(t, f.clock.BlockUntilContext(ctx, 2))
		f.clock.Advance(time.Minute)
		<-done
		for {
			select {
			case <-revisions:
				env, err := f.service.Envelope(t.Context(), "personal")
				requireNoError(t, err)
				if !env.Targets[0].Refreshing {
					requireEqual(t, StatusError, env.Targets[0].Status)
					requireEqual(t, ErrorCodeNetworkError, env.Targets[0].Error.Code)
					return
				}
			case <-ctx.Done():
				t.Fatal("expired collection did not publish failure")
			}
		}
	})
}

func TestUsagePartialSnapshots(t *testing.T) {
	t.Run("Should publish fenced partial account data and preserve same owner last good data", func(t *testing.T) {
		t.Parallel()
		clock := fixtureClock(t)
		target := collectorTestTarget(t, "gemini", false)
		target.ProfileID = "personal"
		target.RouteFingerprint = "gemini-route"
		var quotaFails atomic.Bool
		quotaFails.Store(true)
		var plan atomic.Value
		plan.Store("First plan")
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1internal:loadCodeAssist" {
				data, err := json.Marshal(
					map[string]any{"currentTier": map[string]string{"name": plan.Load().(string)}},
				)
				if err != nil {
					t.Error(err)
					return
				}
				writeFixture(w, t, string(data))
				return
			}
			if quotaFails.Load() {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			writeFixture(w, t, `{"buckets":[{"modelId":"pro","remainingFraction":0.25}]}`)
		})
		catalog := &TargetCatalog{}
		catalog.Replace(target.ProfileID, []Target{target})
		service := NewService(
			Options{
				Resolver: catalog,
				Clock:    clock,
				Collectors: DefaultRegistry(
					RegistryOptions{BaseURLs: map[string]string{"gemini": server.URL}, Now: clock.Now},
				),
			},
		)
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		refresh := func() Snapshot {
			t.Helper()
			env, err := service.Refresh(
				t.Context(),
				RefreshRequest{Wait: 15 * time.Second, ProfileID: target.ProfileID},
			)
			requireNoError(t, err)
			return env.Targets[0]
		}
		partial := refresh()
		requireEqual(t, StatusError, partial.Status)
		requireEqual(t, ErrorCodeProviderUnavailable, partial.Error.Code)
		requireEqual(t, "First plan", partial.Account.Plan)
		requireEqual(t, (*time.Time)(nil), partial.MeasuredAt)
		partial.Account.Plan = "mutated by caller"
		cached, err := service.Envelope(t.Context(), target.ProfileID)
		requireNoError(t, err)
		requireEqual(t, "First plan", cached.Targets[0].Account.Plan)
		quotaFails.Store(false)
		good := refresh()
		requireEqual(t, StatusOK, good.Status)
		clock.Advance(time.Minute)
		quotaFails.Store(true)
		plan.Store("New plan")
		stale := refresh()
		requireEqual(t, StatusStale, stale.Status)
		requireEqual(t, "First plan", stale.Account.Plan)
		requireEqual(t, *good.MeasuredAt, *stale.MeasuredAt)
		requireEqual(t, 75.0, *stale.Windows[0].UsedPercent)
		path := filepath.Join(routeEnv(target, "HOME"), ".gemini", "oauth_creds.json")
		data, err := json.Marshal(
			map[string]any{"access_token": "replacement-token", "expiry_date": clock.Now().Add(time.Hour).UnixMilli()},
		)
		requireNoError(t, err)
		requireNoError(t, os.WriteFile(path, data, 0o600))
		replaced := refresh()
		requireEqual(t, StatusError, replaced.Status)
		requireEqual(t, "New plan", replaced.Account.Plan)
		requireEqual(t, 0, len(replaced.Windows))
		requireEqual(t, (*time.Time)(nil), replaced.MeasuredAt)
	})
}
