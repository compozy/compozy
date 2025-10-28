## status: completed

<task_context>
<domain>sdk/knowledge</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>medium</complexity>
<dependencies>sdk2/model</dependencies>
</task_context>

# Task 9.0: Migrate knowledge Package to Functional Options

## Overview

Migrate `sdk/knowledge` for RAG (Retrieval Augmented Generation) configurations. Knowledge has 4 different config types: Base, Binding, Embedder, and VectorDB.

**Estimated Time:** 3-4 hours

**Dependency:** Requires Task 1.0 (model) complete

<critical>
- **MULTIPLE TYPES:** 4 separate constructors needed (Base, Binding, Embedder, VectorDB)
- **COMPLEX VALIDATION:** Each type has unique validation rules
</critical>

<requirements>
- Generate options for 4 config types
- Create 4 separate constructors (NewBase, NewBinding, NewEmbedder, NewVectorDB)
- Validate embedder models (text-embedding-3-small, etc.)
- Validate vector DB types (pgvector, qdrant, pinecone, etc.)
- Handle knowledge source paths
- Deep copy and comprehensive tests
</requirements>

## Subtasks

- [x] 9.1 Create sdk2/knowledge/ directory structure
- [x] 9.2 Create 4 generate.go files (or one with 4 directives)
- [x] 9.3 Generate options for all 4 types
- [x] 9.4 Create NewBase constructor
- [x] 9.5 Create NewBinding constructor
- [x] 9.6 Create NewEmbedder constructor
- [x] 9.7 Create NewVectorDB constructor
- [x] 9.8 Tests for all 4 types
- [x] 9.9 Document multi-type approach

## Implementation Details

### Config Types

#### 1. Base Config
```go
func NewBase(ctx context.Context, id string, opts ...BaseOption) (*knowledge.BaseConfig, error)
```
Fields: ID, Sources (paths), ChunkSize, ChunkOverlap, Metadata

#### 2. Binding Config
```go
func NewBinding(ctx context.Context, id string, baseID string, opts ...BindingOption) (*knowledge.BindingConfig, error)
```
Fields: ID, BaseID (reference), FilterQuery, MaxResults

#### 3. Embedder Config
```go
func NewEmbedder(ctx context.Context, id string, model string, opts ...EmbedderOption) (*knowledge.EmbedderConfig, error)
```
Fields: ID, Model (text-embedding-3-small, etc.), Dimensions, Provider

#### 4. VectorDB Config
```go
func NewVectorDB(ctx context.Context, id string, dbType string, opts ...VectorDBOption) (*knowledge.VectorDBConfig, error)
```
Fields: ID, Type (pgvector/qdrant/pinecone), ConnectionString, CollectionName, IndexConfig

### Validation Per Type

**Base:**
- At least one source path
- ChunkSize > 0
- ChunkOverlap < ChunkSize

**Binding:**
- BaseID references valid base
- MaxResults > 0

**Embedder:**
- Model in supported list
- Dimensions match model

**VectorDB:**
- Type enum validation
- Connection string format
- Collection name non-empty

### Relevant Files

**Reference (for understanding):**
- `sdk/knowledge/builder.go` - Old builder pattern to understand requirements (~250+ LOC)
- `sdk/knowledge/builder_test.go` - Old tests to understand test cases
- `engine/knowledge/config.go` - Source structs for all 4 config types

**To Create in sdk2/knowledge/:**
- `generate.go` - Code generation directives (4 types)
- `base_options_generated.go` - Generated options for Base
- `binding_options_generated.go` - Generated options for Binding
- `embedder_options_generated.go` - Generated options for Embedder
- `vectordb_options_generated.go` - Generated options for VectorDB
- `constructors.go` - All 4 constructors (NewBase, NewBinding, NewEmbedder, NewVectorDB)
- `constructors_test.go` - Tests for all 4 types
- `README.md` - Documentation for multi-type approach

**Note:** Do NOT delete or modify anything in `sdk/knowledge/` - keep for reference during transition. All 4 config types go in the same sdk2/knowledge/ package.

## Tests

- [x] Base with single source
- [x] Base with multiple sources
- [x] ChunkSize validation
- [x] ChunkOverlap vs ChunkSize
- [x] Binding with valid BaseID
- [x] Binding with max results
- [x] Embedder with supported model
- [x] Embedder dimension validation
- [x] VectorDB pgvector config
- [x] VectorDB qdrant config
- [x] Invalid types fail
- [x] Missing required fields fail

## Success Criteria

- [x] sdk2/knowledge/ directory structure created
- [x] All 4 config types working in sdk2/knowledge/
- [x] Type-specific validation complete
- [x] Clear API separation between types
- [x] Tests pass: `gotestsum -- ./sdk2/knowledge` (40 tests passing)
- [x] Linter clean: `golangci-lint run ./sdk2/knowledge/...` (0 issues)
- [x] Reduction: ~250+ LOC → ~120 LOC (52% reduction)
- [x] README clearly documents when to use each type
- [x] Old sdk/knowledge/ remains untouched
