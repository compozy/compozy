package attachments

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MIMEImagePNG       = "image/png"
	MIMEImageJPEG      = "image/jpeg"
	MIMEImageWebP      = "image/webp"
	MIMEApplicationPDF = "application/pdf"
	MIMETextMarkdown   = "text/markdown"
	MIMETextPlain      = "text/plain"
)

var (
	pngMagic  = [...]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	jpegMagic = [...]byte{0xff, 0xd8, 0xff}
	pdfMagic  = [...]byte{'%', 'P', 'D', 'F'}
	riffMagic = [...]byte{'R', 'I', 'F', 'F'}
	webpMagic = [...]byte{'W', 'E', 'B', 'P'}

	defaultAllowedMIME = [...]string{
		MIMEImagePNG,
		MIMEImageJPEG,
		MIMEImageWebP,
		MIMEApplicationPDF,
		MIMETextMarkdown,
		MIMETextPlain,
	}
)

// UnsupportedMIMEError names the sniffed type and the types the store accepts.
type UnsupportedMIMEError struct {
	MIME    string
	Allowed []string
}

var _ error = (*UnsupportedMIMEError)(nil)

// DefaultAllowedMIMEs returns the v1 sniff allowlist. Client Content-Type is ignored.
func DefaultAllowedMIMEs() []string {
	return slices.Clone(defaultAllowedMIME[:])
}

func (e *UnsupportedMIMEError) Error() string {
	allowed := strings.Join(e.Allowed, ", ")
	if e.MIME == "" {
		return fmt.Sprintf("%s: allowed types are %s", ErrUnsupportedMIME, allowed)
	}
	return fmt.Sprintf("%s %q: allowed types are %s", ErrUnsupportedMIME, e.MIME, allowed)
}

func (e *UnsupportedMIMEError) Unwrap() error {
	return ErrUnsupportedMIME
}

// SniffMIME detects a v1 type from magic bytes. Filename is used only to
// disambiguate valid UTF-8 text as markdown versus plain text.
func SniffMIME(data []byte, filename string) (string, string, int, int, error) {
	if mime, kind, width, height, ok := sniffBinaryMIME(data); ok {
		return mime, kind, width, height, nil
	}
	if isRejectedBinary(data, filename) {
		return "", "", 0, 0, newUnsupportedMIMEError("")
	}
	if isUTF8Text(data) {
		mime := MIMETextPlain
		if isMarkdownFilename(filename) {
			mime = MIMETextMarkdown
		}
		return mime, KindFile, 0, 0, nil
	}
	return "", "", 0, 0, newUnsupportedMIMEError("")
}

func sniffBinaryMIME(data []byte) (string, string, int, int, bool) {
	switch {
	case bytes.HasPrefix(data, pngMagic[:]):
		width, height := pngDimensions(data)
		return MIMEImagePNG, KindImage, width, height, true
	case bytes.HasPrefix(data, jpegMagic[:]):
		width, height := jpegDimensions(data)
		return MIMEImageJPEG, KindImage, width, height, true
	case isWebP(data):
		width, height := webpDimensions(data)
		return MIMEImageWebP, KindImage, width, height, true
	case bytes.HasPrefix(data, pdfMagic[:]):
		return MIMEApplicationPDF, KindFile, 0, 0, true
	default:
		return "", "", 0, 0, false
	}
}

func pngDimensions(data []byte) (int, int) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func jpegDimensions(data []byte) (int, int) {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func isUTF8Text(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	for _, r := range string(data) {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

func isWebP(data []byte) bool {
	return len(data) >= 12 && bytes.Equal(data[:4], riffMagic[:]) && bytes.Equal(data[8:12], webpMagic[:])
}

func isRejectedBinary(data []byte, filename string) bool {
	switch {
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return true
	case bytes.HasPrefix(data, []byte("PK")):
		return true
	case bytes.HasPrefix(data, riffMagic[:]):
		return true
	case strings.EqualFold(filepath.Ext(strings.TrimSpace(filename)), ".svg"):
		return true
	case looksLikeSVG(data):
		return true
	default:
		return false
	}
}

func looksLikeSVG(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("<svg")) || bytes.HasPrefix(trimmed, []byte("<SVG")) {
		return true
	}
	return bytes.HasPrefix(trimmed, []byte("<?xml")) && bytes.Contains(bytes.ToLower(trimmed), []byte("<svg"))
}

func isMarkdownFilename(filename string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(filename))) {
	case ".md", ".markdown", ".mdown":
		return true
	default:
		return false
	}
}

// NormalizeAllowedMIME validates and canonicalizes the attachment MIME allowlist.
func NormalizeAllowedMIME(allowed []string) ([]string, error) {
	if len(allowed) == 0 {
		return nil, fmt.Errorf("attachment allowed_mime must not be empty")
	}
	normalized := make([]string, 0, len(allowed))
	seen := make(map[string]struct{}, len(allowed))
	for _, raw := range allowed {
		mime := strings.ToLower(strings.TrimSpace(raw))
		if mime == "" {
			return nil, fmt.Errorf("attachment allowed_mime contains a blank entry")
		}
		if !slices.Contains(defaultAllowedMIME[:], mime) {
			return nil, newUnsupportedMIMEError(mime)
		}
		if _, exists := seen[mime]; exists {
			return nil, fmt.Errorf("attachment allowed_mime duplicates %q", mime)
		}
		seen[mime] = struct{}{}
		normalized = append(normalized, mime)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("attachment allowed_mime must not be empty")
	}
	return normalized, nil
}

func mimeAllowed(mime string, allowed []string) error {
	if slices.Contains(allowed, mime) {
		return nil
	}
	return newUnsupportedMIMEError(mime, allowed)
}

func newUnsupportedMIMEError(mime string, allowed ...[]string) *UnsupportedMIMEError {
	if len(allowed) == 0 {
		return &UnsupportedMIMEError{MIME: mime, Allowed: DefaultAllowedMIMEs()}
	}
	return &UnsupportedMIMEError{MIME: mime, Allowed: slices.Clone(allowed[0])}
}
