// Package attachment provides the polymorphic attachments domain model
// and core interfaces used across tasks, agents, and actions.
package attachment

import (
	"context"
	"io"

	"github.com/compozy/compozy/engine/core"
)

// Type represents the attachment type discriminator.
type Type string

const (
	TypeImage Type = "image"
	TypeVideo Type = "video"
	TypeAudio Type = "audio"
	TypePDF   Type = "pdf"
	TypeFile  Type = "file"
	TypeURL   Type = "url"
)

// Source represents the origin kind for attachments that support multiple sources.
type Source string

const (
	SourceURL  Source = "url"
	SourcePath Source = "path"
)

// Attachment is the common interface for all attachment configurations.
type Attachment interface {
	Type() Type
	Name() string
	Meta() map[string]any
	Resolve(ctx context.Context, cwd *core.PathCWD) (Resolved, error)
}

// Resolved represents a transport-agnostic resolved attachment handle.
type Resolved interface {
	AsURL() (string, bool)
	AsFilePath() (string, bool)
	Open() (io.ReadCloser, error)
	MIME() string
	Cleanup()
}
