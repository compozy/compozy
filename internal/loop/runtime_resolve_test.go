package loop_test

import (
	"reflect"
	"testing"

	loop "github.com/compozy/compozy/internal/loop"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestResolveItemRuntimeShouldMergeFieldsByPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve defaults with default provenance", func(t *testing.T) {
		t.Parallel()

		got := resolveRuntimeForTest(t, loop.RuntimeLayers{
			Defaults: loop.RuntimeSpec{Provider: "claude", Model: "opus", Reasoning: "xhigh"},
		}, loop.ItemRuntime{})
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{Provider: "claude", Model: "opus", Reasoning: "xhigh"},
			loop.RuntimeProvenance{
				Provider: loop.RuntimeSourceDefault,
				Model:    loop.RuntimeSourceDefault, Reasoning: loop.RuntimeSourceDefault,
			},
		)
	})

	t.Run("Should merge frontmatter one field at a time", func(t *testing.T) {
		t.Parallel()

		got := resolveRuntimeForTest(t, loop.RuntimeLayers{
			Defaults: loop.RuntimeSpec{Provider: "codex", Model: "gpt-5.4", Reasoning: "high"},
		}, loop.ItemRuntime{Frontmatter: loop.RuntimeSpec{Model: "gpt-5.5"}})
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{Provider: "codex", Model: "gpt-5.5", Reasoning: "high"},
			loop.RuntimeProvenance{
				Provider: loop.RuntimeSourceDefault,
				Model:    loop.RuntimeSourceFrontmatter, Reasoning: loop.RuntimeSourceDefault,
			},
		)
	})

	t.Run("Should retain the source of each independently resolved field", func(t *testing.T) {
		t.Parallel()

		got := resolveRuntimeForTest(t, loop.RuntimeLayers{
			ConfigRules: []loop.RuntimeRule{{
				Match: loop.RuntimeMatch{Type: "frontend"}, Runtime: loop.RuntimeSpec{Reasoning: "high"},
			}},
			RunRules: []loop.RuntimeRule{{
				Match: loop.RuntimeMatch{Type: "frontend"}, Runtime: loop.RuntimeSpec{Provider: "claude"},
			}},
		}, loop.ItemRuntime{
			TaskID: "task_01", TaskType: "frontend", Frontmatter: loop.RuntimeSpec{Model: "opus"},
		})
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{Provider: "claude", Model: "opus", Reasoning: "high"},
			loop.RuntimeProvenance{
				Provider: loop.RuntimeSourceRun,
				Model:    loop.RuntimeSourceFrontmatter, Reasoning: loop.RuntimeSourceConfig,
			},
		)
	})

	t.Run("Should prefer id then type then complexity per field and later equal rules", func(t *testing.T) {
		t.Parallel()

		got := resolveRuntimeForTest(t, loop.RuntimeLayers{ConfigRules: []loop.RuntimeRule{
			{Match: loop.RuntimeMatch{Complexity: "high"}, Runtime: loop.RuntimeSpec{
				Provider: "complexity-provider", Model: "complexity-model", Reasoning: "low",
			}},
			{Match: loop.RuntimeMatch{Type: "frontend"}, Runtime: loop.RuntimeSpec{
				Provider: "type-provider", Model: "type-model",
			}},
			{Match: loop.RuntimeMatch{ID: "task_01"}, Runtime: loop.RuntimeSpec{Provider: "id-provider"}},
			{Match: loop.RuntimeMatch{Type: "frontend"}, Runtime: loop.RuntimeSpec{
				Model: "later-type-model", Reasoning: "medium",
			}},
			{Match: loop.RuntimeMatch{ID: "task_01"}, Runtime: loop.RuntimeSpec{Provider: "later-id-provider"}},
		}}, loop.ItemRuntime{TaskID: "task_01", TaskType: "frontend", Complexity: "high"})
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{Provider: "later-id-provider", Model: "later-type-model", Reasoning: "medium"},
			loop.RuntimeProvenance{
				Provider: loop.RuntimeSourceConfig,
				Model:    loop.RuntimeSourceConfig, Reasoning: loop.RuntimeSourceConfig,
			},
		)
	})

	t.Run("Should require type and complexity conjunctions to match together", func(t *testing.T) {
		t.Parallel()

		matrix := loop.RuntimeRule{
			Match:   loop.RuntimeMatch{Type: "frontend", Complexity: "high"},
			Runtime: loop.RuntimeSpec{Provider: "cursor", Model: "frontier", Reasoning: "high"},
		}
		for name, item := range map[string]loop.ItemRuntime{
			"Should match frontend high":        {TaskType: "frontend", Complexity: "high"},
			"Should not match frontend low":     {TaskType: "frontend", Complexity: "low"},
			"Should not match backend high":     {TaskType: "backend", Complexity: "high"},
			"Should not match empty type":       {Complexity: "high"},
			"Should not match empty complexity": {TaskType: "frontend"},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				got := resolveRuntimeForTest(t, loop.RuntimeLayers{ConfigRules: []loop.RuntimeRule{matrix}}, item)
				if item.TaskType == "frontend" && item.Complexity == "high" {
					assertResolvedRuntime(t, got, matrix.Runtime, loop.RuntimeProvenance{
						Provider:  loop.RuntimeSourceConfig,
						Model:     loop.RuntimeSourceConfig,
						Reasoning: loop.RuntimeSourceConfig,
					})
					return
				}
				assertResolvedRuntime(t, got, loop.RuntimeSpec{}, loop.RuntimeProvenance{})
			})
		}
	})

	t.Run("Should prefer exact over matrix over type over complexity per field", func(t *testing.T) {
		t.Parallel()

		rules := []loop.RuntimeRule{
			{
				Match:   loop.RuntimeMatch{Complexity: "high"},
				Runtime: loop.RuntimeSpec{Reasoning: "medium", Model: "complexity"},
			},
			{Match: loop.RuntimeMatch{Type: "frontend"}, Runtime: loop.RuntimeSpec{Provider: "type", Model: "type"}},
			{
				Match: loop.RuntimeMatch{Type: "frontend", Complexity: "high"},
				Runtime: loop.RuntimeSpec{
					Provider: "matrix", Model: "matrix", Reasoning: "high",
				},
			},
			{Match: loop.RuntimeMatch{ID: "task_01"}, Runtime: loop.RuntimeSpec{Provider: "exact"}},
		}
		got := resolveRuntimeForTest(t, loop.RuntimeLayers{ConfigRules: rules}, loop.ItemRuntime{
			TaskID: "task_01", TaskType: "frontend", Complexity: "high",
		})
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{Provider: "exact", Model: "matrix", Reasoning: "high"},
			loop.RuntimeProvenance{
				Provider:  loop.RuntimeSourceConfig,
				Model:     loop.RuntimeSourceConfig,
				Reasoning: loop.RuntimeSourceConfig,
			},
		)
	})

	t.Run("Should merge disjoint matrix fields and prefer later equal-specificity fields", func(t *testing.T) {
		t.Parallel()

		matrixMatch := loop.RuntimeMatch{Type: "frontend", Complexity: "high"}
		got := resolveRuntimeForTest(t, loop.RuntimeLayers{ConfigRules: []loop.RuntimeRule{
			{Match: matrixMatch, Runtime: loop.RuntimeSpec{Provider: "codex", Model: "first-model"}},
			{Match: matrixMatch, Runtime: loop.RuntimeSpec{Reasoning: "high"}},
			{Match: matrixMatch, Runtime: loop.RuntimeSpec{Model: "later-model"}},
		}}, loop.ItemRuntime{TaskType: "frontend", Complexity: "high"})
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{Provider: "codex", Model: "later-model", Reasoning: "high"},
			loop.RuntimeProvenance{
				Provider: loop.RuntimeSourceConfig,
				Model:    loop.RuntimeSourceConfig, Reasoning: loop.RuntimeSourceConfig,
			},
		)
	})

	t.Run("Should enforce pairwise item layer precedence", func(t *testing.T) {
		t.Parallel()

		item := loop.ItemRuntime{
			TaskID: "task_01", TaskType: "frontend", Complexity: "high",
			Node: loop.RuntimeSpec{Provider: "node-provider", Model: "node-model", Reasoning: "low"},
			Frontmatter: loop.RuntimeSpec{
				Provider: "frontmatter-provider", Model: "frontmatter-model", Reasoning: "high",
			},
		}
		got := resolveRuntimeForTest(t, loop.RuntimeLayers{
			Defaults: loop.RuntimeSpec{
				Provider: "default-provider", Model: "default-model", Reasoning: "none",
			},
			ConfigRules: []loop.RuntimeRule{{
				Match: loop.RuntimeMatch{Type: "frontend"},
				Runtime: loop.RuntimeSpec{
					Provider: "config-provider", Model: "config-model", Reasoning: "medium",
				},
			}},
			RunRules: []loop.RuntimeRule{{
				Match:   loop.RuntimeMatch{ID: "task_01"},
				Runtime: loop.RuntimeSpec{Provider: "run-provider", Reasoning: "max"},
			}},
		}, item)
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{Provider: "run-provider", Model: "frontmatter-model", Reasoning: "max"},
			loop.RuntimeProvenance{
				Provider: loop.RuntimeSourceRun,
				Model:    loop.RuntimeSourceFrontmatter, Reasoning: loop.RuntimeSourceRun,
			},
		)
	})

	t.Run("Should place typed input between config and frontmatter per field", func(t *testing.T) {
		t.Parallel()

		got := resolveRuntimeForTest(t, loop.RuntimeLayers{
			Defaults: loop.RuntimeSpec{
				Provider: "default", Model: "default", Reasoning: "none", Speed: speedpkg.SpeedNormal,
			},
			ConfigRules: []loop.RuntimeRule{{
				Match: loop.RuntimeMatch{Type: "frontend"},
				Runtime: loop.RuntimeSpec{
					Provider: "config", Model: "config", Reasoning: "medium", Speed: speedpkg.SpeedNormal,
				},
			}},
			RunRules: []loop.RuntimeRule{{
				Match:   loop.RuntimeMatch{ID: "task_01"},
				Runtime: loop.RuntimeSpec{Reasoning: "max"},
			}},
		}, loop.ItemRuntime{
			TaskID: "task_01", TaskType: "frontend",
			Node: loop.RuntimeSpec{Provider: "node", Model: "node", Reasoning: "low"},
			Input: loop.RuntimeSpec{
				Provider: "input", Model: "input", Reasoning: "high", Speed: speedpkg.SpeedFast,
			},
			Frontmatter: loop.RuntimeSpec{Model: "frontmatter"},
		})
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{
				Provider: "input", Model: "frontmatter", Reasoning: "max", Speed: speedpkg.SpeedFast,
			},
			loop.RuntimeProvenance{
				Provider:  loop.RuntimeSourceInput,
				Model:     loop.RuntimeSourceFrontmatter,
				Reasoning: loop.RuntimeSourceRun,
				Speed:     loop.RuntimeSourceInput,
			},
		)
	})

	t.Run("Should apply defaults only to non task items", func(t *testing.T) {
		t.Parallel()

		got := resolveRuntimeForTest(t, loop.RuntimeLayers{
			Defaults: loop.RuntimeSpec{Model: "default-model"},
			ConfigRules: []loop.RuntimeRule{{
				Match: loop.RuntimeMatch{Type: "frontend"}, Runtime: loop.RuntimeSpec{Model: "matched-model"},
			}},
		}, loop.ItemRuntime{})
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{Model: "default-model"},
			loop.RuntimeProvenance{Model: loop.RuntimeSourceDefault},
		)
	})

	t.Run("Should leave unmatched fields empty for the binder", func(t *testing.T) {
		t.Parallel()

		got := resolveRuntimeForTest(t, loop.RuntimeLayers{}, loop.ItemRuntime{})
		assertResolvedRuntime(t, got, loop.RuntimeSpec{}, loop.RuntimeProvenance{})
	})

	t.Run("Should let rendered node runtime win only over defaults", func(t *testing.T) {
		t.Parallel()

		got := resolveRuntimeForTest(t, loop.RuntimeLayers{
			Defaults: loop.RuntimeSpec{Provider: "claude", Model: "opus", Reasoning: "high"},
			ConfigRules: []loop.RuntimeRule{{
				Match: loop.RuntimeMatch{Type: "frontend"}, Runtime: loop.RuntimeSpec{Provider: "config-provider"},
			}},
			RunRules: []loop.RuntimeRule{{
				Match: loop.RuntimeMatch{ID: "task_01"}, Runtime: loop.RuntimeSpec{Reasoning: "max"},
			}},
		}, loop.ItemRuntime{
			TaskID: "task_01", TaskType: "frontend",
			Node:        loop.RuntimeSpec{Provider: "node-provider", Model: "sonnet", Reasoning: "low"},
			Frontmatter: loop.RuntimeSpec{Model: "frontmatter-model"},
		})
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{Provider: "config-provider", Model: "frontmatter-model", Reasoning: "max"},
			loop.RuntimeProvenance{
				Provider: loop.RuntimeSourceConfig,
				Model:    loop.RuntimeSourceFrontmatter, Reasoning: loop.RuntimeSourceRun,
			},
		)
	})

	t.Run("Should source a node model and keep default fields", func(t *testing.T) {
		t.Parallel()

		got := resolveRuntimeForTest(t, loop.RuntimeLayers{
			Defaults: loop.RuntimeSpec{Provider: "claude", Model: "opus", Reasoning: "high"},
		}, loop.ItemRuntime{Node: loop.RuntimeSpec{Model: "sonnet"}})
		assertResolvedRuntime(t, got,
			loop.RuntimeSpec{Provider: "claude", Model: "sonnet", Reasoning: "high"},
			loop.RuntimeProvenance{
				Provider: loop.RuntimeSourceDefault,
				Model:    loop.RuntimeSourceNode, Reasoning: loop.RuntimeSourceDefault,
			},
		)
	})
}

func resolveRuntimeForTest(t *testing.T, layers loop.RuntimeLayers, item loop.ItemRuntime) loop.ResolvedRuntime {
	t.Helper()
	resolved, err := loop.ResolveItemRuntime(layers, item)
	if err != nil {
		t.Fatalf("ResolveItemRuntime() error = %v", err)
	}
	return resolved
}

func assertResolvedRuntime(
	t *testing.T,
	got loop.ResolvedRuntime,
	wantRuntime loop.RuntimeSpec,
	wantSource loop.RuntimeProvenance,
) {
	t.Helper()
	if got.Runtime.Provider != wantRuntime.Provider || got.Runtime.Model != wantRuntime.Model ||
		got.Runtime.Reasoning != wantRuntime.Reasoning || got.Runtime.Speed != wantRuntime.Speed {
		t.Fatalf("ResolvedRuntime.Runtime = %#v, want %#v", got.Runtime, wantRuntime)
	}
	if !reflect.DeepEqual(got.Source, wantSource) {
		t.Fatalf("ResolvedRuntime.Source = %#v, want %#v", got.Source, wantSource)
	}
}
