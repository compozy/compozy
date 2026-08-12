package github

import (
	"net/url"
	"strings"
)

type repository struct {
	owner string
	name  string
	url   string
}

func selectRepository(remoteURLs []string) (repository, bool) {
	for _, remoteURL := range remoteURLs {
		if repo, ok := parseRepository(remoteURL); ok {
			return repo, true
		}
	}
	return repository{}, false
}

func parseRepository(raw string) (repository, bool) {
	value := strings.TrimSpace(raw)
	const scpPrefix = "git@github.com:"
	if strings.HasPrefix(strings.ToLower(value), scpPrefix) {
		return repositoryFromPath(value[len(scpPrefix):])
	}
	parsed, err := url.Parse(value)
	if err != nil || !validGitHubRemoteURL(parsed) {
		return repository{}, false
	}
	return repositoryFromPath(parsed.Path)
}

func validGitHubRemoteURL(value *url.URL) bool {
	if value == nil || !strings.EqualFold(value.Hostname(), "github.com") ||
		value.RawQuery != "" || value.Fragment != "" || value.RawPath != "" {
		return false
	}
	switch strings.ToLower(value.Scheme) {
	case "https", "ssh", "git":
		return true
	default:
		return false
	}
}

func repositoryFromPath(path string) (repository, bool) {
	parts := strings.Split(strings.Trim(strings.TrimSuffix(path, ".git"), "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" ||
		parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." ||
		strings.ContainsAny(parts[0]+parts[1], "?#") {
		return repository{}, false
	}
	return repository{
		owner: parts[0],
		name:  parts[1],
		url:   "https://github.com/" + parts[0] + "/" + parts[1],
	}, true
}
