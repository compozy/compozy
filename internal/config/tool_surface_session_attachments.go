package config

const (
	sessionAttachmentsMaxFileBytesPath      = "session.attachments.max_file_bytes"
	sessionAttachmentsMaxFilesPerPromptPath = "session.attachments.max_files_per_prompt"
	sessionAttachmentsAllowedMIMEPath       = "session.attachments.allowed_mime"
	sessionAttachmentsRetentionMaxCountPath = "session.attachments.retention.max_count"
	sessionAttachmentsRetentionMaxBytesPath = "session.attachments.retention.max_bytes"
	sessionAttachmentsRetentionMaxAgePath   = "session.attachments.retention.max_age"
)

func sessionAttachmentsToolPathKinds() map[string]ValueKind {
	return map[string]ValueKind{
		sessionAttachmentsMaxFileBytesPath:      ConfigValueInt64,
		sessionAttachmentsMaxFilesPerPromptPath: ConfigValueInt,
		sessionAttachmentsAllowedMIMEPath:       ConfigValueStringSlice,
		sessionAttachmentsRetentionMaxCountPath: ConfigValueInt,
		sessionAttachmentsRetentionMaxBytesPath: ConfigValueInt64,
		sessionAttachmentsRetentionMaxAgePath:   ConfigValueDuration,
	}
}
