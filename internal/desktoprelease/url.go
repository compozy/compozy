package desktoprelease

import (
	"fmt"
	"net/url"
)

func validateHTTPSURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("URL must use HTTPS without user information")
	}
	return parsed.String(), nil
}
