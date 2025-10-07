package chunk

// Document represents raw content prior to chunking.
type Document struct {
	ID       string
	Text     string
	Metadata map[string]any
}

// Settings configures chunking and preprocessing behavior.
type Settings struct {
	Strategy          string
	Size              int
	Overlap           int
	RemoveHTML        bool
	Deduplicate       bool
	NormalizeNewlines bool
}

// Chunk represents a processed slice ready for embedding.
type Chunk struct {
	ID       string
	Text     string
	Hash     string
	Metadata map[string]any
}
