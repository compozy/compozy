package projectfiles

import (
	"context"
	"errors"
)

type RootKind string

const (
	RootKindWorkspace RootKind = "workspace"
	RootKindWorktree  RootKind = "worktree"
)

type RootRef struct{ WorkspaceID, WorktreeID, SessionID string }
type Root struct {
	Kind                                                    RootKind
	WorkspaceID, WorktreeID, Name, Branch, BaseRef, AbsPath string
}
type RootResolver interface {
	Resolve(context.Context, RootRef) (Root, error)
}
type EntryKind string

const (
	EntryKindFile      EntryKind = "file"
	EntryKindDirectory EntryKind = "directory"
)

type GitStatus string
type Entry struct {
	Name, Path                  string
	Kind                        EntryKind
	Size                        int64
	Symlink, Ignored, Sensitive bool
	GitStatus                   GitStatus
	ContainsChanges             bool
}
type Listing struct {
	Root                         Root
	Path                         string
	Entries                      []Entry
	Truncated, StatusUnavailable bool
}
type ContentKind string

const (
	ContentKindText      ContentKind = "text"
	ContentKindImage     ContentKind = "image"
	ContentKindBinary    ContentKind = "binary"
	ContentKindSensitive ContentKind = "sensitive"
)

type Content struct {
	Path                      string
	Kind                      ContentKind
	Size                      int64
	Language, Text, ImageMIME string
	ImageData                 []byte
	Sensitive, Truncated      bool
	Revision                  string
}
type Reader interface {
	List(context.Context, Root, string, ListOptions) (Listing, error)
	Read(context.Context, Root, string, ReadOptions) (Content, error)
}
type ListOptions struct{ ShowIgnored, Refresh bool }
type ReadOptions struct{ Reveal bool }

var (
	ErrInvalidPath     = errors.New("projectfiles: invalid path")
	ErrPathOutsideRoot = errors.New("projectfiles: path outside root")
	ErrPathDenied      = errors.New("projectfiles: path denied")
	ErrNotFound        = errors.New("projectfiles: not found")
	ErrNotADirectory   = errors.New("projectfiles: not a directory")
	ErrNotAFile        = errors.New("projectfiles: not a file")
	ErrRootUnavailable = errors.New("projectfiles: root unavailable")
	ErrRootNotFound    = errors.New("projectfiles: root not found")
)
