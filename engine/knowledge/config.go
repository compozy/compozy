package knowledge

import (
	"fmt"
	"strings"

	appconfig "github.com/compozy/compozy/pkg/config"
)

const (
	DefaultEmbedderBatchSize = 512
	DefaultChunkSize         = 800
	DefaultChunkOverlap      = 120
	MinChunkSize             = 64
	MaxChunkSize             = 8192
	DefaultRetrievalTopK     = 5
	maxRetrievalTopK         = 50
	MinScoreFloor            = 0.0
	MaxScoreCeiling          = 1.0
)

// Defaults captures global defaults used during knowledge normalization.
type Defaults struct {
	EmbedderBatchSize int
	ChunkSize         int
	ChunkOverlap      int
	RetrievalTopK     int
	RetrievalMinScore float64
}

// DefaultDefaults returns the built-in defaults used when no configuration override is supplied.
func DefaultDefaults() Defaults {
	return Defaults{
		EmbedderBatchSize: DefaultEmbedderBatchSize,
		ChunkSize:         DefaultChunkSize,
		ChunkOverlap:      DefaultChunkOverlap,
		RetrievalTopK:     DefaultRetrievalTopK,
		RetrievalMinScore: MinScoreFloor,
	}
}

// DefaultsFromConfig builds Defaults from the global application configuration.
// Invalid values fall back to the built-in defaults to keep normalization predictable.
func DefaultsFromConfig(cfg *appconfig.Config) Defaults {
	defaults := DefaultDefaults()
	if cfg == nil {
		return defaults
	}
	settings := cfg.Knowledge
	if settings.EmbedderBatchSize > 0 {
		defaults.EmbedderBatchSize = settings.EmbedderBatchSize
	}
	if settings.ChunkSize >= MinChunkSize && settings.ChunkSize <= MaxChunkSize {
		defaults.ChunkSize = settings.ChunkSize
	}
	if settings.ChunkOverlap >= 0 && settings.ChunkOverlap < defaults.ChunkSize {
		defaults.ChunkOverlap = settings.ChunkOverlap
	}
	if settings.RetrievalTopK >= 1 && settings.RetrievalTopK <= maxRetrievalTopK {
		defaults.RetrievalTopK = settings.RetrievalTopK
	}
	if settings.RetrievalMinScore >= MinScoreFloor && settings.RetrievalMinScore <= MaxScoreCeiling {
		defaults.RetrievalMinScore = settings.RetrievalMinScore
	}
	return sanitizeDefaults(defaults)
}

func sanitizeDefaults(in Defaults) Defaults {
	out := in
	if out.EmbedderBatchSize <= 0 {
		out.EmbedderBatchSize = DefaultEmbedderBatchSize
	}
	if out.ChunkSize < MinChunkSize || out.ChunkSize > MaxChunkSize {
		out.ChunkSize = DefaultChunkSize
	}
	if out.ChunkOverlap < 0 || out.ChunkOverlap >= out.ChunkSize {
		out.ChunkOverlap = DefaultChunkOverlap
	}
	if out.RetrievalTopK < 1 || out.RetrievalTopK > maxRetrievalTopK {
		out.RetrievalTopK = DefaultRetrievalTopK
	}
	if out.RetrievalMinScore < MinScoreFloor || out.RetrievalMinScore > MaxScoreCeiling {
		out.RetrievalMinScore = MinScoreFloor
	}
	return out
}

type SourceType string

const (
	SourceTypePDFURL          SourceType = "pdf_url"
	SourceTypeMarkdownGlob    SourceType = "markdown_glob"
	SourceTypeCloudStorage    SourceType = "cloud_storage"
	SourceTypeMediaTranscript SourceType = "media_transcript"
)

type ChunkStrategy string

const (
	ChunkStrategyRecursiveTextSplitter ChunkStrategy = "recursive_text_splitter"
)

type VectorDBType string

const (
	VectorDBTypePGVector VectorDBType = "pgvector"
	VectorDBTypeQdrant   VectorDBType = "qdrant"
	VectorDBTypeMemory   VectorDBType = "memory"
)

type Definitions struct {
	Embedders      []EmbedderConfig `json:"embedders"       yaml:"embedders"       mapstructure:"embedders"`
	VectorDBs      []VectorDBConfig `json:"vector_dbs"      yaml:"vector_dbs"      mapstructure:"vector_dbs"`
	KnowledgeBases []BaseConfig     `json:"knowledge_bases" yaml:"knowledge_bases" mapstructure:"knowledge_bases"`
}

type EmbedderConfig struct {
	ID       string                `json:"id"                yaml:"id"                mapstructure:"id"`
	Provider string                `json:"provider"          yaml:"provider"          mapstructure:"provider"`
	Model    string                `json:"model"             yaml:"model"             mapstructure:"model"`
	APIKey   string                `json:"api_key,omitempty" yaml:"api_key,omitempty" mapstructure:"api_key,omitempty"`
	Config   EmbedderRuntimeConfig `json:"config"            yaml:"config"            mapstructure:"config"`
}

type EmbedderRuntimeConfig struct {
	Dimension     int            `json:"dimension"                yaml:"dimension"                mapstructure:"dimension"`
	BatchSize     int            `json:"batch_size,omitempty"     yaml:"batch_size,omitempty"     mapstructure:"batch_size,omitempty"`
	StripNewLines *bool          `json:"strip_newlines,omitempty" yaml:"strip_newlines,omitempty" mapstructure:"strip_newlines,omitempty"`
	Retry         map[string]any `json:"retry,omitempty"          yaml:"retry,omitempty"          mapstructure:"retry,omitempty"`
}

type VectorDBConfig struct {
	ID     string             `json:"id"     yaml:"id"     mapstructure:"id"`
	Type   VectorDBType       `json:"type"   yaml:"type"   mapstructure:"type"`
	Config VectorDBConnConfig `json:"config" yaml:"config" mapstructure:"config"`
}

type VectorDBConnConfig struct {
	DSN         string            `json:"dsn,omitempty"         yaml:"dsn,omitempty"         mapstructure:"dsn,omitempty"`
	Table       string            `json:"table,omitempty"       yaml:"table,omitempty"       mapstructure:"table,omitempty"`
	Index       string            `json:"index,omitempty"       yaml:"index,omitempty"       mapstructure:"index,omitempty"`
	Metric      string            `json:"metric,omitempty"      yaml:"metric,omitempty"      mapstructure:"metric,omitempty"`
	Dimension   int               `json:"dimension,omitempty"   yaml:"dimension,omitempty"   mapstructure:"dimension,omitempty"`
	Consistency string            `json:"consistency,omitempty" yaml:"consistency,omitempty" mapstructure:"consistency,omitempty"`
	Auth        map[string]string `json:"auth,omitempty"        yaml:"auth,omitempty"        mapstructure:"auth,omitempty"`
}

type BaseConfig struct {
	ID          string           `json:"id"                    yaml:"id"                    mapstructure:"id"`
	Description string           `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`
	Embedder    string           `json:"embedder"              yaml:"embedder"              mapstructure:"embedder"`
	VectorDB    string           `json:"vector_db"             yaml:"vector_db"             mapstructure:"vector_db"`
	Sources     []SourceConfig   `json:"sources"               yaml:"sources"               mapstructure:"sources"`
	Chunking    ChunkingConfig   `json:"chunking,omitempty"    yaml:"chunking,omitempty"    mapstructure:"chunking,omitempty"`
	Preprocess  PreprocessConfig `json:"preprocess,omitempty"  yaml:"preprocess,omitempty"  mapstructure:"preprocess,omitempty"`
	Retrieval   RetrievalConfig  `json:"retrieval,omitempty"   yaml:"retrieval,omitempty"   mapstructure:"retrieval,omitempty"`
	Metadata    MetadataConfig   `json:"metadata,omitempty"    yaml:"metadata,omitempty"    mapstructure:"metadata,omitempty"`
}

type SourceConfig struct {
	Type     SourceType        `json:"type"               yaml:"type"               mapstructure:"type"`
	Paths    []string          `json:"paths,omitempty"    yaml:"paths,omitempty"    mapstructure:"paths,omitempty"`
	Path     string            `json:"path,omitempty"     yaml:"path,omitempty"     mapstructure:"path,omitempty"`
	URLs     []string          `json:"urls,omitempty"     yaml:"urls,omitempty"     mapstructure:"urls,omitempty"`
	Provider string            `json:"provider,omitempty" yaml:"provider,omitempty" mapstructure:"provider,omitempty"`
	Bucket   string            `json:"bucket,omitempty"   yaml:"bucket,omitempty"   mapstructure:"bucket,omitempty"`
	Prefix   string            `json:"prefix,omitempty"   yaml:"prefix,omitempty"   mapstructure:"prefix,omitempty"`
	VideoID  string            `json:"video_id,omitempty" yaml:"video_id,omitempty" mapstructure:"video_id,omitempty"`
	Options  map[string]string `json:"options,omitempty"  yaml:"options,omitempty"  mapstructure:"options,omitempty"`
}

type ChunkingConfig struct {
	Strategy ChunkStrategy `json:"strategy,omitempty" yaml:"strategy,omitempty" mapstructure:"strategy,omitempty"`
	Size     int           `json:"size,omitempty"     yaml:"size,omitempty"     mapstructure:"size,omitempty"`
	Overlap  int           `json:"overlap,omitempty"  yaml:"overlap,omitempty"  mapstructure:"overlap,omitempty"`
}

type PreprocessConfig struct {
	Deduplicate *bool `json:"dedupe,omitempty"      yaml:"dedupe,omitempty"      mapstructure:"dedupe,omitempty"`
	RemoveHTML  bool  `json:"remove_html,omitempty" yaml:"remove_html,omitempty" mapstructure:"remove_html,omitempty"`
}

type RetrievalConfig struct {
	TopK      int               `json:"top_k,omitempty"      yaml:"top_k,omitempty"      mapstructure:"top_k,omitempty"`
	MinScore  float64           `json:"min_score,omitempty"  yaml:"min_score,omitempty"  mapstructure:"min_score,omitempty"`
	MaxTokens int               `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty" mapstructure:"max_tokens,omitempty"`
	InjectAs  string            `json:"inject_as,omitempty"  yaml:"inject_as,omitempty"  mapstructure:"inject_as,omitempty"`
	Fallback  string            `json:"fallback,omitempty"   yaml:"fallback,omitempty"   mapstructure:"fallback,omitempty"`
	Filters   map[string]string `json:"filters,omitempty"    yaml:"filters,omitempty"    mapstructure:"filters,omitempty"`
}

type MetadataConfig struct {
	Tags   []string `json:"tags,omitempty"   yaml:"tags,omitempty"   mapstructure:"tags,omitempty"`
	Owners []string `json:"owners,omitempty" yaml:"owners,omitempty" mapstructure:"owners,omitempty"`
}

func (d *Definitions) Normalize() {
	d.NormalizeWithDefaults(DefaultDefaults())
}

func (d *Definitions) NormalizeWithDefaults(defaults Defaults) {
	defaults = sanitizeDefaults(defaults)
	for i := range d.Embedders {
		d.Embedders[i].normalize(defaults)
	}
	for i := range d.VectorDBs {
		d.VectorDBs[i].normalize()
	}
	for i := range d.KnowledgeBases {
		d.KnowledgeBases[i].normalize(defaults)
	}
}

func (c *EmbedderConfig) normalize(defaults Defaults) {
	if c.Config.BatchSize <= 0 {
		c.Config.BatchSize = defaults.EmbedderBatchSize
	}
	if c.Config.StripNewLines == nil {
		val := true
		c.Config.StripNewLines = &val
	}
}

func (c *VectorDBConfig) normalize() {}

func (c *BaseConfig) normalize(defaults Defaults) {
	c.Chunking.normalize(defaults)
	c.Preprocess.normalize()
	c.Retrieval.normalize(defaults)
}

func (c *ChunkingConfig) normalize(defaults Defaults) {
	if c.Strategy == "" {
		c.Strategy = ChunkStrategyRecursiveTextSplitter
	}
	if c.Size == 0 {
		c.Size = defaults.ChunkSize
	}
	if c.Overlap == 0 {
		c.Overlap = defaults.ChunkOverlap
	}
}

func (c *PreprocessConfig) normalize() {
	if c.Deduplicate == nil {
		val := true
		c.Deduplicate = &val
	}
}

func (c *RetrievalConfig) normalize(defaults Defaults) {
	if c.TopK <= 0 {
		c.TopK = defaults.RetrievalTopK
	}
	if c.MinScore == 0 {
		c.MinScore = defaults.RetrievalMinScore
	}
}

func (d *Definitions) Validate() error {
	embedderIndex, embedderErrs := validateEmbedders(d.Embedders)
	vectorIndex, vectorErrs := validateVectorDBs(d.VectorDBs)
	kbErrs := validateKnowledgeBases(d.KnowledgeBases, embedderIndex, vectorIndex)

	totalErrs := len(embedderErrs) + len(vectorErrs) + len(kbErrs)
	errs := make([]error, 0, totalErrs)
	errs = append(errs, embedderErrs...)
	errs = append(errs, vectorErrs...)
	errs = append(errs, kbErrs...)
	if len(errs) == 0 {
		return nil
	}
	return ValidationErrors{errors: errs}
}

func validateEmbedders(list []EmbedderConfig) (map[string]*EmbedderConfig, []error) {
	index := make(map[string]*EmbedderConfig, len(list))
	var errs []error
	for i := range list {
		embedder := &list[i]
		if embedder.ID == "" {
			errs = append(errs, fmt.Errorf("knowledge: embedder id is required"))
			continue
		}
		if _, exists := index[embedder.ID]; exists {
			errs = append(errs, fmt.Errorf("knowledge: embedder %q defined more than once", embedder.ID))
			continue
		}
		if embedder.Provider == "" {
			errs = append(errs, fmt.Errorf("knowledge: embedder %q provider is required", embedder.ID))
		}
		if embedder.Model == "" {
			errs = append(errs, fmt.Errorf("knowledge: embedder %q model is required", embedder.ID))
		}
		if embedder.APIKey != "" && !isTemplatedValue(embedder.APIKey) {
			errs = append(
				errs,
				fmt.Errorf("knowledge: embedder %q api_key must use env or secret interpolation", embedder.ID),
			)
		}
		if embedder.Config.Dimension <= 0 {
			errs = append(
				errs,
				fmt.Errorf("knowledge: embedder %q config.dimension must be greater than zero", embedder.ID),
			)
		}
		if embedder.Config.BatchSize <= 0 {
			errs = append(
				errs,
				fmt.Errorf("knowledge: embedder %q config.batch_size must be greater than zero", embedder.ID),
			)
		}
		index[embedder.ID] = embedder
	}
	return index, errs
}

func validateVectorDBs(list []VectorDBConfig) (map[string]*VectorDBConfig, []error) {
	index := make(map[string]*VectorDBConfig, len(list))
	var errs []error
	for i := range list {
		vector := &list[i]
		if vector.ID == "" {
			errs = append(errs, fmt.Errorf("knowledge: vector_db id is required"))
			continue
		}
		if _, exists := index[vector.ID]; exists {
			errs = append(errs, fmt.Errorf("knowledge: vector_db %q defined more than once", vector.ID))
			continue
		}
		switch vector.Type {
		case VectorDBTypePGVector, VectorDBTypeQdrant:
			if vector.Config.DSN == "" {
				errs = append(errs, fmt.Errorf("knowledge: vector_db %q requires config.dsn", vector.ID))
			} else if !isTemplatedValue(vector.Config.DSN) {
				errs = append(errs, fmt.Errorf("knowledge: vector_db %q dsn must use env or secret interpolation", vector.ID))
			}
			if vector.Config.Dimension <= 0 {
				errs = append(
					errs,
					fmt.Errorf("knowledge: vector_db %q config.dimension must be greater than zero", vector.ID),
				)
			}
		case VectorDBTypeMemory:
		default:
			errs = append(errs, fmt.Errorf("knowledge: vector_db %q type %q is not supported", vector.ID, vector.Type))
		}
		index[vector.ID] = vector
	}
	return index, errs
}

func validateKnowledgeBases(
	list []BaseConfig,
	embedderIndex map[string]*EmbedderConfig,
	vectorIndex map[string]*VectorDBConfig,
) []error {
	seen := make(map[string]struct{}, len(list))
	out := make([]error, 0)
	for i := range list {
		kb := &list[i]
		if kb.ID == "" {
			out = append(out, fmt.Errorf("knowledge: knowledge_base id is required"))
			continue
		}
		if _, exists := seen[kb.ID]; exists {
			out = append(out, fmt.Errorf("knowledge: knowledge_base %q defined more than once", kb.ID))
			continue
		}
		seen[kb.ID] = struct{}{}
		embedder := embedderIndex[kb.Embedder]
		vector := vectorIndex[kb.VectorDB]
		out = append(out, validateKnowledgeBase(kb, embedder, vector)...)
	}
	return out
}

func validateKnowledgeBase(
	kb *BaseConfig,
	embedder *EmbedderConfig,
	vector *VectorDBConfig,
) []error {
	errs := make([]error, 0, 10)
	errs = append(errs, validateKnowledgeBaseReferences(kb, embedder, vector)...)
	errs = append(errs, validateKnowledgeBaseSources(kb)...)
	errs = append(errs, validateKnowledgeBaseChunking(kb)...)
	errs = append(errs, validateKnowledgeBaseRetrieval(kb)...)
	return errs
}

func validateKnowledgeBaseReferences(
	kb *BaseConfig,
	embedder *EmbedderConfig,
	vector *VectorDBConfig,
) []error {
	errs := make([]error, 0, 4)
	if kb.Embedder == "" {
		errs = append(errs, fmt.Errorf("knowledge: knowledge_base %q embedder is required", kb.ID))
	}
	if kb.VectorDB == "" {
		errs = append(errs, fmt.Errorf("knowledge: knowledge_base %q vector_db is required", kb.ID))
	}
	if embedder == nil && kb.Embedder != "" {
		errs = append(
			errs,
			fmt.Errorf("knowledge: knowledge_base %q references unknown embedder %q", kb.ID, kb.Embedder),
		)
	}
	if vector == nil && kb.VectorDB != "" {
		errs = append(
			errs,
			fmt.Errorf("knowledge: knowledge_base %q references unknown vector_db %q", kb.ID, kb.VectorDB),
		)
	}
	if embedder != nil && vector != nil && embedder.Config.Dimension != vector.Config.Dimension {
		errs = append(errs, fmt.Errorf(
			"knowledge: knowledge_base %q embedder dimension %d != vector_db dimension %d",
			kb.ID,
			embedder.Config.Dimension,
			vector.Config.Dimension,
		))
	}
	return errs
}

func validateKnowledgeBaseSources(kb *BaseConfig) []error {
	if len(kb.Sources) == 0 {
		return []error{fmt.Errorf("knowledge: knowledge_base %q must define at least one source", kb.ID)}
	}
	errs := make([]error, 0, len(kb.Sources))
	for j := range kb.Sources {
		if err := validateSource(kb.ID, &kb.Sources[j]); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func validateKnowledgeBaseChunking(kb *BaseConfig) []error {
	errs := make([]error, 0, 2)
	if kb.Chunking.Size < MinChunkSize || kb.Chunking.Size > MaxChunkSize {
		errs = append(errs, fmt.Errorf(
			"knowledge: knowledge_base %q chunking.size must be in [%d,%d]",
			kb.ID,
			MinChunkSize,
			MaxChunkSize,
		))
	}
	if kb.Chunking.Overlap < 0 {
		errs = append(errs, fmt.Errorf("knowledge: knowledge_base %q chunking.overlap must be >= 0", kb.ID))
	} else if kb.Chunking.Overlap >= kb.Chunking.Size {
		errs = append(errs, fmt.Errorf("knowledge: knowledge_base %q chunking.overlap must be < chunking.size", kb.ID))
	}
	return errs
}

func validateKnowledgeBaseRetrieval(kb *BaseConfig) []error {
	errs := make([]error, 0, 3)
	if kb.Retrieval.TopK < 1 || kb.Retrieval.TopK > maxRetrievalTopK {
		errs = append(errs, fmt.Errorf(
			"knowledge: knowledge_base %q retrieval.top_k must be between 1 and %d",
			kb.ID,
			maxRetrievalTopK,
		))
	}
	if kb.Retrieval.MinScore < MinScoreFloor || kb.Retrieval.MinScore > MaxScoreCeiling {
		errs = append(errs, fmt.Errorf(
			"knowledge: knowledge_base %q retrieval.min_score must be within [%.2f, %.2f]",
			kb.ID,
			MinScoreFloor,
			MaxScoreCeiling,
		))
	}
	if kb.Retrieval.MaxTokens < 0 {
		errs = append(errs, fmt.Errorf("knowledge: knowledge_base %q retrieval.max_tokens cannot be negative", kb.ID))
	}
	return errs
}

func validateSource(kbID string, source *SourceConfig) error {
	switch source.Type {
	case SourceTypePDFURL:
		if len(source.URLs) == 0 {
			return fmt.Errorf("knowledge: knowledge_base %q pdf_url source requires urls", kbID)
		}
	case SourceTypeMarkdownGlob:
		if source.Path == "" && len(source.Paths) == 0 {
			return fmt.Errorf("knowledge: knowledge_base %q markdown_glob source requires path or paths", kbID)
		}
	case SourceTypeCloudStorage:
		if source.Provider == "" {
			return fmt.Errorf("knowledge: knowledge_base %q cloud_storage source requires provider", kbID)
		}
		if source.Bucket == "" {
			return fmt.Errorf("knowledge: knowledge_base %q cloud_storage source requires bucket", kbID)
		}
		if source.Prefix == "" {
			return fmt.Errorf("knowledge: knowledge_base %q cloud_storage source requires prefix", kbID)
		}
	case SourceTypeMediaTranscript:
		if source.Provider == "" {
			return fmt.Errorf("knowledge: knowledge_base %q media_transcript source requires provider", kbID)
		}
		if source.VideoID == "" {
			return fmt.Errorf("knowledge: knowledge_base %q media_transcript source requires video_id", kbID)
		}
	default:
		return fmt.Errorf("knowledge: knowledge_base %q source type %q is not supported", kbID, source.Type)
	}
	return nil
}

func isTemplatedValue(val string) bool {
	return strings.Contains(val, "{{") && strings.Contains(val, "}}")
}
