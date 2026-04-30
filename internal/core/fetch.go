package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/providerdefaults"
	"github.com/compozy/compozy/internal/core/reviews"
)

var defaultProviderRegistry = providerdefaults.DefaultRegistryForWorkspace

func fetchReviews(ctx context.Context, cfg *model.RuntimeConfig) (*FetchResult, error) {
	registry := provider.ResolveRegistry(defaultProviderRegistry(fetchWorkspaceRoot(cfg)))
	return fetchReviewsWithRegistry(ctx, cfg, registry)
}

func fetchReviewsWithRegistry(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	registry provider.RegistryReader,
) (*FetchResult, error) {
	pending, err := fetchReviewItemsWithRegistry(ctx, cfg, registry)
	if err != nil {
		return nil, err
	}
	result, err := writeFetchedReviewRound(pending)
	if err != nil {
		return nil, err
	}

	cfg.Round = result.Round
	cfg.ReviewsDir = result.ReviewsDir
	cfg.Provider = result.Provider
	return result, nil
}

// FetchedReviewItems captures normalized provider feedback before round files are written.
type FetchedReviewItems struct {
	Name       string
	Provider   string
	PR         string
	Round      int
	ReviewsDir string
	Items      []provider.ReviewItem
}

func fetchReviewItemsWithRegistry(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	registry provider.RegistryReader,
) (*FetchedReviewItems, error) {
	if err := validateFetchConfig(cfg); err != nil {
		return nil, err
	}

	resolvedPRDDir, err := resolveFetchPRDDirectory(cfg)
	if err != nil {
		return nil, err
	}

	round, err := resolveFetchRound(cfg.Round, resolvedPRDDir)
	if err != nil {
		return nil, err
	}

	reviewsDir := reviews.ReviewDirectory(resolvedPRDDir, round)
	if err := ensureReviewRoundDoesNotExist(reviewsDir); err != nil {
		return nil, err
	}

	if registry == nil {
		registry = provider.ResolveRegistry(defaultProviderRegistry(cfg.WorkspaceRoot))
	}
	reviewProvider, err := resolveFetchReviewProvider(registry, cfg.Provider)
	if err != nil {
		return nil, err
	}

	items, err := reviewProvider.FetchReviews(ctx, provider.FetchRequest{
		PR:              cfg.PR,
		IncludeNitpicks: cfg.Nitpicks,
	})
	if err != nil {
		return nil, err
	}
	items, err = filterFetchedReviewBodyComments(resolvedPRDDir, round, items)
	if err != nil {
		return nil, err
	}

	return &FetchedReviewItems{
		Name:       cfg.Name,
		Provider:   reviewProvider.Name(),
		PR:         cfg.PR,
		Round:      round,
		ReviewsDir: reviewsDir,
		Items:      items,
	}, nil
}

func fetchWorkspaceRoot(cfg *model.RuntimeConfig) string {
	if cfg == nil {
		return ""
	}
	return cfg.WorkspaceRoot
}

func writeFetchedReviewRound(pending *FetchedReviewItems) (*FetchResult, error) {
	if pending == nil {
		return nil, fmt.Errorf("fetched review items are required")
	}
	meta := model.RoundMeta{
		Provider:  pending.Provider,
		PR:        pending.PR,
		Round:     pending.Round,
		CreatedAt: time.Now().UTC(),
	}
	if err := reviews.WriteRound(pending.ReviewsDir, meta, pending.Items); err != nil {
		return nil, err
	}

	slog.Info(
		"fetched review issues",
		"provider",
		pending.Provider,
		"pr",
		pending.PR,
		"count",
		len(pending.Items),
		"round",
		pending.Round,
		"reviews_dir",
		pending.ReviewsDir,
	)

	return &FetchResult{
		Name:       pending.Name,
		Provider:   pending.Provider,
		PR:         pending.PR,
		Round:      pending.Round,
		ReviewsDir: pending.ReviewsDir,
		Total:      len(pending.Items),
	}, nil
}

func resolveFetchReviewProvider(
	registry provider.RegistryReader,
	providerName string,
) (provider.Provider, error) {
	return registry.Get(providerName)
}

func resolveFetchRound(round int, prdDir string) (int, error) {
	if round > 0 {
		return round, nil
	}
	return reviews.NextRound(prdDir)
}

func FetchReviewsWithRegistryDirect(
	ctx context.Context,
	cfg Config,
	registry provider.RegistryReader,
) (*FetchResult, error) {
	return fetchReviewsWithRegistry(ctx, cfg.runtime(), registry)
}

// FetchReviewItemsWithRegistryDirect fetches normalized review items without writing round files.
func FetchReviewItemsWithRegistryDirect(
	ctx context.Context,
	cfg Config,
	registry provider.RegistryReader,
) (*FetchedReviewItems, error) {
	return fetchReviewItemsWithRegistry(ctx, cfg.runtime(), registry)
}

// WriteFetchedReviewRoundDirect writes a previously fetched non-empty review round.
func WriteFetchedReviewRoundDirect(pending *FetchedReviewItems) (*FetchResult, error) {
	return writeFetchedReviewRound(pending)
}

func resolveFetchPRDDirectory(cfg *model.RuntimeConfig) (string, error) {
	prdDir := reviews.TaskDirectoryForWorkspace(cfg.WorkspaceRoot, cfg.Name)
	resolvedPRDDir, err := filepath.Abs(prdDir)
	if err != nil {
		return "", fmt.Errorf("resolve prd dir: %w", err)
	}
	if err := ensureFetchPRDDirectory(resolvedPRDDir); err != nil {
		return "", err
	}
	return resolvedPRDDir, nil
}

func validateFetchConfig(cfg *model.RuntimeConfig) error {
	if cfg == nil {
		return fmt.Errorf("runtime config is nil")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("reviews fetch requires --name")
	}
	if strings.TrimSpace(cfg.Provider) == "" {
		return fmt.Errorf("reviews fetch requires --provider")
	}
	if strings.TrimSpace(cfg.PR) == "" {
		return fmt.Errorf("reviews fetch requires --pr")
	}
	if cfg.Round < 0 {
		return fmt.Errorf("reviews fetch round cannot be negative (got %d)", cfg.Round)
	}
	return nil
}

func ensureFetchPRDDirectory(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("prd directory not found: %s", dir)
		}
		return fmt.Errorf("stat prd directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("prd path is not a directory: %s", dir)
	}
	return nil
}

func ensureReviewRoundDoesNotExist(reviewsDir string) error {
	if _, err := os.Stat(reviewsDir); err == nil {
		return fmt.Errorf("review round already exists: %s", reviewsDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check review round directory: %w", err)
	}
	return nil
}

type reviewBodyCommentHistoryState struct {
	Resolved                bool
	Round                   int
	SourceReviewID          string
	SourceReviewSubmittedAt time.Time
}

func filterFetchedReviewBodyComments(
	prdDir string,
	currentRound int,
	items []provider.ReviewItem,
) ([]provider.ReviewItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	history, err := loadReviewBodyCommentHistory(prdDir, currentRound)
	if err != nil {
		return nil, err
	}
	if len(history) == 0 {
		return items, nil
	}

	filtered := make([]provider.ReviewItem, 0, len(items))
	for idx := range items {
		item := &items[idx]
		if strings.TrimSpace(item.ReviewHash) == "" {
			filtered = append(filtered, *item)
			continue
		}

		record, ok := history[item.ReviewHash]
		if !ok || !record.Resolved {
			filtered = append(filtered, *item)
			continue
		}

		if fetchedReviewBodyCommentIsNewer(*item, record) {
			filtered = append(filtered, *item)
		}
	}

	return filtered, nil
}

func loadReviewBodyCommentHistory(
	prdDir string,
	currentRound int,
) (map[string]reviewBodyCommentHistoryState, error) {
	rounds, err := reviews.DiscoverRounds(prdDir)
	if err != nil {
		return nil, err
	}

	history := make(map[string]reviewBodyCommentHistoryState)
	for _, round := range rounds {
		if round >= currentRound {
			continue
		}

		reviewDir := reviews.ReviewDirectory(prdDir, round)
		entries, err := reviews.ReadReviewEntries(reviewDir)
		if err != nil {
			return nil, err
		}

		for _, entry := range entries {
			ctx, err := reviews.ParseReviewContext(entry.Content)
			if err != nil {
				return nil, fmt.Errorf("parse review body comment history %s: %w", entry.AbsPath, err)
			}
			hash := strings.TrimSpace(ctx.ReviewHash)
			if hash == "" {
				continue
			}

			next := reviewBodyCommentHistoryState{
				Resolved:                strings.EqualFold(strings.TrimSpace(ctx.Status), "resolved"),
				Round:                   round,
				SourceReviewID:          strings.TrimSpace(ctx.SourceReviewID),
				SourceReviewSubmittedAt: parseReviewSubmittedAt(ctx.SourceReviewSubmittedAt),
			}
			current, ok := history[hash]
			if !ok || reviewBodyCommentHistoryEntryIsNewer(next, current) {
				history[hash] = next
			}
		}
	}

	return history, nil
}

func parseReviewSubmittedAt(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}

	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func fetchedReviewBodyCommentIsNewer(item provider.ReviewItem, record reviewBodyCommentHistoryState) bool {
	itemTime := parseReviewSubmittedAt(item.SourceReviewSubmittedAt)
	if itemTime.After(record.SourceReviewSubmittedAt) {
		return true
	}
	if record.SourceReviewSubmittedAt.After(itemTime) {
		return false
	}
	return compareSourceReviewIDs(item.SourceReviewID, record.SourceReviewID) > 0
}

func reviewBodyCommentHistoryEntryIsNewer(
	candidate reviewBodyCommentHistoryState,
	current reviewBodyCommentHistoryState,
) bool {
	if candidate.Round != current.Round {
		return candidate.Round > current.Round
	}
	if candidate.SourceReviewSubmittedAt.After(current.SourceReviewSubmittedAt) {
		return true
	}
	if current.SourceReviewSubmittedAt.After(candidate.SourceReviewSubmittedAt) {
		return false
	}
	return compareSourceReviewIDs(candidate.SourceReviewID, current.SourceReviewID) > 0
}

func compareSourceReviewIDs(left string, right string) int {
	leftID, leftErr := parseSourceReviewID(left)
	rightID, rightErr := parseSourceReviewID(right)
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

func parseSourceReviewID(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}
