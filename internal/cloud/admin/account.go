// SPDX-License-Identifier: FSL-1.1-MIT

package admin

import (
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
)

type accountView struct {
	Profile struct {
		ID             string `json:"id"`
		Email          string `json:"email"`
		Name           string `json:"name"`
		State          string `json:"state"`
		CloudRoutesMax int32  `json:"cloud_routes_max"`
	} `json:"account"`
	Consent struct {
		OptIn    bool       `json:"marketing_opt_in"`
		Source   string     `json:"marketing_consent_source"`
		At       *time.Time `json:"marketing_consent_at"`
		SyncedAt *time.Time `json:"contact_synced_at"`
		RetryAt  *time.Time `json:"contact_retry_at"`
	} `json:"consent"`
	Identities []struct {
		Provider string `json:"provider"`
		Subject  string `json:"subject"`
		Email    string `json:"email"`
	} `json:"identities"`
	Installations []struct {
		ID       string     `json:"id"`
		Source   string     `json:"source"`
		LastSeen *time.Time `json:"last_seen_at"`
	} `json:"installations"`
	Devices []struct {
		ID        string     `json:"id"`
		Name      string     `json:"name"`
		Kind      string     `json:"kind"`
		Admin     bool       `json:"admin"`
		LastSeen  *time.Time `json:"last_seen_at"`
		RevokedAt *time.Time `json:"revoked_at"`
	} `json:"devices"`
	Sessions []struct {
		ID        string     `json:"id"`
		Provider  string     `json:"provider"`
		DeviceID  string     `json:"device_id"`
		LastUsed  *time.Time `json:"last_used_at"`
		RevokedAt *time.Time `json:"revoked_at"`
	} `json:"device_sessions"`
	Computers []struct {
		ID            string     `json:"id"`
		Name          string     `json:"name"`
		Platform      string     `json:"platform"`
		Version       string     `json:"version"`
		RemoteEnabled bool       `json:"remote_enabled"`
		LastControl   *time.Time `json:"last_control_at"`
		UnlinkedAt    *time.Time `json:"unlinked_at"`
	} `json:"computers"`
	Tunnels []struct {
		Hostname   string     `json:"hostname"`
		State      string     `json:"state"`
		Generation int64      `json:"generation"`
		TakenDown  *time.Time `json:"admin_takedown_at"`
	} `json:"tunnels"`
	Subscriptions []struct {
		ID        string     `json:"id"`
		Source    string     `json:"source"`
		Status    string     `json:"status"`
		Interval  string     `json:"interval"`
		Customer  string     `json:"stripe_customer_id"`
		EndsAt    *time.Time `json:"ends_at"`
		PeriodEnd *time.Time `json:"current_period_end"`
	} `json:"subscriptions"`
	Events []store.AdminEvent `json:"security_events"`
}
