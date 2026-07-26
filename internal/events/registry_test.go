package events

import (
	"strings"
	"testing"
)

func TestRegistryMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should expose one complete metadata row per event", func(t *testing.T) {
		t.Parallel()

		seen := make(map[string]struct{})
		for _, meta := range All() {
			if strings.TrimSpace(meta.Name) == "" {
				t.Fatal("registry entry has empty name")
			}
			if _, ok := seen[meta.Name]; ok {
				t.Fatalf("registry entry %q is duplicated", meta.Name)
			}
			seen[meta.Name] = struct{}{}
			if strings.TrimSpace(meta.Family) == "" {
				t.Fatalf("registry entry %q has empty family", meta.Name)
			}
			if strings.TrimSpace(meta.Component) == "" {
				t.Fatalf("registry entry %q has empty component", meta.Name)
			}
			if !ValidOutcome(string(meta.Outcome)) || meta.Outcome == "" {
				t.Fatalf("registry entry %q has invalid outcome %q", meta.Name, meta.Outcome)
			}
			if !meta.EmitsToLogs {
				t.Fatalf("registry entry %q must declare log eligibility", meta.Name)
			}
			lookup, ok := Lookup(meta.Name)
			if !ok {
				t.Fatalf("Lookup(%q) failed", meta.Name)
			}
			if lookup != meta {
				t.Fatalf("Lookup(%q) = %#v, want %#v", meta.Name, lookup, meta)
			}
		}
	})

	t.Run("Should preserve task run underscore events and reject task_run dot family", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{
			TaskRunEnqueued,
			TaskRunClaimed,
			TaskRunStarted,
			TaskRunCompleted,
			TaskRunFailed,
			TaskRunCanceled,
		} {
			if _, ok := Lookup(name); !ok {
				t.Fatalf("Lookup(%q) = false, want true", name)
			}
		}
		if _, ok := Lookup("task_run.completed"); ok {
			t.Fatal("Lookup(task_run.completed) = true, want false")
		}
		if err := ValidatePublicName("task_run.completed"); err == nil {
			t.Fatal("ValidatePublicName(task_run.completed) error = nil, want non-nil")
		}
	})

	t.Run("Should expose metadata consumed by logs and notifications", func(t *testing.T) {
		t.Parallel()

		failed, ok := Lookup(TaskRunFailed)
		if !ok {
			t.Fatal("Lookup(TaskRunFailed) = false")
		}
		if failed.Outcome != OutcomeFailure || failed.Component != ComponentTask || !failed.NotificationEligible {
			t.Fatalf("TaskRunFailed metadata = %#v", failed)
		}
		retry, ok := Lookup(TaskRunOperatorRetry)
		if !ok {
			t.Fatal("Lookup(TaskRunOperatorRetry) = false")
		}
		if retry.Outcome != OutcomeInfo || retry.Component != ComponentTask || !retry.NotificationEligible {
			t.Fatalf("TaskRunOperatorRetry metadata = %#v", retry)
		}
		shadowed, ok := Lookup(SkillShadowed)
		if !ok {
			t.Fatal("Lookup(SkillShadowed) = false")
		}
		if shadowed.Component != ComponentSkill || shadowed.Outcome != OutcomeWarning || !shadowed.GlobalScope {
			t.Fatalf("SkillShadowed metadata = %#v", shadowed)
		}
		if _, ok := Lookup("skills.shadow"); ok {
			t.Fatal("Lookup(skills.shadow) = true, want false after hard cut")
		}
		catalogRefresh, ok := Lookup(MarketplaceCatalogRefresh)
		if !ok {
			t.Fatal("Lookup(MarketplaceCatalogRefresh) = false")
		}
		if catalogRefresh.Family != "marketplace.catalog" ||
			catalogRefresh.Component != ComponentMarketplace ||
			catalogRefresh.Outcome != OutcomeInfo || !catalogRefresh.GlobalScope {
			t.Fatalf("MarketplaceCatalogRefresh metadata = %#v", catalogRefresh)
		}
		install, ok := Lookup(MarketplaceInstall)
		if !ok {
			t.Fatal("Lookup(MarketplaceInstall) = false")
		}
		if install.Family != "marketplace.install" ||
			install.Component != ComponentMarketplace ||
			install.Outcome != OutcomeInfo || !install.GlobalScope {
			t.Fatalf("MarketplaceInstall metadata = %#v", install)
		}
		digestVerify, ok := Lookup(ExtensionDigestVerify)
		if !ok {
			t.Fatal("Lookup(ExtensionDigestVerify) = false")
		}
		if digestVerify.Family != "extension.digest" || digestVerify.Component != ComponentExtension ||
			digestVerify.Outcome != OutcomeInfo || !digestVerify.GlobalScope {
			t.Fatalf("ExtensionDigestVerify metadata = %#v", digestVerify)
		}
		for _, name := range []string{MCPAuthBegin, MCPAuthExchange, MCPAuthLogout} {
			metadata, ok := Lookup(name)
			if !ok {
				t.Fatalf("Lookup(%q) = false", name)
			}
			if metadata.Family != "mcp.auth" || metadata.Component != ComponentMCP ||
				metadata.Outcome != OutcomeInfo || !metadata.GlobalScope || metadata.NotificationEligible {
				t.Fatalf("MCP auth metadata for %q = %#v", name, metadata)
			}
		}
	})

	t.Run("Should expose canonical role invocation metadata", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			outcome Outcome
		}{
			{name: RoleFallbackUsed, outcome: OutcomeInfo},
			{name: RoleResolveError, outcome: OutcomeFailure},
		}
		for _, tc := range cases {
			t.Run("Should expose "+tc.name, func(t *testing.T) {
				t.Parallel()
				meta, ok := Lookup(tc.name)
				if !ok {
					t.Fatalf("Lookup(%q) = false", tc.name)
				}
				if meta.Family != "role" || meta.Component != ComponentRole ||
					meta.Outcome != tc.outcome || !meta.GlobalScope || !meta.EmitsToLogs ||
					meta.NotificationEligible {
					t.Fatalf("role metadata for %q = %#v", tc.name, meta)
				}
			})
		}
	})

	t.Run("Should keep memory operation projections out of direct global writes", func(t *testing.T) {
		t.Parallel()

		if AllowsGlobalScope(MemoryWriteCommitted) {
			t.Fatal("AllowsGlobalScope(MemoryWriteCommitted) = true, want false for memory_events projection")
		}
		if !AllowsGlobalScope(MemoryProviderCollision) {
			t.Fatal("AllowsGlobalScope(MemoryProviderCollision) = false, want true for extension collision summaries")
		}
	})

	t.Run("Should expose transcript stream snapshot metadata", func(t *testing.T) {
		t.Parallel()

		meta, ok := Lookup(SessionStreamSnapshotServed)
		if !ok {
			t.Fatal("Lookup(SessionStreamSnapshotServed) = false")
		}
		if meta.Family != "transcript_stream" ||
			meta.Component != ComponentTranscript ||
			meta.Outcome != OutcomeInfo ||
			!meta.EmitsToLogs {
			t.Fatalf("SessionStreamSnapshotServed metadata = %#v", meta)
		}
		subscribed, ok := Lookup(SessionStreamSubscribed)
		if !ok {
			t.Fatal("Lookup(SessionStreamSubscribed) = false")
		}
		if subscribed.Family != "transcript_stream" ||
			subscribed.Component != ComponentTranscript ||
			subscribed.Outcome != OutcomeInfo ||
			!subscribed.EmitsToLogs {
			t.Fatalf("SessionStreamSubscribed metadata = %#v", subscribed)
		}
		overflow, ok := Lookup(SessionStreamOverflowFallback)
		if !ok {
			t.Fatal("Lookup(SessionStreamOverflowFallback) = false")
		}
		if overflow.Family != "transcript_stream" ||
			overflow.Component != ComponentTranscript ||
			overflow.Outcome != OutcomeWarning ||
			!overflow.EmitsToLogs {
			t.Fatalf("SessionStreamOverflowFallback metadata = %#v", overflow)
		}
	})

	t.Run("Should expose session compaction lifecycle metadata", func(t *testing.T) {
		t.Parallel()

		meta, ok := Lookup(SessionCompactionFired)
		if !ok {
			t.Fatal("Lookup(SessionCompactionFired) = false")
		}
		if meta.Family != "session" ||
			meta.Component != ComponentSession ||
			meta.Outcome != OutcomeInfo ||
			!meta.EmitsToLogs ||
			meta.NotificationEligible {
			t.Fatalf("SessionCompactionFired metadata = %#v", meta)
		}
	})

	t.Run("Should expose dead entity transition metadata", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			outcome Outcome
		}{
			{name: DeadEntityMarked, outcome: OutcomeWarning},
			{name: DeadEntityCleared, outcome: OutcomeSuccess},
		}
		for _, tc := range cases {
			meta, ok := Lookup(tc.name)
			if !ok {
				t.Fatalf("Lookup(%q) = false", tc.name)
			}
			if meta.Family != "reliability.dead_entity" ||
				meta.Component != ComponentReliability ||
				meta.Outcome != tc.outcome ||
				!meta.GlobalScope ||
				!meta.EmitsToLogs {
				t.Fatalf("Lookup(%q) metadata = %#v", tc.name, meta)
			}
		}
	})

	t.Run("Should expose durable tool approval transition metadata", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			outcome Outcome
		}{
			{name: ToolApprovalGrantPut, outcome: OutcomeSuccess},
			{name: ToolApprovalGrantRevoked, outcome: OutcomeWarning},
		}
		for _, tc := range cases {
			meta, ok := Lookup(tc.name)
			if !ok {
				t.Fatalf("Lookup(%q) = false", tc.name)
			}
			if meta.Family != "tool.approval_grant" ||
				meta.Component != ComponentTools ||
				meta.Outcome != tc.outcome ||
				!meta.GlobalScope ||
				!meta.EmitsToLogs {
				t.Fatalf("Lookup(%q) metadata = %#v", tc.name, meta)
			}
		}
	})
}
