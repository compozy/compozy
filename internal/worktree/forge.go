package worktree

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type ForgeCapabilities struct {
	Provider           string
	Winner             string
	RequestNoun        string
	OpenActionLabel    string
	ViewActionLabel    string
	SupportsDraft      bool
	CompareURLTemplate string
	CredentialSource   string
	DefaultBranch      string
	TemplatePaths      []string
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

type ForgeFailure struct {
	kind  error
	cause string
	err   error
}

func NewForgeFailure(kind error, cause string, err error) error {
	return &ForgeFailure{kind: kind, cause: strings.TrimSpace(cause), err: err}
}

func (e *ForgeFailure) Error() string {
	if e == nil {
		return ErrForge.Error()
	}
	kind := e.kind
	if kind == nil {
		kind = ErrForge
	}
	if e.cause != "" {
		return fmt.Sprintf("%s: %s", kind, e.cause)
	}
	if e.err != nil {
		return fmt.Sprintf("%s: %s", kind, e.err)
	}
	return kind.Error()
}

func (e *ForgeFailure) Unwrap() []error {
	if e == nil {
		return nil
	}
	causes := make([]error, 0, 2)
	if e.kind != nil {
		causes = append(causes, e.kind)
	}
	if e.err != nil {
		causes = append(causes, e.err)
	}
	return causes
}

func ForgeFailureCause(err error) string {
	var failure *ForgeFailure
	if errors.As(err, &failure) {
		return failure.cause
	}
	return ""
}
