package reviews

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
	"github.com/compozy/compozy/internal/core/provider"
)

const (
	metaFileName  = "_meta.md"
	reviewsPrefix = "reviews-"
)

var ErrNoReviewRounds = errors.New("no review rounds found")

func TaskDirectory(name string) string {
	return TaskDirectoryForWorkspace("", name)
}

func TaskDirectoryForWorkspace(workspaceRoot, name string) string {
	return model.TaskDirectoryForWorkspace(workspaceRoot, name)
}

func RoundDirName(round int) string {
	return fmt.Sprintf("%s%03d", reviewsPrefix, round)
}

func ReviewDirectory(prdDir string, round int) string {
	return filepath.Join(prdDir, RoundDirName(round))
}

func MetaPath(reviewDir string) string {
	return filepath.Join(reviewDir, metaFileName)
}

func DiscoverRounds(prdDir string) ([]int, error) {
	files, err := os.ReadDir(prdDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read prd directory: %w", err)
	}

	re := regexp.MustCompile(`^reviews-(\d+)$`)
	rounds := make([]int, 0, len(files))
	for _, entry := range files {
		if !entry.IsDir() {
			continue
		}
		matches := re.FindStringSubmatch(entry.Name())
		if len(matches) < 2 {
			continue
		}
		round, convErr := strconv.Atoi(matches[1])
		if convErr != nil {
			continue
		}
		rounds = append(rounds, round)
	}

	sort.Ints(rounds)
	return rounds, nil
}

func LatestRound(prdDir string) (int, error) {
	rounds, err := DiscoverRounds(prdDir)
	if err != nil {
		return 0, err
	}
	if len(rounds) == 0 {
		return 0, ErrNoReviewRounds
	}
	return rounds[len(rounds)-1], nil
}

func NextRound(prdDir string) (int, error) {
	rounds, err := DiscoverRounds(prdDir)
	if err != nil {
		return 0, err
	}
	if len(rounds) == 0 {
		return 1, nil
	}
	return rounds[len(rounds)-1] + 1, nil
}

func ReadReviewEntries(reviewDir string) ([]model.IssueEntry, error) {
	files, err := os.ReadDir(reviewDir)
	if err != nil {
		return nil, fmt.Errorf("read reviews directory: %w", err)
	}

	names := make([]string, 0, len(files))
	for _, entry := range files {
		if !entry.Type().IsRegular() {
			continue
		}
		if prompt.ExtractIssueNumber(entry.Name()) == 0 {
			continue
		}
		names = append(names, entry.Name())
	}

	sort.SliceStable(names, func(i, j int) bool {
		return prompt.ExtractIssueNumber(names[i]) < prompt.ExtractIssueNumber(names[j])
	})

	entries := make([]model.IssueEntry, 0, len(names))
	for _, name := range names {
		absPath := filepath.Join(reviewDir, name)
		body, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}

		content := string(body)
		codeFile := UnknownCodeFile(name)
		ctx, parseErr := prompt.ParseReviewContext(content)
		if parseErr != nil {
			if errors.Is(parseErr, prompt.ErrLegacyReviewMetadata) {
				return nil, fmt.Errorf("legacy review artifact detected at %s; run `compozy migrate`", absPath)
			}
			return nil, fmt.Errorf("parse review entry %s: %w", absPath, parseErr)
		}
		if ctx.File != "" {
			codeFile = ctx.File
		}

		entries = append(entries, model.IssueEntry{
			Name:     name,
			AbsPath:  absPath,
			Content:  content,
			CodeFile: codeFile,
		})
	}

	return entries, nil
}

func UnknownCodeFile(issueFileName string) string {
	return "__unknown__:" + issueFileName
}

func WriteRound(reviewDir string, meta model.RoundMeta, items []provider.ReviewItem) error {
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		return fmt.Errorf("mkdir review directory: %w", err)
	}

	meta.Total = len(items)
	meta.Resolved = 0
	meta.Unresolved = len(items)
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now().UTC()
	}

	if err := WriteRoundMeta(reviewDir, meta); err != nil {
		return err
	}
	for index, item := range items {
		if err := writeIssueFile(reviewDir, index+1, item); err != nil {
			return err
		}
	}
	return nil
}

func WriteRoundMeta(reviewDir string, meta model.RoundMeta) error {
	content, err := formatRoundMeta(meta)
	if err != nil {
		return fmt.Errorf("format round meta: %w", err)
	}
	if err := os.WriteFile(MetaPath(reviewDir), []byte(content), 0o600); err != nil {
		return fmt.Errorf("write round meta: %w", err)
	}
	return nil
}

func ReadRoundMeta(reviewDir string) (model.RoundMeta, error) {
	body, err := os.ReadFile(MetaPath(reviewDir))
	if err != nil {
		return model.RoundMeta{}, fmt.Errorf("read round meta: %w", err)
	}
	return parseRoundMeta(string(body))
}

func RefreshRoundMeta(reviewDir string) (model.RoundMeta, error) {
	meta, err := ReadRoundMeta(reviewDir)
	if err != nil {
		return model.RoundMeta{}, err
	}

	entries, err := ReadReviewEntries(reviewDir)
	if err != nil {
		return model.RoundMeta{}, err
	}

	meta.Total = len(entries)
	meta.Resolved = 0
	for _, entry := range entries {
		resolved, err := prompt.IsReviewResolved(entry.Content)
		if err != nil {
			return model.RoundMeta{}, fmt.Errorf("refresh round meta from %s: %w", entry.AbsPath, err)
		}
		if resolved {
			meta.Resolved++
		}
	}
	meta.Unresolved = meta.Total - meta.Resolved

	if err := WriteRoundMeta(reviewDir, meta); err != nil {
		return model.RoundMeta{}, err
	}
	return meta, nil
}

func FinalizeIssueStatuses(reviewDir string, entries []model.IssueEntry) error {
	root, err := os.OpenRoot(strings.TrimSpace(reviewDir))
	if err != nil {
		return fmt.Errorf("open review root: %w", err)
	}
	defer root.Close()

	for _, entry := range entries {
		if err := finalizeIssueStatus(root, reviewDir, entry.Name); err != nil {
			return err
		}
	}
	return nil
}

func formatRoundMeta(meta model.RoundMeta) (string, error) {
	type roundMetaFrontMatter struct {
		Provider  string    `yaml:"provider"`
		PR        string    `yaml:"pr,omitempty"`
		Round     int       `yaml:"round"`
		CreatedAt time.Time `yaml:"created_at"`
	}

	summary := strings.Join([]string{
		"## Summary",
		fmt.Sprintf("- Total: %d", meta.Total),
		fmt.Sprintf("- Resolved: %d", meta.Resolved),
		fmt.Sprintf("- Unresolved: %d", meta.Unresolved),
		"",
	}, "\n")

	return frontmatter.Format(roundMetaFrontMatter{
		Provider:  meta.Provider,
		PR:        meta.PR,
		Round:     meta.Round,
		CreatedAt: meta.CreatedAt.UTC(),
	}, summary)
}

func parseRoundMeta(content string) (model.RoundMeta, error) {
	type roundMetaFrontMatter struct {
		Provider  string    `yaml:"provider"`
		PR        string    `yaml:"pr,omitempty"`
		Round     int       `yaml:"round"`
		CreatedAt time.Time `yaml:"created_at"`
	}

	var frontMatter roundMetaFrontMatter
	body, err := frontmatter.Parse(content, &frontMatter)
	if err != nil {
		return model.RoundMeta{}, fmt.Errorf("parse round meta front matter: %w", err)
	}

	meta := model.RoundMeta{
		Provider:  strings.TrimSpace(frontMatter.Provider),
		PR:        strings.TrimSpace(frontMatter.PR),
		Round:     frontMatter.Round,
		CreatedAt: frontMatter.CreatedAt,
	}
	if meta.Provider == "" || meta.Round <= 0 || meta.CreatedAt.IsZero() {
		return model.RoundMeta{}, errors.New("meta front matter is incomplete")
	}

	if err := parseRoundMetaSummary(strings.Split(body, "\n"), &meta); err != nil {
		return model.RoundMeta{}, err
	}
	return meta, nil
}

func writeIssueFile(reviewDir string, number int, item provider.ReviewItem) error {
	meta := model.ReviewFileMeta{
		Status:      "pending",
		File:        fallback(item.File, model.UnknownFileName),
		Line:        floorAt(item.Line, 0),
		Severity:    strings.TrimSpace(item.Severity),
		Author:      fallback(item.Author, "unknown"),
		ProviderRef: strings.TrimSpace(item.ProviderRef),
	}

	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = "Review comment"
	}
	body := strings.TrimSpace(item.Body)
	if body == "" {
		body = "_No review comment body provided by provider._"
	}

	contentBody := strings.Join([]string{
		fmt.Sprintf("# Issue %03d: %s", number, title),
		"## Review Comment",
		"",
		body,
		"",
		"## Triage",
		"",
		"- Decision: `UNREVIEWED`",
		"- Notes:",
		"",
	}, "\n")
	content, err := frontmatter.Format(meta, contentBody)
	if err != nil {
		return fmt.Errorf("format review issue %03d front matter: %w", number, err)
	}

	path := filepath.Join(reviewDir, fmt.Sprintf("issue_%03d.md", number))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write issue_%03d.md: %w", number, err)
	}
	return nil
}

func finalizeIssueStatus(root *os.Root, reviewDir, issueName string) error {
	issueName, err := resolveIssueName(issueName)
	if err != nil {
		return err
	}

	content, err := root.ReadFile(issueName)
	if err != nil {
		return fmt.Errorf("read review issue %s: %w", issueName, err)
	}

	ctx, err := prompt.ParseReviewContext(string(content))
	if err != nil {
		return wrapReviewParseError(filepath.Join(strings.TrimSpace(reviewDir), issueName), err)
	}

	switch ctx.Status {
	case "resolved":
		return nil
	case "valid", "invalid":
		rewritten, err := frontmatter.RewriteStringField(string(content), "status", "resolved")
		if err != nil {
			return fmt.Errorf("rewrite review issue %s: %w", issueName, err)
		}
		if err := root.WriteFile(issueName, []byte(rewritten), 0o600); err != nil {
			return fmt.Errorf("write review issue %s: %w", issueName, err)
		}
		return nil
	case "pending":
		return fmt.Errorf("review issue %s remained pending after successful batch", issueName)
	default:
		return fmt.Errorf("review issue %s has unsupported status %q after successful batch", issueName, ctx.Status)
	}
}

func resolveIssueName(issueName string) (string, error) {
	name := filepath.Base(strings.TrimSpace(issueName))
	if prompt.ExtractIssueNumber(name) == 0 {
		return "", fmt.Errorf("invalid issue file name %q", issueName)
	}
	return name, nil
}

func fallback(value string, fallbackValue string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallbackValue
	}
	return trimmed
}

func floorAt(value int, fallbackValue int) int {
	if value < fallbackValue {
		return fallbackValue
	}
	return value
}

func parseRoundMetaSummary(lines []string, meta *model.RoundMeta) error {
	counts := map[string]*int{
		"Total":      &meta.Total,
		"Resolved":   &meta.Resolved,
		"Unresolved": &meta.Unresolved,
	}
	reCount := regexp.MustCompile(`^- (Total|Resolved|Unresolved): (\d+)$`)
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		matches := reCount.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		value, err := strconv.Atoi(matches[2])
		if err != nil {
			return fmt.Errorf("parse %s count: %w", matches[1], err)
		}
		*counts[matches[1]] = value
	}
	return nil
}

func wrapReviewParseError(path string, err error) error {
	if errors.Is(err, prompt.ErrLegacyReviewMetadata) {
		return fmt.Errorf("legacy review artifact detected at %s; run `compozy migrate`", path)
	}
	return fmt.Errorf("parse review artifact %s: %w", path, err)
}
