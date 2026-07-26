package config

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestMemoryV2ConfigDefaultsAndOverlay(t *testing.T) {
	t.Run("Should expose approved Slice 1 defaults", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		memory := cfg.Memory

		if !memory.Enabled || memory.GlobalDir != homePaths.MemoryDir {
			t.Fatalf("DefaultWithHome() Memory = %#v, want enabled global memory dir", memory)
		}
		if memory.Controller.Mode != "hybrid" ||
			memory.Controller.MaxLatency != 300*time.Millisecond ||
			memory.Controller.DefaultOpOnFail != "noop" {
			t.Fatalf("DefaultWithHome() Controller = %#v", memory.Controller)
		}
		if !slices.Equal(memory.Controller.Policy.AllowOrigins, []string{
			"cli",
			"http",
			"uds",
			"tool",
			"extractor",
			"dreaming",
			"file",
			"provider",
		}) {
			t.Fatalf("DefaultWithHome() Controller.Policy.AllowOrigins = %#v", memory.Controller.Policy.AllowOrigins)
		}
		if memory.Recall.TopK != 5 ||
			memory.Recall.RawCandidates != 50 ||
			memory.Recall.Fusion != "weighted" ||
			memory.Recall.Weights.BM25Unicode != 0.55 ||
			memory.Recall.Freshness.BannerAfterDays != 1 ||
			memory.Recall.Signals.QueueCapacity != 256 {
			t.Fatalf("DefaultWithHome() Recall = %#v", memory.Recall)
		}
		if memory.Decisions.PruneAfterAppliedDays != 90 ||
			!memory.Decisions.KeepAuditSummary ||
			memory.Decisions.MaxPostContentBytes != 65536 {
			t.Fatalf("DefaultWithHome() Decisions = %#v", memory.Decisions)
		}
		if memory.Extractor.Mode != configExtractorModePostMessage ||
			memory.Extractor.Deadline != time.Minute ||
			memory.Extractor.Queue.Capacity != 1 ||
			memory.Extractor.Queue.CoalesceMax != 16 {
			t.Fatalf("DefaultWithHome() Extractor = %#v", memory.Extractor)
		}
		if memory.Dream.Debounce != 10*time.Minute ||
			memory.Dream.PromptVersion != "v1" ||
			memory.Dream.Gates.MinScore != 0.75 ||
			memory.Dream.Scoring.RecencyHalfLifeDays != 14 {
			t.Fatalf("DefaultWithHome() Dream = %#v", memory.Dream)
		}
		if memory.Session.LedgerFormat != "jsonl" ||
			memory.Session.LedgerRoot != homePaths.SessionsDir ||
			memory.Session.EventsPurgeGrace != 24*time.Hour ||
			memory.Session.UnboundPartition != "_unbound" {
			t.Fatalf("DefaultWithHome() Session = %#v", memory.Session)
		}
		if memory.Daily.RotateFormat != "{date}.{seq}.md" ||
			memory.Daily.SweepHour != 3 ||
			memory.File.MaxLines != 200 ||
			memory.Provider.Timeout != 2*time.Second ||
			memory.Workspace.TOMLPath != defaultMemoryWorkspaceTOMLPath ||
			!memory.Workspace.AutoCreate {
			t.Fatalf("DefaultWithHome() tail config = %#v", memory)
		}
	})

	// not parallel: this subtest uses t.Setenv for AGH_HOME overlay resolution.
	t.Run("Should merge every Memory v2 backend section from overlays", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		homeRoot := filepath.Join(t.TempDir(), "home")
		t.Setenv("AGH_HOME", homeRoot)

		homePaths, err := ResolveHomePaths()
		if err != nil {
			t.Fatalf("ResolveHomePaths() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, `
[memory.controller]
mode = "rules"
max_latency = "150ms"
default_op_on_fail = "reject"

[memory.controller.policy]
max_content_chars = 2048
max_writes_per_min = 30
allow_origins = ["cli", "tool"]

[memory.recall]
top_k = 3
raw_candidates = 30
fusion = "rrf"
include_already_surfaced = true
include_system = true

[memory.recall.weights]
bm25_unicode = 0.40
bm25_trigram = 0.30
recency = 0.20
recall_signal = 0.10

[memory.recall.freshness]
banner_after_days = 2

[memory.recall.signals]
queue_capacity = 128
worker_retry_max = 5
metrics_enabled = false

[memory.decisions]
prune_after_applied_days = 30
keep_audit_summary = false
max_post_content_bytes = 32768

[memory.extractor]
mode = "post_message"
throttle_turns = 2
deadline = "45s"
sandbox_inbox_only = false
inbox_path = "~/agh-inbox"
dlq_path = "~/agh-dlq"

[memory.extractor.queue]
capacity = 2
coalesce_max = 8

[memory.dream]
min_hours = 12
min_sessions = 4
debounce = "5m"
prompt_version = "v2"
check_interval = "20m"

[memory.dream.gates]
min_unpromoted = 7
min_recall_count = 3
min_score = 0.85

[memory.dream.scoring]
recency_half_life_days = 10

[memory.dream.scoring.weights]
frequency = 0.25
relevance = 0.40
recency = 0.20
freshness = 0.15

[memory.session]
ledger_format = "jsonl"
ledger_root = "~/agh-sessions"
events_purge_grace = "12h"
cold_archive_days = 14
hard_delete_days = 1
max_archive_bytes = 2048
unbound_partition = "_orphans"

[memory.daily]
max_bytes = 2048
max_lines = 200
rotate_format = "{date}.md"
dreaming_window = 3
cold_archive_days = 7
hard_delete_days = 1
max_archive_bytes = 4096
sweep_hour = 4
archive_path = "_system/daily-archive"

[memory.file]
max_lines = 50
max_bytes = 8192

[memory.provider]
name = "local"
timeout = "1s"
failure_threshold = 3
cooldown = "10s"

[memory.workspace]
auto_create = false
`)

		cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		memory := cfg.Memory
		if memory.Controller.Mode != "rules" ||
			memory.Controller.MaxLatency != 150*time.Millisecond ||
			!slices.Equal(memory.Controller.Policy.AllowOrigins, []string{"cli", "tool"}) {
			t.Fatalf("Load() Controller = %#v", memory.Controller)
		}
		if memory.Recall.Fusion != "rrf" ||
			!memory.Recall.IncludeAlreadySurfaced ||
			!memory.Recall.IncludeSystem ||
			memory.Recall.Signals.MetricsEnabled {
			t.Fatalf("Load() Recall = %#v", memory.Recall)
		}
		if memory.Decisions.MaxPostContentBytes != 32768 ||
			memory.Extractor.Queue.CoalesceMax != 8 ||
			memory.Dream.Gates.MinScore != 0.85 ||
			memory.Session.UnboundPartition != "_orphans" ||
			memory.Daily.ArchivePath != "_system/daily-archive" ||
			memory.File.MaxBytes != 8192 ||
			memory.Provider.Name != "local" ||
			memory.Workspace.AutoCreate {
			t.Fatalf("Load() Memory tail config = %#v", memory)
		}
		if !strings.HasSuffix(memory.Extractor.InboxPath, "agh-inbox") ||
			!strings.HasSuffix(memory.Session.LedgerRoot, "agh-sessions") {
			t.Fatalf("Load() normalized paths = %q/%q", memory.Extractor.InboxPath, memory.Session.LedgerRoot)
		}
	})
}

func TestMemoryV2ConfigValidationRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	base := DefaultWithHome(homePaths).Memory
	tests := []struct {
		name  string
		patch func(*MemoryConfig)
		want  string
	}{
		{
			name: "controller mode",
			patch: func(cfg *MemoryConfig) {
				cfg.Controller.Mode = "auto"
			},
			want: "memory.controller.mode",
		},
		{
			name: "recall weights",
			patch: func(cfg *MemoryConfig) {
				cfg.Recall.Weights.RecallSignal = 0.50
			},
			want: "memory.recall.weights",
		},
		{
			name: "extractor mode",
			patch: func(cfg *MemoryConfig) {
				cfg.Extractor.Mode = "tail"
			},
			want: "memory.extractor.mode",
		},
		{
			name: "dream gate score",
			patch: func(cfg *MemoryConfig) {
				cfg.Dream.Gates.MinScore = 1.5
			},
			want: "memory.dream.gates.min_score",
		},
		{
			name: "session ledger format",
			patch: func(cfg *MemoryConfig) {
				cfg.Session.LedgerFormat = "json"
			},
			want: "memory.session.ledger_format",
		},
		{
			name: "daily sweep hour",
			patch: func(cfg *MemoryConfig) {
				cfg.Daily.SweepHour = 24
			},
			want: "memory.daily.sweep_hour",
		},
		{
			name: "workspace toml path",
			patch: func(cfg *MemoryConfig) {
				cfg.Workspace.TOMLPath = ".agh/workspace.toml"
			},
			want: "memory.workspace.toml_path",
		},
	}

	for _, tc := range tests {
		t.Run("Should reject "+tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := base
			tc.patch(&cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestMemoryV2ConfigValidationCoversOptionalBranches(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	base := DefaultWithHome(homePaths).Memory

	t.Run("Should accept controller LLM mode because role routing is validated separately", func(t *testing.T) {
		t.Parallel()

		cfg := base
		cfg.Controller.Mode = "llm"

		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("Should require an enabled memory controller role for LLM mode", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(homePaths)
		cfg.Memory.Controller.Mode = "llm"
		cfg.Roles.MemoryController.Enabled = false

		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "roles.memory_controller.enabled") {
			t.Fatalf("Config.Validate() error = %v, want memory controller role requirement", err)
		}
	})

	tests := []struct {
		name  string
		patch func(*MemoryConfig)
		want  string
	}{
		{
			name: "duplicate controller origin",
			patch: func(cfg *MemoryConfig) {
				cfg.Controller.Policy.AllowOrigins = []string{"cli", "cli"}
			},
			want: "duplicates",
		},
		{
			name: "extractor queue capacity",
			patch: func(cfg *MemoryConfig) {
				cfg.Extractor.Queue.Capacity = 0
			},
			want: "memory.extractor.queue.capacity",
		},
		{
			name: "extractor coalesce below throttle",
			patch: func(cfg *MemoryConfig) {
				cfg.Extractor.ThrottleTurns = 4
				cfg.Extractor.Queue.CoalesceMax = 3
			},
			want: configExtractorQueueCoalesceMaxPath,
		},
		{
			name: "dream scoring weight sum",
			patch: func(cfg *MemoryConfig) {
				cfg.Dream.Scoring.Weights.Freshness = 0.50
			},
			want: "memory.dream.scoring.weights",
		},
		{
			name: "unsafe unbound partition",
			patch: func(cfg *MemoryConfig) {
				cfg.Session.UnboundPartition = "../bad"
			},
			want: "memory.session.unbound_partition",
		},
		{
			name: "daily archive path",
			patch: func(cfg *MemoryConfig) {
				cfg.Daily.ArchivePath = ""
			},
			want: "memory.daily.archive_path",
		},
		{
			name: "file max bytes",
			patch: func(cfg *MemoryConfig) {
				cfg.File.MaxBytes = 0
			},
			want: "memory.file.max_bytes",
		},
		{
			name: "provider timeout",
			patch: func(cfg *MemoryConfig) {
				cfg.Provider.Timeout = 0
			},
			want: "memory.provider.timeout",
		},
	}

	for _, tc := range tests {
		t.Run("Should reject "+tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := base
			tc.patch(&cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestMemoryV2ConfigValidationNormalizesAcceptedEnums(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize accepted enum values", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths).Memory
		cfg.Controller.Mode = " Hybrid "
		cfg.Controller.DefaultOpOnFail = " Reject "
		cfg.Controller.Policy.AllowOrigins = []string{" CLI ", "Tool"}
		cfg.Recall.Fusion = " RRF "
		cfg.Extractor.Mode = " Post_Message "
		cfg.Session.LedgerFormat = " Jsonl "

		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if cfg.Controller.Mode != "hybrid" || cfg.Controller.DefaultOpOnFail != "reject" {
			t.Fatalf("controller enums = %#v, want canonical lowercase values", cfg.Controller)
		}
		if !slices.Equal(cfg.Controller.Policy.AllowOrigins, []string{"cli", "tool"}) {
			t.Fatalf(
				"controller allow origins = %#v, want canonical lowercase values",
				cfg.Controller.Policy.AllowOrigins,
			)
		}
		if cfg.Recall.Fusion != "rrf" {
			t.Fatalf("recall fusion = %q, want %q", cfg.Recall.Fusion, "rrf")
		}
		if cfg.Extractor.Mode != configExtractorModePostMessage {
			t.Fatalf("extractor mode = %q, want %q", cfg.Extractor.Mode, configExtractorModePostMessage)
		}
		if cfg.Session.LedgerFormat != "jsonl" {
			t.Fatalf("session ledger format = %q, want %q", cfg.Session.LedgerFormat, "jsonl")
		}
	})
}
