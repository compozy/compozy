package tokens

import (
	"context"
	"fmt"
	"sync"
	"time"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
)

// AsyncTokenCounter processes token counting in the background
type AsyncTokenCounter struct {
	realCounter memcore.TokenCounter
	queue       chan *tokenCountRequest
	workers     int
	wg          sync.WaitGroup
	metrics     *TokenMetrics
	baseCtx     context.Context
}

type tokenCountRequest struct {
	ctx        context.Context
	memoryRef  string
	text       string
	resultChan chan<- tokenCountResult
}

type tokenCountResult struct {
	count int
	err   error
}

// NewAsyncTokenCounter creates a new async token counter
func NewAsyncTokenCounter(
	ctx context.Context,
	counter memcore.TokenCounter,
	workers int,
	bufferSize int,
) *AsyncTokenCounter {
	if workers <= 0 {
		workers = 10 // Default worker pool size
	}
	if bufferSize <= 0 {
		bufferSize = 1000 // Default buffer size
	}
	atc := &AsyncTokenCounter{
		realCounter: counter,
		queue:       make(chan *tokenCountRequest, bufferSize),
		workers:     workers,
		metrics:     NewTokenMetrics(),
		baseCtx:     context.WithoutCancel(ctx),
	}
	atc.start()
	return atc
}

// NewAsyncTokenCounterWithContext creates a new async token counter with a base context
func NewAsyncTokenCounterWithContext(
	ctx context.Context,
	counter memcore.TokenCounter,
	workers int,
	bufferSize int,
) *AsyncTokenCounter {
	if workers <= 0 {
		workers = 10
	}
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	atc := &AsyncTokenCounter{
		realCounter: counter,
		queue:       make(chan *tokenCountRequest, bufferSize),
		workers:     workers,
		metrics:     NewTokenMetrics(),
		baseCtx:     context.WithoutCancel(ctx),
	}
	atc.start()
	return atc
}

// start initializes the worker pool
func (atc *AsyncTokenCounter) start() {
	for i := 0; i < atc.workers; i++ {
		atc.wg.Add(1)
		go atc.worker(atc.baseCtx, i)
	}
}

// worker processes token count requests
func (atc *AsyncTokenCounter) worker(ctx context.Context, id int) {
	log := logger.FromContext(ctx)
	defer atc.wg.Done()
	for req := range atc.queue {
		start := time.Now()
		count, err := atc.realCounter.CountTokens(req.ctx, req.text)
		atc.metrics.RecordDuration(time.Since(start))
		if req.resultChan != nil {
			req.resultChan <- tokenCountResult{
				count: count,
				err:   err,
			}
		}
		if err != nil {
			log.Error("Failed to count tokens",
				"error", err,
				"memory_ref", req.memoryRef,
				"worker_id", id,
			)
			atc.metrics.IncrementErrors()
		} else {
			atc.metrics.IncrementSuccess()
		}
	}
}

// ProcessAsync queues a message for token counting without blocking
func (atc *AsyncTokenCounter) ProcessAsync(ctx context.Context, memoryRef string, text string) {
	log := logger.FromContext(ctx)
	defer func() {
		if r := recover(); r != nil {
			// Handle panic from sending on closed channel
			log.Warn("Cannot process token count, counter is shut down",
				"memory_ref", memoryRef,
				"panic", r,
			)
			atc.metrics.IncrementDropped()
		}
	}()
	select {
	case atc.queue <- &tokenCountRequest{
		ctx:       ctx,
		memoryRef: memoryRef,
		text:      text,
	}:
		// Successfully queued
	default:
		// Queue full, log and continue
		log.Warn("Token counter queue full, skipping message",
			"memory_ref", memoryRef,
		)
		atc.metrics.IncrementDropped()
	}
}

// ProcessWithResult queues a message and waits for the result
func (atc *AsyncTokenCounter) ProcessWithResult(ctx context.Context, memoryRef string, text string) (int, error) {
	resultChan := make(chan tokenCountResult, 1)
	select {
	case atc.queue <- &tokenCountRequest{
		ctx:        ctx,
		memoryRef:  memoryRef,
		text:       text,
		resultChan: resultChan,
	}:
		// Wait for result with timeout
		select {
		case result := <-resultChan:
			return result.count, result.err
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(5 * time.Second):
			return 0, fmt.Errorf("token counting timeout")
		}
	default:
		return 0, fmt.Errorf("token counter queue full")
	}
}

// Shutdown gracefully stops the async counter
func (atc *AsyncTokenCounter) Shutdown() {
	close(atc.queue)
	atc.wg.Wait()
}
