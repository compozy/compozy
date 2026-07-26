package bridgesdk

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func TestParentMessageCacheBoundsAndIsolatesReplyContext(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate parent messages by workspace instance conversation and message", func(t *testing.T) {
		t.Parallel()

		cache := NewParentMessageCache(4)
		key := ParentMessageKey{
			WorkspaceID:      "ws-a",
			BridgeInstanceID: "brg-a",
			ConversationID:   "conversation-a",
			MessageID:        "message-a",
		}
		cache.Put(key, ParentMessage{
			Text:       "parent text",
			AuthorID:   "user-a",
			AuthorName: "Alice",
		})

		got, ok := cache.Get(key)
		if !ok {
			t.Fatal("Get(exact key) ok = false, want true")
		}
		if got.Text != "parent text" || got.AuthorID != "user-a" || got.AuthorName != "Alice" {
			t.Fatalf("Get(exact key) = %#v, want complete parent context", got)
		}

		isolatedKeys := []ParentMessageKey{
			{WorkspaceID: "ws-b", BridgeInstanceID: "brg-a", ConversationID: "conversation-a", MessageID: "message-a"},
			{WorkspaceID: "ws-a", BridgeInstanceID: "brg-b", ConversationID: "conversation-a", MessageID: "message-a"},
			{WorkspaceID: "ws-a", BridgeInstanceID: "brg-a", ConversationID: "conversation-b", MessageID: "message-a"},
			{WorkspaceID: "ws-a", BridgeInstanceID: "brg-a", ConversationID: "conversation-a", MessageID: "message-b"},
		}
		for _, isolatedKey := range isolatedKeys {
			if value, found := cache.Get(isolatedKey); found {
				t.Fatalf("Get(isolated key %#v) = (%#v, true), want miss", isolatedKey, value)
			}
		}
	})

	t.Run("Should cap cached text at five hundred runes", func(t *testing.T) {
		t.Parallel()

		cache := NewParentMessageCache(1)
		key := ParentMessageKey{
			WorkspaceID:      "ws-a",
			BridgeInstanceID: "brg-a",
			ConversationID:   "conversation-a",
			MessageID:        "message-a",
		}
		cache.Put(key, ParentMessage{Text: strings.Repeat("🙂", ParentMessageTextLimit+17)})

		got, ok := cache.Get(key)
		if !ok {
			t.Fatal("Get(capped parent) ok = false, want true")
		}
		if runeCount := len([]rune(got.Text)); runeCount != ParentMessageTextLimit {
			t.Fatalf("cached text rune count = %d, want %d", runeCount, ParentMessageTextLimit)
		}
		if !strings.HasSuffix(got.Text, "🙂") {
			t.Fatalf("cached text = %q, want rune-safe suffix", got.Text)
		}
	})

	t.Run("Should evict the least recently used parent deterministically", func(t *testing.T) {
		t.Parallel()

		cache := NewParentMessageCache(2)
		key := func(messageID string) ParentMessageKey {
			return ParentMessageKey{
				WorkspaceID:      "ws-a",
				BridgeInstanceID: "brg-a",
				ConversationID:   "conversation-a",
				MessageID:        messageID,
			}
		}
		cache.Put(key("message-a"), ParentMessage{Text: "A"})
		cache.Put(key("message-b"), ParentMessage{Text: "B"})
		if _, ok := cache.Get(key("message-a")); !ok {
			t.Fatal("Get(message-a) ok = false, want true before promotion")
		}
		cache.Put(key("message-c"), ParentMessage{Text: "C"})

		if value, ok := cache.Get(key("message-b")); ok {
			t.Fatalf("Get(message-b) = (%#v, true), want LRU eviction", value)
		}
		for _, messageID := range []string{"message-a", "message-c"} {
			if _, ok := cache.Get(key(messageID)); !ok {
				t.Fatalf("Get(%s) ok = false, want retained entry", messageID)
			}
		}
	})

	t.Run("Should cap oversized configurations at four thousand parents", func(t *testing.T) {
		t.Parallel()

		cache := NewParentMessageCache(ParentMessageCacheLimit + 1)
		key := func(messageID string) ParentMessageKey {
			return ParentMessageKey{
				WorkspaceID:      "ws-a",
				BridgeInstanceID: "brg-a",
				ConversationID:   "conversation-a",
				MessageID:        messageID,
			}
		}
		for index := 0; index <= ParentMessageCacheLimit; index++ {
			cache.Put(key(strconv.Itoa(index)), ParentMessage{Text: "parent"})
		}
		if value, ok := cache.Get(key("0")); ok {
			t.Fatalf("Get(oldest parent) = (%#v, true), want bounded eviction", value)
		}
		if value, ok := cache.Get(key(strconv.Itoa(ParentMessageCacheLimit))); !ok || value.Text != "parent" {
			t.Fatalf("Get(newest parent) = (%#v, %v), want retained entry", value, ok)
		}
	})

	t.Run("Should replace and delete cached parent context from accepted edit envelopes", func(t *testing.T) {
		t.Parallel()

		cache := NewParentMessageCache(4)
		base := bridgepkg.InboundMessageEnvelope{
			BridgeInstanceID:  "brg-a",
			WorkspaceID:       "ws-a",
			GroupID:           "conversation-a",
			PlatformMessageID: "message-a",
			Sender: bridgepkg.MessageSender{
				ID:          "user-a",
				DisplayName: "Alice",
			},
			Content:     bridgepkg.MessageContent{Text: "original text"},
			EventFamily: bridgepkg.InboundEventFamilyMessage,
		}
		cache.RememberEnvelope(base)

		updated := base
		updated.PlatformMessageID = ""
		updated.Content = bridgepkg.MessageContent{}
		updated.EventFamily = bridgepkg.InboundEventFamilyEdit
		updated.Edit = &bridgepkg.InboundEdit{
			MessageID:         "message-a",
			NewText:           "corrected text",
			OriginalTimestamp: time.Date(2026, 7, 12, 14, 0, 0, 0, time.UTC),
			Operation:         bridgepkg.InboundEditOperationUpdated,
		}
		cache.RememberEnvelope(updated)

		probe := bridgepkg.InboundMessageEnvelope{
			BridgeInstanceID: "brg-a",
			WorkspaceID:      "ws-a",
			GroupID:          "conversation-a",
		}
		if ok := cache.EnrichReply(&probe, "message-a"); !ok {
			t.Fatal("EnrichReply(updated parent) ok = false, want true")
		}
		if probe.ReplyToText != "corrected text" ||
			probe.ReplyToAuthorID != "user-a" ||
			probe.ReplyToAuthorName != "Alice" {
			t.Fatalf("updated reply context = %#v, want corrected parent", probe)
		}

		deleted := updated
		deleted.Edit = &bridgepkg.InboundEdit{
			MessageID:         "message-a",
			OriginalTimestamp: updated.Edit.OriginalTimestamp,
			Operation:         bridgepkg.InboundEditOperationDeleted,
		}
		cache.RememberEnvelope(deleted)
		if ok := cache.EnrichReply(&probe, "message-a"); ok {
			t.Fatal("EnrichReply(deleted parent) ok = true, want cache miss")
		}
	})

	t.Run("Should remain race safe under concurrent provider ingress", func(t *testing.T) {
		t.Parallel()

		cache := NewParentMessageCache(32)
		var wait sync.WaitGroup
		for index := range 64 {
			wait.Add(1)
			go func(index int) {
				defer wait.Done()
				key := ParentMessageKey{
					WorkspaceID:      "ws-a",
					BridgeInstanceID: "brg-a",
					ConversationID:   "conversation-a",
					MessageID:        strconv.Itoa(index),
				}
				cache.Put(key, ParentMessage{Text: "parent"})
				cache.Get(key)
			}(index)
		}
		wait.Wait()

		finalKey := ParentMessageKey{
			WorkspaceID:      "ws-a",
			BridgeInstanceID: "brg-a",
			ConversationID:   "conversation-a",
			MessageID:        "final",
		}
		cache.Put(finalKey, ParentMessage{Text: "final parent"})
		if got, ok := cache.Get(finalKey); !ok || got.Text != "final parent" {
			t.Fatalf("Get(final) = (%#v, %v), want final parent", got, ok)
		}
	})
}
