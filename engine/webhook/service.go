package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// Error taxonomy (router maps to HTTP later)
var (
	ErrNotFound            = errors.New("not found")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrBadRequest          = errors.New("bad request")
	ErrUnprocessableEntity = errors.New("unprocessable entity")
)

// Result is transport-agnostic processing outcome; router will translate to HTTP
type Result struct {
	Status  int
	Payload any
}

// Orchestrator coordinates verification, idempotency, filtering, rendering and dispatch.
// It is safe for concurrent use provided its dependencies are thread-safe.
// Notes:
//   - The provided request body is fully consumed during processing.
//     Upstream routers must not attempt to re-read the body without buffering.
type Orchestrator struct {
	reg             Lookup
	verifierFactory func(VerifyConfig) (Verifier, error)
	idem            Service
	renderer        *TemplateRenderer
	filter          *CELAdapter
	disp            services.SignalDispatcher
	maxBody         int64
	dedupeTTL       time.Duration
	metrics         *Metrics
}

// NewOrchestrator creates a new orchestrator with provided dependencies
func NewOrchestrator(
	cfg *config.Config,
	reg Lookup,
	filter *CELAdapter,
	disp services.SignalDispatcher,
	idem Service,
	maxBody int64,
	dedupeTTL time.Duration,
) *Orchestrator {
	o := &Orchestrator{
		reg:       reg,
		filter:    filter,
		disp:      disp,
		idem:      idem,
		maxBody:   maxBody,
		dedupeTTL: dedupeTTL,
	}
	// Ensure Stripe default skew and other verifier defaults use application cfg
	o.verifierFactory = func(v VerifyConfig) (Verifier, error) {
		if v.Strategy == StrategyStripe && v.Skew == 0 {
			v.Skew = cfg.Webhooks.StripeSkew
		}
		return NewVerifier(v)
	}
	o.renderer = NewTemplateRenderer()
	if o.maxBody <= 0 {
		o.maxBody = cfg.Webhooks.DefaultMaxBody
	}
	if o.dedupeTTL <= 0 {
		o.dedupeTTL = cfg.Webhooks.DefaultDedupeTTL
	}
	return o
}

// SetMetrics attaches metrics instrumentation
func (o *Orchestrator) SetMetrics(m *Metrics) {
	o.metrics = m
}

// Process executes the webhook pipeline for a given slug and request
func (o *Orchestrator) Process(ctx context.Context, slug string, r *http.Request) (Result, error) {
	start := time.Now()
	corrID := requestCorrelationID(r)
	entry, ok := o.reg.Get(slug)
	workflowID := ""
	if ok {
		workflowID = entry.WorkflowID
	}
	if o.metrics != nil {
		o.metrics.OnReceived(ctx, slug, workflowID)
	}
	if !ok {
		logger.FromContext(ctx).Warn("webhook slug not found", "slug", slug, "correlation_id", corrID)
		if o.metrics != nil {
			o.metrics.ObserveOverall(ctx, slug, "", time.Since(start))
		}
		return Result{Status: http.StatusNotFound}, ErrNotFound
	}
	logger.FromContext(ctx).
		Info(
			"webhook request received",
			"slug",
			slug,
			"workflow_id",
			entry.WorkflowID,
			"correlation_id",
			corrID,
		)
	body, rres, rerr := o.readBody(ctx, r)
	if rerr != nil {
		if o.metrics != nil {
			o.metrics.OnFailed(ctx, slug, entry.WorkflowID, "bad_request")
			o.metrics.ObserveOverall(ctx, slug, entry.WorkflowID, time.Since(start))
		}
		return rres, rerr
	}
	vres, verr := o.verifyWithMetrics(ctx, entry, r, body, slug)
	if verr != nil {
		if o.metrics != nil {
			o.metrics.ObserveOverall(ctx, slug, entry.WorkflowID, time.Since(start))
		}
		return vres, verr
	}
	ires, ierr := o.checkIdempotency(ctx, entry, slug, r, body)
	if ierr != nil {
		if errors.Is(ierr, ErrDuplicate) && o.metrics != nil {
			o.metrics.OnDuplicate(ctx, slug, entry.WorkflowID)
			o.metrics.OnFailed(ctx, slug, entry.WorkflowID, "duplicate")
			o.metrics.ObserveOverall(ctx, slug, entry.WorkflowID, time.Since(start))
		}
		return ires, ierr
	}
	res, err := o.processEventsWithMetrics(ctx, entry, r, body, slug, corrID)
	if o.metrics != nil {
		o.metrics.ObserveOverall(ctx, slug, entry.WorkflowID, time.Since(start))
	}
	return res, err
}

func requestCorrelationID(r *http.Request) string {
	if v := r.Header.Get("X-Correlation-ID"); v != "" {
		return v
	}
	if v := r.Header.Get("X-Request-ID"); v != "" {
		return v
	}
	return core.MustNewID().String()
}

func (o *Orchestrator) readBody(ctx context.Context, r *http.Request) ([]byte, Result, error) {
	log := logger.FromContext(ctx)
	b, err := ReadRawJSON(r.Body, o.maxBody)
	if err != nil {
		log.Warn("invalid webhook body", "error", err)
		return nil, Result{Status: http.StatusBadRequest}, ErrBadRequest
	}
	return b, Result{}, nil
}

func (o *Orchestrator) verify(
	ctx context.Context,
	entry RegistryEntry,
	r *http.Request,
	body []byte,
) (Result, error) {
	log := logger.FromContext(ctx)
	if entry.Webhook != nil && entry.Webhook.Verify != nil && entry.Webhook.Verify.Strategy != StrategyNone {
		v, err := o.verifierFactory(entry.Webhook.Verify.ToVerifyConfig())
		if err != nil {
			log.Error("verifier init failed", "error", err)
			return Result{Status: http.StatusInternalServerError}, err
		}
		if err = v.Verify(ctx, r, body); err != nil {
			log.Warn("signature verification failed", "error", err)
			return Result{Status: http.StatusUnauthorized}, ErrUnauthorized
		}
	}
	return Result{}, nil
}

func (o *Orchestrator) verifyWithMetrics(
	ctx context.Context,
	entry RegistryEntry,
	r *http.Request,
	body []byte,
	slug string,
) (Result, error) {
	start := time.Now()
	res, err := o.verify(ctx, entry, r, body)
	if o.metrics != nil && entry.Webhook != nil && entry.Webhook.Verify != nil &&
		entry.Webhook.Verify.Strategy != StrategyNone {
		o.metrics.ObserveVerify(ctx, slug, entry.WorkflowID, time.Since(start))
		if err == nil {
			o.metrics.OnVerified(ctx, slug, entry.WorkflowID)
		} else {
			o.metrics.OnFailed(ctx, slug, entry.WorkflowID, "verification_failed")
		}
	}
	return res, err
}

func (o *Orchestrator) checkIdempotency(
	ctx context.Context,
	entry RegistryEntry,
	slug string,
	r *http.Request,
	body []byte,
) (Result, error) {
	log := logger.FromContext(ctx)
	if o.idem == nil {
		return Result{}, nil
	}
	var jsonField string
	var enabled = true
	var ttl = o.dedupeTTL
	if entry.Webhook != nil && entry.Webhook.Dedupe != nil {
		if entry.Webhook.Dedupe.TTL != "" {
			if d, err := time.ParseDuration(entry.Webhook.Dedupe.TTL); err == nil {
				ttl = d
			}
		}
		jsonField = entry.Webhook.Dedupe.Key
		enabled = entry.Webhook.Dedupe.Enabled
	}
	if !enabled {
		return Result{}, nil
	}
	key, kerr := DeriveKey(r.Header, body, jsonField)
	if kerr != nil || key == "" {
		return Result{}, nil
	}
	ns := KeyWithNamespace(entry.Webhook.Slug, key)
	if err := o.idem.CheckAndSet(ctx, ns, ttl); err != nil {
		if errors.Is(err, ErrDuplicate) {
			log.Info("duplicate webhook request", "slug", slug, "key", key)
			return Result{Status: http.StatusConflict}, ErrDuplicate
		}
		log.Error("idempotency check failed", "error", err)
		return Result{Status: http.StatusInternalServerError}, err
	}
	return Result{}, nil
}

func (o *Orchestrator) processEventsWithMetrics(
	ctx context.Context,
	entry RegistryEntry,
	r *http.Request,
	body []byte,
	slug string,
	corrID string,
) (Result, error) {
	payload, rres, rerr := o.parsePayload(ctx, slug, entry.WorkflowID, corrID, body)
	if rerr != nil {
		return rres, rerr
	}
	ctxData := BuildContext(payload, r.Header, r.URL.Query())
	// Add a reasonable timeout for event processing
	eventCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, ev := range entry.Webhook.Events {
		if err := eventCtx.Err(); err != nil {
			return Result{Status: http.StatusRequestTimeout}, err
		}
		allow, rres, rerr := o.allowEvent(eventCtx, slug, entry.WorkflowID, corrID, ev, ctxData)
		if rerr != nil {
			return rres, rerr
		}
		if !allow {
			continue
		}
		rendered, rres, rerr := o.renderEvent(eventCtx, slug, entry.WorkflowID, corrID, ev, payload)
		if rerr != nil {
			return rres, rerr
		}
		rres2, rerr2 := o.validateEvent(eventCtx, slug, entry.WorkflowID, corrID, ev, rendered)
		if rerr2 != nil {
			return rres2, rerr2
		}
		return o.dispatchEvent(eventCtx, slug, entry.WorkflowID, corrID, ev, rendered)
	}
	if o.metrics != nil {
		o.metrics.OnNoMatch(ctx, slug, entry.WorkflowID)
		o.metrics.OnFailed(ctx, slug, entry.WorkflowID, "no_match")
	}
	logger.FromContext(ctx).Info(
		"webhook no matching event",
		"slug", slug,
		"workflow_id", entry.WorkflowID,
		"correlation_id", corrID,
	)
	return Result{Status: http.StatusNoContent, Payload: map[string]any{"status": "no_matching_event"}}, nil
}

func (o *Orchestrator) parsePayload(
	ctx context.Context,
	slug string,
	workflowID string,
	corrID string,
	body []byte,
) (map[string]any, Result, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		logger.FromContext(ctx).Warn(
			"failed to parse json",
			"error", err,
			"slug", slug,
			"workflow_id", workflowID,
			"correlation_id", corrID,
		)
		if o.metrics != nil {
			o.metrics.OnFailed(ctx, slug, workflowID, "bad_request")
		}
		return nil, Result{Status: http.StatusBadRequest}, ErrBadRequest
	}
	return payload, Result{}, nil
}

func (o *Orchestrator) allowEvent(
	ctx context.Context,
	slug string,
	workflowID string,
	corrID string,
	ev EventConfig,
	ctxData map[string]any,
) (bool, Result, error) {
	allow, ferr := o.filter.Allow(ctx, ev.Filter, ctxData)
	if ferr != nil {
		logger.FromContext(ctx).Warn(
			"filter evaluation failed",
			"error", ferr,
			"event", ev.Name,
			"slug", slug,
			"workflow_id", workflowID,
			"correlation_id", corrID,
		)
		if o.metrics != nil {
			o.metrics.OnFailed(ctx, slug, workflowID, "bad_request")
		}
		return false, Result{Status: http.StatusBadRequest}, ErrBadRequest
	}
	return allow, Result{}, nil
}

func (o *Orchestrator) renderEvent(
	ctx context.Context,
	slug string,
	workflowID string,
	corrID string,
	ev EventConfig,
	payload map[string]any,
) (map[string]any, Result, error) {
	start := time.Now()
	rendered, merr := o.renderer.RenderTemplate(ctx, RenderContext{Payload: payload}, ev.Input)
	if o.metrics != nil {
		o.metrics.ObserveRender(ctx, slug, workflowID, ev.Name, time.Since(start))
	}
	if merr != nil {
		logger.FromContext(ctx).Warn(
			"render failed",
			"error", merr,
			"event", ev.Name,
			"slug", slug,
			"workflow_id", workflowID,
			"correlation_id", corrID,
		)
		if o.metrics != nil {
			o.metrics.OnFailed(ctx, slug, workflowID, "render_error")
		}
		return nil, Result{Status: http.StatusBadRequest}, ErrBadRequest
	}
	return rendered, Result{}, nil
}

func (o *Orchestrator) validateEvent(
	ctx context.Context,
	slug string,
	workflowID string,
	corrID string,
	ev EventConfig,
	rendered map[string]any,
) (Result, error) {
	if verr := ValidateTemplate(ctx, rendered, ev.Schema); verr != nil {
		logger.FromContext(ctx).Warn(
			"schema validation failed",
			"error", verr,
			"event", ev.Name,
			"slug", slug,
			"workflow_id", workflowID,
			"correlation_id", corrID,
		)
		if o.metrics != nil {
			o.metrics.OnFailed(ctx, slug, workflowID, "schema_error")
		}
		return Result{Status: http.StatusUnprocessableEntity}, ErrUnprocessableEntity
	}
	return Result{}, nil
}

func (o *Orchestrator) dispatchEvent(
	ctx context.Context,
	slug string,
	workflowID string,
	corrID string,
	ev EventConfig,
	rendered map[string]any,
) (Result, error) {
	start := time.Now()
	if err := o.disp.DispatchSignal(ctx, ev.Name, rendered, corrID); err != nil {
		logger.FromContext(ctx).Error(
			"dispatch failed",
			"error", err,
			"event", ev.Name,
			"slug", slug,
			"workflow_id", workflowID,
			"correlation_id", corrID,
		)
		if o.metrics != nil {
			o.metrics.OnFailed(ctx, slug, workflowID, "dispatch_error")
		}
		return Result{Status: http.StatusInternalServerError}, err
	}
	if o.metrics != nil {
		o.metrics.ObserveDispatch(ctx, slug, workflowID, ev.Name, time.Since(start))
		o.metrics.OnDispatched(ctx, slug, workflowID, ev.Name)
	}
	logger.FromContext(ctx).Info(
		"webhook event dispatched",
		"event", ev.Name,
		"slug", slug,
		"workflow_id", workflowID,
		"correlation_id", corrID,
	)
	return Result{Status: http.StatusAccepted, Payload: map[string]any{"status": "accepted", "event": ev.Name}}, nil
}
