package attachment

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePhase1_GlobExpansionAndMetadata(t *testing.T) {
	t.Run("Should expand paths with glob and inherit metadata", func(t *testing.T) {
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "a.png"))
		mustWrite(t, filepath.Join(dir, "b.png"))
		cwd := &core.PathCWD{Path: dir}
		eng := tplengine.NewEngine(tplengine.FormatText)
		att := &ImageAttachment{
			baseAttachment: baseAttachment{NameStr: "img", MetaMap: map[string]any{"k": "v"}},
			Source:         SourcePath,
			Paths:          []string{"*.png"},
		}
		res, err := NormalizePhase1(context.Background(), eng, cwd, []Attachment{att}, map[string]any{})
		require.NoError(t, err)
		require.Len(t, res, 2)
		got := []string{}
		for _, a := range res {
			img, ok := a.(*ImageAttachment)
			require.True(t, ok)
			assert.Equal(t, SourcePath, img.Source)
			assert.Equal(t, "img", img.Name())
			assert.Equal(t, "v", img.Meta()["k"])
			got = append(got, img.Path)
		}
		sort.Strings(got)
		assert.Equal(t, []string{"a.png", "b.png"}, got)
	})
}

func TestNormalize_TemplatesDeferralAndPhase2(t *testing.T) {
	t.Run("Should defer .tasks.* in phase1 and resolve in phase2", func(t *testing.T) {
		cwd := &core.PathCWD{Path: t.TempDir()}
		eng := tplengine.NewEngine(tplengine.FormatText)
		in := &ImageAttachment{
			baseAttachment: baseAttachment{NameStr: "img"},
			Source:         SourceURL,
			URLs:           []string{"{{ .tasks.prev.output.url }}", "https://x/y.png"},
		}
		phase1, err := NormalizePhase1(context.Background(), eng, cwd, []Attachment{in}, map[string]any{})
		require.NoError(t, err)
		require.Len(t, phase1, 2)
		// One expanded child and one parent with deferred URLs
		var child *ImageAttachment
		var parent *ImageAttachment
		for _, a := range phase1 {
			if v, ok := a.(*ImageAttachment); ok {
				if v.URL != "" {
					child = v
				} else {
					parent = v
				}
			}
		}
		require.NotNil(t, child)
		assert.Equal(t, "https://x/y.png", child.URL)
		require.NotNil(t, parent)
		require.Len(t, parent.URLs, 1)
		assert.Contains(t, parent.URLs[0], ".tasks.prev.output.url")
		// Phase 2: now provide tasks context so it resolves
		ctx := map[string]any{
			"tasks": map[string]any{"prev": map[string]any{"output": map[string]any{"url": "https://z/img.png"}}},
		}
		phase2, err := NormalizePhase2(context.Background(), eng, cwd, []Attachment{parent}, ctx)
		require.NoError(t, err)
		require.Len(t, phase2, 1)
		v, ok := phase2[0].(*ImageAttachment)
		require.True(t, ok)
		assert.Equal(t, "https://z/img.png", v.URL)
		assert.Empty(t, v.URLs)
	})
}

func TestNormalizePhase1_UnmatchedPattern_NoExpansion(t *testing.T) {
	t.Run("Should not expand unmatched patterns and not error", func(t *testing.T) {
		cwd := &core.PathCWD{Path: t.TempDir()}
		eng := tplengine.NewEngine(tplengine.FormatText)
		in := &ImageAttachment{
			baseAttachment: baseAttachment{NameStr: "img"},
			Source:         SourcePath,
			Paths:          []string{"nomatch/*.png"},
		}
		res, err := NormalizePhase1(context.Background(), eng, cwd, []Attachment{in}, map[string]any{})
		require.NoError(t, err)
		assert.Len(t, res, 0)
	})
}

func TestNormalizePhase1_NestedGlob(t *testing.T) {
	t.Run("Should expand recursive ** patterns", func(t *testing.T) {
		dir := t.TempDir()
		mk := func(p string) { mustWrite(t, filepath.Join(dir, p)) }
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755))
		mk("a/x.png")
		mk("a/b/y.png")
		mk("a/b/c/z.png")
		cwd := &core.PathCWD{Path: dir}
		eng := tplengine.NewEngine(tplengine.FormatText)
		in := &ImageAttachment{Source: SourcePath, Paths: []string{"a/**/*.png"}}
		res, err := NormalizePhase1(context.Background(), eng, cwd, []Attachment{in}, map[string]any{})
		require.NoError(t, err)
		got := []string{}
		for _, a := range res {
			got = append(got, a.(*ImageAttachment).Path)
		}
		sort.Strings(got)
		assert.Equal(t, []string{"a/b/c/z.png", "a/b/y.png", "a/x.png"}, got)
	})
}

func TestNormalizePhase1_PathTraversalPrevented(t *testing.T) {
	t.Run("Should ignore patterns that escape CWD", func(t *testing.T) {
		dir := t.TempDir()
		// create sibling file outside a subdir to attempt traversal
		sibling := t.TempDir()
		mustWrite(t, filepath.Join(sibling, "outside.png"))
		cwd := &core.PathCWD{Path: dir}
		eng := tplengine.NewEngine(tplengine.FormatText)
		in := &ImageAttachment{Source: SourcePath, Paths: []string{"../*.png"}}
		res, err := NormalizePhase1(context.Background(), eng, cwd, []Attachment{in}, map[string]any{})
		require.NoError(t, err)
		assert.Len(t, res, 0)
	})
}

func TestNormalizePhase1_URLMetadataInheritance(t *testing.T) {
	t.Run("Should inherit base name/meta for URL children", func(t *testing.T) {
		cwd := &core.PathCWD{Path: t.TempDir()}
		eng := tplengine.NewEngine(tplengine.FormatText)
		in := &ImageAttachment{
			baseAttachment: baseAttachment{NameStr: "img", MetaMap: map[string]any{"k": "v"}},
			Source:         SourceURL,
			URLs:           []string{"https://a/1.png", "https://a/2.png"},
		}
		res, err := NormalizePhase1(context.Background(), eng, cwd, []Attachment{in}, map[string]any{})
		require.NoError(t, err)
		require.Len(t, res, 2)
		for _, a := range res {
			v := a.(*ImageAttachment)
			assert.Equal(t, "img", v.Name())
			assert.Equal(t, "v", v.Meta()["k"])
		}
	})
}

func TestNormalizePhase1_PathsDeferralAndPhase2(t *testing.T) {
	t.Run("Should defer pattern templates in phase1 and expand in phase2", func(t *testing.T) {
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "a.png"))
		cwd := &core.PathCWD{Path: dir}
		eng := tplengine.NewEngine(tplengine.FormatText)
		in := &ImageAttachment{Source: SourcePath, Paths: []string{"{{ .tasks.prev.output.pattern }}"}}
		p1, err := NormalizePhase1(context.Background(), eng, cwd, []Attachment{in}, map[string]any{})
		require.NoError(t, err)
		// Expect a parent with deferred Paths
		require.Len(t, p1, 1)
		parent := p1[0].(*ImageAttachment)
		require.Len(t, parent.Paths, 1)
		ctx := map[string]any{
			"tasks": map[string]any{"prev": map[string]any{"output": map[string]any{"pattern": "*.png"}}},
		}
		p2, err := NormalizePhase2(context.Background(), eng, cwd, []Attachment{parent}, ctx)
		require.NoError(t, err)
		require.Len(t, p2, 1)
		child := p2[0].(*ImageAttachment)
		assert.Equal(t, "a.png", child.Path)
		assert.Empty(t, child.Paths)
	})
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	err := os.WriteFile(path, []byte("x"), 0o644)
	require.NoError(t, err)
}
