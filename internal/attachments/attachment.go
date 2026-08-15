package attachments

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// AttachmentURIPrefix identifies persisted session attachments.
	AttachmentURIPrefix = "compozy://session-attachments/"
	// KindImage marks image bytes that can carry pixel dimensions.
	KindImage = "image"
	// KindFile marks non-image attachment bytes.
	KindFile = "file"
)

var (
	// ErrNotFound hides missing, expired, and foreign-scope attachments behind one error.
	ErrNotFound = errors.New("attachments: attachment not found")
	// ErrCorrupt reports retained bytes that do not match their content identity.
	ErrCorrupt = errors.New("attachments: attachment is corrupt")
	// ErrPersistence reports a durable write or retention failure.
	ErrPersistence = errors.New("attachments: attachment persistence failed")
	// ErrInvalidID reports a path-unsafe workspace, session, or attachment identity.
	ErrInvalidID = errors.New("attachments: invalid attachment identity")
	// ErrTooLarge reports a single file that exceeds the configured byte ceiling.
	ErrTooLarge = errors.New("attachments: file exceeds max_file_bytes")
	// ErrUnsupportedMIME reports bytes outside the v1 allowlist.
	ErrUnsupportedMIME = errors.New("attachments: unsupported mime type")

	attachmentIDPattern = regexp.MustCompile(`^att_([0-9a-f]{64})$`)
	scopeIDPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// AttachmentRef is the durable identity and metadata for one stored attachment.
type AttachmentRef struct {
	ID        string
	URI       string
	Name      string
	MIMEType  string
	Bytes     int64
	SHA256    string
	Kind      string
	Width     int
	Height    int
	CreatedAt time.Time
}

// AttachmentRetention defines global disk bounds for session-scoped attachments.
type AttachmentRetention struct {
	MaxCount int
	MaxBytes int64
	MaxAge   time.Duration
}

// Validate ensures retention always establishes finite positive bounds.
func (r AttachmentRetention) Validate() error {
	if r.MaxCount <= 0 {
		return fmt.Errorf("attachment max count must be greater than zero: %d", r.MaxCount)
	}
	if r.MaxBytes <= 0 {
		return fmt.Errorf("attachment max bytes must be greater than zero: %d", r.MaxBytes)
	}
	if r.MaxAge <= 0 {
		return fmt.Errorf("attachment max age must be greater than zero: %s", r.MaxAge)
	}
	return nil
}

// ParseAttachmentURI validates an opaque attachment URI and returns its path-safe identity.
func ParseAttachmentURI(uri string) (string, error) {
	trimmed := strings.TrimSpace(uri)
	if trimmed != uri || !strings.HasPrefix(trimmed, AttachmentURIPrefix) {
		return "", fmt.Errorf("invalid attachment URI")
	}
	id := strings.TrimPrefix(trimmed, AttachmentURIPrefix)
	if !attachmentIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid attachment URI")
	}
	return id, nil
}

// ParseAttachmentID validates a persisted content-addressed attachment identity.
func ParseAttachmentID(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed != value || !attachmentIDPattern.MatchString(trimmed) {
		return "", fmt.Errorf("%w: %q", ErrInvalidID, value)
	}
	return trimmed, nil
}

// ValidateSize rejects a negative count or a payload over the configured per-file ceiling.
func ValidateSize(bytes int64, maxFileBytes int64) error {
	if maxFileBytes <= 0 {
		return fmt.Errorf("attachment max_file_bytes must be greater than zero: %d", maxFileBytes)
	}
	if bytes < 0 {
		return fmt.Errorf("attachment bytes must be greater than or equal to zero: %d", bytes)
	}
	if bytes > maxFileBytes {
		return fmt.Errorf("%w: %d exceeds max_file_bytes %d", ErrTooLarge, bytes, maxFileBytes)
	}
	return nil
}

func attachmentSHA256(id string) string {
	matches := attachmentIDPattern.FindStringSubmatch(id)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func attachmentRef(id string, meta attachmentMeta) AttachmentRef {
	return AttachmentRef{
		ID:        id,
		URI:       AttachmentURIPrefix + id,
		Name:      meta.Name,
		MIMEType:  meta.MIMEType,
		Bytes:     meta.Bytes,
		SHA256:    attachmentSHA256(id),
		Kind:      meta.Kind,
		Width:     meta.Width,
		Height:    meta.Height,
		CreatedAt: meta.CreatedAt,
	}
}
