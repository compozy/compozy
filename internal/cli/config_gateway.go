package cli

func gatewayConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		"gateway.enabled":                   configSetBool,
		"gateway.private_port":              configSetInt,
		"gateway.public_port":               configSetInt,
		"gateway.pairing.ttl":               configSetDuration,
		"gateway.pairing.max_pending":       configSetInt,
		"gateway.stream_ticket.ttl":         configSetDuration,
		"gateway.auth.rate_limit.window":    configSetDuration,
		"gateway.auth.rate_limit.max_fails": configSetInt,
		"gateway.verify.timeout":            configSetDuration,
	}
}
