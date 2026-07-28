package contract_test

func resolvedChannelFromJSON(got map[string]any) string {
	raw, ok := got["resolved_network_participation"].(map[string]any)
	if !ok {
		return ""
	}
	v, _ := raw["channel_id"].(string)
	return v
}
