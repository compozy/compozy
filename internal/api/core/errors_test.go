package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/admission"
	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/diagnostics"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestStatusForBridgeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "Should return bad request for body path mismatch",
			err:  contract.ErrBridgeInstanceMismatch,
			want: http.StatusBadRequest,
		},
		{
			name: "Should return bad request for invalid secret binding",
			err:  bridgepkg.ErrInvalidBridgeSecretBinding,
			want: http.StatusBadRequest,
		},
		{
			name: "Should return not found for missing bridge",
			err:  bridgepkg.ErrBridgeInstanceNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "Should return not found for missing route",
			err:  bridgepkg.ErrBridgeRouteNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "Should return not found for missing workspace",
			err:  workspacepkg.ErrWorkspaceNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "Should return conflict for unavailable instance",
			err:  bridgepkg.ErrBridgeInstanceUnavailable,
			want: http.StatusConflict,
		},
		{
			name: "Should return conflict for invalid state transition",
			err:  bridgepkg.ErrInvalidBridgeStateTransition,
			want: http.StatusConflict,
		},
		{
			name: "Should return not found for missing delivery",
			err:  bridgepkg.ErrDeliveryNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "Should return service unavailable for saturated delivery queue",
			err:  bridgepkg.ErrDeliveryQueueSaturated,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "Should return service unavailable for transport outage",
			err:  bridgepkg.ErrDeliveryTransportUnavailable,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "Should return service unavailable for bridge control transport outage",
			err:  bridgepkg.ErrBridgeControlTransportUnavailable,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "Should return internal server error for unknown failures",
			err:  errors.New("boom"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StatusForBridgeError(tt.err); got != tt.want {
				t.Fatalf("StatusForBridgeError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestTaskErrorHelpers(t *testing.T) {
	t.Parallel()

	wrapped := NewTaskValidationError(errors.New("bad input"))
	if !errors.Is(wrapped, taskpkg.ErrValidation) {
		t.Fatalf("NewTaskValidationError() = %v, want wrapped task validation sentinel", wrapped)
	}
	if got := NewTaskValidationError(nil); got != nil {
		t.Fatalf("NewTaskValidationError(nil) = %v, want nil", got)
	}

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: http.StatusOK},
		{name: "invalid scope", err: taskpkg.ErrInvalidScopeBinding, want: http.StatusBadRequest},
		{name: "immutable field", err: taskpkg.ErrImmutableField, want: http.StatusBadRequest},
		{name: "run missing", err: taskpkg.ErrTaskRunNotFound, want: http.StatusNotFound},
		{name: "session missing", err: session.ErrSessionNotFound, want: http.StatusNotFound},
		{name: "os not exist", err: os.ErrNotExist, want: http.StatusNotFound},
		{name: "workspace root missing", err: workspacepkg.ErrWorkspaceRootMissing, want: http.StatusGone},
		{
			name: "workspace resolver unavailable",
			err:  workspacepkg.ErrWorkspaceResolverUnavailable,
			want: http.StatusServiceUnavailable,
		},
		{name: "attach forbidden", err: taskpkg.ErrSessionAttachNotAllowed, want: http.StatusConflict},
		{name: "workspace capacity full", err: taskpkg.ErrWorkspaceActiveRunCapReached, want: http.StatusConflict},
		{name: "default", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StatusForTaskError(tt.err); got != tt.want {
				t.Fatalf("StatusForTaskError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestAutomationAndNetworkErrorHelpers(t *testing.T) {
	t.Parallel()

	automationErr := NewAutomationValidationError(errors.New("bad automation request"))
	if !errors.Is(automationErr, ErrAutomationValidation) {
		t.Fatalf("NewAutomationValidationError() = %v, want ErrAutomationValidation", automationErr)
	}
	if got := NewAutomationValidationError(nil); got != nil {
		t.Fatalf("NewAutomationValidationError(nil) = %v, want nil", got)
	}

	networkErr := NewNetworkValidationError(errors.New("bad network request"))
	if !errors.Is(networkErr, ErrNetworkValidation) {
		t.Fatalf("NewNetworkValidationError() = %v, want ErrNetworkValidation", networkErr)
	}
	if got := NewNetworkValidationError(nil); got != nil {
		t.Fatalf("NewNetworkValidationError(nil) = %v, want nil", got)
	}

	if got := StatusForAutomationError(nil); got != http.StatusOK {
		t.Fatalf("StatusForAutomationError(nil) = %d, want %d", got, http.StatusOK)
	}
	if got := StatusForAutomationError(automationpkg.ErrManagerNotRunning); got != http.StatusServiceUnavailable {
		t.Fatalf("StatusForAutomationError(manager not running) = %d, want %d", got, http.StatusServiceUnavailable)
	}
	if got := StatusForAutomationError(automationpkg.ErrWebhookSignatureInvalid); got != http.StatusUnauthorized {
		t.Fatalf("StatusForAutomationError(signature invalid) = %d, want %d", got, http.StatusUnauthorized)
	}
	if got := StatusForAutomationError(automationpkg.ErrListCursorInvalid); got != http.StatusBadRequest {
		t.Fatalf("StatusForAutomationError(invalid cursor) = %d, want %d", got, http.StatusBadRequest)
	}
	if got := StatusForAutomationError(looppkg.ErrValidation); got != http.StatusUnprocessableEntity {
		t.Fatalf("StatusForAutomationError(loop validation) = %d, want %d", got, http.StatusUnprocessableEntity)
	}
	if got := StatusForAutomationError(looppkg.ErrDefinitionNotFound); got != http.StatusNotFound {
		t.Fatalf("StatusForAutomationError(loop not found) = %d, want %d", got, http.StatusNotFound)
	}

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: http.StatusOK},
		{name: "validation", err: ErrNetworkValidation, want: http.StatusBadRequest},
		{name: "local peer missing", err: network.ErrLocalPeerNotFound, want: http.StatusNotFound},
		{name: "missing field", err: network.ErrMissingField, want: http.StatusBadRequest},
		{name: "default", err: errors.New("boom"), want: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StatusForNetworkError(tt.err); got != tt.want {
				t.Fatalf("StatusForNetworkError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestLoopErrorHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "Should map nil Loop errors to OK", err: nil, want: http.StatusOK},
		{
			name: "Should map duplicate Loop definitions to conflict",
			err:  looppkg.ErrDefinitionExists,
			want: http.StatusConflict,
		},
		{
			name: "Should map missing Loop definitions to not found",
			err:  looppkg.ErrDefinitionNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "Should map Loop validation errors to bad request",
			err:  looppkg.ErrValidation,
			want: http.StatusBadRequest,
		},
		{
			name: "Should map invalid Loop transitions to unprocessable entity",
			err:  looppkg.ErrInvalidTransition,
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "Should map Loop transition conflicts to conflict",
			err:  looppkg.ErrTransitionConflict,
			want: http.StatusConflict,
		},
		{
			name: "Should map Loop catalog cursor errors to bad request",
			err:  looppkg.ErrCatalogCursorInvalid,
			want: http.StatusBadRequest,
		},
		{
			name: "Should map missing Loop catalog workspaces to not found",
			err:  workspacepkg.ErrWorkspaceNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "Should map missing Loop catalog roots to gone",
			err:  workspacepkg.ErrWorkspaceRootMissing,
			want: http.StatusGone,
		},
		{
			name: "Should map unavailable Loop catalogs to service unavailable",
			err:  looppkg.ErrCatalogUnavailable,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "Should preserve missing actor identity as unauthorized",
			err:  agentidentity.ErrIdentityRequired,
			want: http.StatusUnauthorized,
		},
		{
			name: "Should preserve unavailable actor lookup as service unavailable",
			err:  agentidentity.ErrIdentityLookupUnavailable,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "Should map unknown Loop errors to internal server error",
			err:  errors.New("boom"),
			want: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StatusForLoopError(tt.err); got != tt.want {
				t.Fatalf("StatusForLoopError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestRespondErrorFallbackBranches(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		status     int
		err        error
		mask       bool
		wantErr    string
		wantStatus int
	}{
		{name: "unknown error fallback", status: 0, err: nil, mask: false, wantErr: "unknown error", wantStatus: 200},
		{
			name:       "masked internal fallback",
			status:     599,
			err:        nil,
			mask:       true,
			wantErr:    "internal server error",
			wantStatus: 599,
		},
		{
			name:       "known draining error remains actionable",
			status:     http.StatusServiceUnavailable,
			err:        admission.ErrDraining,
			mask:       true,
			wantErr:    admission.ErrDraining.Error(),
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			RespondError(ctx, tt.status, tt.err, tt.mask)

			var payload contract.ErrorPayload
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if payload.Error != tt.wantErr {
				t.Fatalf("payload.Error = %q, want %q", payload.Error, tt.wantErr)
			}
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}

func TestRespondErrorDiagnosticPayload(t *testing.T) {
	t.Run("Should include redacted diagnostic when error carries one", func(t *testing.T) {
		// not parallel: gin.SetMode mutates process-global state.
		gin.SetMode(gin.TestMode)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		item := diagnostics.NewItem(
			"api.config_invalid",
			contract.CodeConfigInvalid,
			contract.CategoryConfig,
			"Config invalid",
			"config token=api-secret failed",
			contract.SeverityCritical,
			contract.FreshnessLive,
			diagnostics.WithEvidence(map[string]any{"api_key": "sk-secret"}),
		)
		RespondError(
			ctx,
			http.StatusUnprocessableEntity,
			diagnostics.NewStructuredError(item, errors.New("token=cause-secret")),
			false,
		)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
		}

		var payload contract.ErrorPayload
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if payload.Diagnostic == nil {
			t.Fatal("payload.Diagnostic = nil, want diagnostic")
		}
		if payload.Diagnostic.Code != contract.CodeConfigInvalid {
			t.Fatalf("payload.Diagnostic.Code = %q, want %q", payload.Diagnostic.Code, contract.CodeConfigInvalid)
		}
		if payload.Diagnostic.Evidence["api_key"] != "[REDACTED]" {
			t.Fatalf(
				"payload.Diagnostic.Evidence[api_key] = %#v, want redacted",
				payload.Diagnostic.Evidence["api_key"],
			)
		}
		for _, leaked := range []string{"api-secret", "sk-secret", "cause-secret"} {
			if body := recorder.Body.String(); strings.Contains(body, leaked) {
				t.Fatalf("response body = %s leaked %q", body, leaked)
			}
		}
	})
}

func TestRespondOpenAIErrorRedaction(t *testing.T) {
	t.Run("Should redact secret-shaped OpenAI error messages", func(t *testing.T) {
		// not parallel: gin.SetMode mutates process-global state.
		gin.SetMode(gin.TestMode)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)

		RespondOpenAIError(
			ctx,
			http.StatusBadRequest,
			errors.New("provider returned Authorization: Bearer sk-openai-secret"),
			false,
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
		}

		var payload contract.OpenAIErrorResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if strings.Contains(payload.Error.Message, "sk-openai-secret") {
			t.Fatalf("payload.Error.Message = %q leaked secret", payload.Error.Message)
		}
		if !strings.Contains(payload.Error.Message, "[REDACTED]") {
			t.Fatalf("payload.Error.Message = %q, want redacted marker", payload.Error.Message)
		}
	})
}

func TestErrorPayloadForError(t *testing.T) {
	t.Parallel()

	t.Run("Should redact structured diagnostic secrets in serialized payload", func(t *testing.T) {
		t.Parallel()

		item := diagnostics.NewItem(
			"stream.daemon_unavailable",
			contract.CodeDaemonUnavailable,
			contract.CategoryDaemon,
			"Daemon unavailable",
			"socket token=stream-secret failed",
			contract.SeverityError,
			contract.FreshnessOffline,
		)
		payload := ErrorPayloadForError(diagnostics.NewStructuredError(item, errors.New("token=cause-secret")))
		if payload.Diagnostic == nil {
			t.Fatal("payload.Diagnostic = nil, want diagnostic")
		}
		if payload.Diagnostic.Code != contract.CodeDaemonUnavailable {
			t.Fatalf("payload.Diagnostic.Code = %q, want %q", payload.Diagnostic.Code, contract.CodeDaemonUnavailable)
		}
		for _, leaked := range []string{"stream-secret", "cause-secret"} {
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if strings.Contains(string(raw), leaked) {
				t.Fatalf("payload = %s leaked %q", raw, leaked)
			}
		}
	})
}
