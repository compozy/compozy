package memory

import (
	"context"

	"database/sql"

	"errors"
	"fmt"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func searchQueryTerms(query string) ([]string, error) {
	terms := tokenizeSearchQuery(query)
	if len(terms) == 0 {
		return nil, wrapValidationError(
			"search query",
			query,
			errors.New("query must include at least one letter or number"),
		)
	}
	return terms, nil
}

func clampSearchLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchLimit
	}
	return min(limit, maxSearchLimit)
}

func clampHistoryLimit(limit int) int {
	if limit <= 0 {
		return defaultHistoryLimit
	}
	return min(limit, maxHistoryLimit)
}

func (c *catalog) upsertState(ctx context.Context, key string, value string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	db, err := c.ensureDB(ctx)
	if err != nil {
		return err
	}
	if db == nil {
		return nil
	}
	return upsertCatalogState(ctx, db, key, value)
}

func (c *catalog) stateValue(ctx context.Context, key string) (string, bool, error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return "", false, err
	}
	if db == nil {
		return "", false, nil
	}

	var raw string
	if err := db.QueryRowContext(
		ctx,
		`SELECT value FROM memory_catalog_state WHERE key = ?`,
		key,
	).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("memory: load catalog state %q: %w", key, err)
	}
	return raw, true, nil
}

func catalogScopeStateKey(scope memcontract.Scope, workspaceID string) string {
	return fmt.Sprintf(
		"%s%s::%s",
		catalogStateKeyScopePrefix,
		scope.Normalize(),
		strings.TrimSpace(workspaceID),
	)
}

func upsertCatalogStateTx(ctx context.Context, tx catalogWriteExecutor, key string, value string) error {
	if tx == nil {
		return errors.New("catalog transaction is required")
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO memory_catalog_state (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key,
		value,
	); err != nil {
		return fmt.Errorf("persist catalog state %q: %w", key, err)
	}
	return nil
}

func upsertCatalogState(ctx context.Context, db *sql.DB, key string, value string) error {
	if db == nil {
		return nil
	}
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO memory_catalog_state (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key,
		value,
	); err != nil {
		return fmt.Errorf("persist catalog state %q: %w", key, err)
	}
	return nil
}

func fallbackDocumentScore(doc catalogDocument, terms []string) float64 {
	searchable := strings.ToLower(strings.Join([]string{doc.Name, doc.Description, doc.Content}, "\n"))
	score := 0.0
	for _, term := range terms {
		count := strings.Count(searchable, term)
		if count == 0 {
			continue
		}
		score += float64(count)
		if strings.Contains(strings.ToLower(doc.Name), term) {
			score += 5
		}
		if strings.Contains(strings.ToLower(doc.Description), term) {
			score += 2
		}
	}
	return score
}

func fallbackSnippet(doc catalogDocument, terms []string) string {
	candidates := []string{doc.Description, doc.Content}
	for _, candidate := range candidates {
		cleaned := cleanSnippet(candidate)
		lower := strings.ToLower(cleaned)
		for _, term := range terms {
			if strings.Contains(lower, term) {
				return clipSnippet(cleaned, term, 180)
			}
		}
	}
	return cleanSnippet(doc.Description)
}

func clipSnippet(text string, term string, maxLen int) string {
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	index := strings.Index(strings.ToLower(text), strings.ToLower(term))
	if index < 0 {
		return text[:maxLen]
	}
	start := max(0, index-(maxLen/3))
	end := min(len(text), start+maxLen)
	return strings.TrimSpace(text[start:end])
}
