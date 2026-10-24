package worktree

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"
)

// NormalizeRepoKey strips transport and credentials; identity never contains secrets.
func NormalizeRepoKey(remote string) (string, error) {
	remote = strings.TrimSpace(remote)
	if !strings.Contains(remote, "://") {
		if at := strings.LastIndex(remote, "@"); at >= 0 {
			remote = remote[at+1:]
		}
		if colon := strings.Index(remote, ":"); colon > 0 {
			remote = "ssh://" + remote[:colon] + "/" + remote[colon+1:]
		}
	}
	parsed, err := url.Parse(remote)
	if err != nil || parsed.Hostname() == "" ||
		(parsed.Scheme != httpsScheme && parsed.Scheme != "http" && parsed.Scheme != "ssh" && parsed.Scheme != "git") ||
		strings.Contains(parsed.EscapedPath(), "%") ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return "", errors.New("worktree: origin repository unavailable")
	}
	path := strings.TrimSuffix(strings.Trim(parsed.Path, "/"), ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || parts[0] == "." || parts[0] == ".." || parts[1] == "." ||
		parts[1] == ".." {
		return "", errors.New("worktree: invalid origin repository")
	}
	return strings.ToLower(parsed.Hostname() + "/" + path), nil
}

func (s *PullRequestService) RepoKey(ctx context.Context, root string) (string, error) {
	s.mu.Lock()
	cached, ok := s.repos[root]
	s.mu.Unlock()
	if ok && s.now().Before(cached.until) {
		return cached.key, nil
	}
	out, _, err := s.runner.Run(ctx, root, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	key, err := NormalizeRepoKey(string(out))
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.repos[root] = repoKeyCache{key: key, until: s.now().Add(time.Minute)}
	s.mu.Unlock()
	return key, nil
}

// IsDefaultBranch uses the local origin HEAD; absent metadata falls back to the
// conventional names. It never contacts the remote and caches each root briefly.
func (s *PullRequestService) IsDefaultBranch(ctx context.Context, root, branch string) bool {
	return slices.Contains(s.DefaultBranchNames(ctx, root), branch)
}

// DefaultBranchNames shares the cached local origin HEAD with display filtering.
func (s *PullRequestService) DefaultBranchNames(ctx context.Context, root string) []string {
	s.mu.Lock()
	cached, ok := s.defaults[root]
	s.mu.Unlock()
	if !ok || !s.now().Before(cached.until) {
		out, _, err := s.runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
		name := ""
		if err == nil {
			name = strings.TrimPrefix(strings.TrimSpace(string(out)), "origin/")
		}
		cached = repoKeyCache{key: name, until: s.now().Add(time.Minute)}
		s.mu.Lock()
		s.defaults[root] = cached
		s.mu.Unlock()
	}
	if cached.key == "" {
		return []string{"main", "master"}
	}
	return []string{cached.key}
}
