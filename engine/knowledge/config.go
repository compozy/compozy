package knowledge

import (
	"context"
	"fmt"
	"strings"

	appconfig "github.com/compozy/compozy/pkg/config"
)

const (
	MinChunkSize     = 64
	MaxChunkSize     = 8192
	maxRetrievalTopK = 50
	MinScoreFloor    = 0.0
	MaxScoreCeiling  = 1.0
)

var builtinDefaults = computeBuiltinDefaults()

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
	return builtinDefaults
}

// DefaultsFromContext retrieves defaults using the application configuration stored in context.
func DefaultsFromContext(ctx context.Context) Defaults {
	return DefaultsFromConfig(appconfig.FromContext(ctx))
}

// DefaultsFromConfig builds Defaults from the global application configuration.
// Invalid values fall back to the built-in defaults to keep normalization predictable.
func DefaultsFromConfig(cfg *appconfig.Config) Defaults {
	if cfg == nil {
		return builtinDefaults
	}
	overrides := defaultsFromKnowledgeConfig(cfg.Knowledge)
	return sanitizeDefaultsWithFallback(overrides, builtinDefaults)
}

func sanitizeDefaults(in Defaults) Defaults {
	return sanitizeDefaultsWithFallback(in, builtinDefaults)
}

func sanitizeDefaultsWithFallback(in Defaults, fallback Defaults) Defaults {
	fb := fallback
	if fb.EmbedderBatchSize <= 0 {
		fb.EmbedderBatchSize = 1
	}
	fb.ChunkSize = clampInt(fb.ChunkSize, MinChunkSize, MaxChunkSize)
	fb.ChunkOverlap = validOverlap(fb.ChunkOverlap, fb.ChunkSize)
	fb.RetrievalTopK = clampInt(fb.RetrievalTopK, 1, maxRetrievalTopK)
	fb.RetrievalMinScore = clampFloat(fb.RetrievalMinScore, MinScoreFloor, MaxScoreCeiling)
	out := Defaults{
		EmbedderBatchSize: in.EmbedderBatchSize,
		ChunkSize:         in.ChunkSize,
		ChunkOverlap:      in.ChunkOverlap,
		RetrievalTopK:     in.RetrievalTopK,
		RetrievalMinScore: in.RetrievalMinScore,
	}
	if out.EmbedderBatchSize <= 0 {
		out.EmbedderBatchSize = fb.EmbedderBatchSize
	}
	if out.EmbedderBatchSize <= 0 {
		out.EmbedderBatchSize = 1
	}
	if out.ChunkSize < MinChunkSize || out.ChunkSize > MaxChunkSize {
		out.ChunkSize = fb.ChunkSize
	}
	out.ChunkSize = clampInt(out.ChunkSize, MinChunkSize, MaxChunkSize)
	if out.ChunkOverlap < 0 || out.ChunkOverlap >= out.ChunkSize {
		out.ChunkOverlap = validOverlap(fb.ChunkOverlap, out.ChunkSize)
	} else {
		out.ChunkOverlap = validOverlap(out.ChunkOverlap, out.ChunkSize)
	}
	if out.RetrievalTopK < 1 || out.RetrievalTopK > maxRetrievalTopK {
		out.RetrievalTopK = fb.RetrievalTopK
	}
	out.RetrievalTopK = clampInt(out.RetrievalTopK, 1, maxRetrievalTopK)
	if out.RetrievalMinScore < MinScoreFloor || out.RetrievalMinScore > MaxScoreCeiling {
		out.RetrievalMinScore = fb.RetrievalMinScore
	}
	out.RetrievalMinScore = clampFloat(out.RetrievalMinScore, MinScoreFloor, MaxScoreCeiling)
	return out
}

func defaultsFromKnowledgeConfig(cfg appconfig.KnowledgeConfig) Defaults {
	return Defaults{
		EmbedderBatchSize: cfg.EmbedderBatchSize,
		ChunkSize:         cfg.ChunkSize,
		ChunkOverlap:      cfg.ChunkOverlap,
		RetrievalTopK:     cfg.RetrievalTopK,
		RetrievalMinScore: cfg.RetrievalMinScore,
	}
}

func computeBuiltinDefaults() Defaults {
	cfg := appconfig.Default()
	raw := defaultsFromKnowledgeConfig(cfg.Knowledge)
	fallback := Defaults{
		EmbedderBatchSize: raw.EmbedderBatchSize,
		ChunkSize:         raw.ChunkSize,
		ChunkOverlap:      raw.ChunkOverlap,
		RetrievalTopK:     raw.RetrievalTopK,
		RetrievalMinScore: raw.RetrievalMinScore,
	}
	return sanitizeDefaultsWithFallback(raw, fallback)
}

func clampInt(value int, lower int, upper int) int {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}

func clampFloat(value float64, lower float64, upper float64) float64 {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}

func validOverlap(overlap int, chunkSize int) int {
	if chunkSize <= 0 {
		return 0
	}
	if overlap < 0 {
		return 0
	}
	if overlap >= chunkSize {
		return chunkSize - 1
	}
	return overlap
}

func ptrInt(v int) *int {
	return &v
}

func ptrFloat64(v float64) *float64 {
	return &v
}

// SourceType identifies the kind of knowledge source available to an ingestion pipeline.
type SourceType string

const (
	SourceTypePDFURL       SourceType = "pdf_url"
	SourceTypeMarkdownGlob SourceType = "markdown_glob"
)

// IngestMode determines when a knowledge base ingestion pipeline should run.
type IngestMode string

const (
	IngestManual  IngestMode = "manual"
	IngestOnStart IngestMode = "on_start"
)

// ChunkStrategy enumerates supported approaches to splitting content into chunks.
type ChunkStrategy string

const (
	ChunkStrategyRecursiveTextSplitter ChunkStrategy = "recursive_text_splitter"
)

// VectorDBType classifies the supported vector database backends.
type VectorDBType string

const (
	VectorDBTypePGVector   VectorDBType = "pgvector"
	VectorDBTypeQdrant     VectorDBType = "qdrant"
	VectorDBTypeRedis      VectorDBType = "redis"
	VectorDBTypeFilesystem VectorDBType = "filesystem"
)

// Definitions aggregates embedders, vector stores, and knowledge bases declared by users.
type Definitions struct {
	Embedders      []EmbedderConfig `json:"embedders"       yaml:"embedders"       mapstructure:"embedders"`
	VectorDBs      []VectorDBConfig `json:"vector_dbs"      yaml:"vector_dbs"      mapstructure:"vector_dbs"`
	KnowledgeBases []BaseConfig     `json:"knowledge_bases" yaml:"knowledge_bases" mapstructure:"knowledge_bases"`
}

// EmbedderConfig describes an embedding provider used during knowledge ingestion.
type EmbedderConfig struct {
	ID       string                `json:"id"                yaml:"id"                mapstructure:"id"`
	Provider string                `json:"provider"          yaml:"provider"          mapstructure:"provider"`
	Model    string                `json:"model"             yaml:"model"             mapstructure:"model"`
	APIKey   string                `json:"api_key,omitempty" yaml:"api_key,omitempty" mapstructure:"api_key,omitempty"`
	Config   EmbedderRuntimeConfig `json:"config"            yaml:"config"            mapstructure:"config"`
}

// EmbedderRuntimeConfig captures runtime tuning options for an embedder client.
type EmbedderRuntimeConfig struct {
	Dimension     int            `json:"dimension"                yaml:"dimension"                mapstructure:"dimension"`
	BatchSize     int            `json:"batch_size,omitempty"     yaml:"batch_size,omitempty"     mapstructure:"batch_size,omitempty"`
	StripNewLines *bool          `json:"strip_newlines,omitempty" yaml:"strip_newlines,omitempty" mapstructure:"strip_newlines,omitempty"`
	Retry         map[string]any `json:"retry,omitempty"          yaml:"retry,omitempty"          mapstructure:"retry,omitempty"`
}

// VectorDBConfig configures a vector database target for knowledge storage.
type VectorDBConfig struct {
	ID     string             `json:"id"     yaml:"id"     mapstructure:"id"`
	Type   VectorDBType       `json:"type"   yaml:"type"   mapstructure:"type"`
	Config VectorDBConnConfig `json:"config" yaml:"config" mapstructure:"config"`
}

// VectorDBConnConfig defines connection and table options for a vector database.
type VectorDBConnConfig struct {
	DSN         string            `json:"dsn,omitempty"          yaml:"dsn,omitempty"          mapstructure:"dsn,omitempty"`
	Path        string            `json:"path,omitempty"         yaml:"path,omitempty"         mapstructure:"path,omitempty"`
	Table       string            `json:"table,omitempty"        yaml:"table,omitempty"        mapstructure:"table,omitempty"`
	Collection  string            `json:"collection,omitempty"   yaml:"collection,omitempty"   mapstructure:"collection,omitempty"`
	Index       string            `json:"index,omitempty"        yaml:"index,omitempty"        mapstructure:"index,omitempty"`
	EnsureIndex bool              `json:"ensure_index,omitempty" yaml:"ensure_index,omitempty" mapstructure:"ensure_index,omitempty"`
	Metric      string            `json:"metric,omitempty"       yaml:"metric,omitempty"       mapstructure:"metric,omitempty"`
	Dimension   int               `json:"dimension,omitempty"    yaml:"dimension,omitempty"    mapstructure:"dimension,omitempty"`
	Consistency string            `json:"consistency,omitempty"  yaml:"consistency,omitempty"  mapstructure:"consistency,omitempty"`
	Auth        map[string]string `json:"auth,omitempty"         yaml:"auth,omitempty"         mapstructure:"auth,omitempty"`
	MaxTopK     int               `json:"max_top_k,omitempty"    yaml:"max_top_k,omitempty"    mapstructure:"max_top_k,omitempty"`
}

// BaseConfig declares a knowledge base and governs how it is ingested and retrieved.
type BaseConfig struct {
	ID          string           `json:"id"                    yaml:"id"                    mapstructure:"id"`
	Description string           `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`
	Embedder    string           `json:"embedder"              yaml:"embedder"              mapstructure:"embedder"`
	VectorDB    string           `json:"vector_db"             yaml:"vector_db"             mapstructure:"vector_db"`
	Ingest      IngestMode       `json:"ingest,omitempty"      yaml:"ingest,omitempty"      mapstructure:"ingest,omitempty"`
	Sources     []SourceConfig   `json:"sources"               yaml:"sources"               mapstructure:"sources"`
	Chunking    ChunkingConfig   `json:"chunking,omitempty"    yaml:"chunking,omitempty"    mapstructure:"chunking,omitempty"`
	Preprocess  PreprocessConfig `json:"preprocess,omitempty"  yaml:"preprocess,omitempty"  mapstructure:"preprocess,omitempty"`
	Retrieval   RetrievalConfig  `json:"retrieval,omitempty"   yaml:"retrieval,omitempty"   mapstructure:"retrieval,omitempty"`
	Metadata    MetadataConfig   `json:"metadata,omitempty"    yaml:"metadata,omitempty"    mapstructure:"metadata,omitempty"`
}

// SourceConfig describes a single ingestion source such as a glob, URL, or bucket.
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

// ChunkingConfig tunes how documents are split before embedding.
type ChunkingConfig struct {
	Strategy ChunkStrategy `json:"strategy,omitempty" yaml:"strategy,omitempty" mapstructure:"strategy,omitempty"`
	Size     int           `json:"size,omitempty"     yaml:"size,omitempty"     mapstructure:"size,omitempty"`
	Overlap  *int          `json:"overlap,omitempty"  yaml:"overlap,omitempty"  mapstructure:"overlap,omitempty"`
}

// PreprocessConfig configures preprocessing steps applied to raw content.
type PreprocessConfig struct {
	Deduplicate *bool `json:"dedupe,omitempty"      yaml:"dedupe,omitempty"      mapstructure:"dedupe,omitempty"`
	RemoveHTML  bool  `json:"remove_html,omitempty" yaml:"remove_html,omitempty" mapstructure:"remove_html,omitempty"`
}

// RetrievalConfig manages how stored chunks are queried and injected into prompts.
type RetrievalConfig struct {
	TopK         int               `json:"top_k,omitempty"         yaml:"top_k,omitempty"         mapstructure:"top_k,omitempty"`
	MinScore     *float64          `json:"min_score,omitempty"     yaml:"min_score,omitempty"     mapstructure:"min_score,omitempty"`
	MaxTokens    int               `json:"max_tokens,omitempty"    yaml:"max_tokens,omitempty"    mapstructure:"max_tokens,omitempty"`
	MinResults   int               `json:"min_results,omitempty"   yaml:"min_results,omitempty"   mapstructure:"min_results,omitempty"`
	InjectAs     string            `json:"inject_as,omitempty"     yaml:"inject_as,omitempty"     mapstructure:"inject_as,omitempty"`
	Fallback     string            `json:"fallback,omitempty"      yaml:"fallback,omitempty"      mapstructure:"fallback,omitempty"`
	Filters      map[string]string `json:"filters,omitempty"       yaml:"filters,omitempty"       mapstructure:"filters,omitempty"`
	ToolFallback ToolFallbackMode  `json:"tool_fallback,omitempty" yaml:"tool_fallback,omitempty" mapstructure:"tool_fallback,omitempty"`
}

// ToolFallbackMode governs how the orchestrator should expose tools when retrieval fails.
type ToolFallbackMode string

const (
	ToolFallbackNever    ToolFallbackMode = "never"
	ToolFallbackEscalate ToolFallbackMode = "escalate"
	ToolFallbackAuto     ToolFallbackMode = "auto"
)

// MetadataConfig carries optional descriptive metadata for knowledge bases.
type MetadataConfig struct {
	Tags   []string `json:"tags,omitempty"   yaml:"tags,omitempty"   mapstructure:"tags,omitempty"`
	Owners []string `json:"owners,omitempty" yaml:"owners,omitempty" mapstructure:"owners,omitempty"`
}

// OverlapValue returns the configured chunk overlap or zero when unset.
func (c ChunkingConfig) OverlapValue() int {
	if c.Overlap == nil {
		return 0
	}
	return *c.Overlap
}

func (c *ChunkingConfig) setOverlap(value int) {
	c.Overlap = ptrInt(value)
}

// MinScoreValue returns the retrieval minimum score or zero when unspecified.
func (c *RetrievalConfig) MinScoreValue() float64 {
	if c == nil || c.MinScore == nil {
		return 0
	}
	return *c.MinScore
}

func (c *RetrievalConfig) setMinScore(value float64) {
	if c == nil {
		return
	}
	c.MinScore = ptrFloat64(value)
}

// MinResultsValue returns the minimum retrieval results treated as success.
func (c *RetrievalConfig) MinResultsValue() int {
	if c == nil || c.MinResults <= 0 {
		return 1
	}
	return c.MinResults
}

// Normalize applies built-in defaults to the configured definitions.
func (d *Definitions) Normalize() {
	d.NormalizeWithDefaults(DefaultDefaults())
}

// NormalizeWithDefaults applies the supplied defaults when normalizing definitions.
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
	if c.Ingest == "" {
		c.Ingest = IngestManual
	}
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
	if c.Overlap == nil {
		c.setOverlap(defaults.ChunkOverlap)
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
	if c.MinScore == nil {
		c.setMinScore(defaults.RetrievalMinScore)
	}
	if c.MinResults <= 0 {
		c.MinResults = 1
	}
	if c.ToolFallback == "" {
		c.ToolFallback = ToolFallbackNever
	}
}

// Validate checks definitions for consistency and aggregates any validation errors.
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
	return NewValidationErrors(errs...)
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
		errs = append(errs, validateVectorProvider(vector)...)
		index[vector.ID] = vector
	}
	return index, errs
}

func validateVectorProvider(vector *VectorDBConfig) []error {
	if vector == nil {
		return []error{fmt.Errorf("knowledge: vector_db config cannot be nil")}
	}
	switch vector.Type {
	case VectorDBTypePGVector:
		return validatePGVectorConfig(vector)
	case VectorDBTypeQdrant:
		return validateQdrantConfig(vector)
	case VectorDBTypeRedis:
		return validateRedisConfig(vector)
	case VectorDBTypeFilesystem:
		return validateFilesystemConfig(vector)
	default:
		return []error{fmt.Errorf("knowledge: vector_db %q type %q is not supported", vector.ID, vector.Type)}
	}
}

func validatePGVectorConfig(vector *VectorDBConfig) []error {
	dsn := strings.TrimSpace(vector.Config.DSN)
	if dsn != vector.Config.DSN {
		vector.Config.DSN = dsn
	}
	var errs []error
	if dsn != "" && !isTemplatedValue(dsn) {
		errs = append(
			errs,
			fmt.Errorf("knowledge: vector_db %q dsn must use env or secret interpolation", vector.ID),
		)
	}
	if vector.Config.Dimension <= 0 {
		errs = append(
			errs,
			fmt.Errorf("knowledge: vector_db %q config.dimension must be greater than zero", vector.ID),
		)
	}
	return errs
}

func validateQdrantConfig(vector *VectorDBConfig) []error {
	dsn := strings.TrimSpace(vector.Config.DSN)
	if dsn != vector.Config.DSN {
		vector.Config.DSN = dsn
	}
	var errs []error
	if dsn == "" {
		errs = append(errs, fmt.Errorf("knowledge: vector_db %q requires config.dsn", vector.ID))
	} else if !isTemplatedValue(dsn) {
		errs = append(errs, fmt.Errorf("knowledge: vector_db %q dsn must use env or secret interpolation", vector.ID))
	}
	if vector.Config.Dimension <= 0 {
		errs = append(
			errs,
			fmt.Errorf("knowledge: vector_db %q config.dimension must be greater than zero", vector.ID),
		)
	}
	return errs
}

func validateRedisConfig(vector *VectorDBConfig) []error {
	dsn := strings.TrimSpace(vector.Config.DSN)
	if dsn != vector.Config.DSN {
		vector.Config.DSN = dsn
	}
	var errs []error
	if dsn != "" && !isTemplatedValue(dsn) {
		errs = append(errs, fmt.Errorf("knowledge: vector_db %q dsn must use env or secret interpolation", vector.ID))
	}
	if vector.Config.Dimension <= 0 {
		errs = append(
			errs,
			fmt.Errorf("knowledge: vector_db %q config.dimension must be greater than zero", vector.ID),
		)
	}
	return errs
}

func validateFilesystemConfig(vector *VectorDBConfig) []error {
	if vector.Config.Dimension <= 0 {
		return []error{
			fmt.Errorf("knowledge: vector_db %q config.dimension must be greater than zero", vector.ID),
		}
	}
	return nil
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
	errs = append(errs, validateKnowledgeBaseIngest(kb)...)
	errs = append(errs, validateKnowledgeBaseReferences(kb, embedder, vector)...)
	errs = append(errs, validateKnowledgeBaseSources(kb)...)
	errs = append(errs, validateKnowledgeBaseChunking(kb)...)
	errs = append(errs, validateKnowledgeBaseRetrieval(kb)...)
	return errs
}

func validateKnowledgeBaseIngest(kb *BaseConfig) []error {
	switch kb.Ingest {
	case IngestManual, IngestOnStart:
		return nil
	default:
		return []error{fmt.Errorf(
			"knowledge: knowledge_base %q ingest must be one of %q or %q",
			kb.ID,
			IngestManual,
			IngestOnStart,
		)}
	}
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
	overlap := kb.Chunking.OverlapValue()
	if overlap < 0 {
		errs = append(errs, fmt.Errorf("knowledge: knowledge_base %q chunking.overlap must be >= 0", kb.ID))
	} else if overlap >= kb.Chunking.Size {
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
	minScore := kb.Retrieval.MinScoreValue()
	if minScore < MinScoreFloor || minScore > MaxScoreCeiling {
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
	default:
		return fmt.Errorf("knowledge: knowledge_base %q source type %q is not supported", kbID, source.Type)
	}
	return nil
}

func isTemplatedValue(val string) bool {
	return strings.Contains(val, "{{") && strings.Contains(val, "}}")
}
