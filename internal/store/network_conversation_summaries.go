package store

import "time"

// NetworkThreadSummary is the list/detail projection for a public thread.
type NetworkThreadSummary struct {
	WorkspaceID           string
	Channel               string
	ThreadID              string
	RootMessageID         string
	Title                 string
	OpenedByPeerID        string
	OpenedSessionID       string
	OpenedAt              time.Time
	OpenedSequence        int64
	LastActivityAt        time.Time
	LastActivitySequence  int64
	MessageCount          int
	ParticipantCount      int
	OpenWorkCount         int
	DeliveredCount        int64
	PromptSizeBytes       int64
	EstimatedPromptTokens int64
	LastMessagePreview    string
}

// NetworkDirectRoomSummary is the list/detail projection for a direct room.
type NetworkDirectRoomSummary struct {
	WorkspaceID          string
	Channel              string
	DirectID             string
	SessionA             string
	SessionB             string
	OpenedAt             time.Time
	OpenedSequence       int64
	LastActivityAt       time.Time
	LastActivitySequence int64
	MessageCount         int
	OpenWorkCount        int
	LastMessagePreview   string
}
