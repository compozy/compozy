package pluginsource

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/registry/gitsrc"
)

var ErrInvalidRef = errors.New("marketplace_source_invalid_ref")

var ErrSourceNameReserved = errors.New("marketplace_source_name_reserved")

var ErrSourceNameInvalid = errors.New("marketplace_source_name_invalid")

var sourceNamePattern = regexp.MustCompile(`^[a-z0-9._-]{1,64}$`)

func ValidateName(name string) error {
	if name == "compozy" || name == "compozy-catalog" {
		return ErrSourceNameReserved
	}
	if !sourceNamePattern.MatchString(name) || name == "." || name == ".." {
		return fmt.Errorf("%w: name must match [a-z0-9._-]{1,64} and cannot be a dot path", ErrSourceNameInvalid)
	}
	return nil
}

var githubRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+$`)

// NormalizeRef assigns one acquisition identity to equivalent source spellings.
func NormalizeRef(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.ContainsRune(value, 0) {
		return "", ErrInvalidRef
	}
	if filepath.IsAbs(value) {
		return folderRef(value), nil
	}
	if repository, ok := strings.CutPrefix(value, "github:"); ok {
		return normalizeGitHubRepository(repository)
	}
	if !strings.Contains(value, ":") {
		return normalizeGitHubRepository(value)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery ||
		parsed.Fragment != "" || strings.Contains(value, "#") || parsed.Opaque != "" {
		return "", ErrInvalidRef
	}
	switch parsed.Scheme {
	case "file":
		if parsed.Host != "" || !filepath.IsAbs(parsed.Path) || strings.ContainsRune(parsed.Path, 0) {
			return "", ErrInvalidRef
		}
		return folderRef(parsed.Path), nil
	case "https", "git+https":
		return normalizeRepositoryURL(parsed)
	default:
		return "", ErrInvalidRef
	}
}

func normalizeRepositoryURL(parsed *url.URL) (string, error) {
	explicitGit := parsed.Scheme == "git+https"
	parsed.Scheme = "https"
	parsed.Host = strings.ToLower(parsed.Host)
	if err := gitsrc.ValidateRepositoryRef(parsed.String()); err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidRef, err)
	}
	if parsed.Hostname() == "github.com" && (parsed.Port() == "" || parsed.Port() == "443") {
		return normalizeGitHubRepository(strings.Trim(parsed.Path, "/"))
	}
	if !explicitGit {
		return "", ErrInvalidRef
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawPath = ""
	return "git+" + parsed.String(), nil
}

func normalizeGitHubRepository(repository string) (string, error) {
	repository = strings.TrimSuffix(strings.TrimSuffix(repository, "/"), ".git")
	if !githubRepositoryPattern.MatchString(repository) {
		return "", ErrInvalidRef
	}
	_, name, _ := strings.Cut(repository, "/")
	if name == "." || name == ".." {
		return "", ErrInvalidRef
	}
	return "github:" + strings.ToLower(repository), nil
}

func folderRef(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Clean(path))}).String()
}
