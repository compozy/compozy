package knowledge

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const (
	// VectorDBTypeChroma identifies the Chroma vector database backend.
	VectorDBTypeChroma VectorDBType = "chroma"
	// VectorDBTypeWeaviate identifies the Weaviate vector database backend.
	VectorDBTypeWeaviate VectorDBType = "weaviate"
	// VectorDBTypeMilvus identifies the Milvus vector database backend.
	VectorDBTypeMilvus VectorDBType = "milvus"
)

var vectorDBTypeList = []string{"pgvector", "chroma", "qdrant", "weaviate", "milvus"}

var supportedVectorDBTypes = map[VectorDBType]struct{}{
	engineknowledge.VectorDBTypePGVector: {},
	VectorDBTypeChroma:                   {},
	engineknowledge.VectorDBTypeQdrant:   {},
	VectorDBTypeWeaviate:                 {},
	VectorDBTypeMilvus:                   {},
}

var canonicalVectorTypes = map[VectorDBType]VectorDBType{
	VectorDBType("pgvector"): engineknowledge.VectorDBTypePGVector,
	VectorDBType("chroma"):   VectorDBTypeChroma,
	VectorDBType("qdrant"):   engineknowledge.VectorDBTypeQdrant,
	VectorDBType("weaviate"): VectorDBTypeWeaviate,
	VectorDBType("milvus"):   VectorDBTypeMilvus,
}

// VectorDBBuilder constructs vector database configurations for knowledge bases.
type VectorDBBuilder struct {
	config *engineknowledge.VectorDBConfig
	errors []error
}

// NewVectorDB creates a new vector database builder for the supplied identifier and type.
func NewVectorDB(id string, dbType VectorDBType) *VectorDBBuilder {
	trimmedID := strings.TrimSpace(id)
	rawType := strings.ToLower(strings.TrimSpace(string(dbType)))
	normalized := VectorDBType(rawType)
	if canonical, ok := canonicalVectorTypes[normalized]; ok {
		normalized = canonical
	}
	return &VectorDBBuilder{
		config: &engineknowledge.VectorDBConfig{
			ID:   trimmedID,
			Type: normalized,
			Config: engineknowledge.VectorDBConnConfig{
				PGVector: nil,
			},
		},
		errors: make([]error, 0),
	}
}

// WithDSN sets the connection string used by DSN-based vector databases.
func (b *VectorDBBuilder) WithDSN(dsn string) *VectorDBBuilder {
	if b == nil {
		return nil
	}
	b.config.Config.DSN = strings.TrimSpace(dsn)
	return b
}

// WithPath sets the filesystem path used by embedded vector databases such as Chroma.
func (b *VectorDBBuilder) WithPath(path string) *VectorDBBuilder {
	if b == nil {
		return nil
	}
	b.config.Config.Path = strings.TrimSpace(path)
	return b
}

// WithCollection sets the collection name used by collection-based vector databases.
func (b *VectorDBBuilder) WithCollection(collection string) *VectorDBBuilder {
	if b == nil {
		return nil
	}
	b.config.Config.Collection = strings.TrimSpace(collection)
	return b
}

// WithPGVectorIndex configures pgvector-specific index parameters.
func (b *VectorDBBuilder) WithPGVectorIndex(indexType string, lists int) *VectorDBBuilder {
	if b == nil {
		return nil
	}
	cfg := b.ensurePGVectorConfig()
	if cfg.Index == nil {
		cfg.Index = &engineknowledge.PGVectorIndexConfig{}
	}
	cfg.Index.Type = strings.ToLower(strings.TrimSpace(indexType))
	cfg.Index.Lists = lists
	return b
}

// WithPGVectorPool configures pgvector connection pool size constraints.
func (b *VectorDBBuilder) WithPGVectorPool(minConns, maxConns int32) *VectorDBBuilder {
	if b == nil {
		return nil
	}
	cfg := b.ensurePGVectorConfig()
	if cfg.Pool == nil {
		cfg.Pool = &engineknowledge.PGVectorPoolConfig{}
	}
	cfg.Pool.MinConns = minConns
	cfg.Pool.MaxConns = maxConns
	return b
}

// Build validates the accumulated configuration and returns a cloned vector database config.
func (b *VectorDBBuilder) Build(ctx context.Context) (*engineknowledge.VectorDBConfig, error) {
	if b == nil {
		return nil, fmt.Errorf("vector db builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("building vector db configuration", "vector_db", b.config.ID, "type", b.config.Type)

	collected := append(make([]error, 0, len(b.errors)+10), b.errors...)
	collected = append(collected, b.validateID(ctx), b.validateType())
	collected = append(collected, b.validateRequirements(ctx)...)
	collected = append(collected, b.validatePGVectorIndex()...)
	collected = append(collected, b.validatePGVectorPool()...)

	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone vector db config: %w", err)
	}
	return cloned, nil
}

func (b *VectorDBBuilder) ensurePGVectorConfig() *engineknowledge.PGVectorConfig {
	if b.config.Config.PGVector == nil {
		b.config.Config.PGVector = &engineknowledge.PGVectorConfig{}
	}
	return b.config.Config.PGVector
}

func (b *VectorDBBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("vector_db id is invalid: %w", err)
	}
	return nil
}

func (b *VectorDBBuilder) validateType() error {
	raw := strings.ToLower(strings.TrimSpace(string(b.config.Type)))
	if raw == "" {
		return fmt.Errorf("vector_db type is required")
	}
	normalized := VectorDBType(raw)
	if canonical, ok := canonicalVectorTypes[normalized]; ok {
		normalized = canonical
	}
	if _, ok := supportedVectorDBTypes[normalized]; !ok {
		return fmt.Errorf(
			"vector_db type %q is not supported; must be one of %s",
			raw,
			strings.Join(vectorDBTypeList, ", "),
		)
	}
	b.config.Type = normalized
	return nil
}

func (b *VectorDBBuilder) validateRequirements(ctx context.Context) []error {
	switch b.config.Type {
	case engineknowledge.VectorDBTypePGVector:
		return b.validatePGVector(ctx)
	case VectorDBTypeChroma:
		return b.validateChroma(ctx)
	case engineknowledge.VectorDBTypeQdrant:
		return b.validateQdrant(ctx)
	case VectorDBTypeWeaviate:
		return b.validateWeaviate(ctx)
	case VectorDBTypeMilvus:
		return b.validateMilvus(ctx)
	default:
		return nil
	}
}

func (b *VectorDBBuilder) validatePGVector(ctx context.Context) []error {
	dsn := strings.TrimSpace(b.config.Config.DSN)
	b.config.Config.DSN = dsn
	var errs []error
	if err := validate.NonEmpty(ctx, "dsn", dsn); err != nil {
		err = fmt.Errorf("pgvector requires config.dsn: %w", err)
		errs = append(errs, err)
	}
	return errs
}

func (b *VectorDBBuilder) validateChroma(ctx context.Context) []error {
	path := strings.TrimSpace(b.config.Config.Path)
	b.config.Config.Path = path
	var errs []error
	if err := validate.NonEmpty(ctx, "path", path); err != nil {
		err = fmt.Errorf("chroma requires config.path: %w", err)
		errs = append(errs, err)
	}
	return errs
}

func (b *VectorDBBuilder) validateQdrant(ctx context.Context) []error {
	dsn := strings.TrimSpace(b.config.Config.DSN)
	b.config.Config.DSN = dsn
	collection := strings.TrimSpace(b.config.Config.Collection)
	b.config.Config.Collection = collection
	errs := make([]error, 0, 3)
	if err := validate.NonEmpty(ctx, "dsn", dsn); err != nil {
		err = fmt.Errorf("qdrant requires config.dsn: %w", err)
		errs = append(errs, err)
	} else if err := validate.URL(ctx, dsn); err != nil {
		err = fmt.Errorf("qdrant config.dsn must be a valid url: %w", err)
		errs = append(errs, err)
	}
	if err := validate.NonEmpty(ctx, "collection", collection); err != nil {
		err = fmt.Errorf("qdrant requires config.collection: %w", err)
		errs = append(errs, err)
	}
	return errs
}

func (b *VectorDBBuilder) validateWeaviate(ctx context.Context) []error {
	dsn := strings.TrimSpace(b.config.Config.DSN)
	b.config.Config.DSN = dsn
	collection := strings.TrimSpace(b.config.Config.Collection)
	b.config.Config.Collection = collection
	errs := make([]error, 0, 3)
	if err := validate.NonEmpty(ctx, "dsn", dsn); err != nil {
		err = fmt.Errorf("weaviate requires config.dsn: %w", err)
		errs = append(errs, err)
	} else if err := validate.URL(ctx, dsn); err != nil {
		err = fmt.Errorf("weaviate config.dsn must be a valid url: %w", err)
		errs = append(errs, err)
	}
	if err := validate.NonEmpty(ctx, "collection", collection); err != nil {
		err = fmt.Errorf("weaviate requires config.collection: %w", err)
		errs = append(errs, err)
	}
	return errs
}

func (b *VectorDBBuilder) validateMilvus(ctx context.Context) []error {
	dsn := strings.TrimSpace(b.config.Config.DSN)
	b.config.Config.DSN = dsn
	collection := strings.TrimSpace(b.config.Config.Collection)
	b.config.Config.Collection = collection
	errs := make([]error, 0, 3)
	if err := validate.NonEmpty(ctx, "dsn", dsn); err != nil {
		err = fmt.Errorf("milvus requires config.dsn: %w", err)
		errs = append(errs, err)
	} else if err := validate.URL(ctx, dsn); err != nil {
		err = fmt.Errorf("milvus config.dsn must be a valid url: %w", err)
		errs = append(errs, err)
	}
	if err := validate.NonEmpty(ctx, "collection", collection); err != nil {
		err = fmt.Errorf("milvus requires config.collection: %w", err)
		errs = append(errs, err)
	}
	return errs
}

func (b *VectorDBBuilder) validatePGVectorIndex() []error {
	cfg := b.config.Config.PGVector
	if cfg == nil || cfg.Index == nil {
		return nil
	}
	cfg.Index.Type = strings.ToLower(strings.TrimSpace(cfg.Index.Type))
	errs := make([]error, 0, 2)
	if cfg.Index.Type == "" {
		errs = append(errs, fmt.Errorf("pgvector.index.type cannot be empty"))
	}
	if cfg.Index.Lists <= 0 {
		errs = append(errs, fmt.Errorf("pgvector.index.lists must be greater than zero"))
	}
	return errs
}

func (b *VectorDBBuilder) validatePGVectorPool() []error {
	cfg := b.config.Config.PGVector
	if cfg == nil || cfg.Pool == nil {
		return nil
	}
	errs := make([]error, 0, 3)
	if cfg.Pool.MinConns < 0 {
		errs = append(errs, fmt.Errorf("pgvector.pool.min_conns must be >= 0"))
	}
	if cfg.Pool.MaxConns < 0 {
		errs = append(errs, fmt.Errorf("pgvector.pool.max_conns must be >= 0"))
	}
	if cfg.Pool.MaxConns > 0 && cfg.Pool.MinConns > cfg.Pool.MaxConns {
		errs = append(errs, fmt.Errorf("pgvector.pool.min_conns cannot exceed max_conns"))
	}
	return errs
}
