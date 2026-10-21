package providerusage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClaudeKeychain struct {
	calls atomic.Int32
	data  string
	err   error
}

var _ KeychainReader = (*fakeClaudeKeychain)(nil)

func (f *fakeClaudeKeychain) Read(context.Context, string, string) ([]byte, error) {
	f.calls.Add(1)
	return []byte(f.data), f.err
}

func claudeFixture(t *testing.T, file bool) (Target, string, string) {
	t.Helper()
	home := t.TempDir()
	config := filepath.Join(home, "route")
	requireNoError(t, os.Mkdir(config, 0o700))
	target := Target{
		ProfileID:        "profile",
		ProviderID:       "claude",
		ProviderName:     "Claude",
		RouteFingerprint: "route-one",
		Route: ResolvedRoute{
			Env:            []string{"HOME=" + home, "CLAUDE_CONFIG_DIR=" + config, "USER=test-user"},
			KeychainAccess: true,
		},
	}
	credential := fmt.Sprintf(
		`{"claudeAiOauth":{"accessToken":"synthetic-secret","expiresAt":%d,"scopes":["user:profile","user:inference"],"subscriptionType":"max","rateLimitTier":"default_claude_max_20x"}}`,
		fixtureClock(t).Now().Add(time.Hour).UnixMilli(),
	)
	if file {
		requireNoError(t, os.WriteFile(filepath.Join(config, ".credentials.json"), []byte(credential), 0o600))
	}
	requireNoError(
		t,
		os.WriteFile(
			filepath.Join(config, ".claude.json"),
			[]byte(`{"oauthAccount":{"accountUuid":"account-A"}}`),
			0o600,
		),
	)
	return target, config, credential
}
func collectClaude(t *testing.T, c Collector, target Target, intent Intent) (Snapshot, error) {
	t.Helper()
	identity, err := c.Identity(t.Context(), target, intent)
	if err != nil {
		return Snapshot{}, err
	}
	return c.Collect(t.Context(), target, intent, identity)
}
func claudeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	_, err := w.Write([]byte(body))
	requireNoError(t, err)
}
func TestClaudeCollector(t *testing.T) {
	t.Run("Should map file credentials and exact extra spending UT-007", func(t *testing.T) {
		t.Parallel()
		target, _, _ := claudeFixture(t, true)
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			requireEqual(t, "/api/oauth/usage", r.URL.Path)
			requireEqual(t, "1", r.URL.Query().Get("cedar_ember"))
			requireEqual(t, "Bearer synthetic-secret", r.Header.Get("Authorization"))
			requireEqual(t, "oauth-2025-04-20", r.Header.Get("Anthropic-Beta"))
			claudeResponse(
				t,
				w,
				`{"five_hour":{"utilization":34,"resets_at":"2026-10-21T05:00:00Z"},"seven_day":{"utilization":61},"seven_day_opus":{"utilization":88},"extra_usage":{"is_enabled":true,"monthly_limit":5000,"used_credits":1240,"currency":"USD"}}`,
			)
		})
		c := NewClaudeCollector(WithClaudeClock(fixtureClock(t).Now), WithClaudeHTTP(server.URL, nil))
		snapshot, err := collectClaude(t, c, target, IntentBackground)
		requireNoError(t, err)
		requireEqual(t, StatusOK, snapshot.Status)
		requireEqual(t, "max_20x", snapshot.Account.Plan)
		requireEqual(t, 3, len(snapshot.Windows))
		requireEqual(t, float64(34), *snapshot.Windows[0].UsedPercent)
		requireEqual(t, "weekly_opus", snapshot.Windows[2].ID)
		requireEqual(t, "Weekly · Opus", snapshot.Windows[2].Label)
		requireEqual(t, "12.40", snapshot.SpendCaps[0].Used.Value)
		requireEqual(t, "50.00", snapshot.SpendCaps[0].Limit.Value)
		requireEqual(t, "Claude Code OAuth", snapshot.Source.Label)
	})
	t.Run("Should reject invalid or expired file credentials without changing them UT-008 UT-009", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name      string
			code      ErrorCode
			transform func(string) string
		}{
			{"Should reject missing scope", ErrorCodeAuthScope, func(s string) string { return strings.ReplaceAll(s, "user:profile", "other") }},
			{"Should reject expired token", ErrorCodeAuthExpired, func(s string) string {
				return strings.ReplaceAll(s, fmt.Sprint(fixtureClock(t).Now().Add(time.Hour).UnixMilli()), "1")
			}},
			{"Should reject malformed file", ErrorCodeParseError, func(string) string { return "{" }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				target, dir, credential := claudeFixture(t, true)
				body := tc.transform(credential)
				path := filepath.Join(dir, ".credentials.json")
				requireNoError(t, os.WriteFile(path, []byte(body), 0o600))
				reader := &fakeClaudeKeychain{}
				c := NewClaudeCollector(WithClaudeClock(fixtureClock(t).Now), WithClaudeKeychain(reader))
				_, err := c.Identity(t.Context(), target, IntentExplicit)
				requireEqual(t, tc.code, AsUsageError(err, "Claude").Code)
				got, err := os.ReadFile(path)
				requireNoError(t, err)
				requireEqual(t, body, string(got))
				requireEqual(t, int32(0), reader.calls.Load())
			})
		}
	})
	t.Run("Should bound cooldowns and classify malformed responses UT-010 UT-011 UT-012", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name   string
			status int
			retry  string
			code   ErrorCode
			delay  time.Duration
			calls  int32
			body   string
		}{
			{"Should default cooldown", 429, "", ErrorCodeProviderRateLimited, 5 * time.Minute, 1, ""},
			{"Should clamp cooldown", 429, "7200", ErrorCodeProviderRateLimited, time.Hour, 1, ""},
			{"Should retry optional query once", 400, "", "", 0, 2, ""},
			{"Should retry forbidden query once", 403, "", "", 0, 2, ""},
			{"Should not retry missing scope", 403, "", ErrorCodeAuthScope, 0, 1, "user:profile"},
			{"Should reject unknown response", 200, "", ErrorCodeParseError, 0, 1, `{"unexpected":true}`},
			{"Should reject unauthorized", 401, "", ErrorCodeAuthMissing, 0, 1, "secret upstream error"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				target, _, _ := claudeFixture(t, true)
				var calls atomic.Int32
				server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if r.URL.RawQuery == "" {
						requireEqual(t, "claude-code/2.1.0", r.Header.Get("User-Agent"))
						claudeResponse(t, w, `{"five_hour":{"utilization":1}}`)
						return
					}
					w.Header().Set("Retry-After", tc.retry)
					w.WriteHeader(tc.status)
					claudeResponse(t, w, tc.body)
				})
				clock := fixtureClock(t)
				c := NewClaudeCollector(WithClaudeClock(clock.Now), WithClaudeHTTP(server.URL, nil))
				_, err := collectClaude(t, c, target, IntentBackground)
				if tc.code == "" {
					requireNoError(t, err)
				} else {
					usage := AsUsageError(err, "Claude")
					requireEqual(t, tc.code, usage.Code)
					if tc.delay > 0 {
						requireEqual(t, clock.Now().Add(tc.delay), *usage.RetryAfter)
					}
				}
				requireEqual(t, tc.calls, calls.Load())
			})
		}
	})
}

func TestClaudeKeychain(t *testing.T) {
	t.Run("Should clear one profile or all cached logins without background Keychain reads", func(t *testing.T) {
		t.Parallel()
		first, _, credential := claudeFixture(t, false)
		second, _, _ := claudeFixture(t, false)
		second.ProfileID = "second-profile"
		reader := &fakeClaudeKeychain{data: credential}
		c := NewClaudeCollector(WithClaudeClock(fixtureClock(t).Now), WithClaudeKeychain(reader))
		c.platform = "darwin"
		t.Cleanup(func() {
			if err := c.Close(); err != nil {
				t.Error(err)
			}
		})
		for _, target := range []Target{first, second} {
			_, err := c.Identity(t.Context(), target, IntentExplicit)
			requireNoError(t, err)
		}
		c.ClearCredentials(first.ProfileID)
		if _, err := c.Identity(t.Context(), first, IntentBackground); err == nil {
			t.Fatal("cleared login reused")
		}
		_, err := c.Identity(t.Context(), second, IntentBackground)
		requireNoError(t, err)
		_, err = c.Identity(t.Context(), first, IntentExplicit)
		requireNoError(t, err)
		c.ClearCredentials("")
		for _, target := range []Target{first, second} {
			if _, err := c.Identity(t.Context(), target, IntentBackground); err == nil {
				t.Fatal("globally cleared login reused")
			}
		}
		requireEqual(t, int32(3), reader.calls.Load())
	})

	t.Run("Should require consent and explicit intent UT-013 UT-015 UT-016 UT-017", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name, platform string
			consent        bool
			intent         Intent
			denied         bool
			code           ErrorCode
			calls          int32
		}{
			{"Should require consent", "darwin", false, IntentExplicit, false, ErrorCodeKeychainConsentRequired, 0},
			{"Should never read on background", "darwin", true, IntentBackground, false, ErrorCodeKeychainConsentRequired, 0},
			{"Should sanitize denied reader", "darwin", true, IntentExplicit, true, ErrorCodeKeychainDenied, 1},
			{"Should not read on Linux", "linux", true, IntentExplicit, false, ErrorCodeAuthMissing, 0},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				target, _, credential := claudeFixture(t, false)
				target.Route.KeychainAccess = tc.consent
				reader := &fakeClaudeKeychain{data: credential}
				if tc.denied {
					reader.err = errors.New("sensitive OS output")
				}
				c := NewClaudeCollector(WithClaudeClock(fixtureClock(t).Now), WithClaudeKeychain(reader))
				c.platform = tc.platform
				_, err := c.Identity(t.Context(), target, tc.intent)
				requireEqual(t, tc.code, AsUsageError(err, "Claude").Code)
				requireEqual(t, tc.calls, reader.calls.Load())
				_, err = c.Identity(t.Context(), target, IntentBackground)
				if err == nil {
					t.Fatal("unexpected background credential")
				}
				requireEqual(t, tc.calls, reader.calls.Load())
			})
		}
	})
	t.Run("Should reuse only while account evidence holds UT-014 UT-015 UT-079 UT-080", func(t *testing.T) {
		t.Parallel()
		for _, change := range []string{"account", "deleted", "malformed", "expiry", "route", "consent", "close", "clear"} {
			t.Run("Should invalidate on "+change, func(t *testing.T) {
				t.Parallel()
				target, dir, credential := claudeFixture(t, false)
				reader := &fakeClaudeKeychain{data: credential}
				clock := fixtureClock(t)
				server := fixtureServer(
					t,
					func(w http.ResponseWriter, _ *http.Request) { claudeResponse(t, w, `{"five_hour":{"utilization":42}}`) },
				)
				c := NewClaudeCollector(
					WithClaudeClock(clock.Now),
					WithClaudeKeychain(reader),
					WithClaudeHTTP(server.URL, nil),
				)
				c.platform = "darwin"
				_, err := collectClaude(t, c, target, IntentExplicit)
				requireNoError(t, err)
				_, err = collectClaude(t, c, target, IntentBackground)
				requireNoError(t, err)
				path := filepath.Join(dir, ".claude.json")
				switch change {
				case "account":
					requireNoError(t, os.WriteFile(path, []byte(`{"oauthAccount":{"accountUuid":"B"}}`), 0o600))
				case "deleted":
					requireNoError(t, os.Remove(path))
				case "malformed":
					requireNoError(t, os.WriteFile(path, []byte("{"), 0o600))
				case "expiry":
					clock.Advance(2 * time.Hour)
				case "route":
					target.RouteFingerprint = "changed"
				case "consent":
					target.Route.KeychainAccess = false
				case "close":
					requireNoError(t, c.Close())
				case "clear":
					c.ClearCredentials(target.ProfileID)
				}
				_, err = collectClaude(t, c, target, IntentBackground)
				if err == nil {
					t.Fatal("reused invalid token")
				}
				requireEqual(t, int32(1), reader.calls.Load())
			})
		}
	})
	t.Run("Should serve indeterminate evidence only once UT-080", func(t *testing.T) {
		t.Parallel()
		target, dir, credential := claudeFixture(t, false)
		requireNoError(t, os.Remove(filepath.Join(dir, ".claude.json")))
		reader := &fakeClaudeKeychain{data: credential}
		server := fixtureServer(
			t,
			func(w http.ResponseWriter, _ *http.Request) { claudeResponse(t, w, `{"five_hour":{"utilization":4}}`) },
		)
		c := NewClaudeCollector(
			WithClaudeClock(fixtureClock(t).Now),
			WithClaudeKeychain(reader),
			WithClaudeHTTP(server.URL, nil),
		)
		c.platform = "darwin"
		identity, err := c.Identity(t.Context(), target, IntentExplicit)
		requireNoError(t, err)
		_, err = c.Collect(t.Context(), target, IntentExplicit, identity)
		requireNoError(t, err)
		requireNoError(t, c.ValidateIdentity(t.Context(), target, identity))
		_, err = c.Identity(t.Context(), target, IntentBackground)
		requireEqual(t, ErrorCodeKeychainTokenExpired, AsUsageError(err, "Claude").Code)
		requireEqual(t, int32(1), reader.calls.Load())
	})
	t.Run("Should reject an account switch during HTTP UT-073 UT-081", func(t *testing.T) {
		t.Parallel()
		target, dir, credential := claudeFixture(t, false)
		reader := &fakeClaudeKeychain{data: credential}
		server := fixtureServer(t, func(w http.ResponseWriter, _ *http.Request) {
			requireNoError(
				t,
				os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(`{"oauthAccount":{"accountUuid":"B"}}`), 0o600),
			)
			claudeResponse(t, w, `{"five_hour":{"utilization":4}}`)
		})
		c := NewClaudeCollector(
			WithClaudeClock(fixtureClock(t).Now),
			WithClaudeKeychain(reader),
			WithClaudeHTTP(server.URL, nil),
		)
		c.platform = "darwin"
		snapshot, err := collectClaude(t, c, target, IntentExplicit)
		requireEqual(t, ErrorCodeKeychainTokenExpired, AsUsageError(err, "Claude").Code)
		requireEqual(t, 0, len(snapshot.Windows))
		requireEqual(t, int32(1), reader.calls.Load())
	})
}

func TestClaudeIsolation(t *testing.T) {
	t.Run("Should resolve exact secure storage service and account", func(t *testing.T) {
		t.Parallel()
		target, _, _ := claudeFixture(t, false)
		target.Route.Env = []string{"HOME=/synthetic/home", "USER=fixture", "CLAUDE_CONFIG_DIR=/synthetic/cafe\u0301/"}
		service, account := claudeKeychainItem(target)
		equivalent := target
		equivalent.Route.Env = []string{"HOME=/synthetic/home", "USER=fixture", "CLAUDE_CONFIG_DIR=/synthetic/café/"}
		normalized, _ := claudeKeychainItem(equivalent)
		requireEqual(t, service, normalized)
		requireEqual(t, "fixture", account)
		if !strings.HasPrefix(service, "Claude Code-credentials-") {
			t.Fatal("missing isolated service hash")
		}
		target.Route.Env = append(target.Route.Env, "CLAUDE_SECURESTORAGE_CONFIG_DIR=", "USER=bad;name")
		service, account = claudeKeychainItem(target)
		requireEqual(t, "Claude Code-credentials", service)
		requireEqual(t, "claude-code-user", account)
		requireEqual(t, "/synthetic/home/.claude/.credentials.json", claudeCredentialPath(target))
		target.Route.Env = append(target.Route.Env, "CLAUDE_SECURESTORAGE_CONFIG_DIR=/synthetic/secure")
		requireEqual(t, "/synthetic/secure/.credentials.json", claudeCredentialPath(target))
		secureService, _ := claudeKeychainItem(target)
		if secureService == service {
			t.Fatal("secure override not isolated")
		}
	})
	t.Run("Should select legacy account evidence without falling through on logout UT-080", func(t *testing.T) {
		t.Parallel()
		target, dir, _ := claudeFixture(t, false)
		legacy := filepath.Join(dir, ".config.json")
		requireNoError(t, os.WriteFile(legacy, []byte(`{"oauthAccount":{"accountUuid":"legacy"}}`), 0o600))
		path, account := claudeAccountEvidence(target)
		requireEqual(t, legacy, path)
		requireEqual(t, "legacy", account)
		requireNoError(t, os.WriteFile(legacy, []byte(`{}`), 0o600))
		_, account = claudeAccountEvidence(target)
		requireEqual(t, "", account)
		home := t.TempDir()
		requireNoError(t, os.Mkdir(filepath.Join(home, ".claude"), 0o700))
		target.Route.Env = []string{"HOME=" + home}
		requireNoError(
			t,
			os.WriteFile(
				filepath.Join(home, ".claude", ".config.json"),
				[]byte(`{"oauthAccount":{"accountUuid":"default-legacy"}}`),
				0o600,
			),
		)
		_, account = claudeAccountEvidence(target)
		requireEqual(t, "default-legacy", account)
	})
	t.Run("Should reject changed file material before publication UT-071 UT-072 UT-073 UT-074", func(t *testing.T) {
		t.Parallel()
		target, dir, credential := claudeFixture(t, true)
		c := NewClaudeCollector(WithClaudeClock(fixtureClock(t).Now))
		before, err := c.Identity(t.Context(), target, IntentBackground)
		requireNoError(t, err)
		requireNoError(
			t,
			os.WriteFile(
				filepath.Join(dir, ".credentials.json"),
				[]byte(strings.ReplaceAll(credential, "synthetic-secret", "replacement-secret")),
				0o600,
			),
		)
		err = c.ValidateIdentity(t.Context(), target, before)
		requireEqual(t, ErrorCodeAuthMissing, AsUsageError(err, "Claude").Code)
		after, err := c.Identity(t.Context(), target, IntentBackground)
		requireNoError(t, err)
		requireEqual(t, false, before.Digest == after.Digest)
		requireEqual(t, true, before.Same(after))
		requireNoError(
			t,
			os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(`{"oauthAccount":{"accountUuid":"B"}}`), 0o600),
		)
		next, err := c.Identity(t.Context(), target, IntentBackground)
		requireNoError(t, err)
		requireEqual(t, false, after.Same(next))
	})
	t.Run("Should not reuse keychain credentials across profiles or a new file", func(t *testing.T) {
		t.Parallel()
		target, dir, credential := claudeFixture(t, false)
		reader := &fakeClaudeKeychain{data: credential}
		c := NewClaudeCollector(WithClaudeClock(fixtureClock(t).Now), WithClaudeKeychain(reader))
		c.platform = "darwin"
		identity, err := c.Identity(t.Context(), target, IntentExplicit)
		requireNoError(t, err)
		other := target
		other.ProfileID = "other"
		_, err = c.Identity(t.Context(), other, IntentBackground)
		requireEqual(t, ErrorCodeKeychainConsentRequired, AsUsageError(err, "Claude").Code)
		requireNoError(t, os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(credential), 0o600))
		err = c.ValidateIdentity(t.Context(), target, identity)
		requireEqual(t, ErrorCodeKeychainTokenExpired, AsUsageError(err, "Claude").Code)
		requireEqual(t, int32(1), reader.calls.Load())
	})
	t.Run("Should avoid ambient files when route home is absent", func(t *testing.T) {
		t.Parallel()
		target := Target{Route: ResolvedRoute{Env: []string{}}}
		requireEqual(t, "", claudeCredentialPath(target))
		_, account := claudeAccountEvidence(target)
		requireEqual(t, "", account)
	})
}

func TestClaudeMapping(t *testing.T) {
	t.Run("Should retain overage unknown percentages and malformed reset text", func(t *testing.T) {
		t.Parallel()
		snapshot, err := decodeClaudeUsage(
			[]byte(
				`{"five_hour":{"utilization":134,"resets_at":"tomorrow"},"seven_day":{"utilization":null},"seven_day_sonnet":{"utilization":-1},"routines":{"utilization":5}}`,
			),
		)
		requireNoError(t, err)
		requireEqual(t, float64(134), *snapshot.Windows[0].UsedPercent)
		requireEqual(t, "tomorrow", snapshot.Windows[0].ResetText)
		requireEqual(t, true, snapshot.Windows[1].UsedPercent == nil)
		requireEqual(t, true, snapshot.Windows[2].UsedPercent == nil)
		requireEqual(t, "routines", snapshot.Windows[3].ID)
	})
	t.Run("Should preserve valid scoped windows beside malformed siblings", func(t *testing.T) {
		t.Parallel()
		snapshot, err := decodeClaudeUsage(
			[]byte(
				`{"limits":[12,{"percent":"bad"},{"kind":"weekly_scoped","group":"weekly","percent":33,"scope":{"model":{"id":"fable-v1","display_name":"Fable"}}},{"kind":"weekly_scoped","group":"weekly","percent":33,"scope":{"model":{"id":"fable-v1","display_name":"Fable"}}},{"kind":"weekly_scoped","group":"weekly","percent":2,"scope":{"model":{"display_name":"All models"}}}]}`,
			),
		)
		requireNoError(t, err)
		requireEqual(t, 1, len(snapshot.Windows))
		requireEqual(t, "model:fable-v1:weekly", snapshot.Windows[0].ID)
	})
	t.Run("Should preserve decimal money without rounding", func(t *testing.T) {
		t.Parallel()
		snapshot, err := decodeClaudeUsage(
			[]byte(`{"extra_usage":{"used_credits":1234.567,"monthly_limit":5e3,"utilization":-1,"is_enabled":false}}`),
		)
		requireNoError(t, err)
		spendCap := snapshot.SpendCaps[0]
		requireEqual(t, "12.34567", spendCap.Used.Value)
		requireEqual(t, "50.00", spendCap.Limit.Value)
		requireEqual(t, "USD", spendCap.Used.Unit)
		requireEqual(t, false, *spendCap.Enabled)
		requireEqual(t, true, spendCap.UsedPercent == nil)
	})
	t.Run("Should reject malformed usage without inventing zero", func(t *testing.T) {
		t.Parallel()
		for _, body := range []string{`null`, `[]`, `{}`, `{"five_hour":false}`, `{"extra_usage":{"used_credits":-1}}`, `{"extra_usage":{"used_credits":"secret"}}`} {
			_, err := decodeClaudeUsage([]byte(body))
			requireEqual(t, ErrorCodeParseError, AsUsageError(err, "Claude").Code)
		}
	})
}
