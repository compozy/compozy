# Technical Architecture Specification: Embeddings System

## Table of Contents

1. [Overview](#overview)
2. [Architecture Principles](#architecture-principles)
3. [System Architecture](#system-architecture)
4. [Domain Model](#domain-model)
5. [Component Design](#component-design)
6. [Interface Contracts](#interface-contracts)
7. [Implementation Guidelines](#implementation-guidelines)
8. [Testing Strategy](#testing-strategy)
9. [Performance Architecture](#performance-architecture)
10. [Security Architecture](#security-architecture)
11. [Development Standards](#development-standards)

## Overview

This document provides the technical architecture specification for the Compozy Embeddings System, designed to support file uploads, document processing, embedding generation, and vector search capabilities. The architecture follows Clean Architecture principles, SOLID design patterns, and aligns with Compozy's existing architectural patterns.

### Key Architectural Decisions

1. **Clean Architecture**: Strict layer separation with dependency inversion
2. **Multi-Provider Support**: Extensible provider system using Strategy pattern
3. **Dimension-Specific Storage**: Separate tables per embedding dimension
4. **Event-Driven Processing**: Temporal workflows for async operations
5. **Multi-Tenancy First**: Row-level security on all data access

## Architecture Principles

### SOLID Principles Application

#### Single Responsibility Principle (SRP)

Each component has one reason to change:

- **Provider**: Only responsible for generating embeddings
- **Chunker**: Only responsible for splitting documents
- **Repository**: Only responsible for data persistence
- **Service**: Only responsible for orchestrating business logic

#### Open/Closed Principle (OCP)

System is open for extension, closed for modification:

- New providers added without changing existing code
- New chunking strategies added via interface implementation
- New storage backends added via repository interface

#### Liskov Substitution Principle (LSP)

All implementations are substitutable:

- Any `EmbeddingProvider` can be used interchangeably
- Any `ChunkingStrategy` produces valid chunks
- Any `DocumentStore` implementation maintains contracts

#### Interface Segregation Principle (ISP)

Interfaces are focused and cohesive:

- `EmbeddingGenerator` for embedding operations
- `CostEstimator` for billing concerns
- `RateLimiter` for quota management

#### Dependency Inversion Principle (DIP)

High-level modules depend on abstractions:

- Use cases depend on repository interfaces
- Services depend on provider interfaces
- Routers depend on use case interfaces

### Clean Code Principles

1. **Meaningful Names**: Use domain language (Document, Chunk, Embedding)
2. **Small Functions**: Max 30 lines, single purpose
3. **No Comments**: Code should be self-documenting
4. **Error Handling**: Explicit, wrapped with context
5. **Testing**: Every public method has tests

### DRY (Don't Repeat Yourself)

Common patterns extracted:

- Hash computation utilities
- Batch processing framework
- Rate limiting middleware
- Error wrapping helpers

## System Architecture

### Layer Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                      │
│  ┌─────────────┐ ┌──────────────┐ ┌───────────────────┐   │
│  │   Router    │ │   Temporal   │ │    Repository     │   │
│  │  (HTTP API) │ │  Activities  │ │  (PostgreSQL)     │   │
│  └─────────────┘ └──────────────┘ └───────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              ↑
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                         │
│  ┌─────────────┐ ┌──────────────┐ ┌───────────────────┐   │
│  │  Use Cases  │ │   Services   │ │    Validators     │   │
│  │ (Orchestr.) │ │ (Bus. Logic) │ │   (Constraints)   │   │
│  └─────────────┘ └──────────────┘ └───────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              ↑
┌─────────────────────────────────────────────────────────────┐
│                      Domain Layer                            │
│  ┌─────────────┐ ┌──────────────┐ ┌───────────────────┐   │
│  │   Entities  │ │    Value     │ │   Domain Events   │   │
│  │  (Document) │ │   Objects    │ │  (JobCompleted)   │   │
│  └─────────────┘ └──────────────┘ └───────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Package Structure

```
engine/embedding/
├── domain.go           # Domain entities and value objects
├── config.go           # Configuration structures
├── errors.go           # Domain-specific errors
├── events.go           # Domain events
├── interfaces.go       # Core interfaces (Repository, Provider, etc.)
├── service.go          # Domain service
├── validators.go       # Domain validation rules
│
├── router/
│   ├── router.go       # HTTP route definitions
│   ├── handlers.go     # HTTP handlers
│   └── dto.go          # Request/Response DTOs
│
├── uc/
│   ├── upload_document.go      # Upload use case
│   ├── generate_embeddings.go  # Embedding generation use case
│   ├── search_similar.go       # Similarity search use case
│   ├── reembed_document.go     # Re-embedding use case
│   └── calculate_costs.go      # Cost calculation use case
│
├── services/
│   ├── chunker/
│   │   ├── interface.go        # ChunkingStrategy interface
│   │   ├── recursive.go        # RecursiveTextChunker
│   │   ├── semantic.go         # SemanticChunker
│   │   └── factory.go          # Chunker factory
│   ├── deduplicator.go         # Hash-based deduplication
│   ├── quota_manager.go        # Rate limiting service
│   ├── cost_tracker.go         # Cost tracking service
│   └── dimension_router.go     # Routes to correct embedding table
│
├── providers/
│   ├── interface.go            # Provider interfaces
│   ├── factory.go              # Provider factory
│   ├── openai/
│   │   ├── provider.go         # OpenAI implementation
│   │   └── models.go           # OpenAI-specific models
│   ├── ollama/
│   │   └── provider.go         # Ollama implementation
│   ├── cohere/
│   │   └── provider.go         # Cohere implementation
│   └── mock/
│       └── provider.go         # Mock for testing
│
├── activities/
│   ├── extract_text.go         # Text extraction activity
│   ├── chunk_document.go       # Document chunking activity
│   ├── generate_batch.go       # Batch embedding activity
│   └── update_status.go        # Status update activity
│
├── workflows/
│   └── embedding_workflow.go   # Temporal workflow definition
│
├── repository/
│   ├── interface.go            # Repository interfaces
│   ├── postgres/
│   │   ├── document_repo.go    # Document repository
│   │   ├── chunk_repo.go       # Chunk repository
│   │   ├── embedding_repo.go   # Embedding repository
│   │   └── migrations/         # SQL migrations
│   └── memory/
│       └── repo.go             # In-memory implementation
│
└── testutil/
    ├── fixtures.go             # Test fixtures
    ├── builders.go             # Test data builders
    └── mocks.go                # Generated mocks
```

## Domain Model

### Core Entities

```go
// domain.go
package embedding

import (
    "time"
    "github.com/google/uuid"
    "github.com/compozy/compozy/core"
)

// Document represents an uploaded document
type Document struct {
    ID           uuid.UUID
    TenantID     uuid.UUID
    Filename     string
    ContentType  string
    SizeBytes    int64
    StoragePath  string
    StorageType  StorageType
    ContentHash  string
    Metadata     map[string]any
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// Chunk represents a document chunk
type Chunk struct {
    ID           uuid.UUID
    DocumentID   uuid.UUID
    TenantID     uuid.UUID
    Index        int
    Content      string
    ContentHash  string
    TokenCount   int
    Metadata     map[string]any
    CreatedAt    time.Time
}

// Embedding represents a vector embedding
type Embedding struct {
    ID            uuid.UUID
    ChunkID       uuid.UUID
    TenantID      uuid.UUID
    Provider      string
    Model         string
    ModelVersion  string
    Vector        []float32
    Dimension     int
    CreatedAt     time.Time
}

// EmbeddingJob represents an async embedding job
type EmbeddingJob struct {
    ID                 uuid.UUID
    DocumentID         uuid.UUID
    TenantID           uuid.UUID
    TemporalWorkflowID string
    Status             JobStatus
    Provider           string
    ErrorMessage       string
    StartedAt          *time.Time
    CompletedAt        *time.Time
    CreatedAt          time.Time
}
```

### Value Objects

```go
// StorageType represents where files are stored
type StorageType string

const (
    StorageTypeS3    StorageType = "s3"
    StorageTypeGCS   StorageType = "gcs"
    StorageTypeAzure StorageType = "azure"
    StorageTypeLocal StorageType = "local" // dev only
)

// JobStatus represents the status of an embedding job
type JobStatus string

const (
    JobStatusPending    JobStatus = "pending"
    JobStatusProcessing JobStatus = "processing"
    JobStatusCompleted  JobStatus = "completed"
    JobStatusFailed     JobStatus = "failed"
    JobStatusCancelled  JobStatus = "cancelled"
)

// SearchResult represents a similarity search result
type SearchResult struct {
    ChunkID      uuid.UUID
    DocumentID   uuid.UUID
    Content      string
    Similarity   float32
    Metadata     map[string]any
}

// ProviderCapabilities describes what a provider can do
type ProviderCapabilities struct {
    Name              string
    Models            []ModelInfo
    MaxBatchSize      int
    MaxTokensPerBatch int
    RateLimit         RateLimit
}

// ModelInfo describes an embedding model
type ModelInfo struct {
    Name         string
    Dimension    int
    MaxTokens    int
    CostPer1kTokens float64
}

// RateLimit defines rate limiting parameters
type RateLimit struct {
    RequestsPerMinute int
    TokensPerMinute   int
}
```

### Domain Events

```go
// events.go
package embedding

// DocumentUploaded is emitted when a document is uploaded
type DocumentUploaded struct {
    DocumentID uuid.UUID
    TenantID   uuid.UUID
    Filename   string
    SizeBytes  int64
    Timestamp  time.Time
}

// EmbeddingGenerated is emitted when embeddings are generated
type EmbeddingGenerated struct {
    ChunkID     uuid.UUID
    Provider    string
    Model       string
    TokensUsed  int
    Cost        float64
    Timestamp   time.Time
}

// JobCompleted is emitted when an embedding job completes
type JobCompleted struct {
    JobID       uuid.UUID
    DocumentID  uuid.UUID
    Status      JobStatus
    ChunksProcessed int
    TotalCost   float64
    Duration    time.Duration
    Timestamp   time.Time
}
```

## Component Design

### Provider System

```go
// providers/interface.go
package providers

import (
    "context"
    "github.com/compozy/compozy/engine/embedding"
)

// EmbeddingGenerator generates embeddings for text
type EmbeddingGenerator interface {
    // Generate creates embeddings for the given texts
    Generate(ctx context.Context, texts []string, model string) ([][]float32, error)

    // GetModelInfo returns information about a specific model
    GetModelInfo(model string) (embedding.ModelInfo, error)
}

// CostEstimator estimates costs for embedding operations
type CostEstimator interface {
    // EstimateCost estimates the cost for the given token count
    EstimateCost(model string, tokenCount int) (float64, error)
}

// RateLimiter manages rate limiting for providers
type RateLimiter interface {
    // WaitN waits until n tokens are available
    WaitN(ctx context.Context, n int) error

    // TryAcquireN tries to acquire n tokens without waiting
    TryAcquireN(n int) bool
}

// Provider combines all provider capabilities
type Provider interface {
    EmbeddingGenerator
    CostEstimator

    // GetCapabilities returns provider capabilities
    GetCapabilities() embedding.ProviderCapabilities

    // HealthCheck verifies the provider is accessible
    HealthCheck(ctx context.Context) error
}
```

### Repository Interfaces

```go
// interfaces.go
package embedding

import (
    "context"
    "github.com/google/uuid"
)

// DocumentRepository manages document persistence
type DocumentRepository interface {
    // Create stores a new document
    Create(ctx context.Context, doc *Document) error

    // GetByID retrieves a document by ID
    GetByID(ctx context.Context, id uuid.UUID) (*Document, error)

    // GetByHash retrieves a document by content hash
    GetByHash(ctx context.Context, tenantID uuid.UUID, hash string) (*Document, error)

    // List retrieves documents with pagination
    List(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*Document, int, error)

    // Delete removes a document and all related data
    Delete(ctx context.Context, id uuid.UUID) error
}

// ChunkRepository manages chunk persistence
type ChunkRepository interface {
    // CreateBatch stores multiple chunks
    CreateBatch(ctx context.Context, chunks []*Chunk) error

    // GetByDocumentID retrieves chunks for a document
    GetByDocumentID(ctx context.Context, docID uuid.UUID) ([]*Chunk, error)

    // GetByIDs retrieves specific chunks
    GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*Chunk, error)

    // GetByHash checks if a chunk hash exists
    GetByHash(ctx context.Context, tenantID uuid.UUID, hash string) (*Chunk, error)
}

// EmbeddingRepository manages embedding persistence
type EmbeddingRepository interface {
    // Store saves embeddings to the appropriate dimension table
    Store(ctx context.Context, embeddings []*Embedding) error

    // Search performs similarity search
    Search(ctx context.Context, params SearchParams) ([]*SearchResult, error)

    // GetByChunkIDs retrieves embeddings for chunks
    GetByChunkIDs(ctx context.Context, chunkIDs []uuid.UUID) ([]*Embedding, error)

    // DeleteByDocumentID removes all embeddings for a document
    DeleteByDocumentID(ctx context.Context, docID uuid.UUID) error
}

// SearchParams defines search parameters
type SearchParams struct {
    TenantID     uuid.UUID
    QueryVector  []float32
    Provider     string
    Model        string
    Limit        int
    Threshold    float32
    Filters      map[string]any
}
```

### Service Layer

```go
// service.go
package embedding

import (
    "context"
    "fmt"
    "github.com/compozy/compozy/engine/embedding/providers"
    "github.com/compozy/compozy/engine/embedding/services"
)

// Service orchestrates embedding operations
type Service struct {
    documentRepo   DocumentRepository
    chunkRepo      ChunkRepository
    embeddingRepo  EmbeddingRepository
    providerFactory providers.Factory
    chunkerFactory  services.ChunkerFactory
    deduplicator   *services.Deduplicator
    quotaManager   *services.QuotaManager
    costTracker    *services.CostTracker
}

// NewService creates a new embedding service
func NewService(
    documentRepo DocumentRepository,
    chunkRepo ChunkRepository,
    embeddingRepo EmbeddingRepository,
    providerFactory providers.Factory,
    chunkerFactory services.ChunkerFactory,
    deduplicator *services.Deduplicator,
    quotaManager *services.QuotaManager,
    costTracker *services.CostTracker,
) *Service {
    return &Service{
        documentRepo:    documentRepo,
        chunkRepo:       chunkRepo,
        embeddingRepo:   embeddingRepo,
        providerFactory: providerFactory,
        chunkerFactory:  chunkerFactory,
        deduplicator:    deduplicator,
        quotaManager:    quotaManager,
        costTracker:     costTracker,
    }
}

// GenerateEmbeddings generates embeddings for a document's chunks
func (s *Service) GenerateEmbeddings(
    ctx context.Context,
    tenantID uuid.UUID,
    chunkIDs []uuid.UUID,
    providerName string,
    model string,
) error {
    // 1. Get provider
    provider, err := s.providerFactory.GetProvider(providerName)
    if err != nil {
        return fmt.Errorf("failed to get provider: %w", err)
    }

    // 2. Check quota
    if err := s.quotaManager.CheckQuota(ctx, tenantID, providerName, len(chunkIDs)); err != nil {
        return fmt.Errorf("quota exceeded: %w", err)
    }

    // 3. Fetch chunks
    chunks, err := s.chunkRepo.GetByIDs(ctx, chunkIDs)
    if err != nil {
        return fmt.Errorf("failed to fetch chunks: %w", err)
    }

    // 4. Extract texts
    texts := make([]string, len(chunks))
    for i, chunk := range chunks {
        texts[i] = chunk.Content
    }

    // 5. Generate embeddings
    vectors, err := provider.Generate(ctx, texts, model)
    if err != nil {
        return fmt.Errorf("failed to generate embeddings: %w", err)
    }

    // 6. Create embedding records
    modelInfo, _ := provider.GetModelInfo(model)
    embeddings := make([]*Embedding, len(chunks))
    for i, chunk := range chunks {
        embeddings[i] = &Embedding{
            ChunkID:      chunk.ID,
            TenantID:     tenantID,
            Provider:     providerName,
            Model:        model,
            ModelVersion: "v1", // TODO: get from provider
            Vector:       vectors[i],
            Dimension:    modelInfo.Dimension,
        }
    }

    // 7. Store embeddings
    if err := s.embeddingRepo.Store(ctx, embeddings); err != nil {
        return fmt.Errorf("failed to store embeddings: %w", err)
    }

    // 8. Track costs
    tokenCount := 0
    for _, chunk := range chunks {
        tokenCount += chunk.TokenCount
    }
    cost, _ := provider.EstimateCost(model, tokenCount)
    if err := s.costTracker.RecordUsage(ctx, tenantID, providerName, model, tokenCount, cost); err != nil {
        // Log but don't fail
        fmt.Printf("failed to track cost: %v\n", err)
    }

    return nil
}
```

## Interface Contracts

### Use Case Interfaces

```go
// uc/interfaces.go
package uc

import (
    "context"
    "io"
    "github.com/google/uuid"
)

// UploadDocumentRequest represents a document upload request
type UploadDocumentRequest struct {
    TenantID      uuid.UUID
    Filename      string
    ContentType   string
    Content       io.Reader
    Metadata      map[string]any
    ChunkingStrategy string
    ChunkSize     int
    ChunkOverlap  int
    Provider      string
    Model         string
}

// UploadDocumentResponse represents the upload response
type UploadDocumentResponse struct {
    DocumentID uuid.UUID
    JobID      uuid.UUID
    Status     string
}

// UploadDocument handles document uploads
type UploadDocument interface {
    Execute(ctx context.Context, req UploadDocumentRequest) (*UploadDocumentResponse, error)
}

// SearchRequest represents a search request
type SearchRequest struct {
    TenantID    uuid.UUID
    Query       string
    Provider    string
    Model       string
    Limit       int
    Offset      int
    Filters     map[string]any
    Threshold   float32
}

// SearchResponse represents search results
type SearchResponse struct {
    Results    []*SearchResult
    TotalCount int
    NextOffset int
}

// SearchSimilar performs vector similarity search
type SearchSimilar interface {
    Execute(ctx context.Context, req SearchRequest) (*SearchResponse, error)
}
```

### Provider Factory

```go
// providers/factory.go
package providers

import (
    "fmt"
    "sync"
)

// Factory creates provider instances
type Factory interface {
    GetProvider(name string) (Provider, error)
    RegisterProvider(name string, provider Provider)
    ListProviders() []string
}

// DefaultFactory implements the provider factory
type DefaultFactory struct {
    providers map[string]Provider
    mu        sync.RWMutex
}

// NewFactory creates a new provider factory
func NewFactory() *DefaultFactory {
    return &DefaultFactory{
        providers: make(map[string]Provider),
    }
}

// GetProvider retrieves a provider by name
func (f *DefaultFactory) GetProvider(name string) (Provider, error) {
    f.mu.RLock()
    defer f.mu.RUnlock()

    provider, ok := f.providers[name]
    if !ok {
        return nil, fmt.Errorf("provider %s not found", name)
    }

    return provider, nil
}

// RegisterProvider registers a new provider
func (f *DefaultFactory) RegisterProvider(name string, provider Provider) {
    f.mu.Lock()
    defer f.mu.Unlock()

    f.providers[name] = provider
}
```

## Implementation Guidelines

### Error Handling

```go
// errors.go
package embedding

import (
    "errors"
    "github.com/compozy/compozy/core"
)

// Sentinel errors
var (
    ErrDocumentNotFound = errors.New("document not found")
    ErrQuotaExceeded    = errors.New("quota exceeded")
    ErrInvalidDimension = errors.New("invalid embedding dimension")
    ErrProviderUnavailable = errors.New("provider unavailable")
)

// Error codes for structured errors
const (
    ErrorCodeDocumentNotFound = "DOCUMENT_NOT_FOUND"
    ErrorCodeQuotaExceeded    = "QUOTA_EXCEEDED"
    ErrorCodeInvalidProvider  = "INVALID_PROVIDER"
    ErrorCodeEmbeddingFailed  = "EMBEDDING_FAILED"
)

// NewDocumentNotFoundError creates a structured error for missing documents
func NewDocumentNotFoundError(id uuid.UUID) error {
    return core.NewError(
        ErrDocumentNotFound,
        ErrorCodeDocumentNotFound,
        map[string]any{"document_id": id},
    )
}

// NewQuotaExceededError creates a structured error for quota violations
func NewQuotaExceededError(tenantID uuid.UUID, provider string) error {
    return core.NewError(
        ErrQuotaExceeded,
        ErrorCodeQuotaExceeded,
        map[string]any{
            "tenant_id": tenantID,
            "provider":  provider,
        },
    )
}
```

### Validation

```go
// validators.go
package embedding

import (
    "fmt"
    "github.com/compozy/compozy/schema"
)

// ValidateUploadRequest validates document upload parameters
func ValidateUploadRequest(req UploadDocumentRequest) error {
    if req.ChunkSize < 100 || req.ChunkSize > 10000 {
        return fmt.Errorf("chunk size must be between 100 and 10000")
    }

    if req.ChunkOverlap < 0 || req.ChunkOverlap >= req.ChunkSize {
        return fmt.Errorf("chunk overlap must be less than chunk size")
    }

    if req.Provider == "" || req.Model == "" {
        return fmt.Errorf("provider and model are required")
    }

    return nil
}

// RegisterValidators registers embedding-specific validators
func RegisterValidators(cv *schema.CompositeValidator) {
    cv.RegisterValidator("chunkingStrategy", func(value any) error {
        strategy, ok := value.(string)
        if !ok {
            return fmt.Errorf("chunking strategy must be a string")
        }

        validStrategies := []string{"recursive", "semantic", "fixed"}
        for _, valid := range validStrategies {
            if strategy == valid {
                return nil
            }
        }

        return fmt.Errorf("invalid chunking strategy: %s", strategy)
    })
}
```

### Temporal Activities

```go
// activities/generate_batch.go
package activities

import (
    "context"
    "github.com/compozy/compozy/engine/embedding"
    "go.temporal.io/sdk/activity"
)

// GenerateBatchActivity generates embeddings for a batch of chunks
type GenerateBatchActivity struct {
    service *embedding.Service
}

// NewGenerateBatchActivity creates a new activity
func NewGenerateBatchActivity(service *embedding.Service) *GenerateBatchActivity {
    return &GenerateBatchActivity{service: service}
}

// ProcessBatchParams defines parameters for batch processing
type ProcessBatchParams struct {
    TenantID uuid.UUID
    ChunkIDs []uuid.UUID
    Provider string
    Model    string
}

// ProcessBatchResult defines the result of batch processing
type ProcessBatchResult struct {
    ProcessedCount int
    TokensUsed     int
    Cost           float64
    Errors         []string
}

// Execute processes a batch of chunks
func (a *GenerateBatchActivity) Execute(
    ctx context.Context,
    params ProcessBatchParams,
) (ProcessBatchResult, error) {
    logger := activity.GetLogger(ctx)
    logger.Info("Processing embedding batch",
        "tenant_id", params.TenantID,
        "chunk_count", len(params.ChunkIDs),
        "provider", params.Provider,
    )

    // Process embeddings
    err := a.service.GenerateEmbeddings(
        ctx,
        params.TenantID,
        params.ChunkIDs,
        params.Provider,
        params.Model,
    )

    if err != nil {
        // Check if it's a retryable error
        if errors.Is(err, embedding.ErrProviderUnavailable) {
            return ProcessBatchResult{}, temporal.NewApplicationError(
                "provider temporarily unavailable",
                "PROVIDER_UNAVAILABLE",
                err,
            )
        }

        // Non-retryable error
        return ProcessBatchResult{
            Errors: []string{err.Error()},
        }, nil
    }

    return ProcessBatchResult{
        ProcessedCount: len(params.ChunkIDs),
        // TODO: Get actual token count and cost from service
    }, nil
}
```

## Testing Strategy

### Unit Testing

```go
// service_test.go
package embedding_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/compozy/compozy/engine/embedding"
    "github.com/compozy/compozy/engine/embedding/testutil"
)

func TestService_GenerateEmbeddings(t *testing.T) {
    t.Run("Should generate embeddings successfully", func(t *testing.T) {
        // Arrange
        ctx := context.Background()
        mockRepo := testutil.NewMockEmbeddingRepository()
        mockProvider := testutil.NewMockProvider()

        service := embedding.NewService(
            mockRepo,
            mockProvider,
            // ... other dependencies
        )

        chunks := testutil.BuildChunks(3)
        mockProvider.On("Generate", mock.Anything, mock.Anything, "text-embedding-3-small").
            Return(testutil.BuildVectors(3, 1536), nil)

        // Act
        err := service.GenerateEmbeddings(ctx, tenantID, chunkIDs, "openai", "text-embedding-3-small")

        // Assert
        assert.NoError(t, err)
        mockProvider.AssertExpectations(t)
        mockRepo.AssertExpectations(t)
    })

    t.Run("Should handle quota exceeded error", func(t *testing.T) {
        // Test quota manager returning error
    })

    t.Run("Should deduplicate chunks before processing", func(t *testing.T) {
        // Test deduplication logic
    })
}
```

### Integration Testing

```go
// integration/embedding_test.go
//go:build integration

package integration_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/suite"
    "github.com/compozy/compozy/testutil"
)

type EmbeddingIntegrationSuite struct {
    suite.Suite
    db       *testutil.TestDB
    temporal *testutil.TestTemporal
}

func (s *EmbeddingIntegrationSuite) SetupSuite() {
    s.db = testutil.SetupTestDB()
    s.temporal = testutil.SetupTestTemporal()
}

func (s *EmbeddingIntegrationSuite) TestEndToEndEmbeddingGeneration() {
    // 1. Upload document
    // 2. Wait for Temporal workflow
    // 3. Verify embeddings stored
    // 4. Perform search
    // 5. Verify results
}

func TestEmbeddingIntegration(t *testing.T) {
    suite.Run(t, new(EmbeddingIntegrationSuite))
}
```

### Contract Testing

```go
// providers/contract_test.go
package providers_test

import (
    "context"
    "testing"
    "github.com/compozy/compozy/engine/embedding/providers"
)

// ProviderContractTest tests that all providers meet the contract
func ProviderContractTest(t *testing.T, provider providers.Provider) {
    ctx := context.Background()

    t.Run("Should return correct dimension", func(t *testing.T) {
        info, err := provider.GetModelInfo("default")
        assert.NoError(t, err)
        assert.Greater(t, info.Dimension, 0)
    })

    t.Run("Should handle empty input", func(t *testing.T) {
        _, err := provider.Generate(ctx, []string{}, "default")
        assert.Error(t, err)
    })

    t.Run("Should generate deterministic embeddings", func(t *testing.T) {
        text := "test embedding"
        vec1, _ := provider.Generate(ctx, []string{text}, "default")
        vec2, _ := provider.Generate(ctx, []string{text}, "default")

        assert.Equal(t, vec1, vec2, "embeddings should be deterministic")
    })
}

// Test each provider
func TestOpenAIProvider(t *testing.T) {
    provider := providers.NewOpenAIProvider(/* config */)
    ProviderContractTest(t, provider)
}
```

## Performance Architecture

### Connection Pooling

```go
// repository/postgres/config.go
package postgres

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

// Config holds PostgreSQL configuration
type Config struct {
    DatabaseURL      string
    MaxConns         int32
    MinConns         int32
    MaxConnLifetime  time.Duration
    MaxConnIdleTime  time.Duration
}

// NewPool creates a new connection pool
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
    poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
    if err != nil {
        return nil, err
    }

    poolConfig.MaxConns = cfg.MaxConns
    poolConfig.MinConns = cfg.MinConns
    poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
    poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

    // Performance optimizations
    poolConfig.ConnConfig.RuntimeParams["random_page_cost"] = "1.1"
    poolConfig.ConnConfig.RuntimeParams["effective_cache_size"] = "4GB"

    return pgxpool.NewWithConfig(ctx, poolConfig)
}
```

### Batch Processing

```go
// repository/postgres/embedding_repo.go
package postgres

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5"
)

// Store implements batch insertion with COPY
func (r *EmbeddingRepository) Store(ctx context.Context, embeddings []*embedding.Embedding) error {
    if len(embeddings) == 0 {
        return nil
    }

    // Group by dimension
    dimensionGroups := make(map[int][]*embedding.Embedding)
    for _, emb := range embeddings {
        dimensionGroups[emb.Dimension] = append(dimensionGroups[emb.Dimension], emb)
    }

    // Process each dimension group
    for dimension, group := range dimensionGroups {
        tableName := fmt.Sprintf("embeddings_%d", dimension)

        // Use COPY for bulk insert
        columns := []string{"id", "chunk_id", "tenant_id", "provider", "model", "model_version", "embedding", "created_at"}

        copyFrom := pgx.CopyFromSlice(len(group), func(i int) ([]any, error) {
            emb := group[i]
            return []any{
                emb.ID,
                emb.ChunkID,
                emb.TenantID,
                emb.Provider,
                emb.Model,
                emb.ModelVersion,
                pgvector.NewVector(emb.Vector),
                emb.CreatedAt,
            }, nil
        })

        _, err := r.pool.CopyFrom(ctx, pgx.Identifier{tableName}, columns, copyFrom)
        if err != nil {
            return fmt.Errorf("failed to copy embeddings: %w", err)
        }
    }

    return nil
}
```

### Caching Strategy

```go
// services/cache.go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    "github.com/redis/go-redis/v9"
)

// EmbeddingCache caches frequently accessed embeddings
type EmbeddingCache struct {
    client *redis.Client
    ttl    time.Duration
}

// NewEmbeddingCache creates a new cache
func NewEmbeddingCache(client *redis.Client, ttl time.Duration) *EmbeddingCache {
    return &EmbeddingCache{
        client: client,
        ttl:    ttl,
    }
}

// GetSearchResults retrieves cached search results
func (c *EmbeddingCache) GetSearchResults(
    ctx context.Context,
    queryHash string,
) ([]*embedding.SearchResult, error) {
    key := fmt.Sprintf("search:%s", queryHash)

    data, err := c.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    var results []*embedding.SearchResult
    if err := json.Unmarshal(data, &results); err != nil {
        return nil, err
    }

    return results, nil
}

// SetSearchResults caches search results
func (c *EmbeddingCache) SetSearchResults(
    ctx context.Context,
    queryHash string,
    results []*embedding.SearchResult,
) error {
    data, err := json.Marshal(results)
    if err != nil {
        return err
    }

    key := fmt.Sprintf("search:%s", queryHash)
    return c.client.Set(ctx, key, data, c.ttl).Err()
}
```

## Security Architecture

### Row Level Security

```sql
-- migrations/002_row_level_security.sql

-- Enable RLS on all tables
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_chunks ENABLE ROW LEVEL SECURITY;
ALTER TABLE embeddings_768 ENABLE ROW LEVEL SECURITY;
ALTER TABLE embeddings_1024 ENABLE ROW LEVEL SECURITY;
ALTER TABLE embeddings_1536 ENABLE ROW LEVEL SECURITY;

-- Create policies
CREATE POLICY tenant_isolation_documents ON documents
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::UUID);

CREATE POLICY tenant_isolation_chunks ON document_chunks
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::UUID);

-- Function to set current tenant
CREATE OR REPLACE FUNCTION set_current_tenant(tenant_id UUID)
RETURNS void AS $$
BEGIN
    PERFORM set_config('app.current_tenant', tenant_id::text, true);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
```

### Input Validation

```go
// validators/file_validator.go
package validators

import (
    "context"
    "fmt"
    "io"
)

// FileValidator validates file uploads
type FileValidator struct {
    maxFileSize  int64
    allowedTypes map[string]bool
}

// NewFileValidator creates a new validator
func NewFileValidator(maxSize int64, allowedTypes []string) *FileValidator {
    typeMap := make(map[string]bool)
    for _, t := range allowedTypes {
        typeMap[t] = true
    }

    return &FileValidator{
        maxFileSize:  maxSize,
        allowedTypes: typeMap,
    }
}

// Validate performs file validation
func (v *FileValidator) Validate(
    ctx context.Context,
    contentType string,
    size int64,
) error {
    // Check file size
    if size > v.maxFileSize {
        return fmt.Errorf("file size %d exceeds maximum %d", size, v.maxFileSize)
    }

    // Check content type
    if !v.allowedTypes[contentType] {
        return fmt.Errorf("content type %s not allowed", contentType)
    }

    return nil
}
```

## Development Standards

### Code Organization

1. **Package Structure**: Domain-driven, with clear boundaries
2. **File Naming**: Descriptive, following Go conventions
3. **Function Length**: Maximum 30 lines
4. **Cyclomatic Complexity**: Maximum 10
5. **Test Coverage**: Minimum 80%

### Git Workflow

```bash
# Feature branch naming
git checkout -b feature/COM-25-embeddings-core

# Commit message format
git commit -m "feat(embeddings): Add provider interface and OpenAI implementation

- Implement Provider interface with embedding generation
- Add OpenAI provider with rate limiting
- Include comprehensive unit tests
- Add contract tests for provider implementations

Refs: COM-25"
```

### Monitoring and Logging

```go
// Standard logging format
logger.Info("embedding.generation.started",
    "trace_id", traceID,
    "tenant_id", tenantID,
    "provider", provider,
    "model", model,
    "chunk_count", len(chunks),
)

// Metrics to track
metrics.RecordHistogram("embedding.generation.duration", duration,
    tag.Provider(provider),
    tag.Model(model),
)

metrics.RecordCount("embedding.generation.tokens", tokenCount,
    tag.TenantID(tenantID),
    tag.Provider(provider),
)
```

### Documentation Standards

```go
// Package embedding provides document embedding generation and vector search capabilities.
//
// The embedding package implements a clean architecture approach with clear separation
// of concerns between domain logic, use cases, and infrastructure. It supports multiple
// embedding providers (OpenAI, Cohere, Ollama) and stores vectors in PostgreSQL with
// pgvector extension.
//
// Example usage:
//
//	service := embedding.NewService(/* dependencies */)
//	err := service.GenerateEmbeddings(ctx, tenantID, chunkIDs, "openai", "text-embedding-3-small")
//
package embedding
```

## Conclusion

This technical architecture specification provides a comprehensive blueprint for implementing the Compozy Embeddings System. The design follows SOLID principles, Clean Architecture patterns, and aligns with Compozy's existing codebase while introducing necessary improvements for scalability, security, and maintainability.

Key architectural decisions ensure the system is:

- **Extensible**: New providers and chunking strategies can be added without modifying existing code
- **Testable**: Clear boundaries and dependency injection enable comprehensive testing
- **Scalable**: Dimension-specific tables, batch processing, and caching support growth
- **Secure**: Multi-tenancy with RLS, input validation, and security scanning
- **Maintainable**: Clear structure, consistent patterns, and comprehensive documentation

The implementation should proceed incrementally, following the phased approach outlined in the PRD, with continuous validation against these architectural guidelines.
