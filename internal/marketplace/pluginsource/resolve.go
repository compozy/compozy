package pluginsource

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/extension/agentplugin"
	"github.com/compozy/compozy/internal/registry"
)

// AcquisitionRecord contains source identity and immutable package evidence, never a temporary path.
type AcquisitionRecord struct {
	SourceRef    string `json:"source_ref"`
	EntryID      string `json:"entry_id"`
	ResolvedRef  string `json:"resolved_ref"`
	PackagePath  string `json:"package_path"`
	DigestSHA256 string `json:"digest_sha256"`
	Version      string `json:"version"`
	Layout       string `json:"layout"`
}

type Resolver struct {
	Cache   *PackageCache
	Sources Sources
}

// Resolve publishes canonical bytes before the caller releases the shared marketplace snapshot.
func (r *Resolver) Resolve(
	ctx context.Context,
	doc Document,
	snapshot *Snapshot,
	plugin Plugin,
) (record AcquisitionRecord, err error) {
	record, archive, err := r.prepare(ctx, doc, snapshot, plugin)
	if err != nil {
		return AcquisitionRecord{}, err
	}
	defer func() { err = errors.Join(err, archive.Close()) }()
	if err := r.Cache.Put(ctx, record.DigestSHA256, archive.file); err != nil {
		return AcquisitionRecord{}, err
	}
	return record, nil
}

func (r *Resolver) prepare(
	ctx context.Context,
	doc Document,
	snapshot *Snapshot,
	plugin Plugin,
) (record AcquisitionRecord, archive *capturedPackage, err error) {
	capacity, err := r.Cache.validate(ctx)
	if err != nil {
		return record, nil, err
	}
	if !pluginNamePattern.MatchString(plugin.Name) {
		return record, nil, errors.New("pluginsource: invalid plugin identity")
	}
	ref, err := NormalizeRef(doc.SourceRef)
	if err != nil || ref != doc.SourceRef {
		return record, nil, ErrInvalidRef
	}
	snapshot, owned, err := r.packageSnapshot(ctx, doc, snapshot, plugin.Source)
	if err != nil {
		return record, nil, err
	}
	defer func() {
		if owned {
			err = errors.Join(err, snapshot.Close())
		}
		if err != nil && archive != nil {
			err = errors.Join(err, archive.Close())
			archive = nil
		}
	}()
	packagePath, err := confinedPackagePath(snapshot.Root, plugin.Source.Path)
	if err != nil {
		return record, nil, err
	}
	archive, err = capturePackage(ctx, snapshot.Root, plugin.Source.Path, capacity, r.Sources.TempDir)
	if err != nil {
		return record, nil, err
	}
	layout, err := r.packageLayout(ctx, archive)
	if err != nil {
		return record, archive, err
	}
	relative, err := filepath.Rel(snapshot.Root, packagePath)
	if err != nil {
		return record, archive, err
	}
	resolved := snapshot.ResolvedRef
	if strings.HasPrefix(resolved, "file:") {
		resolved = doc.SourceRef + "@" + archive.digest
	} else if isArchiveRef(resolved) {
		resolved += "@" + archive.digest
	}
	version := strings.TrimSpace(plugin.Version)
	if version == "" {
		separator := strings.LastIndexByte(resolved, '@')
		if separator < 0 || len(resolved)-separator-1 < 12 {
			return record, archive, errors.New("pluginsource: package has no resolved revision")
		}
		version = resolved[separator+1 : separator+13]
	}
	return AcquisitionRecord{
		SourceRef: doc.SourceRef, EntryID: plugin.Name, ResolvedRef: resolved,
		PackagePath: filepath.ToSlash(relative), DigestSHA256: archive.digest, Version: version, Layout: layout,
	}, archive, nil
}

func (r *Resolver) packageSnapshot(
	ctx context.Context,
	doc Document,
	snapshot *Snapshot,
	source PluginSource,
) (*Snapshot, bool, error) {
	if source.Kind == SourceRelative {
		if snapshot == nil || snapshot.ResolvedRef != doc.ResolvedRef {
			return nil, false, errors.New("pluginsource: relative package requires the document snapshot")
		}
		return snapshot, false, nil
	}
	if source.Kind != SourceGitHub && source.Kind != SourceHTTPS {
		return nil, false, errors.New("unsupported_source")
	}
	snapshot, err := r.Sources.openPlugin(ctx, source)
	return snapshot, true, err
}

func (r *Resolver) packageLayout(ctx context.Context, archive *capturedPackage) (layout string, err error) {
	root, err := os.MkdirTemp(r.Sources.TempDir, "compozy-marketplace-inspect-*")
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(root)) }()
	download := &registry.DownloadResult{Reader: io.NopCloser(archive.file), ContentType: registry.TarContentType}
	if err := extractSnapshot(ctx, root, download); err != nil {
		return "", err
	}
	_, layout, err = agentplugin.LocateManifest(root)
	if missing, _ := errors.AsType[*agentplugin.NotManifestError](err); missing != nil {
		// Acquisition retains malformed packages so projection can show the loader's install blocker.
		layout, err = "", nil
	}
	_, seekErr := archive.file.Seek(0, io.SeekStart)
	return layout, errors.Join(err, seekErr)
}
