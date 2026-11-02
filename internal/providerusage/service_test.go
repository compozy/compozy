package providerusage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	for _, test := range []struct {
		name   string
		action string
	}{
		{name: "Should serve unrelated profiles while credential evidence is pending", action: "read"},
		{name: "Should preserve a credential reset over an older pending read fence", action: "clear"},
		{name: "Should preserve a removed target over an older pending read fence", action: "remove"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service, collector, catalog, _ := credentialFenceFixture(t)
			gate := &credentialProbeGate{entered: make(chan struct{}), release: make(chan struct{})}
			release := sync.OnceFunc(func() { close(gate.release) })
			collector.gate.Store(gate)
			ownerDone := make(chan struct{})
			var ownerEnvelope Envelope
			var ownerErr error
			go func() {
				defer close(ownerDone)
				ownerEnvelope, ownerErr = service.Envelope(t.Context(), "personal")
			}()
			t.Cleanup(func() { release(); <-ownerDone })
			select {
			case <-gate.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("credential probe did not start")
			}

			operationDone := make(chan struct{})
			var operationErr error
			go func() {
				defer close(operationDone)
				switch test.action {
				case "clear":
					service.ClearCredentials("personal", "codex")
				case "remove":
					catalog.Replace("personal", nil)
					_, operationErr = service.Envelope(t.Context(), "personal")
				default:
					_, operationErr = service.Envelope(t.Context(), "other")
				}
			}()
			t.Cleanup(func() { release(); <-operationDone })
			select {
			case <-operationDone:
				requireNoError(t, operationErr)
			case <-time.After(5 * time.Second):
				t.Fatal("credential I/O held the manager lock")
			}
			release()
			<-ownerDone
			requireNoError(t, ownerErr)
			switch test.action {
			case "clear":
				requireEqual(t, StatusRefreshing, ownerEnvelope.Targets[0].Status)
				requireEqual(t, 0, len(ownerEnvelope.Targets[0].Windows))
			case "remove":
				requireEqual(t, 0, len(ownerEnvelope.Targets))
			default:
				requireEqual(t, StatusOK, ownerEnvelope.Targets[0].Status)
			}
		})
	}
	for _, change := range []string{"identity", "epoch", "publication"} {
		name := "Should preserve a newer credential " + change + " over an older pending read mismatch"
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			service, collector, _, path := credentialFenceFixture(t)
			original, err := os.ReadFile(path)
			requireNoError(t, err)
			renewed := bytes.ReplaceAll(original, []byte("fixture-owner"), []byte("next-owner"))
			requireNoError(t, os.WriteFile(path, renewed, 0o600))
			gate := &credentialProbeGate{entered: make(chan struct{}), release: make(chan struct{})}
			release := sync.OnceFunc(func() { close(gate.release) })
			collector.gate.Store(gate)
			ownerDone := make(chan struct{})
			var ownerEnvelope Envelope
			var ownerErr error
			go func() {
				defer close(ownerDone)
				ownerEnvelope, ownerErr = service.Envelope(t.Context(), "personal")
			}()
			t.Cleanup(func() { release(); <-ownerDone })
			select {
			case <-gate.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("credential probe did not start")
			}
			if change == "epoch" {
				service.ClearCredentials("personal", "codex")
			}
			if change != "identity" {
				requireNoError(t, os.WriteFile(path, original, 0o600))
			}
			if change == "publication" {
				service.options.Clock.(*clockwork.FakeClock).Advance(time.Minute)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			latest, err := service.Refresh(ctx, RefreshRequest{
				ProfileID: "personal", Intent: IntentExplicit, Wait: time.Second,
			})
			requireNoError(t, err)
			requireEqual(t, StatusOK, latest.Targets[0].Status)
			// Hold any accidental recollection so it cannot hide a stale read clearing the new snapshot.
			recollect := &credentialProbeGate{entered: make(chan struct{}), release: make(chan struct{})}
			collector.gate.Store(recollect)
			t.Cleanup(func() { close(recollect.release) })
			release()
			<-ownerDone
			requireNoError(t, ownerErr)
			requireEqual(t, StatusOK, ownerEnvelope.Targets[0].Status)
			requireEqual(t, len(latest.Targets[0].Windows), len(ownerEnvelope.Targets[0].Windows))
		})
	}
	for _, change := range []string{"native login", "config directory", "secure storage directory", "home"} {
		t.Run("Should recover Claude usage after switching "+change, func(t *testing.T) {
			t.Parallel()
			target, dir, credential := claudeFixture(t, false)
			clock := fixtureClock(t)
			var login atomic.Value
			login.Store(credential)
			reader := &fakeClaudeKeychain{
				read: func(_ context.Context, service, account string, _ bool) ([]byte, error) {
					wantService, wantAccount := claudeKeychainItem(target)
					requireEqual(t, wantService, service)
					requireEqual(t, wantAccount, account)
					return []byte(login.Load().(string)), nil
				},
			}
			var requests atomic.Int32
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Header.Get("Authorization") == "Bearer replacement-secret" {
					writeFixture(w, t, `{"five_hour":{"utilization":17}}`)
					return
				}
				requireEqual(t, "Bearer synthetic-secret", r.Header.Get("Authorization"))
				writeFixture(w, t, `{"five_hour":{"utilization":42}}`)
			})
			collector := NewClaudeCollector(
				WithClaudeClock(clock.Now), WithClaudeKeychain(reader),
				WithClaudeHTTP(server.URL, nil), WithClaudePlatform("darwin"),
			)
			catalog := &TargetCatalog{}
			catalog.Replace(target.ProfileID, []Target{target})
			service := NewService(Options{Resolver: catalog, Collectors: Registry{"claude": collector}, Clock: clock})
			t.Cleanup(func() {
				if err := service.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			refresh := func(intent Intent) Snapshot {
				t.Helper()
				env, err := service.Refresh(t.Context(), RefreshRequest{
					ProfileID: target.ProfileID, Intent: intent, Wait: 15 * time.Second,
				})
				requireNoError(t, err)
				return env.Targets[0]
			}
			first := refresh(IntentExplicit)
			requireEqual(t, StatusOK, first.Status)
			requireEqual(t, 42.0, *first.Windows[0].UsedPercent)
			requireEqual(t, "max_20x", first.Account.Plan)
			if change != "native login" {
				target.RouteFingerprint = "route-two"
				switch change {
				case "config directory":
					dir = t.TempDir()
					target.Route.Env = append(target.Route.Env, "CLAUDE_CONFIG_DIR="+dir)
				case "secure storage directory":
					target.Route.Env = append(target.Route.Env, "CLAUDE_SECURESTORAGE_CONFIG_DIR="+t.TempDir())
				case "home":
					dir = t.TempDir()
					target.Route.Env = []string{"HOME=" + dir, "USER=test-user"}
				}
			}
			requireNoError(t, os.WriteFile(filepath.Join(dir, ".claude.json"),
				[]byte(`{"oauthAccount":{"accountUuid":"account-B"}}`), 0o600))
			renewed := strings.ReplaceAll(credential, "synthetic-secret", "replacement-secret")
			renewed = strings.ReplaceAll(renewed, "default_claude_max_20x", "default_claude_max_5x")
			login.Store(renewed)
			catalog.Replace(target.ProfileID, []Target{target})
			after := refresh(IntentBackground)
			requireEqual(t, StatusOK, after.Status)
			requireEqual(t, 17.0, *after.Windows[0].UsedPercent)
			requireEqual(t, "max_5x", after.Account.Plan)
			requireEqual(t, int32(1), reader.interactive.Load())
			clock.Advance(5 * time.Minute)
			next := refresh(IntentBackground)
			requireEqual(t, StatusOK, next.Status)
			requireEqual(t, 17.0, *next.Windows[0].UsedPercent)
			requireEqual(t, "max_5x", next.Account.Plan)
			requireEqual(t, int32(2), reader.calls.Load())
			requireEqual(t, int32(3), requests.Load())
		})
	}

	t.Run("Should not let an older overlapping resolution restore a disabled provider", func(t *testing.T) {
		t.Parallel()
		entered, release := make(chan struct{}), make(chan struct{})
		var resolves atomic.Int32
		service := NewService(Options{Resolver: usageResolverFunc(func(context.Context, string) ([]Target, error) {
			if resolves.Add(1) == 1 {
				close(entered)
				<-release
				return []Target{{ProviderID: "codex", ProviderName: "Codex"}}, nil
			}
			return nil, nil
		})})
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		stale := make(chan error, 1)
		go func() {
			_, err := service.Envelope(context.Background(), "personal")
			stale <- err
		}()
		<-entered
		current, err := service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		requireEqual(t, 0, len(current.Targets))
		close(release)
		requireNoError(t, <-stale)
		requireEqual(t, 0, len(service.cached("personal").Targets))
	})

	t.Run("Should collect a changed route after startup collection has finished", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) == 2 {
				close(entered)
				<-release
			}
			writeFixture(w, t, codexHTTPFixture)
		})
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		requireEqual(t, StatusOK, refreshFixture(t, f, IntentBackground).Targets[0].Status)
		f.target.RouteFingerprint = "saved-provider-route"
		f.catalog.Replace("personal", []Target{f.target})
		requireNoError(t, f.service.Start(t.Context(), []string{"personal"}))
		env, err := f.service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		requireEqual(t, 0, len(env.Targets[0].Windows))
		requireEqual(t, true, env.Targets[0].Refreshing)
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("replacement route was left waiting for the interval")
		}
		once.Do(func() { close(release) })
		requireEqual(t, StatusOK, refreshFixture(t, f, IntentBackground).Targets[0].Status)
	})

	t.Run("Should discard a changed file login on read and collect the replacement", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		f := usageFixture(t, func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) > 1 {
				requireEqual(t, "account-b", r.Header.Get("ChatGPT-Account-Id"))
				close(entered)
				<-release
			}
			writeFixture(w, t, codexHTTPFixture)
		})
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		requireEqual(t, StatusOK, refreshFixture(t, f, IntentBackground).Targets[0].Status)
		_, err := f.service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		path := filepath.Join(codexHome(f.target), "auth.json")
		stat, err := os.Stat(path)
		requireNoError(t, err)
		requireNoError(
			t,
			os.WriteFile(path+".new", []byte(fixtureAuth(t, "account-b", f.clock.Now().Add(24*time.Hour))), 0o600),
		)
		requireNoError(t, os.Chtimes(path+".new", stat.ModTime(), stat.ModTime()))
		requireNoError(t, os.Rename(path+".new", path))
		env, err := f.service.Envelope(t.Context(), "personal")
		requireNoError(t, err)
		requireEqual(t, StatusRefreshing, env.Targets[0].Status)
		requireEqual(t, 0, len(env.Targets[0].Windows))
		requireEqual(t, (*Account)(nil), env.Targets[0].Account)
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("replacement collection was not scheduled")
		}
		once.Do(func() { close(release) })
		requireEqual(t, StatusOK, refreshFixture(t, f, IntentBackground).Targets[0].Status)
	})

	for _, mutation := range []string{"logout", "malformed", "keyring"} {
		t.Run("Should discard last good on read after "+mutation, func(t *testing.T) {
			t.Parallel()
			f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) { writeFixture(w, t, codexHTTPFixture) })
			requireEqual(t, StatusOK, refreshFixture(t, f, IntentBackground).Targets[0].Status)
			before, err := f.service.Envelope(t.Context(), "personal")
			requireNoError(t, err)
			unchanged, err := f.service.Envelope(t.Context(), "personal")
			requireNoError(t, err)
			requireEqual(t, before.Revision, unchanged.Revision)
			path := filepath.Join(codexHome(f.target), "auth.json")
			switch mutation {
			case "logout":
				requireNoError(t, os.Remove(path))
			case "malformed":
				requireNoError(t, os.WriteFile(path, []byte("{"), 0o600))
			case "keyring":
				requireNoError(
					t,
					os.WriteFile(
						filepath.Join(codexHome(f.target), "config.toml"),
						[]byte(`cli_auth_credentials_store = "keyring"`),
						0o600,
					),
				)
			}
			env, err := f.service.Envelope(t.Context(), "personal")
			requireNoError(t, err)
			requireEqual(t, StatusRefreshing, env.Targets[0].Status)
			requireEqual(t, 0, len(env.Targets[0].Windows))
			requireEqual(t, (*Account)(nil), env.Targets[0].Account)
		})
	}

	for _, file := range []bool{false, true} {
		name := "Should retain same owner Claude Keychain data after expiry UT083"
		if file {
			name = "Should retain same owner Claude file data after expiry UT083"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			target, _, credential := claudeFixture(t, file)
			clock := fixtureClock(t)
			reader := &fakeClaudeKeychain{data: credential}
			var expectedToken atomic.Value
			expectedToken.Store("synthetic-secret")
			server := fixtureServer(
				t,
				func(w http.ResponseWriter, r *http.Request) {
					requireEqual(t, "Bearer "+expectedToken.Load().(string), r.Header.Get("Authorization"))
					writeFixture(w, t, `{"five_hour":{"utilization":42}}`)
				},
			)
			collector := NewClaudeCollector(
				WithClaudeClock(clock.Now),
				WithClaudeKeychain(reader),
				WithClaudeHTTP(server.URL, nil),
			)
			collector.platform = "darwin"
			catalog := &TargetCatalog{}
			catalog.Replace(target.ProfileID, []Target{target})
			service := NewService(Options{Resolver: catalog, Collectors: Registry{"claude": collector}, Clock: clock})
			defer func() { requireNoError(t, service.Shutdown(context.Background())) }()
			refresh := func(intent Intent) Snapshot {
				env, err := service.Refresh(
					t.Context(),
					RefreshRequest{ProfileID: target.ProfileID, Intent: intent, Wait: 15 * time.Second},
				)
				requireNoError(t, err)
				return env.Targets[0]
			}
			first := refresh(IntentExplicit)
			requireEqual(t, StatusOK, first.Status)
			oldExpiry := clock.Now().Add(time.Hour).UnixMilli()
			clock.Advance(2 * time.Hour)
			stale := refresh(IntentBackground)
			requireEqual(t, StatusStale, stale.Status)
			requireEqual(t, *first.MeasuredAt, *stale.MeasuredAt)
			requireEqual(t, 42.0, *stale.Windows[0].UsedPercent)
			requireEqual(t, "Run claude /login to sign in again", stale.Error.Message)
			requireEqual(t, string(ActionSignIn), stale.Error.Action)
			renewed := strings.Replace(
				credential,
				fmt.Sprint(oldExpiry),
				fmt.Sprint(clock.Now().Add(time.Hour).UnixMilli()),
				1,
			)
			renewed = strings.ReplaceAll(renewed, "synthetic-secret", "renewed-secret")
			expectedToken.Store("renewed-secret")
			if file {
				requireNoError(t, os.WriteFile(claudeCredentialPath(target), []byte(renewed), 0o600))
			} else {
				reader.data = renewed
			}
			requireEqual(t, StatusOK, refresh(IntentBackground).Status)
			wantInteractive := int32(1)
			if file {
				wantInteractive = 0
			}
			requireEqual(t, wantInteractive, reader.interactive.Load())
		})
	}

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

type usageResolverFunc func(context.Context, string) ([]Target, error)

func (f usageResolverFunc) Targets(ctx context.Context, profile string) ([]Target, error) {
	return f(ctx, profile)
}

type usageMetricsChannel chan Status

func (c usageMetricsChannel) Collect(_ string, status Status, _ time.Duration) { c <- status }

func TestUsageScheduling(t *testing.T) {
	t.Run("Should collect a new profile after the startup tick without duplicate flights", func(t *testing.T) {
		t.Parallel()
		entered, release := make(chan struct{}), make(chan struct{})
		startup := make(chan struct{}, 4)
		var once sync.Once
		var workCalls atomic.Int32
		f := usageFixture(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("ChatGPT-Account-Id") == "work-account" {
				if workCalls.Add(1) == 1 {
					close(entered)
				}
				<-release
			}
			if r.Header.Get("ChatGPT-Account-Id") != "work-account" {
				startup <- struct{}{}
			}
			writeFixture(w, t, codexHTTPFixture)
		})
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		requireNoError(t, f.service.Start(t.Context(), []string{"personal"}))
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		requireNoError(t, f.clock.BlockUntilContext(ctx, 1))
		f.clock.Advance(10 * time.Second)
		select {
		case <-startup:
		case <-ctx.Done():
			t.Fatal("startup collection did not run")
		}
		requireEqual(t, StatusOK, refreshFixture(t, f, IntentBackground).Targets[0].Status)
		work := codexTarget(t, "work-account", f.clock.Now().Add(time.Hour))
		work.ProfileID = "work"
		f.catalog.Replace("work", []Target{work})
		requireNoError(t, f.service.Start(ctx, []string{"personal", "work"}))
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("new profile was left waiting for the refresh interval")
		}
		requireNoError(t, f.service.Start(ctx, []string{"personal", "work"}))
		env, err := f.service.Envelope(ctx, "work")
		requireNoError(t, err)
		requireEqual(t, true, env.Targets[0].Refreshing)
		requireEqual(t, int32(1), workCalls.Load())
		once.Do(func() { close(release) })
		env, err = f.service.Refresh(
			ctx,
			RefreshRequest{ProfileID: "work", Wait: time.Second, Intent: IntentBackground},
		)
		requireNoError(t, err)
		requireEqual(t, StatusOK, env.Targets[0].Status)
	})

	for _, operation := range []string{"disable", "shutdown"} {
		t.Run("Should cancel blocked scheduled resolution on "+operation, func(t *testing.T) {
			t.Parallel()
			f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) { writeFixture(w, t, codexHTTPFixture) })
			entered, release := make(chan context.Context, 1), make(chan struct{})
			t.Cleanup(func() { close(release) })
			var calls atomic.Int32
			f.service.options.Resolver = usageResolverFunc(func(ctx context.Context, profile string) ([]Target, error) {
				if calls.Add(1) > 1 {
					entered <- ctx
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-release:
						return nil, context.Canceled
					}
				}
				return f.catalog.Targets(ctx, profile)
			})
			requireNoError(t, f.service.Start(t.Context(), []string{"personal"}))
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			requireNoError(t, f.clock.BlockUntilContext(ctx, 1))
			f.clock.Advance(10 * time.Second)
			var resolution context.Context
			select {
			case resolution = <-entered:
			case <-ctx.Done():
				t.Fatal("scheduled resolution did not start")
			}
			var err error
			if operation == "disable" {
				err = f.service.SetEnabled(ctx, false)
			} else {
				err = f.service.Shutdown(ctx)
			}
			requireNoError(t, err)
			requireEqual(t, context.Canceled, resolution.Err())
		})
	}

	t.Run("Should drop deleted profiles and close only their subscriptions", func(t *testing.T) {
		t.Parallel()
		entered := make(chan struct{})
		var calls atomic.Int32
		f := usageFixture(t, func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				close(entered)
				<-r.Context().Done()
				return
			}
			writeFixture(w, t, codexHTTPFixture)
		})
		other := f.target
		other.ProfileID = "work"
		f.catalog.Replace("work", []Target{other})
		requireNoError(t, f.service.Start(t.Context(), []string{"personal", "work"}))
		_, err := f.service.Refresh(t.Context(), RefreshRequest{ProfileID: "personal"})
		requireNoError(t, err)
		<-entered
		deleted, unsubscribe := f.service.Subscribe("personal")
		defer unsubscribe()
		live, unsubscribeLive := f.service.Subscribe("work")
		defer unsubscribeLive()
		requireNoError(t, f.service.Start(t.Context(), []string{"work"}))
		select {
		case _, ok := <-deleted:
			requireEqual(t, false, ok)
		default:
			t.Fatal("deleted profile subscription remains open")
		}
		select {
		case _, ok := <-live:
			if !ok {
				t.Fatal("live profile subscription was closed")
			}
		default:
		}
		f.service.mu.Lock()
		_, retained := f.service.profiles["personal"]
		f.service.mu.Unlock()
		requireEqual(t, false, retained)
		env, err := f.service.Refresh(t.Context(), RefreshRequest{ProfileID: "work", Wait: 15 * time.Second})
		requireNoError(t, err)
		requireEqual(t, StatusOK, env.Targets[0].Status)
	})
	for _, slow := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should recover a failed publication fence slow=%t", slow), func(t *testing.T) {
			t.Parallel()
			var upstream atomic.Int32
			f := usageFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				n := upstream.Add(1)
				writeFixture(
					w,
					t,
					strings.Replace(codexHTTPFixture, `"used_percent":40`, fmt.Sprintf(`"used_percent":%d`, 40+n), 1),
				)
			})
			first := refreshFixture(t, f, IntentBackground).Targets[0]
			f.clock.Advance(time.Minute)
			var resolves atomic.Int32
			f.service.options.Resolver = usageResolverFunc(func(ctx context.Context, profile string) ([]Target, error) {
				if n := resolves.Add(1); n == 2 || n == 3 {
					if slow {
						<-ctx.Done()
					}
					return nil, fmt.Errorf("private resolver detail")
				}
				return f.catalog.Targets(ctx, profile)
			})
			metrics := make(usageMetricsChannel, 4)
			f.service.options.Metrics = metrics
			var logs bytes.Buffer
			f.service.options.Logger = slog.New(slog.NewTextHandler(&logs, nil))
			_, err := f.service.Refresh(t.Context(), RefreshRequest{ProfileID: "personal"})
			requireNoError(t, err)
			select {
			case status := <-metrics:
				requireEqual(t, StatusStale, status)
			case <-time.After(3 * time.Second):
				t.Fatal("fence failure emitted no collection metric")
			}
			row := f.service.cached("personal").Targets[0]
			requireEqual(t, StatusStale, row.Status)
			requireEqual(t, ErrorCodeProviderUnavailable, row.Error.Code)
			requireEqual(t, *first.MeasuredAt, *row.MeasuredAt)
			requireEqual(t, *first.Windows[0].UsedPercent, *row.Windows[0].UsedPercent)
			requireEqual(t, string(ActionRetry), row.Error.Action)
			if !strings.Contains(logs.String(), "provider_unavailable") ||
				strings.Contains(logs.String(), "private resolver detail") {
				t.Fatal("missing or unsafe fence diagnostic")
			}
			f.clock.Advance(5 * time.Second)
			select {
			case status := <-metrics:
				requireEqual(t, StatusStale, status)
			case <-time.After(3 * time.Second):
				t.Fatal("fence retry did not run")
			}
			requireEqual(t, int32(2), upstream.Load())
			f.clock.Advance(5 * time.Second)
			env, err := f.service.Refresh(t.Context(), RefreshRequest{ProfileID: "personal", Wait: 15 * time.Second})
			requireNoError(t, err)
			requireEqual(t, StatusOK, env.Targets[0].Status)
			requireEqual(t, 42.0, *env.Targets[0].Windows[0].UsedPercent)
			requireEqual(t, int32(2), upstream.Load())
		})
	}
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

	t.Run("Should honor an explicit refresh joined during a Claude authentication failure", func(t *testing.T) {
		t.Parallel()
		target, _, credential := claudeFixture(t, false)
		clock := fixtureClock(t)
		var login atomic.Value
		login.Store(credential)
		reader := &fakeClaudeKeychain{read: func(context.Context, string, string, bool) ([]byte, error) {
			return []byte(login.Load().(string)), nil
		}}
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		defer once.Do(func() { close(release) })
		var requests atomic.Int32
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			if requests.Add(1) == 1 {
				requireEqual(t, "Bearer synthetic-secret", r.Header.Get("Authorization"))
				close(entered)
				<-release
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			requireEqual(t, "Bearer renewed-secret", r.Header.Get("Authorization"))
			writeFixture(w, t, `{"five_hour":{"utilization":17}}`)
		})
		collector := NewClaudeCollector(WithClaudeClock(clock.Now), WithClaudeKeychain(reader),
			WithClaudeHTTP(server.URL, nil), WithClaudePlatform("darwin"))
		catalog := &TargetCatalog{}
		catalog.Replace(target.ProfileID, []Target{target})
		service := NewService(Options{Resolver: catalog, Collectors: Registry{"claude": collector}, Clock: clock})
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		results := make(chan Envelope, 1)
		go func() {
			env, err := service.Refresh(ctx, RefreshRequest{
				ProfileID: target.ProfileID, Intent: IntentBackground, Wait: 15 * time.Second,
			})
			if err != nil {
				t.Error(err)
			}
			results <- env
		}()
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("background collection did not start")
		}
		login.Store(strings.ReplaceAll(credential, "synthetic-secret", "renewed-secret"))
		pending, err := service.Refresh(ctx, RefreshRequest{ProfileID: target.ProfileID, Intent: IntentExplicit})
		requireNoError(t, err)
		requireEqual(t, true, pending.Targets[0].Refreshing)
		once.Do(func() { close(release) })
		select {
		case result := <-results:
			requireEqual(t, 1, len(result.Targets))
			requireEqual(t, StatusOK, result.Targets[0].Status)
			requireEqual(t, 17.0, *result.Targets[0].Windows[0].UsedPercent)
		case <-ctx.Done():
			t.Fatal("joined explicit refresh did not finish")
		}
		requireEqual(t, int32(2), requests.Load())
		requireEqual(t, int32(2), reader.calls.Load())
		requireEqual(t, int32(1), reader.interactive.Load())
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
		startCtx, stopStart := context.WithCancel(t.Context())
		requireNoError(t, f.service.Start(startCtx, []string{"personal"}))
		stopStart()
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
	t.Run("Should recollect a rotated key after a failed old publication fence AstraR3", func(t *testing.T) {
		t.Parallel()
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		var upstream, resolves atomic.Int32
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			upstream.Add(1)
			if r.Header.Get("Authorization") == "Bearer synthetic-key-a" {
				writeFixture(w, t, `{"balance":"10","total_used":"90"}`)
				return
			}
			writeFixture(w, t, `{"balance":"90","total_used":"10"}`)
		})
		target := Target{
			ProfileID:        "personal",
			ProviderID:       "vercel-ai-gateway",
			ProviderName:     "Vercel",
			RouteFingerprint: "same-route",
			Route:            ResolvedRoute{BoundSecrets: map[string]string{"api_key": "synthetic-key-a"}},
		}
		catalog := &TargetCatalog{}
		catalog.Replace("personal", []Target{target})
		resolver := usageResolverFunc(func(ctx context.Context, profile string) ([]Target, error) {
			if resolves.Add(1) == 2 {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
				}
				return nil, fmt.Errorf("transient resolver failure")
			}
			return catalog.Targets(ctx, profile)
		})
		svc := NewService(
			Options{
				Resolver:   resolver,
				Collectors: Registry{"vercel-ai-gateway": NewVercelCollector(WithCollectorBaseURL(server.URL))},
			},
		)
		defer func() { requireNoError(t, svc.Shutdown(context.Background())) }()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		done := make(chan Envelope, 1)
		go func() {
			env, err := svc.Refresh(ctx, RefreshRequest{ProfileID: "personal", Wait: 15 * time.Second})
			if err != nil {
				t.Error(err)
			}
			done <- env
		}()
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("publication fence did not start")
		}
		target.Route.BoundSecrets = map[string]string{"api_key": "synthetic-key-b"}
		catalog.Replace("personal", []Target{target})
		_, err := svc.Envelope(ctx, "personal")
		requireNoError(t, err)
		once.Do(func() { close(release) })
		var env Envelope
		select {
		case env = <-done:
		case <-ctx.Done():
			t.Fatal("refresh did not finish")
		}
		row := env.Targets[0]
		if row.Status != StatusOK || len(row.Balances) != 1 || row.Balances[0].Remaining.Value != "90" {
			t.Fatalf(
				"replacement refresh stranded: status=%s refreshing=%t balances=%d calls=%d error=%v",
				row.Status,
				row.Refreshing,
				len(row.Balances),
				upstream.Load(),
				row.Error,
			)
		}
		requireEqual(t, int32(2), upstream.Load())
	})

	t.Run("Should return disabled reads before canceled transport cleanup", func(t *testing.T) {
		t.Parallel()
		entered, release := make(chan struct{}), make(chan struct{})
		f := usageFixture(
			t,
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		)
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		f.service.options.Collectors[codexProviderID] = NewCodexCollector(
			CodexOptions{
				Now: f.clock.Now,
				Transport: codexFixtureTransport(func(r *http.Request) (*http.Response, error) {
					close(entered)
					<-r.Context().Done()
					<-release
					return nil, r.Context().Err()
				}),
			},
		)
		_, err := f.service.Refresh(t.Context(), RefreshRequest{ProfileID: "personal"})
		requireNoError(t, err)
		<-entered
		f.target.Status = StatusDisabled
		f.catalog.Replace("personal", []Target{f.target})
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		env, err := f.service.Envelope(ctx, "personal")
		requireNoError(t, err)
		requireEqual(t, StatusDisabled, env.Targets[0].Status)
		requireEqual(t, false, env.Targets[0].Refreshing)
		_, err = f.service.Refresh(ctx, RefreshRequest{ProfileID: "personal"})
		requireNoError(t, err)
		once.Do(func() { close(release) })
		requireNoError(t, f.service.Start(t.Context(), []string{"personal"}))
	})
	t.Run("Should reject replaced bound keys before publication Astra", func(t *testing.T) {
		var calls atomic.Int32
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.Header.Get("Authorization") == "Bearer synthetic-key-a" {
				close(entered)
				<-release
				writeFixture(w, t, `{"balance":"10","total_used":"90"}`)
				return
			}
			writeFixture(w, t, `{"balance":"90","total_used":"10"}`)
		})
		target := Target{
			ProfileID:        "personal",
			ProviderID:       "vercel-ai-gateway",
			ProviderName:     "Vercel",
			RouteFingerprint: "same-route",
			Route:            ResolvedRoute{BoundSecrets: map[string]string{"api_key": "synthetic-key-a"}},
		}
		catalog := &TargetCatalog{}
		catalog.Replace("personal", []Target{target})
		svc := NewService(
			Options{
				Resolver:   catalog,
				Collectors: Registry{"vercel-ai-gateway": NewVercelCollector(WithCollectorBaseURL(server.URL))},
			},
		)
		defer func() { requireNoError(t, svc.Shutdown(context.Background())) }()
		done := make(chan Envelope, 1)
		go func() {
			env, err := svc.Refresh(t.Context(), RefreshRequest{ProfileID: "personal", Wait: 15 * time.Second})
			if err != nil {
				t.Error(err)
			}
			done <- env
		}()
		<-entered
		target.Route.BoundSecrets = map[string]string{"api_key": "synthetic-key-b"}
		catalog.Replace("personal", []Target{target})
		once.Do(func() { close(release) })
		env := <-done
		if len(env.Targets) != 1 || len(env.Targets[0].Balances) != 1 ||
			env.Targets[0].Balances[0].Remaining.Value != "90" {
			t.Fatalf("rotated key should recollect before publishing: calls=%d envelope=%+v", calls.Load(), env)
		}
	})

	for _, removed := range []bool{false, true} {
		name := "Should cancel only the disabled provider flight Astra"
		if removed {
			name = "Should cancel only the removed provider flight Astra"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entered, otherEntered := make(chan struct{}), make(chan struct{})
			release, otherRelease := make(chan struct{}), make(chan struct{})
			var once, otherOnce sync.Once
			t.Cleanup(func() { once.Do(func() { close(release) }); otherOnce.Do(func() { close(otherRelease) }) })
			var credits, otherCredits atomic.Int32
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				other := r.Header.Get("Authorization") == "Bearer other-profile-key"
				if r.URL.Path == "/api/v1/key" {
					started, done := entered, release
					if other {
						started, done = otherEntered, otherRelease
					}
					close(started)
					select {
					case <-done:
					case <-r.Context().Done():
						return
					}
					writeFixture(w, t, `{"data":{"limit":100,"usage":25}}`)
					return
				}
				if other {
					otherCredits.Add(1)
				} else {
					credits.Add(1)
				}
				writeFixture(w, t, `{"data":{"total_credits":100,"total_usage":25}}`)
			})
			target := Target{
				ProfileID:        "personal",
				ProviderID:       "openrouter",
				ProviderName:     "OpenRouter",
				RouteFingerprint: "same-route",
				Route:            ResolvedRoute{BoundSecrets: map[string]string{"api_key": "synthetic-key"}},
			}
			other := target
			other.ProfileID = "work"
			other.Route.BoundSecrets = map[string]string{"api_key": "other-profile-key"}
			catalog := &TargetCatalog{}
			catalog.Replace("personal", []Target{target})
			catalog.Replace("work", []Target{other})
			svc := NewService(
				Options{
					Resolver:   catalog,
					Collectors: Registry{"openrouter": NewOpenRouterCollector(WithCollectorBaseURL(server.URL))},
				},
			)
			defer func() { requireNoError(t, svc.Shutdown(context.Background())) }()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			done, otherDone := make(chan Envelope, 1), make(chan Envelope, 1)
			refresh := func(profile string, result chan<- Envelope) {
				env, err := svc.Refresh(ctx, RefreshRequest{ProfileID: profile, Wait: 15 * time.Second})
				if err != nil {
					t.Error(err)
				}
				result <- env
			}
			go refresh("personal", done)
			go refresh("work", otherDone)
			for _, started := range []<-chan struct{}{entered, otherEntered} {
				select {
				case <-started:
				case <-ctx.Done():
					t.Fatal("collection did not start")
				}
			}
			target.Status = StatusDisabled
			target.Error = NewUsageError(ErrorCodeDisabled, "OpenRouter")
			targets := []Target{target}
			if removed {
				targets = nil
			}
			catalog.Replace("personal", targets)
			disabled, err := svc.Envelope(ctx, "personal")
			requireNoError(t, err)
			if removed {
				requireEqual(t, 0, len(disabled.Targets))
			} else {
				requireEqual(t, StatusDisabled, disabled.Targets[0].Status)
				requireEqual(t, false, disabled.Targets[0].Refreshing)
			}
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("canceled collection did not finish")
			}
			once.Do(func() { close(release) })
			otherOnce.Do(func() { close(otherRelease) })
			select {
			case env := <-otherDone:
				requireEqual(t, StatusOK, env.Targets[0].Status)
			case <-ctx.Done():
				t.Fatal("other profile was interrupted")
			}
			requireEqual(t, int32(0), credits.Load())
			requireEqual(t, int32(1), otherCredits.Load())
		})
	}

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

// controlledCredentialProbe gates one completed local credential read to expose lock ownership.
type controlledCredentialProbe struct {
	Collector
	gate atomic.Pointer[credentialProbeGate]
}

type credentialProbeGate struct {
	entered chan struct{}
	release chan struct{}
}

func (c *controlledCredentialProbe) Identity(
	ctx context.Context, target Target, intent Intent,
) (CredentialIdentity, error) {
	identity, err := c.Collector.Identity(ctx, target, intent)
	if gate := c.gate.Swap(nil); gate != nil {
		close(gate.entered)
		select {
		case <-gate.release:
		case <-ctx.Done():
			return CredentialIdentity{}, ctx.Err()
		}
	}
	return identity, err
}

func credentialFenceFixture(t testing.TB) (*Manager, *controlledCredentialProbe, *TargetCatalog, string) {
	t.Helper()
	now := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)
	clock := clockwork.NewFakeClockAt(now)
	home := t.TempDir()
	path := filepath.Join(home, "auth.json")
	token := "fixture." + base64.RawURLEncoding.EncodeToString(
		fmt.Appendf(nil, `{"exp":%d}`, now.Add(24*time.Hour).Unix()),
	) + ".signature"
	data := fmt.Appendf(nil, `{"tokens":{"access_token":%q,"account_id":"fixture-owner"}}`, token)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, codexHTTPFixture); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	collector := &controlledCredentialProbe{Collector: NewCodexCollector(CodexOptions{
		BaseURL: server.URL, Now: clock.Now,
	})}
	catalog := &TargetCatalog{}
	catalog.Replace("personal", []Target{{
		ProfileID: "personal", ProviderID: "codex", ProviderName: "Codex", RouteFingerprint: "fixture",
		Route: ResolvedRoute{Env: []string{"CODEX_HOME=" + home}},
	}})
	service := NewService(Options{
		Resolver: catalog, Collectors: Registry{"codex": collector}, Clock: clock,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	t.Cleanup(func() {
		if err := service.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	env, err := service.Refresh(ctx, RefreshRequest{ProfileID: "personal", Wait: time.Second})
	if err != nil || len(env.Targets) != 1 || env.Targets[0].Status != StatusOK {
		t.Fatalf("seed usage: status=%v err=%v", env.Targets, err)
	}
	return service, collector, catalog, path
}

func BenchmarkUsageCredentialReadContention(b *testing.B) {
	b.ReportAllocs()
	service, collector, _, path := credentialFenceFixture(b)
	var elapsed time.Duration
	iteration := 0
	for b.Loop() {
		iteration++
		changedAt := time.Date(2026, time.November, 1, 12, 0, iteration, 0, time.UTC)
		if err := os.Chtimes(path, changedAt, changedAt); err != nil {
			b.Fatal(err)
		}
		gate := &credentialProbeGate{entered: make(chan struct{}), release: make(chan struct{})}
		collector.gate.Store(gate)
		ownerDone := make(chan error, 1)
		go func() {
			_, err := service.Envelope(b.Context(), "personal")
			ownerDone <- err
		}()
		<-gate.entered
		timer := time.AfterFunc(20*time.Millisecond, func() { close(gate.release) })
		started := time.Now()
		_, err := service.Envelope(b.Context(), "other")
		elapsed += time.Since(started)
		if err != nil {
			b.Error(err)
		}
		if err := <-ownerDone; err != nil {
			b.Error(err)
		}
		timer.Stop()
	}
	b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N), "other-read-ns/op")
}
