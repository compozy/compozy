package contract

import "time"

type CreateSupportBundleRequest struct {
	Yes           bool  `json:"yes"`
	IncludeStatus *bool `json:"include_status,omitempty"`
}

type SupportBundleOperationResponse struct {
	Operation SupportBundleOperationPayload `json:"operation"`
}

type SupportBundleOperationPayload struct {
	OperationID   string                        `json:"operation_id"`
	Status        string                        `json:"status"`
	StatusURL     string                        `json:"status_url"`
	DownloadURL   string                        `json:"download_url,omitempty"`
	FileName      string                        `json:"file_name,omitempty"`
	SizeBytes     int64                         `json:"size_bytes,omitempty"`
	Manifest      *SupportBundleManifestPayload `json:"manifest,omitempty"`
	FailureReason string                        `json:"failure_reason,omitempty"`
	CreatedAt     time.Time                     `json:"created_at"`
	UpdatedAt     time.Time                     `json:"updated_at"`
	CompletedAt   *time.Time                    `json:"completed_at,omitempty"`
}

type SupportBundleManifestPayload struct {
	SchemaVersion        string                                 `json:"schema_version"`
	OperationID          string                                 `json:"operation_id"`
	CreatedAt            time.Time                              `json:"created_at"`
	BundleMaxBytes       int64                                  `json:"bundle_max_bytes"`
	ArtifactMaxBytes     int64                                  `json:"artifact_max_bytes"`
	LogTailMaxBytes      int64                                  `json:"log_tail_max_bytes"`
	EventSummaryMaxBytes int64                                  `json:"event_summary_max_bytes"`
	RedactionVersion     string                                 `json:"redaction_version"`
	Artifacts            []SupportBundleManifestArtifactPayload `json:"artifacts"`
}

type SupportBundleManifestArtifactPayload struct {
	Path             string `json:"path"`
	Included         bool   `json:"included"`
	OmittedReason    string `json:"omitted_reason,omitempty"`
	Bytes            int64  `json:"bytes"`
	Truncated        bool   `json:"truncated"`
	RedactionVersion string `json:"redaction_version,omitempty"`
}
