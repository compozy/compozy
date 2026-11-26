// SPDX-License-Identifier: FSL-1.1-MIT

package main

import (
	"strings"

	"github.com/compozy/compozy/internal/cloud/admin"
)

func loadAdminAccess(getenv func(string) string) *admin.AccessConfig {
	team := getenv("COMPOZY_CLOUD_ACCESS_TEAM_DOMAIN")
	audience := getenv("COMPOZY_CLOUD_ACCESS_AUD")
	operators := getenv("COMPOZY_CLOUD_ADMIN_ALLOWLIST")
	if team == "" && audience == "" && operators == "" {
		return nil
	}
	return &admin.AccessConfig{TeamDomain: team, Audience: audience, Allowlist: strings.Split(operators, ",")}
}
