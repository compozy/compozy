package config

import "time"

type sessionAttachmentsOverlay struct {
	MaxFileBytes      *int64                             `toml:"max_file_bytes"`
	MaxFilesPerPrompt *int                               `toml:"max_files_per_prompt"`
	AllowedMIME       *[]string                          `toml:"allowed_mime"`
	Retention         sessionAttachmentsRetentionOverlay `toml:"retention"`
}

type sessionAttachmentsRetentionOverlay struct {
	MaxCount *int           `toml:"max_count"`
	MaxBytes *int64         `toml:"max_bytes"`
	MaxAge   *time.Duration `toml:"max_age"`
}

func (o sessionAttachmentsOverlay) Apply(dst *SessionAttachmentsConfig) {
	if o.MaxFileBytes != nil {
		dst.MaxFileBytes = *o.MaxFileBytes
	}
	if o.MaxFilesPerPrompt != nil {
		dst.MaxFilesPerPrompt = *o.MaxFilesPerPrompt
	}
	if o.AllowedMIME != nil {
		dst.AllowedMIME = append([]string(nil), (*o.AllowedMIME)...)
	}
	o.Retention.Apply(&dst.Retention)
}

func (o sessionAttachmentsRetentionOverlay) Apply(dst *SessionAttachmentsRetentionConfig) {
	if o.MaxCount != nil {
		dst.MaxCount = *o.MaxCount
	}
	if o.MaxBytes != nil {
		dst.MaxBytes = *o.MaxBytes
	}
	if o.MaxAge != nil {
		dst.MaxAge = *o.MaxAge
	}
}
