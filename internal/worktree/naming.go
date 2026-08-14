package worktree

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

const fallbackSlug = "worktree"

var (
	nonSlugCharacter = regexp.MustCompile(`[^a-z0-9]+`)
	repeatedDash     = regexp.MustCompile(`-+`)
	nameAdjectives   = []string{"calm", "bright", "swift", "steady", "clear", "bold"}
	nameNouns        = []string{"badger", "falcon", "otter", "heron", "lynx", "wren"}
)

type DerivedNames struct {
	Branch    string
	Directory string
}

func SanitizeName(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = nonSlugCharacter.ReplaceAllString(slug, "-")
	slug = repeatedDash.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return fallbackSlug
	}
	return slug
}

func DeriveNames(value string) DerivedNames {
	slug := SanitizeName(value)
	return DerivedNames{Branch: slug, Directory: slug}
}

func discoveredLabel(entry GitWorktree) string {
	if branch := strings.TrimSpace(entry.Branch); branch != "" {
		return SanitizeName(branch)
	}
	sha := strings.TrimSpace(entry.HeadSHA)
	if len(sha) > 8 {
		sha = sha[:8]
	}
	return SanitizeName("detached-" + sha)
}

func GenerateName(taken func(string) bool) string {
	base := nameAdjectives[0] + "-" + nameNouns[0]
	if taken == nil || !taken(base) {
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if !taken(candidate) {
			return candidate
		}
	}
}

func RunBranchName(namespace, taskSlug, runID string) string {
	return namespace + RunWorktreeName(taskSlug, runID)
}

// RunWorktreeName returns the readable per-run name without its branch namespace.
func RunWorktreeName(taskSlug, runID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(runID)))
	suffix := fmt.Sprintf("%x", digest[:4])
	return SanitizeName(taskSlug) + "-" + suffix
}

func ReclamationEligible(branch, recordedNamespace string, createdBranch bool) bool {
	return createdBranch && recordedNamespace != "" && strings.HasPrefix(branch, recordedNamespace)
}

func generateID(prefix string) (string, error) {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("worktree: generate %s id: %w", prefix, err)
	}
	return prefix + "_" + hex.EncodeToString(entropy[:]), nil
}
