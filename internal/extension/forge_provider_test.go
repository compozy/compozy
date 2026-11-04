package extensionpkg

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/subprocess"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// Canonical suite: forge.provider selection, dispatch, and response validation.
func TestManagerForgeProvider(t *testing.T) {
	t.Parallel()

	t.Run("Should classify a disabled forge extension without invoking its process", func(t *testing.T) {
		t.Parallel()
		process := newFakeProcess(5199)
		process.callFn = func(context.Context, string, any, any) error {
			t.Error("disabled provider invoked")
			return errors.New("unexpected call")
		}
		manager := NewManager(nil)
		installForgeTestProvider(manager, "github-provider", process, time.Unix(1, 0))
		manager.mu.Lock()
		manager.extensions["github-provider"].active = false
		manager.mu.Unlock()
		_, err := manager.ForgeStatus(
			t.Context(),
			extensioncontract.ForgeStatusRequest{RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "topic"},
		)
		if !errors.Is(err, toolspkg.ErrToolUnavailable) || ForgeProviderErrorCause(err) != "provider_disabled" {
			t.Fatalf("disabled provider = %v, cause=%q", err, ForgeProviderErrorCause(err))
		}
	})

	t.Run("Should validate optional display facts without requiring them", func(t *testing.T) {
		t.Parallel()
		base := extensioncontract.ForgeStatusResponse{Provider: "github", FetchedAt: time.Unix(1, 0)}
		if err := validateForgeStatusResponse(base); err != nil {
			t.Fatal(err)
		}
		atLimit := base
		atLimit.Checks = &extensioncontract.ForgeChecks{
			State: "failing", FailedNames: make([]string, extensioncontract.MaxForgeFailedNames),
		}
		for i := range atLimit.Checks.FailedNames {
			atLimit.Checks.FailedNames[i] = "check"
		}
		if err := validateForgeStatusResponse(atLimit); err != nil {
			t.Fatalf("rejected %d failed names: %v", extensioncontract.MaxForgeFailedNames, err)
		}
		for _, value := range []extensioncontract.ForgeStatusResponse{
			{ReviewDecision: "approved_by_author"},
			{Mergeable: "yes"},
			{Selection: "partial"},
			{RetryAfter: new(time.Unix(2, 0))},
			{Checks: &extensioncontract.ForgeChecks{State: "passing", Passed: -1}},
			{Checks: &extensioncontract.ForgeChecks{
				State: "failing", FailedNames: make([]string, extensioncontract.MaxForgeFailedNames+1),
			}},
			{Reviews: &extensioncontract.ForgeReviews{UnresolvedThreads: -1}},
			{Reviews: &extensioncontract.ForgeReviews{UnresolvedThreads: 2}},
			{Reviews: &extensioncontract.ForgeReviews{Threads: []extensioncontract.ForgeReviewThread{{}}, UnresolvedThreads: 1}},
			{Reviews: &extensioncontract.ForgeReviews{
				Threads: make([]extensioncontract.ForgeReviewThread, extensioncontract.MaxForgeThreads+1), UnresolvedThreads: 200, Truncated: true,
			}},
			{ReviewsUnavailable: true, Reviews: &extensioncontract.ForgeReviews{}},
			{Reviews: &extensioncontract.ForgeReviews{Items: []extensioncontract.ForgeReview{{State: "approved"}}}},
			{Reviews: &extensioncontract.ForgeReviews{Items: []extensioncontract.ForgeReview{{ID: "r", State: "pending"}}}},
			{Reviews: &extensioncontract.ForgeReviews{Items: make([]extensioncontract.ForgeReview, extensioncontract.MaxForgeReviews+1)}},
		} {
			value.Provider, value.FetchedAt = base.Provider, base.FetchedAt
			if err := validateForgeStatusResponse(value); err == nil {
				t.Fatalf("accepted invalid facts: %#v", value)
			}
		}
		reviewed := base
		reviewed.Reviews = &extensioncontract.ForgeReviews{
			Items: []extensioncontract.ForgeReview{
				{ID: "r1", State: extensioncontract.ForgeReviewChangesRequested},
			},
			Threads: []extensioncontract.ForgeReviewThread{
				{ID: "t1", Commit: "head-a"},
			},
			UnresolvedThreads: 2,
			Truncated:         true,
		}
		if err := validateForgeStatusResponse(reviewed); err != nil {
			t.Fatalf("rejected review activity: %v", err)
		}
	})

	t.Run("Should select the first enabled provider that serves the remote", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		calls := make([]string, 0, 8)
		record := func(value string) {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, value)
		}

		unsupported := newFakeProcess(5101)
		unsupported.callFn = func(_ context.Context, method string, params, result any) error {
			record("unsupported:" + method)
			if method != string(extensionprotocol.ExtensionServiceMethodForgeCapabilities) {
				return errors.New("unexpected method on unsupported provider")
			}
			request := params.(extensioncontract.ForgeCapabilitiesRequest)
			if !slices.Equal(request.RemoteURLs, []string{"https://github.com/acme/repo.git"}) {
				return errors.New("unexpected normalized remotes")
			}
			*result.(*extensioncontract.ForgeCapabilitiesResponse) = extensioncontract.ForgeCapabilitiesResponse{
				Cause: extensioncontract.ForgeCauseUnsupportedRemote,
			}
			return nil
		}
		github := newFakeProcess(5102)
		fetchedAt := time.Unix(1, 0).UTC()
		github.callFn = func(_ context.Context, method string, params, result any) error {
			record("github:" + method)
			switch method {
			case string(extensionprotocol.ExtensionServiceMethodForgeCapabilities):
				*result.(*extensioncontract.ForgeCapabilitiesResponse) = availableForgeCapabilities()
			case string(extensionprotocol.ExtensionServiceMethodForgeStatus):
				request := params.(extensioncontract.ForgeStatusRequest)
				if request.Branch != "feature/exit" {
					return errors.New("unexpected status branch")
				}
				state, merged, number := "open", false, 7
				*result.(*extensioncontract.ForgeStatusResponse) = extensioncontract.ForgeStatusResponse{
					PRNumber: &number, PRState: &state, PRURL: "https://github.com/acme/repo/pull/7",
					Merged: &merged, FetchedAt: fetchedAt,
				}
			case string(extensionprotocol.ExtensionServiceMethodForgePRCreate):
				request := params.(extensioncontract.ForgePRCreateRequest)
				if request.Head != "feature/exit" || request.Base != "main" || request.Title != "Exit worktree" {
					return errors.New("unexpected create request")
				}
				*result.(*extensioncontract.ForgePRCreateResponse) = extensioncontract.ForgePRCreateResponse{
					Status: "created", Number: 8, URL: "https://github.com/acme/repo/pull/8",
				}
			default:
				return errors.New("unexpected forge method")
			}
			return nil
		}

		manager := NewManager(nil)
		installForgeTestProvider(manager, "first-enabled", unsupported, time.Unix(1, 0))
		installForgeTestProvider(manager, "github-provider", github, time.Unix(2, 0))
		remotes := []string{" https://github.com/acme/repo.git ", "https://github.com/acme/repo.git"}

		capabilities, err := manager.ForgeCapabilities(t.Context(), remotes)
		if err != nil || capabilities.Winner != "github-provider" || !capabilities.Available {
			t.Fatalf("ForgeCapabilities() = %#v, %v", capabilities, err)
		}
		status, err := manager.ForgeStatus(t.Context(), extensioncontract.ForgeStatusRequest{
			RemoteURLs: remotes, Branch: "feature/exit",
		})
		if err != nil || status.Provider != "github-provider" || status.PRNumber == nil || *status.PRNumber != 7 {
			t.Fatalf("ForgeStatus() = %#v, %v", status, err)
		}
		created, err := manager.ForgeCreatePR(t.Context(), extensioncontract.ForgePRCreateRequest{
			RemoteURLs: remotes, Head: "feature/exit", Base: "main", Title: "Exit worktree",
		})
		if err != nil || created.Status != "created" || created.Number != 8 {
			t.Fatalf("ForgeCreatePR() = %#v, %v", created, err)
		}
		mu.Lock()
		defer mu.Unlock()
		// Capabilities are asked once per remote set; later operations reuse the cached winner.
		want := []string{
			"unsupported:forge/capabilities", "github:forge/capabilities",
			"github:forge/status", "github:forge/pr_create",
		}
		if !slices.Equal(calls, want) {
			t.Fatalf("forge calls = %#v, want %#v", calls, want)
		}
	})

	t.Run("Should reject an incomplete served capability response", func(t *testing.T) {
		t.Parallel()
		process := newFakeProcess(5103)
		process.callFn = func(_ context.Context, _ string, _ any, result any) error {
			*result.(*extensioncontract.ForgeCapabilitiesResponse) = extensioncontract.ForgeCapabilitiesResponse{
				Served: true, Available: true, Provider: "broken", CredentialSource: "binding",
			}
			return nil
		}
		manager := NewManager(nil)
		installForgeTestProvider(manager, "broken", process, time.Unix(1, 0))
		_, err := manager.ForgeCapabilities(t.Context(), []string{"https://github.com/acme/repo"})
		if err == nil || !strings.Contains(err.Error(), "served forge capabilities are incomplete") {
			t.Fatalf("ForgeCapabilities() error = %v, want incomplete served capabilities", err)
		}
	})

	t.Run("Should reject an incomplete successful pull request", func(t *testing.T) {
		t.Parallel()
		process := newFakeProcess(5106)
		process.callFn = func(_ context.Context, method string, _ any, result any) error {
			switch method {
			case string(extensionprotocol.ExtensionServiceMethodForgeCapabilities):
				*result.(*extensioncontract.ForgeCapabilitiesResponse) = availableForgeCapabilities()
			case string(extensionprotocol.ExtensionServiceMethodForgePRCreate):
				*result.(*extensioncontract.ForgePRCreateResponse) = extensioncontract.ForgePRCreateResponse{
					Status: "created",
				}
			}
			return nil
		}
		manager := NewManager(nil)
		installForgeTestProvider(manager, "incomplete-pr", process, time.Unix(1, 0))
		_, err := manager.ForgeCreatePR(t.Context(), extensioncontract.ForgePRCreateRequest{
			RemoteURLs: []string{"https://github.com/acme/repo"}, Head: "feature", Base: "main", Title: "Feature",
		})
		if err == nil || !strings.Contains(err.Error(), "returned an invalid pull request") {
			t.Fatalf("ForgeCreatePR(incomplete) error = %v, want invalid pull request", err)
		}
	})

	t.Run("Should reject incomplete forge status evidence", func(t *testing.T) {
		t.Parallel()
		state := "open"
		number := 7
		merged := false
		for _, test := range []struct {
			response extensioncontract.ForgeStatusResponse
			want     string
		}{
			{
				response: extensioncontract.ForgeStatusResponse{
					Provider: "github", FetchedAt: time.Unix(1, 0), PRNumber: &number, PRState: &state,
				},
				want: "must be returned together",
			},
			{
				response: extensioncontract.ForgeStatusResponse{
					Provider: "github", FetchedAt: time.Unix(1, 0), PRNumber: &number, PRState: &state,
					PRURL: "javascript://host/pr/7", Merged: &merged,
				},
				want: "credential-free HTTP(S) URL",
			},
		} {
			err := validateForgeStatusResponse(test.response)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateForgeStatusResponse(%#v) error = %v, want %q", test.response, err, test.want)
			}
		}
	})

	t.Run("Should map an unavailable winner to the stable tool error", func(t *testing.T) {
		t.Parallel()
		process := newFakeProcess(5104)
		process.callFn = func(_ context.Context, _ string, _ any, result any) error {
			response := availableForgeCapabilities()
			response.Available = false
			response.CredentialSource = ""
			response.Cause = extensioncontract.ForgeCauseCredentialAbsent
			*result.(*extensioncontract.ForgeCapabilitiesResponse) = response
			return nil
		}
		manager := NewManager(nil)
		installForgeTestProvider(manager, "github-provider", process, time.Unix(1, 0))
		_, err := manager.ForgeStatus(t.Context(), extensioncontract.ForgeStatusRequest{
			RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "feature/exit",
		})
		if !errors.Is(err, toolspkg.ErrToolUnavailable) {
			t.Fatalf("ForgeStatus() error = %v, want ErrToolUnavailable", err)
		}
	})

	for _, cause := range []string{
		extensioncontract.ForgeCauseRateLimited,
		extensioncontract.ForgeCauseCredentialExpired,
	} {
		t.Run("Should preserve transient cause "+cause+" as a forge failure", func(t *testing.T) {
			t.Parallel()
			process := newFakeProcess(5105)
			process.callFn = func(_ context.Context, _ string, _ any, result any) error {
				response := availableForgeCapabilities()
				response.Available = false
				response.CredentialSource = ""
				response.Cause = cause
				*result.(*extensioncontract.ForgeCapabilitiesResponse) = response
				return nil
			}
			manager := NewManager(nil)
			installForgeTestProvider(manager, "github-provider", process, time.Unix(1, 0))
			_, err := manager.ForgeStatus(t.Context(), extensioncontract.ForgeStatusRequest{
				RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "feature/exit",
			})
			if err == nil || errors.Is(err, toolspkg.ErrToolUnavailable) || ForgeProviderErrorCause(err) != cause {
				t.Fatalf("ForgeStatus() error = %v, want transient cause %q", err, cause)
			}
		})
	}
}

// Invariants: capabilities are cached per remote set and provider start, and a provider that
// reported a rate limit is not called by any forge read until its reset.
// Owner: Manager forge quota; canonical forge provider suite.
func TestManagerForgeQuota(t *testing.T) {
	t.Parallel()
	remotes := []string{"https://github.com/acme/repo"}
	request := extensioncontract.ForgeStatusRequest{RemoteURLs: remotes, Branch: "topic"}

	t.Run("Should cache capabilities until the window ends or the provider restarts", func(t *testing.T) {
		t.Parallel()
		var capabilityCalls atomic.Int32
		process := newFakeProcess(5201)
		process.callFn = func(_ context.Context, method string, _ any, result any) error {
			if method == string(extensionprotocol.ExtensionServiceMethodForgeCapabilities) {
				capabilityCalls.Add(1)
				*result.(*extensioncontract.ForgeCapabilitiesResponse) = availableForgeCapabilities()
				return nil
			}
			*result.(*extensioncontract.ForgeStatusResponse) = extensioncontract.ForgeStatusResponse{
				Provider: "github", FetchedAt: time.Unix(1, 0),
			}
			return nil
		}
		manager := NewManager(nil)
		now := time.Unix(1_800_000_000, 0).UTC()
		manager.now = func() time.Time { return now }
		installForgeTestProvider(manager, "github-provider", process, time.Unix(1, 0))
		for range 5 {
			if _, err := manager.ForgeStatus(t.Context(), request); err != nil {
				t.Fatal(err)
			}
		}
		if capabilityCalls.Load() != 1 {
			t.Fatalf("five status reads asked capabilities %d times, want 1", capabilityCalls.Load())
		}
		now = now.Add(forgeCapabilitiesTTL)
		if _, err := manager.ForgeStatus(t.Context(), request); err != nil || capabilityCalls.Load() != 2 {
			t.Fatalf("expired window: calls=%d err=%v", capabilityCalls.Load(), err)
		}
		manager.mu.Lock()
		manager.extensions["github-provider"].lastStartedAt = time.Unix(2, 0)
		manager.mu.Unlock()
		if _, err := manager.ForgeStatus(t.Context(), request); err != nil || capabilityCalls.Load() != 3 {
			t.Fatalf("restarted provider: calls=%d err=%v", capabilityCalls.Load(), err)
		}
	})

	// Invariant: simultaneous readers share the repository capability lookup.
	// Owner: Manager capability admission; canonical forge quota suite.
	t.Run("Should coalesce concurrent capability misses", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		process := newFakeProcess(5204)
		process.callFn = func(_ context.Context, _ string, _ any, result any) error {
			calls.Add(1)
			*result.(*extensioncontract.ForgeCapabilitiesResponse) = availableForgeCapabilities()
			return nil
		}
		manager := NewManager(nil)
		installForgeTestProvider(manager, "github-provider", process, time.Unix(1, 0))
		results := make(chan error, 12)
		start := make(chan struct{})
		for range 12 {
			go func() { <-start; _, err := manager.ForgeCapabilities(t.Context(), remotes); results <- err }()
		}
		close(start)
		for range 12 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		if calls.Load() != 1 {
			t.Fatalf("capability calls = %d, want 1", calls.Load())
		}
	})

	for _, source := range []string{"status", "capabilities"} {
		t.Run("Should hold every forge read until a "+source+" rate limit resets", func(t *testing.T) {
			t.Parallel()
			now := time.Unix(1_800_000_000, 0).UTC()
			reset := now.Add(20 * time.Minute)
			var calls atomic.Int32
			var limited atomic.Bool
			limited.Store(true)
			process := newFakeProcess(5202)
			process.callFn = func(_ context.Context, method string, _ any, result any) error {
				calls.Add(1)
				switch method {
				case string(extensionprotocol.ExtensionServiceMethodForgeCapabilities):
					response := availableForgeCapabilities()
					if source == "capabilities" && limited.Load() {
						response.Available, response.CredentialSource = false, ""
						response.Cause, response.RetryAfter = extensioncontract.ForgeCauseRateLimited, &reset
					}
					*result.(*extensioncontract.ForgeCapabilitiesResponse) = response
				case string(extensionprotocol.ExtensionServiceMethodForgeStatus):
					response := extensioncontract.ForgeStatusResponse{Provider: "github", FetchedAt: now}
					if limited.Load() {
						response = extensioncontract.ForgeStatusResponse{
							Cause: extensioncontract.ForgeCauseRateLimited, RetryAfter: &reset,
						}
					}
					*result.(*extensioncontract.ForgeStatusResponse) = response
				default:
					return errors.New("unexpected forge method")
				}
				return nil
			}
			manager := NewManager(nil)
			manager.now = func() time.Time { return now }
			installForgeTestProvider(manager, "github-provider", process, time.Unix(1, 0))
			_, err := manager.ForgeStatus(t.Context(), request)
			failure, ok := errors.AsType[*ForgeProviderError](err)
			if !ok || failure.cause != extensioncontract.ForgeCauseRateLimited || failure.RetryAfter == nil ||
				!failure.RetryAfter.Equal(reset) {
				t.Fatalf("first read error = %v", err)
			}
			made := calls.Load()
			limited.Store(false)
			now = reset.Add(-time.Second)
			for range 3 {
				_, err = manager.ForgeStatus(t.Context(), request)
				if ForgeProviderErrorCause(err) != extensioncontract.ForgeCauseRateLimited {
					t.Fatalf("held read error = %v", err)
				}
			}
			if _, err := manager.ForgeSearchPRs(t.Context(), extensioncontract.ForgePRSearchRequest{
				RemoteURLs: remotes,
			}); ForgeProviderErrorCause(err) != extensioncontract.ForgeCauseRateLimited {
				t.Fatalf("held discovery error = %v", err)
			}
			if _, err := manager.ForgeCapabilities(t.Context(), remotes); ForgeProviderErrorCause(err) !=
				extensioncontract.ForgeCauseRateLimited {
				t.Fatalf("held capabilities error = %v", err)
			}
			if calls.Load() != made {
				t.Fatalf("rate-limited provider called %d more times before reset", calls.Load()-made)
			}
			now = reset
			if _, err := manager.ForgeStatus(t.Context(), request); err != nil {
				t.Fatalf("read after reset: %v", err)
			}
		})
	}

	t.Run("Should hold every forge read after a discovery rate limit", func(t *testing.T) {
		t.Parallel()
		now := time.Unix(1_800_000_000, 0).UTC()
		reset := now.Add(15 * time.Minute)
		var statusCalls atomic.Int32
		process := newFakeProcess(5203)
		process.callFn = func(_ context.Context, method string, _ any, result any) error {
			switch method {
			case string(extensionprotocol.ExtensionServiceMethodForgeCapabilities):
				response := availableForgeCapabilities()
				response.SupportsPRDiscovery = true
				*result.(*extensioncontract.ForgeCapabilitiesResponse) = response
				return nil
			case string(extensionprotocol.ExtensionServiceMethodForgePRSearch):
				return &subprocess.RPCError{
					Code: -32000, Message: "GitHub rate limit reached; retry later",
					Data: []byte(`{"code":"rate_limited","retry_after":"` + reset.Format(time.RFC3339) + `"}`),
				}
			default:
				statusCalls.Add(1)
				return errors.New("status must stay held")
			}
		}
		manager := NewManager(nil)
		manager.now = func() time.Time { return now }
		installForgeTestProvider(manager, "github-provider", process, time.Unix(1, 0))
		_, err := manager.ForgeSearchPRs(t.Context(), extensioncontract.ForgePRSearchRequest{RemoteURLs: remotes})
		failure, ok := errors.AsType[*ForgeProviderError](err)
		if !ok || failure.cause != extensioncontract.ForgeCauseRateLimited || !failure.RetryAfter.Equal(reset) {
			t.Fatalf("search error = %v", err)
		}
		if _, err := manager.ForgeStatus(t.Context(), request); ForgeProviderErrorCause(err) !=
			extensioncontract.ForgeCauseRateLimited || statusCalls.Load() != 0 {
			t.Fatalf("status after discovery limit = %v, calls %d", err, statusCalls.Load())
		}
	})

	t.Run("Should hold a provider for a minute when it names no reset", func(t *testing.T) {
		t.Parallel()
		var quota forgeQuota
		now := time.Unix(1_800_000_000, 0).UTC()
		if until := quota.limit("p", nil, now); !until.Equal(now.Add(forgeRateLimitFallback)) {
			t.Fatalf("fallback until = %s", until)
		}
		later := now.Add(time.Hour)
		quota.limit("p", &later, now)
		if until := quota.limit("p", nil, now); !until.Equal(later) {
			t.Fatalf("short hint shortened the known reset: %s", until)
		}
	})
}

func installForgeTestProvider(manager *Manager, name string, process *fakeProcess, startedAt time.Time) {
	provides := []string{extensionprotocol.CapabilityProvideForgeProvider}
	manager.extensions[name] = &managedExtension{
		key: GlobalInstanceKey(
			name,
		),
		info:          ExtensionInfo{Name: name, Capabilities: CapabilitiesConfig{Provides: provides}},
		manifest:      &Manifest{Name: name, Capabilities: CapabilitiesConfig{Provides: provides}},
		process:       process,
		active:        true,
		lastStartedAt: startedAt,
		initialize: &subprocess.InitializeResponse{
			ImplementedMethods: extensionprotocol.CapabilityServiceMethods(provides),
		},
	}
}

func availableForgeCapabilities() extensioncontract.ForgeCapabilitiesResponse {
	return extensioncontract.ForgeCapabilitiesResponse{
		Served: true, Available: true, Provider: "github", ServedRemote: "https://github.com/acme/repo",
		RequestNoun: "PR", OpenActionLabel: "Open PR", ViewActionLabel: "View PR",
		CredentialSource: "binding", DefaultBranch: "main",
	}
}
