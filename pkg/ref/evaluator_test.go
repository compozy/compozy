package ref

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		{
			name:  "Should fail on cyclic reference",
			input: `{"$ref":"local::a"}`,
			options: []EvalConfigOption{WithLocalScope(map[string]any{
				"a": map[string]any{"$ref": "local::b"},
				"b": map[string]any{"$ref": "local::a"},
			})},
			wantErr:     true,
			errContains: "cyclic reference",
		},
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
// Merge Directive Tests
// -----------------------------------------------------------------------------

func TestMergeDirective(t *testing.T) {
	localScope := map[string]any{
		"defaults": map[string]any{
			"deploy": map[string]any{
				"replicas": 1,
				"resources": map[string]any{
					"cpu":    "100m",
					"memory": "128Mi",
				},
			},
		},
		"base_tags": []any{"base", "common"},
		"prod": map[string]any{
			"deploy": map[string]any{
				"replicas": 3,
				"resources": map[string]any{
					"cpu": "500m",
				},
			},
		},
		"extra_tags": []any{"extra", "additional"},
	}

	globalScope := map[string]any{
		"envs": map[string]any{
			"prod": map[string]any{
				"deploy": map[string]any{
					"resources": map[string]any{
						"memory": "512Mi",
					},
					"autoscaling": true,
				},
			},
		},
	}

	t.Run("Should merge objects with shorthand syntax", func(t *testing.T) {
		input := `{"deploy":{"$merge":[{"host":"localhost","port":80},{"port":8080,"proto":"https"}]}}`
		got := MustEvalBytes(t, []byte(input))
		want := map[string]any{
			"deploy": map[string]any{
				"host":  "localhost",
				"port":  float64(8080),
				"proto": "https",
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should merge arrays with shorthand syntax", func(t *testing.T) {
		input := `{"tags":{"$merge":[["auth","logging"],["metrics","tracing"]]}}`
		got := MustEvalBytes(t, []byte(input))
		want := map[string]any{
			"tags": []any{"auth", "logging", "metrics", "tracing"},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should deep merge objects by default", func(t *testing.T) {
		input := `{"deploy":{"$merge":[{"$ref":"local::defaults.deploy"},{"$ref":"local::prod.deploy"}]}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(localScope))
		want := map[string]any{
			"deploy": map[string]any{
				"replicas": float64(3),
				"resources": map[string]any{
					"cpu":    "500m",
					"memory": "128Mi",
				},
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should merge with explicit deep strategy", func(t *testing.T) {
		input := `{"deploy":{"$merge":{"strategy":"deep","sources":[{"$ref":"local::defaults.deploy"},{"$ref":"global::envs.prod.deploy"},{"retries":5}]}}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(localScope), WithGlobalScope(globalScope))
		want := map[string]any{
			"deploy": map[string]any{
				"replicas": float64(1),
				"resources": map[string]any{
					"cpu":    "100m",
					"memory": "512Mi",
				},
				"autoscaling": true,
				"retries":     float64(5),
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should merge with shallow strategy", func(t *testing.T) {
		input := `{"deploy":{"$merge":{"strategy":"shallow","sources":[{"$ref":"local::defaults.deploy"},{"$ref":"local::prod.deploy"}]}}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(localScope))
		want := map[string]any{
			"deploy": map[string]any{
				"replicas": float64(3),
				"resources": map[string]any{
					"cpu": "500m",
				},
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should handle key_conflict=first", func(t *testing.T) {
		input := `{"config":{"$merge":{"key_conflict":"first","sources":[{"port":8080},{"port":9090},{"host":"localhost"}]}}}`
		got := MustEvalBytes(t, []byte(input))
		want := map[string]any{
			"config": map[string]any{
				"port": float64(8080),
				"host": "localhost",
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should handle key_conflict=error", func(t *testing.T) {
		input := `{"config":{"$merge":{"key_conflict":"error","sources":[{"port":8080},{"port":9090}]}}}`
		_, err := ProcessBytes([]byte(input))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key conflict: 'port' already exists")
	})

	t.Run("Should merge arrays with unique strategy", func(t *testing.T) {
		input := `{"tags":{"$merge":{"strategy":"unique","sources":[{"$ref":"local::base_tags"},["build","docker","build"]]}}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(localScope))
		want := map[string]any{
			"tags": []any{"base", "common", "build", "docker"},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should merge arrays with prepend strategy", func(t *testing.T) {
		input := `{"tags":{"$merge":{"strategy":"prepend","sources":[["first","second"],["third","fourth"]]}}}`
		got := MustEvalBytes(t, []byte(input))
		want := map[string]any{
			"tags": []any{"third", "fourth", "first", "second"},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should handle nil sources gracefully", func(t *testing.T) {
		input := `{"config":{"$merge":[{"$ref":"local::nullable"},{"port":8080}]}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(map[string]any{"nullable": nil}))
		want := map[string]any{
			"config": map[string]any{
				"port": float64(8080),
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should resolve nested directives in merge sources", func(t *testing.T) {
		nestedScope := map[string]any{
			"configs": []any{
				map[string]any{"$ref": "local::base.config"},
				map[string]any{"name": "override"},
			},
			"base": map[string]any{
				"config": map[string]any{"host": "localhost", "port": 3000},
			},
		}
		input := `{"result":{"$merge":{"$ref":"local::configs"}}}`
		got := MustEvalBytes(t, []byte(input), WithLocalScope(nestedScope))
		want := map[string]any{
			"result": map[string]any{
				"host": "localhost",
				"port": float64(3000),
				"name": "override",
			},
		}
		assert.Equal(t, want, got)
	})
}

func TestMergeDirectiveErrors(t *testing.T) {
	cases := []testCase{
		{
			name:        "Should fail on empty sources",
			input:       `{"$merge":[]}`,
			wantErr:     true,
			errContains: "sources cannot be empty",
		},
		{
			name:        "Should fail on mixed source types",
			input:       `{"$merge":[{"key":"value"},["array","item"]]}`,
			wantErr:     true,
			errContains: "must be all objects or all arrays",
		},
		{
			name:        "Should fail on invalid object strategy",
			input:       `{"$merge":{"strategy":"invalid","sources":[{"a":1},{"b":2}]}}`,
			wantErr:     true,
			errContains: "invalid object merge strategy",
		},
		{
			name:        "Should fail on invalid array strategy",
			input:       `{"$merge":{"strategy":"invalid","sources":[["a"],["b"]]}}`,
			wantErr:     true,
			errContains: "invalid array merge strategy",
		},
		{
			name:        "Should fail on invalid key_conflict",
			input:       `{"$merge":{"key_conflict":"invalid","sources":[{"a":1},{"b":2}]}}`,
			wantErr:     true,
			errContains: "invalid key_conflict",
		},
		{
			name:        "Should fail on unknown merge option",
			input:       `{"$merge":{"unknown":"option","sources":[{"a":1}]}}`,
			wantErr:     true,
			errContains: "unknown key in $merge",
		},
		{
			name:        "Should fail on missing sources in mapping",
			input:       `{"$merge":{"strategy":"deep"}}`,
			wantErr:     true,
			errContains: "must contain 'sources' key",
		},
		{
			name:        "Should fail when sources is not a sequence",
			input:       `{"$merge":{"sources":"not-a-list"}}`,
			wantErr:     true,
			errContains: "sources must be a sequence",
		},
		{
			name:        "Should fail on scalar merge source",
			input:       `{"$merge":["string",{"key":"value"}]}`,
			wantErr:     true,
			errContains: "must be an object or array",
		},
		{
			name:        "Should fail on sibling keys with $merge",
			input:       `{"$merge":[{"a":1}],"extra":"key"}`,
			wantErr:     true,
			errContains: "sibling keys",
		},
		{
			name:        "Should fail when $ref evaluates to scalar",
			input:       `{"$merge":[{"$ref":"local::scalar"},{"key":"value"}]}`,
			options:     []EvalConfigOption{WithLocalScope(map[string]any{"scalar": "string value"})},
			wantErr:     true,
			errContains: "must be an object or array",
		},
	}
	runTestCases(t, cases)
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

func TestWithScopes(t *testing.T) {
	localScope := map[string]any{
		"local_key": "local_value",
	}
	globalScope := map[string]any{
		"global_key": "global_value",
	}

	t.Run("Should set both scopes with WithScopes", func(t *testing.T) {
		yamlDoc := `
local:
  $ref: "local::local_key"
global:
  $ref: "global::global_key"`

		got := MustEvalBytes(t, []byte(yamlDoc), WithScopes(localScope, globalScope))
		want := map[string]any{
			"local":  "local_value",
			"global": "global_value",
		}
		assert.Equal(t, want, got)
	})
}

func TestWithPreEval(t *testing.T) {
	t.Run("Should apply pre-eval hook to transform nodes", func(t *testing.T) {
		// Pre-eval hook that converts strings to uppercase
		preEval := func(node Node) (Node, error) {
			if str, ok := node.(string); ok {
				return strings.ToUpper(str), nil
			}
			return node, nil
		}

		yamlDoc := `
name: john
city: paris`

		got := MustEvalBytes(t, []byte(yamlDoc), WithPreEval(preEval))
		want := map[string]any{
			"name": "JOHN",
			"city": "PARIS",
		}
		assert.Equal(t, want, got)
	})

	t.Run("Should handle pre-eval errors", func(t *testing.T) {
		// Pre-eval hook that fails on specific values
		preEval := func(node Node) (Node, error) {
			if str, ok := node.(string); ok && str == "forbidden" {
				return nil, fmt.Errorf("forbidden value")
			}
			return node, nil
		}

		yamlDoc := `value: forbidden`

		_, err := ProcessBytes([]byte(yamlDoc), WithPreEval(preEval))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pre-evaluation hook failed")
		assert.Contains(t, err.Error(), "forbidden value")
	})

	t.Run("Should apply pre-eval before directive evaluation", func(t *testing.T) {
		// Pre-eval hook that transforms a special prefix into a directive
		preEval := func(node Node) (Node, error) {
			if str, ok := node.(string); ok && strings.HasPrefix(str, "REF:") {
				ref := strings.TrimPrefix(str, "REF:")
				return map[string]any{"$ref": ref}, nil
			}
			return node, nil
		}

		localScope := map[string]any{"value": "success"}
		yamlDoc := `result: REF:local::value`

		got := MustEvalBytes(t, []byte(yamlDoc), WithLocalScope(localScope), WithPreEval(preEval))
		want := map[string]any{"result": "success"}
		assert.Equal(t, want, got)
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

// -----------------------------------------------------------------------------
// Directive Registration Tests
// -----------------------------------------------------------------------------

func TestRegisterDirective(t *testing.T) {
	// Custom directive that doubles numbers
	doubleDirective := Directive{
		Name: "$double",
		Validator: func(node Node) error {
			switch v := node.(type) {
			case float64, int:
				return nil
			default:
				return fmt.Errorf("$double expects a number, got %T", v)
			}
		},
		Handler: func(_ EvaluatorContext, node Node) (Node, error) {
			switch v := node.(type) {
			case float64:
				return v * 2, nil
			case int:
				return float64(v) * 2, nil
			default:
				return nil, fmt.Errorf("$double expects a number")
			}
		},
	}

	t.Run("Should register custom directive", func(t *testing.T) {
		err := Register(doubleDirective)
		require.NoError(t, err)

		// Test using the custom directive
		input := `{"result": {"$double": 21}}`
		got := MustEvalBytes(t, []byte(input))
		want := map[string]any{"result": float64(42)}
		assert.Equal(t, want, got)
	})

	t.Run("Should fail to register duplicate directive", func(t *testing.T) {
		err := Register(doubleDirective)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("Should fail to register directive without $", func(t *testing.T) {
		err := Register(Directive{Name: "invalid", Handler: func(_ EvaluatorContext, _ Node) (Node, error) { return nil, nil }})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must start with '$'")
	})

	t.Run("Should fail to register directive without handler", func(t *testing.T) {
		err := Register(Directive{Name: "$nohandler"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "handler cannot be nil")
	})

	t.Run("Should fail to register directive without name", func(t *testing.T) {
		err := Register(Directive{Handler: func(_ EvaluatorContext, _ Node) (Node, error) { return nil, nil }})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name cannot be empty")
	})
}
