package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/ledongthuc/pdf"

	"github.com/compozy/compozy/engine/attachment"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/chunk"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const MaxMarkdownFileSizeBytes = 4 * 1024 * 1024

type documentList struct {
	items []chunk.Document
	hash  map[string]struct{}
}

func enumerateSources(ctx context.Context, kb *knowledge.BaseConfig, opts *Options) ([]chunk.Document, error) {
	if kb == nil {
		return nil, errors.New("knowledge: knowledge base is required")
	}
	if opts == nil {
		return nil, errors.New("knowledge: ingest options are required")
	}
	list := documentList{items: make([]chunk.Document, 0), hash: make(map[string]struct{})}
	for i := range kb.Sources {
		src := &kb.Sources[i]
		switch src.Type {
		case knowledge.SourceTypeMarkdownGlob:
			if err := list.appendMarkdown(ctx, kb.ID, src, opts.CWD); err != nil {
				return nil, err
			}
		case knowledge.SourceTypePDFURL:
			if err := list.appendPDFURLs(ctx, kb.ID, src); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("knowledge: source type %q not supported in ingestion MVP", src.Type)
		}
	}
	return list.items, nil
}

func (l *documentList) appendMarkdown(
	ctx context.Context,
	kbID string,
	src *knowledge.SourceConfig,
	cwd *core.PathCWD,
) error {
	if cwd == nil {
		return errors.New("knowledge: markdown_glob requires working directory")
	}
	pattern := strings.TrimSpace(src.Path)
	if pattern == "" && len(src.Paths) > 0 {
		pattern = strings.TrimSpace(src.Paths[0])
	}
	if pattern == "" {
		return fmt.Errorf("knowledge: markdown_glob source missing path")
	}
	root := filepath.Clean(cwd.PathStr())
	absPattern := filepath.Clean(filepath.Join(root, pattern))
	matches, err := doublestar.FilepathGlob(absPattern)
	if err != nil {
		return fmt.Errorf("knowledge: glob %q failed: %w", pattern, err)
	}
	if len(matches) == 0 {
		logger.FromContext(ctx).Warn("Knowledge ingestion glob returned no files", "pattern", pattern)
	}
	for _, abs := range matches {
		within, werr := pathInside(root, abs)
		if werr != nil {
			return werr
		}
		if !within {
			return fmt.Errorf("knowledge: glob match %q escapes working directory", abs)
		}
		rel, rerr := filepath.Rel(root, abs)
		if rerr != nil {
			return fmt.Errorf("knowledge: resolve relative path for %q: %w", abs, rerr)
		}
		text, readErr := readMarkdownFile(abs)
		if readErr != nil {
			return readErr
		}
		if text == "" {
			continue
		}
		hash := hashContent(text)
		if _, exists := l.hash[hash]; exists {
			continue
		}
		l.hash[hash] = struct{}{}
		docID := filepath.ToSlash(rel)
		meta := map[string]any{
			"source_type":  string(knowledge.SourceTypeMarkdownGlob),
			"source_path":  docID,
			"content_hash": hash,
			"kb_id":        kbID,
		}
		l.items = append(l.items, chunk.Document{ID: docID, Text: text, Metadata: meta})
	}
	return nil
}

func (l *documentList) appendPDFURLs(ctx context.Context, kbID string, src *knowledge.SourceConfig) error {
	urls := make([]string, 0, len(src.URLs))
	for i := range src.URLs {
		if u := strings.TrimSpace(src.URLs[i]); u != "" {
			urls = append(urls, u)
		}
	}
	if primary := strings.TrimSpace(src.Path); primary != "" {
		urls = append(urls, primary)
	}
	if len(urls) == 0 {
		return fmt.Errorf("knowledge: pdf_url source requires urls")
	}
	for i := range urls {
		url := urls[i]
		text, err := fetchPDFText(ctx, url)
		if err != nil {
			return err
		}
		content := strings.TrimSpace(text)
		if content == "" {
			continue
		}
		hash := hashContent(content)
		if _, exists := l.hash[hash]; exists {
			continue
		}
		l.hash[hash] = struct{}{}
		meta := map[string]any{
			"source_type":  string(knowledge.SourceTypePDFURL),
			"source_url":   url,
			"content_hash": hash,
			"kb_id":        kbID,
		}
		l.items = append(l.items, chunk.Document{ID: url, Text: content, Metadata: meta})
	}
	return nil
}

func readMarkdownFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("knowledge: open markdown %q: %w", path, err)
	}
	defer file.Close()
	info, statErr := file.Stat()
	if statErr != nil {
		return "", fmt.Errorf("knowledge: stat markdown %q: %w", path, statErr)
	}
	if info.Size() > int64(MaxMarkdownFileSizeBytes) {
		return "", fmt.Errorf(
			"knowledge: markdown file %q exceeds maximum size of %d bytes",
			path,
			MaxMarkdownFileSizeBytes,
		)
	}
	reader := io.LimitReader(file, int64(MaxMarkdownFileSizeBytes)+1)
	data, readErr := io.ReadAll(reader)
	if readErr != nil {
		return "", fmt.Errorf("knowledge: read markdown %q: %w", path, readErr)
	}
	if len(data) > MaxMarkdownFileSizeBytes {
		return "", fmt.Errorf(
			"knowledge: markdown file %q changed during ingestion and exceeded %d bytes",
			path,
			MaxMarkdownFileSizeBytes,
		)
	}
	return strings.TrimSpace(string(data)), nil
}

func pathInside(root, target string) (bool, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, fmt.Errorf("knowledge: resolve root %q: %w", root, err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("knowledge: target path does not exist: %s", target)
		}
		return false, fmt.Errorf("knowledge: resolve target %q: %w", target, err)
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil {
		return false, fmt.Errorf("knowledge: compute relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false, nil
	}
	return true, nil
}

func fetchPDFText(ctx context.Context, url string) (string, error) {
	att := &attachment.PDFAttachment{Source: attachment.SourceURL, URL: url}
	resolved, err := att.Resolve(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("knowledge: resolve pdf url %q: %w", url, err)
	}
	defer resolved.Cleanup()
	path, ok := resolved.AsFilePath()
	if !ok || path == "" {
		rc, oerr := resolved.Open()
		if oerr != nil {
			return "", fmt.Errorf("knowledge: open pdf stream %q: %w", url, oerr)
		}
		defer rc.Close()
		temp, terr := os.CreateTemp("", "kb-pdf-*")
		if terr != nil {
			return "", fmt.Errorf("knowledge: create temp file: %w", terr)
		}
		defer func() {
			temp.Close()
			os.Remove(temp.Name())
		}()
		if _, werr := io.Copy(temp, rc); werr != nil {
			return "", fmt.Errorf("knowledge: buffer pdf stream: %w", werr)
		}
		if _, serr := temp.Seek(0, io.SeekStart); serr != nil {
			return "", fmt.Errorf("knowledge: rewind temp file: %w", serr)
		}
		return extractPDFText(ctx, temp.Name())
	}
	return extractPDFText(ctx, path)
}

func extractPDFText(ctx context.Context, path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("knowledge: open pdf %q: %w", path, err)
	}
	defer f.Close()
	rd, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("knowledge: extract pdf text %q: %w", path, err)
	}
	if closer, ok := rd.(io.ReadCloser); ok {
		defer closer.Close()
	}
	limit := int64(attachment.MaxPDFExtractChars)
	if cfg := appconfig.FromContext(ctx); cfg != nil && cfg.Attachments.PDFExtractMaxChars > 0 {
		limit = int64(cfg.Attachments.PDFExtractMaxChars)
	}
	var b strings.Builder
	if _, err := io.Copy(&b, io.LimitReader(rd, limit)); err != nil {
		return "", fmt.Errorf("knowledge: read pdf %q: %w", path, err)
	}
	return b.String(), nil
}

func hashContent(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:16])
}
