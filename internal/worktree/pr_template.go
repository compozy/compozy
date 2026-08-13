package worktree

import (
	"bytes"
	"context"
	"path"
	"strings"
	"unicode/utf8"
)

const prTemplateMaxBytes = 8_000

var defaultPRTemplatePaths = []string{
	".github/pull_request_template.md",
	".github/pull_request_template.txt",
	"pull_request_template.md",
	"pull_request_template.txt",
	"docs/pull_request_template.md",
	"docs/pull_request_template.txt",
}

func (s *Service) detectPRTemplate(
	ctx context.Context,
	worktreePath string,
	base string,
	templatePaths []string,
) string {
	stdout, _, err := s.runner.Run(ctx, worktreePath, "ls-tree", "-r", "-z", base)
	if err != nil {
		return ""
	}
	entries := make(map[string]string)
	directoryCandidates := make([]string, 0)
	for raw := range bytes.SplitSeq(stdout, []byte{0}) {
		metadataAndName := strings.SplitN(string(raw), "\t", 2)
		if len(metadataAndName) != 2 {
			continue
		}
		metadata := strings.Fields(metadataAndName[0])
		if len(metadata) != 3 || metadata[1] != "blob" ||
			(metadata[0] != "100644" && metadata[0] != "100755") {
			continue
		}
		name := metadataAndName[1]
		lower := strings.ToLower(name)
		entries[lower] = name
		if isPRTemplateDirectoryCandidate(lower) {
			directoryCandidates = append(directoryCandidates, name)
		}
	}
	for _, group := range prTemplatePathGroups(templatePaths) {
		candidates := make([]string, 0, len(group))
		for _, candidate := range group {
			if actual := entries[candidate]; actual != "" {
				candidates = append(candidates, actual)
			}
		}
		if len(candidates) > 1 {
			return ""
		}
		if len(candidates) == 1 {
			return s.readPRTemplateBlob(ctx, worktreePath, base, candidates[0])
		}
	}
	if len(directoryCandidates) != 1 {
		return ""
	}
	return s.readPRTemplateBlob(ctx, worktreePath, base, directoryCandidates[0])
}

func prTemplatePathGroups(templatePaths []string) [][]string {
	if len(templatePaths) == 0 {
		templatePaths = defaultPRTemplatePaths
	}
	groups := make([][]string, 0, len(templatePaths))
	groupByStem := make(map[string]int, len(templatePaths))
	for _, candidate := range templatePaths {
		clean := path.Clean(strings.TrimSpace(candidate))
		if clean == "." || path.IsAbs(clean) || strings.HasPrefix(clean, "../") {
			continue
		}
		lower := strings.ToLower(clean)
		extension := path.Ext(lower)
		if extension != ".md" && extension != ".txt" {
			continue
		}
		stem := strings.TrimSuffix(lower, extension)
		if index, ok := groupByStem[stem]; ok {
			groups[index] = append(groups[index], lower)
			continue
		}
		groupByStem[stem] = len(groups)
		groups = append(groups, []string{lower})
	}
	return groups
}

func isPRTemplateDirectoryCandidate(lower string) bool {
	if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".txt") {
		return false
	}
	return strings.Contains(lower, "/pull_request_template/") ||
		strings.HasPrefix(lower, "pull_request_template/")
}

func (s *Service) readPRTemplateBlob(ctx context.Context, worktreePath, base, name string) string {
	stdout, _, err := s.runner.Run(ctx, worktreePath, "show", base+":"+name)
	if err != nil || bytes.IndexByte(stdout, 0) >= 0 {
		return ""
	}
	if len(stdout) > prTemplateMaxBytes {
		stdout = stdout[:prTemplateMaxBytes]
		for len(stdout) > 0 && !utf8.Valid(stdout) {
			stdout = stdout[:len(stdout)-1]
		}
	}
	return strings.TrimSpace(string(stdout))
}
