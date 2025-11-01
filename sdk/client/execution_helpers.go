package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type executionRequestConfig[T any] struct {
	ResourceID    string
	ResourceLabel string
	RouteBase     string
	PathSuffix    string
	Body          any
	ExpectedCode  int
	OnSuccess     func(context.Context, string, *T)
}

func executeRequest[T any](ctx context.Context, client *Client, cfg executionRequestConfig[T]) (*T, error) {
	id := strings.TrimSpace(cfg.ResourceID)
	if id == "" {
		return nil, fmt.Errorf("%s id is required", cfg.ResourceLabel)
	}
	path := fmt.Sprintf("%s/%s%s", cfg.RouteBase, url.PathEscape(id), cfg.PathSuffix)
	resp, err := client.postJSON(ctx, path, cfg.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	result, err := decodeEnvelope[T](resp, cfg.ExpectedCode)
	if err != nil {
		return nil, err
	}
	if cfg.OnSuccess != nil {
		cfg.OnSuccess(ctx, id, &result)
	}
	return &result, nil
}
