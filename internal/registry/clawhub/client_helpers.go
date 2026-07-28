package clawhub

import (
	"errors"

	"net/http"

	"strings"
)

func closeIdleConnections(httpClient *http.Client) {
	if httpClient == nil {
		return
	}
	if httpClient.Transport == nil {
		if transport, ok := http.DefaultTransport.(interface{ CloseIdleConnections() }); ok {
			transport.CloseIdleConnections()
		}
		return
	}
	if transport, ok := httpClient.Transport.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

func contentSize(length int64) int64 {
	if length > 0 {
		return length
	}
	return -1
}

func joinErrors(errs ...error) error {
	var compact []error
	for _, err := range errs {
		if err != nil {
			compact = append(compact, err)
		}
	}
	if len(compact) == 0 {
		return nil
	}
	return errors.Join(compact...)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonBlankPreserve(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
