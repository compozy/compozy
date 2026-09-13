package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/store"
)

// marketplaceFeedSource keeps preset acquisition inside the service-owned feed refresh flight.
type marketplaceFeedSource struct {
	feed    marketplace.FeedSource
	runtime *marketplaceRuntime
	baseURL string
}

var _ marketplace.Source = (*marketplaceFeedSource)(nil)

func (s *marketplaceFeedSource) Fetch(ctx context.Context) (*marketplace.Document, error) {
	document, err := s.feed.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	presets, err := s.feed.FetchPresets(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.runtime.applyPresets(ctx, s.baseURL, presets); err != nil {
		return nil, err
	}
	return document, nil
}

func (r *marketplaceRuntime) applyPresets(
	ctx context.Context,
	baseURL string,
	presets *marketplace.PresetDocument,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped || baseURL != r.config.Catalog.EffectiveBaseURL() {
		return store.ErrMarketplaceCatalogGenerationStale
	}
	sources, ttl, timeout, err := r.buildBindings(r.config, presets)
	if err != nil {
		return err
	}
	if err := r.service.Reconfigure(ctx, sources, ttl, timeout); err != nil {
		return err
	}
	r.presets = presets
	content, err := json.Marshal(marketplacePresetCache{BaseURL: baseURL, Document: presets})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.presetPath), 0o700); err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(r.presetPath, content, 0o600)
}

type marketplacePresetCache struct {
	BaseURL  string                      `json:"base_url"`
	Document *marketplace.PresetDocument `json:"document"`
}

// readMarketplacePresets restores only the last validated feed's derived registrations, never user overrides.
func readMarketplacePresets(path, baseURL string) (_ *marketplace.PresetDocument, err error) {
	file, err := fileutil.OpenRegularFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	// JSON escaping can expand the bounded 2 MiB source document up to sixfold.
	const maxPresetCacheBytes = 16 << 20
	raw, err := io.ReadAll(io.LimitReader(file, maxPresetCacheBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxPresetCacheBytes {
		return nil, errors.New("daemon: marketplace preset cache exceeds its size limit")
	}
	var cached marketplacePresetCache
	if err := json.Unmarshal(raw, &cached); err != nil {
		return nil, fmt.Errorf("daemon: decode marketplace preset cache: %w", err)
	}
	if cached.BaseURL != baseURL {
		return nil, nil
	}
	encoded, err := json.Marshal(cached.Document)
	if err != nil {
		return nil, err
	}
	return marketplace.DecodePresets(encoded)
}
