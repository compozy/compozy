package attachment

import "time"

// TempFilePrefix is the filename prefix used for attachment temp files.
// Shared by production code and tests to ensure consistency.
const TempFilePrefix = "compozy-att-"

// HTTP defaults and policy knobs for attachment handling. These serve as
// package-level fallbacks when global configuration is unavailable.
// Values mirror defaults registered in pkg/config/definition/schema.go.
var (
	// DefaultMaxDownloadSizeBytes caps single-download size (bytes).
	DefaultMaxDownloadSizeBytes int64 = 10 * 1024 * 1024
	// DefaultDownloadTimeout bounds a single download end-to-end.
	DefaultDownloadTimeout = 30 * time.Second
	// DefaultMaxRedirects limits HTTP redirects followed during download.
	DefaultMaxRedirects = 3
)

// User agent used for outbound attachment HTTP requests.
const HTTPUserAgent = "Compozy/1.0"

// MIME detection reads the first N bytes of a stream/file.
// Keep in sync with http.DetectContentType guidance (512 bytes).
const MIMEHeadMaxBytes = 512

// Content conversion/capture bounds.
const (
	// MaxTextFileBytes limits bytes loaded into a TextPart from a file.
	MaxTextFileBytes = 5 * 1024 * 1024
	// MaxPDFExtractChars limits characters extracted when converting PDF to text.
	MaxPDFExtractChars = 1_000_000
)
