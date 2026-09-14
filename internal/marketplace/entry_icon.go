package marketplace

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strings"
)

const maxIconBytes = 64 * 1024

// CatalogDiagnostic reports a non-fatal field rejection without retaining unsafe values.
type CatalogDiagnostic struct {
	EntryID string `json:"entry_id"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ValidateIcon rejects unsupported or oversized catalog image references.
func ValidateIcon(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > maxIconBytes {
		return fmt.Errorf("icon exceeds 64 KiB")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("icon must be an HTTPS or data URL")
	}
	switch parsed.Scheme {
	case "https":
		if parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			return fmt.Errorf("icon must use HTTPS without credentials or fragment")
		}
		switch strings.ToLower(path.Ext(parsed.Path)) {
		case ".png", ".svg", ".webp":
			return nil
		default:
			return fmt.Errorf("icon must use PNG, SVG or WebP")
		}
	case "data":
		header, content, found := strings.Cut(strings.TrimPrefix(value, "data:"), ",")
		if !found || content == "" {
			return fmt.Errorf("icon data URL is empty or malformed")
		}
		media, encoded := strings.CutSuffix(header, ";base64")
		switch media {
		case "image/png", "image/svg+xml", "image/webp":
		default:
			return fmt.Errorf("icon must use PNG, SVG or WebP")
		}
		if encoded {
			if _, err := base64.StdEncoding.DecodeString(content); err != nil {
				return fmt.Errorf("icon data URL has invalid base64")
			}
		} else if _, err := url.PathUnescape(content); err != nil {
			return fmt.Errorf("icon data URL has invalid escaping")
		}
		return nil
	default:
		return fmt.Errorf("icon must be an HTTPS or data URL")
	}
}
