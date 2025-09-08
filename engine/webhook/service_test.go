package webhook

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newOrchestratorForTest(t *testing.T, disp services.SignalDispatcher) *Orchestrator {
	t.Helper()
	eval, err := task.NewCELEvaluator()
	require.NoError(t, err)
	filter := NewCELAdapter(eval)
	redisClient := &MockRedisClient{}
	redisSvc := NewRedisService(redisClient)
	reg := &MockLookup{}
	return NewOrchestrator(reg, filter, disp, redisSvc, 1024*64, time.Minute)
}

func TestOrchestrator_Process(t *testing.T) {
	t.Run("Should return 404 when slug not found", func(t *testing.T) {
		o := newOrchestratorForTest(t, services.NewMockSignalDispatcher())
		r := httptestRequest("{}")

		// Setup mock expectations
		reg := o.reg.(*MockLookup)
		reg.On("Get", "missing").Return(RegistryEntry{}, false)

		res, err := o.Process(context.Background(), "missing", r)
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, res.Status)
		reg.AssertExpectations(t)
	})

	t.Run("Should return 400 for invalid JSON body", func(t *testing.T) {
		o := newOrchestratorForTest(t, services.NewMockSignalDispatcher())
		r := httptestRequest("{invalid}")

		// Setup mock expectations
		reg := o.reg.(*MockLookup)
		entry := RegistryEntry{
			WorkflowID: "wf1",
			Webhook: &Config{
				Slug: "test",
				Events: []EventConfig{
					{Name: "ev", Filter: "true", Input: map[string]string{"id": "{{ .payload.id }}"}},
				},
			},
		}
		reg.On("Get", "test").Return(entry, true)

		res, err := o.Process(context.Background(), "test", r)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, res.Status)
		reg.AssertExpectations(t)
	})

	t.Run("Should return 401 when verification fails", func(t *testing.T) {
		disp := services.NewMockSignalDispatcher()
		o := newOrchestratorForTest(t, disp)

		// Setup failing verifier
		failingVerifier := &MockVerifier{}
		failingVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)
		o.verifierFactory = func(VerifyConfig) (Verifier, error) { return failingVerifier, nil }

		r := httptestRequest("{\"id\":\"1\"}")

		// Setup mock expectations
		reg := o.reg.(*MockLookup)
		entry := RegistryEntry{
			WorkflowID: "wf1",
			Webhook: &Config{
				Slug:   "test",
				Verify: &VerifySpec{Strategy: StrategyHMAC, Header: "X-Sig", Secret: "s"},
				Events: []EventConfig{
					{Name: "ev", Filter: "true", Input: map[string]string{"id": "{{ .payload.id }}"}},
				},
			},
		}
		reg.On("Get", "test").Return(entry, true)

		res, err := o.Process(context.Background(), "test", r)
		require.Error(t, err)
		assert.Equal(t, http.StatusUnauthorized, res.Status)
		assert.Len(t, disp.Calls, 0)
		reg.AssertExpectations(t)
		failingVerifier.AssertExpectations(t)
	})

	t.Run("Should return 202 then 409 for duplicates", func(t *testing.T) {
		disp := services.NewMockSignalDispatcher()
		o := newOrchestratorForTest(t, disp)

		r1 := httptestRequest("{\"id\":\"1\"}")
		r1.Header.Set(HeaderIdempotencyKey, "K")
		r2 := httptestRequest("{\"id\":\"1\"}")
		r2.Header.Set(HeaderIdempotencyKey, "K")

		// Setup mock expectations
		reg := o.reg.(*MockLookup)
		entry := RegistryEntry{
			WorkflowID: "wf2",
			Webhook: &Config{
				Slug:   "dupe",
				Dedupe: &DedupeSpec{Enabled: true, TTL: "1m"},
				Events: []EventConfig{
					{Name: "ev", Filter: "true", Input: map[string]string{"id": "{{ .payload.id }}"}},
				},
			},
		}
		reg.On("Get", "dupe").Return(entry, true)

		// Setup Redis mock - first call succeeds, second fails (duplicate)
		redisClient := o.idem.(*redisSvc).client.(*MockRedisClient)
		redisClient.On("SetNX", mock.Anything, "idempotency:webhook:dupe:K", 1, mock.AnythingOfType("time.Duration")).
			Return(true, nil).
			Once()
		redisClient.On("SetNX", mock.Anything, "idempotency:webhook:dupe:K", 1, mock.AnythingOfType("time.Duration")).
			Return(false, nil).
			Once()

		res, err := o.Process(context.Background(), "dupe", r1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, res.Status)

		res, err = o.Process(context.Background(), "dupe", r2)
		require.Error(t, err)
		assert.Equal(t, http.StatusConflict, res.Status)

		reg.AssertExpectations(t)
		redisClient.AssertExpectations(t)
	})

	t.Run("Should return 204 (router maps to 200) when no matching event", func(t *testing.T) {
		disp := services.NewMockSignalDispatcher()
		o := newOrchestratorForTest(t, disp)
		r := httptestRequest("{\"id\":\"1\"}")

		// Setup mock expectations
		reg := o.reg.(*MockLookup)
		entry := RegistryEntry{
			WorkflowID: "wf3",
			Webhook: &Config{
				Slug: "flt",
				Events: []EventConfig{
					{Name: "ev", Filter: "false", Input: map[string]string{"id": "{{ .payload.id }}"}},
				},
			},
		}
		reg.On("Get", "flt").Return(entry, true)

		res, err := o.Process(context.Background(), "flt", r)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, res.Status)
		assert.Equal(t, map[string]any{"status": "no_matching_event"}, res.Payload)
		reg.AssertExpectations(t)
	})

	t.Run("Should dispatch on success with correlation ID", func(t *testing.T) {
		disp := services.NewMockSignalDispatcher()
		o := newOrchestratorForTest(t, disp)
		r := httptestRequest("{\"id\":\"evt_123\"}")
		r.Header.Set("X-Correlation-ID", "corr-1")

		// Setup mock expectations
		reg := o.reg.(*MockLookup)
		entry := RegistryEntry{
			WorkflowID: "wf4",
			Webhook: &Config{
				Slug: "ok",
				Events: []EventConfig{
					{
						Name:   "payment.created",
						Filter: "true",
						Input:  map[string]string{"event_id": "{{ .payload.id }}"},
					},
				},
			},
		}
		reg.On("Get", "ok").Return(entry, true)

		res, err := o.Process(context.Background(), "ok", r)
		require.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, res.Status)
		require.Len(t, disp.Calls, 1)
		assert.Equal(t, "payment.created", disp.Calls[0].SignalName)
		assert.Equal(t, "corr-1", disp.Calls[0].CorrelationID)
		assert.Equal(t, "evt_123", disp.Calls[0].Payload["event_id"])
		reg.AssertExpectations(t)
	})
}

func httptestRequest(body string) *http.Request {
	req, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.com",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")
	return req
}
