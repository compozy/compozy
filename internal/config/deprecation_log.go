package config

import (
	"context"
	"log/slog"
	"sync"
)

// maxBufferedDeprecationWarnings bounds the warnings held between BufferDeprecationWarnings and
// the next RouteDeprecationWarnings. Each alias warns once per process or per file, so boot stays
// far below it.
const maxBufferedDeprecationWarnings = 256

// deprecationWarnings routes the v0.5.1 input-alias warnings (agent kind, config table, config
// paths, request kind) to the daemon logger. Removal with the aliases: v0.5.2.
var deprecationWarnings struct {
	mu       sync.Mutex
	logger   *slog.Logger
	buffered *deprecationBuffer
}

// BufferDeprecationWarnings holds alias deprecation warnings in memory until the next
// RouteDeprecationWarnings call replays them. The daemon calls it before its first config load,
// because its logger depends on that config.
func BufferDeprecationWarnings() {
	buffer := &deprecationBuffer{}
	deprecationWarnings.mu.Lock()
	deprecationWarnings.buffered = buffer
	deprecationWarnings.logger = slog.New(buffer)
	deprecationWarnings.mu.Unlock()
	agentKindAliasWarnings.Clear()
	configTableAliasWarnings.Clear()
}

// RouteDeprecationWarnings sends alias deprecation warnings to logger. A nil logger restores
// slog.Default. Warnings held by BufferDeprecationWarnings are replayed to logger and stay
// deduplicated, so each still fires once per process. Without a buffer, routing re-arms the
// once-per-process deduplication for the new logger.
func RouteDeprecationWarnings(logger *slog.Logger) {
	deprecationWarnings.mu.Lock()
	buffer := deprecationWarnings.buffered
	deprecationWarnings.buffered = nil
	deprecationWarnings.logger = logger
	deprecationWarnings.mu.Unlock()
	if buffer != nil {
		target := logger
		if target == nil {
			target = slog.Default()
		}
		buffer.replay(target)
		return
	}
	agentKindAliasWarnings.Clear()
	configTableAliasWarnings.Clear()
}

// DeprecationLogger returns the logger that receives alias deprecation warnings.
func DeprecationLogger() *slog.Logger {
	deprecationWarnings.mu.Lock()
	defer deprecationWarnings.mu.Unlock()
	if deprecationWarnings.logger == nil {
		return slog.Default()
	}
	return deprecationWarnings.logger
}

// deprecationBuffer is a slog.Handler that keeps records for a later replay. Callers log through
// DeprecationLogger with record attributes only, so WithAttrs and WithGroup are not needed.
type deprecationBuffer struct {
	mu      sync.Mutex
	records []slog.Record
}

func (b *deprecationBuffer) Enabled(context.Context, slog.Level) bool { return true }

func (b *deprecationBuffer) Handle(_ context.Context, record slog.Record) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.records) < maxBufferedDeprecationWarnings {
		b.records = append(b.records, record.Clone())
	}
	return nil
}

func (b *deprecationBuffer) WithAttrs([]slog.Attr) slog.Handler { return b }

func (b *deprecationBuffer) WithGroup(string) slog.Handler { return b }

func (b *deprecationBuffer) replay(logger *slog.Logger) {
	b.mu.Lock()
	records := b.records
	b.records = nil
	b.mu.Unlock()
	ctx := context.Background()
	for _, record := range records {
		attrs := make([]slog.Attr, 0, record.NumAttrs())
		record.Attrs(func(attr slog.Attr) bool {
			attrs = append(attrs, attr)
			return true
		})
		logger.LogAttrs(ctx, record.Level, record.Message, attrs...)
	}
}
