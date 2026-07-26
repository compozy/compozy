//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"testing"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	"github.com/compozy/agh/internal/transcript"
)

func mockFixturePath(t testing.TB, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "testutil", "acpmock", "testdata", name)
}

func createFixtureBackedSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	agentName string,
	name string,
) aghcontract.SessionPayload {
	t.Helper()

	session, err := harness.CreateSession(ctx, aghcontract.CreateSessionRequest{
		AgentName:     agentName,
		Name:          name,
		WorkspacePath: harness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession(%q) error = %v", agentName, err)
	}
	active, err := harness.WaitForSessionActive(ctx, session.ID)
	if err != nil {
		t.Fatalf("WaitForSessionActive(%q) error = %v", session.ID, err)
	}
	return active
}

func createSessionHTTPFailure(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	request aghcontract.CreateSessionRequest,
) (int, aghcontract.ErrorPayload) {
	t.Helper()

	response := postSessionHTTP(t, ctx, harness, request)
	return response.StatusCode, decodeSessionHTTPResponse[aghcontract.ErrorPayload](t, response)
}

func createSessionHTTPAccepted(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	request aghcontract.CreateSessionRequest,
) (int, aghcontract.SessionPayload) {
	t.Helper()

	response := postSessionHTTP(t, ctx, harness, request)
	payload := decodeSessionHTTPResponse[aghcontract.SessionResponse](t, response)
	return response.StatusCode, payload.Session
}

func decodeSessionHTTPResponse[T any](t testing.TB, response *http.Response) T {
	t.Helper()
	var payload T
	decodeErr := json.NewDecoder(response.Body).Decode(&payload)
	closeErr := response.Body.Close()
	if err := errors.Join(decodeErr, closeErr); err != nil {
		t.Fatalf("decode/close HTTP create session response error = %v", err)
	}
	return payload
}

func postSessionHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	request aghcontract.CreateSessionRequest,
) *http.Response {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(create session request) error = %v", err)
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		harness.HTTPURL("/api/sessions"),
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(create session) error = %v", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := harness.HTTPClient.Do(httpRequest)
	if err != nil {
		t.Fatalf("HTTP create session error = %v", err)
	}
	return response
}

func providerModelListHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	providerID string,
	view string,
) (int, aghcontract.ProviderModelListResponse) {
	t.Helper()

	path := "/api/model-catalog/providers/" + url.PathEscape(providerID) + "/models"
	if view != "" {
		path += "?view=" + url.QueryEscape(view)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, harness.HTTPURL(path), nil)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(provider models) error = %v", err)
	}
	response, err := harness.HTTPClient.Do(request)
	if err != nil {
		t.Fatalf("HTTP provider models error = %v", err)
	}
	var payload aghcontract.ProviderModelListResponse
	decodeErr := json.NewDecoder(response.Body).Decode(&payload)
	closeErr := response.Body.Close()
	if decodeErr != nil {
		t.Fatalf("decode HTTP provider models error = %v", decodeErr)
	}
	if closeErr != nil {
		t.Fatalf("close HTTP provider models body error = %v", closeErr)
	}
	return response.StatusCode, payload
}

func joinTranscriptContent(messages []transcript.UIMessage) string {
	return transcript.JoinUIMessageText(messages)
}

func sessionTranscriptMessages(response aghcontract.SessionTranscriptResponse) []transcript.UIMessage {
	return transcript.MessagesFromEntries(response.Entries)
}
