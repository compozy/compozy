package marketplace

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

type PluginSourceReader interface {
	Fetch(context.Context, string) (pluginsource.Document, error)
	OpenSnapshot(context.Context, pluginsource.Document) (*pluginsource.Snapshot, error)
}

var _ PluginSourceReader = (*pluginsource.Sources)(nil)

type PluginSource struct {
	config    ResolvedSource
	reader    PluginSourceReader
	projector *PluginProjector
}

var _ Source = (*PluginSource)(nil)

func NewPluginSource(
	config ResolvedSource,
	reader PluginSourceReader,
	projector *PluginProjector,
) (*PluginSource, error) {
	if err := pluginsource.ValidateName(config.Name); err != nil {
		return nil, err
	}
	ref, err := pluginsource.NormalizeRef(config.Ref)
	if err != nil {
		return nil, err
	}
	if config.Kind != SourceKindPreset && config.Kind != SourceKindCustom {
		return nil, errors.New("marketplace: plugin source must be preset or custom")
	}
	if reader == nil || projector == nil {
		return nil, errors.New("marketplace: plugin source reader and projector are required")
	}
	config.Ref = ref
	return &PluginSource{config: config, reader: reader, projector: projector}, nil
}

// Fetch owns one deadline and joins projection workers before releasing its shared snapshot.
func (s *PluginSource) Fetch(ctx context.Context) (_ *Document, err error) {
	if ctx == nil {
		return nil, errors.New("marketplace: source context is required")
	}
	workCtx, cancel := context.WithTimeout(ctx, s.projector.budget)
	defer cancel()
	defer func() {
		if err != nil && errors.Is(workCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			err = errors.Join(ErrRefreshBudgetExhausted, err)
		}
	}()
	doc, err := s.reader.Fetch(workCtx, s.config.Ref)
	if err != nil {
		return nil, err
	}
	if doc.SourceRef != s.config.Ref {
		return nil, errors.New("marketplace: fetched document origin does not match its source")
	}
	var snapshot *pluginsource.Snapshot
	for _, plugin := range doc.Plugins {
		if plugin.Source.Kind == pluginsource.SourceRelative {
			snapshot, err = s.reader.OpenSnapshot(workCtx, doc)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if snapshot != nil {
		defer func() { err = errors.Join(err, snapshot.Close()) }()
	}
	entries, diagnostics, err := s.projector.project(ctx, workCtx, doc, s.config.Name, snapshot)
	if err != nil {
		return nil, err
	}
	return &Document{
		ManifestVersion: ManifestVersion, SourceRef: doc.SourceRef, SourceKind: s.config.Kind,
		DocumentDigest: doc.DigestSHA256, DocumentPath: doc.Path, Owner: doc.Owner,
		Entries: entries, Diagnostics: diagnostics,
	}, nil
}
