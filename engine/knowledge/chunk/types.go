package chunk

// Document represents raw content prior to chunking.
type Document struct {
	// ID uniquely identifies the source document.
	ID string
	// Text contains the raw content to be chunked.
	Text string
	// Metadata carries domain-specific attributes such as file path or author.
	Metadata map[string]any
}

// Settings configures chunking and preprocessing behavior.
type Settings struct {
	// Strategy determines the chunking algorithm to use (e.g., "fixed" or "sentence").
	Strategy string
	// Size specifies the target chunk length in characters; values must be greater than zero.
	Size int
	// Overlap defines how many characters consecutive chunks share.
	Overlap int
	// RemoveHTML toggles HTML stripping before chunking.
	RemoveHTML bool
	// Deduplicate removes duplicated segments detected during preprocessing.
	Deduplicate bool
	// NormalizeNewlines standardizes newline characters within the source text.
	NormalizeNewlines bool
}

// Chunk represents a processed slice ready for embedding.
type Chunk struct {
	// ID uniquely identifies the chunk, typically derived from the document and position.
	ID string
	// Text holds the processed content ready for embedding.
	Text string
	// Hash stores a deterministic hash of Text for deduplication and caching.
	Hash string
	// Metadata mirrors source document attributes and chunking artifacts.
	Metadata map[string]any
}
