package task

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCELEvaluator_ConcurrentAccess(t *testing.T) {
	evaluator, err := NewCELEvaluator(WithCacheSize(100))
	require.NoError(t, err)

	ctx := context.Background()
	numGoroutines := 50
	evaluationsPerGoroutine := 100

	expressions := []string{
		`signal.payload.status == "approved"`,
		`signal.payload.value > 100`,
		`signal.payload.ready == true`,
		`processor.output.valid == true`,
		`signal.payload.count >= 5`,
		`has(signal.payload.data) && size(signal.payload.data) > 0`,
		`signal.payload.priority >= 3 && signal.payload.category == "urgent"`,
		`signal.payload.timestamp > 1640995200`,
	}

	data := map[string]any{
		"signal": map[string]any{
			"payload": map[string]any{
				"status":    "approved",
				"value":     150,
				"ready":     true,
				"count":     10,
				"data":      []string{"item1", "item2"},
				"priority":  5,
				"category":  "urgent",
				"timestamp": 1640995300,
			},
		},
		"processor": map[string]any{
			"output": map[string]any{
				"valid": true,
			},
		},
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines*evaluationsPerGoroutine)

	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < evaluationsPerGoroutine; j++ {
				expr := expressions[(goroutineID*evaluationsPerGoroutine+j)%len(expressions)]

				result, err := evaluator.Evaluate(ctx, expr, data)
				if err != nil {
					errorChan <- fmt.Errorf("goroutine %d, iteration %d: %w", goroutineID, j, err)
					return
				}

				if !result {
					errorChan <- fmt.Errorf("goroutine %d, iteration %d: expected true result for %s", goroutineID, j, expr)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for any errors
	for err := range errorChan {
		t.Error(err)
	}
}

func TestCELEvaluator_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	evaluator, err := NewCELEvaluator(WithCacheSize(1000), WithCostLimit(10000))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Complex expressions to stress test the evaluator
	complexExpressions := []string{
		`signal.payload.status == "approved" && signal.payload.priority > 5 && has(signal.payload.metadata)`,
		`size(signal.payload.items) > 0 && signal.payload.items.all(item, item.valid == true)`,
		`signal.payload.timestamp > 1640995200 && signal.payload.timestamp < 1672531200`,
		`processor.output.score > 0.8 && processor.output.confidence >= 0.9`,
		`has(signal.payload.user) && signal.payload.user.role in ["admin", "moderator"]`,
		`signal.payload.data.type == "important" && size(signal.payload.data.content) > 100`,
	}

	data := map[string]any{
		"signal": map[string]any{
			"payload": map[string]any{
				"status":   "approved",
				"priority": 10,
				"metadata": map[string]any{"source": "test"},
				"items": []map[string]any{
					{"valid": true, "id": 1},
					{"valid": true, "id": 2},
				},
				"timestamp": 1641000000,
				"user": map[string]any{
					"role": "admin",
					"id":   "user123",
				},
				"data": map[string]any{
					"type":    "important",
					"content": strings.Repeat("test content ", 17), // ~200 character string
				},
			},
		},
		"processor": map[string]any{
			"output": map[string]any{
				"score":      0.95,
				"confidence": 0.98,
			},
		},
	}

	evaluationCount := 0
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			t.Logf("Stress test completed: %d evaluations in %v", evaluationCount, time.Since(startTime))
			return
		default:
			for _, expr := range complexExpressions {
				result, err := evaluator.Evaluate(ctx, expr, data)
				if err != nil {
					// If context is canceled, stop gracefully
					if ctx.Err() != nil {
						t.Logf("Stress test completed: %d evaluations in %v", evaluationCount, time.Since(startTime))
						return
					}
					t.Fatalf("Evaluation failed: %v", err)
				}
				assert.True(t, result)
				evaluationCount++

				if evaluationCount%10000 == 0 {
					t.Logf("Completed %d evaluations", evaluationCount)
				}
			}
		}
	}
}

func TestWaitTask_ExpressionComplexity(t *testing.T) {
	evaluator, err := NewCELEvaluator(WithCostLimit(50000)) // Higher limit for complex expressions
	require.NoError(t, err)

	ctx := context.Background()

	// Test increasingly complex expressions
	complexityTests := []struct {
		name       string
		expression string
		data       map[string]any
		expected   bool
	}{
		{
			name:       "simple",
			expression: `signal.payload.status == "approved"`,
			data: map[string]any{
				"signal": map[string]any{
					"payload": map[string]any{"status": "approved"},
				},
			},
			expected: true,
		},
		{
			name:       "medium complexity",
			expression: `signal.payload.status == "approved" && signal.payload.priority > 5`,
			data: map[string]any{
				"signal": map[string]any{
					"payload": map[string]any{
						"status":   "approved",
						"priority": 10,
					},
				},
			},
			expected: true,
		},
		{
			name: "high complexity",
			expression: `signal.payload.status == "approved" &&
						 signal.payload.priority > 5 &&
						 has(signal.payload.metadata) &&
						 signal.payload.metadata.source in ["api", "ui"] &&
						 size(signal.payload.items) > 0 &&
						 signal.payload.items.all(item, item.valid == true && item.score > 0.5)`,
			data: map[string]any{
				"signal": map[string]any{
					"payload": map[string]any{
						"status":   "approved",
						"priority": 10,
						"metadata": map[string]any{"source": "api"},
						"items": []map[string]any{
							{"valid": true, "score": 0.8},
							{"valid": true, "score": 0.9},
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range complexityTests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			result, err := evaluator.Evaluate(ctx, tc.expression, tc.data)
			duration := time.Since(start)

			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)

			t.Logf("%s evaluation took %v", tc.name, duration)

			// Even complex expressions should evaluate quickly
			assert.Less(t, duration, 10*time.Millisecond, "Expression evaluation should be fast")
		})
	}
}
