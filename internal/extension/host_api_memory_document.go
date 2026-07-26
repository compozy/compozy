package extensionpkg

import (
	"fmt"
	"path/filepath"
	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	yaml "github.com/goccy/go-yaml"
)

type hostAPIMemoryDocument struct {
	Key     string
	Scope   memcontract.Scope
	Content string
	Tags    []string
}

func renderMemoryDocument(doc hostAPIMemoryDocument) (string, error) {
	header := memcontract.Header{
		Name:        memoryNameFromFilename(doc.Key),
		Description: memoryDescriptionFromContent(doc.Content),
		Type:        memoryTypeForScope(doc.Scope, doc.Tags),
	}
	if err := header.Validate(); err != nil {
		return "", invalidParamsRPCError(err)
	}

	metadata, err := yaml.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("extension: marshal memory frontmatter: %w", err)
	}

	var builder strings.Builder
	builder.WriteString("---\n")
	builder.Write(metadata)
	builder.WriteString("---\n\n")
	body := strings.TrimSpace(doc.Content)
	tags := normalizeUniqueStrings(doc.Tags)
	if len(tags) > 0 {
		builder.WriteString(tagCommentPrefix)
		builder.WriteByte(' ')
		builder.WriteString(strings.Join(tags, ", "))
		builder.WriteString(" -->\n\n")
	}
	builder.WriteString(body)
	return builder.String(), nil
}

func memoryTypeForScope(scope memcontract.Scope, tags []string) memcontract.Type {
	for _, tag := range normalizeUniqueStrings(tags) {
		switch memcontract.Type(tag).Normalize() {
		case memcontract.TypeUser, memcontract.TypeFeedback, memcontract.TypeProject, memcontract.TypeReference:
			return memcontract.Type(tag).Normalize()
		}
	}
	if scope == memcontract.ScopeWorkspace {
		return memcontract.TypeProject
	}
	return memcontract.TypeUser
}

func memoryNameFromFilename(filename string) string {
	base := strings.TrimSuffix(filepath.Base(strings.TrimSpace(filename)), filepath.Ext(strings.TrimSpace(filename)))
	if base == "" {
		return ""
	}

	normalized := strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(base)
	parts := strings.Fields(normalized)
	for i, part := range parts {
		parts[i] = titleCaseWord(part)
	}
	return strings.Join(parts, " ")
}

func titleCaseWord(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) == 1 {
		return strings.ToUpper(trimmed)
	}
	return strings.ToUpper(trimmed[:1]) + strings.ToLower(trimmed[1:])
}

func memoryDescriptionFromContent(content string) string {
	firstLine := strings.TrimSpace(strings.Split(strings.TrimSpace(content), "\n")[0])
	if len(firstLine) <= maxMemoryDescriptionLength {
		return firstLine
	}
	return strings.TrimSpace(firstLine[:maxMemoryDescriptionLength]) + "..."
}

func normalizeMemoryFilename(key string) string {
	filename := strings.TrimSpace(key)
	if filepath.Ext(filename) == "" {
		filename += ".md"
	}
	return filename
}

func hostAPIMemoryRecallEntries(packaged memcontract.Packaged, limit int) []hostAPIMemoryRecallEntry {
	entries := make([]hostAPIMemoryRecallEntry, 0)
	for _, block := range packaged.Blocks {
		for _, entry := range block.Entries {
			entries = append(entries, hostAPIMemoryRecallEntry{
				Key:     strings.TrimSpace(entry.ID),
				Content: taskpkg.RedactClaimTokens(strings.TrimSpace(entry.Body)),
				Score:   float64(len(entries) + 1),
			})
		}
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i].Score, entries[j].Score = entries[j].Score, entries[i].Score
	}
	if limit > 0 && len(entries) > limit {
		return entries[:limit]
	}
	return entries
}

func hostAPISessionStatusFromInfo(info *session.Info) hostAPISessionStatus {
	if info == nil {
		return hostAPISessionStatus{}
	}
	return hostAPISessionStatus{
		SessionID:    info.ID,
		Name:         info.Name,
		Agent:        info.AgentName,
		Provider:     info.Provider,
		WorkspaceID:  info.WorkspaceID,
		Workspace:    info.Workspace,
		State:        info.State,
		StopReason:   info.StopReason,
		StopDetail:   info.StopDetail,
		ACPSessionID: info.ACPSessionID,
		CreatedAt:    info.CreatedAt,
		UpdatedAt:    info.UpdatedAt,
	}
}
