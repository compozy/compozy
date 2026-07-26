package windowmanager

// Suite: window-manager topology
// Invariant: normalized topology is deterministic and every accepted tree satisfies ownership and geometry rules.
// Boundary IN: normalizer, validator, layout reducer, and real in-memory repository.
// Boundary OUT: HTTP/UDS DTOs and browser pixel projection.

import (
	"errors"
	"math"
	"slices"
	"testing"
)

func TestInitialSnapshot(t *testing.T) {
	t.Run("Should return one stable standard desktop without persisting a read", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		first, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("first Snapshot() error = %v", err)
		}
		second, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("second Snapshot() error = %v", err)
		}
		if first.Revision != 0 || len(first.Desktops) != 1 || first.Desktops[0].Purpose != DesktopPurposeStandard ||
			len(first.Windows) != 0 {
			t.Fatalf("initial snapshot = %+v", first)
		}
		equal, err := statesEqual(snapshotState(first), snapshotState(second))
		if err != nil {
			t.Fatalf("statesEqual() error = %v", err)
		}
		if !equal {
			t.Fatal("fresh snapshots differ")
		}
		if commits := environment.repository.Commits("workspace-a"); len(commits) != 0 {
			t.Fatalf("read commits = %d, want 0", len(commits))
		}
		requireValidSnapshot(t, first)
	})
}

func TestNormalizeSnapshot(t *testing.T) {
	t.Run("Should flatten same-axis splits, collapse singleton stacks, and remain idempotent", func(t *testing.T) {
		t.Parallel()
		w1, w2, w3, missing := WindowID("w1"), WindowID("w2"), WindowID("w3"), WindowID("missing")
		axis := AxisHorizontal
		snapshot := Snapshot{
			Version:     SnapshotVersion,
			WorkspaceID: "workspace-a",
			Desktops: []Desktop{
				{
					ID:      "d1",
					Name:    "One",
					Order:   4,
					Purpose: DesktopPurposeStandard,
					Groups: []LayoutGroup{
						{
							ID:    "g1",
							Frame: fullRect(),
							Root: LayoutNode{
								ID:      "root",
								Kind:    NodeKindSplit,
								Axis:    &axis,
								Weights: []float64{0.4, 0.6},
								Children: []LayoutNode{
									{ID: "n1", Kind: NodeKindLeaf, WindowID: &w1},
									{
										ID:      "nested",
										Kind:    NodeKindSplit,
										Axis:    &axis,
										Weights: []float64{2, 2, 2},
										Children: []LayoutNode{
											{ID: "n2", Kind: NodeKindLeaf, WindowID: &w2},
											{ID: "s1", Kind: NodeKindStack, WindowIDs: []WindowID{w3}, ActiveID: &w3},
											{ID: "bad", Kind: NodeKindLeaf, WindowID: &missing},
										},
									},
								},
							},
						},
					},
					Floating: []WindowID{w1, w2},
				},
			},
			Windows: map[WindowID]Window{
				w1: {ID: w1, App: "One", Route: testRoute("/one"), DesktopID: "d1", FloatingRect: fullRect()},
				w2: {ID: w2, App: "Two", Route: testRoute("/two"), DesktopID: "d1", FloatingRect: fullRect()},
				w3: {
					ID: w3, App: "Three", Route: testRoute("/three"), DesktopID: "d1",
					FloatingRect: fullRect(),
				},
			},
		}
		first := NormalizeSnapshot(snapshot)
		second := NormalizeSnapshot(first)
		equal, err := statesEqual(snapshotState(first), snapshotState(second))
		if err != nil {
			t.Fatalf("statesEqual() error = %v", err)
		}
		if !equal {
			t.Fatalf("normalization is not idempotent\nfirst=%+v\nsecond=%+v", first, second)
		}
		root := first.Desktops[0].Groups[0].Root
		if root.Kind != NodeKindSplit || len(root.Children) != 3 {
			t.Fatalf("normalized root = %+v", root)
		}
		wantWindowIDs := []WindowID{w1, w2, w3}
		wantNodeIDs := []NodeID{"n1", "n2", "s1"}
		wantWeights := []float64{0.4, 0.3, 0.3}
		for index, child := range root.Children {
			if child.Kind != NodeKindLeaf || child.ID != wantNodeIDs[index] ||
				valueOrZero(child.WindowID) != wantWindowIDs[index] ||
				math.Abs(root.Weights[index]-wantWeights[index]) > weightTolerance {
				t.Fatalf("normalized child[%d] = %+v weight=%f", index, child, root.Weights[index])
			}
		}
		if len(first.Desktops[0].Floating) != 0 {
			t.Fatalf("floating duplicates = %v", first.Desktops[0].Floating)
		}
		requireValidSnapshot(t, first)
	})

	t.Run("Should keep normalized split weights byte-identical across repeated passes", func(t *testing.T) {
		t.Parallel()
		w1, w2, w3 := WindowID("w1"), WindowID("w2"), WindowID("w3")
		axis := AxisHorizontal
		snapshot := Snapshot{
			Version:     SnapshotVersion,
			WorkspaceID: "workspace-a",
			Desktops: []Desktop{
				{
					ID:      "d1",
					Name:    "One",
					Purpose: DesktopPurposeStandard,
					Groups: []LayoutGroup{
						{
							ID:    "g1",
							Frame: fullRect(),
							Root: LayoutNode{
								ID:   "split",
								Kind: NodeKindSplit,
								Axis: &axis,
								Children: []LayoutNode{
									{ID: "n1", Kind: NodeKindLeaf, WindowID: &w1},
									{ID: "n2", Kind: NodeKindLeaf, WindowID: &w2},
									{ID: "n3", Kind: NodeKindLeaf, WindowID: &w3},
								},
								// Raw weights whose normalized sum lands within float
								// noise of 1: a second pass must not drift them.
								Weights: []float64{1, 2, 3},
							},
						},
					},
					Floating: []WindowID{},
				},
			},
			Windows: map[WindowID]Window{
				w1: {ID: w1, App: "Test", Route: testRoute("/test"), DesktopID: "d1", FloatingRect: fullRect()},
				w2: {ID: w2, App: "Test", Route: testRoute("/test"), DesktopID: "d1", FloatingRect: fullRect()},
				w3: {ID: w3, App: "Test", Route: testRoute("/test"), DesktopID: "d1", FloatingRect: fullRect()},
			},
		}
		first := NormalizeSnapshot(snapshot)
		second := NormalizeSnapshot(first)
		firstWeights := first.Desktops[0].Groups[0].Root.Weights
		secondWeights := second.Desktops[0].Groups[0].Root.Weights
		if !slices.Equal(firstWeights, secondWeights) {
			t.Fatalf("weights drifted across passes: first=%v second=%v", firstWeights, secondWeights)
		}
	})
}

func TestValidateSnapshot(t *testing.T) {
	tests := []struct {
		name, code, path string
		mutate           func(*Snapshot)
	}{
		{
			name:   "Should reject duplicate node identities",
			code:   "topology.node_cycle_or_duplicate",
			mutate: func(snapshot *Snapshot) { snapshot.Desktops[0].Groups[0].Root.Children[1].ID = "leaf-1" },
		},
		{
			name: "Should reject unknown window members",
			code: "topology.window_unknown",
			mutate: func(snapshot *Snapshot) {
				unknown := WindowID("unknown")
				snapshot.Desktops[0].Groups[0].Root.Children[0].WindowID = &unknown
			},
		},
		{
			name: "Should reject overlapping group frames",
			code: "topology.group_overlap",
			mutate: func(snapshot *Snapshot) {
				w4 := WindowID("w4")
				snapshot.Windows[w4] = Window{
					ID:           w4,
					App:          "Four",
					DesktopID:    "desktop-default",
					Route:        testRoute("/four"),
					Placement:    WindowPlacementTiled,
					FloatingRect: fullRect(),
				}
				snapshot.Desktops[0].Groups = append(
					snapshot.Desktops[0].Groups,
					LayoutGroup{
						ID:    "g2",
						Frame: fullRect(),
						Root:  LayoutNode{ID: "l4", Kind: NodeKindLeaf, WindowID: &w4},
					},
				)
			},
		},
		{
			name:   "Should reject invalid split weights",
			code:   "topology.split_weight",
			mutate: func(snapshot *Snapshot) { snapshot.Desktops[0].Groups[0].Root.Weights[0] = math.NaN() },
		},
		{
			name:   "Should reject an absent final desktop",
			code:   "topology.final_desktop_required",
			mutate: func(snapshot *Snapshot) { snapshot.Desktops = nil; snapshot.Windows = map[WindowID]Window{} },
		},
		{
			name: "Should reject a focus owner from another desktop",
			code: "topology.focus_owner_desktop",
			path: "desktops[1].focus_owner",
			mutate: func(snapshot *Snapshot) {
				owner := WindowID("w1")
				snapshot.Desktops = append(snapshot.Desktops, Desktop{
					ID: "focus", Name: "Focus", Order: 1, Purpose: DesktopPurposeFocus,
					FocusOwner: &owner, Groups: []LayoutGroup{}, Floating: []WindowID{},
				})
			},
		},
		{
			name: "Should reject duplicate node identities in a return anchor source group",
			code: "topology.node_cycle_or_duplicate",
			mutate: func(snapshot *Snapshot) {
				sourceGroup := cloneLayoutGroup(snapshot.Desktops[0].Groups[0])
				groupID := sourceGroup.ID
				window := snapshot.Windows["w1"]
				window.ReturnAnchor = &ReturnAnchor{
					DesktopID:   snapshot.Desktops[0].ID,
					GroupID:     &groupID,
					SourceGroup: &sourceGroup,
				}
				snapshot.Windows["w1"] = window
				source := window.ReturnAnchor.SourceGroup
				source.Root.Children[1].ID = source.Root.Children[0].ID
			},
		},
		{
			name: "Should reject a return anchor source revision above the JSON-safe limit",
			code: "topology.return_anchor_source_revision",
			path: `windows["w1"].return_anchor.source_revision`,
			mutate: func(snapshot *Snapshot) {
				window := snapshot.Windows["w1"]
				window.ReturnAnchor = &ReturnAnchor{
					DesktopID:      snapshot.Desktops[0].ID,
					SourceRevision: Revision(MaxWireRevision) + 1,
				}
				snapshot.Windows["w1"] = window
			},
		},
		{
			name: "Should reject a tiled return anchor without its exact source group",
			code: "topology.return_anchor_source_group_required",
			mutate: func(snapshot *Snapshot) {
				groupID := snapshot.Desktops[0].Groups[0].ID
				window := snapshot.Windows["w1"]
				window.ReturnAnchor = &ReturnAnchor{DesktopID: snapshot.Desktops[0].ID, GroupID: &groupID}
				snapshot.Windows["w1"] = window
			},
		},
		{
			name: "Should reject invalid weights in a return anchor source group",
			code: "topology.split_weight",
			path: `windows["w1"].return_anchor.source_group.root.weights[0]`,
			mutate: func(snapshot *Snapshot) {
				sourceGroup := cloneLayoutGroup(snapshot.Desktops[0].Groups[0])
				groupID := sourceGroup.ID
				window := snapshot.Windows["w1"]
				window.ReturnAnchor = &ReturnAnchor{
					DesktopID:   snapshot.Desktops[0].ID,
					GroupID:     &groupID,
					SourceGroup: &sourceGroup,
				}
				snapshot.Windows["w1"] = window
				window.ReturnAnchor.SourceGroup.Root.Weights[0] = math.NaN()
			},
		},
		{
			name: "Should reject a return anchor source group without its anchored window",
			code: "topology.return_anchor_window_membership",
			mutate: func(snapshot *Snapshot) {
				sourceGroup := cloneLayoutGroup(snapshot.Desktops[0].Groups[0])
				groupID := sourceGroup.ID
				window := snapshot.Windows["w1"]
				window.ReturnAnchor = &ReturnAnchor{
					DesktopID:   snapshot.Desktops[0].ID,
					GroupID:     &groupID,
					SourceGroup: &sourceGroup,
				}
				snapshot.Windows["w1"] = window
				replacement := WindowID("w2")
				window.ReturnAnchor.SourceGroup.Root.Children[0].WindowID = &replacement
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			snapshot := validThreeWindowSnapshot()
			test.mutate(&snapshot)
			err := ValidateSnapshot(snapshot)
			var topologyErr *TopologyError
			if !errors.As(err, &topologyErr) {
				t.Fatalf("ValidateSnapshot() error = %v, want TopologyError", err)
			}
			var matched *Diagnostic
			for _, diagnostic := range topologyErr.Diagnostics {
				if diagnostic.Code == test.code {
					matched = &diagnostic
					break
				}
			}
			if matched == nil {
				t.Fatalf("diagnostics = %+v, want %q", topologyErr.Diagnostics, test.code)
			}
			if test.path != "" && matched.Path != test.path {
				t.Fatalf("diagnostic path = %q, want %q", matched.Path, test.path)
			}
		})
	}

	t.Run("Should return diagnostics in stable window-ID order", func(t *testing.T) {
		t.Parallel()
		snapshot := validThreeWindowSnapshot()
		for _, windowID := range []WindowID{"w1", "w2", "w3"} {
			window := snapshot.Windows[windowID]
			window.App = ""
			snapshot.Windows[windowID] = window
		}
		var baseline []Diagnostic
		for iteration := range 20 {
			err := ValidateSnapshot(snapshot)
			var topologyErr *TopologyError
			if !errors.As(err, &topologyErr) {
				t.Fatalf("ValidateSnapshot() error = %v, want TopologyError", err)
			}
			if iteration == 0 {
				baseline = append([]Diagnostic(nil), topologyErr.Diagnostics...)
				continue
			}
			if !slices.Equal(topologyErr.Diagnostics, baseline) {
				t.Fatalf("diagnostics changed order: got %+v, want %+v", topologyErr.Diagnostics, baseline)
			}
		}
	})
}

func TestSharedBoundaryResize(t *testing.T) {
	t.Run("Should resize one-to-many descendants through one split boundary", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		document := validThreeWindowSnapshot()
		document.Revision = 0
		layout := LayoutDocument{
			Version:     SnapshotVersion,
			WorkspaceID: "workspace-a",
			Desktops:    document.Desktops,
			Windows:     document.Windows,
		}
		replaced, err := environment.manager.ReplaceLayout(
			t.Context(),
			ReplaceLayoutRequest{WorkspaceID: "workspace-a", ExpectedRevision: 0, Document: layout},
		)
		if err != nil {
			t.Fatalf("ReplaceLayout() error = %v", err)
		}
		result, err := environment.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: replaced.Snapshot.Revision,
				Payload:          ResizeLayoutCommand{SplitID: "root", BoundaryIndex: 0, Delta: 0.1},
			},
		)
		if err != nil {
			t.Fatalf("Execute(resize) error = %v", err)
		}
		root, exists := findNodeInSnapshot(&result.Snapshot, "root")
		if !exists {
			t.Fatal("root split missing")
		}
		if math.Abs(root.Weights[0]-0.6) > weightTolerance || math.Abs(root.Weights[1]-0.4) > weightTolerance {
			t.Fatalf("weights = %v", root.Weights)
		}
		rects := projectedRects(result.Snapshot, "desktop-default")
		if math.Abs(rects["w2"].Width-0.4) > weightTolerance || math.Abs(rects["w3"].Width-0.4) > weightTolerance {
			t.Fatalf("descendant widths = %+v", rects)
		}
		requireValidSnapshot(t, result.Snapshot)
	})
}
