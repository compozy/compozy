package vectordb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/compozy/compozy/engine/core"
)

// fileStore persists embeddings to a JSON file for deterministic demo storage.
type fileStore struct {
	mu        sync.RWMutex
	path      string
	dimension int
	records   map[string]Record
}

func newFileStore(cfg *Config) (Store, error) {
	if cfg == nil {
		return nil, errors.New("filesystem: config is required")
	}
	storePath := filepath.Clean(cfg.Path)
	dir := filepath.Dir(storePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("filesystem: ensure directory %q: %w", dir, err)
	}
	fs := &fileStore{
		path:      storePath,
		dimension: cfg.Dimension,
		records:   make(map[string]Record),
	}
	if err := fs.load(); err != nil {
		return nil, err
	}
	return fs, nil
}

func (s *fileStore) Upsert(_ context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range records {
		rec := records[i]
		if len(rec.Embedding) != s.dimension {
			return fmt.Errorf(
				"filesystem: record %q dimension mismatch (got %d want %d)",
				rec.ID,
				len(rec.Embedding),
				s.dimension,
			)
		}
		s.records[rec.ID] = Record{
			ID:        rec.ID,
			Text:      rec.Text,
			Embedding: append([]float32(nil), rec.Embedding...),
			Metadata:  core.CloneMap(rec.Metadata),
		}
	}
	return s.persistLocked()
}

func (s *fileStore) Search(_ context.Context, query []float32, opts SearchOptions) ([]Match, error) {
	if len(query) != s.dimension {
		return nil, fmt.Errorf("filesystem: query dimension mismatch (got %d want %d)", len(query), s.dimension)
	}
	threshold := opts.MinScore
	topK := opts.TopK
	if topK <= 0 {
		topK = defaultTopK
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	candidates := make([]Match, 0, len(s.records))
	for _, rec := range s.records {
		if !metadataMatches(rec.Metadata, opts.Filters) {
			continue
		}
		score := cosineSimilarity(rec.Embedding, query)
		if score < threshold {
			continue
		}
		candidates = append(candidates, Match{
			ID:       rec.ID,
			Score:    score,
			Text:     rec.Text,
			Metadata: core.CloneMap(rec.Metadata),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}
	return candidates, nil
}

func (s *fileStore) Delete(_ context.Context, filter Filter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(filter.IDs) > 0 {
		for _, id := range filter.IDs {
			delete(s.records, id)
		}
		return s.persistLocked()
	}
	changed := false
	for id, rec := range s.records {
		if metadataMatches(rec.Metadata, filter.Metadata) {
			delete(s.records, id)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.persistLocked()
}

func (s *fileStore) Close(context.Context) error {
	return nil
}

func (s *fileStore) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("filesystem: read %q: %w", s.path, err)
	}
	var payload fileStorePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("filesystem: decode %q: %w", s.path, err)
	}
	if payload.Dimension > 0 && s.dimension != payload.Dimension {
		return fmt.Errorf(
			"filesystem: stored dimension %d does not match config %d for %q",
			payload.Dimension,
			s.dimension,
			s.path,
		)
	}
	if payload.Dimension == 0 {
		payload.Dimension = s.dimension
	}
	s.dimension = payload.Dimension
	for i := range payload.Records {
		rec := payload.Records[i]
		s.records[rec.ID] = Record{
			ID:        rec.ID,
			Text:      rec.Text,
			Embedding: toFloat32(rec.Embedding, s.dimension),
			Metadata:  rec.Metadata,
		}
	}
	return nil
}

func (s *fileStore) persistLocked() error {
	payload := fileStorePayload{
		Dimension: s.dimension,
		Records:   make([]fileStoreRecord, 0, len(s.records)),
	}
	for _, rec := range s.records {
		payload.Records = append(payload.Records, fileStoreRecord{
			ID:        rec.ID,
			Text:      rec.Text,
			Embedding: toFloat64(rec.Embedding),
			Metadata:  rec.Metadata,
		})
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("filesystem: encode snapshot: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("filesystem: write snapshot: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("filesystem: commit snapshot: %w", err)
	}
	return nil
}

type fileStorePayload struct {
	Dimension int               `json:"dimension"`
	Records   []fileStoreRecord `json:"records"`
}

type fileStoreRecord struct {
	ID        string         `json:"id"`
	Text      string         `json:"text"`
	Embedding []float64      `json:"embedding"`
	Metadata  map[string]any `json:"metadata"`
}

func toFloat64(values []float32) []float64 {
	if len(values) == 0 {
		return nil
	}
	out := make([]float64, len(values))
	for i := range values {
		out[i] = float64(values[i])
	}
	return out
}

func toFloat32(values []float64, dimension int) []float32 {
	out := make([]float32, len(values))
	for i := range values {
		out[i] = float32(values[i])
	}
	if len(out) == dimension {
		return out
	}
	if len(out) > dimension {
		return out[:dimension]
	}
	extended := make([]float32, dimension)
	copy(extended, out)
	return extended
}
