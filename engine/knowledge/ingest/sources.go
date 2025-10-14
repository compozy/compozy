package ingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gabriel-vasile/mimetype"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"

	"github.com/compozy/compozy/engine/attachment"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/chunk"
	"github.com/compozy/compozy/engine/pdftext"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const MaxMarkdownFileSizeBytes = 4 * 1024 * 1024

type documentList struct {
	items []chunk.Document
	hash  map[string]struct{}
}

var (
	downloadToTemp = attachment.DownloadURLToTemp
	pdfExtractor   = extractPDF
)

type remoteFetchResult struct {
	text        string
	contentType string
	size        int64
	filename    string
	pdfStats    *pdftext.Stats
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
		case knowledge.SourceTypeURL:
			if err := list.appendRemoteURLs(ctx, kb.ID, src); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("knowledge: source type %q not supported", src.Type)
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
	patterns := make([]string, 0, len(src.Paths)+1)
	if single := strings.TrimSpace(src.Path); single != "" {
		patterns = append(patterns, single)
	}
	for i := range src.Paths {
		if trimmed := strings.TrimSpace(src.Paths[i]); trimmed != "" {
			patterns = append(patterns, trimmed)
		}
	}
	if len(patterns) == 0 {
		return fmt.Errorf("knowledge: markdown_glob source missing path")
	}
	root := filepath.Clean(cwd.PathStr())
	for _, pattern := range patterns {
		if err := l.appendMarkdownPattern(ctx, root, kbID, pattern); err != nil {
			return err
		}
	}
	return nil
}

func (l *documentList) appendMarkdownPattern(
	ctx context.Context,
	root string,
	kbID string,
	pattern string,
) error {
	absPattern := filepath.Clean(filepath.Join(root, pattern))
	matches, err := doublestar.FilepathGlob(absPattern)
	if err != nil {
		return fmt.Errorf("knowledge: glob %q failed: %w", pattern, err)
	}
	if len(matches) == 0 {
		logger.FromContext(ctx).Warn("Knowledge ingestion glob returned no files", "pattern", pattern)
		return nil
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
		docID := filepath.ToSlash(rel)
		meta := map[string]any{
			"source_type": string(knowledge.SourceTypeMarkdownGlob),
			"source_path": docID,
		}
		l.appendDocument(kbID, docID, text, meta)
	}
	return nil
}

func (l *documentList) appendDocument(kbID, docID, text string, meta map[string]any) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return
	}
	hash := hashContent(trimmed)
	if _, exists := l.hash[hash]; exists {
		return
	}
	if meta == nil {
		meta = make(map[string]any, 2)
	}
	meta["content_hash"] = hash
	meta["kb_id"] = kbID
	l.hash[hash] = struct{}{}
	l.items = append(l.items, chunk.Document{ID: docID, Text: trimmed, Metadata: meta})
}

func (l *documentList) appendRemoteURLs(ctx context.Context, kbID string, src *knowledge.SourceConfig) error {
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
		return fmt.Errorf("knowledge: url source requires url or urls")
	}
	for _, raw := range urls {
		result, err := fetchRemoteDocument(ctx, raw)
		if err != nil {
			return err
		}
		if result.text == "" {
			continue
		}
		meta := map[string]any{
			"source_type":  string(knowledge.SourceTypeURL),
			"source_url":   raw,
			"content_type": result.contentType,
			"bytes":        result.size,
		}
		if result.filename != "" {
			meta["filename"] = result.filename
		}
		if result.pdfStats != nil {
			logPDFReadability(ctx, raw, *result.pdfStats)
			meta["pdf_readability"] = encodePDFStats(*result.pdfStats)
		}
		l.appendDocument(kbID, raw, result.text, meta)
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

func fetchRemoteDocument(ctx context.Context, rawURL string) (remoteFetchResult, error) {
	handle, size, err := downloadToTemp(ctx, rawURL, 0)
	if err != nil {
		return remoteFetchResult{}, fmt.Errorf("knowledge: download url %q: %w", rawURL, err)
	}
	defer handle.Cleanup()

	path, ok := handle.AsFilePath()
	if !ok || path == "" {
		return remoteFetchResult{}, fmt.Errorf("knowledge: downloaded url %q missing file path", rawURL)
	}
	mime := normalizeContentType(path, handle.MIME())
	filename := filenameFromURL(rawURL)

	if isPDFContentType(mime) {
		result, err := pdfExtractor(ctx, path)
		if err != nil {
			return remoteFetchResult{}, fmt.Errorf("knowledge: extract pdf %q: %w", rawURL, err)
		}
		return remoteFetchResult{
			text:        strings.TrimSpace(normalizeRemoteText(result.Text)),
			contentType: mime,
			size:        size,
			filename:    filename,
			pdfStats:    &result.Stats,
		}, nil
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return remoteFetchResult{}, fmt.Errorf("knowledge: read url %q: %w", rawURL, readErr)
	}
	if len(data) > MaxMarkdownFileSizeBytes {
		return remoteFetchResult{}, fmt.Errorf(
			"knowledge: url %q exceeds maximum size of %d bytes",
			rawURL,
			MaxMarkdownFileSizeBytes,
		)
	}
	text, decodeErr := decodeRemoteText(data, mime)
	if decodeErr != nil {
		return remoteFetchResult{}, fmt.Errorf("knowledge: decode url %q: %w", rawURL, decodeErr)
	}
	return remoteFetchResult{
		text:        strings.TrimSpace(text),
		contentType: mime,
		size:        size,
		filename:    filename,
	}, nil
}

func decodeRemoteText(data []byte, mime string) (string, error) {
	if utf8.Valid(data) {
		return normalizeRemoteText(string(data)), nil
	}
	enc, name, _ := charset.DetermineEncoding(data, mime)
	reader := transform.NewReader(bytes.NewReader(data), enc.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("transcode from %s: %w", name, err)
	}
	if !utf8.Valid(decoded) {
		return "", fmt.Errorf("transcoded result invalid utf-8")
	}
	return normalizeRemoteText(string(decoded)), nil
}

func normalizeRemoteText(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func normalizeContentType(path string, raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "application/octet-stream") {
		if detected, err := mimetype.DetectFile(path); err == nil && detected != nil {
			return detected.String()
		}
	}
	return value
}

func isPDFContentType(contentType string) bool {
	if contentType == "" {
		return false
	}
	lowered := strings.ToLower(contentType)
	return strings.HasPrefix(lowered, "application/pdf")
}

func filenameFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	name := path.Base(parsed.Path)
	if name == "." || name == "/" {
		return ""
	}
	return name
}

func encodePDFStats(stats pdftext.Stats) map[string]any {
	return map[string]any{
		"rune_count":          stats.RuneCount,
		"word_count":          stats.WordCount,
		"space_count":         stats.SpaceCount,
		"line_count":          stats.LineCount,
		"average_word_length": stats.AverageWordLength,
		"space_ratio":         stats.SpaceRatio,
		"fallback_used":       stats.FallbackUsed,
	}
}

func extractPDF(ctx context.Context, path string) (pdftext.Result, error) {
	extractor, err := pdftext.Default()
	if err != nil {
		return pdftext.Result{}, fmt.Errorf("knowledge: initialize pdf extractor: %w", err)
	}
	return extractor.ExtractFile(ctx, path, pdfRuneLimit(ctx))
}

func pdfRuneLimit(ctx context.Context) int64 {
	limit := int64(attachment.MaxPDFExtractChars)
	if cfg := appconfig.FromContext(ctx); cfg != nil && cfg.Attachments.PDFExtractMaxChars > 0 {
		limit = int64(cfg.Attachments.PDFExtractMaxChars)
	}
	return limit
}

func logPDFReadability(ctx context.Context, source string, stats pdftext.Stats) {
	if stats.FallbackUsed && stats.IsReadable() {
		logger.FromContext(ctx).Debug(
			"PDF extraction fallback succeeded",
			"source", source,
			"space_ratio", stats.SpaceRatio,
			"avg_word_length", stats.AverageWordLength,
		)
		return
	}
	if stats.IsReadable() {
		return
	}
	issues := stats.Issues()
	if len(issues) == 0 {
		return
	}
	logger.FromContext(ctx).Warn(
		"PDF extraction readability issues",
		"source", source,
		"issues", strings.Join(issues, ", "),
		"space_ratio", stats.SpaceRatio,
		"avg_word_length", stats.AverageWordLength,
		"fallback_used", stats.FallbackUsed,
	)
}

func hashContent(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:16])
}
