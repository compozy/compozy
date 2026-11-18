package changes

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/projectfiles"
)

type Scope string

const (
	Branch      Scope = "branch"
	Uncommitted Scope = "uncommitted"
)

type Request struct {
	Scope             Scope
	BaseRef, Revision string
	Reveal            bool
}
type BaseSource string
type Status string

const Conflicted Status = "conflicted"

type Summary struct {
	Scope                        Scope
	HeadBranch, HeadSHA, BaseRef string
	BaseSource                   BaseSource
	MergeBaseSHA                 string
	Files                        []FileChange
	Totals                       Totals
	Truncated                    bool
	Revision                     string
}
type FileChange struct {
	oldOID                       string
	Path, OldPath                string
	Status                       Status
	Additions, Deletions         int
	Binary, Generated, Sensitive bool
	Fingerprint                  string
}
type Totals struct {
	Files, Additions, Deletions int
	Partial                     bool
}
type Patch struct {
	Path, Patch         string
	TooLarge, Sensitive bool
	Fingerprint         string
}
type PatchTarget struct{ Path, OldPath string }
type Contents struct {
	Path, Fingerprint, OldPath string
	Old, New                   *Side
}
type Side struct {
	Text      string
	Sensitive bool
}
type BaseList struct {
	Default    *BaseRef
	Resolvable bool
	Refs       []BaseRef
	Truncated  bool
}
type BaseRef struct {
	Ref, Kind string
	IsDefault bool
	Source    BaseSource
}
type Engine interface {
	Summary(context.Context, projectfiles.Root, Request) (Summary, error)
	Patches(context.Context, projectfiles.Root, Request, []PatchTarget) ([]Patch, error)
	Contents(context.Context, projectfiles.Root, Request, PatchTarget) (Contents, error)
	Bases(context.Context, projectfiles.Root, string, int) (BaseList, error)
}

var (
	ErrNotARepository   = errors.New("changes: not a git repository")
	ErrBaseNotFound     = errors.New("changes: base ref not found")
	ErrStale            = errors.New("changes: working tree changed since revision")
	ErrTooLargeToExpand = errors.New("changes: file too large to expand")
	ErrInvalidScope     = errors.New("changes: invalid scope")
	ErrTooManyPaths     = errors.New("changes: patch batch exceeds limits")
	ErrUnavailable      = errors.New("changes: git read failed")
)
