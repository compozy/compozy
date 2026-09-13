package pluginsource

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/registry"
)

type Snapshot struct {
	Root        string
	ResolvedRef string
	release     func() error
}

var _ io.Closer = (*Snapshot)(nil)

func (s *Snapshot) Close() error {
	return s.release()
}

// OpenSnapshot acquires the document's exact repository revision once for all relative plugins.
func (s *GitHubSource) OpenSnapshot(ctx context.Context, document Document, tempDir string) (_ *Snapshot, err error) {
	if ctx == nil {
		return nil, errors.New("pluginsource: context is required")
	}
	commit, err := repositoryCommit(document, s.ref)
	if err != nil {
		return nil, err
	}
	download, err := s.client.DownloadRevision(ctx, s.repo, commit, registry.DefaultMaxArchiveSize)
	if err != nil {
		return nil, remoteSourceError(err)
	}
	readerOwned := true
	defer func() {
		if readerOwned {
			err = errors.Join(err, download.Reader.Close())
		}
	}()
	root, err := os.MkdirTemp(tempDir, "compozy-marketplace-checkout-*")
	if err != nil {
		return nil, err
	}
	snapshot := &Snapshot{
		ResolvedRef: document.ResolvedRef,
		release:     sync.OnceValue(func() error { return os.RemoveAll(root) }),
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, snapshot.Close())
		}
	}()
	if err := extractSnapshot(ctx, root, download); err != nil {
		return nil, err
	}
	readerOwned = false
	if err := download.Reader.Close(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil, errors.New("pluginsource: repository archive must contain one root directory")
	}
	snapshot.Root = filepath.Join(root, entries[0].Name())
	if err := validateSnapshotDocument(ctx, snapshot.Root, document); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func repositoryCommit(document Document, sourceRef string) (string, error) {
	if document.SourceRef != sourceRef {
		return "", errors.New("pluginsource: document origin does not match its source")
	}
	commit, ok := strings.CutPrefix(document.ResolvedRef, sourceRef+"@")
	if !ok {
		return "", errors.New("pluginsource: document has no resolved repository revision")
	}
	decoded, err := hex.DecodeString(commit)
	if err != nil || (len(decoded) != 20 && len(decoded) != 32) || hex.EncodeToString(decoded) != commit {
		return "", errors.New("pluginsource: repository revision must be a full lowercase commit hash")
	}
	return commit, nil
}

func validateSnapshotDocument(ctx context.Context, root string, document Document) error {
	captured, err := ReadDirectory(ctx, root)
	if err != nil {
		return err
	}
	if captured.DigestSHA256 != document.DigestSHA256 || captured.Path != document.Path {
		return errors.New("pluginsource: repository snapshot differs from the fetched marketplace document")
	}
	return nil
}

func extractSnapshot(ctx context.Context, path string, download *registry.DownloadResult) (err error) {
	root, err := fileutil.OpenDirectoryForMutation(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	if err := registry.ExtractArchive(
		ctx,
		download.Reader,
		root,
		registry.ExtractionLimits{},
		download.ContentType,
	); err != nil {
		return fmt.Errorf("pluginsource: extract repository snapshot: %w", err)
	}
	return nil
}
