package reviews

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
)

var (
	ErrLegacyReviewMetadata = errors.New("legacy XML review metadata detected")

	legacyReviewStatusHeadingRe = regexp.MustCompile(`(?mi)^##\s*status:`)
	legacyReviewStatusRe        = regexp.MustCompile(`(?mi)^##\s*status:\s*([a-z]+)\b`)
	issueFileNumberRe           = regexp.MustCompile(`^issue_(\d+)\.md$`)
)

const (
	reviewStatusPending  = "pending"
	reviewStatusResolved = "resolved"
	reviewStatusValid    = "valid"
	reviewStatusInvalid  = "invalid"
)

func ParseReviewContext(content string) (model.ReviewContext, error) {
	var meta model.ReviewFileMeta
	if _, err := frontmatter.Parse(content, &meta); err != nil {
		if LooksLikeLegacyReviewFile(content) {
			return model.ReviewContext{}, ErrLegacyReviewMetadata
		}
		return model.ReviewContext{}, fmt.Errorf("parse review front matter: %w", err)
	}

	ctx := model.ReviewContext{
		Status:      strings.ToLower(strings.TrimSpace(meta.Status)),
		File:        strings.TrimSpace(meta.File),
		Line:        meta.Line,
		Severity:    strings.TrimSpace(meta.Severity),
		Author:      strings.TrimSpace(meta.Author),
		ProviderRef: strings.TrimSpace(meta.ProviderRef),
	}
	if ctx.Status == "" {
		return model.ReviewContext{}, errors.New("review front matter missing status")
	}
	return ctx, nil
}

func ParseLegacyReviewContext(content string) (model.ReviewContext, error) {
	if !LooksLikeLegacyReviewFile(content) {
		return model.ReviewContext{}, errors.New("legacy review metadata not found")
	}

	ctx := model.ReviewContext{
		Status:      strings.ToLower(strings.TrimSpace(extractLegacyStatus(content))),
		File:        extractXMLTag(content, "file"),
		Severity:    extractXMLTag(content, "severity"),
		Author:      extractXMLTag(content, "author"),
		ProviderRef: extractXMLTag(content, "provider_ref"),
	}
	lineValue := strings.TrimSpace(extractXMLTag(content, "line"))
	if lineValue != "" {
		lineNumber, err := strconv.Atoi(lineValue)
		if err != nil {
			return model.ReviewContext{}, fmt.Errorf("parse legacy review line: %w", err)
		}
		ctx.Line = lineNumber
	}
	if ctx.Status == "" {
		return model.ReviewContext{}, errors.New("legacy review status not found")
	}
	return ctx, nil
}

func IsReviewResolved(content string) (bool, error) {
	ctx, err := ParseReviewContext(content)
	if err != nil {
		return false, err
	}
	return ctx.Status == reviewStatusResolved, nil
}

func ExtractIssueNumber(filename string) int {
	return extractFileNumber(filename, issueFileNumberRe)
}

func LooksLikeLegacyReviewFile(content string) bool {
	return strings.Contains(content, "<review_context>") ||
		legacyReviewStatusHeadingRe.MatchString(content)
}

func ExtractLegacyReviewBody(content string) (string, error) {
	if !LooksLikeLegacyReviewFile(content) {
		return "", errors.New("legacy review metadata not found")
	}

	lines := strings.Split(content, "\n")
	body := make([]string, 0, len(lines))
	inContext := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case legacyReviewStatusHeadingRe.MatchString(line):
			continue
		case trimmed == "<review_context>":
			inContext = true
			continue
		case trimmed == "</review_context>":
			inContext = false
			continue
		case inContext:
			continue
		default:
			body = append(body, line)
		}
	}

	return strings.TrimLeft(strings.Join(body, "\n"), "\n"), nil
}

func WrapParseError(path string, err error) error {
	if errors.Is(err, ErrLegacyReviewMetadata) {
		return fmt.Errorf("legacy review artifact detected at %s; run `compozy migrate`", path)
	}
	return fmt.Errorf("parse review artifact %s: %w", path, err)
}

func extractFileNumber(filename string, pattern *regexp.Regexp) int {
	matches := pattern.FindStringSubmatch(filepath.Base(filename))
	if len(matches) < 2 {
		return 0
	}
	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return num
}

func extractXMLTag(content, tag string) string {
	content = extractReviewContextBlock(content)
	if content == "" {
		return ""
	}

	openTag := "<" + tag + ">"
	start := strings.Index(content, openTag)
	if start < 0 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(content[start:], "</"+tag+">")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(content[start : start+end])
}

func extractReviewContextBlock(content string) string {
	const openTag = "<review_context>"
	const closeTag = "</review_context>"

	start := strings.Index(content, openTag)
	if start < 0 {
		return ""
	}
	start += len(openTag)

	end := strings.Index(content[start:], closeTag)
	if end < 0 {
		return ""
	}
	return content[start : start+end]
}

func extractLegacyStatus(content string) string {
	if matches := legacyReviewStatusRe.FindStringSubmatch(content); len(matches) > 1 {
		return matches[1]
	}
	return ""
}
