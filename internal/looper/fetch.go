package looper

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/looper/internal/looper/model"
	"github.com/compozy/looper/internal/looper/providers"
	"github.com/compozy/looper/internal/looper/reviews"
)

var defaultProviderRegistry = providers.DefaultRegistry

func fetchReviews(ctx context.Context, cfg *model.RuntimeConfig) (*FetchResult, error) {
	if err := validateFetchConfig(cfg); err != nil {
		return nil, err
	}

	prdDir := reviews.PRDDirectory(cfg.Name)
	resolvedPRDDir, err := filepath.Abs(prdDir)
	if err != nil {
		return nil, fmt.Errorf("resolve prd dir: %w", err)
	}
	if err := ensureFetchPRDDirectory(resolvedPRDDir); err != nil {
		return nil, err
	}

	round := cfg.Round
	if round <= 0 {
		round, err = reviews.NextRound(resolvedPRDDir)
		if err != nil {
			return nil, err
		}
	}

	reviewsDir := reviews.ReviewDirectory(resolvedPRDDir, round)
	if _, err := os.Stat(reviewsDir); err == nil {
		return nil, fmt.Errorf("review round already exists: %s", reviewsDir)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("check review round directory: %w", err)
	}

	registry := defaultProviderRegistry()
	reviewProvider, err := registry.Get(cfg.Provider)
	if err != nil {
		return nil, err
	}

	items, err := reviewProvider.FetchReviews(ctx, cfg.PR)
	if err != nil {
		return nil, err
	}

	meta := model.RoundMeta{
		Provider:  reviewProvider.Name(),
		PR:        cfg.PR,
		Round:     round,
		CreatedAt: time.Now().UTC(),
	}
	if err := reviews.WriteRound(reviewsDir, meta, items); err != nil {
		return nil, err
	}

	cfg.Round = round
	cfg.ReviewsDir = reviewsDir
	cfg.Provider = reviewProvider.Name()

	slog.Info(
		"fetched review issues",
		"provider",
		reviewProvider.Name(),
		"pr",
		cfg.PR,
		"count",
		len(items),
		"round",
		round,
		"reviews_dir",
		reviewsDir,
	)

	return &FetchResult{
		Name:       cfg.Name,
		Provider:   reviewProvider.Name(),
		PR:         cfg.PR,
		Round:      round,
		ReviewsDir: reviewsDir,
		Total:      len(items),
	}, nil
}

func validateFetchConfig(cfg *model.RuntimeConfig) error {
	if cfg == nil {
		return fmt.Errorf("runtime config is nil")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("fetch-reviews requires --name")
	}
	if strings.TrimSpace(cfg.Provider) == "" {
		return fmt.Errorf("fetch-reviews requires --provider")
	}
	if strings.TrimSpace(cfg.PR) == "" {
		return fmt.Errorf("fetch-reviews requires --pr")
	}
	if cfg.Round < 0 {
		return fmt.Errorf("fetch-reviews round cannot be negative (got %d)", cfg.Round)
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
