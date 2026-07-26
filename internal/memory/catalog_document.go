package memory

import (
	"crypto/sha256"

	"encoding/hex"

	"fmt"
	"sort"
	"strings"

	"unicode"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func catalogChunksForDocument(doc catalogDocument) []catalogChunk {
	searchText := strings.TrimSpace(strings.Join([]string{doc.Name, doc.Description, doc.Content}, "\n"))
	if searchText == "" {
		searchText = strings.TrimSpace(doc.Filename)
	}
	return []catalogChunk{{
		id:          doc.ID + "::chunk:0001",
		content:     searchText,
		contentHash: hashMemoryContent([]byte(searchText)),
		startLine:   1,
		endLine:     max(1, strings.Count(doc.Content, "\n")+1),
	}}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildCatalogMatchQuery(query string) (string, error) {
	terms, err := searchQueryTerms(query)
	if err != nil {
		return "", err
	}
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, quoteCatalogMatchTerm(term))
	}
	return strings.Join(quoted, " AND "), nil
}

func quoteCatalogMatchTerm(term string) string {
	return `"` + strings.ReplaceAll(strings.TrimSpace(term), `"`, `""`) + `"`
}

func tokenizeSearchQuery(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(query)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	out := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func cleanSnippet(value string) string {
	replacer := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")
	return strings.Join(strings.Fields(replacer.Replace(strings.TrimSpace(value))), " ")
}

func catalogDocID(scope memcontract.Scope, workspaceID string, filename string) string {
	return strings.Join(
		[]string{string(scope.Normalize()), strings.TrimSpace(workspaceID), strings.TrimSpace(filename)},
		"::",
	)
}

func catalogDocIDForHeader(scope memcontract.Scope, workspaceID string, header memcontract.Header) string {
	if scope.Normalize() != memcontract.ScopeAgent {
		return catalogDocID(scope, workspaceID, header.Filename)
	}
	return strings.Join(
		[]string{
			string(scope.Normalize()),
			strings.TrimSpace(workspaceID),
			strings.TrimSpace(header.AgentName),
			string(header.AgentTier.Normalize()),
			strings.TrimSpace(header.Filename),
		},
		"::",
	)
}

func hashMemoryContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func buildCatalogDocument(
	scope memcontract.Scope,
	workspaceID string,
	header memcontract.Header,
	rawContent []byte,
) (catalogDocument, error) {
	body, err := parseFrontmatter(rawContent, &memcontract.Header{})
	if err != nil {
		return catalogDocument{}, fmt.Errorf("memory: parse memory body for %q: %w", header.Filename, err)
	}
	return catalogDocument{
		ID:          catalogDocIDForHeader(scope, workspaceID, header),
		Scope:       scope.Normalize(),
		WorkspaceID: strings.TrimSpace(workspaceID),
		AgentName:   strings.TrimSpace(header.AgentName),
		AgentTier:   header.AgentTier.Normalize(),
		Filename:    header.Filename,
		Type:        header.Type.Normalize(),
		Name:        header.Name,
		Description: header.Description,
		Content:     strings.TrimSpace(body),
		ContentHash: hashMemoryContent(rawContent),
		Injection:   !strings.HasPrefix(strings.TrimSpace(header.Filename), "_system"),
		UpdatedAt:   header.ModTime.UTC(),
	}, nil
}

func fallbackSearchDocuments(query string, docs []catalogDocument, limit int) ([]memcontract.SearchResult, error) {
	terms, err := searchQueryTerms(query)
	if err != nil {
		return nil, err
	}
	limit = clampSearchLimit(limit)

	results := make([]memcontract.SearchResult, 0, min(limit, len(docs)))
	for _, doc := range docs {
		score := fallbackDocumentScore(doc, terms)
		if score <= 0 {
			continue
		}
		results = append(results, memcontract.SearchResult{
			Filename:    doc.Filename,
			Scope:       doc.Scope,
			Workspace:   doc.WorkspaceID,
			Type:        doc.Type,
			Name:        doc.Name,
			Description: doc.Description,
			Score:       score,
			Snippet:     fallbackSnippet(doc, terms),
			ModTime:     doc.UpdatedAt.UTC(),
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			if results[i].ModTime.Equal(results[j].ModTime) {
				return results[i].Filename < results[j].Filename
			}
			return results[i].ModTime.After(results[j].ModTime)
		}
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
