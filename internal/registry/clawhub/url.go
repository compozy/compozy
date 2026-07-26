package clawhub

import (
	"fmt"
	"net/url"
	"strings"
)

func (c *Client) buildURL(requestPath string, query url.Values) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}

	escapedPath := strings.TrimRight(base.EscapedPath(), "/") + "/" + strings.TrimLeft(requestPath, "/")
	decodedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return "", fmt.Errorf("decode request path: %w", err)
	}
	base.Path = decodedPath
	base.RawPath = escapedPath
	if len(query) > 0 {
		base.RawQuery = query.Encode()
	}

	return base.String(), nil
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultBaseURL
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return strings.TrimRight(trimmed, "/")
	}

	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		path = "/api/v1"
	}
	parsed.Path = path

	return strings.TrimRight(parsed.String(), "/")
}
