package pluginsource

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/registry/gitsrc"
)

// Sources owns public acquisition settings. Configure it before concurrent source refreshes.
type Sources struct {
	GitHubOptions []GitHubOption
	GitOptions    []gitsrc.Option
	HTTPClient    *http.Client
	TempDir       string
}

func (s *Sources) Fetch(ctx context.Context, ref string) (document Document, err error) {
	normalized, err := NormalizeRef(ref)
	if err != nil {
		return Document{}, err
	}
	switch {
	case strings.HasPrefix(normalized, "github:"):
		source, err := NewGitHubSource(normalized, s.GitHubOptions...)
		if err != nil {
			return Document{}, err
		}
		defer func() { err = errors.Join(err, source.Close()) }()
		return source.Fetch(ctx)
	case strings.HasPrefix(normalized, "git+"):
		source, err := NewGitSource(normalized, s.GitOptions...)
		if err != nil {
			return Document{}, err
		}
		return source.Fetch(ctx)
	default:
		source, err := NewDirectorySource(normalized)
		if err != nil {
			return Document{}, err
		}
		return source.Fetch(ctx)
	}
}

// OpenSnapshot retains the document's revision for every relative package in a refresh.
func (s *Sources) OpenSnapshot(ctx context.Context, document Document) (_ *Snapshot, err error) {
	switch {
	case strings.HasPrefix(document.SourceRef, "github:"):
		source, err := NewGitHubSource(document.SourceRef, s.GitHubOptions...)
		if err != nil {
			return nil, err
		}
		snapshot, openErr := source.OpenSnapshot(ctx, document, s.TempDir)
		return closeSnapshotSource(snapshot, errors.Join(openErr, source.Close()))
	case strings.HasPrefix(document.SourceRef, "git+"):
		source, err := NewGitSource(document.SourceRef, s.GitOptions...)
		if err != nil {
			return nil, err
		}
		return source.OpenSnapshot(ctx, document, s.TempDir)
	default:
		source, err := NewDirectorySource(document.SourceRef)
		if err != nil {
			return nil, err
		}
		return source.OpenSnapshot(ctx, document)
	}
}

func closeSnapshotSource(snapshot *Snapshot, err error) (*Snapshot, error) {
	if err != nil && snapshot != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	return snapshot, err
}

func (s *Sources) openPlugin(ctx context.Context, plugin PluginSource) (_ *Snapshot, err error) {
	if plugin.Kind == SourceHTTPS && isArchiveRef(plugin.Ref) {
		return s.openArchive(ctx, plugin.Ref)
	}
	ref := plugin.Ref
	if plugin.Kind == SourceHTTPS {
		ref = "git+" + ref
	}
	normalized, err := NormalizeRef(ref)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(normalized, "github:") {
		source, err := NewGitHubSource(normalized, s.GitHubOptions...)
		if err != nil {
			return nil, err
		}
		ref := plugin.GitRef
		if ref == "" {
			ref = "HEAD"
		}
		commit, resolveErr := source.client.ResolveCommit(ctx, source.repo, ref)
		if resolveErr != nil {
			return nil, errors.Join(remoteSourceError(resolveErr), source.Close())
		}
		snapshot, openErr := source.openRevision(ctx, commit, s.TempDir)
		return closeSnapshotSource(snapshot, errors.Join(openErr, source.Close()))
	}
	source, err := NewGitSource(normalized, s.GitOptions...)
	if err != nil {
		return nil, err
	}
	checkout, err := source.checkout(ctx, plugin.GitRef, s.TempDir)
	if err != nil {
		return nil, remoteSourceError(err)
	}
	return &Snapshot{Root: checkout.Path, ResolvedRef: normalized + "@" + checkout.Commit, release: checkout.Close}, nil
}
