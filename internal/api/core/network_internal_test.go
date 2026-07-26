package core

import (
	"slices"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/store"
)

func TestNetworkThreadPromotionDigestInternal(t *testing.T) {
	t.Parallel()

	t.Run("Should keep scanning source message ids after digest reaches the preview limit", func(t *testing.T) {
		t.Parallel()

		digest, sourceIDs, found := networkThreadPromotionDigest([]store.NetworkConversationMessage{
			{
				MessageID: "msg-large",
				PeerFrom:  "coordinator.sess-a",
				Text:      strings.Repeat("large preview ", 420),
			},
			{
				MessageID: "msg-target",
				PeerFrom:  "reviewer.sess-b",
				Text:      "target message",
			},
		}, "msg-target")
		if !found {
			t.Fatal("networkThreadPromotionDigest() found = false, want true")
		}
		if !slices.Contains(sourceIDs, "msg-target") {
			t.Fatalf("sourceIDs = %#v, want target id after digest limit", sourceIDs)
		}
		if len(digest) > 4000 {
			t.Fatalf("len(digest) = %d, want <= 4000", len(digest))
		}
	})
}

func TestCloneNetworkMessageEntryInternal(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve mentions on cloned message entries", func(t *testing.T) {
		t.Parallel()

		entry := cloneNetworkMessageEntry(store.NetworkMessageEntry{
			MessageID: "msg-mention",
			Mentions:  []string{" reviewer.sess-a ", "", "coordinator.sess-b"},
		})
		if got, want := entry.Mentions, []string{"reviewer.sess-a", "coordinator.sess-b"}; !slices.Equal(got, want) {
			t.Fatalf("Mentions = %#v, want %#v", got, want)
		}
	})
}
