package cloudclient

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/remoteproto"
)

func (c *Client) Computers(ctx context.Context, creds Credentials) ([]remoteproto.AccountComputer, error) {
	var result struct {
		Computers []remoteproto.AccountComputer `json:"computers"`
	}
	err := c.deviceJSON(ctx, creds, http.MethodGet, "/v1/computers", nil, &result)
	return result.Computers, err
}

func (c *Client) ComputerGrant(
	ctx context.Context, creds Credentials, id remoteproto.ComputerID, route remoteproto.RouteKind,
) (string, error) {
	input := struct {
		Route remoteproto.RouteKind `json:"route"`
	}{Route: route}
	var result struct {
		Grant string `json:"grant"`
	}
	err := c.deviceJSON(ctx, creds, http.MethodPost, computerPath(id)+"/grants", input, &result)
	return result.Grant, err
}
