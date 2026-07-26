package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func TestLinearOAuthTokenFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should stop GraphQL when OAuth token endpoint fails", func(t *testing.T) {
		t.Parallel()

		var (
			mu           sync.Mutex
			tokenCalls   int
			graphQLCalls int
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/oauth/token":
				mu.Lock()
				tokenCalls++
				mu.Unlock()
				http.Error(w, "token endpoint unavailable", http.StatusInternalServerError)
			case "/graphql":
				mu.Lock()
				graphQLCalls++
				mu.Unlock()
				http.Error(w, "empty bearer token", http.StatusUnauthorized)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		client := &linearClient{
			cfg: resolvedInstanceConfig{
				authMode:        linearAuthModeOAuth,
				mode:            linearModeAgentSessions,
				apiBaseURL:      server.URL,
				oauthTokenURL:   server.URL + "/oauth/token",
				clientID:        "client-id",
				clientSecret:    "client-secret",
				oauthTokenCache: &linearOAuthTokenCache{},
			},
			httpClient: server.Client(),
			now:        func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) },
		}
		if _, err := client.ValidateAuth(context.Background()); err == nil {
			t.Fatal("ValidateAuth(oauth token 500) error = nil, want non-nil")
		} else {
			var transientErr *bridgesdk.TransientError
			if !errors.As(err, &transientErr) {
				t.Fatalf("ValidateAuth(oauth token 500) error = %#v, want transient error", err)
			}
		}

		mu.Lock()
		defer mu.Unlock()
		if got, want := tokenCalls, 1; got != want {
			t.Fatalf("token calls = %d, want %d", got, want)
		}
		if graphQLCalls != 0 {
			t.Fatalf("graphql calls after token failure = %d, want 0", graphQLCalls)
		}
	})

	t.Run("Should keep initial state degraded when OAuth token endpoint fails", func(t *testing.T) {
		t.Parallel()

		var (
			mu           sync.Mutex
			graphQLCalls int
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/oauth/token":
				http.Error(w, "token endpoint unavailable", http.StatusInternalServerError)
			case "/graphql":
				mu.Lock()
				graphQLCalls++
				mu.Unlock()
				http.Error(w, "empty bearer token", http.StatusUnauthorized)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		provider := &linearProvider{
			apiFactory: func(cfg resolvedInstanceConfig) linearAPI {
				cfg.apiBaseURL = server.URL
				cfg.oauthTokenURL = server.URL + "/oauth/token"
				if cfg.oauthTokenCache == nil {
					cfg.oauthTokenCache = &linearOAuthTokenCache{}
				}
				return &linearClient{
					cfg:        cfg,
					httpClient: server.Client(),
					now:        func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) },
				}
			},
		}
		_, status, degradation, err := provider.determineInitialState(context.Background(), resolvedInstanceConfig{
			instanceID:      "brg-oauth-token-failure",
			organizationID:  "org-agent",
			mode:            linearModeAgentSessions,
			authMode:        linearAuthModeOAuth,
			webhookSecret:   "webhook-secret",
			clientID:        "client-id",
			clientSecret:    "client-secret",
			oauthTokenCache: &linearOAuthTokenCache{},
		})
		if err == nil {
			t.Fatal("determineInitialState(oauth token 500) error = nil, want non-nil")
		}
		if got, want := status, bridgepkg.BridgeStatusDegraded; got != want {
			t.Fatalf("oauth token failure status = %q, want %q", got, want)
		}
		if degradation == nil || degradation.Reason == bridgepkg.BridgeDegradationReasonAuthFailed {
			t.Fatalf("oauth token failure degradation = %#v, want non-auth degraded state", degradation)
		}

		mu.Lock()
		defer mu.Unlock()
		if graphQLCalls != 0 {
			t.Fatalf("graphql calls after determineInitialState token failure = %d, want 0", graphQLCalls)
		}
	})
}
