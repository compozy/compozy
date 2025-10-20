package chunk

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/engine/core"
	"github.com/tmc/langchaingo/textsplitter"
)

const (
	StrategyRecursive = "recursive_text_splitter"
)

const (
	minAdaptiveChunkSize = 64
	maxAdaptiveChunkSize = 8192

	shortTextThreshold    = 1500
	longTextThreshold     = 10000
	veryLongTextThreshold = 20000

	headingCharsPerHeading = 400

	overlapDenVeryLong       = 5
	overlapDenLong           = 6
	overlapDenPDF            = 5
	overlapDenMarkdown       = 8
	overlapDenTranscript     = 4
	overlapDenHeadingDensity = 7
)

var (
	newlinePattern = regexp.MustCompile(`\r\n|\r`)
	headingPattern = regexp.MustCompile(`(?m)^#{1,6}\s`)
)

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
	seen := make(map[string]struct{})
	chunks := make([]Chunk, 0, len(docs))
	for di := range docs {
		doc := docs[di]
		text := p.preprocess(doc.Text)
		if text == "" {
			continue
		}
		size, overlap := p.effectiveChunkSettings(doc.Metadata, text)
		splitter := textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(size),
			textsplitter.WithChunkOverlap(overlap),
		)
		segments, err := splitter.SplitText(text)
		if err != nil {
			return nil, fmt.Errorf("chunk: split document %s: %w", doc.ID, err)
		}
		for idx, segment := range segments {
			if ch, ok := p.buildChunk(kbID, doc, idx, segment, seen); ok {
				chunks = append(chunks, ch)
			}
		}
	}
	return chunks, nil
}

func (p *Processor) buildChunk(
	kbID string,
	doc Document,
	idx int,
	segment string,
	seen map[string]struct{},
) (Chunk, bool) {
	chunkText := strings.TrimSpace(segment)
	if chunkText == "" {
		return Chunk{}, false
	}
	hash := hashText(chunkText)
	if p.settings.Deduplicate {
		if _, exists := seen[hash]; exists {
			return Chunk{}, false
		}
		seen[hash] = struct{}{}
	}
	chunkID := hashText(kbID + "::" + doc.ID + "::" + fmt.Sprint(idx) + "::" + hash)
	metadata := core.CloneMap(doc.Metadata)
	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadata["chunk_index"] = idx
	metadata["source_id"] = doc.ID
	return Chunk{
		ID:       chunkID,
		Text:     chunkText,
		Hash:     hash,
		Metadata: metadata,
	}, true
}

func (p *Processor) effectiveChunkSettings(meta map[string]any, text string) (int, int) {
	size := clampChunkSize(p.settings.Size)
	overlap := p.settings.Overlap
	length := utf8.RuneCountInString(text)

	switch {
	case length > veryLongTextThreshold:
		size = clampChunkSize(size * 2)
		overlap = maxInt(overlap, size/overlapDenVeryLong)
	case length > longTextThreshold:
		size = clampChunkSize(size + size/2)
		overlap = maxInt(overlap, size/overlapDenLong)
	case length < shortTextThreshold:
		size = clampChunkSize(maxInt(minAdaptiveChunkSize, size/2))
	}

	contentType := strings.ToLower(metadataString(meta, "content_type"))
	filename := strings.ToLower(metadataString(meta, "filename"))
	sourceType := strings.ToLower(metadataString(meta, "source_type"))

	isPDF := strings.Contains(contentType, "pdf") || strings.HasSuffix(filename, ".pdf")
	isMD := strings.Contains(contentType, "markdown") || strings.HasSuffix(filename, ".md")
	isData := strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "yaml") ||
		strings.Contains(contentType, "toml")

	switch {
	case isPDF:
		size = clampChunkSize(size * 2)
		overlap = maxInt(overlap, size/overlapDenPDF)
	case isMD:
		size = clampChunkSize(maxInt(minAdaptiveChunkSize, (size*3)/4))
		overlap = maxInt(overlap, size/overlapDenMarkdown)
	case isData:
		size = clampChunkSize(maxInt(minAdaptiveChunkSize, size/2))
	}

	if strings.Contains(sourceType, "transcript") || strings.Contains(sourceType, "meeting") {
		size = clampChunkSize(maxInt(minAdaptiveChunkSize, size/2))
		overlap = maxInt(overlap, size/overlapDenTranscript)
	}

	if n := len(headingPattern.FindAllStringIndex(text, -1)); n > 0 && length/n < headingCharsPerHeading {
		size = clampChunkSize(maxInt(minAdaptiveChunkSize, (size*3)/4))
		overlap = maxInt(overlap, size/overlapDenHeadingDensity)
	}

	return size, clampOverlap(overlap, size)
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

func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	value, ok := meta[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func clampChunkSize(size int) int {
	if size < minAdaptiveChunkSize {
		return minAdaptiveChunkSize
	}
	if size > maxAdaptiveChunkSize {
		return maxAdaptiveChunkSize
	}
	return size
}

// clampOverlap ensures overlap is valid for the given size.
// Returns 0 for negative overlap or very small sizes (<=4).
// When overlap >= size, applies a 25% cap (size/4) as an intentional policy
// to provide meaningful overlap while ensuring chunks make forward progress.
// Otherwise, returns overlap unchanged if it's already valid.
func clampOverlap(overlap, size int) int {
	if overlap < 0 {
		return 0
	}
	if overlap >= size {
		if size <= 4 {
			return 0
		}
		// Intentional 25% cap when overlap would equal or exceed size
		return size / 4
	}
	return overlap
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
