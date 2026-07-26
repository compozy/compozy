package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/support"
	"github.com/gin-gonic/gin"
)

type supportBundleServiceStub struct {
	createFn       func(context.Context, support.CreateRequest) (support.Operation, error)
	getFn          func(context.Context, string) (support.Operation, error)
	downloadPathFn func(context.Context, string) (support.Operation, string, error)
}

func (s supportBundleServiceStub) Create(
	ctx context.Context,
	request support.CreateRequest,
) (support.Operation, error) {
	if s.createFn != nil {
		return s.createFn(ctx, request)
	}
	return support.Operation{}, support.ErrOperationNotFound
}

func (s supportBundleServiceStub) Get(ctx context.Context, operationID string) (support.Operation, error) {
	if s.getFn != nil {
		return s.getFn(ctx, operationID)
	}
	return support.Operation{}, support.ErrOperationNotFound
}

func (s supportBundleServiceStub) DownloadPath(
	ctx context.Context,
	operationID string,
) (support.Operation, string, error) {
	if s.downloadPathFn != nil {
		return s.downloadPathFn(ctx, operationID)
	}
	return support.Operation{}, "", support.ErrOperationNotFound
}

func TestBaseHandlersSupportBundles(t *testing.T) {
	t.Parallel()

	t.Run("ShouldCreateAndPollSupportBundleOperations", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
		var includeStatus bool
		engine := newSupportBundleTestEngine(t, supportBundleServiceStub{
			createFn: func(_ context.Context, request support.CreateRequest) (support.Operation, error) {
				includeStatus = request.IncludeStatus
				return support.Operation{
					OperationID: "op_123",
					Status:      support.OperationPending,
					CreatedAt:   now,
					UpdatedAt:   now,
				}, nil
			},
			getFn: func(_ context.Context, operationID string) (support.Operation, error) {
				if operationID != "op_123" {
					t.Fatalf("Get() operationID = %q, want op_123", operationID)
				}
				return support.Operation{
					OperationID: "op_123",
					Status:      support.OperationRunning,
					CreatedAt:   now,
					UpdatedAt:   now.Add(time.Second),
				}, nil
			},
		})

		createResp := performRequest(t, engine, http.MethodPost, "/support/bundles", []byte(`{"yes":true}`))
		if createResp.Code != http.StatusAccepted {
			t.Fatalf("POST /support/bundles status = %d body=%s", createResp.Code, createResp.Body.String())
		}
		if !includeStatus {
			t.Fatal("Create() IncludeStatus = false, want default true")
		}
		var createPayload contract.SupportBundleOperationResponse
		if err := json.Unmarshal(createResp.Body.Bytes(), &createPayload); err != nil {
			t.Fatalf("json.Unmarshal(create) error = %v", err)
		}
		if createPayload.Operation.OperationID != "op_123" ||
			!strings.HasSuffix(createPayload.Operation.StatusURL, "/api/support/bundles/op_123") {
			t.Fatalf("create payload = %#v, want operation URL", createPayload.Operation)
		}

		getResp := performRequest(t, engine, http.MethodGet, "/support/bundles/op_123", nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("GET /support/bundles/op_123 status = %d body=%s", getResp.Code, getResp.Body.String())
		}
		var getPayload contract.SupportBundleOperationResponse
		if err := json.Unmarshal(getResp.Body.Bytes(), &getPayload); err != nil {
			t.Fatalf("json.Unmarshal(get) error = %v", err)
		}
		if getPayload.Operation.Status != string(support.OperationRunning) {
			t.Fatalf("get payload status = %q, want running", getPayload.Operation.Status)
		}
	})

	t.Run("Should preserve request deadline when creating detached operations", func(t *testing.T) {
		t.Parallel()

		deadline := time.Now().UTC().Add(time.Hour)
		seenDeadline := time.Time{}
		engine := newSupportBundleTestEngine(t, supportBundleServiceStub{
			createFn: func(ctx context.Context, request support.CreateRequest) (support.Operation, error) {
				if !request.IncludeStatus {
					t.Fatal("Create() IncludeStatus = false, want default true")
				}
				var ok bool
				seenDeadline, ok = ctx.Deadline()
				if !ok {
					t.Fatal("Create() context has no deadline")
				}
				return support.Operation{
					OperationID: "op_deadline",
					Status:      support.OperationPending,
					CreatedAt:   deadline,
					UpdatedAt:   deadline,
				}, nil
			},
		})
		reqCtx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		request := httptest.NewRequestWithContext(
			reqCtx,
			http.MethodPost,
			"/support/bundles",
			strings.NewReader(`{"yes":true}`),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if response.Code != http.StatusAccepted {
			t.Fatalf("POST /support/bundles status = %d body=%s", response.Code, response.Body.String())
		}
		if !seenDeadline.Equal(deadline) {
			t.Fatalf("Create() deadline = %s, want %s", seenDeadline, deadline)
		}
	})

	t.Run("Should reject create without explicit consent diagnostic", func(t *testing.T) {
		t.Parallel()

		engine := newSupportBundleTestEngine(t, supportBundleServiceStub{
			createFn: func(context.Context, support.CreateRequest) (support.Operation, error) {
				t.Fatal("Create() called without support bundle consent")
				return support.Operation{}, nil
			},
		})

		response := performRequest(t, engine, http.MethodPost, "/support/bundles", nil)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("POST /support/bundles status = %d body=%s", response.Code, response.Body.String())
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(error) error = %v", err)
		}
		if payload.Diagnostic == nil || payload.Diagnostic.Code != contract.CodeBundleConsentRequired {
			t.Fatalf("diagnostic = %#v, want %s", payload.Diagnostic, contract.CodeBundleConsentRequired)
		}
	})

	t.Run("ShouldDownloadCompletedBundleAndRejectNotReadyOperation", func(t *testing.T) {
		t.Parallel()

		bundlePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
		if err := os.WriteFile(bundlePath, []byte("bundle-bytes"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", bundlePath, err)
		}
		engine := newSupportBundleTestEngine(t, supportBundleServiceStub{
			downloadPathFn: func(_ context.Context, operationID string) (support.Operation, string, error) {
				if operationID == "pending" {
					return support.Operation{}, "", support.ErrOperationNotReady
				}
				if operationID != "op_123" {
					return support.Operation{}, "", support.ErrOperationNotFound
				}
				return support.Operation{
					OperationID: "op_123",
					Status:      support.OperationCompleted,
					FileName:    "bundle.tar.gz",
				}, bundlePath, nil
			},
		})

		downResp := performRequest(t, engine, http.MethodGet, "/support/bundles/op_123/download", nil)
		if downResp.Code != http.StatusOK {
			t.Fatalf("download status = %d body=%s", downResp.Code, downResp.Body.String())
		}
		if downResp.Body.String() != "bundle-bytes" {
			t.Fatalf("download body = %q, want bundle-bytes", downResp.Body.String())
		}
		if got := downResp.Header().Get("Content-Disposition"); !strings.Contains(got, "bundle.tar.gz") {
			t.Fatalf("Content-Disposition = %q, want bundle filename", got)
		}

		pendingResp := performRequest(t, engine, http.MethodGet, "/support/bundles/pending/download", nil)
		if pendingResp.Code != http.StatusConflict {
			t.Fatalf("pending download status = %d body=%s", pendingResp.Code, pendingResp.Body.String())
		}
	})
}

func newSupportBundleTestEngine(t *testing.T, service supportBundleServiceStub) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	homePaths := testutil.NewTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName:      "api-core-test",
		MaskInternalErrors: false,
		SupportBundles:     service,
		HomePaths:          homePaths,
		Config:             cfg,
		Logger:             testutil.DiscardLogger(),
	})
	engine := gin.New()
	engine.POST("/support/bundles", handlers.CreateSupportBundle)
	engine.GET("/support/bundles/:operation_id", handlers.GetSupportBundle)
	engine.GET("/support/bundles/:operation_id/download", handlers.DownloadSupportBundle)
	return engine
}
