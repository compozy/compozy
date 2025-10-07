package chunk

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tmc/langchaingo/textsplitter"
)

const (
	StrategyRecursive = "recursive_text_splitter"
)

var newlinePattern = regexp.MustCompile(`\r\n|\r`)

// Processor handles chunking according to supplied configuration.
type Processor struct {
	settings Settings
}

// NewProcessor builds a processor with sanitized defaults.
func NewProcessor(settings Settings) (*Processor, error) {
	if settings.Strategy == "" {
		settings.Strategy = StrategyRecursive
	}
	if settings.Size <= 0 {
		return nil, errors.New("chunk: size must be greater than zero")
	}
	if settings.Overlap < 0 {
		return nil, errors.New("chunk: overlap cannot be negative")
	}
	if settings.Overlap >= settings.Size {
		return nil, fmt.Errorf("chunk: overlap %d must be smaller than size %d", settings.Overlap, settings.Size)
	}
	return &Processor{settings: settings}, nil
}

// Process splits documents into deterministic chunks.
func (p *Processor) Process(kbID string, docs []Document) ([]Chunk, error) {
	if strings.TrimSpace(kbID) == "" {
		return nil, errors.New("chunk: knowledge base id is required")
	}
	if len(docs) == 0 {
		return nil, nil
	}
	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithChunkSize(p.settings.Size),
		textsplitter.WithChunkOverlap(p.settings.Overlap),
	)
	seen := make(map[string]struct{})
	chunks := make([]Chunk, 0, len(docs))
	for di := range docs {
		doc := docs[di]
		text := p.preprocess(doc.Text)
		if text == "" {
			continue
		}
		segments, err := splitter.SplitText(text)
		if err != nil {
			return nil, fmt.Errorf("chunk: split document %s: %w", doc.ID, err)
		}
		for idx, segment := range segments {
			chunkText := strings.TrimSpace(segment)
			if chunkText == "" {
				continue
			}
			hash := hashText(chunkText)
			if p.settings.Deduplicate {
				if _, exists := seen[hash]; exists {
					continue
				}
				seen[hash] = struct{}{}
			}
			chunkID := hashText(kbID + "::" + doc.ID + "::" + fmt.Sprint(idx) + "::" + hash)
			metadata := cloneMetadata(doc.Metadata)
			if metadata == nil {
				metadata = make(map[string]any)
			}
			metadata["chunk_index"] = idx
			metadata["source_id"] = doc.ID
			chunks = append(chunks, Chunk{
				ID:       chunkID,
				Text:     chunkText,
				Hash:     hash,
				Metadata: metadata,
			})
		}
	}
	return chunks, nil
}

func (p *Processor) preprocess(text string) string {
	normalized := text
	if p.settings.RemoveHTML {
		normalized = stripHTML(normalized)
	}
	if p.settings.NormalizeNewlines {
		normalized = newlinePattern.ReplaceAllString(normalized, "\n")
	}
	return strings.TrimSpace(normalized)
}

func hashText(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:16])
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
