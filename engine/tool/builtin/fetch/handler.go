package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
)

var inputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"url"},
	"properties": map[string]any{
		"url": map[string]any{
			"type":        "string",
			"format":      "uri",
			"description": "Destination URL (http or https).",
		},
		"method": map[string]any{
			"type":        "string",
			"description": "HTTP method (default GET).",
		},
		"headers": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{"type": "string"},
			"description":          "Optional headers to include in the request.",
		},
		"body": map[string]any{
			"description": "Optional request body. Strings sent raw, objects JSON-encoded.",
		},
		"timeout_ms": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Optional timeout override in milliseconds.",
		},
	},
}

var outputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"status_code", "status_text", "headers", "body", "duration_ms"},
	"properties": map[string]any{
		"status_code": map[string]any{
			"type":        "integer",
			"description": "HTTP status code returned by the server.",
		},
		"status_text": map[string]any{"type": "string"},
		"headers": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{"type": "string"},
		},
		"body":           map[string]any{"type": "string"},
		"body_truncated": map[string]any{"type": "boolean"},
		"duration_ms":    map[string]any{"type": "integer"},
	},
}

func Definition() builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:            toolID,
		Description:   "Perform an HTTP request with strict safety limits.",
		InputSchema:   inputSchema,
		OutputSchema:  outputSchema,
		ArgsPrototype: Args{},
		Handler:       executeHandler,
	}
}

func executeHandler(ctx context.Context, payload map[string]any) (core.Output, error) {
	start := time.Now()
	status := builtin.StatusFailure
	responseBytes := 0
	cfg := loadToolConfig(ctx)
	defer func() {
		builtin.RecordInvocation(
			ctx,
			toolID,
			builtin.RequestIDFromContext(ctx),
			status,
			time.Since(start),
			responseBytes,
			"",
		)
	}()
	args, err := decodeArgs(payload)
	if err != nil {
		return nil, builtin.InvalidArgument(err, nil)
	}
	output, err := performRequest(ctx, cfg, args)
	if err != nil {
		return nil, err
	}
	switch body := output["body"].(type) {
	case string:
		responseBytes = len(body)
	case []byte:
		responseBytes = len(body)
	}
	if code, ok := output["status_code"].(int); ok && code < http.StatusBadRequest {
		status = builtin.StatusSuccess
	}
	return output, nil
}

func performRequest(ctx context.Context, cfg toolConfig, args Args) (core.Output, error) {
	method := normalizeMethod(args.Method)
	if err := ensureMethodAllowed(method, cfg); err != nil {
		return nil, err
	}
	if err := ensureBodyAllowed(method, args.Body); err != nil {
		return nil, err
	}
	target, err := parseTargetURL(args.URL)
	if err != nil {
		return nil, err
	}
	bodyReader, contentType, err := buildRequestBody(args.Body)
	if err != nil {
		return nil, builtin.InvalidArgument(err, nil)
	}
	effectiveTimeout := determineRequestTimeout(cfg.Timeout, args.TimeoutMs)
	reqCtx, cancel := buildRequestContext(ctx, effectiveTimeout)
	if cancel != nil {
		defer cancel()
	}
	req, err := http.NewRequestWithContext(reqCtx, method, target.String(), bodyReader)
	if err != nil {
		return nil, builtin.Internal(fmt.Errorf("failed to build request: %w", err), nil)
	}
	applyHeaders(req, args.Headers, contentType)
	client := newHTTPClient(cfg, effectiveTimeout)
	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return nil, classifyRequestError(err)
	}
	defer resp.Body.Close()
	body, truncated, err := readBody(resp.Body, cfg.MaxBodyBytes)
	if err != nil {
		return nil, builtin.Internal(fmt.Errorf("failed to read response body: %w", err), nil)
	}
	headers := flattenHeaders(resp.Header)
	logFetch(ctx, method, target.String(), resp.StatusCode, duration, truncated)
	return core.Output{
		"status_code":    resp.StatusCode,
		"status_text":    resp.Status,
		"headers":        headers,
		"body":           body,
		"body_truncated": truncated,
		"duration_ms":    duration.Milliseconds(),
	}, nil
}

func normalizeMethod(method string) string {
	upper := strings.ToUpper(strings.TrimSpace(method))
	if upper == "" {
		return http.MethodGet
	}
	return upper
}

func ensureMethodAllowed(method string, cfg toolConfig) error {
	if _, ok := cfg.AllowedMethods[method]; ok {
		return nil
	}
	details := map[string]any{"method": method}
	return builtin.InvalidArgument(fmt.Errorf("method %s is not allowed", method), details)
}

func ensureBodyAllowed(method string, body any) error {
	if (method == http.MethodGet || method == http.MethodHead) && body != nil {
		details := map[string]any{"method": method}
		return builtin.InvalidArgument(errors.New("request body not permitted for this method"), details)
	}
	return nil
}

func parseTargetURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		details := map[string]any{"field": "url"}
		return nil, builtin.InvalidArgument(errors.New("url must be provided"), details)
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		details := map[string]any{"url": raw}
		return nil, builtin.InvalidArgument(fmt.Errorf("invalid url: %w", err), details)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		details := map[string]any{"scheme": parsed.Scheme}
		return nil, builtin.InvalidArgument(errors.New("only http and https schemes are supported"), details)
	}
	return parsed, nil
}

func determineRequestTimeout(base time.Duration, overrideMs int) time.Duration {
	effective := base
	if overrideMs > 0 {
		override := time.Duration(overrideMs) * time.Millisecond
		if effective <= 0 || override < effective {
			effective = override
		}
	}
	if effective <= 0 {
		effective = base
	}
	if effective <= 0 {
		effective = defaultTimeout
	}
	return effective
}

func buildRequestContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, nil
	}
	return context.WithTimeout(ctx, timeout)
}

func applyHeaders(req *http.Request, headers map[string]string, contentType string) {
	for key, value := range headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}
}

func newHTTPClient(cfg toolConfig, requestTimeout time.Duration) *http.Client {
	effective := requestTimeout
	if effective <= 0 {
		effective = cfg.Timeout
	}
	if effective <= 0 {
		effective = defaultTimeout
	}
	client := &http.Client{Transport: http.DefaultTransport, Timeout: effective}
	if cfg.MaxRedirects > 0 {
		client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
			if len(via) >= cfg.MaxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		}
	}
	return client
}

func logFetch(
	ctx context.Context,
	method, url string,
	status int,
	duration time.Duration,
	truncated bool,
) {
	logger.FromContext(ctx).Info(
		"Executed cp__fetch request",
		"tool_id", toolID,
		"request_id", builtin.RequestIDFromContext(ctx),
		"method", method,
		"url", url,
		"status_code", status,
		"duration_ms", duration.Milliseconds(),
		"body_truncated", truncated,
	)
}

func buildRequestBody(body any) (io.Reader, string, error) {
	if body == nil {
		return nil, "", nil
	}
	switch value := body.(type) {
	case string:
		return strings.NewReader(value), "", nil
	case json.RawMessage:
		return bytes.NewReader(value), "application/json", nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, "", fmt.Errorf("failed to encode request body: %w", err)
		}
		return bytes.NewReader(encoded), "application/json", nil
	}
}

func readBody(r io.Reader, limit int64) (string, bool, error) {
	if limit <= 0 {
		data, err := io.ReadAll(r)
		if err != nil {
			return "", false, err
		}
		return string(data), false, nil
	}
	limited := io.LimitReader(r, limit)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", false, err
	}
	extra := make([]byte, 1)
	n, extraErr := r.Read(extra)
	if extraErr != nil && !errors.Is(extraErr, io.EOF) {
		return "", false, extraErr
	}
	truncated := n > 0 || (errors.Is(extraErr, io.EOF) && n > 0)
	return string(data), truncated, nil
}

func flattenHeaders(header http.Header) map[string]string {
	result := make(map[string]string, len(header))
	for key, values := range header {
		result[key] = strings.Join(values, ", ")
	}
	return result
}

func classifyRequestError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return builtin.Internal(fmt.Errorf("request timeout: %w", err), map[string]any{"timeout": true})
		}
		return builtin.Internal(fmt.Errorf("http request failed: %w", err), map[string]any{"op": urlErr.Op})
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return builtin.Internal(errors.New("request timeout"), map[string]any{"timeout": true})
	}
	return builtin.Internal(fmt.Errorf("http request failed: %w", err), nil)
}
