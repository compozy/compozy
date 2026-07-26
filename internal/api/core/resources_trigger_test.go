package core

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/agh/internal/resources"
	"github.com/gin-gonic/gin"
)

func TestResourceContractTriggerFailures(t *testing.T) {
	t.Parallel()

	const genericKind resources.ResourceKind = "test.resource"
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	triggerErr := errors.New("reconcile trigger unavailable")

	t.Run("Should return created resource after committed put when trigger fails", func(t *testing.T) {
		t.Parallel()

		committed := false
		service, err := NewOperatorResourceService(&ResourceServiceConfig{
			RawStore: stubRawStore{
				PutRawFn: func(_ context.Context, _ resources.MutationActor, draft resources.RawDraft) (resources.RawRecord, error) {
					committed = true
					return resources.RawRecord{
						Kind:    draft.Kind,
						ID:      draft.ID,
						Version: 1,
						Scope:   draft.Scope,
						Owner: resources.ResourceOwner{
							Kind: resources.ResourceOwnerKind("daemon"),
							ID:   "daemon-control",
						},
						Source:    resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "system"},
						SpecJSON:  append([]byte(nil), draft.SpecJSON...),
						CreatedAt: now,
						UpdatedAt: now,
					}, nil
				},
			},
			Trigger: func(context.Context, resources.ResourceKind, resources.ReconcileReason) error {
				return triggerErr
			},
		})
		if err != nil {
			t.Fatalf("NewOperatorResourceService() error = %v", err)
		}
		handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "core-test", Resources: service})
		ctx, recorder := newResourceRequestContext(
			t,
			http.MethodPut,
			"/api/resources/test.resource/demo",
			[]byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
			gin.Params{{Key: "kind", Value: string(genericKind)}, {Key: "id", Value: "demo"}},
		)

		handlers.PutResource(ctx)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
		}
		if !committed {
			t.Fatal("PutRaw() was not committed before trigger failure")
		}
	})

	t.Run("Should return deleted resource after committed delete when trigger fails", func(t *testing.T) {
		t.Parallel()

		committed := false
		service, err := NewOperatorResourceService(&ResourceServiceConfig{
			RawStore: stubRawStore{
				DeleteRawFn: func(_ context.Context, _ resources.MutationActor, kind resources.ResourceKind, id string, expectedVersion int64) error {
					committed = true
					if kind != genericKind || id != "demo" || expectedVersion != 3 {
						t.Fatalf("DeleteRaw() args = kind:%q id:%q expected_version:%d", kind, id, expectedVersion)
					}
					return nil
				},
			},
			Trigger: func(context.Context, resources.ResourceKind, resources.ReconcileReason) error {
				return triggerErr
			},
		})
		if err != nil {
			t.Fatalf("NewOperatorResourceService() error = %v", err)
		}
		handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "core-test", Resources: service})
		ctx, recorder := newResourceRequestContext(
			t,
			http.MethodDelete,
			"/api/resources/test.resource/demo",
			[]byte(`{"expected_version":3}`),
			gin.Params{{Key: "kind", Value: string(genericKind)}, {Key: "id", Value: "demo"}},
		)

		handlers.DeleteResource(ctx)

		if got := ctx.Writer.Status(); got != http.StatusNoContent {
			t.Fatalf(
				"status = %d, want %d; recorder=%d body=%s",
				got,
				http.StatusNoContent,
				recorder.Code,
				recorder.Body.String(),
			)
		}
		if !committed {
			t.Fatal("DeleteRaw() was not committed before trigger failure")
		}
	})
}
