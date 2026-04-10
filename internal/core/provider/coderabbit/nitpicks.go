package coderabbit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/provider"
)

const (
	nitpickSeverity   = "nitpick"
	nitpickHashLength = 12
)

var (
	nitpickHeaderRe     = regexp.MustCompile("(?m)^`([^`]+)`:\\s*\\*\\*(.+?)\\*\\*\\s*$")
	reviewFileSummaryRe = regexp.MustCompile(`^(.+?)\s+\(\d+\)$`)
)

type pullRequestReview struct {
	ID          int    `json:"id"`
	Body        string `json:"body"`
	SubmittedAt string `json:"submitted_at"`
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
}

type detailsBlock struct {
	start   int
	end     int
	summary string
	body    string
}

func (p *Provider) fetchPullRequestReviews(
	ctx context.Context,
	owner string,
	repo string,
	pr string,
) ([]pullRequestReview, error) {
	reviews := make([]pullRequestReview, 0, 16)
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("repos/%s/%s/pulls/%s/reviews?per_page=100&page=%d", owner, repo, pr, page)
		output, err := p.run(ctx, "api", endpoint)
		if err != nil {
			return nil, fmt.Errorf("fetch pull request reviews page %d: %w", page, err)
		}

		var pageReviews []pullRequestReview
		if err := json.Unmarshal(output, &pageReviews); err != nil {
			return nil, fmt.Errorf("decode pull request reviews page %d: %w", page, err)
		}

		reviews = append(reviews, pageReviews...)
		if len(pageReviews) < 100 {
			break
		}
	}
	return reviews, nil
}

func parseNitpickReviewItems(reviews []pullRequestReview, botLogin string) []provider.ReviewItem {
	if len(reviews) == 0 {
		return nil
	}

	latestByHash := make(map[string]*provider.ReviewItem)
	for _, review := range reviews {
		if review.User.Login != botLogin || strings.TrimSpace(review.Body) == "" {
			continue
		}

		parsedItems := parseNitpickReview(review)
		for idx := range parsedItems {
			item := parsedItems[idx]
			if item.ReviewHash == "" {
				continue
			}

			current, ok := latestByHash[item.ReviewHash]
			if !ok || nitpickItemIsNewer(item, *current) {
				next := item
				latestByHash[item.ReviewHash] = &next
			}
		}
	}

	items := make([]provider.ReviewItem, 0, len(latestByHash))
	for _, item := range latestByHash {
		items = append(items, *item)
	}
	return items
}

func parseNitpickReview(review pullRequestReview) []provider.ReviewItem {
	nitpickBlock, ok := findDetailsBlock(review.Body, func(summary string) bool {
		return strings.Contains(strings.ToLower(summary), "nitpick comments")
	})
	if !ok {
		return nil
	}

	fileBlocks := extractTopLevelDetailsBlocks(trimEnclosingTag(nitpickBlock.body, "blockquote"))
	items := make([]provider.ReviewItem, 0, len(fileBlocks))
	for _, fileBlock := range fileBlocks {
		filePath := parseNitpickFilePath(fileBlock.summary)
		if filePath == "" {
			continue
		}

		items = append(items, parseNitpicksForFile(
			review,
			filePath,
			trimEnclosingTag(fileBlock.body, "blockquote"),
		)...)
	}
	return items
}

func parseNitpicksForFile(review pullRequestReview, filePath string, body string) []provider.ReviewItem {
	trimmed := strings.TrimSpace(stripTopLevelDetailsBlocks(body))
	if trimmed == "" {
		return nil
	}

	matches := nitpickHeaderRe.FindAllStringSubmatchIndex(trimmed, -1)
	items := make([]provider.ReviewItem, 0, len(matches))
	for idx, match := range matches {
		lineRange := strings.TrimSpace(trimmed[match[2]:match[3]])
		title := normalizeNitpickText(trimmed[match[4]:match[5]])
		if title == "" {
			continue
		}

		bodyStart := match[1]
		bodyEnd := len(trimmed)
		if idx+1 < len(matches) {
			bodyEnd = matches[idx+1][0]
		}

		nitpickBody := normalizeNitpickBody(trimmed[bodyStart:bodyEnd])
		if nitpickBody == "" {
			nitpickBody = title
		}

		reviewID := strconv.Itoa(review.ID)
		reviewHash := buildNitpickHash(filePath, title, nitpickBody)
		items = append(items, provider.ReviewItem{
			Title:                   title,
			File:                    filePath,
			Line:                    parseNitpickLine(lineRange),
			Severity:                nitpickSeverity,
			Author:                  review.User.Login,
			Body:                    nitpickBody,
			ProviderRef:             buildNitpickProviderRef(reviewID, reviewHash),
			ReviewHash:              reviewHash,
			SourceReviewID:          reviewID,
			SourceReviewSubmittedAt: strings.TrimSpace(review.SubmittedAt),
		})
	}

	return items
}

func findDetailsBlock(text string, match func(string) bool) (detailsBlock, bool) {
	for _, block := range extractTopLevelDetailsBlocks(text) {
		if match(block.summary) {
			return block, true
		}
	}
	return detailsBlock{}, false
}

func extractTopLevelDetailsBlocks(text string) []detailsBlock {
	blocks := make([]detailsBlock, 0, 8)
	cursor := 0
	for {
		relativeStart := strings.Index(text[cursor:], "<details>")
		if relativeStart < 0 {
			break
		}

		start := cursor + relativeStart
		end := matchingDetailsEnd(text, start)
		if end < 0 {
			break
		}

		block := parseDetailsBlock(start, end, text[start:end])
		blocks = append(blocks, block)
		cursor = end
	}
	return blocks
}

func matchingDetailsEnd(text string, start int) int {
	cursor := start
	depth := 0
	for cursor < len(text) {
		nextOpen := strings.Index(text[cursor:], "<details>")
		if nextOpen >= 0 {
			nextOpen += cursor
		}
		nextClose := strings.Index(text[cursor:], "</details>")
		if nextClose >= 0 {
			nextClose += cursor
		}

		switch {
		case nextOpen >= 0 && (nextClose < 0 || nextOpen < nextClose):
			depth++
			cursor = nextOpen + len("<details>")
		case nextClose >= 0:
			depth--
			cursor = nextClose + len("</details>")
			if depth == 0 {
				return cursor
			}
		default:
			return -1
		}
	}
	return -1
}

func parseDetailsBlock(start int, end int, raw string) detailsBlock {
	inside := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "<details>"), "</details>"))
	summaryStart := strings.Index(inside, "<summary>")
	summaryEnd := strings.Index(inside, "</summary>")
	if summaryStart < 0 || summaryEnd < 0 || summaryEnd < summaryStart {
		return detailsBlock{
			start: start,
			end:   end,
			body:  strings.TrimSpace(inside),
		}
	}

	summary := html.UnescapeString(strings.TrimSpace(inside[summaryStart+len("<summary>") : summaryEnd]))
	body := strings.TrimSpace(inside[summaryEnd+len("</summary>"):])
	return detailsBlock{
		start:   start,
		end:     end,
		summary: summary,
		body:    body,
	}
}

func stripTopLevelDetailsBlocks(text string) string {
	blocks := extractTopLevelDetailsBlocks(text)
	if len(blocks) == 0 {
		return strings.TrimSpace(text)
	}

	var builder strings.Builder
	cursor := 0
	for _, block := range blocks {
		if block.start > cursor {
			builder.WriteString(text[cursor:block.start])
		}
		cursor = block.end
	}
	if cursor < len(text) {
		builder.WriteString(text[cursor:])
	}
	return strings.TrimSpace(builder.String())
}

func trimEnclosingTag(text string, tag string) string {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"

	trimmed := strings.TrimSpace(text)
	for strings.HasPrefix(trimmed, openTag) && strings.HasSuffix(trimmed, closeTag) {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, openTag))
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, closeTag))
	}
	return trimmed
}

func parseNitpickFilePath(summary string) string {
	trimmed := strings.TrimSpace(summary)
	matches := reviewFileSummaryRe.FindStringSubmatch(trimmed)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func parseNitpickLine(lineRange string) int {
	trimmed := strings.TrimSpace(lineRange)
	if trimmed == "" {
		return 0
	}

	for idx, r := range trimmed {
		if r < '0' || r > '9' {
			trimmed = trimmed[:idx]
			break
		}
	}

	line, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0
	}
	return line
}

func normalizeNitpickText(value string) string {
	trimmed := html.UnescapeString(strings.TrimSpace(value))
	trimmed = strings.ReplaceAll(trimmed, "`", "")
	return strings.Join(strings.Fields(trimmed), " ")
}

func normalizeNitpickBody(body string) string {
	lines := strings.Split(html.UnescapeString(body), "\n")
	normalized := make([]string, 0, len(lines))
	previousBlank := true

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			if !previousBlank {
				normalized = append(normalized, "")
			}
			previousBlank = true
			continue
		}

		normalized = append(normalized, strings.Join(strings.Fields(line), " "))
		previousBlank = false
	}

	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func buildNitpickHash(filePath string, title string, body string) string {
	canonical := strings.Join([]string{
		"provider:" + name,
		"file:" + canonicalHashValue(filePath),
		"title:" + canonicalHashValue(title),
		"body:" + canonicalHashValue(firstParagraph(body)),
	}, "\n")

	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])[:nitpickHashLength]
}

func canonicalHashValue(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func firstParagraph(body string) string {
	for _, paragraph := range strings.Split(body, "\n\n") {
		if trimmed := strings.TrimSpace(paragraph); trimmed != "" {
			return trimmed
		}
	}
	return strings.TrimSpace(body)
}

func buildNitpickProviderRef(reviewID string, reviewHash string) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(reviewID) != "" {
		parts = append(parts, "review:"+strings.TrimSpace(reviewID))
	}
	if strings.TrimSpace(reviewHash) != "" {
		parts = append(parts, "nitpick_hash:"+strings.TrimSpace(reviewHash))
	}
	return strings.Join(parts, ",")
}

func nitpickItemIsNewer(candidate provider.ReviewItem, current provider.ReviewItem) bool {
	candidateTime := parseNitpickSubmittedAt(candidate.SourceReviewSubmittedAt)
	currentTime := parseNitpickSubmittedAt(current.SourceReviewSubmittedAt)
	if candidateTime.After(currentTime) {
		return true
	}
	if currentTime.After(candidateTime) {
		return false
	}
	return compareReviewIDs(candidate.SourceReviewID, current.SourceReviewID) > 0
}

func parseNitpickSubmittedAt(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func compareReviewIDs(left string, right string) int {
	leftID, leftErr := strconv.ParseInt(strings.TrimSpace(left), 10, 64)
	rightID, rightErr := strconv.ParseInt(strings.TrimSpace(right), 10, 64)
	if leftErr == nil && rightErr == nil {
		switch {
		case leftID > rightID:
			return 1
		case leftID < rightID:
			return -1
		default:
			return 0
		}
	}

	return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
}
