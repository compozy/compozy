package settings

import (
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

type gatewaySettingField struct {
	path    []string
	changed bool
	value   any
}

func gatewaySettingFields(
	current compozyconfig.GatewayConfig,
	desired compozyconfig.GatewayConfig,
) []gatewaySettingField {
	return []gatewaySettingField{
		{
			path: []string{sectionsEnabledKey}, changed: current.Enabled != desired.Enabled,
			value: desired.Enabled,
		},
		{
			path: []string{"private_port"}, changed: current.PrivatePort != desired.PrivatePort,
			value: desired.PrivatePort,
		},
		{
			path: []string{"public_port"}, changed: current.PublicPort != desired.PublicPort,
			value: desired.PublicPort,
		},
		{
			path: []string{"pairing", "ttl"}, changed: current.Pairing.TTL != desired.Pairing.TTL,
			value: desired.Pairing.TTL.String(),
		},
		{
			path:    []string{"pairing", "max_pending"},
			changed: current.Pairing.MaxPending != desired.Pairing.MaxPending,
			value:   desired.Pairing.MaxPending,
		},
		{
			path:    []string{"stream_ticket", "ttl"},
			changed: current.StreamTicket.TTL != desired.StreamTicket.TTL,
			value:   desired.StreamTicket.TTL.String(),
		},
		{
			path:    []string{"auth", "rate_limit", "window"},
			changed: current.Auth.RateLimit.Window != desired.Auth.RateLimit.Window,
			value:   desired.Auth.RateLimit.Window.String(),
		},
		{
			path:    []string{"auth", "rate_limit", "max_fails"},
			changed: current.Auth.RateLimit.MaxFails != desired.Auth.RateLimit.MaxFails,
			value:   desired.Auth.RateLimit.MaxFails,
		},
		{
			path: []string{"verify", "timeout"}, changed: current.Verify.Timeout != desired.Verify.Timeout,
			value: desired.Verify.Timeout.String(),
		},
	}
}

func diffGatewaySettings(current compozyconfig.GatewayConfig, desired compozyconfig.GatewayConfig) []string {
	changed := make([]string, 0, len(gatewaySettingFields(current, desired)))
	for _, field := range gatewaySettingFields(current, desired) {
		if field.changed {
			changed = append(changed, "gateway."+strings.Join(field.path, "."))
		}
	}
	return changed
}

func applyGatewaySettings(editor *compozyconfig.OverlayEditor, settings compozyconfig.GatewayConfig) error {
	path := func(parts ...string) []string {
		return append([]string{string(SectionGateway)}, parts...)
	}
	fields := gatewaySettingFields(settings, settings)
	updates := make([]struct {
		path  []string
		value any
	}, 0, len(fields))
	for _, field := range fields {
		updates = append(updates, struct {
			path  []string
			value any
		}{path: path(field.path...), value: field.value})
	}
	return applyValueUpdates(editor, updates)
}
