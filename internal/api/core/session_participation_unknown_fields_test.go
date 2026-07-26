package core_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/testutil"
)

func TestSessionCreateRejectsLegacyParticipationFields(t *testing.T) {
	t.Parallel()

	t.Run("Should reject channel as unknown field on session create", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/sessions",
			[]byte(`{"agent_name":"coder","workspace":"ws-alpha","channel":"builders"}`),
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
		}
		if body := resp.Body.String(); !strings.Contains(body, "unknown_field") || !strings.Contains(body, "channel") {
			t.Fatalf("body = %s, want unknown_field with channel named", body)
		}
	})

	t.Run("Should reject network_channel as unknown field on session create", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/sessions",
			[]byte(`{"agent_name":"coder","workspace":"ws-alpha","network_channel":"builders"}`),
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
		}
		body := resp.Body.String()
		if !strings.Contains(body, "unknown_field") || !strings.Contains(body, "network_channel") {
			t.Fatalf("body = %s, want unknown_field with network_channel named", body)
		}
	})

	t.Run("Should reject coordination_channel_id as unknown field on session create", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/sessions",
			[]byte(`{"agent_name":"coder","workspace":"ws-alpha","coordination_channel_id":"coord-1"}`),
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
		}
		if body := resp.Body.String(); !strings.Contains(body, "unknown_field") ||
			!strings.Contains(body, "coordination_channel_id") {
			t.Fatalf("body = %s, want unknown_field with coordination_channel_id named", body)
		}
	})
}
