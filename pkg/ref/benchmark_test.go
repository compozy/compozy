package ref

import (
	"context"
	"fmt"
	"testing"
)

// -----------------------------------------------------------------------------
// Reference Resolution Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkResolve_LargeMap(b *testing.B) {
	// Create a large map to trigger parallel processing
	data := make(map[string]any)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("item%d", i)
		data[key] = map[string]any{
			"id":    i,
			"value": fmt.Sprintf("item_%d_value", i),
			"nested": map[string]any{
				"deep": fmt.Sprintf("nested_value_%d", i),
			},
		}
	}

	ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolve_LargeArray(b *testing.B) {
	// Create a large array to trigger parallel processing
	data := make([]any, 100)
	for i := 0; i < 100; i++ {
		data[i] = map[string]any{
			"index": i,
			"data":  fmt.Sprintf("array_item_%d", i),
			"nested": map[string]any{
				"value": fmt.Sprintf("nested_%d", i),
			},
		}
	}

	ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolve_NestedReferences(b *testing.B) {
	// Create data with nested references
	data := map[string]any{
		"target": map[string]any{
			"value": "resolved_value",
		},
		"refs": map[string]any{
			"ref1": map[string]any{"$ref": "target.value"},
			"ref2": map[string]any{"$ref": "target.value"},
			"ref3": map[string]any{"$ref": "target.value"},
			"ref4": map[string]any{"$ref": "target.value"},
			"ref5": map[string]any{"$ref": "target.value"},
		},
	}

	ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolve_PathLookup(b *testing.B) {
	// Create a document for path lookups
	data := map[string]any{
		"schemas": []any{
			map[string]any{
				"id":   "schema1",
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
			map[string]any{
				"id":   "schema2",
				"type": "object",
				"properties": map[string]any{
					"email": map[string]any{"type": "string"},
				},
			},
		},
	}

	ref := &Ref{
		Type: TypeProperty,
		Path: "schemas.#(id==\"schema1\").properties.name",
		Mode: ModeMerge,
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// -----------------------------------------------------------------------------
// Cache Performance Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkCache_DocumentLoad(b *testing.B) {
	// Benchmark cache performance for document loading
	testData := map[string]any{
		"config": map[string]any{
			"version": "1.0",
			"name":    "test_config",
		},
	}

	cache := getResolvedDocsCache()
	testKey := "/benchmark/test.yaml"

	b.Run("CacheWrite", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("%s_%d", testKey, i)
			cache.Add(key, testData)
		}
	})

	b.Run("CacheRead", func(b *testing.B) {
		// Pre-populate cache
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("%s_%d", testKey, i)
			cache.Add(key, testData)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("%s_%d", testKey, i%1000)
			_, _ = cache.Get(key)
		}
	})
}

func BenchmarkPathCache_Performance(b *testing.B) {
	// Benchmark path cache performance
	if pathCache := getPathCache(); pathCache != nil {
		testPaths := []string{
			"schemas.0.id",
			"schemas.#(id==\"test\").properties",
			"config.database.host",
			"nested.level1.level2.value",
			"array.#(name==\"item\").data",
		}

		b.Run("PathCacheWrite", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				path := testPaths[i%len(testPaths)]
				pathCache.Add(path, path)
			}
		})

		b.Run("PathCacheRead", func(b *testing.B) {
			// Pre-populate cache
			for _, path := range testPaths {
				pathCache.Add(path, path)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				path := testPaths[i%len(testPaths)]
				_, _ = pathCache.Get(path)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// WithRef Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkWithRef_MapResolution(b *testing.B) {
	// Setup test data with nested references
	data := map[string]any{
		"config": map[string]any{
			"$ref":  "target.nested",
			"extra": "value",
		},
		"target": map[string]any{
			"nested": map[string]any{
				"key": "resolved_value",
			},
		},
		"array": []any{
			map[string]any{
				"$ref": "target.nested",
				"id":   "item1",
			},
			map[string]any{
				"$ref": "target.nested",
				"id":   "item2",
			},
		},
	}

	withRef := &WithRef{}
	withRef.SetRefMetadata("/test/file.yaml", "/test")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := withRef.ResolveMapReference(ctx, data, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// -----------------------------------------------------------------------------
// Memory Allocation Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkResolve_Allocations(b *testing.B) {
	// Test memory allocations for typical use cases
	data := map[string]any{
		"target": "resolved_value",
		"ref1":   map[string]any{"$ref": "target"},
		"ref2":   map[string]any{"$ref": "target"},
		"ref3":   map[string]any{"$ref": "target"},
	}

	ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// -----------------------------------------------------------------------------
// Scaling Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkResolve_Scaling(b *testing.B) {
	sizes := []int{10, 50, 100, 500}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			data := make(map[string]any)
			data["target"] = "resolved_value"

			for i := 0; i < size; i++ {
				key := fmt.Sprintf("ref%d", i)
				data[key] = map[string]any{"$ref": "target"}
			}

			ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
