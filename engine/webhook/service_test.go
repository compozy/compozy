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
	"github.com/stretchr/testify/require"
)

type failingVerifier struct{}

func (f failingVerifier) Verify(_ context.Context, _ *http.Request, _ []byte) error {
	return assert.AnError
}

type mapRedis struct {
	seen map[string]struct{}
}

func (m *mapRedis) SetNX(_ context.Context, key string, _ any, _ time.Duration) (bool, error) {
	if m.seen == nil {
		m.seen = map[string]struct{}{}
	}
	if _, ok := m.seen[key]; ok {
		return false, nil
	}
	m.seen[key] = struct{}{}
	return true, nil
}

type mockRegistry struct {
	entry RegistryEntry
	ok    bool
}

func (m *mockRegistry) Get(_ string) (RegistryEntry, bool) {
	return m.entry, m.ok
}

func newOrchestratorForTest(t *testing.T, disp services.SignalDispatcher) *Orchestrator {
	t.Helper()
	eval, err := task.NewCELEvaluator()
	require.NoError(t, err)
	filter := NewCELAdapter(eval)
	idem := NewRedisClient(&mapRedis{})
	reg := &mockRegistry{}
	return NewOrchestrator(reg, filter, disp, idem, 1024*64, time.Minute)
}

func TestOrchestrator_NotFound(t *testing.T) {
	o := newOrchestratorForTest(t, services.NewMockSignalDispatcher())
	r := httptestRequest("{}")
	res, err := o.Process(context.Background(), "missing", r)
	require.Error(t, err)
	assert.Equal(t, http.StatusNotFound, res.Status)
}

func TestOrchestrator_InvalidBody(t *testing.T) {
	o := newOrchestratorForTest(t, services.NewMockSignalDispatcher())
	o.reg = &mockRegistry{
		entry: RegistryEntry{
			WorkflowID: "wf1",
			Webhook: &Config{
				Slug: "test",
				Events: []EventConfig{
					{Name: "ev", Filter: "true", Input: map[string]string{"id": "{{ .payload.id }}"}},
				},
			},
		},
		ok: true,
	}
	r := httptestRequest("{invalid}")
	res, err := o.Process(context.Background(), "test", r)
	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, res.Status)
}

func TestOrchestrator_VerificationFailure(t *testing.T) {
	disp := services.NewMockSignalDispatcher()
	o := newOrchestratorForTest(t, disp)
	o.verifierFactory = func(VerifyConfig) (Verifier, error) { return failingVerifier{}, nil }
	o.reg = &mockRegistry{
		entry: RegistryEntry{
			WorkflowID: "wf1",
			Webhook: &Config{
				Slug:   "test",
				Verify: &VerifySpec{Strategy: StrategyHMAC, Header: "X-Sig", Secret: "s"},
				Events: []EventConfig{
					{Name: "ev", Filter: "true", Input: map[string]string{"id": "{{ .payload.id }}"}},
				},
			},
		},
		ok: true,
	}
	r := httptestRequest("{\"id\":\"1\"}")
	res, err := o.Process(context.Background(), "test", r)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, res.Status)
	assert.Len(t, disp.Calls, 0)
}

func TestOrchestrator_Duplicate(t *testing.T) {
	disp := services.NewMockSignalDispatcher()
	o := newOrchestratorForTest(t, disp)
	o.reg = &mockRegistry{
		entry: RegistryEntry{
			WorkflowID: "wf2",
			Webhook: &Config{
				Slug:   "dupe",
				Dedupe: &DedupeSpec{Enabled: true, TTL: "1m"},
				Events: []EventConfig{
					{Name: "ev", Filter: "true", Input: map[string]string{"id": "{{ .payload.id }}"}},
				},
			},
		},
		ok: true,
	}
	r1 := httptestRequest("{\"id\":\"1\"}")
	r1.Header.Set(HeaderIdempotencyKey, "K")
	r2 := httptestRequest("{\"id\":\"1\"}")
	r2.Header.Set(HeaderIdempotencyKey, "K")
	res, err := o.Process(context.Background(), "dupe", r1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, res.Status)
	res, err = o.Process(context.Background(), "dupe", r2)
	require.Error(t, err)
	assert.Equal(t, http.StatusConflict, res.Status)
}

func TestOrchestrator_FilteredNoMatch(t *testing.T) {
	disp := services.NewMockSignalDispatcher()
	o := newOrchestratorForTest(t, disp)
	o.reg = &mockRegistry{
		entry: RegistryEntry{
			WorkflowID: "wf3",
			Webhook: &Config{
				Slug: "flt",
				Events: []EventConfig{
					{Name: "ev", Filter: "false", Input: map[string]string{"id": "{{ .payload.id }}"}},
				},
			},
		},
		ok: true,
	}
	r := httptestRequest("{\"id\":\"1\"}")
	res, err := o.Process(context.Background(), "flt", r)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, res.Status)
	assert.Equal(t, map[string]any{"status": "no_matching_event"}, res.Payload)
}

func TestOrchestrator_SuccessDispatch(t *testing.T) {
	disp := services.NewMockSignalDispatcher()
	o := newOrchestratorForTest(t, disp)
	o.reg = &mockRegistry{
		entry: RegistryEntry{
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
		},
		ok: true,
	}
	r := httptestRequest("{\"id\":\"evt_123\"}")
	r.Header.Set("X-Correlation-ID", "corr-1")
	res, err := o.Process(context.Background(), "ok", r)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, res.Status)
	require.Len(t, disp.Calls, 1)
	assert.Equal(t, "payment.created", disp.Calls[0].SignalName)
	assert.Equal(t, "corr-1", disp.Calls[0].CorrelationID)
	assert.Equal(t, "evt_123", disp.Calls[0].Payload["event_id"])
}

func httptestRequest(body string) *http.Request {
	req, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.com",
		bytes.NewBufferString(body),
	)
	return req
}
