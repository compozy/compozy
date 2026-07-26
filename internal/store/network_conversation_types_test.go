package store

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

const networkConversationTestWorkspaceID = "ws_store_validation"

func TestNetworkUsageQueryValidationAndCursor(t *testing.T) {
	t.Parallel()

	owner := &participation.OwnerRef{
		WorkspaceID: networkConversationTestWorkspaceID,
		Kind:        participation.OwnerKindTaskRun,
		ID:          "run-1",
	}
	normalized, cursor, err := NormalizeNetworkUsageQuery(NetworkUsageQuery{
		WorkspaceID: networkConversationTestWorkspaceID,
		Owner:       owner,
		Channel:     " builders ",
	})
	if err != nil {
		t.Fatalf("NormalizeNetworkUsageQuery(default) error = %v", err)
	}
	if normalized.Limit != DefaultNetworkUsageLimit || normalized.Channel != "builders" || cursor != nil {
		t.Fatalf("NormalizeNetworkUsageQuery(default) = %#v, cursor=%#v", normalized, cursor)
	}

	normalized, _, err = NormalizeNetworkUsageQuery(NetworkUsageQuery{
		WorkspaceID: networkConversationTestWorkspaceID,
		Limit:       MaxNetworkUsageLimit,
		LimitSet:    true,
	})
	if err != nil || normalized.Limit != MaxNetworkUsageLimit {
		t.Fatalf("NormalizeNetworkUsageQuery(max) = %#v, error = %v", normalized, err)
	}

	for _, limit := range []int{0, MaxNetworkUsageLimit + 1} {
		_, _, err = NormalizeNetworkUsageQuery(NetworkUsageQuery{
			WorkspaceID: networkConversationTestWorkspaceID,
			Limit:       limit,
			LimitSet:    true,
		})
		if !errors.Is(err, ErrNetworkUsageLimitInvalid) {
			t.Fatalf("NormalizeNetworkUsageQuery(limit=%d) error = %v", limit, err)
		}
	}

	_, _, err = NormalizeNetworkUsageQuery(NetworkUsageQuery{
		WorkspaceID: networkConversationTestWorkspaceID,
		Owner:       owner,
		RunID:       "run-1",
	})
	if !errors.Is(err, ErrNetworkUsageQueryInvalid) {
		t.Fatalf("NormalizeNetworkUsageQuery(owner+run) error = %v", err)
	}

	position := NetworkUsageCursorPosition{
		ReservedAt: time.Date(2026, 7, 16, 18, 30, 0, 123, time.UTC),
		WakeID:     "wake-050",
	}
	encoded, err := EncodeNetworkUsageCursor(normalized, position)
	if err != nil {
		t.Fatalf("EncodeNetworkUsageCursor() error = %v", err)
	}
	normalized.Cursor = encoded
	_, decoded, err := NormalizeNetworkUsageQuery(normalized)
	if err != nil {
		t.Fatalf("NormalizeNetworkUsageQuery(cursor) error = %v", err)
	}
	if decoded == nil || !decoded.ReservedAt.Equal(position.ReservedAt) || decoded.WakeID != position.WakeID {
		t.Fatalf("decoded cursor = %#v, want %#v", decoded, position)
	}

	normalized.Channel = "another-channel"
	_, _, err = NormalizeNetworkUsageQuery(normalized)
	if !errors.Is(err, ErrNetworkUsageCursorInvalid) {
		t.Fatalf("NormalizeNetworkUsageQuery(reused cursor) error = %v", err)
	}
	_, _, err = NormalizeNetworkUsageQuery(NetworkUsageQuery{
		WorkspaceID: networkConversationTestWorkspaceID,
		Cursor:      "not-base64",
	})
	if !errors.Is(err, ErrNetworkUsageCursorInvalid) {
		t.Fatalf("NormalizeNetworkUsageQuery(malformed cursor) error = %v", err)
	}
}

func TestValidateJavaScriptSafeInteger(t *testing.T) {
	t.Parallel()

	if err := ValidateJavaScriptSafeInteger("tokens", MaxJavaScriptSafeInteger); err != nil {
		t.Fatalf("ValidateJavaScriptSafeInteger(max) error = %v", err)
	}
	if err := ValidateJavaScriptSafeInteger("tokens", MaxJavaScriptSafeInteger+1); !errors.Is(
		err,
		ErrNetworkUsageUnsafeInteger,
	) {
		t.Fatalf("ValidateJavaScriptSafeInteger(max+1) error = %v", err)
	}
}

func TestNetworkConversationRefValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should derive the stable direct-room identity from the documented hash vector", func(t *testing.T) {
		t.Parallel()

		directID, sessionA, sessionB, err := NetworkDirectRoomIdentity(
			networkConversationTestWorkspaceID,
			"builders",
			"sess-b",
			"sess-a",
		)
		if err != nil {
			t.Fatalf("NetworkDirectRoomIdentity() error = %v", err)
		}
		if got, want := directID, "direct_5101ac7ba31424a87228708e21b268bb"; got != want {
			t.Fatalf("direct id = %q, want known vector %q", got, want)
		}
		if sessionA != "sess-a" || sessionB != "sess-b" {
			t.Fatalf("ordered sessions = %q/%q, want sess-a/sess-b", sessionA, sessionB)
		}
	})

	tests := []struct {
		name    string
		ref     NetworkConversationRef
		wantErr string
	}{
		{
			name: "Should accept thread references",
			ref: NetworkConversationRef{
				WorkspaceID: networkConversationTestWorkspaceID,
				Channel:     "builders",
				Surface:     NetworkSurfaceThread,
				ThreadID:    "thread_patch_42",
			},
		},
		{
			name: "Should accept direct references",
			ref: NetworkConversationRef{
				WorkspaceID: networkConversationTestWorkspaceID,
				Channel:     "builders",
				Surface:     NetworkSurfaceDirect,
				DirectID:    "direct_0123456789abcdef0123456789abcdef",
			},
		},
		{
			name: "Should reject missing thread ids",
			ref: NetworkConversationRef{
				WorkspaceID: networkConversationTestWorkspaceID,
				Channel:     "builders",
				Surface:     NetworkSurfaceThread,
			},
			wantErr: "thread_id",
		},
		{
			name: "Should reject dual containers",
			ref: NetworkConversationRef{
				WorkspaceID: networkConversationTestWorkspaceID,
				Channel:     "builders",
				Surface:     NetworkSurfaceThread,
				ThreadID:    "thread_patch_42",
				DirectID:    "direct_0123456789abcdef0123456789abcdef",
			},
			wantErr: "direct_id",
		},
		{
			name: "Should reject unknown surfaces",
			ref: NetworkConversationRef{
				WorkspaceID: networkConversationTestWorkspaceID,
				Channel:     "builders",
				Surface:     "room",
				ThreadID:    "thread_patch_42",
			},
			wantErr: "surface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.ref.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestNetworkChannelEntryValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entry   NetworkChannelEntry
		wantErr string
	}{
		{
			name: "Should accept coordinator fanout with a coordinator peer",
			entry: NetworkChannelEntry{
				WorkspaceID:       networkConversationTestWorkspaceID,
				Channel:           "builders",
				Purpose:           "Pair reviews",
				FanoutPolicy:      NetworkFanoutPolicyCoordinator,
				CoordinatorPeerID: "reviewer.sess-a",
			},
		},
		{
			name: "Should reject coordinator fanout without a coordinator peer",
			entry: NetworkChannelEntry{
				WorkspaceID:  networkConversationTestWorkspaceID,
				Channel:      "builders",
				Purpose:      "Pair reviews",
				FanoutPolicy: NetworkFanoutPolicyCoordinator,
			},
			wantErr: "coordinator_peer_id",
		},
		{
			name: "Should reject coordinator peers on non coordinator fanout",
			entry: NetworkChannelEntry{
				WorkspaceID:       networkConversationTestWorkspaceID,
				Channel:           "builders",
				Purpose:           "Pair reviews",
				FanoutPolicy:      NetworkFanoutPolicyCapabilityMatch,
				CoordinatorPeerID: "reviewer.sess-a",
			},
			wantErr: "requires coordinator fanout policy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.entry.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeNetworkDirectRoomSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should return peers in lexicographic order", func(t *testing.T) {
		t.Parallel()

		peerA, peerB, err := NormalizeNetworkDirectRoomSessions("reviewer.sess-xyz", "coder.sess-abc")
		if err != nil {
			t.Fatalf("NormalizeNetworkDirectRoomSessions() error = %v", err)
		}
		if got, want := peerA, "coder.sess-abc"; got != want {
			t.Fatalf("peerA = %q, want %q", got, want)
		}
		if got, want := peerB, "reviewer.sess-xyz"; got != want {
			t.Fatalf("peerB = %q, want %q", got, want)
		}
	})

	t.Run("Should reject same peers", func(t *testing.T) {
		t.Parallel()

		_, _, err := NormalizeNetworkDirectRoomSessions("coder.sess-abc", " coder.sess-abc ")
		if err == nil || !strings.Contains(err.Error(), "must differ") {
			t.Fatalf("NormalizeNetworkDirectRoomSessions() error = %v, want same-peer rejection", err)
		}
	})

	t.Run("Should reject missing sessions", func(t *testing.T) {
		t.Parallel()

		_, _, err := NormalizeNetworkDirectRoomSessions("  ", "reviewer.sess-xyz")
		if err == nil || !strings.Contains(err.Error(), "session_a") {
			t.Fatalf("NormalizeNetworkDirectRoomSessions() error = %v, want session_a rejection", err)
		}
	})
}

func TestNetworkConversationSummaryValidation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

	t.Run("Should validate thread summaries", func(t *testing.T) {
		t.Parallel()

		summary := NetworkThreadSummary{
			WorkspaceID:           networkConversationTestWorkspaceID,
			Channel:               "builders",
			ThreadID:              "thread_patch_42",
			RootMessageID:         "msg_patch_42",
			OpenedByPeerID:        "coder.sess-abc",
			OpenedAt:              now,
			LastActivityAt:        now,
			MessageCount:          1,
			ParticipantCount:      1,
			OpenWorkCount:         0,
			DeliveredCount:        1,
			PromptSizeBytes:       128,
			EstimatedPromptTokens: 32,
			LastMessagePreview:    "hello",
		}
		if err := summary.Validate(); err != nil {
			t.Fatalf("Validate(thread summary) error = %v", err)
		}

		summary.ParticipantCount = -1
		if err := summary.Validate(); err == nil || !strings.Contains(err.Error(), "participant_count") {
			t.Fatalf("Validate(thread summary) error = %v, want participant_count rejection", err)
		}
		summary.ParticipantCount = 1
		summary.PromptSizeBytes = -1
		if err := summary.Validate(); err == nil || !strings.Contains(err.Error(), "prompt_size_bytes") {
			t.Fatalf("Validate(thread summary) error = %v, want prompt_size_bytes rejection", err)
		}
	})

	t.Run("Should validate direct room summaries", func(t *testing.T) {
		t.Parallel()

		summary := NetworkDirectRoomSummary{
			WorkspaceID:    networkConversationTestWorkspaceID,
			Channel:        "builders",
			DirectID:       "direct_0123456789abcdef0123456789abcdef",
			SessionA:       "coder.sess-abc",
			SessionB:       "reviewer.sess-xyz",
			OpenedAt:       now,
			LastActivityAt: now,
			MessageCount:   1,
			OpenWorkCount:  0,
		}
		if err := summary.Validate(); err != nil {
			t.Fatalf("Validate(direct summary) error = %v", err)
		}

		summary.SessionA = "reviewer.sess-xyz"
		summary.SessionB = "coder.sess-abc"
		if err := summary.Validate(); err == nil || !strings.Contains(err.Error(), "lexicographic") {
			t.Fatalf("Validate(direct summary) error = %v, want peer ordering rejection", err)
		}
	})
}

func TestNetworkDirectRoomEntryValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should validate direct room write rows", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		entry := NetworkDirectRoomEntry{
			WorkspaceID:    networkConversationTestWorkspaceID,
			Channel:        "builders",
			DirectID:       "direct_0123456789abcdef0123456789abcdef",
			SessionA:       "sess-coder",
			SessionB:       "sess-reviewer",
			OpenedAt:       now,
			LastActivityAt: now,
		}
		if err := entry.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}

		entry.LastActivityAt = time.Time{}
		if err := entry.Validate(); err == nil || !strings.Contains(err.Error(), "last_activity_at") {
			t.Fatalf("Validate() error = %v, want last_activity_at rejection", err)
		}
	})
}

func TestNetworkWorkEntryValidation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	valid := NetworkWorkEntry{
		WorkspaceID:       networkConversationTestWorkspaceID,
		WorkID:            "work_patch_42",
		Channel:           "builders",
		Surface:           NetworkSurfaceThread,
		ThreadID:          "thread_patch_42",
		OpenedBySessionID: "sess-coder",
		State:             NetworkWorkStateSubmitted,
		OpenedAt:          now,
		LastActivityAt:    now,
	}

	tests := []struct {
		name    string
		mutate  func(*NetworkWorkEntry)
		wantErr string
	}{
		{
			name: "Should accept valid work rows",
		},
		{
			name: "Should reject dangling thread refs",
			mutate: func(entry *NetworkWorkEntry) {
				entry.ThreadID = ""
			},
			wantErr: "thread_id",
		},
		{
			name: "Should reject dual container refs",
			mutate: func(entry *NetworkWorkEntry) {
				entry.DirectID = "direct_0123456789abcdef0123456789abcdef"
			},
			wantErr: "direct_id",
		},
		{
			name: "Should reject terminal timestamps on active states",
			mutate: func(entry *NetworkWorkEntry) {
				entry.TerminalAt = &now
			},
			wantErr: "terminal_at",
		},
		{
			name: "Should reject unknown states",
			mutate: func(entry *NetworkWorkEntry) {
				entry.State = "paused"
			},
			wantErr: "state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entry := valid
			if tt.mutate != nil {
				tt.mutate(&entry)
			}
			err := entry.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestNetworkConversationQueryValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid summary limits", func(t *testing.T) {
		t.Parallel()

		if err := (NetworkThreadQuery{Limit: -1}).Validate(); err == nil || !strings.Contains(err.Error(), "limit") {
			t.Fatalf("Validate(thread query) error = %v, want limit rejection", err)
		}
		if err := (NetworkDirectRoomQuery{Limit: -1}).Validate(); err == nil ||
			!strings.Contains(err.Error(), "limit") {
			t.Fatalf("Validate(direct query) error = %v, want limit rejection", err)
		}
	})

	t.Run("Should normalize semantically equivalent conversation list queries", func(t *testing.T) {
		t.Parallel()

		thread := (NetworkThreadQuery{Search: "  Launch  ", Limit: 0}).Normalize()
		if thread.Search != "launch" || thread.Sort != NetworkConversationSortRecentActivity ||
			thread.Limit != NetworkConversationListDefaultLimit {
			t.Fatalf("Normalize(thread query) = %#v, want lowercase search and canonical defaults", thread)
		}
		direct := (NetworkDirectRoomQuery{Search: "  REVIEW  ", Limit: 999}).Normalize()
		if direct.Search != "review" || direct.Sort != NetworkConversationSortRecentActivity ||
			direct.Limit != NetworkConversationListMaxLimit {
			t.Fatalf("Normalize(direct query) = %#v, want lowercase search and capped limit", direct)
		}
	})

	t.Run("Should reject dual message cursors and invalid work ids", func(t *testing.T) {
		t.Parallel()

		query := NetworkConversationMessageQuery{
			BeforeMessageID: "msg_before",
			AfterMessageID:  "msg_after",
			Limit:           10,
		}
		if err := query.Validate(); err == nil || !strings.Contains(err.Error(), "both before and after") {
			t.Fatalf("Validate(message query) error = %v, want cursor rejection", err)
		}

		query.BeforeMessageID = ""
		query.WorkID = "work/bad"
		if err := query.Validate(); err == nil || !strings.Contains(err.Error(), "work_id") {
			t.Fatalf("Validate(message query) error = %v, want work_id rejection", err)
		}
	})
}

func TestNetworkConversationMessageValidation(t *testing.T) {
	t.Parallel()

	valid := NetworkConversationMessage{
		WorkspaceID: networkConversationTestWorkspaceID,
		MessageID:   "msg_patch_42",
		Channel:     "builders",
		Surface:     NetworkSurfaceDirect,
		DirectID:    "direct_0123456789abcdef0123456789abcdef",
		Direction:   "sent",
		PeerFrom:    "coder.sess-abc",
		PeerTo:      "reviewer.sess-xyz",
		Kind:        NetworkKindReceipt,
		WorkID:      "work_patch_42",
		PreviewText: "accepted",
		Body:        []byte(`{"status":"accepted"}`),
	}

	tests := []struct {
		name    string
		mutate  func(*NetworkConversationMessage)
		wantErr string
	}{
		{
			name: "Should accept conversation messages",
		},
		{
			name: "Should reject receipt without work",
			mutate: func(entry *NetworkConversationMessage) {
				entry.WorkID = ""
			},
			wantErr: "work_id",
		},
		{
			name: "Should reject greet with conversation fields",
			mutate: func(entry *NetworkConversationMessage) {
				entry.Kind = NetworkKindGreet
				entry.WorkID = ""
			},
			wantErr: "conversation",
		},
		{
			name: "Should reject messages without matching direct id",
			mutate: func(entry *NetworkConversationMessage) {
				entry.DirectID = ""
			},
			wantErr: "direct_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entry := valid
			if tt.mutate != nil {
				tt.mutate(&entry)
			}
			err := entry.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
