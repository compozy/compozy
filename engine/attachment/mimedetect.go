package attachment

import (
	"context"
	"net/http"
	"strings"

	"github.com/compozy/compozy/pkg/config"
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
		return strings.HasPrefix(mime, "image/")
	case TypePDF:
		return mime == "application/pdf"
	case TypeAudio:
		return strings.HasPrefix(mime, "audio/")
	case TypeVideo:
		return strings.HasPrefix(mime, "video/")
	default:
		return true
	}
}

// isAllowedByTypeCtx consults global configuration allow-lists when available.
// Falls back to the heuristic in isAllowedByType when config or list is empty.
func isAllowedByTypeCtx(ctx context.Context, t Type, mime string) bool {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return isAllowedByType(t, mime)
	}
	var allow []string
	switch t {
	case TypeImage:
		allow = cfg.Attachments.AllowedMIMETypes.Image
	case TypePDF:
		allow = cfg.Attachments.AllowedMIMETypes.PDF
	case TypeAudio:
		allow = cfg.Attachments.AllowedMIMETypes.Audio
	case TypeVideo:
		allow = cfg.Attachments.AllowedMIMETypes.Video
	default:
		return true
	}
	if len(allow) == 0 {
		return isAllowedByType(t, mime)
	}
	lmime := strings.ToLower(mime)
	for i := range allow {
		pat := strings.TrimSpace(allow[i])
		if pat == "" {
			continue
		}
		if strings.HasSuffix(pat, "/*") {
			base := strings.ToLower(strings.TrimSuffix(pat, "/*"))
			if strings.HasPrefix(lmime, base) {
				return true
			}
			continue
		}
		if strings.EqualFold(lmime, strings.ToLower(pat)) {
			return true
		}
	}
	return false
}
