package sessionid_test

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/session/sessionid"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subagentid"
)

func TestIdentityShapesAcceptGeneratedIdentities(t *testing.T) {
	t.Parallel()

	newID := func(t *testing.T, prefix string) string {
		t.Helper()
		id, err := store.NewID(prefix)
		if err != nil {
			t.Fatalf("store.NewID(%q) error = %v", prefix, err)
		}
		return id
	}

	t.Run("Should accept generated, subagent-derived, and Loop binding session ids", func(t *testing.T) {
		t.Parallel()

		parent := newID(t, "sess")
		// Mirrors session.subagentChildSessionID.
		subagent := "sess-" + strings.TrimPrefix(subagentid.Derive(parent, "tool-call-1"), "sub-")
		_, binding := goal.DeriveBindingIdentity(goal.TurnKey{
			LoopRunID: "looprun-0123456789abcdef", WorkspaceID: "ws_0123456789abcdef", Generation: 2, NodeID: "build",
		}, "handle", 3)
		for _, id := range []string{parent, subagent, binding} {
			if !sessionid.IsSessionID(id) {
				t.Fatalf("IsSessionID(%q) = false, want true", id)
			}
		}
	})

	t.Run("Should accept generated and restart continuation turn ids", func(t *testing.T) {
		t.Parallel()

		turnID := newID(t, "turn")
		// Mirrors the restart continuation turn id: "turn-" + session.restartEventID(turnID, "continue").
		digest := sha256.Sum256([]byte(turnID + ":continue"))
		continuation := "turn-ev_restart_" + hex.EncodeToString(digest[:16])
		for _, id := range []string{turnID, continuation} {
			if !sessionid.IsTurnID(id) {
				t.Fatalf("IsTurnID(%q) = false, want true", id)
			}
		}
	})

	t.Run("Should accept the daemon task claim handoff and role activation turn ids", func(t *testing.T) {
		t.Parallel()

		// Mirrors daemon.taskClaimHandoffTurnID and daemon.taskRolePromptTurnID: 16 bytes of a sha256 digest.
		digest := sha256.Sum256([]byte(newID(t, "sess") + "\x00" + "run_01ARZ3NDEKTSV4RRFFQ69G5FAV"))
		for _, prefix := range []string{"task-claim-handoff-", "task-role-activation-"} {
			if id := prefix + hex.EncodeToString(digest[:16]); !sessionid.IsTurnID(id) {
				t.Fatalf("IsTurnID(%q) = false, want true", id)
			}
		}
	})

	t.Run("Should accept generated run ids", func(t *testing.T) {
		t.Parallel()

		if id := newID(t, "run"); !sessionid.IsRunID(id) {
			t.Fatalf("IsRunID(%q) = false, want true", id)
		}
	})

	t.Run("Should reject free text and foreign identity shapes", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{
			"", "my session", "/home/ana/sess-0123456789abcdef", "sess-0123456789ABCDEF", " sess-0123456789abcdef",
			"turn-0123456789abcdef",
		} {
			if sessionid.IsSessionID(value) {
				t.Fatalf("IsSessionID(%q) = true, want false", value)
			}
		}
		for _, value := range []string{
			"turn_story", "turn-ev_restart_short", "sess-0123456789abcdef", "task-role-activation-0123456789abcdef",
		} {
			if sessionid.IsTurnID(value) {
				t.Fatalf("IsTurnID(%q) = true, want false", value)
			}
		}
		if sessionid.IsRunID("run_01ARZ3NDEKTSV4RRFFQ69G5FAV") {
			t.Fatal("IsRunID(task run ULID) = true, want false")
		}
	})

	// Exporting an identity is exporting its text, so a value that merely contains a canonical shape (a path, a
	// sentence, a longer hex run) must not pass for one.
	t.Run("Should reject a canonical identity embedded in other text", func(t *testing.T) {
		t.Parallel()

		validators := []struct {
			name  string
			valid func(string) bool
			ids   []string
		}{
			{
				name:  "session",
				valid: sessionid.IsSessionID,
				ids:   []string{"sess-0123456789abcdef", "sess_0123456789abcdef0123456789abcdef"},
			},
			{
				name:  "turn",
				valid: sessionid.IsTurnID,
				ids: []string{
					"turn-0123456789abcdef",
					"turn-ev_restart_0123456789abcdef0123456789abcdef",
					"task-claim-handoff-0123456789abcdef0123456789abcdef",
					"task-role-activation-0123456789abcdef0123456789abcdef",
				},
			},
			{name: "run", valid: sessionid.IsRunID, ids: []string{"run-0123456789abcdef"}},
		}
		for _, tc := range validators {
			for _, id := range tc.ids {
				if !tc.valid(id) {
					t.Fatalf("%s validator(%q) = false, want the canonical shape accepted", tc.name, id)
				}
				for _, embedded := range []string{"/home/ana/" + id, id + "/home/ana", id + "0", "x" + id} {
					if tc.valid(embedded) {
						t.Fatalf("%s validator(%q) = true, want false for a canonical identity inside other text",
							tc.name, embedded)
					}
				}
			}
		}
	})
}
