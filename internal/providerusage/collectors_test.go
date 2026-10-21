package providerusage

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type collectorFixture struct {
	id        string
	responses map[string]string
	windows   int
}

func collectorFixtures() []collectorFixture {
	return []collectorFixture{
		{
			"openrouter",
			map[string]string{
				"/api/v1/key":     `{"data":{"limit":100,"usage":37.5}}`,
				"/api/v1/credits": `{"data":{"total_credits":50,"total_usage":12.5}}`,
			},
			0,
		},
		{"vercel-ai-gateway", map[string]string{"/v1/credits": `{"balance":"84.20","total_used":"15.80"}`}, 0},
		{
			"zai",
			map[string]string{
				"/api/monitor/usage/quota/limit": `{"success":true,"code":200,"data":{"planName":"Pro","limits":[{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":40,"nextResetTime":1792548000000},{"type":"TOKENS_LIMIT","unit":6,"number":1,"percentage":10}]}}`,
			},
			2,
		},
		{
			"moonshot",
			map[string]string{
				"/coding/v1/usages": `{"usages":{"limit_5h":{"used_ratio":0.4},"limit_7d":{"used_ratio":0.1},"limit_month_total":{"used_ratio":1.12,"reset_time":"2026-11-01T00:00:00Z"}}}`,
			},
			3,
		},
		{
			"kimi-cli",
			map[string]string{
				"/coding/v1/usages": `{"usages":{"limit_5h":{"used_ratio":0.4},"limit_7d":{"used_ratio":0.1},"limit_month_total":{"used_ratio":0.7}}}`,
			},
			3,
		},
		{
			"cursor",
			map[string]string{
				"/api/usage-summary": `{"membershipType":"pro","billingCycleEnd":"2026-11-01T00:00:00Z","individualUsage":{"plan":{"totalPercentUsed":62},"onDemand":{"used":810,"limit":2000}}}`,
				"/api/auth/me":       `{"email":"fixture@example.test","sub":"fixture-user"}`,
			},
			1,
		},
		{
			"gemini",
			map[string]string{
				"/v1internal:loadCodeAssist":    `{"cloudaicompanionProject":"fixture-project","paidTier":{"name":"Code Assist"}}`,
				"/v1internal:retrieveUserQuota": `{"buckets":[{"modelId":"pro","remainingFraction":0.25,"resetTime":"2026-10-22T00:00:00Z"},{"modelId":"flash","remainingFraction":0.9,"resetTime":"2026-10-22T00:00:00Z"}]}`,
			},
			2,
		},
	}
}
func collectorTestTarget(t *testing.T, id string, expired bool) Target {
	t.Helper()
	home := t.TempDir()
	now := fixtureClock(t).Now()
	expiry := now.Add(time.Hour)
	if expired {
		expiry = now.Add(-time.Hour)
	}
	claims, err := json.Marshal(
		map[string]any{"sub": "fixture-user", "email": "fixture@example.test", "exp": expiry.Unix()},
	)
	requireNoError(t, err)
	token := "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString(claims) + ".fixture"
	target := Target{
		ProviderID:   id,
		ProviderName: id,
		Route: ResolvedRoute{
			Env:          []string{"HOME=" + home},
			BoundSecrets: map[string]string{"api_key": "fixture-bound-secret"},
		},
	}
	var path string
	var payload any
	switch id {
	case "cursor":
		path = filepath.Join(home, ".config", "cursor", "auth.json")
		payload = map[string]any{"accessToken": token}
	case "gemini":
		path = filepath.Join(home, ".gemini", "oauth_creds.json")
		payload = map[string]any{
			"access_token":  "fixture-access-secret",
			"id_token":      token,
			"refresh_token": "never-redeem",
			"expiry_date":   expiry.UnixMilli(),
		}
	case "kimi-cli":
		path = filepath.Join(home, ".kimi-code", "credentials", "kimi-code.json")
		payload = map[string]any{"access_token": token, "refresh_token": "never-redeem", "expires_at": expiry.Unix()}
	default:
		return target
	}
	requireNoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	data, err := json.Marshal(payload)
	requireNoError(t, err)
	requireNoError(t, os.WriteFile(path, data, 0o600))
	return target
}
func collectorTestRegistry(t *testing.T, url string) Registry {
	t.Helper()
	bases := map[string]string{}
	for _, id := range []string{"codex", "claude", "openrouter", "vercel-ai-gateway", "zai", "moonshot", "kimi-cli", "cursor", "gemini"} {
		bases[id] = url
	}
	return DefaultRegistry(RegistryOptions{BaseURLs: bases, Now: fixtureClock(t).Now})
}
func collectorRun(t *testing.T, c Collector, target Target) (Snapshot, error) {
	t.Helper()
	cred, err := c.Identity(t.Context(), target, IntentBackground)
	if err != nil {
		return Snapshot{}, err
	}
	return c.Collect(t.Context(), target, IntentBackground, cred)
}
func writeCollectorResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	_, err := w.Write([]byte(body))
	requireNoError(t, err)
}
func TestCollectors(t *testing.T) {
	t.Run("Should register every supported provider without probing credentials", func(t *testing.T) {
		t.Parallel()
		registry := DefaultRegistry(RegistryOptions{})
		ids := []string{
			"codex",
			"claude",
			"openrouter",
			"vercel-ai-gateway",
			"zai",
			"moonshot",
			"kimi-cli",
			"cursor",
			"gemini",
		}
		requireEqual(t, len(ids), len(registry))
		for _, id := range ids {
			collector := registry[id]
			if collector == nil {
				t.Fatalf("missing collector %s", id)
			}
			requireEqual(t, id, collector.ProviderID())
		}
	})

	for _, fixture := range collectorFixtures() {
		t.Run("Should translate "+fixture.id+" usage from its resolved credential", func(t *testing.T) {
			t.Parallel()
			target := collectorTestTarget(t, fixture.id, false)
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				body, ok := fixture.responses[r.URL.Path]
				if !ok {
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(404)
					return
				}
				if fixture.id == "cursor" {
					if !strings.HasPrefix(r.Header.Get("Cookie"), "WorkosCursorSessionToken=fixture-user%3A%3A") {
						t.Error("missing derived CLI session")
					}
				} else if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
					t.Error("missing bearer")
				}
				if fixture.id == "kimi-cli" {
					requireEqual(t, "kimi_code_cli", r.Header.Get("X-Msh-Platform"))
				}
				if fixture.id == "gemini" {
					requireEqual(t, http.MethodPost, r.Method)
					if r.URL.Path == "/v1internal:retrieveUserQuota" {
						var body struct {
							Project string `json:"project"`
						}
						requireNoError(t, json.NewDecoder(r.Body).Decode(&body))
						requireEqual(t, "fixture-project", body.Project)
					}
				}
				writeCollectorResponse(t, w, body)
			})
			snapshot, err := collectorRun(t, collectorTestRegistry(t, server.URL)[fixture.id], target)
			requireNoError(t, err)
			requireEqual(t, StatusOK, snapshot.Status)
			requireEqual(t, fixture.windows, len(snapshot.Windows))
			requireEqual(t, fixtureClock(t).Now(), *snapshot.MeasuredAt)
			switch fixture.id {
			case "openrouter":
				requireEqual(t, "37.50", snapshot.Balances[0].Remaining.Value)
				requireEqual(t, "37.5", snapshot.SpendCaps[0].Used.Value)
				requireEqual(t, "100", snapshot.SpendCaps[0].Limit.Value)
			case "vercel-ai-gateway":
				requireEqual(t, "84.20", snapshot.Balances[0].Remaining.Value)
				requireEqual(t, "15.80", snapshot.SpendCaps[0].Used.Value)
				requireEqual(t, (*Amount)(nil), snapshot.SpendCaps[0].Limit)
			case "zai":
				requireEqual(t, 40.0, *snapshot.Windows[0].UsedPercent)
				requireEqual(t, "weekly", snapshot.Windows[1].ID)
			case "moonshot", "kimi-cli":
				requireEqual(t, "Kimi Coding", snapshot.ProviderName)
				requireEqual(t, (*Amount)(nil), snapshot.Windows[0].Used)
				requireEqual(t, (*int64)(nil), snapshot.Windows[2].DurationSeconds)
			case "cursor":
				requireEqual(t, 62.0, *snapshot.Windows[0].UsedPercent)
				requireEqual(t, "8.10", snapshot.SpendCaps[0].Used.Value)
				requireEqual(t, "20.00", snapshot.SpendCaps[0].Limit.Value)
				requireEqual(t, "2026-11-01T00:00:00Z", snapshot.Windows[0].ResetsAt.Format(time.RFC3339))
			case "gemini":
				requireEqual(t, "model:flash", snapshot.Windows[0].ID)
				if math.Abs(*snapshot.Windows[0].UsedPercent-10) > 1e-9 {
					t.Fatal("wrong fraction conversion")
				}
				requireEqual(t, 75.0, *snapshot.Windows[1].UsedPercent)
				requireEqual(t, (*int64)(nil), snapshot.Windows[0].DurationSeconds)
			}
		})
	}
	for _, fixture := range collectorFixtures() {
		for _, failure := range []struct {
			name   string
			status int
			body   string
			code   ErrorCode
		}{
			{"unauthorized", 401, `fixture-private-body`, ErrorCodeAuthMissing},
			{"rate limited", 429, `fixture-private-body`, ErrorCodeProviderRateLimited},
			{"unavailable", 503, `fixture-private-body`, ErrorCodeProviderUnavailable},
			{"malformed", 200, `{`, ErrorCodeParseError},
			{"unknown shape", 200, `{"unexpected":true}`, ErrorCodeParseError},
			{"oversized", 200, strings.Repeat("x", MaxResponseBytes+1), ErrorCodeParseError},
		} {
			t.Run("Should classify "+fixture.id+" "+failure.name, func(t *testing.T) {
				t.Parallel()
				server := fixtureServer(t, func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Retry-After", "120")
					w.WriteHeader(failure.status)
					writeCollectorResponse(t, w, failure.body)
				})
				_, err := collectorRun(
					t,
					collectorTestRegistry(t, server.URL)[fixture.id],
					collectorTestTarget(t, fixture.id, false),
				)
				if err == nil {
					t.Fatal("expected typed error")
				}
				usage := AsUsageError(err, fixture.id)
				requireEqual(t, failure.code, usage.Code)
				if failure.status == 401 {
					requireEqual(t, StatusSignedOut, usage.Status())
				}
				if failure.status == 429 {
					requireEqual(t, fixtureClock(t).Now().Add(2*time.Minute), *usage.RetryAfter)
				}
				if strings.Contains(usage.Message, "fixture-") {
					t.Fatal("private response leaked")
				}
			})
		}
	}
}
func TestCollectorCredentials(t *testing.T) {
	for _, id := range []string{"cursor", "gemini", "kimi-cli"} {
		t.Run("Should reject expired "+id+" credentials without writing or requesting", func(t *testing.T) {
			t.Parallel()
			target := collectorTestTarget(t, id, true)
			before := map[string]string{}
			mtimes := map[string]time.Time{}
			home := routeEnv(target, "HOME")
			requireNoError(t, filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				before[path] = string(data)
				mtimes[path] = info.ModTime()
				return nil
			}))
			server := fixtureServer(
				t,
				func(w http.ResponseWriter, _ *http.Request) { t.Error("expired credential sent"); w.WriteHeader(500) },
			)
			_, err := collectorRun(t, collectorTestRegistry(t, server.URL)[id], target)
			requireEqual(t, ErrorCodeAuthExpired, AsUsageError(err, id).Code)
			for path, content := range before {
				data, err := os.ReadFile(path)
				requireNoError(t, err)
				requireEqual(t, content, string(data))
				info, err := os.Stat(path)
				requireNoError(t, err)
				requireEqual(t, mtimes[path], info.ModTime())
			}
		})
	}
	t.Run("Should fence a replaced bound key before sending it", func(t *testing.T) {
		t.Parallel()
		target := collectorTestTarget(t, "openrouter", false)
		c := NewOpenRouterCollector()
		a, err := c.Identity(t.Context(), target, IntentBackground)
		requireNoError(t, err)
		target.Route.BoundSecrets["api_key"] = "replacement"
		b, err := c.Identity(t.Context(), target, IntentBackground)
		requireNoError(t, err)
		requireEqual(t, false, a.Same(b))
		_, err = c.Collect(t.Context(), target, IntentBackground, a)
		requireEqual(t, ErrorCodeAuthMissing, AsUsageError(err, "OpenRouter").Code)
	})
	t.Run("Should never fall back to credential environment variables", func(t *testing.T) {
		t.Parallel()
		for _, id := range []string{"openrouter", "vercel-ai-gateway", "zai", "moonshot"} {
			target := Target{ProviderID: id, Route: ResolvedRoute{Env: []string{"OPENROUTER_API_KEY=ambient"}}}
			_, err := collectorTestRegistry(t, "")[id].Identity(t.Context(), target, IntentBackground)
			requireEqual(t, ErrorCodeAuthMissing, AsUsageError(err, id).Code)
		}
	})
}
func TestCollectorBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, id, path, body string
		code                 ErrorCode
		check                func(*testing.T, Snapshot)
	}{
		{"unknown Cursor total from partial percentages", "cursor", "/api/usage-summary", `{"individualUsage":{"plan":{"autoPercentUsed":100,"apiPercentUsed":0}}}`, "", func(t *testing.T, s Snapshot) {
			t.Helper()
			requireEqual(t, (*float64)(nil), s.Windows[0].UsedPercent)
		}},
		{"no z.ai plan", "zai", "/api/monitor/usage/quota/limit", `{"success":true,"code":200,"data":{}}`, ErrorCodeNoPlan, nil},
		{"uncapped Cursor spend", "cursor", "/api/usage-summary", `{"individualUsage":{"onDemand":{"used":0}}}`, "", func(t *testing.T, s Snapshot) {
			t.Helper()
			requireEqual(t, (*Amount)(nil), s.SpendCaps[0].Limit)
			requireEqual(t, "0.00", s.SpendCaps[0].Used.Value)
		}},
		{"uncapped OpenRouter spend", "openrouter", "/api/v1/key", `{"data":{"limit":null,"usage":0}}`, "", func(t *testing.T, s Snapshot) {
			t.Helper()
			requireEqual(t, (*Amount)(nil), s.SpendCaps[0].Limit)
			requireEqual(t, (*float64)(nil), s.SpendCaps[0].UsedPercent)
		}},
		{"negative Vercel balance", "vercel-ai-gateway", "/v1/credits", `{"balance":"-0.123456789012345678","total_used":"10.20"}`, "", func(t *testing.T, s Snapshot) {
			t.Helper()
			requireEqual(t, "-0.123456789012345678", s.Balances[0].Remaining.Value)
		}},
		{"legacy Kimi counts", "moonshot", "/coding/v1/usages", `{"usage":{"limit":"100","used":"25"},"limits":[{"window":{"duration":300,"timeUnit":"TIME_UNIT_MINUTE"},"detail":{"limit":"20","used":"5"}}]}`, "", func(t *testing.T, s Snapshot) {
			t.Helper()
			requireEqual(t, 2, len(s.Windows))
			requireEqual(t, "requests", s.Windows[0].Used.Unit)
			requireEqual(t, 25.0, *s.Windows[0].UsedPercent)
		}},
	} {
		t.Run("Should preserve "+tc.name, func(t *testing.T) {
			t.Parallel()
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				body := tc.body
				if r.URL.Path != tc.path {
					switch r.URL.Path {
					case "/api/auth/me":
						body = `{"sub":"fixture-user"}`
					case "/api/v1/credits":
						body = `{"data":{"total_credits":0,"total_usage":0}}`
					default:
						t.Errorf("unexpected %s", r.URL.Path)
					}
				}
				writeCollectorResponse(t, w, body)
			})
			snapshot, err := collectorRun(
				t,
				collectorTestRegistry(t, server.URL)[tc.id],
				collectorTestTarget(t, tc.id, false),
			)
			if tc.code != "" {
				requireEqual(t, tc.code, AsUsageError(err, tc.id).Code)
				requireEqual(t, string(ActionConfigure), AsUsageError(err, tc.id).Action)
				return
			}
			requireNoError(t, err)
			tc.check(t, snapshot)
		})
	}
	t.Run("Should retain Gemini plan when quota is unavailable", func(t *testing.T) {
		t.Parallel()
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "loadCodeAssist") {
				writeCollectorResponse(t, w, `{"paidTier":{"name":"Code Assist"}}`)
				return
			}
			w.WriteHeader(http.StatusForbidden)
		})
		snapshot, err := collectorRun(
			t,
			collectorTestRegistry(t, server.URL)["gemini"],
			collectorTestTarget(t, "gemini", false),
		)
		requireEqual(t, ErrorCodeProviderUnavailable, AsUsageError(err, "Gemini").Code)
		requireEqual(t, StatusError, snapshot.Status)
		requireEqual(t, "Code Assist", snapshot.Account.Plan)
	})
	t.Run("Should subtract decimal balances without float rounding", func(t *testing.T) {
		t.Parallel()
		requireEqual(t, "0.10", string(collectorSubtract("0.30", "0.20", 2)))
		requireEqual(t, "1.2345", collectorCents("123.45").Value)
		requireEqual(
			t,
			"[redacted credential identity]",
			fmt.Sprint(NewCredentialIdentity(SourceKindAPIKeyHTTP, []byte("secret"), "")),
		)
	})
}

func TestCollectorIsolation(t *testing.T) {
	for _, id := range []string{"cursor", "gemini", "kimi-cli"} {
		t.Run("Should reject absent and malformed "+id+" stores without ambient fallback", func(t *testing.T) {
			t.Parallel()
			collector := collectorTestRegistry(t, "http://unused.invalid")[id]
			for _, target := range []Target{{ProviderID: id}, {ProviderID: id, Route: ResolvedRoute{Env: []string{"HOME=relative"}}}, {ProviderID: id, Route: ResolvedRoute{Env: []string{"HOME=" + t.TempDir()}}}} {
				_, err := collector.Identity(t.Context(), target, IntentExplicit)
				requireEqual(t, ErrorCodeAuthMissing, AsUsageError(err, id).Code)
			}
			target := collectorTestTarget(t, id, false)
			requireNoError(
				t,
				filepath.WalkDir(routeEnv(target, "HOME"), func(path string, entry os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !entry.IsDir() {
						return os.WriteFile(path, []byte(`{`), 0o600)
					}
					return nil
				}),
			)
			_, err := collector.Identity(t.Context(), target, IntentExplicit)
			requireEqual(t, ErrorCodeParseError, AsUsageError(err, id).Code)
		})
	}
	t.Run("Should use the explicit Cursor config home and reject a changed account", func(t *testing.T) {
		t.Parallel()
		target := collectorTestTarget(t, "cursor", false)
		home := routeEnv(target, "HOME")
		target.Route.Env = []string{"HOME=" + t.TempDir(), "XDG_CONFIG_HOME=" + filepath.Join(home, ".config")}
		collector := NewCursorCollector(WithCollectorClock(fixtureClock(t).Now))
		cred, err := collector.Identity(t.Context(), target, IntentExplicit)
		requireNoError(t, err)
		foreign := NewCredentialIdentity(SourceKindOAuthHTTP, []byte("other"), "different-account")
		requireEqual(t, false, cred.Same(foreign))
		_, err = collector.Collect(t.Context(), target, IntentExplicit, foreign)
		requireEqual(t, ErrorCodeAuthMissing, AsUsageError(err, "Cursor").Code)
	})
	t.Run("Should prefer z.ai counts and omit implausible five hour reset", func(t *testing.T) {
		t.Parallel()
		now := fixtureClock(t).Now()
		body := fmt.Sprintf(
			`{"success":true,"code":200,"data":{"limits":[{"type":"CREDIT_LIMIT","unit":3,"number":5,"percentage":5,"usage":100,"currentValue":120,"remaining":10,"nextResetTime":%d}]}}`,
			now.Add(6*time.Hour).UnixMilli(),
		)
		server := fixtureServer(t, func(w http.ResponseWriter, _ *http.Request) { writeCollectorResponse(t, w, body) })
		snapshot, err := collectorRun(
			t,
			collectorTestRegistry(t, server.URL)["zai"],
			collectorTestTarget(t, "zai", false),
		)
		requireNoError(t, err)
		requireEqual(t, 120.0, *snapshot.Windows[0].UsedPercent)
		requireEqual(t, (*time.Time)(nil), snapshot.Windows[0].ResetsAt)
	})
	t.Run("Should reconcile Kimi counts only within the same reset window", func(t *testing.T) {
		t.Parallel()
		body := `{"usages":{"limit_7d":{"used_ratio":0.2,"reset_time":"2026-10-28T00:00:00Z"}},"usage":{"limit":"100","remaining":"70","resetTime":"2026-10-28T00:00:01Z"}}`
		server := fixtureServer(t, func(w http.ResponseWriter, _ *http.Request) { writeCollectorResponse(t, w, body) })
		snapshot, err := collectorRun(
			t,
			collectorTestRegistry(t, server.URL)["moonshot"],
			collectorTestTarget(t, "moonshot", false),
		)
		requireNoError(t, err)
		requireEqual(t, 1, len(snapshot.Windows))
		requireEqual(t, 30.0, *snapshot.Windows[0].UsedPercent)
	})
	t.Run("Should choose the tightest Gemini bucket without inventing a duration", func(t *testing.T) {
		t.Parallel()
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "loadCodeAssist") {
				writeCollectorResponse(
					t,
					w,
					`{"currentTier":{"id":"standard-tier"},"cloudaicompanionProject":{"id":"project"}}`,
				)
				return
			}
			writeCollectorResponse(
				t,
				w,
				`{"buckets":[{"modelId":"gemini-pro","remainingFraction":0.9},{"modelId":"gemini-pro","remainingFraction":0.2}]}`,
			)
		})
		snapshot, err := collectorRun(
			t,
			collectorTestRegistry(t, server.URL)["gemini"],
			collectorTestTarget(t, "gemini", false),
		)
		requireNoError(t, err)
		requireEqual(t, 1, len(snapshot.Windows))
		requireEqual(t, 80.0, *snapshot.Windows[0].UsedPercent)
		requireEqual(t, "standard-tier", snapshot.Account.Plan)
	})
}
