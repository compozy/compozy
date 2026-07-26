package udsapi

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	extensionpkg "github.com/compozy/agh/internal/extension"
	taskpkg "github.com/compozy/agh/internal/task"
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
				if req.Path != "/tmp/ext-a" || req.Checksum != "sha256:abc" {
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
			[]byte(`{"path":" /tmp/ext-a ","checksum":" sha256:abc "}`),
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
			name:        "ShouldRejectMissingPath",
			payload:     `{"checksum":"sha256:abc"}`,
			wantMessage: "path",
		},
		{
			name:        "ShouldRejectMissingChecksum",
			payload:     `{"path":"/tmp/ext-a"}`,
			wantMessage: "checksum",
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
			[]byte(`{"path":"/tmp/blocked","checksum":"sha256:abc","allow_unverified":true}`),
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
			"extensions.marketplace.allow_unverified",
		} {
			if !strings.Contains(recorder.Body.String(), want) {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), want)
			}
		}
	})
}

func TestEnableDisableExtensionHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should enable and disable extensions", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		handlers := newTestHandlersWithExtensions(t, stubSessionManager{}, stubObserver{}, stubExtensionService{
			EnableFn: func(
				_ context.Context,
				name string,
				_ taskpkg.ActorContext,
			) (contract.ExtensionPayload, error) {
				if name != "ext-a" {
					t.Fatalf("Enable() name = %q, want ext-a", name)
				}
				return contract.ExtensionPayload{Name: name, Enabled: true, State: "active"}, nil
			},
			DisableFn: func(
				_ context.Context,
				name string,
				_ taskpkg.ActorContext,
			) (contract.ExtensionPayload, error) {
				if name != "ext-a" {
					t.Fatalf("Disable() name = %q, want ext-a", name)
				}
				return contract.ExtensionPayload{Name: name, Enabled: false, State: "inactive"}, nil
			},
		}, homePaths)
		engine := newTestRouter(t, handlers)

		enableResp := performRequest(t, engine, http.MethodPost, "/api/extensions/%20ext-a%20/enable", nil)
		if enableResp.Code != http.StatusOK {
			t.Fatalf("enable status = %d, want %d; body=%s", enableResp.Code, http.StatusOK, enableResp.Body.String())
		}

		disableResp := performRequest(t, engine, http.MethodPost, "/api/extensions/%20ext-a%20/disable", nil)
		if disableResp.Code != http.StatusOK {
			t.Fatalf(
				"disable status = %d, want %d; body=%s",
				disableResp.Code,
				http.StatusOK,
				disableResp.Body.String(),
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

		blankName := performRequest(t, engine, http.MethodPost, "/api/extensions/%20%20/enable", nil)
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
			ApproveFn: func(_ context.Context, id string, req acp.ApproveRequest) error {
				gotID = id
				gotReq = req
				return nil
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
	})
}
