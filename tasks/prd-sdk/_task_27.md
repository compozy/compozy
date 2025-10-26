## status: pending

<task_context>
<domain>sdk/knowledge</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/knowledge, sdk/internal</dependencies>
</task_context>

# Task 27.0: Knowledge: VectorDB (S)

## Overview

Implement VectorDBBuilder in `sdk/knowledge/vectordb.go` to configure vector database backends (PGVector, Chroma, Qdrant, etc.) for knowledge storage and retrieval.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Context-first patterns)
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Knowledge section 6.2)
</critical>

<requirements>
- Support multiple vector DB types (PGVector, Chroma, Qdrant, etc.)
- Configure connection strings, paths, and collections
- PGVector-specific: index type, pool configuration
- Follow error accumulation pattern (BuildError)
- Use logger.FromContext(ctx) in Build method
</requirements>

## Subtasks

- [ ] 27.1 Create sdk/knowledge/vectordb.go with VectorDBBuilder
- [ ] 27.2 Implement NewVectorDB(id, dbType)
- [ ] 27.3 Implement WithDSN, WithPath, WithCollection methods
- [ ] 27.4 Implement PGVector-specific methods (WithPGVectorIndex, WithPGVectorPool)
- [ ] 27.5 Add unit tests for all DB types and configurations

## Implementation Details

Per 03-sdk-entities.md section 6.2:

```go
type VectorDBBuilder struct {
    config *knowledge.VectorDBConfig
    errors []error
}

func NewVectorDB(id string, dbType knowledge.VectorDBType) *VectorDBBuilder
func (b *VectorDBBuilder) WithDSN(dsn string) *VectorDBBuilder
func (b *VectorDBBuilder) WithPath(path string) *VectorDBBuilder
func (b *VectorDBBuilder) WithCollection(collection string) *VectorDBBuilder

// PGVector-specific
func (b *VectorDBBuilder) WithPGVectorIndex(indexType string, lists int) *VectorDBBuilder
func (b *VectorDBBuilder) WithPGVectorPool(minConns, maxConns int32) *VectorDBBuilder

func (b *VectorDBBuilder) Build(ctx context.Context) (*knowledge.VectorDBConfig, error)
```

Supported DB types: pgvector, chroma, qdrant, weaviate, milvus

### Relevant Files

- `sdk/knowledge/vectordb.go` (new)
- `engine/knowledge/vectordb_config.go` (reference)
- `sdk/internal/errors/build_error.go` (existing)

### Dependent Files

- `sdk/knowledge/base.go` (consumer)
- `sdk/project/builder.go` (consumer)

## Deliverables

- sdk/knowledge/vectordb.go with complete VectorDBBuilder
- Unit tests for each DB type (PGVector, Chroma, etc.)
- Validation for required fields per DB type
- GoDoc comments for all methods
- Type-safe VectorDBType enum usage

## Tests

Unit tests from _tests.md (Knowledge section):

- [ ] Valid PGVector config with DSN builds successfully
- [ ] Valid Chroma config with path builds successfully
- [ ] Valid Qdrant config with URL builds successfully
- [ ] VectorDB with invalid type returns BuildError
- [ ] PGVector without DSN returns validation error
- [ ] Chroma without path returns validation error
- [ ] PGVector index configuration validates lists>0
- [ ] PGVector pool configuration validates minConns<=maxConns
- [ ] Build(ctx) propagates context to validation
- [ ] logger.FromContext(ctx) used in Build method
- [ ] DB-specific validations enforce required fields

## Success Criteria

- All unit tests pass with >95% coverage
- make lint passes
- Each DB type has correct validation rules
- Error messages specify which configuration is missing
- PGVector configurations match engine expectations
