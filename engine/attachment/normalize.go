package attachment

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

// NormalizePhase1 expands plural sources and evaluates templates, deferring
// unresolved .tasks.* expressions. Returns a new slice; input is not modified.
func NormalizePhase1(
	ctx context.Context,
	eng *tplengine.TemplateEngine,
	cwd *core.PathCWD,
	atts []Attachment,
	tplCtx map[string]any,
) ([]Attachment, error) {
	if len(atts) == 0 {
		return nil, nil
	}
	out := make([]Attachment, 0, len(atts))
	for i, a := range atts {
		items, parent, err := processAttachment(ctx, eng, cwd, tplCtx, a)
		if err != nil {
			return nil, fmt.Errorf("attachment[%d]: %w", i, err)
		}
		out = append(out, items...)
		if parent != nil {
			out = append(out, parent)
		}
	}
	return out, nil
}

// processAttachment normalizes a single attachment and returns expanded items
// plus an optional parent with deferred sources for phase 2.
func processAttachment(
	ctx context.Context,
	eng *tplengine.TemplateEngine,
	cwd *core.PathCWD,
	tplCtx map[string]any,
	a Attachment,
) ([]Attachment, Attachment, error) {
	switch v := a.(type) {
	case *ImageAttachment:
		return expandMulti(ctx, eng, cwd, &params{
			base:   v.baseAttachment,
			src:    v.Source,
			url:    v.URL,
			path:   v.Path,
			urls:   v.URLs,
			paths:  v.Paths,
			kind:   TypeImage,
			tplCtx: tplCtx,
		})
	case *PDFAttachment:
		items, parent, err := expandMulti(ctx, eng, cwd, &params{
			base:   v.baseAttachment,
			src:    v.Source,
			url:    v.URL,
			path:   v.Path,
			urls:   v.URLs,
			paths:  v.Paths,
			kind:   TypePDF,
			tplCtx: tplCtx,
		})
		if err != nil {
			return nil, nil, err
		}
		for _, it := range items {
			if p, ok := it.(*PDFAttachment); ok {
				p.MaxPages = v.MaxPages
			}
		}
		if parentPDF, ok := parent.(*PDFAttachment); ok {
			parentPDF.MaxPages = v.MaxPages
		}
		return items, parent, nil
	case *AudioAttachment:
		return expandMulti(ctx, eng, cwd, &params{
			base:   v.baseAttachment,
			src:    v.Source,
			url:    v.URL,
			path:   v.Path,
			urls:   v.URLs,
			paths:  v.Paths,
			kind:   TypeAudio,
			tplCtx: tplCtx,
		})
	case *VideoAttachment:
		return expandMulti(ctx, eng, cwd, &params{
			base:   v.baseAttachment,
			src:    v.Source,
			url:    v.URL,
			path:   v.Path,
			urls:   v.URLs,
			paths:  v.Paths,
			kind:   TypeVideo,
			tplCtx: tplCtx,
		})
	case *URLAttachment:
		s, _, err := applyTemplateString(eng, tplCtx, v.URL)
		if err != nil {
			return nil, nil, fmt.Errorf("url template: %w", err)
		}
		return []Attachment{&URLAttachment{baseAttachment: v.baseAttachment, URL: s}}, nil, nil
	case *FileAttachment:
		s, _, err := applyTemplateString(eng, tplCtx, v.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("file template: %w", err)
		}
		return []Attachment{&FileAttachment{baseAttachment: v.baseAttachment, Path: s}}, nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported attachment type %T", a)
	}
}

// NormalizePhase2 finalizes any deferred templates from phase 1 and expands remaining plural sources.
func NormalizePhase2(
	ctx context.Context,
	eng *tplengine.TemplateEngine,
	cwd *core.PathCWD,
	atts []Attachment,
	tplCtx map[string]any,
) ([]Attachment, error) {
	// Re-run the same logic; any previously deferred templates should now resolve and expand.
	return NormalizePhase1(ctx, eng, cwd, atts, tplCtx)
}

// params holds parameters for the expandMulti function.
type params struct {
	base   baseAttachment
	src    Source
	url    string
	path   string
	urls   []string
	paths  []string
	kind   Type
	tplCtx map[string]any
}

// expandMulti applies base templating and dispatches to URL/Path expansion.
func expandMulti(
	ctx context.Context,
	eng *tplengine.TemplateEngine,
	cwd *core.PathCWD,
	a *params,
) ([]Attachment, Attachment, error) {
	if err := applyTemplateOnBase(eng, a.tplCtx, &a.base); err != nil {
		return nil, nil, err
	}
	switch a.src {
	case SourceURL:
		return expandURLSource(eng, a.base, a.url, a.urls, a.kind, a.tplCtx)
	case SourcePath:
		return expandPathSource(ctx, eng, cwd, a.base, a.path, a.paths, a.kind, a.tplCtx)
	default:
		return nil, nil, fmt.Errorf("unknown source %s", a.src)
	}
}

// expandURLSource expands URL and URLs fields and defers unresolved entries.
func expandURLSource(
	eng *tplengine.TemplateEngine,
	base baseAttachment,
	url string,
	urls []string,
	kind Type,
	tplCtx map[string]any,
) ([]Attachment, Attachment, error) {
	if url != "" {
		us, _, err := applyTemplateString(eng, tplCtx, url)
		if err != nil {
			return nil, nil, err
		}
		return []Attachment{newURLItem(kind, base, us)}, nil, nil
	}
	if len(urls) == 0 {
		return nil, nil, fmt.Errorf("invalid attachment state for normalization")
	}
	resolved, deferred, err := splitResolvedURLs(eng, tplCtx, urls)
	if err != nil {
		return nil, nil, err
	}
	items := buildURLItems(kind, base, resolved)
	if len(deferred) > 0 {
		return items, newURLParent(kind, base, deferred), nil
	}
	return items, nil, nil
}

// expandPathSource expands Path and Paths patterns with doublestar and CWD checks.
func expandPathSource(
	ctx context.Context,
	eng *tplengine.TemplateEngine,
	cwd *core.PathCWD,
	base baseAttachment,
	path string,
	paths []string,
	kind Type,
	tplCtx map[string]any,
) ([]Attachment, Attachment, error) {
	if path != "" {
		ps, _, err := applyTemplateString(eng, tplCtx, path)
		if err != nil {
			return nil, nil, err
		}
		return []Attachment{newPathItem(kind, base, ps)}, nil, nil
	}
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("invalid attachment state for normalization")
	}
	if cwd == nil || cwd.Path == "" {
		return nil, nil, fmt.Errorf("cwd required for path expansion")
	}
	matches, deferred, err := expandPathPatterns(ctx, eng, cwd, tplCtx, paths)
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(matches)
	items := buildPathItems(kind, base, matches)
	if len(deferred) > 0 {
		return items, newPathParent(kind, base, deferred), nil
	}
	return items, nil, nil
}

// applyTemplateOnBase evaluates templates in base fields (name, mime, meta).
func applyTemplateOnBase(eng *tplengine.TemplateEngine, tplCtx map[string]any, b *baseAttachment) error {
	if b == nil {
		return nil
	}
	if b.NameStr != "" {
		v, _, err := applyTemplateString(eng, tplCtx, b.NameStr)
		if err != nil {
			return err
		}
		b.NameStr = v
	}
	if b.MIME != "" {
		v, _, err := applyTemplateString(eng, tplCtx, b.MIME)
		if err != nil {
			return err
		}
		b.MIME = v
	}
	if len(b.MetaMap) > 0 {
		for k, val := range b.MetaMap {
			if s, ok := val.(string); ok {
				v, _, err := applyTemplateString(eng, tplCtx, s)
				if err != nil {
					return err
				}
				b.MetaMap[k] = v
			}
		}
	}
	return nil
}

// applyTemplateString renders a string; when .tasks.* cannot resolve, returns
// the original string and marks it as deferred.
func applyTemplateString(eng *tplengine.TemplateEngine, ctx map[string]any, in string) (string, bool, error) {
	if eng == nil {
		return in, false, nil
	}
	res, err := eng.ParseMapWithFilter(in, ctx, nil)
	if err != nil {
		return "", false, err
	}
	s, ok := res.(string)
	if !ok {
		return "", false, fmt.Errorf("template did not resolve to string")
	}
	// Deferred if original template references .tasks.* and still contains template syntax after evaluation
	deferred := strings.Contains(in, ".tasks.") && tplengine.HasTemplate(s)
	return s, deferred, nil
}

// globWithinCWD expands a pattern under CWD, returning CWD-relative matches.
func globWithinCWD(cwd *core.PathCWD, pattern string) ([]string, error) {
	root := filepath.Clean(cwd.Path)
	absPattern := filepath.Clean(filepath.Join(root, pattern))
	matches, err := doublestar.FilepathGlob(absPattern)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		within, err := pathWithin(root, m)
		if err != nil {
			return nil, fmt.Errorf("path validation failed for match %s: %w", m, err)
		}
		if !within {
			return nil, fmt.Errorf("match outside CWD: %s", m)
		}
		rel, rerr := filepath.Rel(root, m)
		if rerr != nil {
			return nil, rerr
		}
		out = append(out, rel)
	}
	return out, nil
}

// helpers to reduce cyclomatic complexity

// attachment factory maps to avoid repetitive switches
var attachmentSingleFactories = map[Type]func(baseAttachment, Source, string) Attachment{
	TypeImage: func(b baseAttachment, s Source, v string) Attachment {
		if s == SourceURL {
			return &ImageAttachment{baseAttachment: b, Source: s, URL: v}
		}
		return &ImageAttachment{baseAttachment: b, Source: s, Path: v}
	},
	TypePDF: func(b baseAttachment, s Source, v string) Attachment {
		if s == SourceURL {
			return &PDFAttachment{baseAttachment: b, Source: s, URL: v}
		}
		return &PDFAttachment{baseAttachment: b, Source: s, Path: v}
	},
	TypeAudio: func(b baseAttachment, s Source, v string) Attachment {
		if s == SourceURL {
			return &AudioAttachment{baseAttachment: b, Source: s, URL: v}
		}
		return &AudioAttachment{baseAttachment: b, Source: s, Path: v}
	},
	TypeVideo: func(b baseAttachment, s Source, v string) Attachment {
		if s == SourceURL {
			return &VideoAttachment{baseAttachment: b, Source: s, URL: v}
		}
		return &VideoAttachment{baseAttachment: b, Source: s, Path: v}
	},
}

var attachmentMultiFactories = map[Type]func(baseAttachment, Source, []string) Attachment{
	TypeImage: func(b baseAttachment, s Source, v []string) Attachment {
		if s == SourceURL {
			return &ImageAttachment{baseAttachment: b, Source: s, URLs: v}
		}
		return &ImageAttachment{baseAttachment: b, Source: s, Paths: v}
	},
	TypePDF: func(b baseAttachment, s Source, v []string) Attachment {
		if s == SourceURL {
			return &PDFAttachment{baseAttachment: b, Source: s, URLs: v}
		}
		return &PDFAttachment{baseAttachment: b, Source: s, Paths: v}
	},
	TypeAudio: func(b baseAttachment, s Source, v []string) Attachment {
		if s == SourceURL {
			return &AudioAttachment{baseAttachment: b, Source: s, URLs: v}
		}
		return &AudioAttachment{baseAttachment: b, Source: s, Paths: v}
	},
	TypeVideo: func(b baseAttachment, s Source, v []string) Attachment {
		if s == SourceURL {
			return &VideoAttachment{baseAttachment: b, Source: s, URLs: v}
		}
		return &VideoAttachment{baseAttachment: b, Source: s, Paths: v}
	},
}

func newAttachmentItem(kind Type, src Source, base baseAttachment, v string) Attachment {
	if f, ok := attachmentSingleFactories[kind]; ok {
		return f(base, src, v)
	}
	// Unreachable with supported kinds; return nil to surface misuse during tests
	return nil
}

func newAttachmentParent(kind Type, src Source, base baseAttachment, v []string) Attachment {
	if f, ok := attachmentMultiFactories[kind]; ok {
		return f(base, src, v)
	}
	return nil
}

// newURLItem creates a concrete attachment for a single URL.
func newURLItem(kind Type, base baseAttachment, u string) Attachment {
	return newAttachmentItem(kind, SourceURL, base, u)
}

// newURLParent creates a parent attachment holding deferred URLs.
func newURLParent(kind Type, base baseAttachment, urls []string) Attachment {
	return newAttachmentParent(kind, SourceURL, base, urls)
}

// buildURLItems builds children for a set of resolved URLs.
func buildURLItems(kind Type, base baseAttachment, resolved []string) []Attachment {
	items := make([]Attachment, 0, len(resolved))
	for _, s := range resolved {
		items = append(items, newURLItem(kind, base, s))
	}
	return items
}

// splitResolvedURLs separates resolved URL strings from deferred templates.
func splitResolvedURLs(eng *tplengine.TemplateEngine, ctx map[string]any, urls []string) ([]string, []string, error) {
	resolved := make([]string, 0, len(urls))
	deferred := make([]string, 0)
	for _, u := range urls {
		s, isDeferred, err := applyTemplateString(eng, ctx, strings.TrimSpace(u))
		if err != nil {
			return nil, nil, err
		}
		if s == "" {
			continue
		}
		if isDeferred {
			deferred = append(deferred, s)
		} else {
			resolved = append(resolved, s)
		}
	}
	return resolved, deferred, nil
}

// newPathItem creates a concrete attachment for a single filesystem path.
func newPathItem(kind Type, base baseAttachment, p string) Attachment {
	return newAttachmentItem(kind, SourcePath, base, p)
}

// newPathParent creates a parent attachment holding deferred path patterns.
func newPathParent(kind Type, base baseAttachment, ps []string) Attachment {
	return newAttachmentParent(kind, SourcePath, base, ps)
}

// buildPathItems builds children for a set of resolved path matches.
func buildPathItems(kind Type, base baseAttachment, matches []string) []Attachment {
	items := make([]Attachment, 0, len(matches))
	for _, rel := range matches {
		items = append(items, newPathItem(kind, base, rel))
	}
	return items
}

// expandPathPatterns evaluates patterns and expands resolvable ones within CWD.
func expandPathPatterns(
	ctx context.Context,
	eng *tplengine.TemplateEngine,
	cwd *core.PathCWD,
	tplCtx map[string]any,
	patterns []string,
) ([]string, []string, error) {
	matches := make([]string, 0)
	deferred := make([]string, 0)
	for _, p := range patterns {
		s, isDeferred, err := applyTemplateString(eng, tplCtx, strings.TrimSpace(p))
		if err != nil {
			return nil, nil, err
		}
		if s == "" {
			continue
		}
		if isDeferred {
			deferred = append(deferred, s)
		} else {
			expanded, err := globWithinCWD(cwd, s)
			if err != nil {
				logger.FromContext(ctx).Warn("glob expansion failed", "pattern", s, "error", err)
				return nil, nil, fmt.Errorf("failed to expand pattern %q: %w", s, err)
			}
			matches = append(matches, expanded...)
		}
	}
	return matches, deferred, nil
}
