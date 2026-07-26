package daemon

import (
	"errors"
	"fmt"

	"os"
	"path/filepath"

	"strings"
	"time"

	memorypkg "github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func redactMemoryPackaged(packaged memcontract.Packaged) memcontract.Packaged {
	redacted := packaged
	redacted.Header.Text = taskpkg.RedactClaimTokens(strings.TrimSpace(redacted.Header.Text))
	for blockIdx := range redacted.Blocks {
		for entryIdx := range redacted.Blocks[blockIdx].Entries {
			entry := &redacted.Blocks[blockIdx].Entries[entryIdx]
			entry.Title = taskpkg.RedactClaimTokens(strings.TrimSpace(entry.Title))
			entry.Body = taskpkg.RedactClaimTokens(strings.TrimSpace(entry.Body))
			entry.StalenessBanner = taskpkg.RedactClaimTokens(strings.TrimSpace(entry.StalenessBanner))
			for i := range entry.WhyRecalled {
				entry.WhyRecalled[i] = taskpkg.RedactClaimTokens(strings.TrimSpace(entry.WhyRecalled[i]))
			}
		}
	}
	return redacted
}

func nativeMemoryRecallResults(packaged memcontract.Packaged) []nativeMemoryRecallEntry {
	total := 0
	for _, block := range packaged.Blocks {
		total += len(block.Entries)
	}
	results := make([]nativeMemoryRecallEntry, 0, total)
	score := float64(total)
	for _, block := range packaged.Blocks {
		for _, entry := range block.Entries {
			results = append(results, nativeMemoryRecallEntry{
				Key:     strings.TrimSpace(entry.ID),
				Content: strings.TrimSpace(entry.Body),
				Score:   score,
			})
			score--
		}
	}
	return results
}

func nativeMemoryFilename(rawType string, seed string) string {
	prefix := string(memcontract.Type(strings.TrimSpace(rawType)).Normalize())
	if prefix == "" {
		prefix = string(HarnessPromptSectionMemory)
	}
	return prefix + "_" + nativeMemorySlug(seed) + ".md"
}

func nativeMemoryAdHocFilename(rawSlug string, content string, now time.Time) string {
	slug := nativeMemorySlug(firstNonEmpty(rawSlug, content, nativeToolsNoteKey))
	return fmt.Sprintf("ad_hoc_%s_%s.md", now.UTC().Format("20060102T150405Z"), slug)
}

func nativeMemoryNameFromFilename(filename string) string {
	base := strings.TrimSuffix(filepath.Base(strings.TrimSpace(filename)), filepath.Ext(strings.TrimSpace(filename)))
	parts := strings.Fields(strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(base))
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func nativeMemoryDescription(content string) string {
	firstLine := strings.TrimSpace(strings.Split(strings.TrimSpace(content), "\n")[0])
	const maxNativeMemoryDescriptionLength = 160
	if len(firstLine) <= maxNativeMemoryDescriptionLength {
		return firstLine
	}
	return strings.TrimSpace(firstLine[:maxNativeMemoryDescriptionLength]) + "..."
}

func nativeMemoryTaggedContent(content string, tags []string) string {
	body := strings.TrimSpace(content)
	normalized := nativeNormalizeUniqueStrings(tags)
	if len(normalized) == 0 {
		return body
	}
	return "<!-- agh-tags: " + strings.Join(normalized, ", ") + " -->\n\n" + body
}

func nativeNormalizeUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func nativeMemorySlug(seed string) string {
	trimmed := strings.ToLower(strings.TrimSpace(seed))
	var builder strings.Builder
	lastDash := false
	for _, r := range trimmed {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return nativeToolsNoteKey
	}
	const maxNativeMemorySlugLength = 48
	if len(slug) > maxNativeMemorySlugLength {
		slug = strings.Trim(slug[:maxNativeMemorySlugLength], "-")
	}
	if slug == "" {
		return nativeToolsNoteKey
	}
	return slug
}

func nativeMemoryToolError(id toolspkg.ToolID, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, memorypkg.ErrValidation):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, os.ErrNotExist):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, err),
			toolspkg.ReasonToolUnknown,
		)
	default:
		return err
	}
}
