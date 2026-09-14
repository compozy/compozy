package marketplace

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// FeedSource reads both documents in the published v3 catalog family.
type FeedSource interface {
	Source
	FetchPresets(context.Context) (*PresetDocument, error)
}

// NewSource creates a catalog source for an HTTP(S) endpoint or absolute file URL.
func NewSource(baseURL string, client *http.Client) (FeedSource, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, errors.New("marketplace catalog: base URL must use http, https, or file")
	}
	switch parsed.Scheme {
	case protocolHTTP, protocolHTTPS:
		return NewHTTPSource(baseURL, client)
	case protocolFile:
		return NewDirectorySource(baseURL)
	default:
		return nil, errors.New("marketplace catalog: base URL must use http, https, or file")
	}
}
