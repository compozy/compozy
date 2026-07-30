package gitsrc

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

func normalizeRepositoryRef(value string) (string, error) {
	repository := strings.TrimSpace(value)
	if repository == "" {
		return "", errors.New("gitsrc: repository URL is required")
	}
	if strings.HasPrefix(repository, "http://") || strings.HasPrefix(repository, "https://") {
		parsed, err := url.Parse(repository)
		if err != nil {
			return "", fmt.Errorf("gitsrc: parse repository URL: %w", err)
		}
		if parsed.User != nil {
			return "", errors.New("gitsrc: repository URL must not include credentials")
		}
		if strings.TrimSpace(parsed.Hostname()) == "" {
			return "", errors.New("gitsrc: repository URL hostname is required")
		}
	}
	return repository, nil
}

func repositoryName(repository string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(repository), "/")
	if colon := strings.LastIndex(trimmed, ":"); colon >= 0 && !strings.Contains(trimmed[colon+1:], "/") {
		trimmed = trimmed[colon+1:]
	}
	name := filepath.Base(trimmed)
	return strings.TrimSuffix(name, ".git")
}
