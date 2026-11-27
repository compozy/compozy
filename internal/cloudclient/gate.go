package cloudclient

import (
	"context"
	"io"
	"net/http"
	"strings"
)

func (c *Client) GatePolicy(ctx context.Context) (string, error) {
	call, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	response, err := c.open(call, http.MethodGet, "/v1/gate-policy", nil, "", nil, false, "application/jose")
	if err != nil {
		if call.Err() != nil && ctx.Err() == nil {
			return "", unavailable()
		}
		return "", err
	}
	defer response.Body.Close()
	const maxPolicy = 16 << 10
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPolicy+1))
	if err != nil {
		return "", unavailable()
	}
	if len(body) > maxPolicy {
		return "", nil
	}
	return strings.TrimSpace(string(body)), nil
}
