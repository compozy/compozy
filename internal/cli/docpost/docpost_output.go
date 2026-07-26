package docpost

import (
	"context"
	"encoding/json"

	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"sort"
	"strings"
)

// computeHasChildren marks a baseName as a "parent" when any other input's
// baseName starts with baseName + "_". Parents render as index.mdx in their
// own directory; leaves render as <segment>.mdx under the parent directory.
func computeHasChildren(inputs []input) map[string]bool {
	result := make(map[string]bool, len(inputs))
	for _, a := range inputs {
		prefix := a.baseName + "_"
		for _, b := range inputs {
			if a.baseName != b.baseName && strings.HasPrefix(b.baseName, prefix) {
				result[a.baseName] = true
				break
			}
		}
	}
	return result
}

// outPath returns the output file path for an input, relative to dstDir, using
// forward slashes.
func outPath(in input, hasChildren map[string]bool) string {
	if len(in.segments) == 0 {
		return docpostAghMDXPath
	}
	if hasChildren[in.baseName] {
		return path.Join(in.segments...) + "/index.mdx"
	}
	if len(in.segments) == 1 {
		return in.segments[0] + ".mdx"
	}
	parent := path.Join(in.segments[:len(in.segments)-1]...)
	return parent + "/" + in.segments[len(in.segments)-1] + ".mdx"
}

// buildTargetMap builds a baseName -> absolute URL map used by remapLinks.
// The root command maps to the generated agh page; every other command maps
// to linkBasePath + its segment path.
func buildTargetMap(inputs []input) map[string]string {
	targets := make(map[string]string, len(inputs))
	for _, in := range inputs {
		targets[in.baseName] = in.targetURL()
	}
	return targets
}

func validateOutputPaths(inputs []input, hasChildren map[string]bool) error {
	seen := make(map[string]string, len(inputs))
	for _, in := range inputs {
		outRel := in.outputPath(hasChildren)
		if previous, ok := seen[outRel]; ok {
			return fmt.Errorf("docpost: output path collision %q for %s and %s", outRel, previous, in.fileName)
		}
		seen[outRel] = in.fileName
	}
	return nil
}

// remapLinks rewrites any `](agh_xxx)` link target in the body to its
// absolute URL under linkBasePath. Runs after TransformMarkdown has already
// stripped `.md` extensions via rewriteLinks.
func remapLinks(body string, targets map[string]string) string {
	return transformMarkdownOutsideCode(body, func(text string) string {
		return strippedLink.ReplaceAllStringFunc(text, func(match string) string {
			m := strippedLink.FindStringSubmatch(match)
			if m == nil {
				return match
			}
			target, ok := targets[m[1]]
			if !ok {
				return match
			}
			return "](" + target + ")"
		})
	})
}

// cleanOutput removes generated files in dstDir while preserving the root
// hand-maintained index.mdx and meta.json.
func cleanOutput(ctx context.Context, dstDir string) error {
	if err := ensureContext(ctx, fmt.Sprintf("clean output dir %s", dstDir)); err != nil {
		return err
	}
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("docpost: read output dir %s: %w", dstDir, err)
	}
	for _, e := range entries {
		if e.Name() == docpostIndexMDXPath || e.Name() == docpostMetaJSONPath {
			continue
		}
		target := filepath.Join(dstDir, e.Name())
		if err := ensureContext(ctx, fmt.Sprintf("remove stale entry %s", target)); err != nil {
			return err
		}
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("docpost: remove stale entry %s: %w", target, err)
		}
	}
	return nil
}

// writeSubdirMetas walks dstDir and emits a meta.json in every subdirectory
// (never the root) listing its direct children alphabetically, with an
// optional leading "index" when an index.mdx is present.
func writeSubdirMetas(ctx context.Context, dstDir string) error {
	return filepath.WalkDir(dstDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("docpost: walk %s: %w", p, err)
		}
		if err := ensureContext(ctx, fmt.Sprintf("write meta for %s", p)); err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dstDir, p)
		if err != nil {
			return fmt.Errorf("docpost: relative path from %s to %s: %w", dstDir, p, err)
		}
		if rel == "." {
			return nil // root is hand-maintained
		}
		return writeDirMeta(ctx, p)
	})
}

func writeDirMeta(ctx context.Context, dir string) error {
	if err := ensureContext(ctx, fmt.Sprintf("write meta for %s", dir)); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("docpost: read dir %s: %w", dir, err)
	}
	var files, subdirs []string
	hasIndex := false
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			subdirs = append(subdirs, name)
			continue
		}
		if !strings.HasSuffix(name, ".mdx") {
			continue
		}
		base := strings.TrimSuffix(name, ".mdx")
		if base == docpostIndexKey {
			hasIndex = true
			continue
		}
		files = append(files, base)
	}
	sort.Strings(files)
	sort.Strings(subdirs)

	pages := make([]string, 0, 1+len(files)+len(subdirs))
	if hasIndex {
		pages = append(pages, docpostIndexKey)
	}
	pages = append(pages, files...)
	pages = append(pages, subdirs...)

	meta := struct {
		Title string   `json:"title"`
		Pages []string `json:"pages"`
	}{
		Title: titleCase(filepath.Base(dir)),
		Pages: pages,
	}
	data, err := json.MarshalIndent(meta, "", "    ")
	if err != nil {
		return fmt.Errorf("docpost: marshal meta for %s: %w", dir, err)
	}
	metaPath := filepath.Join(dir, docpostMetaJSONPath)
	if err := os.WriteFile(metaPath, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("docpost: write %s: %w", metaPath, err)
	}
	return nil
}

// titleCase renders a lowercase command segment as a display title.
// Replaces dashes with spaces and capitalises each word.
func titleCase(seg string) string {
	parts := strings.FieldsFunc(seg, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range parts {
		if w == "" {
			continue
		}
		parts[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(parts, " ")
}
