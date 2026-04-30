package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/frontmatter"
	taskscore "github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type documentReader struct {
	mu    sync.RWMutex
	cache map[string]cachedDocument
}

type cachedDocument struct {
	updatedAt time.Time
	sizeBytes int64
	doc       MarkdownDocument
}

type markdownDirEntry struct {
	absPath     string
	displayPath string
	sizeBytes   int64
	updatedAt   time.Time
}

func newDocumentReader() *documentReader {
	return &documentReader{
		cache: make(map[string]cachedDocument),
	}
}

func (r *documentReader) Read(
	ctx context.Context,
	path string,
	kind string,
	id string,
) (MarkdownDocument, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "." || cleanPath == "" {
		return MarkdownDocument{}, errors.New("daemon: document path is required")
	}

	info, err := regularMarkdownFileInfo(cleanPath)
	if err != nil {
		return MarkdownDocument{}, err
	}
	if ctx != nil && ctx.Err() != nil {
		return MarkdownDocument{}, ctx.Err()
	}

	if doc, ok := r.cached(cleanPath, info.ModTime().UTC(), info.Size()); ok {
		return doc, nil
	}

	content, err := os.ReadFile(cleanPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MarkdownDocument{}, ErrDocumentMissing
		}
		return MarkdownDocument{}, fmt.Errorf("daemon: read document %q: %w", cleanPath, err)
	}
	doc, err := normalizeMarkdownDocument(cleanPath, kind, id, content, info.ModTime().UTC())
	if err != nil {
		return MarkdownDocument{}, err
	}
	r.store(cleanPath, info.ModTime().UTC(), info.Size(), doc)
	return cloneMarkdownDocument(doc), nil
}

func (r *documentReader) cached(path string, updatedAt time.Time, sizeBytes int64) (MarkdownDocument, bool) {
	if r == nil {
		return MarkdownDocument{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.cache[path]
	if !ok {
		return MarkdownDocument{}, false
	}
	if !item.updatedAt.Equal(updatedAt) || item.sizeBytes != sizeBytes {
		return MarkdownDocument{}, false
	}
	return cloneMarkdownDocument(item.doc), true
}

func (r *documentReader) store(path string, updatedAt time.Time, sizeBytes int64, doc MarkdownDocument) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[path] = cachedDocument{
		updatedAt: updatedAt,
		sizeBytes: sizeBytes,
		doc:       cloneMarkdownDocument(doc),
	}
}

func normalizeMarkdownDocument(
	path string,
	kind string,
	id string,
	content []byte,
	updatedAt time.Time,
) (MarkdownDocument, error) {
	metadata := make(map[string]any)
	raw := string(content)
	markdown, err := frontmatter.Parse(raw, &metadata)
	switch {
	case err == nil:
	case errors.Is(err, frontmatter.ErrHeaderNotFound):
		metadata = nil
		markdown = raw
	default:
		return MarkdownDocument{}, fmt.Errorf("daemon: parse document %q front matter: %w", path, err)
	}

	doc := MarkdownDocument{
		ID:        strings.TrimSpace(id),
		Kind:      strings.TrimSpace(kind),
		Title:     documentTitle(path, kind, metadata, markdown),
		UpdatedAt: updatedAt,
		Markdown:  markdown,
		Metadata:  cloneMetadataMap(metadata),
	}
	return doc, nil
}

func markdownDocumentFromSnapshot(
	snapshot globaldb.ArtifactSnapshotRow,
	kind string,
	id string,
) (MarkdownDocument, error) {
	metadata := make(map[string]any)
	frontmatterJSON := strings.TrimSpace(snapshot.FrontmatterJSON)
	if frontmatterJSON != "" && frontmatterJSON != "{}" {
		if err := json.Unmarshal([]byte(frontmatterJSON), &metadata); err != nil {
			return MarkdownDocument{}, fmt.Errorf(
				"daemon: parse snapshot front matter %q: %w",
				snapshot.RelativePath,
				err,
			)
		}
	}
	markdown := snapshot.BodyText
	doc := MarkdownDocument{
		ID:        strings.TrimSpace(id),
		Kind:      strings.TrimSpace(kind),
		Title:     documentTitle(snapshot.RelativePath, kind, metadata, markdown),
		UpdatedAt: snapshot.SourceMTime.UTC(),
		Markdown:  markdown,
		Metadata:  cloneMetadataMap(metadata),
	}
	return doc, nil
}

func readMarkdownDir(root string) ([]markdownDirEntry, error) {
	cleanRoot := filepath.Clean(strings.TrimSpace(root))
	if cleanRoot == "." || cleanRoot == "" {
		return nil, errors.New("daemon: markdown directory is required")
	}
	if err := fileInfo(cleanRoot); err != nil {
		return nil, err
	}

	entries := make([]markdownDirEntry, 0)
	err := filepath.WalkDir(cleanRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return ErrDocumentMissing
			}
			return fmt.Errorf("daemon: walk markdown directory %q: %w", cleanRoot, walkErr)
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("daemon: symlinked markdown file %q is not allowed", path)
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("daemon: stat markdown file %q: %w", path, err)
		}
		relativePath, err := filepath.Rel(cleanRoot, path)
		if err != nil {
			return fmt.Errorf("daemon: resolve markdown relative path %q: %w", path, err)
		}
		entries = append(entries, markdownDirEntry{
			absPath:     path,
			displayPath: filepath.ToSlash(relativePath),
			sizeBytes:   info.Size(),
			updatedAt:   info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrDocumentMissing) {
			return nil, ErrDocumentMissing
		}
		return nil, err
	}

	sortMarkdownDirEntries(entries)
	return entries, nil
}

func sortMarkdownDirEntries(entries []markdownDirEntry) {
	if len(entries) < 2 {
		return
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].displayPath < entries[j].displayPath
	})
}

func fileInfo(path string) error {
	_, err := os.Stat(strings.TrimSpace(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrDocumentMissing
		}
		return fmt.Errorf("daemon: stat %q: %w", path, err)
	}
	return nil
}

func regularMarkdownFileInfo(path string) (fs.FileInfo, error) {
	info, err := os.Lstat(strings.TrimSpace(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrDocumentMissing
		}
		return nil, fmt.Errorf("daemon: stat %q: %w", path, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("daemon: symlinked markdown file %q is not allowed", path)
	}
	return info, nil
}

func memoryFileID(workspaceID string, workflowSlug string, displayPath string) string {
	hash := sha256.Sum256([]byte(
		strings.TrimSpace(workspaceID) + "\x00" +
			strings.TrimSpace(workflowSlug) + "\x00" +
			filepath.ToSlash(strings.TrimSpace(displayPath)),
	))
	return "mem_" + hex.EncodeToString(hash[:16])
}

func documentTitle(path string, kind string, metadata map[string]any, markdown string) string {
	if title := metadataString(metadata, "title"); title != "" {
		return title
	}
	if strings.EqualFold(strings.TrimSpace(kind), markdownDocumentKindTask) {
		if title := taskscore.ExtractTaskBodyTitle(markdown); title != "" {
			return title
		}
	}
	if title := firstMarkdownHeading(markdown); title != "" {
		return title
	}

	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	switch base {
	case "_prd":
		return "PRD"
	case "_techspec":
		return "TechSpec"
	case "MEMORY":
		return "Memory"
	default:
		return strings.TrimSpace(strings.ReplaceAll(base, "_", " "))
	}
}

func firstMarkdownHeading(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "# ") {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		if title != "" {
			return title
		}
	}
	return ""
}

func metadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	raw, ok := metadata[strings.TrimSpace(key)]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func cloneMetadataMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = cloneMetadataValue(value)
	}
	return dst
}

func cloneMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMetadataMap(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i := range typed {
			cloned[i] = cloneMetadataValue(typed[i])
		}
		return cloned
	default:
		return typed
	}
}

func cloneMarkdownDocument(doc MarkdownDocument) MarkdownDocument {
	doc.Metadata = cloneMetadataMap(doc.Metadata)
	return doc
}
