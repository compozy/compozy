# Technical Specification: Document Embeddings for Agent Knowledge

## Executive Summary

This technical specification outlines the implementation of document embeddings for Compozy agents, enabling them to access persistent, queryable knowledge bases. The design follows Compozy's established architecture patterns: Clean Architecture with domain-driven design, MCP integration for external services, asynchronous Temporal workflows, and Redis-based storage with atomic operations.

**Key Architectural Decisions:**

- New `engine/knowledge` domain for clear separation of concerns
- Asynchronous document processing via Temporal workflows
- MCP abstraction for embedding providers and vector databases
- Integration with existing agent memory system via token budgeting
- Workspace-scoped security through metadata filtering

## Architecture Overview

### System Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   CLI/API       │    │  Agent Memory   │    │  LLM Service    │
│                 │    │   (Session)     │    │ (Orchestrator)  │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          ▼                      │                      ▼
┌─────────────────┐              │            ┌─────────────────┐
│ Knowledge       │              │            │ Context         │
│ Service         │              │            │ Assembly        │
│ (Document CRUD) │              │            │ (Memory +       │
│                 │              │            │  Knowledge)     │
└─────────┬───────┘              │            └─────────┬───────┘
          │                      │                      │
          ▼                      ▼                      │
┌─────────────────┐    ┌─────────────────┐              │
│ Document        │    │ Vector Query    │              │
│ Processing      │    │ Service         │◄─────────────┘
│ Workflow        │    │                 │
│ (Temporal)      │    └─────────┬───────┘
└─────────┬───────┘              │
          │                      │
          ▼                      ▼
┌─────────────────┐    ┌─────────────────┐
│ MCP Embedding   │    │ MCP Vector      │
│ Provider        │    │ Database        │
│ (OpenAI/Cohere) │    │ (Weaviate/etc)  │
└─────────────────┘    └─────────────────┘
```

### Domain Structure

Following Compozy's established patterns, the knowledge system will be implemented as:

```
engine/knowledge/
├── config.go              # Knowledge configuration types
├── service.go             # Main knowledge service
├── repository.go          # Repository interface definitions
├── document.go            # Document entity and operations
├── chunk.go               # Chunk entity and operations
├── query.go               # Query service for retrieval
├── activities/            # Temporal activities
│   ├── document_processing.go
│   ├── embedding_generation.go
│   └── vector_storage.go
├── workflows/             # Temporal workflows
│   ├── document_ingestion.go
│   └── document_update.go
└── router/                # HTTP handlers
    ├── document_handlers.go
    └── query_handlers.go
```

## Detailed Design

### 1. Core Entities

#### Document Entity

```go
// Document represents a user-uploaded document with metadata
type Document struct {
    ID          core.ID                `json:"id" bson:"_id"`
    WorkspaceID core.ID                `json:"workspace_id" bson:"workspace_id"`
    Name        string                 `json:"name" bson:"name"`
    OriginalURL string                 `json:"original_url,omitempty" bson:"original_url,omitempty"`
    MimeType    string                 `json:"mime_type" bson:"mime_type"`
    SizeBytes   int64                  `json:"size_bytes" bson:"size_bytes"`
    Content     string                 `json:"content,omitempty" bson:"content,omitempty"`
    Metadata    map[string]interface{} `json:"metadata" bson:"metadata"`
    Status      ProcessingStatus       `json:"status" bson:"status"`
    ChunkCount  int                    `json:"chunk_count" bson:"chunk_count"`
    CreatedAt   time.Time              `json:"created_at" bson:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at" bson:"updated_at"`
    CreatedBy   core.ID                `json:"created_by" bson:"created_by"`
}

type ProcessingStatus string

const (
    StatusPending    ProcessingStatus = "pending"
    StatusProcessing ProcessingStatus = "processing"
    StatusCompleted  ProcessingStatus = "completed"
    StatusFailed     ProcessingStatus = "failed"
)
```

#### Chunk Entity

```go
// Chunk represents a processable piece of a document
type Chunk struct {
    ID         core.ID         `json:"id"`
    DocumentID core.ID         `json:"document_id"`
    Index      int             `json:"index"`
    Content    string          `json:"content"`
    Metadata   ChunkMetadata   `json:"metadata"`
    TokenCount int             `json:"token_count"`
    CreatedAt  time.Time       `json:"created_at"`
}

type ChunkMetadata struct {
    StartOffset    int                    `json:"start_offset"`
    EndOffset      int                    `json:"end_offset"`
    SourceSection  string                 `json:"source_section,omitempty"`
    Headers        []string               `json:"headers,omitempty"`
    Custom         map[string]interface{} `json:"custom,omitempty"`
}
```

### 2. Configuration System

Following the established pattern from `engine/memory/config.go`:

```go
// Config represents knowledge base configuration for agents
type Config struct {
    // Knowledge base reference
    KnowledgeBase string `yaml:"knowledge_base" json:"knowledge_base" validate:"required"`

    // Query configuration
    Query QueryConfig `yaml:"query" json:"query"`

    // Access mode
    Mode AccessMode `yaml:"mode" json:"mode" validate:"required,oneof=read-only read-write"`

    // Token allocation for retrieved knowledge
    TokenAllocation float64 `yaml:"token_allocation" json:"token_allocation" validate:"min=0,max=1"`
}

type QueryConfig struct {
    MaxResults     int     `yaml:"max_results" json:"max_results" validate:"min=1,max=20"`
    SimilarityThreshold float64 `yaml:"similarity_threshold" json:"similarity_threshold" validate:"min=0,max=1"`
    RetrievalStrategy   string  `yaml:"retrieval_strategy" json:"retrieval_strategy" validate:"oneof=semantic hybrid keyword"`
}

type AccessMode string

const (
    AccessModeReadOnly  AccessMode = "read-only"
    AccessModeReadWrite AccessMode = "read-write"
)
```

### 3. Service Layer

#### Knowledge Service

```go
// Service manages document knowledge operations
type Service struct {
    config      *Config
    repo        Repository
    vectorRepo  VectorRepository
    mcpClient   *mcp.Client
    tmplEngine  *tplengine.TemplateEngine
    workflows   WorkflowService
}

// NewService creates a new knowledge service
func NewService(
    config *Config,
    repo Repository,
    vectorRepo VectorRepository,
    mcpClient *mcp.Client,
    tmplEngine *tplengine.TemplateEngine,
    workflows WorkflowService,
) *Service {
    if config == nil {
        config = DefaultConfig()
    }
    return &Service{
        config:     config,
        repo:       repo,
        vectorRepo: vectorRepo,
        mcpClient:  mcpClient,
        tmplEngine: tmplEngine,
        workflows:  workflows,
    }
}

// UploadDocument uploads and processes a document asynchronously
func (s *Service) UploadDocument(ctx context.Context, req *UploadRequest) (*Document, error) {
    log := logger.FromContext(ctx)

    // Validate request
    if err := req.Validate(); err != nil {
        return nil, core.NewError(err, "INVALID_UPLOAD_REQUEST", map[string]any{
            "file_name": req.FileName,
            "size_bytes": req.SizeBytes,
        })
    }

    // Create document entity
    doc := &Document{
        ID:          core.NewID(),
        WorkspaceID: req.WorkspaceID,
        Name:        req.FileName,
        MimeType:    req.MimeType,
        SizeBytes:   req.SizeBytes,
        Content:     req.Content,
        Status:      StatusPending,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
        CreatedBy:   req.UserID,
    }

    // Store document
    if err := s.repo.CreateDocument(ctx, doc); err != nil {
        return nil, fmt.Errorf("failed to store document: %w", err)
    }

    // Start async processing workflow
    if err := s.workflows.StartDocumentProcessing(ctx, doc.ID); err != nil {
        log.Error("failed to start document processing workflow",
            "document_id", doc.ID, "error", err)
        // Update status to failed
        doc.Status = StatusFailed
        s.repo.UpdateDocument(ctx, doc)
        return nil, fmt.Errorf("failed to start document processing: %w", err)
    }

    log.Info("document uploaded and queued for processing",
        "document_id", doc.ID, "file_name", req.FileName)

    return doc, nil
}

// QueryDocuments retrieves relevant document chunks for a query
func (s *Service) QueryDocuments(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
    log := logger.FromContext(ctx)

    // Generate query embedding
    embedding, err := s.generateQueryEmbedding(ctx, req.Query)
    if err != nil {
        return nil, fmt.Errorf("failed to generate query embedding: %w", err)
    }

    // Search vector database
    results, err := s.vectorRepo.Search(ctx, &VectorSearchRequest{
        Embedding:     embedding,
        WorkspaceID:   req.WorkspaceID,
        MaxResults:    req.MaxResults,
        Threshold:     req.SimilarityThreshold,
        StrategyType:  req.Strategy,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to search vector database: %w", err)
    }

    // Enhance results with document metadata
    enhanced, err := s.enhanceSearchResults(ctx, results)
    if err != nil {
        return nil, fmt.Errorf("failed to enhance search results: %w", err)
    }

    log.Info("knowledge query completed",
        "query_length", len(req.Query), "results_count", len(enhanced))

    return &QueryResponse{
        Results:   enhanced,
        QueryTime: time.Since(time.Now()),
    }, nil
}
```

### 4. Asynchronous Processing

#### Document Processing Workflow

```go
// DocumentProcessingWorkflow handles document ingestion pipeline
func DocumentProcessingWorkflow(ctx workflow.Context, documentID core.ID) error {
    log := workflow.GetLogger(ctx)

    // Configure activity options
    activityOptions := workflow.ActivityOptions{
        StartToCloseTimeout: 10 * time.Minute,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval:    time.Second,
            BackoffCoefficient: 2.0,
            MaximumInterval:    time.Minute,
            MaximumAttempts:    3,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, activityOptions)

    // Step 1: Parse and chunk document
    var chunkResult ChunkDocumentResult
    err := workflow.ExecuteActivity(ctx, "ChunkDocument", documentID).Get(ctx, &chunkResult)
    if err != nil {
        log.Error("failed to chunk document", "document_id", documentID, "error", err)
        return temporal.NewNonRetryableError(fmt.Errorf("chunking failed: %w", err))
    }

    // Step 2: Generate embeddings for chunks
    var embeddingResult GenerateEmbeddingsResult
    err = workflow.ExecuteActivity(ctx, "GenerateEmbeddings", chunkResult).Get(ctx, &embeddingResult)
    if err != nil {
        log.Error("failed to generate embeddings", "document_id", documentID, "error", err)
        return fmt.Errorf("embedding generation failed: %w", err)
    }

    // Step 3: Store embeddings in vector database
    var storeResult StoreEmbeddingsResult
    err = workflow.ExecuteActivity(ctx, "StoreEmbeddings", embeddingResult).Get(ctx, &storeResult)
    if err != nil {
        log.Error("failed to store embeddings", "document_id", documentID, "error", err)
        return fmt.Errorf("embedding storage failed: %w", err)
    }

    // Step 4: Update document status
    err = workflow.ExecuteActivity(ctx, "UpdateDocumentStatus", UpdateStatusRequest{
        DocumentID: documentID,
        Status:     StatusCompleted,
        ChunkCount: len(chunkResult.Chunks),
    }).Get(ctx, nil)
    if err != nil {
        log.Error("failed to update document status", "document_id", documentID, "error", err)
        return fmt.Errorf("status update failed: %w", err)
    }

    log.Info("document processing completed successfully", "document_id", documentID)
    return nil
}
```

#### Activity Implementations

```go
// ChunkDocumentActivity processes a document into searchable chunks
func ChunkDocumentActivity(ctx context.Context, documentID core.ID) (*ChunkDocumentResult, error) {
    log := logger.FromContext(ctx)

    // Load document from repository
    repo := GetRepositoryFromContext(ctx)
    doc, err := repo.GetDocument(ctx, documentID)
    if err != nil {
        return nil, fmt.Errorf("failed to load document: %w", err)
    }

    // Parse content based on MIME type
    parser := GetParserForMimeType(doc.MimeType)
    content, err := parser.Parse(doc.Content)
    if err != nil {
        return nil, fmt.Errorf("failed to parse document content: %w", err)
    }

    // Chunk content
    chunker := NewSemanticChunker(ChunkerConfig{
        MaxChunkSize:    1000,
        ChunkOverlap:    200,
        SplitOnSentence: true,
    })

    chunks, err := chunker.Chunk(content)
    if err != nil {
        return nil, fmt.Errorf("failed to chunk document: %w", err)
    }

    // Store chunks
    chunkEntities := make([]Chunk, 0, len(chunks))
    for i, chunk := range chunks {
        entity := Chunk{
            ID:         core.NewID(),
            DocumentID: documentID,
            Index:      i,
            Content:    chunk.Text,
            Metadata: ChunkMetadata{
                StartOffset:   chunk.StartOffset,
                EndOffset:     chunk.EndOffset,
                SourceSection: chunk.Section,
                Headers:       chunk.Headers,
            },
            TokenCount: estimateTokenCount(chunk.Text),
            CreatedAt:  time.Now(),
        }
        chunkEntities = append(chunkEntities, entity)
    }

    if err := repo.CreateChunks(ctx, chunkEntities); err != nil {
        return nil, fmt.Errorf("failed to store chunks: %w", err)
    }

    log.Info("document chunked successfully",
        "document_id", documentID, "chunk_count", len(chunkEntities))

    return &ChunkDocumentResult{
        DocumentID: documentID,
        Chunks:     chunkEntities,
    }, nil
}

// GenerateEmbeddingsActivity generates vector embeddings via MCP
func GenerateEmbeddingsActivity(ctx context.Context, req ChunkDocumentResult) (*GenerateEmbeddingsResult, error) {
    log := logger.FromContext(ctx)

    mcpClient := GetMCPClientFromContext(ctx)

    // Prepare embedding requests
    texts := make([]string, 0, len(req.Chunks))
    for _, chunk := range req.Chunks {
        texts = append(texts, chunk.Content)
    }

    // Call embedding provider via MCP
    result, err := mcpClient.CallTool(ctx, "embedding-provider", "generate_embeddings", map[string]any{
        "texts": texts,
        "model": "text-embedding-ada-002", // configurable
    })
    if err != nil {
        return nil, fmt.Errorf("failed to generate embeddings via MCP: %w", err)
    }

    // Parse embedding response
    embeddings, err := parseEmbeddingResponse(result)
    if err != nil {
        return nil, fmt.Errorf("failed to parse embedding response: %w", err)
    }

    if len(embeddings) != len(req.Chunks) {
        return nil, fmt.Errorf("embedding count mismatch: got %d, expected %d",
            len(embeddings), len(req.Chunks))
    }

    // Combine chunks with embeddings
    chunkEmbeddings := make([]ChunkEmbedding, 0, len(req.Chunks))
    for i, chunk := range req.Chunks {
        chunkEmbeddings = append(chunkEmbeddings, ChunkEmbedding{
            Chunk:     chunk,
            Embedding: embeddings[i],
        })
    }

    log.Info("embeddings generated successfully",
        "document_id", req.DocumentID, "chunk_count", len(chunkEmbeddings))

    return &GenerateEmbeddingsResult{
        DocumentID:       req.DocumentID,
        ChunkEmbeddings: chunkEmbeddings,
    }, nil
}
```

### 5. MCP Integration

#### MCP Provider Configurations

**Embedding Provider MCP:**

```yaml
# MCP configuration for embedding service
mcps:
  - id: embedding-provider
    url: "http://embedding-service:8080"
    transport: sse
    env:
      OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    tools:
      - generate_embeddings
      - get_embedding_model_info
```

**Vector Database MCP:**

```yaml
# MCP configuration for vector database
mcps:
  - id: vector-store
    command: "docker run --rm -i weaviate/weaviate-mcp:latest"
    transport: stdio
    env:
      WEAVIATE_URL: "{{ .env.WEAVIATE_URL }}"
      WEAVIATE_API_KEY: "{{ .env.WEAVIATE_API_KEY }}"
    tools:
      - store_vectors
      - search_vectors
      - delete_vectors
      - create_collection
```

### 6. Agent Integration

#### LLM Service Enhancement

```go
// Enhanced LLM service with knowledge integration
func (o *orchestrator) Execute(ctx context.Context, req Request) (*core.Output, error) {
    log := logger.FromContext(ctx)

    // Build base context from memory
    memoryContext, err := o.buildMemoryContext(ctx, req.Agent)
    if err != nil {
        return nil, fmt.Errorf("failed to build memory context: %w", err)
    }

    // Retrieve relevant knowledge if agent has knowledge configuration
    var knowledgeContext []string
    if req.Agent.Knowledge != nil {
        knowledge, err := o.buildKnowledgeContext(ctx, req.Agent, req.Action.Prompt)
        if err != nil {
            log.Warn("failed to retrieve knowledge context", "error", err)
            // Continue without knowledge context - don't fail the entire request
        } else {
            knowledgeContext = knowledge
        }
    }

    // Calculate token allocation
    allocation := calculateTokenAllocation(req.Agent, len(knowledgeContext))

    // Assemble final prompt with proper token budgeting
    prompt, err := o.promptBuilder.Build(PromptRequest{
        SystemPrompt:      req.Agent.Instructions,
        KnowledgeContext: knowledgeContext,
        MemoryContext:    memoryContext,
        UserPrompt:       req.Action.Prompt,
        TokenAllocation:  allocation,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to build prompt: %w", err)
    }

    // Execute LLM request
    response, err := o.executeWithLLM(ctx, req.Agent, prompt)
    if err != nil {
        return nil, fmt.Errorf("failed to execute LLM request: %w", err)
    }

    // Store response in memory (async)
    go o.storeInMemory(context.Background(), req.Agent, req.Action.Prompt, response.Content)

    return response, nil
}

func (o *orchestrator) buildKnowledgeContext(ctx context.Context, agent *agent.Config, query string) ([]string, error) {
    if o.knowledgeService == nil {
        return nil, fmt.Errorf("knowledge service not configured")
    }

    // Query knowledge base
    result, err := o.knowledgeService.QueryDocuments(ctx, &QueryRequest{
        Query:               query,
        WorkspaceID:         agent.WorkspaceID,
        MaxResults:          agent.Knowledge.Query.MaxResults,
        SimilarityThreshold: agent.Knowledge.Query.SimilarityThreshold,
        Strategy:            agent.Knowledge.Query.RetrievalStrategy,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to query knowledge base: %w", err)
    }

    // Format knowledge context
    context := make([]string, 0, len(result.Results))
    for _, chunk := range result.Results {
        formatted := fmt.Sprintf("# %s\n\n%s\n\n---\n", chunk.Source, chunk.Content)
        context = append(context, formatted)
    }

    return context, nil
}
```

### 7. Data Layer

#### Repository Interfaces

```go
// Repository defines document and chunk persistence operations
type Repository interface {
    // Document operations
    CreateDocument(ctx context.Context, doc *Document) error
    GetDocument(ctx context.Context, id core.ID) (*Document, error)
    UpdateDocument(ctx context.Context, doc *Document) error
    DeleteDocument(ctx context.Context, id core.ID) error
    ListDocuments(ctx context.Context, workspaceID core.ID, opts ListOptions) ([]*Document, error)

    // Chunk operations
    CreateChunks(ctx context.Context, chunks []Chunk) error
    GetChunksByDocument(ctx context.Context, documentID core.ID) ([]Chunk, error)
    DeleteChunksByDocument(ctx context.Context, documentID core.ID) error
}

// VectorRepository defines vector database operations
type VectorRepository interface {
    StoreVectors(ctx context.Context, req *StoreVectorsRequest) error
    Search(ctx context.Context, req *VectorSearchRequest) (*VectorSearchResponse, error)
    DeleteVectors(ctx context.Context, documentID core.ID) error
    GetCollectionInfo(ctx context.Context, workspaceID core.ID) (*CollectionInfo, error)
}
```

#### Redis Implementation

```go
// RedisRepository implements Repository using Redis
type RedisRepository struct {
    client *redis.Client
    prefix string
}

func NewRedisRepository(client *redis.Client) Repository {
    return &RedisRepository{
        client: client,
        prefix: "compozy:knowledge",
    }
}

func (r *RedisRepository) CreateDocument(ctx context.Context, doc *Document) error {
    key := fmt.Sprintf("%s:documents:%s", r.prefix, doc.ID)

    data, err := json.Marshal(doc)
    if err != nil {
        return fmt.Errorf("failed to marshal document: %w", err)
    }

    pipe := r.client.Pipeline()
    pipe.Set(ctx, key, data, 0)
    pipe.SAdd(ctx, fmt.Sprintf("%s:workspace:%s:documents", r.prefix, doc.WorkspaceID), doc.ID.String())

    _, err = pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("failed to store document in Redis: %w", err)
    }

    return nil
}

func (r *RedisRepository) ListDocuments(ctx context.Context, workspaceID core.ID, opts ListOptions) ([]*Document, error) {
    // Get document IDs for workspace
    key := fmt.Sprintf("%s:workspace:%s:documents", r.prefix, workspaceID)
    ids, err := r.client.SMembers(ctx, key).Result()
    if err != nil {
        return nil, fmt.Errorf("failed to get document IDs: %w", err)
    }

    if len(ids) == 0 {
        return []*Document{}, nil
    }

    // Get documents in batch
    keys := make([]string, 0, len(ids))
    for _, id := range ids {
        keys = append(keys, fmt.Sprintf("%s:documents:%s", r.prefix, id))
    }

    results, err := r.client.MGet(ctx, keys...).Result()
    if err != nil {
        return nil, fmt.Errorf("failed to get documents: %w", err)
    }

    documents := make([]*Document, 0, len(results))
    for _, result := range results {
        if result == nil {
            continue
        }

        var doc Document
        if err := json.Unmarshal([]byte(result.(string)), &doc); err != nil {
            continue // Skip malformed documents
        }

        documents = append(documents, &doc)
    }

    // Apply sorting and pagination
    return r.applyListOptions(documents, opts), nil
}
```

### 8. CLI Integration

#### Document Management Commands

```go
// CLI commands following established patterns
func init() {
    docsCmd := &cobra.Command{
        Use:   "docs",
        Short: "Manage knowledge base documents",
        Long:  "Upload, list, and manage documents in agent knowledge bases",
    }

    docsCmd.AddCommand(
        newDocsUploadCmd(),
        newDocsListCmd(),
        newDocsDeleteCmd(),
        newDocsStatusCmd(),
    )

    rootCmd.AddCommand(docsCmd)
}

func newDocsUploadCmd() *cobra.Command {
    var (
        knowledgeBase string
        recursive     bool
        exclude       []string
    )

    cmd := &cobra.Command{
        Use:   "upload [files...]",
        Short: "Upload documents to knowledge base",
        Args:  cobra.MinimumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()

            client := cli.GetAPIClient(ctx)

            for _, file := range args {
                if err := uploadFile(ctx, client, file, knowledgeBase); err != nil {
                    return fmt.Errorf("failed to upload %s: %w", file, err)
                }
                fmt.Printf("✓ Uploaded: %s\n", file)
            }

            return nil
        },
    }

    cmd.Flags().StringVar(&knowledgeBase, "knowledge-base", "", "Knowledge base ID")
    cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Upload files recursively")
    cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "File patterns to exclude")

    cmd.MarkFlagRequired("knowledge-base")

    return cmd
}
```

### 9. Security & Access Control

#### Workspace-Level Security

```go
// Security middleware for knowledge operations
func WorkspaceSecurityMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := c.Request.Context()

        // Extract workspace ID from context/token
        workspaceID, exists := auth.GetWorkspaceID(ctx)
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "workspace not specified"})
            c.Abort()
            return
        }

        // Add workspace filter to context
        ctx = knowledge.WithWorkspaceFilter(ctx, workspaceID)
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}

// Repository operations automatically filter by workspace
func (r *RedisRepository) ListDocuments(ctx context.Context, workspaceID core.ID, opts ListOptions) ([]*Document, error) {
    // Validate workspace access
    if err := auth.ValidateWorkspaceAccess(ctx, workspaceID); err != nil {
        return nil, core.NewError(err, "WORKSPACE_ACCESS_DENIED", map[string]any{
            "workspace_id": workspaceID,
        })
    }

    // Continue with workspace-scoped query...
}
```

#### Audit Logging

```go
// Audit logging for knowledge operations
func (s *Service) UploadDocument(ctx context.Context, req *UploadRequest) (*Document, error) {
    log := logger.FromContext(ctx)

    // Audit log
    defer func() {
        audit.Log(ctx, audit.Event{
            Action:      "knowledge.document.upload",
            ResourceID:  req.FileName,
            WorkspaceID: req.WorkspaceID,
            UserID:      req.UserID,
            Metadata: map[string]any{
                "file_size": req.SizeBytes,
                "mime_type": req.MimeType,
            },
        })
    }()

    // Implementation continues...
}
```

### 10. Monitoring & Observability

#### Metrics

```go
// Metrics for knowledge operations
var (
    documentsUploaded = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "compozy_knowledge_documents_uploaded_total",
            Help: "Total number of documents uploaded",
        },
        []string{"workspace_id", "mime_type", "status"},
    )

    embeddingGenerationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "compozy_knowledge_embedding_generation_duration_seconds",
            Help:    "Time taken to generate embeddings",
            Buckets: prometheus.DefBuckets,
        },
        []string{"provider", "model"},
    )

    knowledgeQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "compozy_knowledge_query_duration_seconds",
            Help:    "Time taken to query knowledge base",
            Buckets: prometheus.DefBuckets,
        },
        []string{"workspace_id", "strategy"},
    )
)
```

#### Health Checks

```go
// Health check for knowledge service
func (s *Service) HealthCheck(ctx context.Context) error {
    var errs []error

    // Check repository health
    if err := s.repo.Ping(ctx); err != nil {
        errs = append(errs, fmt.Errorf("repository unhealthy: %w", err))
    }

    // Check vector database health
    if err := s.vectorRepo.Ping(ctx); err != nil {
        errs = append(errs, fmt.Errorf("vector database unhealthy: %w", err))
    }

    // Check MCP providers
    if err := s.mcpClient.Health(ctx); err != nil {
        errs = append(errs, fmt.Errorf("MCP providers unhealthy: %w", err))
    }

    if len(errs) > 0 {
        return errors.Join(errs...)
    }

    return nil
}
```

## Implementation Plan

### Phase 1: Foundation (2-3 weeks)

1. **Core Domain Setup**
   - Create `engine/knowledge` package structure
   - Implement core entities (Document, Chunk)
   - Define repository interfaces
   - Basic configuration system

2. **Repository Layer**
   - Redis-based document repository
   - MCP-based vector repository abstraction
   - Basic CRUD operations

3. **CLI Integration**
   - Basic `compozy docs upload` command
   - Document listing and status commands

### Phase 2: Processing Pipeline (3-4 weeks)

1. **Temporal Workflows**
   - Document processing workflow
   - Activity implementations (chunking, embedding, storage)
   - Error handling and retry logic

2. **MCP Integration**
   - Embedding provider MCP specification
   - Vector database MCP specification
   - Sample implementations for OpenAI + Weaviate

3. **Document Processing**
   - Multi-format document parsers
   - Semantic chunking algorithms
   - Token counting and validation

### Phase 3: Agent Integration (2-3 weeks)

1. **LLM Service Enhancement**
   - Knowledge context retrieval
   - Token budget management
   - Prompt assembly with knowledge

2. **Agent Configuration**
   - Knowledge base configuration types
   - Agent config validation
   - Template resolution integration

3. **Query Service**
   - Semantic search implementation
   - Hybrid search strategies
   - Result ranking and filtering

### Phase 4: Production Features (2-3 weeks)

1. **Security & Access Control**
   - Workspace-level security
   - RBAC implementation
   - Audit logging

2. **Monitoring & Observability**
   - Metrics collection
   - Health checks
   - Performance monitoring

3. **API & UI**
   - REST API endpoints
   - API documentation
   - Basic management UI

## Testing Strategy

### Unit Tests

- Core business logic (service methods)
- Entity validation and operations
- Configuration parsing and validation
- Repository interfaces with mocks

### Integration Tests

- Temporal workflow execution
- MCP provider communication
- Redis storage operations
- End-to-end document processing

### Performance Tests

- Large document processing
- Concurrent upload handling
- Query response times
- Memory usage optimization

## Deployment Considerations

### Infrastructure Requirements

- **Redis**: For document metadata and configuration storage
- **Vector Database**: Weaviate, Pinecone, or PGVector for embeddings
- **Temporal**: For asynchronous workflow orchestration
- **MCP Providers**: Embedding services and vector database connectors

### Configuration Management

```yaml
# compozy.yaml - Project configuration
autoload:
  enabled: true
  include:
    - "knowledge/**/*.yaml"
    - "agents/**/*.yaml"

# knowledge/research.yaml - Knowledge base configuration
apiVersion: knowledge/v1
kind: KnowledgeBase
metadata:
  id: research-docs
spec:
  description: "Technical research documents"
  embedding_provider: openai-embeddings
  vector_store: weaviate-research
  chunking:
    strategy: semantic
    max_chunk_size: 1000
    overlap: 200
  access:
    read_roles: ["researcher", "engineer"]
    write_roles: ["researcher"]
```

### Scalability Considerations

- Horizontal scaling of workflow workers
- Vector database sharding by workspace
- Document storage partitioning
- Rate limiting for embedding APIs

## Success Metrics

### Technical Metrics

- **Document Processing**: <30 seconds for 50MB files
- **Query Latency**: <2 seconds for knowledge retrieval
- **Throughput**: 1000+ concurrent users
- **Availability**: 99.9% uptime for query operations

### Business Metrics

- **Agent Performance**: 40% reduction in manual context input
- **User Adoption**: 80% of active workspaces using knowledge features
- **Knowledge Utilization**: Average 3+ documents referenced per agent session
- **Retrieval Accuracy**: >80% relevant chunks in top-3 results

This technical specification provides a comprehensive roadmap for implementing document embeddings while maintaining consistency with Compozy's established architectural patterns and quality standards.
