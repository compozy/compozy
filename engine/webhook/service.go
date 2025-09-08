package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task/services"
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
	render          func(context.Context, RenderContext, map[string]string) (map[string]any, error)
	filter          *CELAdapter
	disp            services.SignalDispatcher
	maxBody         int64
	dedupeTTL       time.Duration
}

// NewOrchestrator creates a new orchestrator with provided dependencies
func NewOrchestrator(
	reg Lookup,
	filter *CELAdapter,
	disp services.SignalDispatcher,
	idem Service,
	maxBody int64,
	dedupeTTL time.Duration,
) *Orchestrator {
	o := &Orchestrator{reg: reg, filter: filter, disp: disp, idem: idem, maxBody: maxBody, dedupeTTL: dedupeTTL}
	o.verifierFactory = NewVerifier
	o.render = RenderTemplate
	if o.maxBody <= 0 {
		o.maxBody = 1 << 20
	}
	if o.dedupeTTL <= 0 {
		o.dedupeTTL = 10 * time.Minute
	}
	return o
}

// Process executes the webhook pipeline for a given slug and request
func (o *Orchestrator) Process(ctx context.Context, slug string, r *http.Request) (Result, error) {
	entry, ok := o.reg.Get(slug)
	if !ok {
		return Result{Status: http.StatusNotFound}, ErrNotFound
	}
	body, rres, rerr := o.readBody(ctx, r)
	if rerr != nil {
		return rres, rerr
	}
	vres, verr := o.verify(ctx, entry, r, body)
	if verr != nil {
		return vres, verr
	}
	ires, ierr := o.checkIdempotency(ctx, entry, slug, r, body)
	if ierr != nil {
		return ires, ierr
	}
	return o.processEvents(ctx, entry, r, body)
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

func (o *Orchestrator) verify(ctx context.Context, entry RegistryEntry, r *http.Request, body []byte) (Result, error) {
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

func (o *Orchestrator) processEvents(
	ctx context.Context,
	entry RegistryEntry,
	r *http.Request,
	body []byte,
) (Result, error) {
	log := logger.FromContext(ctx)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Warn("failed to parse json", "error", err)
		return Result{Status: http.StatusBadRequest}, ErrBadRequest
	}
	ctxData := BuildContext(payload, r.Header, r.URL.Query())
	for _, ev := range entry.Webhook.Events {
		if err := ctx.Err(); err != nil {
			log.Warn("context canceled", "error", err)
			return Result{Status: http.StatusRequestTimeout}, err
		}
		allow, ferr := o.filter.Allow(ctx, ev.Filter, ctxData)
		if ferr != nil {
			log.Warn("filter evaluation failed", "error", ferr, "event", ev.Name)
			return Result{Status: http.StatusBadRequest}, ErrBadRequest
		}
		if !allow {
			continue
		}
		rendered, merr := o.render(ctx, RenderContext{Payload: payload}, ev.Input)
		if merr != nil {
			log.Warn("render failed", "error", merr, "event", ev.Name)
			return Result{Status: http.StatusBadRequest}, ErrBadRequest
		}
		if verr := ValidateTemplate(ctx, rendered, ev.Schema); verr != nil {
			log.Warn("schema validation failed", "error", verr, "event", ev.Name)
			return Result{Status: http.StatusUnprocessableEntity}, ErrUnprocessableEntity
		}
		corrID := requestCorrelationID(r)
		if err := o.disp.DispatchSignal(ctx, ev.Name, rendered, corrID); err != nil {
			log.Error("dispatch failed", "error", err, "event", ev.Name)
			return Result{Status: http.StatusInternalServerError}, err
		}
		return Result{Status: http.StatusAccepted, Payload: map[string]any{"status": "accepted", "event": ev.Name}}, nil
	}
	return Result{Status: http.StatusNoContent, Payload: map[string]any{"status": "no_matching_event"}}, nil
}
