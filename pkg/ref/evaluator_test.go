package ref

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// Use Directive Tests
// -----------------------------------------------------------------------------

func TestUseDirective(t *testing.T) {
	localScope := map[string]any{
		"agents": map[string]any{
			"foo": map[string]any{
				"name": "Foo Agent",
				"type": "assistant",
			},
		},
		"env": map[string]any{
			"db": map[string]any{
				"host": "localhost",
				"port": 5432,
			},
		},
	}
	globalScope := map[string]any{
		"tasks": map[string]any{
			"build": map[string]any{
				"name": "Build Task",
				"cmd":  "make build",
			},
		},
	}

	t.Run("Should resolve local agent", func(t *testing.T) {
		input := `{"$use":"agent(local::agents.foo)"}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(localScope))
		want := map[string]any{
			"agent": map[string]any{
				"name": "Foo Agent",
				"type": "assistant",
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should resolve global task", func(t *testing.T) {
		input := `{"$use":"task(global::tasks.build)"}`
		got := MustEvalBytes(t, []byte(input), WithGlobalScope(globalScope))
		want := map[string]any{
			"task": map[string]any{
				"name": "Build Task",
				"cmd":  "make build",
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should resolve nested object", func(t *testing.T) {
		input := `{"env":{"$use":"agent(local::env.db)"}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(localScope))
		want := map[string]any{
			"env": map[string]any{
				"agent": map[string]any{
					"host": "localhost",
					"port": float64(5432),
				},
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should apply TransformUse function", func(t *testing.T) {
		input := `{"$use":"tool(local::agents.foo)"}`
		transform := func(component string, cfg Node) (string, Node, error) {
			return "wrapped_" + component, map[string]any{"val": cfg}, nil
		}
		got := MustEvalBytes(t, []byte(input), WithLocalScope(localScope), WithTransformUse(transform))
		want := map[string]any{
			"wrapped_tool": map[string]any{
				"val": map[string]any{
					"name": "Foo Agent",
					"type": "assistant",
				},
			},
		}
		assert.Equal(t, want, got)
	})
}

// -----------------------------------------------------------------------------
// Ref Directive Tests
// -----------------------------------------------------------------------------

func TestRefDirective(t *testing.T) {
	local := map[string]any{
		"config": map[string]any{
			"postgres": map[string]any{"port": 5432},
		},
		"services": []any{
			map[string]any{"name": "api", "port": 8080},
			map[string]any{"name": "worker", "metadata": map[string]any{"url": "http://worker:9090"}},
		},
	}
	global := map[string]any{
		"defaults": map[string]any{"db": map[string]any{"host": "localhost", "port": 5432, "user": "postgres"}},
	}

	t.Run("Should resolve scalar value", func(t *testing.T) {
		input := `{"port":{"$ref":"local::config.postgres.port"}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(local))
		want := map[string]any{"port": float64(5432)}
		assert.Equal(t, want, got)
	})

	t.Run("Should resolve object value", func(t *testing.T) {
		input := `{"db":{"$ref":"global::defaults.db"}}`
		got := MustEvalBytes(t, []byte(input), WithGlobalScope(global))
		want := map[string]any{"db": map[string]any{"host": "localhost", "port": float64(5432), "user": "postgres"}}
		assert.Equal(t, want, got)
	})

	t.Run("Should resolve deep path with array index", func(t *testing.T) {
		input := `{"$ref":"local::services.1.metadata.url"}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(local))
		want := "http://worker:9090"
		assert.Equal(t, want, got)
	})

	t.Run("Should resolve array value", func(t *testing.T) {
		input := `{"services":{"$ref":"local::services"}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(local))
		want := map[string]any{"services": []any{
			map[string]any{"name": "api", "port": float64(8080)},
			map[string]any{"name": "worker", "metadata": map[string]any{"url": "http://worker:9090"}},
		}}
		assert.Equal(t, want, got)
	})
}

// -----------------------------------------------------------------------------
// Recursive Evaluation Tests
// -----------------------------------------------------------------------------

func TestRecursiveEvaluation(t *testing.T) {
	scope := map[string]any{
		"base": map[string]any{
			"config": map[string]any{
				"host": "localhost",
				"port": 8080,
			},
		},
		"server": map[string]any{
			"$ref": "local::base.config",
		},
		"nested": map[string]any{
			"deep": map[string]any{
				"$ref": "local::server",
			},
		},
		"agents": map[string]any{
			"api": map[string]any{
				"name": "API Agent",
				"config": map[string]any{
					"$ref": "local::base.config",
				},
			},
		},
	}

	t.Run("Should resolve nested $ref directives", func(t *testing.T) {
		input := `{"result":{"$ref":"local::nested.deep"}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(scope))
		want := map[string]any{
			"result": map[string]any{
				"host": "localhost",
				"port": float64(8080),
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should resolve mixed directives recursively", func(t *testing.T) {
		input := `{"service":{"$use":"agent(local::agents.api)"}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(scope))
		want := map[string]any{
			"service": map[string]any{
				"agent": map[string]any{
					"name": "API Agent",
					"config": map[string]any{
						"host": "localhost",
						"port": float64(8080),
					},
				},
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should handle multiple levels of recursion", func(t *testing.T) {
		multiLevel := map[string]any{
			"level1": map[string]any{"$ref": "local::level2"},
			"level2": map[string]any{"$ref": "local::level3"},
			"level3": map[string]any{"$ref": "local::final"},
			"final":  map[string]any{"value": "success"},
		}
		input := `{"$ref":"local::level1"}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(multiLevel))
		want := map[string]any{"value": "success"}
		assert.Equal(t, want, got)
	})

	t.Run("Should handle recursive evaluation in arrays", func(t *testing.T) {
		arrayScope := map[string]any{
			"configs": []any{
				map[string]any{"$ref": "local::base.config"},
				map[string]any{"name": "static"},
			},
			"base": map[string]any{
				"config": map[string]any{"host": "localhost", "port": 3000},
			},
		}
		input := `{"services":{"$ref":"local::configs"}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(arrayScope))
		want := map[string]any{
			"services": []any{
				map[string]any{"host": "localhost", "port": float64(3000)},
				map[string]any{"name": "static"},
			},
		}
		assert.Equal(t, want, got)
	})
}

// -----------------------------------------------------------------------------
// Error Condition Tests
// -----------------------------------------------------------------------------

func TestDirectiveErrors(t *testing.T) {
	scope := map[string]any{"foo": "bar"}
	cases := []testCase{
		{name: "Should fail on unknown scope", input: `{"$ref":"planet::foo"}`, wantErr: true, errContains: "invalid $ref syntax"},
		{name: "Should fail on missing path", input: `{"$ref":"local::does.not.exist"}`, options: []EvalConfigOption{WithLocalScope(scope)}, wantErr: true, errContains: "not found"},
		{name: "Should fail on bad $use syntax", input: `{"$use":"agent(bad)"}`, wantErr: true, errContains: "invalid $use syntax"},
		{name: "Should fail on sibling keys", input: `{"$ref":"local::foo","oops":1}`, wantErr: true, errContains: "sibling keys"},
		{name: "Should fail when $ref not string", input: `{"$ref":123}`, wantErr: true, errContains: "$ref must be a string"},
		{name: "Should fail on empty $use", input: `{"$use":""}`, wantErr: true, errContains: "invalid $use syntax"},
		{name: "Should fail on empty ref path", input: `{"$ref":"local::"}`, options: []EvalConfigOption{WithLocalScope(scope)}, wantErr: true, errContains: "invalid $ref syntax"},
		{name: "Should fail when local scope nil", input: `{"$ref":"local::foo"}`, wantErr: true, errContains: "local scope is not configured"},
		{name: "Should fail when global scope nil", input: `{"$ref":"global::foo"}`, wantErr: true, errContains: "global scope is not configured"},
	}
	runTestCases(t, cases)
}

// -----------------------------------------------------------------------------
// Determinism Tests
// -----------------------------------------------------------------------------

func TestDeterministicDirectivePick(t *testing.T) {
	t.Run("Should consistently fail on sibling keys", func(t *testing.T) {
		for range 10 {
			_, err := ProcessBytes([]byte(`{"$ref":"local::x","$use":"agent(local::y)"}`), WithLocalScope(map[string]any{"x": "a", "y": "b"}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "sibling keys")
		}
	})
}

// -----------------------------------------------------------------------------
// Performance Tests
// -----------------------------------------------------------------------------

func TestJSONCaching(t *testing.T) {
	t.Run("Should cache JSON representations efficiently", func(t *testing.T) {
		huge := make(map[string]any)
		for i := range 1_000 {
			huge[string(rune(i))] = map[string]any{"deep": map[string]any{"path": map[string]any{"value": i}}}
		}
		ev := NewEvaluator(WithLocalScope(huge))
		_, _ = ev.ResolvePath("local", "0.deep.path.value")
		allocs := testing.AllocsPerRun(100, func() {
			_, _ = ev.ResolvePath("local", "500.deep.path.value")
		})
		assert.Less(t, allocs, float64(100))
	})
}

// -----------------------------------------------------------------------------
// Concurrency Tests
// -----------------------------------------------------------------------------

func TestEvaluatorConcurrency(t *testing.T) {
	t.Run("Should be safe for concurrent use", func(t *testing.T) {
		scope := map[string]any{"value": "ok"}
		ev := NewEvaluator(WithLocalScope(scope))
		var wg sync.WaitGroup
		errs := make(chan error, 100)
		for range 100 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result, err := ev.Eval(map[string]any{"v": map[string]any{"$ref": "local::value"}})
				if err != nil {
					errs <- err
					return
				}
				if m, ok := result.(map[string]any); !ok || m["v"] != "ok" {
					errs <- errors.New("wrong result")
				}
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("race: %v", err)
		}
	})
}

// -----------------------------------------------------------------------------
// Edge Case Tests
// -----------------------------------------------------------------------------

func TestEdgeCases(t *testing.T) {
	t.Run("Should handle nil target", func(t *testing.T) {
		got := MustEvalBytes(t, []byte(`{"val":{"$ref":"local::nullable"}}`), WithLocalScope(map[string]any{"nullable": nil}))
		want := map[string]any{"val": nil}
		assert.Equal(t, want, got)
	})

	t.Run("Should handle unicode keys", func(t *testing.T) {
		got := MustEvalBytes(t, []byte(`{"$ref":"local::métricas.latência"}`), WithLocalScope(map[string]any{"métricas": map[string]any{"latência": 99}}))
		want := float64(99)
		assert.Equal(t, want, got)
	})

	t.Run("Should handle deep nesting", func(t *testing.T) {
		depth := 100
		v := map[string]any{"value": "end"}
		for i := 0; i < depth; i++ {
			v = map[string]any{"next": v}
		}
		scope := map[string]any{"root": v}
		path := "root"
		for i := 0; i < depth; i++ {
			path += ".next"
		}
		path += ".value"
		got := MustEvalBytes(t, []byte(`{"$ref":"local::`+path+`"}`), WithLocalScope(scope))
		want := "end"
		assert.Equal(t, want, got)
	})
}

// -----------------------------------------------------------------------------
// Public API Tests
// -----------------------------------------------------------------------------

func TestPublicAPI(t *testing.T) {
	yamlDoc := `
val:
  $ref: "local::k"`
	scope := map[string]any{"k": "yes"}
	expected := map[string]any{"val": "yes"}

	t.Run("Should process bytes correctly", func(t *testing.T) {
		got := MustEvalBytes(t, []byte(yamlDoc), WithLocalScope(scope))
		assert.Equal(t, expected, got)
	})

	t.Run("Should process reader correctly", func(t *testing.T) {
		r := bytes.NewReader([]byte(yamlDoc))
		got, err := ProcessReader(r, WithLocalScope(scope))
		require.NoError(t, err)
		assert.Equal(t, expected, got)
	})

	t.Run("Should process file correctly", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "doc.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yamlDoc), 0644))
		got := MustEval(t, file, WithLocalScope(scope))
		assert.Equal(t, expected, got)
	})
}

// -----------------------------------------------------------------------------
// Idempotence Tests
// -----------------------------------------------------------------------------

func TestIdempotenceAndRoundTrip(t *testing.T) {
	t.Run("Should be idempotent and support round-trip", func(t *testing.T) {
		scope := map[string]any{"config": map[string]any{"host": "localhost", "port": 8080}}
		input := `
name: demo
server:
  $ref: "local::config"`
		out1 := MustEvalBytes(t, []byte(input), WithLocalScope(scope))
		out2 := MustEvalBytes(t, []byte(input), WithLocalScope(scope))
		assert.Equal(t, out1, out2)
		yml, err := yaml.Marshal(out1)
		require.NoError(t, err)
		out3 := MustEvalBytes(t, yml, WithLocalScope(scope))
		normalizeNumbers(out1)
		normalizeNumbers(out3)
		expect := map[string]any{"name": "demo", "server": map[string]any{"host": "localhost", "port": float64(8080)}}
		assert.Equal(t, expect, out1)
		assert.Equal(t, expect, out3)
	})
}
