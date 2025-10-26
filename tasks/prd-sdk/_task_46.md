## status: pending

<task_context>
<domain>sdk/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>sdk/knowledge</dependencies>
</task_context>

# Task 46.0: Example: Knowledge (RAG) (S)

## Overview

Create comprehensive RAG example demonstrating the complete knowledge system with embedder, vector DB, sources, knowledge base, and agent binding.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/05-examples.md (Example 3: Knowledge Base)
- **MUST** demonstrate all 5 knowledge builders
- **MUST** show multiple source types (file, directory, URL)
</critical>

<requirements>
- Runnable example: sdk/examples/03_knowledge_rag.go
- Demonstrates: All 5 knowledge builders (Embedder, VectorDB, Source, Base, Binding)
- Shows multiple source types
- PGVector configuration with indexes
- Agent integration with knowledge binding
- Clear comments on RAG patterns
</requirements>

## Subtasks

- [ ] 46.1 Create sdk/examples/03_knowledge_rag.go
- [ ] 46.2 Build embedder configuration (OpenAI)
- [ ] 46.3 Build vector DB configuration (PGVector)
- [ ] 46.4 Create multiple source configs (file, directory, URL)
- [ ] 46.5 Build knowledge base with chunking/preprocessing
- [ ] 46.6 Build knowledge binding with retrieval params
- [ ] 46.7 Create agent with knowledge integration
- [ ] 46.8 Build complete project with knowledge system
- [ ] 46.9 Add comments explaining RAG workflow
- [ ] 46.10 Update README.md with RAG example
- [ ] 46.11 Test example runs successfully

## Implementation Details

Per 05-examples.md section 3:

**Embedder configuration:**
```go
embedder, err := knowledge.NewEmbedder("openai-embedder", "openai", "text-embedding-3-small").
    WithAPIKey(os.Getenv("OPENAI_API_KEY")).
    WithDimension(1536).
    WithBatchSize(100).
    WithMaxConcurrentWorkers(4).
    Build(ctx)
```

**Vector DB with PGVector:**
```go
vectorDB, err := knowledge.NewVectorDB("docs-db", knowledge.VectorDBTypePGVector).
    WithDSN("postgres://localhost/myapp").
    WithCollection("documentation").
    WithPGVectorIndex("hnsw", 100).
    WithPGVectorPool(5, 20).
    Build(ctx)
```

**Knowledge base with retrieval:**
```go
kb, err := knowledge.NewBase("product-docs").
    WithEmbedder("openai-embedder").
    WithVectorDB("docs-db").
    AddSource(markdownSource).
    WithChunking(knowledge.ChunkStrategyRecursive, 1000, 200).
    WithRetrieval(5, 0.7, 2000).
    Build(ctx)
```

### Relevant Files

- `sdk/examples/03_knowledge_rag.go` - Main example
- `sdk/examples/README.md` - Updated instructions

### Dependent Files

- `sdk/knowledge/embedder.go` - EmbedderBuilder
- `sdk/knowledge/vectordb.go` - VectorDBBuilder
- `sdk/knowledge/source.go` - SourceBuilder
- `sdk/knowledge/base.go` - BaseBuilder
- `sdk/knowledge/binding.go` - BindingBuilder

## Deliverables

- [ ] sdk/examples/03_knowledge_rag.go (runnable)
- [ ] Updated README.md with RAG example section
- [ ] Comments explaining RAG workflow (embed → store → retrieve)
- [ ] All 5 knowledge builders demonstrated
- [ ] Multiple source types shown
- [ ] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [ ] Code compiles without errors
  - [ ] Example runs with valid DB connection
  - [ ] Embedder config matches engine expectations
  - [ ] VectorDB config with PGVector indexes
  - [ ] Source builders create valid configs
  - [ ] Knowledge binding works with agent
  - [ ] Retrieval parameters validated

## Success Criteria

- Example demonstrates complete RAG system
- All 5 knowledge builders used correctly
- Comments explain RAG workflow
- README updated with setup requirements (Postgres + pgvector)
- Example runs end-to-end successfully
- Code passes `make lint`
