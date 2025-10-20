package service

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance/strategies"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Ensure memoryOperationsService implements MemoryOperationsService at compile time
var _ MemoryOperationsService = (*memoryOperationsService)(nil)

// memoryOperationsService implements MemoryOperationsService
type memoryOperationsService struct {
	memoryManager     memcore.ManagerInterface
	templateEngine    *tplengine.TemplateEngine
	tokenCounter      memcore.TokenCounter
	strategyFactory   *strategies.StrategyFactory
	config            *Config
	tracer            trace.Tracer
	meter             metric.Meter
	operationCount    metric.Int64Counter
	operationDuration metric.Float64Histogram
	workflowContext   map[string]any
}

// NewMemoryOperationsService creates a new memory operations service
func NewMemoryOperationsService(
	memoryManager memcore.ManagerInterface,
	templateEngine *tplengine.TemplateEngine,
	tokenCounter memcore.TokenCounter,
	config *Config,
	workflowContext map[string]any,
) (MemoryOperationsService, error) {
	if memoryManager == nil {
		return nil, fmt.Errorf("memoryManager is required")
	}
	if config == nil {
		config = DefaultConfig()
	}
	tracer := otel.Tracer("memory.service")
	meter := otel.Meter("memory.service")
	// Create metrics
	operationCount, err := meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem("memory", "operations_total"),
		metric.WithDescription("Total number of memory operations"),
	)
	if err != nil {
		// Log error but continue - metrics are not critical for core functionality
		operationCount = nil
	}
	operationDuration, err := meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("memory", "operation_duration_seconds"),
		metric.WithDescription("Duration of memory operations in seconds"),
	)
	if err != nil {
		// Log error but continue - metrics are not critical for core functionality
		operationDuration = nil
	}
	return &memoryOperationsService{
		memoryManager:     memoryManager,
		templateEngine:    templateEngine,
		tokenCounter:      tokenCounter,
		strategyFactory:   strategies.NewStrategyFactoryWithTokenCounter(tokenCounter),
		config:            config,
		tracer:            tracer,
		meter:             meter,
		operationCount:    operationCount,
		operationDuration: operationDuration,
		workflowContext:   workflowContext,
	}, nil
}

// Read executes a memory read operation
func (s *memoryOperationsService) Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error) {
	ctx, span := s.tracer.Start(ctx, "memory.service.Read")
	defer span.End()
	// Add attributes
	span.SetAttributes(
		attribute.String("memory_ref", req.MemoryRef),
		attribute.String("key", req.Key),
	)
	// Record operation
	if s.operationCount != nil {
		s.operationCount.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", "read"),
			attribute.String("memory_ref", req.MemoryRef),
		))
	}
	// Validate request
	if err := ValidateBaseRequestWithLimits(&req.BaseRequest, &s.config.ValidationLimits); err != nil {
		return nil, err
	}
	// Get memory instance
	instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "read")
	if err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeMemoryRead,
			"failed to get memory instance",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	// Read messages
	messages, err := instance.Read(ctx)
	if err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeMemoryRead,
			"failed to read memory",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	span.SetAttributes(attribute.Int("message_count", len(messages)))
	return &ReadResponse{
		Messages: messages,
		Count:    len(messages),
		Key:      req.Key,
	}, nil
}

// ReadPaginated executes a memory read operation with pagination
func (s *memoryOperationsService) ReadPaginated(
	ctx context.Context,
	req *ReadPaginatedRequest,
) (*ReadPaginatedResponse, error) {
	ctx, span := s.startOperationSpan(
		ctx,
		"memory.service.ReadPaginated",
		attribute.String("memory_ref", req.MemoryRef),
		attribute.String("key", req.Key),
		attribute.Int("offset", req.Offset),
		attribute.Int("limit", req.Limit),
	)
	defer span.End()
	s.recordOperationMetric(ctx, "read_paginated", req.MemoryRef)
	if err := s.validateReadPaginatedRequest(req); err != nil {
		return nil, err
	}
	messages, totalCount, err := s.readPaginatedMessages(ctx, req)
	if err != nil {
		return nil, err
	}
	hasMore := (req.Offset + len(messages)) < totalCount
	span.SetAttributes(
		attribute.Int("message_count", len(messages)),
		attribute.Int("total_count", totalCount),
		attribute.Bool("has_more", hasMore),
	)
	return &ReadPaginatedResponse{
		Messages:   messages,
		Count:      len(messages),
		TotalCount: totalCount,
		Offset:     req.Offset,
		Limit:      req.Limit,
		HasMore:    hasMore,
		Key:        req.Key,
	}, nil
}

// Write executes a memory write operation with atomic transaction
func (s *memoryOperationsService) Write(ctx context.Context, req *WriteRequest) (*WriteResponse, error) {
	ctx, span := s.startOperationSpan(
		ctx,
		"memory.service.Write",
		attribute.String("memory_ref", req.MemoryRef),
		attribute.String("key", req.Key),
	)
	defer span.End()
	s.recordOperationMetric(ctx, "write", req.MemoryRef)
	if err := s.validateWriteRequest(req); err != nil {
		return nil, err
	}
	instance, messages, err := s.prepareWriteResources(ctx, req)
	if err != nil {
		return nil, err
	}
	if atomicInstance, ok := instance.(memcore.AtomicOperations); ok {
		return s.performAtomicWrite(ctx, atomicInstance, req.Key, messages)
	}
	return s.performTransactionalWrite(ctx, instance, req.Key, messages)
}

// Append executes a memory append operation
func (s *memoryOperationsService) Append(ctx context.Context, req *AppendRequest) (*AppendResponse, error) {
	ctx, span := s.tracer.Start(ctx, "memory.service.Append")
	defer span.End()
	s.recordAppendMetrics(ctx, span, req)
	if err := ValidateBaseRequestWithLimits(&req.BaseRequest, &s.config.ValidationLimits); err != nil {
		return nil, err
	}
	instance, messages, beforeCount, err := s.prepareAppendOperation(ctx, req)
	if err != nil {
		return nil, err
	}
	totalCount, err := s.executeAppendTransaction(ctx, instance, messages, req)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(
		attribute.Int("appended_count", totalCount-beforeCount),
		attribute.Int("total_count", totalCount),
	)
	return &AppendResponse{
		Success:    true,
		Appended:   totalCount - beforeCount,
		TotalCount: totalCount,
		Key:        req.Key,
	}, nil
}

// AppendMany executes an atomic append of multiple messages derived from the payload
func (s *memoryOperationsService) AppendMany(ctx context.Context, req *AppendRequest) (*AppendResponse, error) {
	ctx, span := s.startOperationSpan(
		ctx,
		"memory.service.AppendMany",
		attribute.String("memory_ref", req.MemoryRef),
		attribute.String("key", req.Key),
	)
	defer span.End()
	s.recordOperationMetric(ctx, "append_many", req.MemoryRef)
	if err := ValidateBaseRequestWithLimits(&req.BaseRequest, &s.config.ValidationLimits); err != nil {
		return nil, err
	}
	instance, messages, beforeCount, err := s.prepareAppendOperation(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return s.appendNoopResponse(span, beforeCount, req.Key), nil
	}
	if err := instance.AppendMany(ctx, messages); err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeMemoryAppend,
			"append_many failed, no partial writes expected",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return s.appendSuccessResponse(ctx, span, instance, beforeCount, req)
}

// Delete executes a memory delete operation
func (s *memoryOperationsService) Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error) {
	ctx, span := s.tracer.Start(ctx, "memory.service.Delete")
	defer span.End()
	// Add attributes
	span.SetAttributes(
		attribute.String("memory_ref", req.MemoryRef),
		attribute.String("key", req.Key),
	)
	// Record operation
	if s.operationCount != nil {
		s.operationCount.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", "delete"),
			attribute.String("memory_ref", req.MemoryRef),
		))
	}
	// Validate request
	if err := ValidateBaseRequestWithLimits(&req.BaseRequest, &s.config.ValidationLimits); err != nil {
		return nil, err
	}
	// Get memory instance
	instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "delete")
	if err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to get memory instance",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	// Clear all messages (delete operation)
	if err := instance.Clear(ctx); err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeMemoryClear,
			"failed to delete memory",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return &DeleteResponse{
		Success: true,
		Key:     req.Key,
	}, nil
}

// Flush executes a memory flush operation
func (s *memoryOperationsService) Flush(ctx context.Context, req *FlushRequest) (*FlushResponse, error) {
	ctx, span := s.tracer.Start(ctx, "memory.service.Flush")
	defer span.End()
	s.recordFlushMetrics(ctx, span, req)
	if err := s.validateFlushRequest(req); err != nil {
		return nil, err
	}
	flushableMem, err := s.prepareFlushOperation(ctx, req)
	if err != nil {
		return nil, err
	}
	if req.Config != nil && req.Config.DryRun {
		return s.performDryRunFlush(ctx, flushableMem, req)
	}
	return s.performActualFlush(ctx, flushableMem, req)
}

// Clear executes a memory clear operation
func (s *memoryOperationsService) Clear(ctx context.Context, req *ClearRequest) (*ClearResponse, error) {
	ctx, span := s.startOperationSpan(
		ctx,
		"memory.service.Clear",
		attribute.String("memory_ref", req.MemoryRef),
		attribute.String("key", req.Key),
	)
	defer span.End()
	s.recordOperationMetric(ctx, "clear", req.MemoryRef)
	if err := s.validateClearRequest(req); err != nil {
		return nil, err
	}
	clearedCount, err := s.performClearOperation(ctx, req)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int("messages_cleared", clearedCount))
	return &ClearResponse{
		Success:         true,
		Key:             req.Key,
		MessagesCleared: clearedCount,
		BackupCreated:   false,
	}, nil
}

// Health executes a memory health check operation
func (s *memoryOperationsService) Health(ctx context.Context, req *HealthRequest) (*HealthResponse, error) {
	ctx, span := s.startOperationSpan(
		ctx,
		"memory.service.Health",
		attribute.String("memory_ref", req.MemoryRef),
		attribute.String("key", req.Key),
	)
	defer span.End()
	s.recordOperationMetric(ctx, "health", req.MemoryRef)
	config, err := s.validateHealthRequest(req)
	if err != nil {
		return nil, err
	}
	instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "health")
	if err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to get memory instance",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	health, err := instance.GetMemoryHealth(ctx)
	if err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to get memory health",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	result := buildHealthResponse(req, health)
	if config.IncludeStats {
		s.populateHealthStats(ctx, instance, result)
	}
	return result, nil
}

// Stats executes a memory stats operation
func (s *memoryOperationsService) Stats(ctx context.Context, req *StatsRequest) (*StatsResponse, error) {
	ctx, span := s.startOperationSpan(
		ctx,
		"memory.service.Stats",
		attribute.String("memory_ref", req.MemoryRef),
		attribute.String("key", req.Key),
	)
	defer span.End()
	s.recordOperationMetric(ctx, "stats", req.MemoryRef)
	config, err := s.validateStatsRequest(req)
	if err != nil {
		return nil, err
	}
	instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "stats")
	if err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to get memory instance",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	messageCount, tokenCount, health, err := s.collectStatsData(ctx, instance, req)
	if err != nil {
		return nil, err
	}
	return buildStatsResponse(req, messageCount, tokenCount, health, config.IncludeContent), nil
}

// Helper methods

func (s *memoryOperationsService) startOperationSpan(
	ctx context.Context,
	name string,
	attrs ...attribute.KeyValue,
) (context.Context, trace.Span) {
	ctx, span := s.tracer.Start(ctx, name)
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return ctx, span
}

func (s *memoryOperationsService) recordOperationMetric(ctx context.Context, operation, memoryRef string) {
	if s.operationCount == nil {
		return
	}
	s.operationCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", operation),
		attribute.String("memory_ref", memoryRef),
	))
}

func (s *memoryOperationsService) validateReadPaginatedRequest(req *ReadPaginatedRequest) error {
	if err := ValidateBaseRequestWithLimits(&req.BaseRequest, &s.config.ValidationLimits); err != nil {
		return err
	}
	return ValidatePaginationParams(req.Offset, req.Limit)
}

func (s *memoryOperationsService) readPaginatedMessages(
	ctx context.Context,
	req *ReadPaginatedRequest,
) ([]llm.Message, int, error) {
	instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "read_paginated")
	if err != nil {
		return nil, 0, memcore.NewMemoryError(
			memcore.ErrCodeMemoryRead,
			"failed to get memory instance",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	messages, totalCount, err := instance.ReadPaginated(ctx, req.Offset, req.Limit)
	if err != nil {
		return nil, 0, memcore.NewMemoryError(
			memcore.ErrCodeMemoryRead,
			"failed to read memory with pagination",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return messages, totalCount, nil
}

func (s *memoryOperationsService) validateWriteRequest(req *WriteRequest) error {
	if err := ValidateBaseRequestWithLimits(&req.BaseRequest, &s.config.ValidationLimits); err != nil {
		return err
	}
	if err := ValidatePayloadType(req.Payload); err != nil {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"invalid payload type",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return nil
}

func (s *memoryOperationsService) prepareWriteResources(
	ctx context.Context,
	req *WriteRequest,
) (memcore.Memory, []llm.Message, error) {
	instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "write")
	if err != nil {
		return nil, nil, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to get memory instance",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	resolvedPayload, err := s.resolvePayload(req.Payload, req.MergedInput, req.WorkflowState)
	if err != nil {
		return nil, nil, memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"failed to resolve payload",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	messages, err := PayloadToMessagesWithLimits(resolvedPayload, &s.config.ValidationLimits)
	if err != nil {
		return nil, nil, memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"failed to convert payload to messages",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return instance, messages, nil
}

func (s *memoryOperationsService) validateClearRequest(req *ClearRequest) error {
	if err := ValidateBaseRequestWithLimits(&req.BaseRequest, &s.config.ValidationLimits); err != nil {
		return err
	}
	return ValidateClearConfig(req.Config)
}

func (s *memoryOperationsService) performClearOperation(
	ctx context.Context,
	req *ClearRequest,
) (int, error) {
	instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "clear")
	if err != nil {
		return 0, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to get memory instance",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	beforeCount, err := instance.Len(ctx)
	if err != nil {
		return 0, memcore.NewMemoryError(
			memcore.ErrCodeMemoryClear,
			"failed to get message count before clear",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	if err := instance.Clear(ctx); err != nil {
		return 0, memcore.NewMemoryError(
			memcore.ErrCodeMemoryClear,
			"failed to clear memory",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return beforeCount, nil
}

func (s *memoryOperationsService) validateStatsRequest(req *StatsRequest) (*StatsConfig, error) {
	if err := ValidateBaseRequestWithLimits(&req.BaseRequest, &s.config.ValidationLimits); err != nil {
		return nil, err
	}
	if req.Config == nil {
		req.Config = &StatsConfig{}
	}
	return req.Config, nil
}

func (s *memoryOperationsService) collectStatsData(
	ctx context.Context,
	instance memcore.Memory,
	req *StatsRequest,
) (int, int, *memcore.Health, error) {
	messageCount, err := instance.Len(ctx)
	if err != nil {
		return 0, 0, nil, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to get message count",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	tokenCount, err := instance.GetTokenCount(ctx)
	if err != nil {
		return 0, 0, nil, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to get token count",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	health, err := instance.GetMemoryHealth(ctx)
	if err != nil {
		return 0, 0, nil, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to get memory health",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return messageCount, tokenCount, health, nil
}

func buildStatsResponse(
	req *StatsRequest,
	messageCount int,
	tokenCount int,
	health *memcore.Health,
	includeContent bool,
) *StatsResponse {
	response := &StatsResponse{
		Key:            req.Key,
		MessageCount:   messageCount,
		TokenCount:     tokenCount,
		ActualStrategy: health.ActualStrategy,
	}
	if health.LastFlush != nil {
		response.LastFlush = health.LastFlush.Format("2006-01-02T15:04:05Z07:00")
	}
	if includeContent && messageCount > 0 {
		response.AvgTokensPerMessage = tokenCount / messageCount
	}
	return response
}

func requestedFlushStrategy(req *FlushRequest) string {
	if req.Config != nil && req.Config.Strategy != "" {
		return req.Config.Strategy
	}
	return ""
}

func (s *memoryOperationsService) executeFlush(
	ctx context.Context,
	flushable memcore.FlushableMemory,
	requestedStrategy string,
	req *FlushRequest,
) (*memcore.FlushMemoryActivityOutput, string, error) {
	if dynamicFlush, ok := flushable.(memcore.DynamicFlushableMemory); ok {
		result, err := dynamicFlush.PerformFlushWithStrategy(ctx, memcore.FlushingStrategyType(requestedStrategy))
		if err != nil {
			return nil, "", err
		}
		actual := requestedStrategy
		if actual == "" {
			actual = string(dynamicFlush.GetConfiguredStrategy())
		}
		return result, actual, nil
	}
	result, err := flushable.PerformFlush(ctx)
	if err != nil {
		return nil, "", err
	}
	actual := s.fallbackFlushStrategy(ctx, flushable, req)
	return result, actual, nil
}

func (s *memoryOperationsService) fallbackFlushStrategy(
	ctx context.Context,
	flushable memcore.FlushableMemory,
	req *FlushRequest,
) string {
	health, err := flushable.GetMemoryHealth(ctx)
	if err != nil {
		logger.FromContext(ctx).Warn("Failed to get memory health for strategy info",
			"error", err,
			"memory_ref", req.MemoryRef,
			"key", req.Key)
		return "unknown"
	}
	if health == nil {
		return "unknown"
	}
	return health.ActualStrategy
}

func buildFlushResponse(
	result *memcore.FlushMemoryActivityOutput,
	key string,
	actualStrategy string,
) *FlushResponse {
	return &FlushResponse{
		Success:          result.Success,
		Key:              key,
		SummaryGenerated: result.SummaryGenerated,
		MessageCount:     result.MessageCount,
		TokenCount:       result.TokenCount,
		ActualStrategy:   actualStrategy,
	}
}

func (s *memoryOperationsService) validateHealthRequest(req *HealthRequest) (*HealthConfig, error) {
	if err := ValidateBaseRequestWithLimits(&req.BaseRequest, &s.config.ValidationLimits); err != nil {
		return nil, err
	}
	if req.Config == nil {
		req.Config = &HealthConfig{}
	}
	return req.Config, nil
}

func buildHealthResponse(req *HealthRequest, health *memcore.Health) *HealthResponse {
	response := &HealthResponse{
		Healthy:        true,
		Key:            req.Key,
		TokenCount:     health.TokenCount,
		MessageCount:   health.MessageCount,
		ActualStrategy: health.ActualStrategy,
	}
	if health.LastFlush != nil {
		response.LastFlush = health.LastFlush.Format("2006-01-02T15:04:05Z07:00")
	}
	return response
}

func (s *memoryOperationsService) populateHealthStats(
	ctx context.Context,
	instance memcore.Memory,
	resp *HealthResponse,
) {
	tokenCount, err := instance.GetTokenCount(ctx)
	if err == nil {
		resp.CurrentTokens = tokenCount
	}
}

// dereferenceInput safely dereferences the workflow input pointer for template resolution
func dereferenceInput(input *core.Input) any {
	if input == nil {
		return nil
	}
	// Dereference the pointer to expose the underlying map
	// This allows templates to access nested fields like .workflow.input.user_id
	return *input
}

// getMemoryInstance retrieves a memory instance for the given parameters
func (s *memoryOperationsService) getMemoryInstance(
	ctx context.Context,
	memoryRef, key, operation string,
) (memcore.Memory, error) {
	// For REST API operations, we use the explicit key provided by the client
	// rather than any template defined in the memory resource configuration.
	// This allows external systems to manage memory with explicit keys.
	// The key can be any format the user chooses: "user:123", "session:abc",
	// "cache:data", "my-custom-key", etc.
	memRef := core.MemoryReference{
		ID:          memoryRef,
		ResolvedKey: key, // Use ResolvedKey for explicit keys from REST API
	}
	// Use the workflow context from the service if available, or create a minimal one
	workflowContext := s.workflowContext
	if workflowContext == nil {
		workflowContext = map[string]any{
			"api_operation": operation,
		}
	}
	return s.memoryManager.GetInstance(ctx, memRef, workflowContext)
}

// resolvePayload resolves template placeholders in payload
func (s *memoryOperationsService) resolvePayload(
	payload any,
	mergedInput *core.Input,
	workflowState *workflow.State,
) (any, error) {
	if payload == nil {
		return nil, nil
	}
	// If no template engine or workflow state, return payload as-is
	if s.templateEngine == nil || workflowState == nil {
		return payload, nil
	}
	// Build context for template evaluation following project standards
	tplCtx := map[string]any{
		"workflow": map[string]any{
			"id":      workflowState.WorkflowID,
			"exec_id": workflowState.WorkflowExecID,
			"input":   dereferenceInput(workflowState.Input),
		},
		"tasks": workflowState.Tasks,
	}
	// Add merged input as "input" at top level for task context
	if mergedInput != nil {
		tplCtx["input"] = dereferenceInput(mergedInput)
	}
	return s.resolvePayloadRecursive(payload, tplCtx)
}

// resolvePayloadRecursive recursively resolves template placeholders
func (s *memoryOperationsService) resolvePayloadRecursive(payload any, context map[string]any) (any, error) {
	switch v := payload.(type) {
	case string:
		// Resolve string templates
		resolved, err := s.templateEngine.RenderString(v, context)
		if err != nil {
			return nil, err
		}
		return resolved, nil
	case map[string]any:
		// Recursively resolve map values
		resolved := make(map[string]any)
		for k, val := range v {
			resolvedVal, err := s.resolvePayloadRecursive(val, context)
			if err != nil {
				return nil, memcore.NewMemoryError(
					memcore.ErrCodeInvalidConfig,
					fmt.Sprintf("failed to resolve payload field '%s'", k),
					err,
				).WithContext("field", k)
			}
			resolved[k] = resolvedVal
		}
		return resolved, nil
	case []any:
		// Recursively resolve array elements
		resolved := make([]any, len(v))
		for i, item := range v {
			resolvedItem, err := s.resolvePayloadRecursive(item, context)
			if err != nil {
				return nil, memcore.NewMemoryError(
					memcore.ErrCodeInvalidConfig,
					fmt.Sprintf("failed to resolve payload[%d]", i),
					err,
				).WithContext("index", i)
			}
			resolved[i] = resolvedItem
		}
		return resolved, nil
	default:
		// Return other types as-is (numbers, booleans, etc)
		return v, nil
	}
}

// calculateTokensNonBlocking calculates total tokens without blocking the operation
func (s *memoryOperationsService) calculateTokensNonBlocking(ctx context.Context, messages []llm.Message) int {
	totalTokens := 0
	if s.tokenCounter == nil {
		return totalTokens
	}
	log := logger.FromContext(ctx)
	for _, msg := range messages {
		count, err := s.tokenCounter.CountTokens(ctx, msg.Content)
		if err != nil {
			// Log error but continue with 0 tokens for this message
			// Token counting should not block the write operation
			log.Warn("Failed to count tokens for message, using 0",
				"error", err,
				"content_length", len(msg.Content))
			count = 0
		}
		totalTokens += count
	}
	return totalTokens
}

// performAtomicWrite performs atomic write using AtomicOperations interface
func (s *memoryOperationsService) performAtomicWrite(
	ctx context.Context,
	instance memcore.AtomicOperations,
	key string,
	messages []llm.Message,
) (*WriteResponse, error) {
	// Calculate total tokens without blocking operation
	totalTokens := s.calculateTokensNonBlocking(ctx, messages)
	// Use atomic replace operation
	if err := instance.ReplaceMessagesWithMetadata(ctx, key, messages, totalTokens); err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to replace messages atomically",
			err,
		).WithContext("key", key).WithContext("message_count", len(messages)).WithContext("total_tokens", totalTokens)
	}
	return &WriteResponse{
		Success: true,
		Count:   len(messages),
		Key:     key,
	}, nil
}

// handleTransactionBeginError creates standardized error for transaction begin failures
func (s *memoryOperationsService) handleTransactionBeginError(err error, key string) error {
	return memcore.NewMemoryError(
		memcore.ErrCodeStoreOperation,
		"failed to begin transaction",
		err,
	).WithContext("key", key)
}

// handleTransactionClearError creates standardized error for transaction clear failures
func (s *memoryOperationsService) handleTransactionClearError(err error, key string) error {
	return memcore.NewMemoryError(
		memcore.ErrCodeMemoryClear,
		"failed to clear memory",
		err,
	).WithContext("key", key)
}

// handleTransactionApplyError handles apply failures with rollback attempt
func (s *memoryOperationsService) handleTransactionApplyError(
	ctx context.Context,
	tx *MemoryTransaction,
	err error,
	key string,
) error {
	if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
		return memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"write failed and rollback failed",
			err,
		).WithContext("rollback_error", rollbackErr.Error()).WithContext("key", key)
	}
	return memcore.NewMemoryError(
		memcore.ErrCodeStoreOperation,
		"write failed, memory restored",
		err,
	).WithContext("key", key)
}

// handleTransactionCommitError creates standardized error for transaction commit failures
func (s *memoryOperationsService) handleTransactionCommitError(err error, key string) error {
	return memcore.NewMemoryError(
		memcore.ErrCodeStoreOperation,
		"failed to commit transaction",
		err,
	).WithContext("key", key)
}

// performTransactionalWrite performs write using transaction pattern
func (s *memoryOperationsService) performTransactionalWrite(
	ctx context.Context,
	instance memcore.Memory,
	key string,
	messages []llm.Message,
) (*WriteResponse, error) {
	tx := NewMemoryTransaction(instance)
	if err := tx.Begin(ctx); err != nil {
		return nil, s.handleTransactionBeginError(err, key)
	}
	if err := tx.Clear(ctx); err != nil {
		return nil, s.handleTransactionClearError(err, key)
	}
	if err := tx.ApplyMessages(ctx, messages); err != nil {
		return nil, s.handleTransactionApplyError(ctx, tx, err, key)
	}
	if err := tx.Commit(); err != nil {
		return nil, s.handleTransactionCommitError(err, key)
	}
	return &WriteResponse{
		Success: true,
		Count:   len(messages),
		Key:     key,
	}, nil
}

// recordAppendMetrics records metrics and span attributes for append operation
func (s *memoryOperationsService) recordAppendMetrics(ctx context.Context, span trace.Span, req *AppendRequest) {
	span.SetAttributes(
		attribute.String("memory_ref", req.MemoryRef),
		attribute.String("key", req.Key),
	)
	if s.operationCount != nil {
		s.operationCount.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", "append"),
			attribute.String("memory_ref", req.MemoryRef),
		))
	}
}

// prepareAppendOperation prepares data for append operation
func (s *memoryOperationsService) prepareAppendOperation(
	ctx context.Context,
	req *AppendRequest,
) (memcore.Memory, []llm.Message, int, error) {
	// Validate payload type
	if err := ValidatePayloadType(req.Payload); err != nil {
		return nil, nil, 0, memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"invalid payload type",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "append")
	if err != nil {
		return nil, nil, 0, memcore.NewMemoryError(
			memcore.ErrCodeMemoryAppend,
			"failed to get memory instance",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	resolvedPayload, err := s.resolvePayload(req.Payload, req.MergedInput, req.WorkflowState)
	if err != nil {
		return nil, nil, 0, memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"failed to resolve payload",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	messages, err := PayloadToMessagesWithLimits(resolvedPayload, &s.config.ValidationLimits)
	if err != nil {
		return nil, nil, 0, memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"failed to convert payload to messages",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	beforeCount, err := instance.Len(ctx)
	if err != nil {
		return nil, nil, 0, memcore.NewMemoryError(
			memcore.ErrCodeMemoryAppend,
			"failed to get initial message count",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return instance, messages, beforeCount, nil
}

func (s *memoryOperationsService) appendNoopResponse(span trace.Span, total int, key string) *AppendResponse {
	span.SetAttributes(
		attribute.Int("appended_count", 0),
		attribute.Int("total_count", total),
	)
	return &AppendResponse{
		Success:    true,
		Appended:   0,
		TotalCount: total,
		Key:        key,
	}
}

func (s *memoryOperationsService) appendSuccessResponse(
	ctx context.Context,
	span trace.Span,
	instance memcore.Memory,
	beforeCount int,
	req *AppendRequest,
) (*AppendResponse, error) {
	totalCount, err := instance.Len(ctx)
	if err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeMemoryAppend,
			"failed to get message count",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	appended := totalCount - beforeCount
	span.SetAttributes(
		attribute.Int("appended_count", appended),
		attribute.Int("total_count", totalCount),
	)
	return &AppendResponse{
		Success:    true,
		Appended:   appended,
		TotalCount: totalCount,
		Key:        req.Key,
	}, nil
}

// executeAppendTransaction performs the append operation within a transaction
func (s *memoryOperationsService) executeAppendTransaction(
	ctx context.Context,
	instance memcore.Memory,
	messages []llm.Message,
	req *AppendRequest,
) (int, error) {
	tx := NewMemoryTransaction(instance)
	if err := tx.Begin(ctx); err != nil {
		return 0, memcore.NewMemoryError(
			memcore.ErrCodeMemoryAppend,
			"failed to begin transaction",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	if err := tx.ApplyMessages(ctx, messages); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return 0, memcore.NewMemoryError(
				memcore.ErrCodeMemoryAppend,
				"append failed and rollback failed",
				err,
			).WithContext("rollback_error", rollbackErr.Error()).WithContext(
				"memory_ref", req.MemoryRef,
			).WithContext("key", req.Key)
		}
		return 0, memcore.NewMemoryError(
			memcore.ErrCodeMemoryAppend,
			"append failed, memory restored",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	if err := tx.Commit(); err != nil {
		return 0, memcore.NewMemoryError(
			memcore.ErrCodeMemoryAppend,
			"failed to commit transaction",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	totalCount, err := instance.Len(ctx)
	if err != nil {
		return 0, memcore.NewMemoryError(
			memcore.ErrCodeMemoryAppend,
			"failed to get message count",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return totalCount, nil
}

// recordFlushMetrics records metrics and span attributes for flush operation
func (s *memoryOperationsService) recordFlushMetrics(ctx context.Context, span trace.Span, req *FlushRequest) {
	span.SetAttributes(
		attribute.String("memory_ref", req.MemoryRef),
		attribute.String("key", req.Key),
	)
	if s.operationCount != nil {
		s.operationCount.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", "flush"),
			attribute.String("memory_ref", req.MemoryRef),
		))
	}
}

// validateFlushRequest validates the flush request
func (s *memoryOperationsService) validateFlushRequest(req *FlushRequest) error {
	if err := ValidateBaseRequestWithLimits(&req.BaseRequest, &s.config.ValidationLimits); err != nil {
		return err
	}
	return ValidateFlushConfig(req.Config, s.strategyFactory)
}

// prepareFlushOperation prepares and validates the flush operation
func (s *memoryOperationsService) prepareFlushOperation(
	ctx context.Context,
	req *FlushRequest,
) (memcore.FlushableMemory, error) {
	instance, err := s.getMemoryInstance(ctx, req.MemoryRef, req.Key, "flush")
	if err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeStoreOperation,
			"failed to get memory instance",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	flushableMem, ok := instance.(memcore.FlushableMemory)
	if !ok {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeFlushFailed,
			"memory instance does not support flush operations",
			nil,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return flushableMem, nil
}

// performDryRunFlush performs a dry run flush operation
func (s *memoryOperationsService) performDryRunFlush(
	ctx context.Context,
	instance memcore.FlushableMemory,
	req *FlushRequest,
) (*FlushResponse, error) {
	health, err := instance.GetMemoryHealth(ctx)
	if err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeFlushFailed,
			"failed to get memory health for dry run",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	// Determine what strategy would be used
	actualStrategy := health.ActualStrategy
	if req.Config != nil && req.Config.Strategy != "" {
		// If user requested a specific strategy, that would be used
		actualStrategy = req.Config.Strategy
	}
	return &FlushResponse{
		Success:        true,
		DryRun:         true,
		Key:            req.Key,
		WouldFlush:     health.TokenCount > 0,
		TokenCount:     health.TokenCount,
		MessageCount:   health.MessageCount,
		ActualStrategy: actualStrategy,
	}, nil
}

// performActualFlush performs the actual flush operation
func (s *memoryOperationsService) performActualFlush(
	ctx context.Context,
	flushableMem memcore.FlushableMemory,
	req *FlushRequest,
) (*FlushResponse, error) {
	requestedStrategy := requestedFlushStrategy(req)
	result, actualStrategy, err := s.executeFlush(ctx, flushableMem, requestedStrategy, req)
	if err != nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeFlushFailed,
			"flush operation failed",
			err,
		).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	response := buildFlushResponse(result, req.Key, actualStrategy)
	if result.Error != "" {
		response.Error = result.Error
		return response, memcore.NewMemoryError(
			memcore.ErrCodeFlushFailed,
			"flush completed with error",
			nil,
		).WithContext("flush_error", result.Error).WithContext("memory_ref", req.MemoryRef).WithContext("key", req.Key)
	}
	return response, nil
}
