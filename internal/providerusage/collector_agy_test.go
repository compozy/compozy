package providerusage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"
)

const agyQuotaFixture = `{"groups":[{"displayName":"Claude and GPT models","buckets":[` +
	`{"bucketId":"3p-weekly","displayName":"Weekly Limit Remaining","window":"weekly",` +
	`"remainingFraction":0.75,"resetTime":"2026-10-27T00:00:00Z"},` +
	`{"bucketId":"3p-5h","displayName":"Five Hour Limit Remaining","window":"5h",` +
	`"remainingFraction":0.4,"resetTime":"2026-10-21T03:00:00Z"},` +
	`{"bucketId":"3p-off","window":"weekly","remainingFraction":1,"disabled":true}]}],` +
	`"buckets":[{"bucketId":"burst","displayName":"Burst","window":"10m","remainingFraction":0}]}`

func agyKeychainItem(t *testing.T, email string, expiry time.Time) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"token": map[string]any{
			"access_token":  "fixture-access-secret",
			"refresh_token": "never-redeem",
			"token_type":    "Bearer",
			"expiry":        expiry.Format(time.RFC3339Nano),
		},
		"auth_method":    "consumer",
		"id_token_email": email,
	})
	requireNoError(t, err)
	return agyKeyringPrefix + base64.StdEncoding.EncodeToString(data)
}

func agyTestTarget(consent bool) Target {
	return Target{
		ProfileID:    "profile",
		ProviderID:   agyProviderID,
		ProviderName: "Google Antigravity",
		Route:        ResolvedRoute{KeychainAccess: consent, NativeCommand: "/fixture/bin/agy"},
	}
}

func agyTestCollector(t *testing.T, url string, keychain KeychainReader, platform string) *AgyCollector {
	t.Helper()
	return NewAgyCollector(
		[]CollectorOption{WithCollectorBaseURL(url), WithCollectorClock(fixtureClock(t).Now)},
		WithAgyKeychain(keychain),
		WithAgyPlatform(platform),
	)
}

func TestAgyCollector(t *testing.T) {
	t.Run("Should translate the quota summary from the CLI Keychain login", func(t *testing.T) {
		t.Parallel()
		keychain := &fakeClaudeKeychain{
			data: agyKeychainItem(t, "fixture@example.test", fixtureClock(t).Now().Add(time.Hour)),
		}
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			requireEqual(t, http.MethodPost, r.Method)
			requireEqual(t, "Bearer fixture-access-secret", r.Header.Get(headerAuthorization))
			requireEqual(t, agyUserAgent, r.Header.Get("User-Agent"))
			switch r.URL.Path {
			case "/v1internal:loadCodeAssist":
				writeCollectorResponse(t, w, `{"currentTier":{"id":"free-tier"},"paidTier":{"name":"Google AI Pro"}}`)
			case "/v1internal:retrieveUserQuotaSummary":
				writeCollectorResponse(t, w, agyQuotaFixture)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		})
		collector := agyTestCollector(t, server.URL, keychain, "darwin")
		snapshot, err := collectorRun(t, collector, agyTestTarget(true))
		requireNoError(t, err)
		requireEqual(t, StatusOK, snapshot.Status)
		requireEqual(t, ConfidencePercentOnly, snapshot.Confidence)
		requireEqual(t, "fixture@example.test", snapshot.Account.Label)
		requireEqual(t, "Google AI Pro", snapshot.Account.Plan)
		requireEqual(t, int32(0), keychain.interactive.Load())
		requireEqual(t, 3, len(snapshot.Windows))
		byID := map[string]Window{}
		for _, window := range snapshot.Windows {
			byID[window.ID] = window
		}
		session := byID["bucket:3p-5h"]
		requireEqual(t, WindowKindSession, session.Kind)
		requireEqual(t, "Claude and GPT models · Session", session.Label)
		requireEqual(t, 60.0, *session.UsedPercent)
		requireEqual(t, int64(18000), *session.DurationSeconds)
		requireEqual(t, time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC), *session.ResetsAt)
		weekly := byID["bucket:3p-weekly"]
		requireEqual(t, WindowKindWeekly, weekly.Kind)
		requireEqual(t, 25.0, *weekly.UsedPercent)
		burst := byID["bucket:burst"]
		requireEqual(t, WindowKindOther, burst.Kind)
		requireEqual(t, "Burst", burst.Label)
		requireEqual(t, 100.0, *burst.UsedPercent)
		if _, disabled := byID["bucket:3p-off"]; disabled {
			t.Fatal("disabled bucket was published")
		}
	})

	t.Run("Should keep the account when the quota summary is refused", func(t *testing.T) {
		t.Parallel()
		keychain := &fakeClaudeKeychain{
			data: agyKeychainItem(t, "fixture@example.test", fixtureClock(t).Now().Add(time.Hour)),
		}
		server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1internal:loadCodeAssist" {
				writeCollectorResponse(t, w, `{"paidTier":{"name":"Google AI Pro"}}`)
				return
			}
			w.WriteHeader(http.StatusForbidden)
		})
		snapshot, err := collectorRun(t, agyTestCollector(t, server.URL, keychain, "darwin"), agyTestTarget(true))
		requireEqual(t, ErrorCodeAuthScope, AsUsageError(err, agyLabel).Code)
		requireEqual(t, StatusSignedOut, snapshot.Status)
		requireEqual(t, "Google AI Pro", snapshot.Account.Plan)
		if snapshot.MeasuredAt != nil {
			t.Fatal("refused quota reported a measurement")
		}
	})

	t.Run("Should reject unavailable logins before requesting quota", func(t *testing.T) {
		t.Parallel()
		now := fixtureClock(t).Now()
		cases := []struct {
			name     string
			target   Target
			platform string
			keychain *fakeClaudeKeychain
			want     ErrorCode
			reads    int32
		}{
			{"Should require consent", agyTestTarget(false), "darwin",
				&fakeClaudeKeychain{}, ErrorCodeKeychainConsentRequired, 0},
			{
				"Should refuse non-macOS hosts",
				agyTestTarget(true),
				"linux",
				&fakeClaudeKeychain{},
				ErrorCodeNoCollector,
				0,
			},
			{
				"Should refuse an alias",
				Target{ProviderID: "agy-work", Route: ResolvedRoute{KeychainAccess: true}},
				"darwin",
				&fakeClaudeKeychain{},
				ErrorCodeNoCollector,
				0,
			},
			{"Should refuse a missing CLI even with consent",
				Target{ProviderID: agyProviderID, Route: ResolvedRoute{KeychainAccess: true}}, "darwin",
				&fakeClaudeKeychain{}, ErrorCodeProviderUnavailable, 0},
			{"Should require a login", agyTestTarget(true), "darwin", &fakeClaudeKeychain{err: os.ErrNotExist},
				ErrorCodeAuthMissing, 1},
			{"Should report a denied Keychain read", agyTestTarget(true), "darwin",
				&fakeClaudeKeychain{err: os.ErrPermission}, ErrorCodeKeychainDenied, 1},
			{"Should report a background read timeout", agyTestTarget(true), "darwin",
				&fakeClaudeKeychain{err: NewUsageError(ErrorCodeKeychainReadTimeout, "Claude")},
				ErrorCodeKeychainReadTimeout, 1},
			{"Should report a Keychain reader failure", agyTestTarget(true), "darwin",
				&fakeClaudeKeychain{err: errors.New("security exited")}, ErrorCodeProviderUnavailable, 1},
			{"Should report an expired token", agyTestTarget(true), "darwin",
				&fakeClaudeKeychain{data: agyKeychainItem(t, "fixture@example.test", now.Add(30*time.Second))},
				ErrorCodeAuthExpired, 1},
			{"Should report a corrupt Keychain item", agyTestTarget(true), "darwin",
				&fakeClaudeKeychain{data: agyKeyringPrefix + "%%%"}, ErrorCodeParseError, 1},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
					t.Errorf("unexpected request %s", r.URL.Path)
				})
				collector := agyTestCollector(t, server.URL, tc.keychain, tc.platform)
				_, err := collector.Collect(t.Context(), tc.target, IntentBackground, CredentialIdentity{})
				usage := AsUsageError(err, agyLabel)
				requireEqual(t, tc.want, usage.Code)
				requireEqual(t, tc.reads, tc.keychain.calls.Load())
			})
		}
	})

	t.Run("Should read the Keychain interactively only for explicit refreshes", func(t *testing.T) {
		t.Parallel()
		var item string
		keychain := &fakeClaudeKeychain{
			read: func(_ context.Context, service, account string, _ bool) ([]byte, error) {
				requireEqual(t, agyKeychainService, service)
				requireEqual(t, agyKeychainAccount, account)
				return []byte(item), nil
			},
		}
		item = agyKeychainItem(t, "fixture@example.test", fixtureClock(t).Now().Add(time.Hour))
		collector := agyTestCollector(t, "http://127.0.0.1:1", keychain, "darwin")
		_, err := collector.Identity(t.Context(), agyTestTarget(true), IntentExplicit)
		requireNoError(t, err)
		requireEqual(t, int32(1), keychain.interactive.Load())
	})

	t.Run("Should keep ownership across CLI token renewals of the same account", func(t *testing.T) {
		t.Parallel()
		expiry := fixtureClock(t).Now().Add(time.Hour)
		first, err := parseAgyCredential(
			[]byte(agyKeychainItem(t, "fixture@example.test", expiry)),
			fixtureClock(t).Now(),
		)
		requireNoError(t, err)
		renewed := first
		renewed.token = "renewed-access-secret"
		requireEqual(t, true, first.identity().Same(renewed.identity()))
		other := renewed
		other.email = "other@example.test"
		requireEqual(t, false, first.identity().Same(other.identity()))
	})

	t.Run("Should reject malformed quota buckets", func(t *testing.T) {
		t.Parallel()
		for _, body := range []string{
			`{}`,
			`{"buckets":[{"bucketId":"a","window":"5h"}]}`,
			`{"buckets":[{"bucketId":"a","disabled":true}]}`,
			`{"buckets":[{"bucketId":"a","window":"5h","remainingFraction":1.5}]}`,
			`{"buckets":[{"window":"5h","remainingFraction":0.5}]}`,
			`{"buckets":[{"bucketId":"a","remainingFraction":0.5},{"bucketId":"A","remainingFraction":0.2}]}`,
		} {
			var summary agyQuotaSummary
			requireNoError(t, json.Unmarshal([]byte(body), &summary))
			_, err := summary.windows()
			requireEqual(t, ErrorCodeParseError, AsUsageError(err, agyLabel).Code)
		}
	})
}
