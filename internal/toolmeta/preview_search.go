package toolmeta

func searchPreview(input map[string]any) string {
	for _, key := range []string{previewHintQuery, "needle", "pattern"} {
		if value := scalarString(input[key]); value != "" {
			return value
		}
	}
	return ""
}
