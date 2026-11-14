package projectfiles

import (
	"path/filepath"
	"strings"
)

const (
	plainTextMediaType = "text/plain; charset=utf-8"
	downloadMediaType  = "application/octet-stream"
)

// inlineMediaTypes is the closed set of extensions a browser may render from raw project bytes.
// Markup and script (html, js, xml) are deliberately absent so they only ever download.
const oggAudioMediaType = "audio/ogg"

var inlineMediaTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".avif": "image/avif", ".bmp": "image/bmp", ".ico": "image/x-icon",
	".svg": "image/svg+xml",
	".mp4": "video/mp4", ".m4v": "video/mp4", ".webm": "video/webm", ".mov": "video/quicktime",
	".ogv": "video/ogg",
	".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": oggAudioMediaType, ".oga": oggAudioMediaType,
	".m4a": "audio/mp4", ".aac": "audio/aac", ".flac": "audio/flac", ".opus": oggAudioMediaType,
	".pdf":  "application/pdf",
	".css":  "text/css; charset=utf-8",
	".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf", ".otf": "font/otf",
	".md": plainTextMediaType, ".markdown": plainTextMediaType, ".mdx": plainTextMediaType,
	".csv": plainTextMediaType, ".tsv": plainTextMediaType, ".json": plainTextMediaType,
	".ipynb": plainTextMediaType, ".mmd": plainTextMediaType, ".txt": plainTextMediaType,
}

// InlineMediaType names the response type for raw file bytes by extension alone, never by
// sniffing. inline is false for every type outside the allowlist, which downloads as octets.
func InlineMediaType(path string) (mime string, inline bool) {
	if mime, ok := inlineMediaTypes[strings.ToLower(filepath.Ext(path))]; ok {
		return mime, true
	}
	return downloadMediaType, false
}
