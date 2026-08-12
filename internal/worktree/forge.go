package worktree

import "context"

type ForgeCapabilities struct {
	Provider           string
	RequestNoun        string
	OpenActionLabel    string
	ViewActionLabel    string
	SupportsDraft      bool
	CompareURLTemplate string
	CredentialSource   string
}

type ForgeStatusRequest struct {
	WorkspaceID string
	WorktreeID  string
	RemoteURLs  []string
	Branch      string
}

type ForgePRRequest struct {
	WorkspaceID string
	WorktreeID  string
	RemoteURLs  []string
	Head        string
	Base        string
	Title       string
	Body        string
	Draft       bool
}

type ForgePRResult struct {
	Status string
	Number int
	URL    string
}

type ForgeProvider interface {
	Capabilities(context.Context, []string) (*ForgeCapabilities, error)
	Status(context.Context, ForgeStatusRequest) (*ForgeStatus, error)
	CreatePR(context.Context, ForgePRRequest) (*ForgePRResult, error)
}
