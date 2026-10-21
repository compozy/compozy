package providerusage

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const codexHTTPFixture = `{"plan_type":"pro","rate_limit":{"primary_window":{"used_percent":40,"reset_at":1761010920,"limit_window_seconds":18000},"secondary_window":{"used_percent":72,"limit_window_seconds":604800}},"credits":{"balance":"120"}}`

func fixtureAuth(t *testing.T, account string, expiry time.Time) string {
	t.Helper()
	token := "fixture." + base64.RawURLEncoding.EncodeToString(
		[]byte(fmt.Sprintf(`{"exp":%d}`, expiry.Unix())),
	) + ".signature"
	data, err := json.Marshal(
		map[string]any{
			"tokens": map[string]string{
				"access_token":  token,
				"refresh_token": "never-redeem-this-secret",
				"account_id":    account,
			},
		},
	)
	requireNoError(t, err)
	return string(data)
}
func codexTarget(t *testing.T, account string, expiry time.Time) Target {
	t.Helper()
	home := credentialFile(t, "auth.json", fixtureAuth(t, account, expiry))
	return Target{
		ProfileID:        "personal",
		ProviderID:       "codex",
		ProviderName:     "Codex",
		RouteFingerprint: "route-one",
		Route:            ResolvedRoute{Env: []string{"CODEX_HOME=" + home}},
	}
}
func fakeCodex(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("testdata/codex")
	requireNoError(t, err)
	return path
}
func collectFixture(t *testing.T, c *CodexCollector, target Target) (Snapshot, error) {
	t.Helper()
	id, err := c.Identity(t.Context(), target, IntentBackground)
	if err != nil {
		return Snapshot{}, err
	}
	return c.Collect(t.Context(), target, IntentBackground, id)
}

type codexFixtureTransport func(*http.Request) (*http.Response, error)

func (f codexFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCodexCollector(t *testing.T) {
	t.Run("Should preserve the default ChatGPT usage endpoint", func(t *testing.T) {
		t.Parallel()
		clock := fixtureClock(t)
		target := codexTarget(t, "account-a", clock.Now().Add(time.Hour))
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			requireEqual(t, "/backend-api/wham/usage", r.URL.Path)
			writeFixture(w, t, codexHTTPFixture)
		})
		local, err := url.Parse(server.URL)
		requireNoError(t, err)
		transport := codexFixtureTransport(func(r *http.Request) (*http.Response, error) {
			requireEqual(t, "https://chatgpt.com/backend-api/wham/usage", r.URL.String())
			request := r.Clone(r.Context())
			request.URL.Scheme, request.URL.Host = local.Scheme, local.Host
			return http.DefaultTransport.RoundTrip(request)
		})
		_, err = collectFixture(t, NewCodexCollector(CodexOptions{Transport: transport, Now: clock.Now}), target)
		requireNoError(t, err)
	})

	for _, tc := range []struct{ name, base, path string }{
		{"Should honor a configured backend API base path Astra", "/backend-api", "/backend-api/wham/usage"},
		{"Should normalize the backend API trailing slash", "/backend-api/", "/backend-api/wham/usage"},
		{"Should honor a Codex API proxy prefix", "/proxy", "/proxy/api/codex/usage"},
		{"Should honor a Codex API origin", "", "/api/codex/usage"},
		{"Should normalize the ChatGPT origin", "https://chatgpt.com", "/backend-api/wham/usage"},
		{"Should normalize the ChatGPT origin trailing slash", "https://chatgpt.com/", "/backend-api/wham/usage"},
		{"Should normalize the original ChatGPT origin", "https://chat.openai.com", "/backend-api/wham/usage"},
		{"Should normalize the original ChatGPT origin trailing slash", "https://chat.openai.com/", "/backend-api/wham/usage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clock := fixtureClock(t)
			target := codexTarget(t, "account-a", clock.Now().Add(time.Hour))
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					t.Errorf("requested path=%s, want %s", r.URL.Path, tc.path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				writeFixture(w, t, codexHTTPFixture)
			})
			base := tc.base
			if !strings.HasPrefix(base, "https://") {
				base = server.URL + base
			}
			local, err := url.Parse(server.URL)
			requireNoError(t, err)
			transport := codexFixtureTransport(func(r *http.Request) (*http.Response, error) {
				request := r.Clone(r.Context())
				request.URL.Scheme, request.URL.Host = local.Scheme, local.Host
				return http.DefaultTransport.RoundTrip(request)
			})
			config := fmt.Sprintf("chatgpt_base_url = %q\n", base)
			requireNoError(t, os.WriteFile(filepath.Join(codexHome(target), "config.toml"), []byte(config), 0o600))
			_, err = collectFixture(t, NewCodexCollector(CodexOptions{Now: clock.Now, Transport: transport}), target)
			requireNoError(t, err)
		})
	}

	t.Run("Should read RPC windows credits and accept same account owner renewal", func(t *testing.T) {
		t.Parallel()
		clock := fixtureClock(t)
		target := codexTarget(t, "account-a", clock.Now())
		target.Route.NativeCommand = fakeCodex(t)
		replacement := fixtureAuth(t, "account-a", clock.Now().Add(time.Hour))
		requireNoError(t, os.WriteFile(filepath.Join(codexHome(target), "renewal.json"), []byte(replacement), 0o600))
		server := fixtureServer(t, func(w http.ResponseWriter, _ *http.Request) {
			t.Error("unexpected HTTP or token redemption")
			w.WriteHeader(500)
		})
		collector := NewCodexCollector(CodexOptions{BaseURL: server.URL, Now: clock.Now})
		snapshot, err := collectFixture(t, collector, target)
		requireNoError(t, err)
		requireEqual(t, SourceKindCLIRPC, snapshot.Source.Kind)
		requireEqual(t, 2, len(snapshot.Windows))
		requireEqual(t, 23.0, *snapshot.Windows[0].UsedPercent)
		requireEqual(t, int64(18000), *snapshot.Windows[0].DurationSeconds)
		requireEqual(t, 72.0, *snapshot.Windows[1].UsedPercent)
		requireEqual(t, "120", snapshot.Balances[0].Remaining.Value)
		requireEqual(t, "pro", snapshot.Account.Plan)
		data, err := os.ReadFile(filepath.Join(codexHome(target), "auth.json"))
		requireNoError(t, err)
		requireEqual(t, replacement, string(data))
	})
	t.Run("Should use scoped OAuth headers and decode additional model limits", func(t *testing.T) {
		t.Parallel()
		clock := fixtureClock(t)
		target := codexTarget(t, "account-a", clock.Now().Add(time.Hour))
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			requireEqual(t, "/backend-api/wham/usage", r.URL.Path)
			requireEqual(t, "account-a", r.Header.Get("ChatGPT-Account-Id"))
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer fixture.") {
				t.Error("missing scoped bearer")
			}
			_, err := w.Write(
				[]byte(
					`{"rate_limit":{"primary_window":{"used_percent":40,"limit_window_seconds":18000}},"additional_rate_limits":[{"limit_name":"GPT-5.3-Codex-Spark","rate_limit":{"secondary_window":{"used_percent":112,"limit_window_seconds":604800}}}]}`,
				),
			)
			requireNoError(t, err)
		})
		snapshot, err := collectFixture(
			t,
			NewCodexCollector(CodexOptions{BaseURL: server.URL + "/backend-api", Now: clock.Now}),
			target,
		)
		requireNoError(t, err)
		requireEqual(t, SourceKindOAuthHTTP, snapshot.Source.Kind)
		requireEqual(t, "model:gpt-5-3-codex-spark:weekly", snapshot.Windows[1].ID)
		requireEqual(t, 112.0, *snapshot.Windows[1].UsedPercent)
	})
	t.Run("Should reject expired credentials without requests or file mutation", func(t *testing.T) {
		t.Parallel()
		clock := fixtureClock(t)
		target := codexTarget(t, "account-a", clock.Now().Add(5*time.Minute))
		path := filepath.Join(codexHome(target), "auth.json")
		before, err := os.ReadFile(path)
		requireNoError(t, err)
		stat, err := os.Stat(path)
		requireNoError(t, err)
		server := fixtureServer(
			t,
			func(w http.ResponseWriter, _ *http.Request) { t.Error("expired token used"); w.WriteHeader(500) },
		)
		_, err = collectFixture(t, NewCodexCollector(CodexOptions{BaseURL: server.URL, Now: clock.Now}), target)
		requireEqual(t, ErrorCodeAuthExpired, AsUsageError(err, "Codex").Code)
		after, err := os.ReadFile(path)
		requireNoError(t, err)
		afterStat, err := os.Stat(path)
		requireNoError(t, err)
		requireEqual(t, string(before), string(after))
		requireEqual(t, stat.ModTime(), afterStat.ModTime())
	})
	t.Run("Should reject keyring and auto before spawning or requesting", func(t *testing.T) {
		t.Parallel()
		for _, store := range []string{"keyring", "auto"} {
			clock := fixtureClock(t)
			target := codexTarget(t, "account-a", clock.Now().Add(time.Hour))
			target.Route.NativeCommand = fakeCodex(t)
			requireNoError(
				t,
				os.WriteFile(
					filepath.Join(codexHome(target), "config.toml"),
					[]byte("cli_auth_credentials_store = \""+store+"\""),
					0o600,
				),
			)
			server := fixtureServer(
				t,
				func(w http.ResponseWriter, _ *http.Request) { t.Error("keyring HTTP request"); w.WriteHeader(500) },
			)
			_, err := collectFixture(t, NewCodexCollector(CodexOptions{BaseURL: server.URL, Now: clock.Now}), target)
			requireEqual(t, ErrorCodeAuthStoreUnsupported, AsUsageError(err, "Codex").Code)
			_, err = os.Stat(filepath.Join(codexHome(target), "spawned"))
			requireEqual(t, true, os.IsNotExist(err))
		}
	})
	t.Run("Should fall back after initialize failure and classify network errors", func(t *testing.T) {
		t.Parallel()
		clock := fixtureClock(t)
		target := codexTarget(t, "account-a", clock.Now().Add(time.Hour))
		target.Route.NativeCommand = fakeCodex(t)
		requireNoError(t, os.WriteFile(filepath.Join(codexHome(target), "exit"), nil, 0o600))
		server := fixtureServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) })
		server.Close()
		_, err := collectFixture(t, NewCodexCollector(CodexOptions{BaseURL: server.URL, Now: clock.Now}), target)
		requireEqual(t, ErrorCodeNetworkError, AsUsageError(err, "Codex").Code)
	})
	t.Run("Should classify rate limits and malformed responses without leaking bodies", func(t *testing.T) {
		t.Parallel()
		clock := fixtureClock(t)
		target := codexTarget(t, "private-account-id", clock.Now().Add(time.Hour))
		var count atomic.Int32
		server := fixtureServer(t, func(w http.ResponseWriter, _ *http.Request) {
			if count.Add(1) == 1 {
				w.Header().Set("Retry-After", "120")
				w.WriteHeader(429)
			}
			_, err := w.Write([]byte(`{"unexpected":"never-redeem-this-secret"}`))
			requireNoError(t, err)
		})
		collector := NewCodexCollector(CodexOptions{BaseURL: server.URL, Now: clock.Now})
		_, err := collectFixture(t, collector, target)
		usage := AsUsageError(err, "Codex")
		requireEqual(t, ErrorCodeProviderRateLimited, usage.Code)
		requireEqual(t, clock.Now().Add(120*time.Second), *usage.RetryAfter)
		_, err = collectFixture(t, collector, target)
		requireEqual(t, ErrorCodeParseError, AsUsageError(err, "Codex").Code)
		if strings.Contains(err.Error(), "never-redeem") || strings.Contains(err.Error(), "private-account-id") {
			t.Fatal("sensitive error")
		}
	})
}
