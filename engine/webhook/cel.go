package webhook

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/engine/task"
)

// CELAdapter evaluates CEL filter expressions for webhook events.
// Context contract for evaluation: { payload: map[string]any }
// The context builder also supports headers and query for future use.
type CELAdapter struct {
	eval *task.CELEvaluator
}

// NewCELAdapter creates a new CELAdapter. Preferred constructor.
func NewCELAdapter(eval *task.CELEvaluator) *CELAdapter {
	return &CELAdapter{eval: eval}
}

// Allow returns true when the CEL expression evaluates to true. Empty expressions allow by default.
func (a *CELAdapter) Allow(ctx context.Context, expr string, data map[string]any) (bool, error) {
	if a == nil || a.eval == nil {
		return false, fmt.Errorf("cel adapter not initialized")
	}
	if expr == "" {
		return true, nil
	}
	return a.eval.Evaluate(ctx, expr, data)
}

const (
	ctxKeyPayload = "payload"
	ctxKeyHeaders = "headers"
	ctxKeyQuery   = "query"
)

// BuildContext builds the evaluation context map for webhook filters.
// Primary contract exposes only payload; headers and query are provided for extensibility.
func BuildContext(payload map[string]any, headers http.Header, query url.Values) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	ctx := map[string]any{ctxKeyPayload: payload}
	if headers != nil {
		ctx[ctxKeyHeaders] = headers
	}
	if query != nil {
		ctx[ctxKeyQuery] = query
	}
	return ctx
}
