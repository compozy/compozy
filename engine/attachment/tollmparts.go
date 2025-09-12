package attachment

import (
	"context"
	"io"
	"strings"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

// ToContentPartsFromEffective resolves the provided effective items and converts
// supported attachments into LLM content parts. Supported mappings:
// - Image + URL -> ImageURLPart
// - Image + Path -> BinaryPart (image/* MIME)
// - Audio + Path/URL -> BinaryPart (audio/* MIME)
// - Video + Path/URL -> BinaryPart (video/* MIME)
// It returns the parts and a cleanup function that MUST be deferred by caller.
func ToContentPartsFromEffective(ctx context.Context, items []EffectiveItem) ([]llmadapter.ContentPart, func(), error) {
	if len(items) == 0 {
		return nil, func() {}, nil
	}
	parts := make([]llmadapter.ContentPart, 0, len(items))
	cleanups := make([]func(), 0)
	log := logger.FromContext(ctx)
	for i := range items {
		if p, c := resolveOneImage(ctx, log, items[i]); p != nil {
			parts = append(parts, p)
			if c != nil {
				cleanups = append(cleanups, c)
			}
		}
		if p, c := resolveOneAudio(ctx, log, items[i]); p != nil {
			parts = append(parts, p)
			if c != nil {
				cleanups = append(cleanups, c)
			}
		}
		if p, c := resolveOneVideo(ctx, log, items[i]); p != nil {
			parts = append(parts, p)
			if c != nil {
				cleanups = append(cleanups, c)
			}
		}
		if p, c := resolveOnePDF(ctx, log, items[i]); p != nil {
			parts = append(parts, p)
			if c != nil {
				cleanups = append(cleanups, c)
			}
		}
		if p, c := resolveOneFile(ctx, log, items[i]); p != nil {
			parts = append(parts, p)
			if c != nil {
				cleanups = append(cleanups, c)
			}
		}
	}
	return parts, func() {
		for i := range cleanups {
			if cleanups[i] != nil {
				cleanups[i]()
			}
		}
	}, nil
}

// resolveOneImage resolves a single EffectiveItem (image-only phase) into a content
// part and optional cleanup function. Non-image or failures return (nil, nil).
func resolveOneImage(ctx context.Context, log logger.Logger, it EffectiveItem) (llmadapter.ContentPart, func()) {
	if it.Att == nil || it.Att.Type() != TypeImage {
		return nil, nil
	}
	resolved, err := it.Att.Resolve(ctx, it.CWD)
	if err != nil || resolved == nil {
		log.Debug("Skip attachment: resolve failed", "type", string(it.Att.Type()), "error", err)
		return nil, nil
	}
	if u, ok := resolved.AsURL(); ok && u != "" {
		return llmadapter.ImageURLPart{URL: u, Detail: detailFromMeta(it.Att.Meta())}, nil
	}
	if p, ok := resolved.AsFilePath(); ok && p != "" {
		bp := buildBinaryPart(log, resolved, p)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	return nil, nil
}

// resolveOneAudio resolves a single EffectiveItem (audio) into a binary content
// part and optional cleanup function. Failures return (nil, nil).
func resolveOneAudio(ctx context.Context, log logger.Logger, it EffectiveItem) (llmadapter.ContentPart, func()) {
	if it.Att == nil || it.Att.Type() != TypeAudio {
		return nil, nil
	}
	resolved, err := it.Att.Resolve(ctx, it.CWD)
	if err != nil || resolved == nil {
		log.Debug("Skip attachment: resolve failed", "type", string(it.Att.Type()), "error", err)
		return nil, nil
	}
	if p, ok := resolved.AsFilePath(); ok && p != "" {
		bp := buildBinaryPart(log, resolved, p)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	return nil, nil
}

// resolveOneVideo resolves a single EffectiveItem (video) into a binary content
// part and optional cleanup function. Failures return (nil, nil).
func resolveOneVideo(ctx context.Context, log logger.Logger, it EffectiveItem) (llmadapter.ContentPart, func()) {
	if it.Att == nil || it.Att.Type() != TypeVideo {
		return nil, nil
	}
	resolved, err := it.Att.Resolve(ctx, it.CWD)
	if err != nil || resolved == nil {
		log.Debug("Skip attachment: resolve failed", "type", string(it.Att.Type()), "error", err)
		return nil, nil
	}
	if p, ok := resolved.AsFilePath(); ok && p != "" {
		bp := buildBinaryPart(log, resolved, p)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	return nil, nil
}

// resolveOnePDF converts a PDF to text when possible, with binary fallback
func resolveOnePDF(ctx context.Context, log logger.Logger, it EffectiveItem) (llmadapter.ContentPart, func()) {
	if it.Att == nil || it.Att.Type() != TypePDF {
		return nil, nil
	}
	resolved, err := it.Att.Resolve(ctx, it.CWD)
	if err != nil || resolved == nil {
		log.Debug("Skip attachment: resolve failed", "type", string(it.Att.Type()), "error", err)
		return nil, nil
	}
	if p, ok := resolved.AsFilePath(); ok && p != "" {
		text, xerr := extractTextFromPDF(p)
		if xerr == nil && strings.TrimSpace(text) != "" {
			return llmadapter.TextPart{Text: text}, resolved.Cleanup
		}
		log.Debug("PDF text extraction failed; falling back to binary", "error", xerr)
		bp := buildBinaryPart(log, resolved, p)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	return nil, nil
}

// resolveOneFile converts text files to TextPart; others to BinaryPart
func resolveOneFile(ctx context.Context, log logger.Logger, it EffectiveItem) (llmadapter.ContentPart, func()) {
	if it.Att == nil || it.Att.Type() != TypeFile {
		return nil, nil
	}
	resolved, err := it.Att.Resolve(ctx, it.CWD)
	if err != nil || resolved == nil {
		log.Debug("Skip attachment: resolve failed", "type", string(it.Att.Type()), "error", err)
		return nil, nil
	}
	if p, ok := resolved.AsFilePath(); ok && p != "" {
		mime := resolved.MIME()
		if strings.HasPrefix(mime, "text/") {
			rc, oerr := resolved.Open()
			if oerr == nil {
				const maxTextBytes = 5 * 1024 * 1024 // 5MB guard
				b, rerr := io.ReadAll(io.LimitReader(rc, maxTextBytes+1))
				_ = rc.Close()
				if rerr == nil {
					if len(b) > maxTextBytes {
						log.Warn("Text file too large; truncating", "limit", maxTextBytes)
						b = b[:maxTextBytes]
					}
					return llmadapter.TextPart{Text: string(b)}, resolved.Cleanup
				}
			}
		}
		bp := buildBinaryPart(log, resolved, p)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	return nil, nil
}

// extractTextFromPDF reads a PDF file and extracts plain text
func extractTextFromPDF(path string) (string, error) {
	reader, f, err := model.NewPdfReaderFromFile(path, nil)
	if err != nil {
		return "", err
	}
	defer func() {
		if f != nil {
			_ = f.Close()
		}
	}()
	n, err := reader.GetNumPages()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		page, err := reader.GetPage(i)
		if err != nil {
			continue
		}
		ex, err := extractor.New(page)
		if err != nil {
			continue
		}
		txt, err := ex.ExtractText()
		if err != nil {
			continue
		}
		if txt != "" {
			sb.WriteString(txt)
			sb.WriteString("\n\n")
		}
	}
	out := sb.String()
	// Truncate extremely large extracted text to protect memory/token budgets
	const maxExtractChars = 1_000_000 // ~1MB of UTF-8 chars
	if len(out) > maxExtractChars {
		out = out[:maxExtractChars]
	}
	return out, nil
}

func detailFromMeta(meta map[string]any) string {
	if meta == nil {
		return ""
	}
	if v, ok := meta["image_detail"].(string); ok {
		return v
	}
	return ""
}

func buildBinaryPart(log logger.Logger, r Resolved, path string) llmadapter.ContentPart {
	rc, oerr := r.Open()
	if oerr != nil {
		log.Debug("Skip attachment: open failed", "path", path, "error", oerr)
		return nil
	}
	b, rerr := io.ReadAll(rc)
	_ = rc.Close()
	if rerr != nil {
		log.Debug("Skip attachment: read failed", "path", path, "error", rerr)
		return nil
	}
	return llmadapter.BinaryPart{MIMEType: r.MIME(), Data: b}
}
