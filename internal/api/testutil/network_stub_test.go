package testutil

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/internal/store"
)

func TestStubNetworkStoreForwardsReadScopes(t *testing.T) {
	t.Parallel()

	scope := store.ReadScope{ProfileID: "profile-marketing"}
	ref := store.NetworkChannelRef{WorkspaceID: "ws-1", Channel: "builders"}
	assertScope := func(t *testing.T, got store.ReadScope) {
		t.Helper()
		if got != scope {
			t.Fatalf("read scope = %#v, want %#v", got, scope)
		}
	}

	t.Run("Should forward channel scope", func(t *testing.T) {
		t.Parallel()
		stub := StubNetworkStore{
			GetNetworkChannelScopedFn: func(_ context.Context, got store.ReadScope, gotRef store.NetworkChannelRef) (store.NetworkChannelEntry, error) {
				assertScope(t, got)
				if gotRef != ref {
					t.Fatalf("channel ref = %#v, want %#v", gotRef, ref)
				}
				return store.NetworkChannelEntry{ProfileID: scope.ProfileID}, nil
			},
		}
		if _, err := stub.GetNetworkChannel(t.Context(), scope, ref); err != nil {
			t.Fatalf("GetNetworkChannel() error = %v", err)
		}
	})

	t.Run("Should forward conversation item scopes", func(t *testing.T) {
		t.Parallel()
		stub := StubNetworkStore{
			GetThreadScopedFn: func(_ context.Context, got store.ReadScope, gotRef store.NetworkChannelRef, gotID string) (store.NetworkThreadSummary, error) {
				assertScope(t, got)
				if gotRef != ref || gotID != "thread-1" {
					t.Fatalf("thread args = %#v/%q, want %#v/thread-1", gotRef, gotID, ref)
				}
				return store.NetworkThreadSummary{}, nil
			},
			GetDirectRoomScopedFn: func(_ context.Context, got store.ReadScope, gotRef store.NetworkChannelRef, gotID string) (store.NetworkDirectRoomSummary, error) {
				assertScope(t, got)
				if gotRef != ref || gotID != "direct-1" {
					t.Fatalf("direct args = %#v/%q, want %#v/direct-1", gotRef, gotID, ref)
				}
				return store.NetworkDirectRoomSummary{}, nil
			},
			GetWorkScopedFn: func(_ context.Context, got store.ReadScope, workspaceID, workID string) (store.NetworkWorkEntry, error) {
				assertScope(t, got)
				if workspaceID != ref.WorkspaceID || workID != "work-1" {
					t.Fatalf("work args = %q/%q, want %s/work-1", workspaceID, workID, ref.WorkspaceID)
				}
				return store.NetworkWorkEntry{}, nil
			},
		}
		if _, err := stub.GetThread(t.Context(), scope, ref, "thread-1"); err != nil {
			t.Fatalf("GetThread() error = %v", err)
		}
		if _, err := stub.GetDirectRoom(t.Context(), scope, ref, "direct-1"); err != nil {
			t.Fatalf("GetDirectRoom() error = %v", err)
		}
		if _, err := stub.GetWork(t.Context(), scope, ref.WorkspaceID, "work-1"); err != nil {
			t.Fatalf("GetWork() error = %v", err)
		}
	})

	t.Run("Should hide foreign profiles returned by legacy callbacks", func(t *testing.T) {
		t.Parallel()
		stub := StubNetworkStore{
			GetNetworkChannelFn: func(context.Context, store.NetworkChannelRef) (store.NetworkChannelEntry, error) {
				return store.NetworkChannelEntry{ProfileID: "profile-foreign"}, nil
			},
			GetThreadFn: func(context.Context, store.NetworkChannelRef, string) (store.NetworkThreadSummary, error) {
				return store.NetworkThreadSummary{ProfileID: "profile-foreign"}, nil
			},
			GetDirectRoomFn: func(context.Context, store.NetworkChannelRef, string) (store.NetworkDirectRoomSummary, error) {
				return store.NetworkDirectRoomSummary{ProfileID: "profile-foreign"}, nil
			},
			GetWorkFn: func(context.Context, string, string) (store.NetworkWorkEntry, error) {
				return store.NetworkWorkEntry{ProfileID: "profile-foreign"}, nil
			},
		}
		if _, err := stub.GetNetworkChannel(t.Context(), scope, ref); !errors.Is(err, store.ErrNetworkChannelNotFound) {
			t.Fatalf("GetNetworkChannel() error = %v, want ErrNetworkChannelNotFound", err)
		}
		if _, err := stub.GetThread(
			t.Context(),
			scope,
			ref,
			"thread-1",
		); !errors.Is(
			err,
			store.ErrNetworkConversationNotFound,
		) {
			t.Fatalf("GetThread() error = %v, want ErrNetworkConversationNotFound", err)
		}
		if _, err := stub.GetDirectRoom(
			t.Context(),
			scope,
			ref,
			"direct-1",
		); !errors.Is(
			err,
			store.ErrNetworkConversationNotFound,
		) {
			t.Fatalf("GetDirectRoom() error = %v, want ErrNetworkConversationNotFound", err)
		}
		if _, err := stub.GetWork(
			t.Context(),
			scope,
			ref.WorkspaceID,
			"work-1",
		); !errors.Is(
			err,
			store.ErrNetworkConversationNotFound,
		) {
			t.Fatalf("GetWork() error = %v, want ErrNetworkConversationNotFound", err)
		}
	})
}
