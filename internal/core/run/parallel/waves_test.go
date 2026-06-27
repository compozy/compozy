package parallelrun

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestBuildWaves(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tasks   []model.TaskEntry
		want    [][]TaskID
		wantErr error
	}{
		{
			name: "linear chain yields one task per wave",
			tasks: []model.TaskEntry{
				task("task_01"),
				task("task_02", "task_01"),
				task("task_03", "task_02"),
			},
			want: [][]TaskID{{"task_01"}, {"task_02"}, {"task_03"}},
		},
		{
			name: "worked example groups independent middle tasks",
			tasks: []model.TaskEntry{
				task("task_01"),
				task("task_02", "task_01"),
				task("task_03", "task_01"),
				task("task_04", "task_02", "task_03"),
				task("task_05", "task_04"),
			},
			want: [][]TaskID{{"task_01"}, {"task_02", "task_03"}, {"task_04"}, {"task_05"}},
		},
		{
			name: "diamond groups branches in one wave",
			tasks: []model.TaskEntry{
				task("task_01"),
				task("task_02", "task_01"),
				task("task_03", "task_01"),
				task("task_04", "task_02", "task_03"),
			},
			want: [][]TaskID{{"task_01"}, {"task_02", "task_03"}, {"task_04"}},
		},
		{
			name: "disconnected components occupy earliest possible waves",
			tasks: []model.TaskEntry{
				task("task_01"),
				task("task_02", "task_01"),
				task("task_03"),
				task("task_04", "task_03"),
			},
			want: [][]TaskID{{"task_01", "task_03"}, {"task_02", "task_04"}},
		},
		{
			name: "missing dependency returns typed hard error",
			tasks: []model.TaskEntry{
				task("task_01", "task_99"),
			},
			wantErr: ErrMissingDependency,
		},
		{
			name: "self loop returns typed cycle error",
			tasks: []model.TaskEntry{
				task("task_01", "task_01"),
			},
			wantErr: ErrCyclicDependencies,
		},
		{
			name: "three node cycle returns typed cycle error",
			tasks: []model.TaskEntry{
				task("task_01", "task_03"),
				task("task_02", "task_01"),
				task("task_03", "task_02"),
			},
			wantErr: ErrCyclicDependencies,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := BuildWaves(tt.tasks)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("BuildWaves() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildWaves(): %v", err)
			}
			if gotLevels := got.Levels(); !reflect.DeepEqual(gotLevels, tt.want) {
				t.Fatalf("unexpected waves\nwant: %#v\ngot:  %#v", tt.want, gotLevels)
			}
		})
	}
}

func TestBuildWavesOrdersWaveByTaskNumberAcrossPermutations(t *testing.T) {
	t.Parallel()

	inputs := [][]model.TaskEntry{
		{
			task("task_10"),
			task("task_02"),
			task("task_01"),
		},
		{
			task("task_01"),
			task("task_10"),
			task("task_02"),
		},
	}
	want := [][]TaskID{{"task_01", "task_02", "task_10"}}

	for _, input := range inputs {
		got, err := BuildWaves(input)
		if err != nil {
			t.Fatalf("BuildWaves(): %v", err)
		}
		if gotLevels := got.Levels(); !reflect.DeepEqual(gotLevels, want) {
			t.Fatalf("unexpected waves\nwant: %#v\ngot:  %#v", want, gotLevels)
		}
	}
}

func TestBuildWavesNormalizesTaskIDsAndDependencies(t *testing.T) {
	t.Parallel()

	got, err := BuildWaves([]model.TaskEntry{
		task("task_01.md"),
		task("task_02.md", "task_01.md"),
	})
	if err != nil {
		t.Fatalf("BuildWaves(): %v", err)
	}
	want := [][]TaskID{{"task_01"}, {"task_02"}}
	if gotLevels := got.Levels(); !reflect.DeepEqual(gotLevels, want) {
		t.Fatalf("unexpected waves\nwant: %#v\ngot:  %#v", want, gotLevels)
	}
}

func TestBuildWavesReportsCycleNodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		tasks []model.TaskEntry
		want  []TaskID
	}{
		{
			name: "self loop",
			tasks: []model.TaskEntry{
				task("task_01", "task_01"),
			},
			want: []TaskID{"task_01"},
		},
		{
			name: "three node cycle excludes downstream dependents",
			tasks: []model.TaskEntry{
				task("task_01", "task_03"),
				task("task_02", "task_01"),
				task("task_03", "task_02"),
				task("task_04", "task_03"),
			},
			want: []TaskID{"task_01", "task_02", "task_03"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := BuildWaves(tt.tasks)
			var cycleErr *CyclicDependenciesError
			if !errors.As(err, &cycleErr) {
				t.Fatalf("BuildWaves() error = %v, want CyclicDependenciesError", err)
			}
			if !reflect.DeepEqual(cycleErr.Nodes, tt.want) {
				t.Fatalf("unexpected cycle nodes\nwant: %#v\ngot:  %#v", tt.want, cycleErr.Nodes)
			}
		})
	}
}

func TestBuildWavesReportsMissingDependencyDetails(t *testing.T) {
	t.Parallel()

	t.Run("Should report missing source dependency from task frontmatter", func(t *testing.T) {
		t.Parallel()

		_, err := BuildWaves([]model.TaskEntry{
			task("task_02", "task_01.md"),
		})
		var missingErr *MissingDependencyError
		if !errors.As(err, &missingErr) {
			t.Fatalf("BuildWaves() error = %v, want MissingDependencyError", err)
		}
		if missingErr.TaskID != "task_02" || missingErr.Dependency != "task_01" {
			t.Fatalf("unexpected missing dependency detail: %#v", missingErr)
		}
	})

	t.Run("Should report missing target node without reversing dependency", func(t *testing.T) {
		t.Parallel()

		_, err := BuildWavesFromEdges(
			[]TaskID{"task_01"},
			[]DependencyEdge{{From: "task_01", To: "task_02"}},
		)
		var missingErr *MissingDependencyError
		if !errors.As(err, &missingErr) {
			t.Fatalf("BuildWavesFromEdges() error = %v, want MissingDependencyError", err)
		}
		if missingErr.TaskID != "task_02" || missingErr.Dependency != "task_01" {
			t.Fatalf("unexpected missing target detail: %#v", missingErr)
		}
	})
}

func TestWavesBlockedByReturnsTransitiveDependents(t *testing.T) {
	t.Parallel()

	waves, err := BuildWaves([]model.TaskEntry{
		task("task_01"),
		task("task_02", "task_01"),
		task("task_03", "task_01"),
		task("task_04", "task_02", "task_03"),
		task("task_05", "task_04"),
	})
	if err != nil {
		t.Fatalf("BuildWaves(): %v", err)
	}

	got := waves.BlockedBy(map[TaskID]bool{"task_03": true})
	want := map[TaskID]TaskID{
		"task_04": "task_03",
		"task_05": "task_03",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected blocked tasks\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestWavesBlockedByExcludesFailedTasks(t *testing.T) {
	t.Parallel()

	waves, err := BuildWaves([]model.TaskEntry{
		task("task_01"),
		task("task_02", "task_01"),
		task("task_03", "task_02"),
	})
	if err != nil {
		t.Fatalf("BuildWaves(): %v", err)
	}

	got := waves.BlockedBy(map[TaskID]bool{"task_01": true, "task_02": true})
	want := map[TaskID]TaskID{"task_03": "task_02"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected blocked tasks\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildWavesRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tasks   []model.TaskEntry
		wantErr string
	}{
		{
			name:    "missing id",
			tasks:   []model.TaskEntry{{Title: "No ID"}},
			wantErr: "missing task id",
		},
		{
			name: "duplicate normalized id",
			tasks: []model.TaskEntry{
				task("task_01"),
				task("task_01.md"),
			},
			wantErr: "duplicate task id",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := BuildWaves(tt.tasks)
			if err == nil {
				t.Fatal("expected BuildWaves() to fail")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func task(id string, dependencies ...string) model.TaskEntry {
	return model.TaskEntry{
		ID:           id,
		Status:       "pending",
		Title:        id,
		TaskType:     "backend",
		Dependencies: dependencies,
	}
}
