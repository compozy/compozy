package github

import (
	"context"
	"encoding/json/v2"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"
)

type forgeSourceKey struct{}

func (c githubClient) logCall(
	ctx context.Context,
	method, path string,
	status int,
	header http.Header,
	payload []byte,
	err error,
) {
	logger := c.logger
	if logger == nil {
		logger = slog.Default()
	}
	source := forgeCallSource(ctx)
	api := "rest"
	if path == "/graphql" {
		api = "graphql"
	}
	remaining := -1
	if value, parseErr := strconv.Atoi(header.Get("X-RateLimit-Remaining")); parseErr == nil {
		remaining = value
	}
	var graph struct {
		Errors []struct {
			Type string `json:"type"`
		} `json:"errors"`
	}
	graphFailed, graphRateLimited := false, false
	if api == "graphql" && json.Unmarshal(payload, &graph) == nil {
		graphFailed = len(graph.Errors) > 0
		graphRateLimited = slices.ContainsFunc(graph.Errors, func(e struct {
			Type string `json:"type"`
		}) bool {
			return e.Type == graphErrorRateLimited
		})
	}
	attrs := []any{
		"source",
		source,
		"method",
		method,
		"api",
		api,
		"status",
		status,
		"remaining_quota",
		remaining,
		"secondary_limit",
		(status == http.StatusForbidden || status == http.StatusTooManyRequests) && remaining != 0 &&
			githubRateLimitSignalled(header, payload),
		"failed",
		err != nil || graphFailed,
	}
	if reset, parseErr := strconv.ParseInt(header.Get("X-RateLimit-Reset"), 10, 64); parseErr == nil {
		attrs = append(attrs, "reset_at", time.Unix(reset, 0).UTC())
	}
	if retry := githubRetryAfter(header, time.Now()); retry != nil {
		attrs = append(attrs, "retry_at", *retry)
	}
	if seconds, parseErr := strconv.Atoi(header.Get("Retry-After")); parseErr == nil {
		attrs = append(attrs, "retry_after_seconds", seconds)
	}
	if graphRateLimited {
		attrs = append(attrs, "graphql_rate_limited", true)
	}
	logger.InfoContext(ctx, "forge.call", attrs...)
}

func forgeCallSource(ctx context.Context) string {
	if source, ok := ctx.Value(forgeSourceKey{}).(string); ok && source != "" {
		return source
	}
	return "forge"
}
