package config

const (
	sessionAttachmentsMaxFileBytesPath  = "session.attachments.max_file_bytes"
	sessionAttachmentsMaxFilesPath      = "session.attachments.max_files_per_prompt"
	sessionAttachmentsAllowedMIMEPath   = "session.attachments.allowed_mime"
	sessionAttachmentsRetentionCountKey = "session.attachments.retention.max_count"
)

func sessionAttachmentsToolPathKinds() map[string]ValueKind {
	return map[string]ValueKind{
		sessionAttachmentsMaxFileBytesPath:        ConfigValueInt64,
		sessionAttachmentsMaxFilesPath:            ConfigValueInt,
		sessionAttachmentsAllowedMIMEPath:         ConfigValueStringSlice,
		sessionAttachmentsRetentionCountKey:       ConfigValueInt,
		"session.attachments.retention.max_bytes": ConfigValueInt64,
		"session.attachments.retention.max_age":   ConfigValueDuration,
	}
}
