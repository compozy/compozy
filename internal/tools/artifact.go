package tools

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// ToolArtifactURIPrefix identifies retained oversized tool results.
	ToolArtifactURIPrefix = "agh://tool-artifacts/"
	// ToolArtifactMIMEType identifies the canonical retained ToolResult JSON envelope.
	ToolArtifactMIMEType = "application/vnd.agh.tool-result+json"
	// ToolArtifactName is the stable display name for retained result envelopes.
	ToolArtifactName = "tool-result.json"
	// DefaultToolArtifactPageBytes is the default artifact read size.
	DefaultToolArtifactPageBytes int64 = 64 << 10
	// MaxToolArtifactPageBytes is the largest public artifact read size.
	MaxToolArtifactPageBytes int64 = 64 << 10
)

var (
	// ErrToolArtifactNotFound hides missing, expired, and foreign-workspace artifacts behind one error.
	ErrToolArtifactNotFound = errors.New("tools: tool artifact not found")
	// ErrToolArtifactInvalidRange reports an offset or limit outside the retained content bounds.
	ErrToolArtifactInvalidRange = errors.New("tools: invalid tool artifact range")
	// ErrToolArtifactCorrupt reports retained bytes that do not match their content identity.
	ErrToolArtifactCorrupt = errors.New("tools: tool artifact is corrupt")
	// ErrToolArtifactPersistence reports a durable write or retention failure.
	ErrToolArtifactPersistence = errors.New("tools: tool artifact persistence failed")

	toolArtifactIDPattern = regexp.MustCompile(`^art_([0-9a-f]{64})$`)
)

// ToolArtifactRetention defines global disk bounds for workspace-scoped artifacts.
type ToolArtifactRetention struct {
	MaxCount int
	MaxBytes int64
	MaxAge   time.Duration
}

// Validate ensures retention always establishes finite positive bounds.
func (r ToolArtifactRetention) Validate() error {
	if r.MaxCount <= 0 {
		return fmt.Errorf("tool artifact max count must be greater than zero: %d", r.MaxCount)
	}
	if r.MaxBytes <= 0 {
		return fmt.Errorf("tool artifact max bytes must be greater than zero: %d", r.MaxBytes)
	}
	if r.MaxAge <= 0 {
		return fmt.Errorf("tool artifact max age must be greater than zero: %s", r.MaxAge)
	}
	return nil
}

// ToolArtifactPage is one exact byte range from a retained result envelope.
type ToolArtifactPage struct {
	Artifact   ArtifactRef `json:"artifact"`
	Offset     int64       `json:"offset"`
	Bytes      int64       `json:"bytes"`
	TotalBytes int64       `json:"total_bytes"`
	DataBase64 string      `json:"data_base64"`
	NextOffset int64       `json:"next_offset"`
	EOF        bool        `json:"eof"`
}

// ToolArtifactStore persists and pages workspace-scoped result envelopes.
type ToolArtifactStore interface {
	Put(ctx context.Context, workspaceID string, content []byte) (ArtifactRef, error)
	ReadPage(
		ctx context.Context,
		workspaceID string,
		uri string,
		offset int64,
		limit int64,
	) (ToolArtifactPage, error)
	Sweep(ctx context.Context) error
}

// ParseToolArtifactURI validates an opaque artifact URI and returns its path-safe identity.
func ParseToolArtifactURI(uri string) (string, error) {
	trimmed := strings.TrimSpace(uri)
	if trimmed != uri || !strings.HasPrefix(trimmed, ToolArtifactURIPrefix) {
		return "", fmt.Errorf("invalid tool artifact URI")
	}
	id := strings.TrimPrefix(trimmed, ToolArtifactURIPrefix)
	if !toolArtifactIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid tool artifact URI")
	}
	return id, nil
}

func toolArtifactSHA256(id string) string {
	matches := toolArtifactIDPattern.FindStringSubmatch(id)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func toolArtifactRef(id string, bytes int64) ArtifactRef {
	return ArtifactRef{
		URI:      ToolArtifactURIPrefix + id,
		Name:     ToolArtifactName,
		MIMEType: ToolArtifactMIMEType,
		Bytes:    bytes,
		SHA256:   toolArtifactSHA256(id),
	}
}
