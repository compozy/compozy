# Memory Operations Service - Technical Implementation Plan

## Overview

This document provides detailed technical specifications for implementing the centralized memory operations service that will consolidate duplicated logic between `task/uc/exec_memory_operation.go` and `memory/uc/*.go` files.

## Current Architecture Analysis

### Source of Truth: `exec_memory_operation.go`

**Location**: `/engine/task/uc/exec_memory_operation.go`

**Key Components to Extract**:

```go
// 1. MemoryTransaction (lines 136-206) - Advanced transaction handling
type MemoryTransaction struct {
    mem     memcore.Memory
    backup  []llm.Message
    cleared bool
}

// 2. Core operation methods (lines 111-489)
func (uc *ExecuteMemoryOperation) executeRead(...)
func (uc *ExecuteMemoryOperation) executeWrite(...)
func (uc *ExecuteMemoryOperation) executeAppend(...)
func (uc *ExecuteMemoryOperation) executeDelete(...)
func (uc *ExecuteMemoryOperation) executeFlush(...)
func (uc *ExecuteMemoryOperation) executeHealth(...)
func (uc *ExecuteMemoryOperation) executeClear(...)
func (uc *ExecuteMemoryOperation) executeStats(...)

// 3. Helper functions (lines 493-614)
func (uc *ExecuteMemoryOperation) resolvePayload(...)
func (uc *ExecuteMemoryOperation) resolvePayloadRecursive(...)
func (uc *ExecuteMemoryOperation) validateMessageRole(...)
func (uc *ExecuteMemoryOperation) mapToMessage(...)
func (uc *ExecuteMemoryOperation) payloadToMessages(...)
```

### Duplicated Logic in `memory/uc/`

**Common Patterns to Consolidate**:

```go
// 1. Memory instance retrieval (repeated in all files)
memRef := core.MemoryReference{ID: memoryRef, Key: key}
workflowContext := map[string]any{"api_operation": "...", "key": key}
instance, err := manager.GetInstance(ctx, memRef, workflowContext)

// 2. Validation patterns (validation.go lines 30-123)
ValidateMemoryRef(ref string) error
ValidateKey(key string) error
ValidateRawMessages(messages []map[string]any) error
ValidateMessageRole(role string) error

// 3. Message conversion (helpers.go lines 9-32)
ConvertToLLMMessages(messages []map[string]any) ([]llm.Message, error)
```

## Service Architecture Design

### Directory Structure

```
engine/memory/service/
├── operations.go          # Main service interface and implementation
├── operations_test.go     # Comprehensive test suite
├── transaction.go         # MemoryTransaction logic (from exec_memory_operation.go)
├── validation.go          # Consolidated validation functions
├── conversion.go          # Message conversion utilities
├── types.go              # Service-specific types and interfaces
└── errors.go             # Service-specific error handling
```

### Core Service Interface

```go
package service

import (
    "context"

    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/llm"
    memcore "github.com/compozy/compozy/engine/memory/core"
    "github.com/compozy/compozy/engine/workflow"
    "github.com/compozy/compozy/pkg/tplengine"
)

// MemoryOperationsService provides centralized memory operations
type MemoryOperationsService interface {
    // Core operations
    Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error)
    Write(ctx context.Context, req *WriteRequest) (*WriteResponse, error)
    Append(ctx context.Context, req *AppendRequest) (*AppendResponse, error)
    Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error)

    // Advanced operations
    Flush(ctx context.Context, req *FlushRequest) (*FlushResponse, error)
    Clear(ctx context.Context, req *ClearRequest) (*ClearResponse, error)
    Health(ctx context.Context, req *HealthRequest) (*HealthResponse, error)
    Stats(ctx context.Context, req *StatsRequest) (*StatsResponse, error)
}

// Service implementation
type memoryOperationsService struct {
    memoryManager  memcore.ManagerInterface
    templateEngine *tplengine.TemplateEngine
}

func NewMemoryOperationsService(
    memoryManager memcore.ManagerInterface,
    templateEngine *tplengine.TemplateEngine,
) MemoryOperationsService {
    return &memoryOperationsService{
        memoryManager:  memoryManager,
        templateEngine: templateEngine,
    }
}
```

### Request/Response Types

```go
// types.go

// Common fields for all requests
type BaseRequest struct {
    MemoryRef string `json:"memory_ref"`
    Key       string `json:"key"`
}

// Read operation
type ReadRequest struct {
    BaseRequest
}

type ReadResponse struct {
    Messages []map[string]any `json:"messages"`
    Count    int              `json:"count"`
    Key      string           `json:"key"`
}

// Write operation
type WriteRequest struct {
    BaseRequest
    Payload       any             `json:"payload"`
    MergedInput   *core.Input     `json:"merged_input,omitempty"`
    WorkflowState *workflow.State `json:"workflow_state,omitempty"`
}

type WriteResponse struct {
    Success bool   `json:"success"`
    Count   int    `json:"count"`
    Key     string `json:"key"`
}

// Append operation
type AppendRequest struct {
    BaseRequest
    Payload       any             `json:"payload"`
    MergedInput   *core.Input     `json:"merged_input,omitempty"`
    WorkflowState *workflow.State `json:"workflow_state,omitempty"`
}

type AppendResponse struct {
    Success    bool   `json:"success"`
    Appended   int    `json:"appended"`
    TotalCount int    `json:"total_count"`
    Key        string `json:"key"`
}

// Delete operation
type DeleteRequest struct {
    BaseRequest
}

type DeleteResponse struct {
    Success bool   `json:"success"`
    Key     string `json:"key"`
}

// Flush operation
type FlushRequest struct {
    BaseRequest
    Config *FlushConfig `json:"config,omitempty"`
}

type FlushResponse struct {
    Success          bool   `json:"success"`
    Key              string `json:"key"`
    SummaryGenerated bool   `json:"summary_generated"`
    MessageCount     int    `json:"message_count"`
    TokenCount       int    `json:"token_count"`
    DryRun           bool   `json:"dry_run,omitempty"`
    WouldFlush       bool   `json:"would_flush,omitempty"`
    FlushStrategy    string `json:"flush_strategy,omitempty"`
    Error            string `json:"error,omitempty"`
}

// Clear operation
type ClearRequest struct {
    BaseRequest
    Config *ClearConfig `json:"config,omitempty"`
}

type ClearResponse struct {
    Success         bool   `json:"success"`
    Key             string `json:"key"`
    MessagesCleared int    `json:"messages_cleared"`
    BackupCreated   bool   `json:"backup_created"`
}

// Health operation
type HealthRequest struct {
    BaseRequest
    Config *HealthConfig `json:"config,omitempty"`
}

type HealthResponse struct {
    Healthy       bool   `json:"healthy"`
    Key           string `json:"key"`
    TokenCount    int    `json:"token_count"`
    MessageCount  int    `json:"message_count"`
    FlushStrategy string `json:"flush_strategy"`
    LastFlush     string `json:"last_flush,omitempty"`
    CurrentTokens int    `json:"current_tokens,omitempty"`
}

// Stats operation
type StatsRequest struct {
    BaseRequest
    Config *StatsConfig `json:"config,omitempty"`
}

type StatsResponse struct {
    Key                   string `json:"key"`
    MessageCount          int    `json:"message_count"`
    TokenCount            int    `json:"token_count"`
    FlushStrategy         string `json:"flush_strategy"`
    LastFlush             string `json:"last_flush,omitempty"`
    AvgTokensPerMessage   int    `json:"avg_tokens_per_message,omitempty"`
}

// Configuration types (reuse from task package)
type FlushConfig struct {
    Strategy  string  `json:"strategy"`
    MaxKeys   int     `json:"max_keys"`
    DryRun    bool    `json:"dry_run"`
    Force     bool    `json:"force"`
    Threshold float64 `json:"threshold"`
}

type ClearConfig struct {
    Confirm bool `json:"confirm"`
    Backup  bool `json:"backup"`
}

type HealthConfig struct {
    IncludeStats      bool `json:"include_stats"`
    CheckConnectivity bool `json:"check_connectivity"`
}

type StatsConfig struct {
    IncludeContent bool   `json:"include_content"`
    GroupBy        string `json:"group_by"`
}
```

## Implementation Details

### Phase 1: Core Service Creation

#### Step 1: Create `transaction.go`

Copy and adapt `MemoryTransaction` from `exec_memory_operation.go`:

```go
// transaction.go
package service

// MemoryTransaction provides transactional operations for memory modifications
type MemoryTransaction struct {
    mem     memcore.Memory
    backup  []llm.Message
    cleared bool
}

func NewMemoryTransaction(mem memcore.Memory) *MemoryTransaction {
    return &MemoryTransaction{mem: mem}
}

func (t *MemoryTransaction) Begin(ctx context.Context) error {
    // Copy logic from exec_memory_operation.go lines 150-158
}

func (t *MemoryTransaction) Clear(ctx context.Context) error {
    // Copy logic from exec_memory_operation.go lines 161-167
}

func (t *MemoryTransaction) Commit() error {
    // Copy logic from exec_memory_operation.go lines 170-175
}

func (t *MemoryTransaction) Rollback(ctx context.Context) error {
    // Copy logic from exec_memory_operation.go lines 178-196
}

func (t *MemoryTransaction) ApplyMessages(ctx context.Context, messages []llm.Message) error {
    // Copy logic from exec_memory_operation.go lines 199-206
}
```

#### Step 2: Create `validation.go`

Consolidate validation functions from `memory/uc/validation.go`:

```go
// validation.go
package service

import (
    "fmt"
    "regexp"

    "github.com/compozy/compozy/engine/llm"
)

var (
    memRefPattern = regexp.MustCompile(`^[a-zA-Z0-9_]{1,100}$`)
    keyPattern    = regexp.MustCompile(`^[^\x00-\x1F\x7F]{1,255}$`)
)

const (
    MaxMemoryRefLength      = 100
    MaxKeyLength            = 255
    MaxMessageContentLength = 10 * 1024
    MaxMessagesPerRequest   = 100
    MaxTotalContentSize     = 100 * 1024
)

func ValidateMemoryRef(ref string) error {
    // Copy logic from memory/uc/validation.go lines 31-39
}

func ValidateKey(key string) error {
    // Copy logic from memory/uc/validation.go lines 42-51
}

func ValidateRawMessages(messages []map[string]any) error {
    // Copy logic from memory/uc/validation.go lines 54-85
}

func ValidateMessageRole(role string) error {
    // Copy logic from memory/uc/validation.go lines 115-123
}

func ValidateBaseRequest(req *BaseRequest) error {
    if err := ValidateMemoryRef(req.MemoryRef); err != nil {
        return err
    }
    return ValidateKey(req.Key)
}
```

#### Step 3: Create `conversion.go`

Consolidate message conversion functions:

```go
// conversion.go
package service

import (
    "fmt"

    "github.com/compozy/compozy/engine/llm"
)

func ConvertToLLMMessages(messages []map[string]any) ([]llm.Message, error) {
    // Copy logic from memory/uc/helpers.go lines 11-32
}

func MapToMessage(msg map[string]any) (llm.Message, error) {
    // Copy logic from exec_memory_operation.go lines 565-579
}

func PayloadToMessages(payload any) ([]llm.Message, error) {
    // Copy logic from exec_memory_operation.go lines 581-614
}
```

#### Step 4: Create `operations.go`

Main service implementation:

```go
// operations.go
package service

func (s *memoryOperationsService) Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error) {
    // Validate request
    if err := ValidateBaseRequest(&req.BaseRequest); err != nil {
        return nil, err
    }

    // Get memory instance
    instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "read")
    if err != nil {
        return nil, err
    }

    // Copy logic from exec_memory_operation.go executeRead (lines 111-133)
}

func (s *memoryOperationsService) Write(ctx context.Context, req *WriteRequest) (*WriteResponse, error) {
    // Validate request
    if err := ValidateBaseRequest(&req.BaseRequest); err != nil {
        return nil, err
    }

    // Get memory instance
    instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "write")
    if err != nil {
        return nil, err
    }

    // Copy logic from exec_memory_operation.go executeWrite (lines 208-259)
    // Use MemoryTransaction for atomic operations
}

func (s *memoryOperationsService) Append(ctx context.Context, req *AppendRequest) (*AppendResponse, error) {
    // Similar pattern for append operation
}

// ... implement all other operations

// Helper methods
func (s *memoryOperationsService) getMemoryInstance(ctx context.Context, memoryRef, key, operation string) (memcore.Memory, error) {
    memRef := core.MemoryReference{
        ID:  memoryRef,
        Key: key,
    }

    workflowContext := map[string]any{
        "api_operation": operation,
        "key":           key,
    }

    return s.memoryManager.GetInstance(ctx, memRef, workflowContext)
}

func (s *memoryOperationsService) resolvePayload(payload any, mergedInput *core.Input, workflowState *workflow.State) (any, error) {
    // Copy logic from exec_memory_operation.go lines 493-515
}
```

### Phase 2: Comprehensive Testing

#### Test Structure

```go
// operations_test.go
package service

import (
    "context"
    "testing"

    "github.com/stretchr/testify/suite"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
)

type MemoryOperationsServiceTestSuite struct {
    suite.Suite
    service       MemoryOperationsService
    mockManager   *MockMemoryManager
    mockTemplate  *MockTemplateEngine
    mockMemory    *MockMemory
    ctx           context.Context
}

func (suite *MemoryOperationsServiceTestSuite) SetupTest() {
    suite.ctx = context.Background()
    suite.mockManager = &MockMemoryManager{}
    suite.mockTemplate = &MockTemplateEngine{}
    suite.mockMemory = &MockMemory{}

    suite.service = NewMemoryOperationsService(
        suite.mockManager,
        suite.mockTemplate,
    )
}

func (suite *MemoryOperationsServiceTestSuite) TestRead_Success() {
    // Test successful read operation
}

func (suite *MemoryOperationsServiceTestSuite) TestWrite_AtomicSuccess() {
    // Test atomic write with transaction
}

func (suite *MemoryOperationsServiceTestSuite) TestWrite_RollbackOnFailure() {
    // Test transaction rollback on failure
}

func (suite *MemoryOperationsServiceTestSuite) TestAppend_ConcurrentOperations() {
    // Test concurrent append operations
}

// ... implement tests for all operations

func TestMemoryOperationsServiceSuite(t *testing.T) {
    suite.Run(t, new(MemoryOperationsServiceTestSuite))
}
```

### Phase 3: Integration Strategy

#### Step 1: Update `exec_memory_operation.go`

```go
// Modify exec_memory_operation.go to use service
type ExecuteMemoryOperation struct {
    memoryService service.MemoryOperationsService
}

func (uc *ExecuteMemoryOperation) Execute(ctx context.Context, input *ExecuteMemoryOperationInput) (*core.Output, error) {
    // Convert task config to service request
    switch input.TaskConfig.Operation {
    case task.MemoryOpRead:
        req := &service.ReadRequest{
            BaseRequest: service.BaseRequest{
                MemoryRef: input.TaskConfig.MemoryRef,
                Key:       input.TaskConfig.KeyTemplate,
            },
        }
        resp, err := uc.memoryService.Read(ctx, req)
        if err != nil {
            return nil, err
        }
        return &core.Output{
            "messages": resp.Messages,
            "count":    resp.Count,
            "key":      resp.Key,
        }, nil

    case task.MemoryOpWrite:
        req := &service.WriteRequest{
            BaseRequest: service.BaseRequest{
                MemoryRef: input.TaskConfig.MemoryRef,
                Key:       input.TaskConfig.KeyTemplate,
            },
            Payload:       input.TaskConfig.Payload,
            MergedInput:   input.MergedInput,
            WorkflowState: input.WorkflowState,
        }
        resp, err := uc.memoryService.Write(ctx, req)
        // Convert response...

    // ... handle all other operations
    }
}
```

#### Step 2: Replace `memory/uc/*.go` files

For each file, replace with thin wrapper:

```go
// append_memory.go (new implementation)
package uc

import (
    "context"

    "github.com/compozy/compozy/engine/memory/service"
)

type AppendMemory struct {
    service service.MemoryOperationsService
}

func NewAppendMemory(service service.MemoryOperationsService) *AppendMemory {
    return &AppendMemory{service: service}
}

func (uc *AppendMemory) Execute(ctx context.Context, memoryRef, key string, input *AppendMemoryInput) (*AppendMemoryResult, error) {
    req := &service.AppendRequest{
        BaseRequest: service.BaseRequest{
            MemoryRef: memoryRef,
            Key:       key,
        },
        Payload: convertInputToPayload(input),
    }

    resp, err := uc.service.Append(ctx, req)
    if err != nil {
        return nil, err
    }

    return &AppendMemoryResult{
        Success:    resp.Success,
        Appended:   resp.Appended,
        TotalCount: resp.TotalCount,
        Key:        resp.Key,
    }, nil
}
```

## Migration Checklist

### Pre-Implementation

- [ ] Review all duplicated code in `memory/uc/` files
- [ ] Identify all interfaces and types that need to be preserved
- [ ] Create backup of current implementation
- [ ] Set up feature flags for gradual migration

### Phase 1: Service Creation

- [ ] Create `engine/memory/service/` directory
- [ ] Implement `transaction.go` with MemoryTransaction
- [ ] Implement `validation.go` with consolidated validation
- [ ] Implement `conversion.go` with message conversion
- [ ] Implement `operations.go` with main service logic
- [ ] Implement `types.go` with request/response types
- [ ] Create comprehensive test suite

### Phase 2: Integration

- [ ] Update `exec_memory_operation.go` to use service
- [ ] Replace `append_memory.go`
- [ ] Replace `write_memory.go`
- [ ] Replace `delete_memory.go`
- [ ] Replace `read_memory.go`
- [ ] Replace `clear_memory.go`
- [ ] Replace `flush_memory.go`

### Phase 3: Validation

- [ ] Run all existing tests
- [ ] Run integration tests
- [ ] Performance benchmarking
- [ ] Code review and cleanup
- [ ] Remove old implementations
- [ ] Update documentation

## Success Metrics

1. **Code Duplication**: 0% duplicated memory operation logic
2. **Test Coverage**: >95% coverage for service package
3. **Performance**: No regression in operation latency
4. **Atomic Operations**: All write operations use transactions
5. **Interface Compliance**: Proper use of memcore interfaces

## Risk Mitigation

1. **Feature Flags**: Enable gradual rollout with fallback
2. **Comprehensive Testing**: Unit, integration, and performance tests
3. **Backward Compatibility**: Maintain existing API contracts
4. **Monitoring**: Add metrics for operation success/failure rates
5. **Rollback Plan**: Keep old implementations until migration is complete

---

This technical plan provides the detailed roadmap for implementing the centralized memory operations service, ensuring we eliminate code duplication while maintaining all existing functionality and improving transaction safety.
