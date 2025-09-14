package attachment

import (
	"context"
	"io"
	"strings"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	pdf "github.com/ledongthuc/pdf"
)

// resolverFunc defines the signature for attachment resolver functions
type resolverFunc func(context.Context, EffectiveItem) (llmadapter.ContentPart, func())

// resolvers maps attachment types to their resolver functions
var resolvers = map[Type]resolverFunc{
	TypeImage: resolveOneImage,
	TypeAudio: resolveOneAudio,
	TypeVideo: resolveOneVideo,
	TypePDF:   resolveOnePDF,
	TypeFile:  resolveOneFile,
}

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
	for i := range items {
		it := items[i]
		if resolver, exists := resolvers[it.Att.Type()]; exists {
			if p, c := resolver(ctx, it); p != nil {
				parts = append(parts, p)
				if c != nil {
					cleanups = append(cleanups, c)
				}
			}
		}
		// ignore unknown types
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
func resolveOneImage(ctx context.Context, it EffectiveItem) (llmadapter.ContentPart, func()) {
	if it.Att == nil || it.Att.Type() != TypeImage {
		return nil, nil
	}
	resolved, err := it.Att.Resolve(ctx, it.CWD)
	if err != nil || resolved == nil {
		logger.FromContext(ctx).Debug("Skip attachment: resolve failed", "type", string(it.Att.Type()), "error", err)
		return nil, nil
	}
	if u, ok := resolved.AsURL(); ok && u != "" {
		return llmadapter.ImageURLPart{URL: u, Detail: detailFromMeta(it.Att.Meta())}, resolved.Cleanup
	}
	if p, ok := resolved.AsFilePath(); ok && p != "" {
		bp := buildBinaryPart(ctx, resolved, p)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	return nil, nil
}

// resolveOneAudio resolves a single EffectiveItem (audio) into a binary content
// part and optional cleanup function. Failures return (nil, nil).
func resolveOneAudio(ctx context.Context, it EffectiveItem) (llmadapter.ContentPart, func()) {
	if it.Att == nil || it.Att.Type() != TypeAudio {
		return nil, nil
	}
	resolved, err := it.Att.Resolve(ctx, it.CWD)
	if err != nil || resolved == nil {
		logger.FromContext(ctx).Debug("Skip attachment: resolve failed", "type", string(it.Att.Type()), "error", err)
		return nil, nil
	}
	if p, ok := resolved.AsFilePath(); ok && p != "" {
		bp := buildBinaryPart(ctx, resolved, p)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	if u, ok := resolved.AsURL(); ok && u != "" {
		bp := buildBinaryPart(ctx, resolved, u)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	return nil, nil
}

// resolveOneVideo resolves a single EffectiveItem (video) into a binary content
// part and optional cleanup function. Failures return (nil, nil).
func resolveOneVideo(ctx context.Context, it EffectiveItem) (llmadapter.ContentPart, func()) {
	if it.Att == nil || it.Att.Type() != TypeVideo {
		return nil, nil
	}
	resolved, err := it.Att.Resolve(ctx, it.CWD)
	if err != nil || resolved == nil {
		logger.FromContext(ctx).Debug("Skip attachment: resolve failed", "type", string(it.Att.Type()), "error", err)
		return nil, nil
	}
	if p, ok := resolved.AsFilePath(); ok && p != "" {
		bp := buildBinaryPart(ctx, resolved, p)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	if u, ok := resolved.AsURL(); ok && u != "" {
		bp := buildBinaryPart(ctx, resolved, u)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	return nil, nil
}

// resolveOnePDF converts a PDF to text when possible, with binary fallback
func resolveOnePDF(ctx context.Context, it EffectiveItem) (llmadapter.ContentPart, func()) {
	if it.Att == nil || it.Att.Type() != TypePDF {
		return nil, nil
	}
	resolved, err := it.Att.Resolve(ctx, it.CWD)
	if err != nil || resolved == nil {
		logger.FromContext(ctx).Debug("Skip attachment: resolve failed", "type", string(it.Att.Type()), "error", err)
		return nil, nil
	}
	if p, ok := resolved.AsFilePath(); ok && p != "" {
		text, xerr := extractTextFromPDF(p)
		if xerr == nil && strings.TrimSpace(text) != "" {
			return llmadapter.TextPart{Text: text}, resolved.Cleanup
		}
		logger.FromContext(ctx).Debug("PDF text extraction failed; falling back to binary", "error", xerr)
		bp := buildBinaryPart(ctx, resolved, p)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	if u, ok := resolved.AsURL(); ok && u != "" {
		bp := buildBinaryPart(ctx, resolved, u)
		if bp != nil {
			return bp, resolved.Cleanup
		}
	}
	return nil, nil
}

// resolveOneFile converts text files to TextPart; others to BinaryPart
func resolveOneFile(ctx context.Context, it EffectiveItem) (llmadapter.ContentPart, func()) {
	if it.Att == nil || it.Att.Type() != TypeFile {
		return nil, nil
	}
	resolved, err := it.Att.Resolve(ctx, it.CWD)
	if err != nil || resolved == nil {
		logger.FromContext(ctx).Debug("Skip attachment: resolve failed", "type", string(it.Att.Type()), "error", err)
		return nil, nil
	}
	// Prefer MIME-based handling first; derive a ref for logging.
	var pathRef string
	if p, ok := resolved.AsFilePath(); ok && p != "" {
		pathRef = p
	} else if u, ok := resolved.AsURL(); ok && u != "" {
		pathRef = u
	}
	mime := resolved.MIME()
	if strings.HasPrefix(mime, "text/") {
		rc, oerr := resolved.Open()
		if oerr == nil {
			const maxTextBytes = 5 * 1024 * 1024
			b, rerr := io.ReadAll(io.LimitReader(rc, maxTextBytes+1))
			_ = rc.Close()
			if rerr == nil {
				if len(b) > maxTextBytes {
					logger.FromContext(ctx).Warn("Text file too large; truncating", "limit", maxTextBytes)
					b = b[:maxTextBytes]
				}
				return llmadapter.TextPart{Text: string(b)}, resolved.Cleanup
			}
		}
	}
	bp := buildBinaryPart(ctx, resolved, pathRef)
	if bp != nil {
		return bp, resolved.Cleanup
	}
	return nil, nil
}

// extractTextFromPDF reads a PDF file and extracts plain text
func extractTextFromPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	rd, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	const maxExtractChars = 1_000_000
	var sb strings.Builder
	if _, err := io.Copy(&sb, io.LimitReader(rd, maxExtractChars)); err != nil {
		return "", err
	}
	return sb.String(), nil
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

func buildBinaryPart(ctx context.Context, r Resolved, path string) llmadapter.ContentPart {
	rc, oerr := r.Open()
	if oerr != nil {
		logger.FromContext(ctx).Debug("Skip attachment: open failed", "path", path, "error", oerr)
		return nil
	}
	defer rc.Close()
	limit := int64(10 * 1024 * 1024)
	if ac := appconfig.FromContext(ctx); ac != nil && ac.Attachments.MaxDownloadSizeBytes > 0 {
		limit = ac.Attachments.MaxDownloadSizeBytes
	}
	b, rerr := io.ReadAll(io.LimitReader(rc, limit+1))
	if rerr != nil {
		logger.FromContext(ctx).Debug("Skip attachment: read failed", "path", path, "error", rerr)
		return nil
	}
	if int64(len(b)) > limit {
		logger.FromContext(ctx).Warn("Binary attachment exceeds limit; truncating", "limit_bytes", limit, "path", path)
		b = b[:limit]
	}
	return llmadapter.BinaryPart{MIMEType: r.MIME(), Data: b}
}
