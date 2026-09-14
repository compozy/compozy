package pluginsource

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/registry/gitsrc"
)

type GitSource struct {
	ref        string
	repository string
	options    []gitsrc.Option
}

func NewGitSource(ref string, options ...gitsrc.Option) (*GitSource, error) {
	normalized, err := NormalizeRef(ref)
	if err != nil {
		return nil, err
	}
	repository, ok := strings.CutPrefix(normalized, "git+")
	if !ok {
		return nil, ErrInvalidRef
	}
	return &GitSource{ref: normalized, repository: repository, options: slices.Clone(options)}, nil
}

func (s *GitSource) Fetch(ctx context.Context) (document Document, err error) {
	checkout, err := s.checkout(ctx, "", "")
	if err != nil {
		return Document{}, remoteSourceError(err)
	}
	defer func() { err = errors.Join(err, checkout.Close()) }()
	document, err = ReadDirectory(ctx, checkout.Path)
	if err != nil {
		return Document{}, err
	}
	document.SourceRef, document.ResolvedRef = s.ref, s.ref+"@"+checkout.Commit
	return document, nil
}

func (s *GitSource) OpenSnapshot(ctx context.Context, document Document, tempDir string) (_ *Snapshot, err error) {
	commit, err := repositoryCommit(document, s.ref)
	if err != nil {
		return nil, err
	}
	checkout, err := s.checkout(ctx, commit, tempDir)
	if err != nil {
		return nil, remoteSourceError(err)
	}
	snapshot := &Snapshot{Root: checkout.Path, ResolvedRef: s.ref + "@" + checkout.Commit, release: checkout.Close}
	defer func() {
		if err != nil {
			err = errors.Join(err, snapshot.Close())
		}
	}()
	if err := validateSnapshotDocument(ctx, snapshot.Root, document); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *GitSource) checkout(ctx context.Context, ref, tempDir string) (*gitsrc.Checkout, error) {
	options := slices.Concat(s.options, []gitsrc.Option{gitsrc.WithTimeout(fetchTimeout)})
	if tempDir != "" {
		options = append(options, gitsrc.WithCheckoutTempDir(tempDir))
	}
	return gitsrc.NewClient(options...).Checkout(ctx, s.repository, ref)
}
