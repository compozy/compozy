// SPDX-License-Identifier: FSL-1.1-MIT

package cloudflare

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type ManagedTunnel struct {
	Hostname   string
	Generation int64
	ID         string
}

func (c *Client) ManagedTunnels(ctx context.Context) ([]ManagedTunnel, error) {
	const pageSize = 100
	var result []ManagedTunnel
	for page := 1; ; page++ {
		params := url.Values{"is_deleted": {"false"}, "per_page": {"100"}, "page": {strconv.Itoa(page)}}
		var tunnels []tunnel
		if err := c.request(ctx, http.MethodGet, c.tunnels+"?"+params.Encode(), nil, &tunnels); err != nil {
			return nil, err
		}
		for _, remote := range tunnels {
			if managed, ok := c.managedTunnel(remote); ok {
				result = append(result, managed)
			}
		}
		if len(tunnels) < pageSize {
			return result, nil
		}
	}
}

func (c *Client) managedTunnel(remote tunnel) (ManagedTunnel, bool) {
	prefix := "compozy-prod-"
	if c.domain == StagingDomain {
		prefix = "compozy-staging-"
	}
	name, prefixed := strings.CutPrefix(remote.Name, prefix)
	label, generationText, split := strings.CutLast(name, "-")
	generation, err := strconv.ParseInt(generationText, 10, 64)
	if !prefixed || !split || err != nil || remote.DeletedAt != nil || !tunnelIDPattern.MatchString(remote.ID) {
		return ManagedTunnel{}, false
	}
	hostname := label + "." + c.domain
	roundTrip, err := c.tunnelName(hostname, generation)
	if err != nil || roundTrip != remote.Name {
		return ManagedTunnel{}, false
	}
	return ManagedTunnel{Hostname: hostname, Generation: generation, ID: remote.ID}, true
}

func (c *Client) DeleteRestoreOrphan(ctx context.Context, remote ManagedTunnel) error {
	if remote.ID == "" {
		return errors.New("cloud restore orphan requires a tunnel ID")
	}
	return c.deleteTunnel(ctx, remote.Hostname, remote.Generation, remote.ID, true)
}
