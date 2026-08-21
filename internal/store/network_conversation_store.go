package store

import "context"

// NetworkConversationStore manages durable conversation containers and work rows.
type NetworkConversationStore interface {
	ResolveDirectRoom(ctx context.Context, entry NetworkDirectRoomEntry) (NetworkDirectRoomSummary, error)
	WriteConversationMessage(
		ctx context.Context,
		entry NetworkConversationMessage,
	) (NetworkConversationWriteResult, error)
	ListThreads(ctx context.Context, ref NetworkChannelRef, query NetworkThreadQuery) (NetworkThreadPage, error)
	GetThread(
		ctx context.Context,
		readScope ReadScope,
		ref NetworkChannelRef,
		threadID string,
	) (NetworkThreadSummary, error)
	ListDirectRooms(
		ctx context.Context,
		ref NetworkChannelRef,
		query NetworkDirectRoomQuery,
	) (NetworkDirectRoomPage, error)
	GetDirectRoom(
		ctx context.Context,
		readScope ReadScope,
		ref NetworkChannelRef,
		directID string,
	) (NetworkDirectRoomSummary, error)
	ListConversationMessages(
		ctx context.Context,
		ref NetworkConversationRef,
		query NetworkConversationMessageQuery,
	) ([]NetworkConversationMessage, error)
	GetWork(ctx context.Context, readScope ReadScope, workspaceID string, workID string) (NetworkWorkEntry, error)
}
