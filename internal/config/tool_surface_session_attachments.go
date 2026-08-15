package config

func sessionAttachmentsToolPathKinds() map[string]ValueKind {
	return map[string]ValueKind{
		"session.attachments.max_file_bytes":       ConfigValueInt64,
		"session.attachments.max_files_per_prompt": ConfigValueInt,
		"session.attachments.allowed_mime":         ConfigValueStringSlice,
		"session.attachments.retention.max_count":  ConfigValueInt,
		"session.attachments.retention.max_bytes":  ConfigValueInt64,
		"session.attachments.retention.max_age":    ConfigValueDuration,
	}
}
