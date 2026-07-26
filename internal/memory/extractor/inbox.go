package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"strings"
	"time"

	"github.com/compozy/agh/internal/fileutil"
	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const (
	inboxDirName        = "_inbox"
	systemDirName       = "_system"
	processingSuffix    = ".processing"
	inboxFilePerm       = 0o644
	inboxDirPerm        = 0o755
	defaultScannerLimit = 1024 * 1024
)

// ProposalSink is the controller-backed handoff used by the inbox consumer.
type ProposalSink interface {
	ProposeCandidate(context.Context, memcontract.Candidate) (memcontract.Decision, error)
}

// EventSink persists extractor telemetry.
type EventSink interface {
	RecordExtractorEvent(context.Context, Event) error
}

// Producer writes extractor candidates into the daemon-owned inbox.
type Producer struct {
	root      string
	inboxRoot string
	now       func() time.Time
}

// ProducerOption customizes inbox production.
type ProducerOption func(*Producer)

// WithProducerInboxPath overrides the default <root>/_inbox directory.
func WithProducerInboxPath(path string) ProducerOption {
	return func(p *Producer) {
		if strings.TrimSpace(path) != "" {
			p.inboxRoot = filepath.Clean(path)
		}
	}
}

// NewProducer constructs an inbox producer rooted at the memory directory.
func NewProducer(root string, now func() time.Time, opts ...ProducerOption) (*Producer, error) {
	clean, err := cleanRoot(root)
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	producer := &Producer{
		root:      clean,
		inboxRoot: filepath.Join(clean, inboxDirName),
		now:       now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(producer)
		}
	}
	return producer, nil
}

// Write persists one JSONL inbox file for a completed extractor turn.
func (p *Producer) Write(
	ctx context.Context,
	turn memcontract.TurnRecord,
	candidates []memcontract.Candidate,
) (string, int, error) {
	if p == nil {
		return "", 0, errors.New("memory extractor: producer is required")
	}
	if ctx == nil {
		return "", 0, errors.New("memory extractor: write context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", 0, fmt.Errorf("memory extractor: write canceled: %w", err)
	}
	if len(candidates) == 0 {
		return "", 0, nil
	}
	turn, err := normalizeTurn(turn, p.now)
	if err != nil {
		return "", 0, err
	}
	sessionDir, err := inboxSessionDir(p.inboxRoot, turn.SessionID)
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(sessionDir, inboxDirPerm); err != nil {
		return "", 0, fmt.Errorf("memory extractor: ensure inbox session dir: %w", err)
	}

	var lines bytes.Buffer
	for idx := range candidates {
		candidate := enrichCandidate(candidates[idx], turn, p.now)
		encoded, err := json.Marshal(candidate)
		if err != nil {
			return "", 0, fmt.Errorf("memory extractor: encode candidate: %w", err)
		}
		if _, err := lines.Write(encoded); err != nil {
			return "", 0, fmt.Errorf("memory extractor: buffer candidate: %w", err)
		}
		if err := lines.WriteByte('\n'); err != nil {
			return "", 0, fmt.Errorf("memory extractor: buffer candidate newline: %w", err)
		}
	}

	path := filepath.Join(sessionDir, inboxFilename(p.now(), turn.UntilMessageSeq))
	if err := fileutil.AtomicWriteFile(path, lines.Bytes(), inboxFilePerm); err != nil {
		return "", 0, fmt.Errorf("memory extractor: write inbox file: %w", err)
	}
	return path, len(candidates), nil
}

// InboxConsumer drains daemon-owned extractor inbox files through the write controller.
type InboxConsumer struct {
	root        string
	inboxRoot   string
	failuresDir string
	sink        ProposalSink
	events      EventSink
	logger      *slog.Logger
	now         func() time.Time
}

// ConsumerOption customizes inbox consumption.
type ConsumerOption func(*InboxConsumer)

// WithConsumerEventSink records consumer telemetry.
func WithConsumerEventSink(sink EventSink) ConsumerOption {
	return func(c *InboxConsumer) {
		c.events = sink
	}
}

// WithConsumerLogger configures warning output.
func WithConsumerLogger(logger *slog.Logger) ConsumerOption {
	return func(c *InboxConsumer) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithConsumerClock injects deterministic time.
func WithConsumerClock(now func() time.Time) ConsumerOption {
	return func(c *InboxConsumer) {
		if now != nil {
			c.now = now
		}
	}
}

// WithConsumerInboxPath overrides the default <root>/_inbox directory.
func WithConsumerInboxPath(path string) ConsumerOption {
	return func(c *InboxConsumer) {
		if strings.TrimSpace(path) != "" {
			c.inboxRoot = filepath.Clean(path)
		}
	}
}

// WithConsumerFailurePath overrides the default <root>/_system/extractor/failures directory.
func WithConsumerFailurePath(path string) ConsumerOption {
	return func(c *InboxConsumer) {
		if strings.TrimSpace(path) != "" {
			c.failuresDir = filepath.Clean(path)
		}
	}
}

// NewInboxConsumer constructs a FIFO inbox consumer.
func NewInboxConsumer(root string, sink ProposalSink, opts ...ConsumerOption) (*InboxConsumer, error) {
	clean, err := cleanRoot(root)
	if err != nil {
		return nil, err
	}
	if sink == nil {
		return nil, errors.New("memory extractor: proposal sink is required")
	}
	consumer := &InboxConsumer{
		root:        clean,
		inboxRoot:   filepath.Join(clean, inboxDirName),
		failuresDir: filepath.Join(clean, systemDirName, "extractor", "failures"),
		sink:        sink,
		logger:      slog.Default(),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(consumer)
		}
	}
	return consumer, nil
}

// ConsumeResult summarizes one inbox pass.
type ConsumeResult struct {
	Files     int
	Proposed  int
	Failed    int
	Decisions []memcontract.Decision
	Failures  []string
}

// ConsumeOnce processes all currently visible JSONL files in FIFO order.
func (c *InboxConsumer) ConsumeOnce(ctx context.Context) (ConsumeResult, error) {
	if c == nil {
		return ConsumeResult{}, errors.New("memory extractor: consumer is required")
	}
	if ctx == nil {
		return ConsumeResult{}, errors.New("memory extractor: consume context is required")
	}
	files, err := c.pendingFiles()
	if err != nil {
		return ConsumeResult{}, err
	}
	result := ConsumeResult{Decisions: make([]memcontract.Decision, 0), Failures: make([]string, 0)}
	var joined error
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("memory extractor: consume canceled: %w", err)
		}
		result.Files++
		fileResult, fileErr := c.consumeFile(ctx, file)
		result.Proposed += fileResult.Proposed
		result.Failed += fileResult.Failed
		result.Decisions = append(result.Decisions, fileResult.Decisions...)
		result.Failures = append(result.Failures, fileResult.Failures...)
		if fileErr != nil {
			joined = errors.Join(joined, fileErr)
		}
	}
	return result, joined
}
