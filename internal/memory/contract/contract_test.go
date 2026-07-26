package contract

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
)

func TestEnumNormalization(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize and validate scope values", func(t *testing.T) {
		t.Parallel()

		if got := Scope(" Agent ").Normalize(); got != ScopeAgent {
			t.Fatalf("Scope.Normalize() = %q, want %q", got, ScopeAgent)
		}
		for _, scope := range []Scope{ScopeGlobal, ScopeWorkspace, ScopeAgent} {
			if err := scope.Validate(); err != nil {
				t.Fatalf("Scope(%q).Validate() error = %v", scope, err)
			}
		}
		if err := Scope("sideways").Validate(); err == nil {
			t.Fatal("Scope(sideways).Validate() error = nil, want validation error")
		}
		if err := Scope("").Validate(); err == nil {
			t.Fatal("Scope(empty).Validate() error = nil, want validation error")
		}
	})

	t.Run("Should normalize and validate agent tier values", func(t *testing.T) {
		t.Parallel()

		if got := AgentTier(" WORKSPACE ").Normalize(); got != AgentTierWorkspace {
			t.Fatalf("AgentTier.Normalize() = %q, want %q", got, AgentTierWorkspace)
		}
		for _, tier := range []AgentTier{AgentTierWorkspace, AgentTierGlobal} {
			if err := tier.Validate(); err != nil {
				t.Fatalf("AgentTier(%q).Validate() error = %v", tier, err)
			}
		}
		if err := AgentTier("team").Validate(); err == nil {
			t.Fatal("AgentTier(team).Validate() error = nil, want validation error")
		}
		if err := AgentTier("").Validate(); err == nil {
			t.Fatal("AgentTier(empty).Validate() error = nil, want validation error")
		}
	})

	t.Run("Should normalize and validate origin values", func(t *testing.T) {
		t.Parallel()

		if got := Origin(" Provider ").Normalize(); got != OriginProvider {
			t.Fatalf("Origin.Normalize() = %q, want %q", got, OriginProvider)
		}
		for _, origin := range []Origin{
			OriginCLI,
			OriginHTTP,
			OriginUDS,
			OriginTool,
			OriginExtractor,
			OriginDreaming,
			OriginFile,
			OriginProvider,
		} {
			if err := origin.Validate(); err != nil {
				t.Fatalf("Origin(%q).Validate() error = %v", origin, err)
			}
		}
		if err := Origin("unknown").Validate(); err == nil {
			t.Fatal("Origin(unknown).Validate() error = nil, want validation error")
		}
		if err := Origin("").Validate(); err == nil {
			t.Fatal("Origin(empty).Validate() error = nil, want validation error")
		}
	})

	t.Run("Should normalize and validate memory type defaults", func(t *testing.T) {
		t.Parallel()

		if got := Type(" Project ").Normalize(); got != TypeProject {
			t.Fatalf("Type.Normalize() = %q, want %q", got, TypeProject)
		}
		for _, typ := range []Type{TypeUser, TypeFeedback, TypeProject, TypeReference} {
			if err := typ.Validate(); err != nil {
				t.Fatalf("Type(%q).Validate() error = %v", typ, err)
			}
		}
		for _, typ := range []Type{TypeUser, TypeFeedback} {
			scope, err := DefaultScopeForType(typ)
			if err != nil {
				t.Fatalf("DefaultScopeForType(%q) error = %v", typ, err)
			}
			if scope != ScopeGlobal {
				t.Fatalf("DefaultScopeForType(%q) = %q, want %q", typ, scope, ScopeGlobal)
			}
		}
		for _, typ := range []Type{TypeProject, TypeReference} {
			scope, err := DefaultScopeForType(typ)
			if err != nil {
				t.Fatalf("DefaultScopeForType(%q) error = %v", typ, err)
			}
			if scope != ScopeWorkspace {
				t.Fatalf("DefaultScopeForType(%q) = %q, want %q", typ, scope, ScopeWorkspace)
			}
		}
		if _, err := DefaultScopeForType(Type("unknown")); err == nil {
			t.Fatal("DefaultScopeForType(unknown) error = nil, want validation error")
		}
		if _, err := DefaultScopeForType(Type("")); err == nil {
			t.Fatal("DefaultScopeForType(empty) error = nil, want validation error")
		}
		if err := Type("").Validate(); err == nil {
			t.Fatal("Type(empty).Validate() error = nil, want validation error")
		}
	})

	t.Run("Should normalize operation and write decision values", func(t *testing.T) {
		t.Parallel()

		if got := Operation(" Memory.Write ").Normalize(); got != OperationWrite {
			t.Fatalf("Operation.Normalize() = %q, want %q", got, OperationWrite)
		}
		if got := OpUpdate.String(); got != "update" {
			t.Fatalf("OpUpdate.String() = %q, want update", got)
		}
		if got := Op(99).String(); got != "" {
			t.Fatalf("Op(99).String() = %q, want empty string", got)
		}
		if got := OpAdd.Normalize(); got != OpAdd {
			t.Fatalf("OpAdd.Normalize() = %v, want %v", got, OpAdd)
		}
		if err := OpDelete.Validate(); err != nil {
			t.Fatalf("OpDelete.Validate() error = %v", err)
		}
		if err := Op(99).Validate(); err == nil {
			t.Fatal("Op(99).Validate() error = nil, want validation error")
		}
		if _, err := json.Marshal(Op(99)); err == nil {
			t.Fatal("json.Marshal(unsupported Op) error = nil, want validation error")
		}
		payload, err := json.Marshal(OpAdd)
		if err != nil {
			t.Fatalf("json.Marshal(OpAdd) error = %v", err)
		}
		if string(payload) != `"add"` {
			t.Fatalf("json.Marshal(OpAdd) = %s, want add string", payload)
		}
		var decoded Op
		if err := json.Unmarshal([]byte(`"DELETE"`), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(OpDelete) error = %v", err)
		}
		if decoded != OpDelete {
			t.Fatalf("decoded Op = %v, want %v", decoded, OpDelete)
		}
		if err := json.Unmarshal([]byte(`"sideways"`), &decoded); err == nil {
			t.Fatal("json.Unmarshal(unsupported Op) error = nil, want validation error")
		}
		if err := json.Unmarshal([]byte(`""`), &decoded); err == nil {
			t.Fatal("json.Unmarshal(empty Op) error = nil, want validation error")
		}
		if err := json.Unmarshal([]byte(`12`), &decoded); err == nil {
			t.Fatal("json.Unmarshal(non-string Op) error = nil, want decode error")
		}
	})

	t.Run("Should normalize and validate decision sources and triggers", func(t *testing.T) {
		t.Parallel()

		if got := DecisionSource(" LLM ").Normalize(); got != SourceLLM {
			t.Fatalf("DecisionSource.Normalize() = %q, want %q", got, SourceLLM)
		}
		for _, source := range []DecisionSource{SourceRule, SourceLLM} {
			if err := source.Validate(); err != nil {
				t.Fatalf("DecisionSource(%q).Validate() error = %v", source, err)
			}
		}
		if err := DecisionSource("sideways").Validate(); err == nil {
			t.Fatal("DecisionSource(sideways).Validate() error = nil, want validation error")
		}
		if err := DecisionSource("").Validate(); err == nil {
			t.Fatal("DecisionSource(empty).Validate() error = nil, want validation error")
		}
		if got := Trigger(" Compaction_Flush ").Normalize(); got != TriggerCompactionFlush {
			t.Fatalf("Trigger.Normalize() = %q, want %q", got, TriggerCompactionFlush)
		}
		for _, trigger := range []Trigger{TriggerPostMessage, TriggerCompactionFlush} {
			if err := trigger.Validate(); err != nil {
				t.Fatalf("Trigger(%q).Validate() error = %v", trigger, err)
			}
		}
		if err := Trigger("manual").Validate(); err == nil {
			t.Fatal("Trigger(manual).Validate() error = nil, want validation error")
		}
		if err := Trigger("").Validate(); err == nil {
			t.Fatal("Trigger(empty).Validate() error = nil, want validation error")
		}
	})
}

func TestHeaderSerialization(t *testing.T) {
	t.Parallel()

	t.Run("Should use canonical YAML agent field and JSON agent name", func(t *testing.T) {
		t.Parallel()

		var header Header
		raw := []byte(
			"name: Prefs\ndescription: User preferences\ntype: user\nscope: agent\nagent: codex\nagent_tier: workspace\n",
		)
		if err := yaml.Unmarshal(raw, &header); err != nil {
			t.Fatalf("yaml.Unmarshal(Header) error = %v", err)
		}
		if err := header.Validate(); err != nil {
			t.Fatalf("Header.Validate() error = %v", err)
		}
		if header.AgentName != "codex" {
			t.Fatalf("Header.AgentName = %q, want codex", header.AgentName)
		}

		payload, err := json.Marshal(header)
		if err != nil {
			t.Fatalf("json.Marshal(Header) error = %v", err)
		}
		for _, forbidden := range []string{"legacy", "agent_name\":\"\"", "provenance"} {
			if strings.Contains(strings.ToLower(string(payload)), forbidden) {
				t.Fatalf("Header JSON contains deprecated field marker %q: %s", forbidden, payload)
			}
		}
		if !strings.Contains(string(payload), `"agent_name":"codex"`) {
			t.Fatalf("Header JSON = %s, want normalized agent_name", payload)
		}

		yamlPayload, err := yaml.Marshal(header)
		if err != nil {
			t.Fatalf("yaml.Marshal(Header) error = %v", err)
		}
		if strings.Contains(string(yamlPayload), "agent_name:") {
			t.Fatalf("Header YAML contains deprecated agent_name field: %s", yamlPayload)
		}
		if !strings.Contains(string(yamlPayload), "agent: codex") {
			t.Fatalf("Header YAML = %s, want canonical agent field", yamlPayload)
		}
	})

	t.Run("Should reject invalid header metadata", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			header Header
		}{
			{name: "Should reject missing name", header: Header{Type: TypeUser}},
			{name: "Should reject missing type", header: Header{Name: "Missing type"}},
			{name: "Should reject bad type", header: Header{Name: "Bad type", Type: Type("sideways")}},
			{
				name:   "Should reject bad scope",
				header: Header{Name: "Bad scope", Type: TypeUser, Scope: Scope("sideways")},
			},
			{
				name:   "Should reject bad tier",
				header: Header{Name: "Bad tier", Type: TypeUser, AgentTier: AgentTier("sideways")},
			},
			{
				name:   "Should reject agent scope without agent name",
				header: Header{Name: "Missing agent", Type: TypeUser, Scope: ScopeAgent, AgentTier: AgentTierWorkspace},
			},
			{
				name:   "Should reject agent scope without agent tier",
				header: Header{Name: "Missing tier", Type: TypeUser, Scope: ScopeAgent, AgentName: "codex"},
			},
			{
				name:   "Should reject agent name outside agent scope",
				header: Header{Name: "Wrong agent metadata", Type: TypeUser, Scope: ScopeWorkspace, AgentName: "codex"},
			},
			{
				name: "Should reject agent tier outside agent scope",
				header: Header{
					Name:      "Wrong tier metadata",
					Type:      TypeUser,
					Scope:     ScopeGlobal,
					AgentTier: AgentTierWorkspace,
				},
			},
			{
				name: "Should reject invalid provenance source actor",
				header: Header{
					Name:       "Bad provenance",
					Type:       TypeUser,
					Provenance: &Provenance{SourceActor: Origin("sideways")},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				if err := tt.header.Validate(); err == nil {
					t.Fatalf("Header(%#v).Validate() error = nil, want validation error", tt.header)
				}
			})
		}
	})

	t.Run("Should normalize and validate provenance source actor", func(t *testing.T) {
		t.Parallel()

		header := Header{
			Name: "Prefs",
			Type: TypeUser,
			Provenance: &Provenance{
				SourceActor:      Origin(" Tool "),
				Confidence:       " high ",
				SourceSessionIDs: []string{" session-1 "},
			},
		}
		if err := header.Validate(); err != nil {
			t.Fatalf("Header.Validate() error = %v", err)
		}
		if header.Provenance.SourceActor != OriginTool {
			t.Fatalf("Provenance.SourceActor = %q, want %q", header.Provenance.SourceActor, OriginTool)
		}
		if header.Provenance.Confidence != "high" {
			t.Fatalf("Provenance.Confidence = %q, want high", header.Provenance.Confidence)
		}
		if got := header.Provenance.SourceSessionIDs[0]; got != "session-1" {
			t.Fatalf("Provenance.SourceSessionIDs[0] = %q, want session-1", got)
		}
	})
}

func TestDTOJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should round trip write records without speculative vector fields", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		record := WriteRecord{
			Candidate: Candidate{
				WorkspaceID: "workspace-1",
				Scope:       ScopeAgent,
				AgentName:   "codex",
				AgentTier:   AgentTierWorkspace,
				Origin:      OriginTool,
				Content:     "Keep summaries concise.",
				Frontmatter: Header{
					Name:        "Style",
					Description: "Style preference",
					Type:        TypeUser,
					Scope:       ScopeAgent,
					AgentName:   "codex",
					AgentTier:   AgentTierWorkspace,
				},
				Entity:      "style",
				Attribute:   "summary",
				Metadata:    map[string]string{"source": "test"},
				SubmittedAt: now,
			},
			Decision: Decision{
				ID:              "decision-1",
				CandidateHash:   "hash-1",
				IdempotencyKey:  "key-1",
				Op:              OpUpdate,
				TargetFilename:  "style.md",
				Frontmatter:     Header{Name: "Style", Type: TypeUser, Scope: ScopeAgent, AgentName: "codex"},
				PostContent:     "Keep summaries concise.",
				PostContentHash: "hash-2",
				Confidence:      0.93,
				Source:          SourceRule,
				RuleTrace:       []RuleHit{{Name: "dedupe", Passed: true}},
				DecidedAt:       now,
			},
		}

		payload, err := json.Marshal(record)
		if err != nil {
			t.Fatalf("json.Marshal(WriteRecord) error = %v", err)
		}
		lowered := strings.ToLower(string(payload))
		for _, forbidden := range []string{
			"embedding",
			"vector",
			"context_ref",
			"resolved_context",
			"token_budget",
			"provider_hook",
		} {
			if strings.Contains(lowered, forbidden) {
				t.Fatalf("WriteRecord JSON contains deprecated field %q: %s", forbidden, payload)
			}
		}
		if !strings.Contains(string(payload), `"op":"update"`) {
			t.Fatalf("WriteRecord JSON = %s, want op string", payload)
		}

		var decoded WriteRecord
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(WriteRecord) error = %v", err)
		}
		if decoded.Decision.Op != OpUpdate {
			t.Fatalf("decoded.Decision.Op = %v, want %v", decoded.Decision.Op, OpUpdate)
		}
		if decoded.Candidate.Scope != ScopeAgent || decoded.Candidate.AgentTier != AgentTierWorkspace {
			t.Fatalf(
				"decoded scope tuple = %q/%q, want agent/workspace",
				decoded.Candidate.Scope,
				decoded.Candidate.AgentTier,
			)
		}
	})
}

func TestProviderInterfaces(t *testing.T) {
	t.Parallel()

	t.Run("Should compile against provider facing interfaces", func(t *testing.T) {
		t.Parallel()

		var _ MemoryProvider = (*providerStub)(nil)
		var _ Controller = controllerStub{}
		var _ Recaller = recallerStub{}
		var _ Extractor = extractorStub{}
	})
}

func TestImportBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Should keep contract below memory runtime packages", func(t *testing.T) {
		t.Parallel()

		repoRoot := findRepoRoot(t)
		cmd := exec.CommandContext(
			t.Context(),
			"go",
			"list",
			"-f",
			"{{join .Imports \"\\n\"}}",
			"github.com/compozy/agh/internal/memory/contract",
		)
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go list contract imports error = %v\n%s", err, output)
		}

		forbiddenPrefixes := []string{
			"github.com/compozy/agh/internal/memory/",
			"github.com/compozy/agh/internal/api",
			"github.com/compozy/agh/internal/cli",
			"github.com/compozy/agh/internal/daemon",
			"github.com/compozy/agh/internal/extension",
		}
		for imported := range strings.FieldsSeq(string(output)) {
			for _, forbidden := range forbiddenPrefixes {
				if strings.HasPrefix(imported, forbidden) {
					t.Fatalf("contract imports forbidden package %q via %q", forbidden, imported)
				}
			}
		}
	})
}

type providerStub struct{}

func (providerStub) Initialize(context.Context, ProviderInit) error {
	return nil
}

func (providerStub) SystemPromptBlock(context.Context, SnapshotRequest) (SnapshotResult, error) {
	return SnapshotResult{}, nil
}

func (providerStub) Recall(context.Context, RecallRequest) (RecallResult, error) {
	return RecallResult{}, nil
}

func (providerStub) Prefetch(context.Context, PrefetchRequest) error {
	return nil
}

func (providerStub) SyncTurn(context.Context, TurnRecord) error {
	return nil
}

func (providerStub) OnSessionEnd(context.Context, SessionEndRecord) error {
	return nil
}

func (providerStub) OnSessionSwitch(context.Context, SessionSwitchRecord) error {
	return nil
}

func (providerStub) OnPreCompress(context.Context, PreCompressRequest) (PreCompressHint, error) {
	return PreCompressHint{}, nil
}

func (providerStub) OnMemoryWrite(context.Context, WriteRecord) error {
	return nil
}

func (providerStub) Shutdown(context.Context) error {
	return nil
}

type controllerStub struct{}

func (controllerStub) Decide(context.Context, Candidate) (Decision, error) {
	return Decision{}, nil
}

type recallerStub struct{}

func (recallerStub) Recall(context.Context, Query, RecallOptions) (Packaged, error) {
	return Packaged{}, nil
}

type extractorStub struct{}

func (extractorStub) Extract(context.Context, TurnRecord) ([]Candidate, error) {
	return nil, nil
}

func (extractorStub) Drain(context.Context) error {
	return nil
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("go.mod not found while resolving repo root")
		}
		dir = next
	}
}
