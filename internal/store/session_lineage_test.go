package store

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSessionLineageNormalizeAndValidate(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize root and child lineage metadata", func(t *testing.T) {
		t.Parallel()

		root := NormalizeSessionLineage("sess-root", nil)
		if root == nil || root.RootSessionID != "sess-root" || root.ParentSessionID != "" || root.SpawnDepth != 0 {
			t.Fatalf("NormalizeSessionLineage(root) = %#v", root)
		}
		if err := ValidateSessionLineage("sess-root", root); err != nil {
			t.Fatalf("ValidateSessionLineage(root) error = %v", err)
		}

		ttl := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		child := NormalizeSessionLineage("sess-child", &SessionLineage{
			ParentSessionID: " sess-root ",
			RootSessionID:   " sess-root ",
			SpawnDepth:      1,
			SpawnRole:       " worker ",
			TTLExpiresAt:    &ttl,
			PermissionPolicy: SessionPermissionPolicy{
				Tools: []string{" agh__skill_view ", "agh__task_update", "agh__task_update"},
			},
		})
		if child.ParentSessionID != "sess-root" ||
			child.RootSessionID != "sess-root" ||
			child.SpawnRole != "worker" ||
			len(child.PermissionPolicy.Tools) != 2 ||
			child.PermissionPolicy.Tools[0] != "agh__skill_view" ||
			child.PermissionPolicy.Tools[1] != "agh__task_update" {
			t.Fatalf("NormalizeSessionLineage(child) = %#v", child)
		}
		if err := ValidateSessionLineage("sess-child", child); err != nil {
			t.Fatalf("ValidateSessionLineage(child) error = %v", err)
		}
	})
}

func TestSessionLineageValidationRejectsInvalidPolicyAndBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      string
		lineage *SessionLineage
		want    string
	}{
		{
			name: "Should reject negative budget",
			id:   "sess-root",
			lineage: &SessionLineage{
				RootSessionID: "sess-root",
				SpawnBudget:   SessionSpawnBudget{MaxChildren: -1},
			},
			want: "max_children",
		},
		{
			name: "Should reject empty permission atom",
			id:   "sess-root",
			lineage: &SessionLineage{
				RootSessionID: "sess-root",
				PermissionPolicy: SessionPermissionPolicy{
					Tools: []string{"agh__skill_view", " "},
				},
			},
			want: "empty atom",
		},
		{
			name: "Should reject invalid tool atom",
			id:   "sess-root",
			lineage: &SessionLineage{
				RootSessionID: "sess-root",
				PermissionPolicy: SessionPermissionPolicy{
					Tools: []string{"read"},
				},
			},
			want: "canonical ToolID",
		},
		{
			name: "Should reject child missing root",
			id:   "sess-child",
			lineage: &SessionLineage{
				ParentSessionID: "sess-parent",
				SpawnDepth:      1,
			},
			want: "root session id is required",
		},
		{
			name: "Should reject root depth",
			id:   "sess-root",
			lineage: &SessionLineage{
				RootSessionID: "sess-root",
				SpawnDepth:    1,
			},
			want: "root session lineage depth",
		},
		{
			name: "Should reject root mismatch",
			id:   "sess-root",
			lineage: &SessionLineage{
				RootSessionID: "sess-other",
			},
			want: "root session lineage root",
		},
		{
			name: "Should reject root autostop",
			id:   "sess-root",
			lineage: &SessionLineage{
				RootSessionID:    "sess-root",
				AutoStopOnParent: true,
			},
			want: "cannot auto-stop",
		},
		{
			name: "Should reject child zero depth",
			id:   "sess-child",
			lineage: &SessionLineage{
				ParentSessionID: "sess-parent",
				RootSessionID:   "sess-root",
			},
			want: "depth must be greater",
		},
		{
			name: "Should reject child parent self",
			id:   "sess-child",
			lineage: &SessionLineage{
				ParentSessionID: "sess-child",
				RootSessionID:   "sess-root",
				SpawnDepth:      1,
			},
			want: "parent cannot be the session itself",
		},
		{
			name: "Should reject child root self",
			id:   "sess-child",
			lineage: &SessionLineage{
				ParentSessionID: "sess-parent",
				RootSessionID:   "sess-child",
				SpawnDepth:      1,
			},
			want: "root cannot be the session itself",
		},
		{
			name: "Should reject negative max depth",
			id:   "sess-root",
			lineage: &SessionLineage{
				RootSessionID: "sess-root",
				SpawnBudget:   SessionSpawnBudget{MaxDepth: -1},
			},
			want: "max_depth",
		},
		{
			name: "Should reject negative ttl seconds",
			id:   "sess-root",
			lineage: &SessionLineage{
				RootSessionID: "sess-root",
				SpawnBudget:   SessionSpawnBudget{TTLSeconds: -1},
			},
			want: "ttl_seconds",
		},
		{
			name: "Should reject negative active workspace cap",
			id:   "sess-root",
			lineage: &SessionLineage{
				RootSessionID: "sess-root",
				SpawnBudget:   SessionSpawnBudget{MaxActivePerWorkspace: -1},
			},
			want: "max_active_per_workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSessionLineage(tt.id, tt.lineage)
			if err == nil {
				t.Fatalf("ValidateSessionLineage(%s) error = nil, want failure", tt.name)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateSessionLineage(%s) error = %v, want substring %q", tt.name, err, tt.want)
			}
		})
	}
}

func TestCloneSessionLineageReturnsDeepCopy(t *testing.T) {
	t.Parallel()

	t.Run("Should return a deep normalized copy", func(t *testing.T) {
		t.Parallel()

		ttl := time.Date(2026, 4, 26, 12, 0, 0, 0, time.FixedZone("UTC-3", -3*60*60))
		original := &SessionLineage{
			ParentSessionID: "sess-parent",
			RootSessionID:   "sess-root",
			SpawnDepth:      2,
			SpawnRole:       "worker",
			TTLExpiresAt:    &ttl,
			PermissionPolicy: SessionPermissionPolicy{
				Tools: []string{"agh__task_update", "agh__skill_view"},
			},
		}

		cloned := CloneSessionLineage(original)
		if cloned == nil {
			t.Fatal("CloneSessionLineage() = nil, want copy")
		}
		if cloned == original {
			t.Fatal("CloneSessionLineage() returned original pointer")
		}
		if cloned.TTLExpiresAt == original.TTLExpiresAt {
			t.Fatal("CloneSessionLineage() reused TTL pointer")
		}
		if !cloned.TTLExpiresAt.Equal(ttl.UTC()) {
			t.Fatalf("cloned TTL = %s, want %s", cloned.TTLExpiresAt, ttl.UTC())
		}
		if cloned.PermissionPolicy.Tools[0] != "agh__skill_view" ||
			cloned.PermissionPolicy.Tools[1] != "agh__task_update" {
			t.Fatalf("cloned tools = %#v, want sorted policy atoms", cloned.PermissionPolicy.Tools)
		}

		original.PermissionPolicy.Tools[0] = "mutated"
		if cloned.PermissionPolicy.Tools[0] != "agh__skill_view" {
			t.Fatalf("cloned tools changed after original mutation: %#v", cloned.PermissionPolicy.Tools)
		}
	})
}

func TestSessionLineageBudgetAndPolicyJSONRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("Should round trip normalized budget and policy metadata", func(t *testing.T) {
		t.Parallel()

		budget := SessionSpawnBudget{
			MaxChildren:           2,
			MaxDepth:              3,
			TTLSeconds:            3600,
			MaxActivePerWorkspace: 4,
		}
		rawBudget, err := EncodeSessionSpawnBudget(budget)
		if err != nil {
			t.Fatalf("EncodeSessionSpawnBudget() error = %v", err)
		}
		decodedBudget, err := DecodeSessionSpawnBudget(rawBudget)
		if err != nil {
			t.Fatalf("DecodeSessionSpawnBudget() error = %v", err)
		}
		if decodedBudget != budget {
			t.Fatalf("decoded budget = %#v, want %#v", decodedBudget, budget)
		}
		emptyBudget, err := DecodeSessionSpawnBudget("  ")
		if err != nil {
			t.Fatalf("DecodeSessionSpawnBudget(empty) error = %v", err)
		}
		if emptyBudget != (SessionSpawnBudget{}) {
			t.Fatalf("DecodeSessionSpawnBudget(empty) = %#v, want zero budget", emptyBudget)
		}

		policy := SessionPermissionPolicy{
			Tools:           []string{"agh__task_update", " agh__skill_view ", "agh__skill_view"},
			Skills:          []string{"go"},
			MCPServers:      []string{"memory"},
			WorkspacePaths:  []string{"/repo"},
			NetworkChannels: []string{"coord"},
			SandboxProfiles: []string{"local"},
		}
		rawPolicy, err := EncodeSessionPermissionPolicy(policy)
		if err != nil {
			t.Fatalf("EncodeSessionPermissionPolicy() error = %v", err)
		}
		decodedPolicy, err := DecodeSessionPermissionPolicy(rawPolicy)
		if err != nil {
			t.Fatalf("DecodeSessionPermissionPolicy() error = %v", err)
		}
		wantPolicy := SessionPermissionPolicy{
			Tools:           []string{"agh__skill_view", "agh__task_update"},
			Skills:          []string{"go"},
			MCPServers:      []string{"memory"},
			WorkspacePaths:  []string{"/repo"},
			NetworkChannels: []string{"coord"},
			SandboxProfiles: []string{"local"},
		}
		if !reflect.DeepEqual(decodedPolicy, wantPolicy) {
			t.Fatalf("decoded policy = %#v, want %#v", decodedPolicy, wantPolicy)
		}
		emptyPolicy, err := DecodeSessionPermissionPolicy("")
		if err != nil {
			t.Fatalf("DecodeSessionPermissionPolicy(empty) error = %v", err)
		}
		if !reflect.DeepEqual(emptyPolicy, NormalizeSessionPermissionPolicy(SessionPermissionPolicy{})) {
			t.Fatalf("DecodeSessionPermissionPolicy(empty) = %#v, want normalized empty policy", emptyPolicy)
		}
	})
}

func TestSessionLineageJSONRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "Should reject encoding negative budget",
			run: func() error {
				_, err := EncodeSessionSpawnBudget(SessionSpawnBudget{MaxChildren: -1})
				return err
			},
			want: "max_children",
		},
		{
			name: "Should reject decoding malformed budget",
			run: func() error {
				_, err := DecodeSessionSpawnBudget("{")
				return err
			},
			want: "parse session spawn budget",
		},
		{
			name: "Should reject decoding negative budget",
			run: func() error {
				_, err := DecodeSessionSpawnBudget(`{"ttl_seconds":-1}`)
				return err
			},
			want: "ttl_seconds",
		},
		{
			name: "Should reject encoding empty policy atom",
			run: func() error {
				_, err := EncodeSessionPermissionPolicy(SessionPermissionPolicy{Skills: []string{"go", " "}})
				return err
			},
			want: "empty atom",
		},
		{
			name: "Should reject decoding malformed policy",
			run: func() error {
				_, err := DecodeSessionPermissionPolicy("{")
				return err
			},
			want: "parse session permission policy",
		},
		{
			name: "Should reject decoding empty policy atom",
			run: func() error {
				_, err := DecodeSessionPermissionPolicy(`{"network_channels":["coord"," "]}`)
				return err
			},
			want: "empty atom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if err == nil {
				t.Fatalf("%s error = nil, want failure", tt.name)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("%s error = %v, want substring %q", tt.name, err, tt.want)
			}
		})
	}
}

func TestValidateChildSessionToolSubset(t *testing.T) {
	t.Parallel()

	t.Run("Should accept child concrete tools within parent policy", func(t *testing.T) {
		t.Parallel()

		parent := SessionPermissionPolicy{
			Tools: []string{"agh__skill_view", "agh__task_read", "agh__task_update"},
		}
		child := SessionPermissionPolicy{
			Tools: []string{" agh__task_read ", "agh__skill_view"},
		}
		if err := ValidateChildSessionToolSubset(parent, child); err != nil {
			t.Fatalf("ValidateChildSessionToolSubset() error = %v", err)
		}
	})

	t.Run("Should reject child concrete tools outside parent policy", func(t *testing.T) {
		t.Parallel()

		parent := SessionPermissionPolicy{
			Tools: []string{"agh__skill_view"},
		}
		child := SessionPermissionPolicy{
			Tools: []string{"agh__skill_view", "agh__task_update"},
		}
		err := ValidateChildSessionToolSubset(parent, child)
		if err == nil {
			t.Fatal("ValidateChildSessionToolSubset() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), "exceeds parent") {
			t.Fatalf("ValidateChildSessionToolSubset() error = %v, want exceeds parent", err)
		}
	})

	t.Run("Should reject invalid concrete tool atoms before subset comparison", func(t *testing.T) {
		t.Parallel()

		parent := SessionPermissionPolicy{
			Tools: []string{"agh__skill_view"},
		}
		child := SessionPermissionPolicy{
			Tools: []string{"read"},
		}
		err := ValidateChildSessionToolSubset(parent, child)
		if err == nil {
			t.Fatal("ValidateChildSessionToolSubset() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), "canonical ToolID") {
			t.Fatalf("ValidateChildSessionToolSubset() error = %v, want canonical ToolID", err)
		}
	})
}
