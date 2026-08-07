package settings

import compozyconfig "github.com/compozy/compozy/internal/config"

func diffGatewaySettings(current compozyconfig.GatewayConfig, desired compozyconfig.GatewayConfig) []string {
	changed := make([]string, 0, 9)
	appendChange := func(path string, differs bool) {
		if differs {
			changed = append(changed, path)
		}
	}
	appendChange("gateway.enabled", current.Enabled != desired.Enabled)
	appendChange("gateway.private_port", current.PrivatePort != desired.PrivatePort)
	appendChange("gateway.public_port", current.PublicPort != desired.PublicPort)
	appendChange("gateway.pairing.ttl", current.Pairing.TTL != desired.Pairing.TTL)
	appendChange("gateway.pairing.max_pending", current.Pairing.MaxPending != desired.Pairing.MaxPending)
	appendChange("gateway.stream_ticket.ttl", current.StreamTicket.TTL != desired.StreamTicket.TTL)
	appendChange(
		"gateway.auth.rate_limit.window",
		current.Auth.RateLimit.Window != desired.Auth.RateLimit.Window,
	)
	appendChange(
		"gateway.auth.rate_limit.max_fails",
		current.Auth.RateLimit.MaxFails != desired.Auth.RateLimit.MaxFails,
	)
	appendChange("gateway.verify.timeout", current.Verify.Timeout != desired.Verify.Timeout)
	return changed
}

func applyGatewaySettings(editor *compozyconfig.OverlayEditor, settings compozyconfig.GatewayConfig) error {
	path := func(parts ...string) []string {
		return append([]string{string(SectionGateway)}, parts...)
	}
	updates := []struct {
		path  []string
		value any
	}{
		{path: path(sectionsEnabledKey), value: settings.Enabled},
		{path: path("private_port"), value: settings.PrivatePort},
		{path: path("public_port"), value: settings.PublicPort},
		{path: path("pairing", "ttl"), value: settings.Pairing.TTL.String()},
		{path: path("pairing", "max_pending"), value: settings.Pairing.MaxPending},
		{path: path("stream_ticket", "ttl"), value: settings.StreamTicket.TTL.String()},
		{path: path("auth", "rate_limit", "window"), value: settings.Auth.RateLimit.Window.String()},
		{path: path("auth", "rate_limit", "max_fails"), value: settings.Auth.RateLimit.MaxFails},
		{path: path("verify", "timeout"), value: settings.Verify.Timeout.String()},
	}
	return applyValueUpdates(editor, updates)
}
