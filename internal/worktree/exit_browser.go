package worktree

import (
	"net/url"
	"strings"
)

const (
	httpsScheme = "https"
	httpScheme  = "http"
)

func browserCompareURL(remotes []string, base, head string, forge *ForgeCapabilities) string {
	if len(remotes) == 0 || strings.TrimSpace(head) == "" {
		return ""
	}
	if forge != nil && strings.TrimSpace(forge.CompareURLTemplate) != "" {
		return strings.NewReplacer(
			"{base}", url.PathEscape(base),
			"{head}", url.PathEscape(head),
		).Replace(forge.CompareURLTemplate)
	}
	remote, ok := parseRemoteWebURL(remotes[0])
	if !ok {
		return ""
	}
	root := strings.TrimRight(remote.String(), "/")
	basePath, headPath := url.PathEscape(base), url.PathEscape(head)
	switch strings.ToLower(remote.Hostname()) {
	case "github.com":
		return root + "/compare/" + basePath + "..." + headPath + "?expand=1"
	case "gitlab.com":
		return root + "/-/compare/" + basePath + "..." + headPath
	case "bitbucket.org":
		return root + "/branches/compare/" + headPath + ".." + basePath
	}
	return root
}

func sanitizeGitRemote(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || strings.HasPrefix(value, "git@") {
		return value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return value
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// sanitizeForgeWebURL accepts only an absolute HTTP(S) URL and strips the
// credential-bearing URL parts that must never cross the worktree boundary.
func sanitizeForgeWebURL(raw string) (string, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != httpsScheme && parsed.Scheme != httpScheme) {
		return "", ErrForge
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func parseRemoteWebURL(raw string) (*url.URL, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, false
	}
	if after, ok := strings.CutPrefix(value, "git@"); ok {
		hostPath := after
		host, repoPath, ok := strings.Cut(hostPath, ":")
		if !ok {
			return nil, false
		}
		return &url.URL{Scheme: "https", Host: host, Path: "/" + strings.TrimSuffix(repoPath, ".git")}, true
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return nil, false
	}
	if parsed.Scheme == "ssh" || parsed.Scheme == "git" {
		parsed.Scheme = "https"
	}
	parsed.User = nil
	parsed.Path = strings.TrimSuffix(parsed.Path, ".git")
	parsed.RawQuery, parsed.Fragment = "", ""
	return parsed, true
}
