package udsapi

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestListExtensionsHandler(t *testing.T) {
	t.Parallel()

	t.Run("Should return installed extensions", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		handlers := newTestHandlersWithExtensions(t, stubSessionManager{}, stubObserver{}, stubExtensionService{
			ListFn: func(context.Context) ([]contract.ExtensionPayload, error) {
				return []contract.ExtensionPayload{{
					Name:    "ext-a",
					Enabled: true,
					State:   "active",
				}}, nil
			},
		}, homePaths)
		engine := newTestRouter(t, handlers)

		recorder := performRequest(t, engine, http.MethodGet, "/api/extensions", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response struct {
			Extensions []contract.ExtensionPayload `json:"extensions"`
		}
		decodeJSONResponse(t, recorder, &response)
		if got, want := len(response.Extensions), 1; got != want {
			t.Fatalf("len(extensions) = %d, want %d", got, want)
		}
		if response.Extensions[0].Name != "ext-a" || !response.Extensions[0].Enabled {
			t.Fatalf("extensions[0] = %#v", response.Extensions[0])
		}
	})
}

func TestInstallExtensionHandler(t *testing.T) {
	t.Parallel()

	t.Run("Should install extensions from validated requests", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		handlers := newTestHandlersWithExtensions(t, stubSessionManager{}, stubObserver{}, stubExtensionService{
			InstallFn: func(
				_ context.Context,
				req contract.InstallExtensionRequest,
				_ taskpkg.ActorContext,
			) (contract.ExtensionPayload, error) {
				if req.Source != contract.InstallExtensionSourceLocalPath || req.Ref != "/tmp/ext-a" {
					t.Fatalf("Install() req = %#v", req)
				}
				return contract.ExtensionPayload{Name: "ext-a", State: "installed"}, nil
			},
		}, homePaths)
		engine := newTestRouter(t, handlers)

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/extensions",
			[]byte(`{"source":"local_path","ref":" /tmp/ext-a "}`),
		)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
		}

		var response struct {
			Extension contract.ExtensionPayload `json:"extension"`
		}
		decodeJSONResponse(t, recorder, &response)
		if response.Extension.Name != "ext-a" {
			t.Fatalf("extension = %#v", response.Extension)
		}
	})

	validationCases := []struct {
		name        string
		payload     string
		wantMessage string
	}{
		{
			name:        "Should reject a missing source",
			payload:     `{"ref":"/tmp/ext-a"}`,
			wantMessage: "source",
		},
		{
			name:        "Should reject a missing ref",
			payload:     `{"source":"local_path"}`,
			wantMessage: "ref",
		},
	}

	for _, tc := range validationCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			homePaths := newTestHomePaths(t)
			handlers := newTestHandlersWithExtensions(
				t,
				stubSessionManager{},
				stubObserver{},
				stubExtensionService{},
				homePaths,
			)
			engine := newTestRouter(t, handlers)

			recorder := performRequest(t, engine, http.MethodPost, "/api/extensions", []byte(tc.payload))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if !strings.Contains(strings.ToLower(recorder.Body.String()), tc.wantMessage) {
				t.Fatalf("body = %q, want substring %q", recorder.Body.String(), tc.wantMessage)
			}
		})
	}

	t.Run("Should return deterministic policy diagnostic for blocked side-loads", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		handlers := newTestHandlersWithExtensions(t, stubSessionManager{}, stubObserver{}, stubExtensionService{
			InstallFn: func(
				context.Context,
				contract.InstallExtensionRequest,
				taskpkg.ActorContext,
			) (contract.ExtensionPayload, error) {
				return contract.ExtensionPayload{}, extensionpkg.ValidateUnverifiedSideLoad(
					"acme/blocked", "local", false, true,
				)
			},
		}, homePaths)
		engine := newTestRouter(t, handlers)

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/extensions",
			[]byte(`{"source":"local_path","ref":"/tmp/blocked","allow_unverified":true}`),
		)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusUnprocessableEntity,
				recorder.Body.String(),
			)
		}
		for _, want := range []string{
			contract.CodeExtensionUnverifiedPolicyBlocked,
			"Settings › Extensions",
			"extensions.trust.allow_unverified",
		} {
			if !strings.Contains(recorder.Body.String(), want) {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), want)
			}
		}
	})
}

func TestExtensionEnablementHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should list and set profile enablement", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		handlers := newTestHandlersWithExtensions(t, stubSessionManager{}, stubObserver{}, stubExtensionService{
			ListEnablementFn: func(_ context.Context, name string) ([]contract.ExtensionEnablementPayload, error) {
				if name != "ext-a" {
					t.Fatalf("ListEnablement() name = %q, want ext-a", name)
				}
				return []contract.ExtensionEnablementPayload{{Profile: "default", Enabled: true}}, nil
			},
			SetEnablementFn: func(
				_ context.Context,
				name string,
				req contract.SetExtensionEnablementRequest,
				_ taskpkg.ActorContext,
			) (contract.ExtensionEnablementPayload, error) {
				if name != "ext-a" || req.Profile != "finance" || req.Enabled {
					t.Fatalf("SetEnablement() name = %q req = %#v", name, req)
				}
				return contract.ExtensionEnablementPayload(req), nil
			},
		}, homePaths)
		engine := newTestRouter(t, handlers)

		listResp := performRequest(t, engine, http.MethodGet, "/api/extensions/%20ext-a%20/enablement", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list status = %d, want %d; body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
		}

		setResp := performRequest(t, engine, http.MethodPut, "/api/extensions/%20ext-a%20/enablement",
			[]byte(`{"profile":"finance","enabled":false}`))
		if setResp.Code != http.StatusOK {
			t.Fatalf(
				"set status = %d, want %d; body=%s",
				setResp.Code,
				http.StatusOK,
				setResp.Body.String(),
			)
		}
	})

	t.Run("Should reject blank extension names", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		engine := newTestRouter(
			t,
			newTestHandlersWithExtensions(t, stubSessionManager{}, stubObserver{}, stubExtensionService{}, homePaths),
		)

		blankName := performRequest(t, engine, http.MethodPut, "/api/extensions/%20%20/enablement",
			[]byte(`{"profile":"default","enabled":true}`))
		if blankName.Code != http.StatusBadRequest {
			t.Fatalf(
				"blank name status = %d, want %d; body=%s",
				blankName.Code,
				http.StatusBadRequest,
				blankName.Body.String(),
			)
		}
		if !strings.Contains(strings.ToLower(blankName.Body.String()), "name") {
			t.Fatalf("blank name body = %q, want substring %q", blankName.Body.String(), "name")
		}
	})
}

func TestExtensionStatusCodeMappings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "ShouldMapNilToOK", err: nil, want: http.StatusOK},
		{name: "ShouldMapNotFoundToNotFound", err: extensionpkg.ErrExtensionNotFound, want: http.StatusNotFound},
		{
			name: "ShouldMapChecksumMismatchToBadRequest",
			err:  extensionpkg.ErrExtensionChecksumMismatch,
			want: http.StatusBadRequest,
		},
		{
			name: "ShouldMapCuratedDigestMismatchToBadRequest",
			err:  extensionpkg.ErrExtensionArchiveDigestMismatch,
			want: http.StatusBadRequest,
		},
		{
			name: "ShouldMapUnverifiedPolicyBlockToUnprocessable",
			err:  extensionpkg.ErrExtensionUnverifiedPolicyBlocked,
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "ShouldMapExistingExtensionToConflict",
			err:  extensionpkg.ErrExtensionExists,
			want: http.StatusConflict,
		},
		{
			name: "ShouldMapInvalidManifestToBadRequest",
			err:  extensionpkg.ErrManifestInvalid,
			want: http.StatusBadRequest,
		},
		{
			name: "ShouldMapIncompatibleManifestToBadRequest",
			err:  extensionpkg.ErrManifestIncompatible,
			want: http.StatusBadRequest,
		},
		{
			name: "ShouldMapMissingManifestToBadRequest",
			err:  extensionpkg.ErrManifestNotFound,
			want: http.StatusBadRequest,
		},
		{name: "ShouldMapMissingFilesToBadRequest", err: os.ErrNotExist, want: http.StatusBadRequest},
		{
			name: "ShouldMapUnexpectedErrorsToInternalServerError",
			err:  errors.New("boom"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := extensionStatusCode(tt.err); got != tt.want {
				t.Fatalf("extensionStatusCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestApproveSessionHandler(t *testing.T) {
	t.Parallel()

	t.Run("Should validate and route approval requests", func(t *testing.T) {
		t.Parallel()

		var (
			gotID  string
			gotReq acp.ApproveRequest
		)
		homePaths := newTestHomePaths(t)
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{
			ApproveFn: func(
				_ context.Context,
				id string,
				req acp.ApproveRequest,
			) (session.ApprovalResult, error) {
				gotID = id
				gotReq = req
				return session.ApprovalResult{
					Outcome:   "applied",
					RequestID: req.RequestID,
					Decision:  req.Decision,
				}, nil
			},
		}, stubObserver{}, homePaths))

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-1/approve",
			[]byte(`{"request_id":"req-1","turn_id":"turn-1","decision":"allow-once"}`),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf(
				"approve status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusOK,
				recorder.Body.String(),
			)
		}
		if gotID != "sess-1" {
			t.Fatalf("approve id = %q, want sess-1", gotID)
		}
		if gotReq.RequestID != "req-1" || gotReq.TurnID != "turn-1" || gotReq.Decision != "allow-once" {
			t.Fatalf("approve request = %#v", gotReq)
		}
		var response contract.SessionApprovalResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Outcome != "applied" || response.RequestID != "req-1" ||
			response.Decision != "allow-once" {
			t.Fatalf("approve response = %#v", response)
		}
	})
}
