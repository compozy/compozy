// SPDX-License-Identifier: FSL-1.1-MIT

package main

import (
	"github.com/compozy/compozy/internal/cloud/cloudflare"
)

func loadHostedTunnels(getenv func(string) string, environment string) (*cloudflare.Client, error) {
	domain := cloudflare.ProductionDomain
	if environment == staging {
		domain = cloudflare.StagingDomain
	}
	config := cloudflare.Config{
		Domain:    domain,
		AccountID: getenv("COMPOZY_CLOUD_CLOUDFLARE_ACCOUNT_ID"),
		ZoneID:    getenv("COMPOZY_CLOUD_CLOUDFLARE_RUN_ZONE_ID"),
		Token:     getenv("COMPOZY_CLOUD_CLOUDFLARE_TUNNEL_API_TOKEN"),
	}
	if environment == development {
		config.BaseURL = getenv("COMPOZY_CLOUD_CLOUDFLARE_API_ORIGIN")
	}
	if config.AccountID == "" && config.ZoneID == "" && config.Token == "" {
		return nil, nil
	}
	return cloudflare.New(config)
}
