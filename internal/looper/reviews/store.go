package reviews

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/looper/internal/looper/model"
	"github.com/compozy/looper/internal/looper/prompt"
	"github.com/compozy/looper/internal/looper/provider"
)

const (
	metaFileName  = "_meta.md"
	groupedDir    = "grouped"
	reviewsPrefix = "reviews-"
)

var ErrNoReviewRounds = errors.New("no review rounds found")

func TaskDirectory(name string) string {
	return filepath.Join("tasks", name)
}

func RoundDirName(round int) string {
	return fmt.Sprintf("%s%03d", reviewsPrefix, round)
}

func ReviewDirectory(prdDir string, round int) string {
	return filepath.Join(prdDir, RoundDirName(round))
}

func GroupedDirectory(reviewDir string) string {
	return filepath.Join(reviewDir, groupedDir)
}

func GroupedFilePath(reviewDir string, codeFile string) string {
	return filepath.Join(GroupedDirectory(reviewDir), fmt.Sprintf("group_%s.md", prompt.SafeFileName(codeFile)))
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
		if ctx, parseErr := prompt.ParseReviewContext(content); parseErr == nil && ctx.File != "" {
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
	content := formatRoundMeta(meta)
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
		if prompt.IsReviewResolved(entry.Content) {
			meta.Resolved++
		}
	}
	meta.Unresolved = meta.Total - meta.Resolved

	if err := WriteRoundMeta(reviewDir, meta); err != nil {
		return model.RoundMeta{}, err
	}
	return meta, nil
}

func formatRoundMeta(meta model.RoundMeta) string {
	createdAt := meta.CreatedAt.UTC().Format(time.RFC3339)
	return strings.Join([]string{
		"---",
		"provider: " + meta.Provider,
		"pr: " + meta.PR,
		fmt.Sprintf("round: %d", meta.Round),
		"created_at: " + createdAt,
		"---",
		"",
		"## Summary",
		fmt.Sprintf("- Total: %d", meta.Total),
		fmt.Sprintf("- Resolved: %d", meta.Resolved),
		fmt.Sprintf("- Unresolved: %d", meta.Unresolved),
		"",
	}, "\n")
}

func parseRoundMeta(content string) (model.RoundMeta, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return model.RoundMeta{}, errors.New("meta front matter header not found")
	}

	meta, summaryStart, err := parseRoundMetaFrontMatter(lines)
	if err != nil {
		return model.RoundMeta{}, err
	}
	if err := parseRoundMetaSummary(lines[summaryStart:], &meta); err != nil {
		return model.RoundMeta{}, err
	}
	return meta, nil
}

func writeIssueFile(reviewDir string, number int, item provider.ReviewItem) error {
	ctx := struct {
		XMLName     xml.Name `xml:"review_context"`
		File        string   `xml:"file"`
		Line        int      `xml:"line"`
		Severity    string   `xml:"severity,omitempty"`
		Author      string   `xml:"author"`
		ProviderRef string   `xml:"provider_ref,omitempty"`
	}{
		File:        fallback(item.File, model.UnknownFileName),
		Line:        floorAt(item.Line, 0),
		Severity:    strings.TrimSpace(item.Severity),
		Author:      fallback(item.Author, "unknown"),
		ProviderRef: strings.TrimSpace(item.ProviderRef),
	}
	xmlBlock, err := xml.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal review context for issue %03d: %w", number, err)
	}

	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = "Review comment"
	}
	body := strings.TrimSpace(item.Body)
	if body == "" {
		body = "_No review comment body provided by provider._"
	}

	content := strings.Join([]string{
		fmt.Sprintf("# Issue %03d: %s", number, title),
		"",
		"## Status: pending",
		"",
		string(xmlBlock),
		"",
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

	path := filepath.Join(reviewDir, fmt.Sprintf("issue_%03d.md", number))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write issue_%03d.md: %w", number, err)
	}
	return nil
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

func parseRoundMetaFrontMatter(lines []string) (model.RoundMeta, int, error) {
	meta := model.RoundMeta{}
	idx := 1
	for ; idx < len(lines); idx++ {
		line := strings.TrimSpace(lines[idx])
		if line == "---" {
			idx++
			break
		}
		if line == "" {
			continue
		}
		if err := applyFrontMatterLine(line, &meta); err != nil {
			return model.RoundMeta{}, 0, err
		}
	}

	if meta.Provider == "" || meta.Round <= 0 || meta.CreatedAt.IsZero() {
		return model.RoundMeta{}, 0, errors.New("meta front matter is incomplete")
	}
	return meta, idx, nil
}

func applyFrontMatterLine(line string, meta *model.RoundMeta) error {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return fmt.Errorf("invalid meta front matter line %q", line)
	}

	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	switch key {
	case "provider":
		meta.Provider = value
	case "pr":
		meta.PR = value
	case "round":
		round, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse round: %w", err)
		}
		meta.Round = round
	case "created_at":
		createdAt, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return fmt.Errorf("parse created_at: %w", err)
		}
		meta.CreatedAt = createdAt
	}
	return nil
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
