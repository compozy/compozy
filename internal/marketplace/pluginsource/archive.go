package pluginsource

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/extension/agentplugin"
	"github.com/compozy/compozy/internal/registry"
	"github.com/compozy/compozy/internal/registry/gitsrc"
)

func isArchiveRef(ref string) bool {
	parsed, err := url.Parse(ref)
	if err != nil {
		return false
	}
	path := strings.ToLower(parsed.Path)
	return strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz") || strings.HasSuffix(path, ".tar")
}

func (s *Sources) openArchive(ctx context.Context, ref string) (_ *Snapshot, err error) {
	if err := gitsrc.ValidateRepositoryRef(ref); err != nil {
		return nil, err
	}
	client := http.Client{Timeout: fetchTimeout}
	if s.HTTPClient != nil {
		client = *s.HTTPClient
		client.Timeout, client.Jar = fetchTimeout, nil
	}
	downloader, err := registry.NewHTTPArchiveDownloader(ref, "source", &client)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	download, err := downloader.Download(ctx, ref, registry.DownloadOpts{})
	if err != nil {
		return nil, remoteSourceError(err)
	}
	var snapshot *Snapshot
	defer func() {
		err = errors.Join(err, download.Reader.Close())
		if err != nil && snapshot != nil {
			err = errors.Join(err, snapshot.Close())
		}
	}()
	root, err := os.MkdirTemp(s.TempDir, "compozy-marketplace-archive-*")
	if err != nil {
		return nil, err
	}
	snapshot = &Snapshot{
		Root:        root,
		ResolvedRef: ref,
		release:     sync.OnceValue(func() error { return os.RemoveAll(root) }),
	}
	bounded := &io.LimitedReader{R: download.Reader, N: registry.DefaultMaxArchiveSize + 1}
	archive := *download
	archive.Reader = io.NopCloser(bounded)
	if err := extractSnapshot(ctx, root, &archive); err != nil {
		return nil, err
	}
	if _, err := io.Copy(io.Discard, bounded); err != nil {
		return nil, err
	}
	if bounded.N == 0 {
		return nil, registry.ErrArchiveTooLargeCompressed
	}
	snapshot.Root, err = archivePackageRoot(root)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func archivePackageRoot(root string) (string, error) {
	if _, _, err := agentplugin.LocateManifest(root); err == nil {
		return root, nil
	} else if missing, _ := errors.AsType[*agentplugin.NotManifestError](err); missing == nil {
		return "", err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(root, entries[0].Name()), nil
	}
	return root, nil
}
