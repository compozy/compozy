package vectordb

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
)

type memoryStore struct {
	mu        sync.RWMutex
	records   map[string]Record
	dimension int
}

func newMemoryStore(cfg *Config) *memoryStore {
	dim := 0
	if cfg != nil {
		dim = cfg.Dimension
	}
	return &memoryStore{
		records:   make(map[string]Record),
		dimension: dim,
	}
}

func (m *memoryStore) Upsert(_ context.Context, records []Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range records {
		rec := records[i]
		if len(rec.Embedding) != m.dimension {
			return errors.New("memory: embedding dimension mismatch")
		}
		cloned := Record{
			ID:        rec.ID,
			Text:      rec.Text,
			Embedding: append([]float32(nil), rec.Embedding...),
			Metadata:  cloneMetadata(rec.Metadata),
		}
		m.records[rec.ID] = cloned
	}
	return nil
}

func (m *memoryStore) Search(_ context.Context, query []float32, opts SearchOptions) ([]Match, error) {
	if len(query) != m.dimension {
		return nil, errors.New("memory: query dimension mismatch")
	}
	threshold := opts.MinScore
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	candidates := make([]Match, 0, len(m.records))
	for _, rec := range m.records {
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
			Metadata: cloneMetadata(rec.Metadata),
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

func (m *memoryStore) Delete(_ context.Context, filter Filter) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(filter.IDs) > 0 {
		for _, id := range filter.IDs {
			delete(m.records, id)
		}
		return nil
	}
	for id, rec := range m.records {
		if metadataMatches(rec.Metadata, filter.Metadata) {
			delete(m.records, id)
		}
	}
	return nil
}

func (m *memoryStore) Close(context.Context) error {
	return nil
}

func cosineSimilarity(vecA, vecB []float32) float64 {
	var dot float64
	var magA float64
	var magB float64
	for i := 0; i < len(vecA); i++ {
		av := float64(vecA[i])
		bv := float64(vecB[i])
		dot += av * bv
		magA += av * av
		magB += bv * bv
	}
	denom := math.Sqrt(magA) * math.Sqrt(magB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func cloneMetadata(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func metadataMatches(meta map[string]any, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}
	for key, expected := range filters {
		val, ok := meta[key]
		if !ok {
			return false
		}
		switch actual := val.(type) {
		case string:
			if actual != expected {
				return false
			}
		case []string:
			found := false
			for _, entry := range actual {
				if entry == expected {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		default:
			return false
		}
	}
	return true
}
