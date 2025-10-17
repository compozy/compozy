---
title: "Missing Usage Metadata Persistence"
group: "ENGINE_GROUP_2_LLM_TOOLS"
category: "performance"
priority: "🔴 HIGH"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_2_LLM_TOOLS_PERFORMANCE.md"
issue_index: "2"
sequence: "8"
---

## Missing Usage Metadata Persistence

**Location:** `engine/llm/provider/openai.go:234–289`, `engine/llm/provider/anthropic.go:198–245`

**Severity:** 🔴 HIGH

**Issue:**

```go
// Usage data is calculated but never persisted
func (p *Provider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    req.Model,
        Messages: convertMessages(req.Messages),
        // ...
    })

    if err != nil {
        return nil, err
    }

    // Usage calculated here
    usage := &UsageMetadata{
        PromptTokens:     resp.Usage.PromptTokens,
        CompletionTokens: resp.Usage.CompletionTokens,
        TotalTokens:      resp.Usage.TotalTokens,
    }

    // But usage is NOT saved to database
    // Lost when execution completes
    return &CompletionResponse{
        Content: resp.Choices[0].Message.Content,
        Usage:   usage, // Only returned, not persisted
    }, nil
}
```

**Problems:**

1. No historical usage tracking for cost analysis
2. Cannot audit token consumption per agent/task/workflow
3. Cannot detect usage anomalies or spikes
4. Billing reconciliation impossible

**Fix - Add Usage Repository:**

```go
// engine/llm/usage/repository.go (NEW)
package usage

import (
    "context"
    "time"
    "github.com/compozy/compozy/pkg/core"
)

type Record struct {
    ID               core.ID
    ExecutionID      core.ID
    Provider         string
    Model            string
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    CostUSD          float64
    Timestamp        time.Time
    Metadata         map[string]any
}

type Repository interface {
    Save(ctx context.Context, record *Record) error
    GetByExecution(ctx context.Context, execID core.ID) ([]*Record, error)
    GetTotalUsage(ctx context.Context, start, end time.Time) (*AggregateSummary, error)
}

// engine/llm/provider/openai.go
func (p *Provider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    resp, err := p.client.CreateChatCompletion(ctx, req)
    if err != nil {
        return nil, err
    }

    usage := &UsageMetadata{
        PromptTokens:     resp.Usage.PromptTokens,
        CompletionTokens: resp.Usage.CompletionTokens,
        TotalTokens:      resp.Usage.TotalTokens,
    }

    // Persist usage asynchronously to avoid blocking
    go func() {
        saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        record := &usage.Record{
            ID:               core.NewID(),
            ExecutionID:      req.ExecutionID,
            Provider:         "openai",
            Model:            req.Model,
            PromptTokens:     usage.PromptTokens,
            CompletionTokens: usage.CompletionTokens,
            TotalTokens:      usage.TotalTokens,
            CostUSD:          calculateCost(req.Model, usage),
            Timestamp:        time.Now(),
        }

        if err := p.usageRepo.Save(saveCtx, record); err != nil {
            logger.FromContext(ctx).Error("failed to save usage", "error", err)
        }
    }()

    return &CompletionResponse{
        Content: resp.Choices[0].Message.Content,
        Usage:   usage,
    }, nil
}

// Cost calculation helper
func calculateCost(model string, usage *UsageMetadata) float64 {
    prices := map[string]struct{ input, output float64 }{
        "gpt-4-turbo":  {0.01 / 1000, 0.03 / 1000},
        "gpt-4":        {0.03 / 1000, 0.06 / 1000},
        "gpt-3.5-turbo": {0.0015 / 1000, 0.002 / 1000},
    }

    price, ok := prices[model]
    if !ok {
        return 0 // Unknown model
    }

    return float64(usage.PromptTokens)*price.input +
           float64(usage.CompletionTokens)*price.output
}
```

**Database Schema:**

```sql
CREATE TABLE llm_usage (
    id UUID PRIMARY KEY,
    execution_id UUID NOT NULL REFERENCES executions(id),
    provider VARCHAR(50) NOT NULL,
    model VARCHAR(100) NOT NULL,
    prompt_tokens INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    total_tokens INTEGER NOT NULL,
    cost_usd DECIMAL(10, 6),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB,

    INDEX idx_usage_execution (execution_id),
    INDEX idx_usage_timestamp (timestamp DESC),
    INDEX idx_usage_provider_model (provider, model)
);
```

**Impact:**

- Enables cost tracking and billing
- Provides audit trail for compliance
- Allows usage analysis and optimization

**Effort:** M (4-5h including schema migration)  
**Risk:** Low - asynchronous write doesn't block critical path
