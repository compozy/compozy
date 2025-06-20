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

	"github.com/compozy/compozy/engine/core"
)

func BenchmarkCELEvaluator_Evaluate(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	require.NoError(b, err)

	ctx := context.Background()
	expression := `signal.payload.status == "approved" && signal.payload.priority > 5`
	data := map[string]any{
		"signal": map[string]any{
			"payload": map[string]any{
				"status":   "approved",
				"priority": 10,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := evaluator.Evaluate(ctx, expression, data)
		if err != nil {
			b.Fatal(err)
		}
		if !result {
			b.Fatal("Expected true result")
		}
	}
}

func BenchmarkCELEvaluator_ValidateExpression(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	require.NoError(b, err)

	expressions := []string{
		`signal.payload.status == "approved"`,
		`signal.payload.value > 100`,
		`has(signal.payload.data) && size(signal.payload.data) > 0`,
		`processor.output.valid == true`,
		`signal.payload.priority >= 5 && signal.payload.category == "urgent"`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expr := expressions[i%len(expressions)]
		err := evaluator.ValidateExpression(expr)
		if err != nil {
			b.Fatalf("Expression validation failed for %s: %v", expr, err)
		}
	}
}

func BenchmarkCELEvaluator_CachePerformance(b *testing.B) {
	evaluator, err := NewCELEvaluator(WithCacheSize(1000))
	require.NoError(b, err)

	ctx := context.Background()

	// Test with repeated expressions to measure cache effectiveness
	expressions := []string{
		`signal.payload.status == "approved"`,
		`signal.payload.value > 100`,
		`signal.payload.ready == true`,
		`processor.output.valid == true`,
		`signal.payload.count >= 5`,
	}

	data := map[string]any{
		"signal": map[string]any{
			"payload": map[string]any{
				"status": "approved",
				"value":  150,
				"ready":  true,
				"count":  10,
			},
		},
		"processor": map[string]any{
			"output": map[string]any{
				"valid": true,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expr := expressions[i%len(expressions)]
		result, err := evaluator.Evaluate(ctx, expr, data)
		if err != nil {
			b.Fatalf("Evaluation failed for %s: %v", expr, err)
		}
		if !result {
			b.Fatalf("Expected true result for %s", expr)
		}
	}
}

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

	for i := 0; i < numGoroutines; i++ {
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

func BenchmarkCELEvaluator_MemoryAndCacheEviction(b *testing.B) {
	evaluator, err := NewCELEvaluator(WithCacheSize(50))
	require.NoError(b, err)
	ctx := context.Background()

	// Create many unique expressions to test cache eviction
	expressions := make([]string, 200)
	for i := 0; i < 200; i++ {
		expressions[i] = fmt.Sprintf(`signal.payload.value_%d > %d`, i, i*10)
	}

	data := map[string]any{
		"signal": map[string]any{
			"payload": map[string]any{},
		},
	}

	// Add all values to data
	for i := 0; i < 200; i++ {
		data["signal"].(map[string]any)["payload"].(map[string]any)[fmt.Sprintf("value_%d", i)] = (i + 1) * 10
	}

	b.ResetTimer()
	b.ReportAllocs() // This is the key addition

	for i := 0; i < b.N; i++ {
		// Evaluate all expressions to force cache interaction
		for _, expr := range expressions {
			result, err := evaluator.Evaluate(ctx, expr, data)
			if err != nil {
				b.Fatal(err)
			}
			if !result {
				b.Fatalf("Expected true result for %s", expr)
			}
		}
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

func BenchmarkWaitTask_ConfigValidation(b *testing.B) {
	CWD, _ := core.CWDFromPath("/tmp")

	// Create a complex wait task configuration
	config := &Config{
		BaseConfig: BaseConfig{
			ID:        "performance-test-wait",
			Type:      TaskTypeWait,
			CWD:       CWD,
			Condition: `signal.payload.status == "approved" && signal.payload.priority > 5 && has(signal.payload.metadata)`,
			Timeout:   "30m",
		},
		WaitTask: WaitTask{
			WaitFor: "complex_approval_signal",
			Processor: &Config{
				BaseConfig: BaseConfig{
					ID:   "signal-processor",
					Type: TaskTypeBasic,
					CWD:  CWD,
				},
				BasicTask: BasicTask{
					Action: "process_signal",
				},
			},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := config.Validate()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWaitTask_LargeSignalPayload(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	require.NoError(b, err)

	ctx := context.Background()
	expression := `has(signal.payload.data) && size(signal.payload.data) > 1000`

	// Create a large payload
	largeData := make(map[string]any)
	for i := 0; i < 2000; i++ {
		largeData[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	data := map[string]any{
		"signal": map[string]any{
			"payload": map[string]any{
				"data": largeData,
			},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := evaluator.Evaluate(ctx, expression, data)
		if err != nil {
			b.Fatal(err)
		}
		if !result {
			b.Fatal("Expected true result")
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
