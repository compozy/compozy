package soul

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestSoulPersistenceSnapshotHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should build a snapshot envelope from resolved Soul output", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.DefaultSoulConfig()
		resolved, err := Parse(context.Background(), ParseRequest{
			SourcePath:    "/workspace/agents/coder/" + FileName,
			WorkspaceRoot: "/workspace",
			Config:        cfg,
			Content: []byte(`---
version: "1"
role: coder
tone:
  - concise
---
Keep context tight.
`),
		})
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		provenance, err := NewConfigProvenance(cfg, "workspace")
		if err != nil {
			t.Fatalf("NewConfigProvenance() error = %v", err)
		}
		createdAt := time.Date(2026, 5, 2, 9, 30, 0, 0, time.UTC)
		snapshot, err := SnapshotFromResolved("snap-1", "ws-1", "coder", &resolved, provenance, createdAt)
		if err != nil {
			t.Fatalf("SnapshotFromResolved() error = %v", err)
		}

		if snapshot.SourcePath != "agents/coder/SOUL.md" || snapshot.Digest != resolved.Digest {
			t.Fatalf("snapshot source/digest = %q/%q, want resolver values", snapshot.SourcePath, snapshot.Digest)
		}
		if snapshot.Body != resolved.ReadModel.Body || !snapshot.CreatedAt.Equal(createdAt) {
			t.Fatalf("snapshot body/time = %q/%s, want resolver body and timestamp", snapshot.Body, snapshot.CreatedAt)
		}
		var profile SnapshotProfile
		if err := json.Unmarshal(snapshot.ProfileJSON, &profile); err != nil {
			t.Fatalf("Unmarshal(ProfileJSON) error = %v", err)
		}
		if profile.SchemaVersion != snapshotProfileSchemaVersion ||
			!profile.Valid ||
			profile.Profile.Role != "coder" ||
			profile.ReadModel.Body != resolved.ReadModel.Body ||
			profile.ConfigProvenance.Digest == "" {
			t.Fatalf("SnapshotProfile = %#v, want full resolver read model and config provenance", profile)
		}
		if strings.Contains(string(snapshot.ProfileJSON), "/workspace") {
			t.Fatalf("ProfileJSON contains absolute workspace path: %s", string(snapshot.ProfileJSON))
		}
	})

	t.Run("Should clone diagnostics for revision JSON and snapshot envelopes", func(t *testing.T) {
		t.Parallel()

		diagnostics := []Diagnostic{{
			Code:       "reserved_section",
			Message:    "SOUL.md section belongs to AGENT.md",
			SourcePath: "agents/coder/SOUL.md",
		}}
		encoded, err := DiagnosticsJSON(diagnostics)
		if err != nil {
			t.Fatalf("DiagnosticsJSON() error = %v", err)
		}
		diagnostics[0].Code = "mutated"

		var decoded []Diagnostic
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("Unmarshal(DiagnosticsJSON) error = %v", err)
		}
		if len(decoded) != 1 || decoded[0].Code != "reserved_section" {
			t.Fatalf("decoded diagnostics = %#v, want cloned original diagnostic", decoded)
		}
	})
}

func TestSoulPersistenceValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should validate snapshot rows and query limits", func(t *testing.T) {
		t.Parallel()

		valid := Snapshot{
			ID:          "snap-1",
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SourcePath:  "agents/coder/SOUL.md",
			Digest:      "sha256:one",
			ProfileJSON: validSnapshotProfileJSON(t),
			CreatedAt:   time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC),
		}
		if err := valid.Validate(); err != nil {
			t.Fatalf("Snapshot.Validate(valid) error = %v", err)
		}
		for _, tt := range []struct {
			name   string
			mutate func(*Snapshot)
		}{
			{name: "Should reject missing id", mutate: func(snapshot *Snapshot) { snapshot.ID = "" }},
			{name: "Should reject missing workspace id", mutate: func(snapshot *Snapshot) { snapshot.WorkspaceID = "" }},
			{name: "Should reject missing agent name", mutate: func(snapshot *Snapshot) { snapshot.AgentName = "" }},
			{name: "Should reject missing source path", mutate: func(snapshot *Snapshot) { snapshot.SourcePath = "" }},
			{name: "Should reject missing digest", mutate: func(snapshot *Snapshot) { snapshot.Digest = "" }},
			{name: "Should reject malformed profile JSON", mutate: func(snapshot *Snapshot) {
				snapshot.ProfileJSON = json.RawMessage(`{`)
			}},
			{name: "Should reject profile JSON without the snapshot schema", mutate: func(snapshot *Snapshot) {
				snapshot.ProfileJSON = json.RawMessage(`{"valid":true}`)
			}},
			{name: "Should reject unsupported profile schema versions", mutate: func(snapshot *Snapshot) {
				snapshot.ProfileJSON = validSnapshotProfileJSON(t, 2)
			}},
			{name: "Should reject missing created timestamp", mutate: func(snapshot *Snapshot) {
				snapshot.CreatedAt = time.Time{}
			}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				candidate := valid
				tt.mutate(&candidate)
				if err := candidate.Validate(); !errors.Is(err, ErrInvalidSnapshot) {
					t.Fatalf("Snapshot.Validate() error = %v, want ErrInvalidSnapshot", err)
				}
			})
		}
		if err := (SnapshotListQuery{Limit: -1}).Validate(); err == nil {
			t.Fatal("SnapshotListQuery.Validate(negative limit) error = nil, want non-nil")
		}
	})

	t.Run("Should validate revision rows list filters and rollback lookup", func(t *testing.T) {
		t.Parallel()

		valid := Revision{
			ID:              "rev-1",
			WorkspaceID:     "ws-1",
			AgentName:       "coder",
			SourcePath:      "agents/coder/SOUL.md",
			Action:          RevisionActionPut,
			NewDigest:       "sha256:one",
			DiagnosticsJSON: json.RawMessage(`[]`),
			CreatedAt:       time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC),
		}
		if err := valid.Validate(); err != nil {
			t.Fatalf("Revision.Validate(valid) error = %v", err)
		}
		deleteRevision := valid
		deleteRevision.Action = RevisionActionDelete
		deleteRevision.NewDigest = ""
		if err := deleteRevision.Validate(); err != nil {
			t.Fatalf("Revision.Validate(delete) error = %v", err)
		}
		for _, tt := range []struct {
			name   string
			mutate func(*Revision)
		}{
			{name: "Should reject missing id", mutate: func(revision *Revision) { revision.ID = "" }},
			{name: "Should reject missing workspace id", mutate: func(revision *Revision) { revision.WorkspaceID = "" }},
			{name: "Should reject missing agent name", mutate: func(revision *Revision) { revision.AgentName = "" }},
			{name: "Should reject missing source path", mutate: func(revision *Revision) { revision.SourcePath = "" }},
			{name: "Should reject unsupported action", mutate: func(revision *Revision) {
				revision.Action = RevisionAction("rewrite")
			}},
			{name: "Should reject delete with new digest", mutate: func(revision *Revision) {
				revision.Action = RevisionActionDelete
				revision.NewDigest = "sha256:delete"
			}},
			{name: "Should reject put without new digest", mutate: func(revision *Revision) { revision.NewDigest = "" }},
			{name: "Should reject malformed diagnostics JSON", mutate: func(revision *Revision) {
				revision.DiagnosticsJSON = json.RawMessage(`{`)
			}},
			{name: "Should reject diagnostics JSON that is not an array", mutate: func(revision *Revision) {
				revision.DiagnosticsJSON = json.RawMessage(`{}`)
			}},
			{name: "Should reject missing created timestamp", mutate: func(revision *Revision) {
				revision.CreatedAt = time.Time{}
			}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				candidate := valid
				tt.mutate(&candidate)
				if err := candidate.Validate(); !errors.Is(err, ErrInvalidRevision) {
					t.Fatalf("Revision.Validate() error = %v, want ErrInvalidRevision", err)
				}
			})
		}
		if err := (RevisionListQuery{Limit: -1}).Validate(); err == nil {
			t.Fatal("RevisionListQuery.Validate(negative limit) error = nil, want non-nil")
		}
		if err := (RevisionListQuery{Action: RevisionAction("rewrite")}).Validate(); err == nil {
			t.Fatal("RevisionListQuery.Validate(invalid action) error = nil, want non-nil")
		}
		for _, tt := range []struct {
			name  string
			query RollbackLookup
		}{
			{name: "Should reject missing workspace id", query: RollbackLookup{AgentName: "coder", RevisionID: "rev-1"}},
			{name: "Should reject missing agent name", query: RollbackLookup{WorkspaceID: "ws-1", RevisionID: "rev-1"}},
			{name: "Should reject missing revision id", query: RollbackLookup{WorkspaceID: "ws-1", AgentName: "coder"}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				if err := tt.query.Validate(); err == nil {
					t.Fatal("RollbackLookup.Validate() error = nil, want non-nil")
				}
			})
		}
	})
}

func validSnapshotProfileJSON(t *testing.T, versions ...int) json.RawMessage {
	t.Helper()

	schemaVersion := snapshotProfileSchemaVersion
	if len(versions) > 0 {
		schemaVersion = versions[0]
	}
	encoded, err := json.Marshal(SnapshotProfile{
		SchemaVersion: schemaVersion,
		Present:       true,
		Active:        true,
		Valid:         true,
		ConfigProvenance: ConfigProvenance{
			Digest:                 "sha256:config",
			Enabled:                true,
			MaxBodyBytes:           32768,
			ContextProjectionBytes: 2048,
		},
	})
	if err != nil {
		t.Fatalf("Marshal(SnapshotProfile version %d) error = %v", schemaVersion, err)
	}
	return json.RawMessage(encoded)
}
