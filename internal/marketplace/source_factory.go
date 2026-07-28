package marketplace

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// NewSource creates a catalog source for an HTTP(S) endpoint or absolute file URL.
func NewSource(kind Kind, baseURL string, client *http.Client) (Source, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, errors.New("marketplace catalog: base URL must use http, https, or file")
	}
	switch parsed.Scheme {
	case protocolHTTP, protocolHTTPS:
		return NewHTTPSource(kind, baseURL, client)
	case protocolFile:
		return NewDirectorySource(kind, baseURL)
	default:
		return nil, errors.New("marketplace catalog: base URL must use http, https, or file")
	}
}
