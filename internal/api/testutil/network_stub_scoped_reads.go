package testutil

import (
	"context"
	"database/sql"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (s StubNetworkStore) GetNetworkChannel(
	ctx context.Context,
	readScope store.ReadScope,
	ref store.NetworkChannelRef,
) (store.NetworkChannelEntry, error) {
	if s.GetNetworkChannelScopedFn != nil {
		return s.GetNetworkChannelScopedFn(ctx, readScope, ref)
	}
	if s.GetNetworkChannelFn == nil {
		return store.NetworkChannelEntry{}, sql.ErrNoRows
	}
	entry, err := s.GetNetworkChannelFn(ctx, ref)
	if err != nil {
		return store.NetworkChannelEntry{}, err
	}
	entry.ProfileID = normalizeStubNetworkProfileID(entry.ProfileID)
	if !readScope.Matches(entry.ProfileID) {
		return store.NetworkChannelEntry{}, store.ErrNetworkChannelNotFound
	}
	return entry, nil
}

func (s StubNetworkStore) GetThread(
	ctx context.Context,
	readScope store.ReadScope,
	ref store.NetworkChannelRef,
	threadID string,
) (store.NetworkThreadSummary, error) {
	if s.GetThreadScopedFn != nil {
		return s.GetThreadScopedFn(ctx, readScope, ref, threadID)
	}
	if s.GetThreadFn == nil {
		return store.NetworkThreadSummary{}, store.ErrNetworkConversationNotFound
	}
	entry, err := s.GetThreadFn(ctx, ref, threadID)
	if err != nil {
		return store.NetworkThreadSummary{}, err
	}
	entry.ProfileID = normalizeStubNetworkProfileID(entry.ProfileID)
	if !readScope.Matches(entry.ProfileID) {
		return store.NetworkThreadSummary{}, store.ErrNetworkConversationNotFound
	}
	return entry, nil
}

func (s StubNetworkStore) GetDirectRoom(
	ctx context.Context,
	readScope store.ReadScope,
	ref store.NetworkChannelRef,
	directID string,
) (store.NetworkDirectRoomSummary, error) {
	if s.GetDirectRoomScopedFn != nil {
		return s.GetDirectRoomScopedFn(ctx, readScope, ref, directID)
	}
	if s.GetDirectRoomFn == nil {
		return store.NetworkDirectRoomSummary{}, store.ErrNetworkConversationNotFound
	}
	entry, err := s.GetDirectRoomFn(ctx, ref, directID)
	if err != nil {
		return store.NetworkDirectRoomSummary{}, err
	}
	entry.ProfileID = normalizeStubNetworkProfileID(entry.ProfileID)
	if !readScope.Matches(entry.ProfileID) {
		return store.NetworkDirectRoomSummary{}, store.ErrNetworkConversationNotFound
	}
	return entry, nil
}

func (s StubNetworkStore) GetWork(
	ctx context.Context,
	readScope store.ReadScope,
	workspaceID string,
	workID string,
) (store.NetworkWorkEntry, error) {
	if s.GetWorkScopedFn != nil {
		return s.GetWorkScopedFn(ctx, readScope, workspaceID, workID)
	}
	if s.GetWorkFn == nil {
		return store.NetworkWorkEntry{}, store.ErrNetworkConversationNotFound
	}
	entry, err := s.GetWorkFn(ctx, workspaceID, workID)
	if err != nil {
		return store.NetworkWorkEntry{}, err
	}
	entry.ProfileID = normalizeStubNetworkProfileID(entry.ProfileID)
	if !readScope.Matches(entry.ProfileID) {
		return store.NetworkWorkEntry{}, store.ErrNetworkConversationNotFound
	}
	return entry, nil
}

func normalizeStubNetworkProfileID(profileID string) string {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return store.DefaultProfileID
	}
	return profileID
}
