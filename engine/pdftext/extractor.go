package pdftext

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// Config controls Extractor construction.
type Config struct {
	Pool pdfium.Pool
}

// Extractor provides text extraction capabilities backed by PDFium.
type Extractor struct {
	pool     pdfium.Pool
	ownsPool bool
}

const (
	defaultPoolMinSize = 2
	defaultPoolMaxSize = 8
)

// New creates a new Extractor. When cfg.Pool is nil, a WebAssembly-backed pool is created.
func New(cfg Config) (*Extractor, error) {
	pool := cfg.Pool
	ownsPool := false
	if pool == nil {
		size := min(max(runtime.NumCPU(), defaultPoolMinSize), defaultPoolMaxSize)
		initCfg := webassembly.Config{
			MinIdle:      1,
			MaxIdle:      size,
			MaxTotal:     size,
			ReuseWorkers: true,
		}
		var err error
		pool, err = webassembly.Init(initCfg)
		if err != nil {
			return nil, fmt.Errorf("pdftext: initialize pdfium pool: %w", err)
		}
		ownsPool = true
	}
	return &Extractor{pool: pool, ownsPool: ownsPool}, nil
}

// Close releases pool resources when the extractor created its own pool.
func (e *Extractor) Close() error {
	if e == nil || !e.ownsPool || e.pool == nil {
		return nil
	}
	return e.pool.Close()
}

var (
	defaultOnce      sync.Once
	defaultExtractor *Extractor
	defaultErr       error
)

// Default returns a process-wide shared extractor.
func Default() (*Extractor, error) {
	defaultOnce.Do(func() {
		defaultExtractor, defaultErr = New(Config{})
	})
	return defaultExtractor, defaultErr
}

// Result captures the extracted text and associated statistics.
type Result struct {
	Text  string
	Stats Stats
}

// ExtractFile reads a PDF from disk and extracts text up to runeLimit characters (<=0 means no limit).
func (e *Extractor) ExtractFile(ctx context.Context, path string, runeLimit int64) (Result, error) {
	if e == nil {
		return Result{}, errors.New("pdftext: extractor is nil")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("pdftext: read file %q: %w", path, err)
	}
	return e.ExtractBytes(ctx, data, runeLimit)
}

// ExtractBytes extracts text from a PDF byte slice.
func (e *Extractor) ExtractBytes(ctx context.Context, data []byte, runeLimit int64) (Result, error) {
	if err := e.validateExtractBytesInput(data); err != nil {
		return Result{}, err
	}
	instance, err := e.acquireInstance(ctx)
	if err != nil {
		return Result{}, err
	}
	defer instance.Close()
	doc, closeDoc, err := e.openDocument(instance, data)
	if err != nil {
		return Result{}, err
	}
	defer closeDoc()
	pageCount, err := e.pageCount(instance, doc)
	if err != nil {
		return Result{}, err
	}
	if pageCount == 0 {
		return Result{Stats: Stats{}}, nil
	}
	return e.extractWithFallback(ctx, instance, doc, runeLimit, pageCount)
}

func (e *Extractor) validateExtractBytesInput(data []byte) error {
	if e == nil {
		return errors.New("pdftext: extractor is nil")
	}
	if e.pool == nil {
		return errors.New("pdftext: extractor pool is uninitialized")
	}
	if len(data) == 0 {
		return errors.New("pdftext: empty pdf payload")
	}
	return nil
}

func (e *Extractor) acquireInstance(ctx context.Context) (pdfium.Pdfium, error) {
	instance, err := e.pool.GetInstanceWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("pdftext: acquire pdfium instance: %w", err)
	}
	return instance, nil
}

func (e *Extractor) openDocument(
	instance pdfium.Pdfium,
	data []byte,
) (*responses.OpenDocument, func(), error) {
	pdfBytes := data
	doc, err := instance.OpenDocument(&requests.OpenDocument{File: &pdfBytes})
	if err != nil {
		return nil, nil, fmt.Errorf("pdftext: open document: %w", err)
	}
	cleanup := func() {
		if _, cerr := instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document}); cerr != nil {
			_ = cerr
		}
	}
	return doc, cleanup, nil
}

func (e *Extractor) pageCount(
	instance pdfium.Pdfium,
	doc *responses.OpenDocument,
) (int, error) {
	resp, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return 0, fmt.Errorf("pdftext: get page count: %w", err)
	}
	return resp.PageCount, nil
}

func (e *Extractor) extractWithFallback(
	ctx context.Context,
	instance pdfium.Pdfium,
	doc *responses.OpenDocument,
	runeLimit int64,
	pageCount int,
) (Result, error) {
	text, stats, err := e.extractPlain(ctx, instance, doc, runeLimit, pageCount)
	if err != nil {
		return Result{}, err
	}
	if stats.IsReadable() {
		return Result{Text: text, Stats: stats}, nil
	}
	fallbackText, fallbackStats, fallbackErr := e.extractStructured(ctx, instance, doc, runeLimit, pageCount)
	if fallbackErr != nil {
		return Result{Text: text, Stats: stats}, fmt.Errorf("pdftext: structured fallback failed: %w", fallbackErr)
	}
	fallbackStats.FallbackUsed = true
	return Result{Text: fallbackText, Stats: fallbackStats}, nil
}

func (e *Extractor) extractPlain(
	ctx context.Context,
	instance pdfium.Pdfium,
	doc *responses.OpenDocument,
	runeLimit int64,
	pageCount int,
) (string, Stats, error) {
	limiter := newTextLimiter(runeLimit)
	for page := range pageCount {
		if err := ctx.Err(); err != nil {
			return "", Stats{}, err
		}
		resp, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    page,
				},
			},
		})
		if err != nil {
			return "", Stats{}, fmt.Errorf("pdftext: get plain text (page %d): %w", page+1, err)
		}
		normalized := normalizePlainText(resp.Text)
		if page > 0 {
			if limiter.AppendString("\n") {
				break
			}
		}
		if limiter.AppendString(normalized) {
			break
		}
	}
	text := strings.TrimSpace(limiter.String())
	return text, computeStats(text, false), nil
}

func normalizePlainText(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}

type textLimiter struct {
	builder   strings.Builder
	runeLimit int64
	runes     int64
}

func newTextLimiter(limit int64) *textLimiter {
	return &textLimiter{runeLimit: limit}
}

func (l *textLimiter) AppendString(s string) bool {
	if s == "" {
		return l.reachedLimit()
	}
	if l.runeLimit <= 0 {
		l.builder.WriteString(s)
		l.runes += int64(utf8.RuneCountInString(s))
		return l.reachedLimit()
	}
	for _, r := range s {
		l.builder.WriteRune(r)
		l.runes++
		if l.reachedLimit() {
			return true
		}
	}
	return l.reachedLimit()
}

func (l *textLimiter) String() string {
	return l.builder.String()
}

func (l *textLimiter) reachedLimit() bool {
	return l.runeLimit > 0 && l.runes >= l.runeLimit
}
