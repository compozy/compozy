package bridgesdk

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

// InboundBatch groups a short burst of inbound bridge envelopes under one routing identity.
type InboundBatch struct {
	Key       string                             `json:"key"`
	Items     []bridgepkg.InboundMessageEnvelope `json:"items"`
	CreatedAt time.Time                          `json:"created_at"`
	UpdatedAt time.Time                          `json:"updated_at"`
}

// InboundBatchDispatch handles one flushed inbound batch.
type InboundBatchDispatch func(context.Context, InboundBatch) error

// InboundBatcherConfig configures the debounce-based inbound batcher.
type InboundBatcherConfig struct {
	Context        context.Context
	Delay          time.Duration
	SplitDelay     time.Duration
	SplitThreshold int
	Dispatch       InboundBatchDispatch
	Now            func() time.Time
}

type pendingInboundBatch struct {
	batch     InboundBatch
	lastChunk int
	timer     *time.Timer
	timerID   uint64
}

// InboundBatcher coalesces rapid-fire inbound envelopes under one routing identity.
type InboundBatcher struct {
	ctx    context.Context
	cancel context.CancelFunc

	delay          time.Duration
	splitDelay     time.Duration
	splitThreshold int
	dispatch       InboundBatchDispatch
	now            func() time.Time

	mu      sync.Mutex
	closed  bool
	pending map[string]*pendingInboundBatch
	timerID uint64
	wg      sync.WaitGroup
	err     error
}

// NewInboundBatcher constructs the debounce-based inbound batcher.
func NewInboundBatcher(config InboundBatcherConfig) (*InboundBatcher, error) {
	if config.Dispatch == nil {
		return nil, errors.New("bridgesdk: inbound batch dispatch is required")
	}
	if config.Now == nil {
		config.Now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if config.Context == nil {
		config.Context = context.Background()
	}
	ctx, cancel := context.WithCancel(config.Context)
	if config.Delay < 0 {
		cancel()
		return nil, errors.New("bridgesdk: inbound batch delay must be >= 0")
	}
	if config.SplitDelay <= 0 {
		config.SplitDelay = config.Delay
	}
	return &InboundBatcher{
		ctx:            ctx,
		cancel:         cancel,
		delay:          config.Delay,
		splitDelay:     config.SplitDelay,
		splitThreshold: config.SplitThreshold,
		dispatch:       config.Dispatch,
		now:            config.Now,
		pending:        make(map[string]*pendingInboundBatch),
	}, nil
}

// Enqueue appends one inbound envelope to the routing-identity batch.
func (b *InboundBatcher) Enqueue(envelope bridgepkg.InboundMessageEnvelope) error {
	if b == nil {
		return errors.New("bridgesdk: inbound batcher is required")
	}
	if err := envelope.Validate(); err != nil {
		return err
	}

	key := InboundBatchKey(envelope)
	if key == "" {
		return errors.New("bridgesdk: inbound batch key is required")
	}

	if b.delay == 0 {
		batch, err := b.immediateBatch(key, envelope)
		if err != nil {
			return err
		}
		return b.dispatch(b.ctx, batch)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return errors.New("bridgesdk: inbound batcher is closed")
	}
	if b.err != nil {
		return b.err
	}

	now := b.now()
	itemCopy := cloneInboundEnvelope(envelope)
	pending := b.pending[key]
	if pending == nil {
		pending = &pendingInboundBatch{
			batch: InboundBatch{
				Key:       key,
				Items:     []bridgepkg.InboundMessageEnvelope{itemCopy},
				CreatedAt: now,
				UpdatedAt: now,
			},
			lastChunk: len(strings.TrimSpace(envelope.Content.Text)),
		}
		b.pending[key] = pending
	} else {
		pending.batch.Items = append(pending.batch.Items, itemCopy)
		pending.batch.UpdatedAt = now
		pending.lastChunk = len(strings.TrimSpace(envelope.Content.Text))
	}

	delay := b.delay
	if b.splitThreshold > 0 && pending.lastChunk >= b.splitThreshold {
		delay = b.splitDelay
	}
	b.scheduleTimerLocked(key, pending, delay)
	return nil
}

func (b *InboundBatcher) immediateBatch(
	key string,
	envelope bridgepkg.InboundMessageEnvelope,
) (InboundBatch, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return InboundBatch{}, errors.New("bridgesdk: inbound batcher is closed")
	}
	if b.err != nil {
		return InboundBatch{}, b.err
	}

	now := b.now()
	return InboundBatch{
		Key:       key,
		Items:     []bridgepkg.InboundMessageEnvelope{cloneInboundEnvelope(envelope)},
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// FlushAll flushes every pending batch immediately.
func (b *InboundBatcher) FlushAll(ctx context.Context) error {
	if b == nil {
		return errors.New("bridgesdk: inbound batcher is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	b.mu.Lock()
	if b.err != nil {
		err := b.err
		b.mu.Unlock()
		return err
	}
	pending := make([]InboundBatch, 0, len(b.pending))
	for key, entry := range b.pending {
		b.stopTimerLocked(entry)
		pending = append(pending, cloneInboundBatch(entry.batch))
		delete(b.pending, key)
	}
	b.mu.Unlock()

	for _, batch := range pending {
		if err := b.dispatch(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

// Close stops the batcher and cancels any unflushed pending batches.
func (b *InboundBatcher) Close() {
	if b == nil {
		return
	}

	b.cancel()

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	for _, entry := range b.pending {
		b.stopTimerLocked(entry)
	}
	b.pending = make(map[string]*pendingInboundBatch)
	b.mu.Unlock()

	b.wg.Wait()
}

// InboundBatchKey derives the stable routing-identity key used for batching.
func InboundBatchKey(envelope bridgepkg.InboundMessageEnvelope) string {
	parts := [...]string{
		strings.TrimSpace(envelope.BridgeInstanceID),
		strings.TrimSpace(string(envelope.Scope)),
		strings.TrimSpace(envelope.WorkspaceID),
		strings.TrimSpace(envelope.PeerID),
		strings.TrimSpace(envelope.ThreadID),
		strings.TrimSpace(envelope.GroupID),
		strings.TrimSpace(envelope.Sender.ID),
		string(inboundBatchEventFamily(envelope)),
	}

	totalLen := 0
	for _, part := range parts {
		totalLen += decimalLen(len(part)) + 1 + len(part) + 1
	}

	var builder strings.Builder
	builder.Grow(totalLen)
	for idx, part := range parts {
		if idx > 0 {
			builder.WriteByte(';')
		}
		writeBatchKeyPart(&builder, part)
	}
	return builder.String()
}

func inboundBatchEventFamily(envelope bridgepkg.InboundMessageEnvelope) bridgepkg.InboundEventFamily {
	family := envelope.EventFamily.Normalize()
	if family == "" && envelope.Command == nil && envelope.Action == nil && envelope.Reaction == nil {
		return bridgepkg.InboundEventFamilyMessage
	}
	return family
}

func writeBatchKeyPart(builder *strings.Builder, part string) {
	var digits [20]byte
	encodedLen := strconv.AppendInt(digits[:0], int64(len(part)), 10)
	builder.Write(encodedLen)
	builder.WriteByte(':')
	builder.WriteString(part)
}

func decimalLen(value int) int {
	if value == 0 {
		return 1
	}
	digits := 0
	for value > 0 {
		value /= 10
		digits++
	}
	return digits
}

func (b *InboundBatcher) scheduleTimerLocked(key string, pending *pendingInboundBatch, delay time.Duration) {
	b.stopTimerLocked(pending)
	b.timerID++
	pending.timerID = b.timerID
	timerID := pending.timerID
	b.wg.Add(1)
	pending.timer = time.AfterFunc(delay, func() {
		defer b.wg.Done()
		b.flushKey(key, timerID)
	})
}

func (b *InboundBatcher) stopTimerLocked(pending *pendingInboundBatch) {
	if pending.timer == nil {
		return
	}
	if pending.timer.Stop() {
		b.wg.Done()
	}
	pending.timer = nil
}

func (b *InboundBatcher) flushKey(key string, timerID uint64) {
	b.mu.Lock()
	entry, ok := b.pending[key]
	if ok && entry.timerID == timerID && !b.closed {
		delete(b.pending, key)
		entry.timer = nil
	} else {
		ok = false
	}
	b.mu.Unlock()
	if !ok {
		return
	}

	if err := b.dispatch(b.ctx, cloneInboundBatch(entry.batch)); err != nil && !errors.Is(err, context.Canceled) {
		b.mu.Lock()
		if b.err == nil {
			b.err = err
		}
		b.mu.Unlock()
	}
}

func cloneInboundBatch(src InboundBatch) InboundBatch {
	cloned := src
	cloned.Items = make([]bridgepkg.InboundMessageEnvelope, 0, len(src.Items))
	for _, item := range src.Items {
		cloned.Items = append(cloned.Items, cloneInboundEnvelope(item))
	}
	return cloned
}

func cloneInboundEnvelope(src bridgepkg.InboundMessageEnvelope) bridgepkg.InboundMessageEnvelope {
	cloned := src
	if len(cloned.Attachments) > 0 {
		cloned.Attachments = append([]bridgepkg.MessageAttachment(nil), cloned.Attachments...)
	}
	if len(cloned.ProviderMetadata) > 0 {
		cloned.ProviderMetadata = append([]byte(nil), cloned.ProviderMetadata...)
	}
	if cloned.Command != nil {
		command := *cloned.Command
		cloned.Command = &command
	}
	if cloned.Action != nil {
		action := *cloned.Action
		cloned.Action = &action
	}
	if cloned.Reaction != nil {
		reaction := *cloned.Reaction
		cloned.Reaction = &reaction
	}
	if cloned.Conversation != nil {
		conversation := *cloned.Conversation
		cloned.Conversation = &conversation
	}
	return cloned
}
