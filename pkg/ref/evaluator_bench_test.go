package ref

import (
	"fmt"
	"testing"
)

// -----------------------------------------------------------------------------
// Benchmark Test Data
// -----------------------------------------------------------------------------

func createLargeScope(depth, breadth int) map[string]any {
	scope := make(map[string]any)
	for i := range breadth {
		key := fmt.Sprintf("item_%d", i)
		scope[key] = createNestedObject(depth)
	}
	return scope
}

func createNestedObject(depth int) map[string]any {
	if depth <= 0 {
		return map[string]any{
			"value": "leaf_node",
			"count": 42,
		}
	}
	return map[string]any{
		"nested": createNestedObject(depth - 1),
		"meta": map[string]any{
			"level": depth,
			"data":  "some_data",
		},
	}
}

// -----------------------------------------------------------------------------
// Path Resolution Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkResolvePath_NoCache(b *testing.B) {
	scope := createLargeScope(5, 100)
	ev := NewEvaluator(WithLocalScope(scope))

	for b.Loop() {
		_, err := ev.ResolvePath("local", "item_50.nested.nested.nested.nested.nested.value")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolvePath_WithCache(b *testing.B) {
	scope := createLargeScope(5, 100)
	ev := NewEvaluator(
		WithLocalScope(scope),
		WithCacheEnabled(),
	)
	for b.Loop() {
		_, err := ev.ResolvePath("local", "item_50.nested.nested.nested.nested.nested.value")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolvePath_MixedPaths_NoCache(b *testing.B) {
	scope := createLargeScope(3, 50)
	ev := NewEvaluator(WithLocalScope(scope))
	paths := []string{
		"item_10.nested.nested.nested.value",
		"item_20.nested.nested.meta.data",
		"item_30.meta.level",
		"item_40.nested.meta.level",
	}
	for i := 0; b.Loop(); i++ {
		path := paths[i%len(paths)]
		_, err := ev.ResolvePath("local", path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolvePath_MixedPaths_WithCache(b *testing.B) {
	scope := createLargeScope(3, 50)
	ev := NewEvaluator(
		WithLocalScope(scope),
		WithCacheEnabled(),
	)
	paths := []string{
		"item_10.nested.nested.nested.value",
		"item_20.nested.nested.meta.data",
		"item_30.meta.level",
		"item_40.nested.meta.level",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := paths[i%len(paths)]
		_, err := ev.ResolvePath("local", path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// -----------------------------------------------------------------------------
// Directive Evaluation Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkEval_SimpleRef(b *testing.B) {
	scope := map[string]any{
		"config": map[string]any{
			"host": "localhost",
			"port": 8080,
		},
	}
	ev := NewEvaluator(WithLocalScope(scope))
	node := map[string]any{
		"server": map[string]any{
			"$ref": "local::config",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ev.Eval(node)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEval_NestedRefs(b *testing.B) {
	scope := map[string]any{
		"base": map[string]any{
			"host": "localhost",
			"port": 8080,
		},
		"extended": map[string]any{
			"$ref": "local::base",
		},
		"final": map[string]any{
			"$ref": "local::extended",
		},
	}
	ev := NewEvaluator(WithLocalScope(scope))
	node := map[string]any{
		"config": map[string]any{
			"$ref": "local::final",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ev.Eval(node)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEval_ComplexDocument(b *testing.B) {
	scope := map[string]any{
		"defaults": map[string]any{
			"server": map[string]any{
				"host": "0.0.0.0",
				"port": 8080,
			},
			"database": map[string]any{
				"host": "localhost",
				"port": 5432,
			},
		},
		"agents": map[string]any{
			"worker": map[string]any{
				"name": "worker",
				"type": "background",
			},
		},
	}

	node := map[string]any{
		"services": []any{
			map[string]any{
				"name": "api",
				"config": map[string]any{
					"$ref": "local::defaults.server",
				},
			},
			map[string]any{
				"name": "worker",
				"agent": map[string]any{
					"$use": "agent(local::agents.worker)",
				},
			},
		},
		"database": map[string]any{
			"$merge": []any{
				map[string]any{"$ref": "local::defaults.database"},
				map[string]any{"maxConnections": 100},
			},
		},
	}

	b.Run("NoCache", func(b *testing.B) {
		ev := NewEvaluator(WithLocalScope(scope))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ev.Eval(node)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WithCache", func(b *testing.B) {
		ev := NewEvaluator(
			WithLocalScope(scope),
			WithCacheEnabled(),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ev.Eval(node)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// -----------------------------------------------------------------------------
// Merge Directive Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkMerge_DeepObjects(b *testing.B) {
	sources := []any{
		createNestedObject(3),
		createNestedObject(3),
		createNestedObject(3),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mergeObjects(sources, StrategyDeep, KeyConflictReplace)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMerge_ShallowObjects(b *testing.B) {
	sources := []any{
		createNestedObject(3),
		createNestedObject(3),
		createNestedObject(3),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mergeObjects(sources, StrategyShallow, KeyConflictReplace)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMerge_LargeArrays(b *testing.B) {
	createArray := func(size int) []any {
		arr := make([]any, size)
		for i := 0; i < size; i++ {
			arr[i] = fmt.Sprintf("item_%d", i)
		}
		return arr
	}

	sources := []any{
		createArray(100),
		createArray(100),
		createArray(100),
	}

	b.Run("Concat", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := mergeArrays(sources, StrategyConcat)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Unique", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := mergeArrays(sources, StrategyUnique)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// -----------------------------------------------------------------------------
// ProcessBytes Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkProcessBytes_SmallDocument(b *testing.B) {
	yamlDoc := []byte(`
name: test-app
server:
  $ref: "local::defaults.server"
tags:
  - production
  - api
`)
	scope := map[string]any{
		"defaults": map[string]any{
			"server": map[string]any{
				"host": "localhost",
				"port": 8080,
			},
		},
	}

	b.Run("NoCache", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ProcessBytes(yamlDoc, WithLocalScope(scope))
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WithCache", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ProcessBytes(yamlDoc, WithLocalScope(scope), WithCacheEnabled())
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("ReusedEvaluator", func(b *testing.B) {
		// Create evaluator once with cache
		ev := NewEvaluator(WithLocalScope(scope), WithCacheEnabled())
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ProcessBytesWithEvaluator(yamlDoc, ev)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkProcessBytes_LargeDocument(b *testing.B) {
	// Create a large YAML document with many references
	yamlDoc := []byte(`
services:
  - name: api-1
    config:
      $merge:
        - $ref: "local::defaults.server"
        - port: 8001
  - name: api-2
    config:
      $merge:
        - $ref: "local::defaults.server"
        - port: 8002
  - name: api-3
    config:
      $merge:
        - $ref: "local::defaults.server"
        - port: 8003
  - name: worker-1
    agent:
      $use: agent(local::agents.worker)
  - name: worker-2
    agent:
      $use: agent(local::agents.worker)

database:
  primary:
    $merge:
      - $ref: "local::defaults.database"
      - role: primary
  replica:
    $merge:
      - $ref: "local::defaults.database"
      - role: replica
      - readonly: true

features:
  $merge:
    - $ref: "local::defaults.features"
    - $ref: "local::overrides.features"
    - custom: true
`)

	scope := map[string]any{
		"defaults": map[string]any{
			"server": map[string]any{
				"host":      "0.0.0.0",
				"port":      8080,
				"timeout":   30,
				"keepalive": true,
			},
			"database": map[string]any{
				"host": "localhost",
				"port": 5432,
				"pool": map[string]any{
					"min": 5,
					"max": 20,
				},
			},
			"features": map[string]any{
				"auth":    true,
				"logging": true,
				"metrics": false,
			},
		},
		"agents": map[string]any{
			"worker": map[string]any{
				"type":     "background",
				"replicas": 3,
				"resources": map[string]any{
					"cpu":    "100m",
					"memory": "256Mi",
				},
			},
		},
		"overrides": map[string]any{
			"features": map[string]any{
				"metrics": true,
				"tracing": true,
			},
		},
	}

	b.Run("NoCache", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ProcessBytes(yamlDoc, WithLocalScope(scope))
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WithCache", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ProcessBytes(yamlDoc, WithLocalScope(scope), WithCacheEnabled())
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("ReusedEvaluator", func(b *testing.B) {
		// Create evaluator once with cache
		ev := NewEvaluator(WithLocalScope(scope), WithCacheEnabled())
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ProcessBytesWithEvaluator(yamlDoc, ev)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// -----------------------------------------------------------------------------
// Concurrent Access Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkConcurrentEval(b *testing.B) {
	scope := createLargeScope(3, 50)
	node := map[string]any{
		"config": map[string]any{
			"$ref": "local::item_25.nested.meta",
		},
	}

	b.Run("NoCache", func(b *testing.B) {
		ev := NewEvaluator(WithLocalScope(scope))
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := ev.Eval(node)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("WithCache", func(b *testing.B) {
		ev := NewEvaluator(
			WithLocalScope(scope),
			WithCacheEnabled(),
		)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := ev.Eval(node)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}
