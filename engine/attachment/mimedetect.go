package attachment

import (
	"net/http"

	"github.com/gabriel-vasile/mimetype"
)

// detectMIME determines a MIME type using stdlib detection first and
// falling back to the broader mimetype library when ambiguous.
// The input should contain at least the first 512 bytes of content
// when available; fewer bytes are handled but may reduce accuracy.
func detectMIME(head []byte) string {
	if len(head) == 0 {
		return "application/octet-stream"
	}
	mt := http.DetectContentType(head)
	if mt != "application/octet-stream" {
		return mt
	}
	return mimetype.Detect(head).String()
}

// isAllowedByType enforces simple allow-lists per attachment type.
// File and URL generic types are permissive here and will be governed
// by global configuration in a later task.
func isAllowedByType(t Type, mime string) bool {
	switch t {
	case TypeImage:
		return hasPrefix(mime, "image/")
	case TypePDF:
		return mime == "application/pdf"
	case TypeAudio:
		return hasPrefix(mime, "audio/")
	case TypeVideo:
		return hasPrefix(mime, "video/")
	default:
		return true
	}
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
