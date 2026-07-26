package network

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	sessionpkg "github.com/compozy/agh/internal/session"
)

func TestPeerRegistryTracksLocalMembership(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	registry, err := NewPeerRegistry(WithPeerRegistryClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewPeerRegistry() error = %v", err)
	}
	buildersCard := mustPeerCard(t, "coder.sess-builders")
	opsCard := mustPeerCard(t, "reviewer.sess-ops")
	if _, err := registry.RegisterLocal(
		"sess-builders",
		testWorkspaceID,
		"builders",
		buildersCard,
		time.Time{},
	); err != nil {
		t.Fatalf("RegisterLocal(builders) error = %v", err)
	}
	if _, err := registry.RegisterLocal("sess-ops", testWorkspaceID, "ops", opsCard, now); err != nil {
		t.Fatalf("RegisterLocal(ops) error = %v", err)
	}

	peers := registry.ListPeers(testWorkspaceID, "builders")
	if got, want := len(peers), 1; got != want {
		t.Fatalf("len(ListPeers(builders)) = %d, want %d", got, want)
	}
	if !peers[0].Local || peers[0].PresenceState != PresenceStateLocal {
		t.Fatalf("ListPeers(builders)[0] = %#v, want local presence", peers[0])
	}
	if peers[0].JoinedAt == nil || !peers[0].JoinedAt.Equal(now) {
		t.Fatalf("ListPeers(builders)[0].JoinedAt = %#v, want %s", peers[0].JoinedAt, now)
	}
	if local, ok := registry.LocalBySession("sess-builders"); !ok || local.PeerID != buildersCard.PeerID {
		t.Fatalf("LocalBySession(sess-builders) = %#v, %v", local, ok)
	}
	local, ok := registry.LocalByPeer(testWorkspaceID, "builders", buildersCard.PeerID)
	if !ok || local.SessionID != "sess-builders" {
		t.Fatalf("LocalByPeer(builders) = %#v, %v", local, ok)
	}

	channels := registry.ListChannels(testWorkspaceID)
	if got, want := len(channels), 2; got != want {
		t.Fatalf("len(ListChannels()) = %d, want %d", got, want)
	}
	if channels[0].Channel != "builders" || channels[0].PeerCount != 1 {
		t.Fatalf("ListChannels()[0] = %#v, want builders with one peer", channels[0])
	}
	if _, ok := registry.LeaveLocal("sess-builders"); !ok {
		t.Fatal("LeaveLocal(sess-builders) ok = false, want true")
	}
	if _, ok := registry.LocalByPeer(testWorkspaceID, "builders", buildersCard.PeerID); ok {
		t.Fatalf("LocalByPeer(builders, %q) remained after leave", buildersCard.PeerID)
	}
}

func TestPeerRegistryMovesLocalPeerAndRejectsDuplicatePeerID(t *testing.T) {
	t.Parallel()

	registry, err := NewPeerRegistry()
	if err != nil {
		t.Fatalf("NewPeerRegistry() error = %v", err)
	}
	now := time.Date(2026, 4, 10, 12, 40, 0, 0, time.UTC)
	card := mustPeerCard(t, "reviewer.sess-b")
	if _, err := registry.RegisterLocal("sess-b", testWorkspaceID, "builders", card, now); err != nil {
		t.Fatalf("RegisterLocal(builders) error = %v", err)
	}
	if _, err := registry.RegisterLocal("sess-c", testWorkspaceID, "builders", card, now); err == nil {
		t.Fatal("RegisterLocal(duplicate peer id) error = nil, want rejection")
	}
	if _, err := registry.RegisterLocal("sess-b", testWorkspaceID, "ops", card, now); err != nil {
		t.Fatalf("RegisterLocal(ops) error = %v", err)
	}
	if _, ok := registry.LocalByPeer(testWorkspaceID, "builders", card.PeerID); ok {
		t.Fatalf("LocalByPeer(builders, %q) present after move", card.PeerID)
	}
	if moved, ok := registry.LocalByPeer(testWorkspaceID, "ops", card.PeerID); !ok || moved.Channel != "ops" {
		t.Fatalf("LocalByPeer(ops, %q) = %#v, %v", card.PeerID, moved, ok)
	}
}

func TestPeerRegistryValidatesConstructorAndDefaultCard(t *testing.T) {
	t.Parallel()

	if _, err := NewPeerRegistry(WithPeerRegistryClock(nil)); err == nil {
		t.Fatal("NewPeerRegistry(nil clock) error = nil, want non-nil")
	}
	if _, err := DefaultPeerCard("Bad Peer"); err == nil {
		t.Fatal("DefaultPeerCard(invalid) error = nil, want non-nil")
	}
}

func TestProjectCapabilityBriefViewMatchesProjectedIDsAndBriefEntries(t *testing.T) {
	t.Parallel()

	t.Run("Should keep capability ids and brief entries aligned in stable order", func(t *testing.T) {
		t.Parallel()

		ids, ext, err := projectCapabilityBriefView([]sessionpkg.NetworkPeerCapability{
			{ID: " review-pr ", Summary: " Review pull requests "},
			{ID: "draft-spec", Summary: "Draft technical specs"},
		})
		if err != nil {
			t.Fatalf("projectCapabilityBriefView() error = %v", err)
		}

		wantBrief := []capabilityBrief{
			{ID: "review-pr", Summary: "Review pull requests"},
			{ID: "draft-spec", Summary: "Draft technical specs"},
		}
		if got, want := ids, []string{"review-pr", "draft-spec"}; !slices.Equal(got, want) {
			t.Fatalf("projected capability ids = %#v, want %#v", got, want)
		}
		if got := decodeCapabilityBriefPayload(t, ext[capabilityBriefExtKey]); !slices.Equal(got, wantBrief) {
			t.Fatalf("capability brief entries = %#v, want %#v", got, wantBrief)
		}
	})

	t.Run("Should keep no-catalog peers discovery-empty without the brief ext key", func(t *testing.T) {
		t.Parallel()

		ids, ext, err := projectCapabilityBriefView(nil)
		if err != nil {
			t.Fatalf("projectCapabilityBriefView(nil) error = %v", err)
		}
		if ids == nil || len(ids) != 0 {
			t.Fatalf("projected ids = %#v, want empty-but-valid slice", ids)
		}
		if ext != nil && ext[capabilityBriefExtKey] != nil {
			t.Fatalf("projected ext = %#v, want omitted capability brief key", ext)
		}
	})
}

func TestCloneAndNormalizePeerCardPreserveCapabilityBriefExt(t *testing.T) {
	t.Parallel()

	card, err := DefaultPeerCard("reviewer.sess-brief")
	if err != nil {
		t.Fatalf("DefaultPeerCard() error = %v", err)
	}
	if err := applyCapabilityBriefProjection(&card, []sessionpkg.NetworkPeerCapability{{
		ID: "review-pr", Summary: "Review pull requests",
	}}); err != nil {
		t.Fatalf("applyCapabilityBriefProjection() error = %v", err)
	}

	cloned := clonePeerCard(card)
	normalized, err := normalizePeerCard(card)
	if err != nil {
		t.Fatalf("normalizePeerCard() error = %v", err)
	}
	wantCapabilities := append([]string(nil), card.Capabilities...)
	wantBriefRaw := append(json.RawMessage(nil), card.Ext[capabilityBriefExtKey]...)
	card.Capabilities[0] = "mutated"
	card.Ext[capabilityBriefExtKey][0] = '{'

	if !slices.Equal(cloned.Capabilities, wantCapabilities) ||
		!slices.Equal(normalized.Capabilities, wantCapabilities) {
		t.Fatalf(
			"cloned capability ids diverged: cloned=%#v normalized=%#v",
			cloned.Capabilities,
			normalized.Capabilities,
		)
	}
	if string(cloned.Ext[capabilityBriefExtKey]) != string(wantBriefRaw) ||
		string(normalized.Ext[capabilityBriefExtKey]) != string(wantBriefRaw) {
		t.Fatal("cloned capability brief bytes changed after source mutation")
	}
}

func TestPeerRegistryPreservesLocalCapabilityCatalog(t *testing.T) {
	t.Parallel()

	registry, err := NewPeerRegistry()
	if err != nil {
		t.Fatalf("NewPeerRegistry() error = %v", err)
	}
	card := mustPeerCard(t, "reviewer.sess-local")
	catalog := []sessionpkg.NetworkPeerCapability{{
		ID: "review-pr", Summary: "Review pull requests", Outcome: "Actionable findings",
		Version: "1.0.0", Digest: "sha256:review-pr-v1", Requirements: []string{"workspace-read"},
	}}
	local, err := registry.RegisterLocalWithCapabilityCatalog(
		"sess-local", testWorkspaceID, "builders", card, catalog, time.Time{},
	)
	if err != nil {
		t.Fatalf("RegisterLocalWithCapabilityCatalog() error = %v", err)
	}
	catalog[0].Summary = "mutated"
	local.CapabilityCatalog[0].Summary = "also mutated"

	peers := registry.ListPeers(testWorkspaceID, "builders")
	if len(peers) != 1 || !peers[0].CapabilityCatalogKnown {
		t.Fatalf("ListPeers() = %#v, want one peer with known catalog", peers)
	}
	if got, want := peers[0].CapabilityCatalog[0].Summary, "Review pull requests"; got != want {
		t.Fatalf("stored catalog summary = %q, want %q", got, want)
	}
}

func mustPeerCard(t *testing.T, peerID string) PeerCard {
	t.Helper()

	card, err := DefaultPeerCard(peerID)
	if err != nil {
		t.Fatalf("DefaultPeerCard(%q) error = %v", peerID, err)
	}
	return card
}

func decodeCapabilityBriefPayload(t *testing.T, raw json.RawMessage) []capabilityBrief {
	t.Helper()

	if len(raw) == 0 {
		return nil
	}
	var brief []capabilityBrief
	if err := json.Unmarshal(raw, &brief); err != nil {
		t.Fatalf("json.Unmarshal(capability brief) error = %v", err)
	}
	return brief
}
