package common

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

// IndexedBatchError is an error that provides per-index error information
// for batch operations. When ExecuteBatch returns an error implementing this
// interface, the batcher routes index-specific errors to individual entries
// instead of sending the same error to all entries.
type IndexedBatchError interface {
	error
	ErrorForIndex(i int) error
}

type batchEntry[Req, Res any] struct {
	ctx    context.Context
	req    Req
	result chan batchResult[Res]
}

type batchResult[Res any] struct {
	val Res
	err error
}

// BatcherConfig holds the callback functions that customize a Batcher's behavior.
type BatcherConfig[Req, Res any] struct {
	BatchSize  int
	FlushDelay time.Duration

	// ExecuteBatch processes a slice of requests and returns a positionally-
	// corresponding slice of results. If it returns a non-nil error, all
	// items in the batch fail (unless ShouldFallback triggers individual retry).
	ExecuteBatch func(ctx context.Context, items []Req) ([]Res, error)

	// Publish is called once after a successful batch execution.
	Publish func(ctx context.Context) error

	// ShouldFallback inspects an ExecuteBatch error and returns true if
	// the batcher should retry items individually via ExecuteOne.
	ShouldFallback func(err error) bool

	// ExecuteOne processes a single request (used for fallback).
	ExecuteOne func(ctx context.Context, item Req) (Res, error)

	// IsPublishOK inspects a Publish error and returns true if the error
	// should be treated as success (e.g., "no changes to publish").
	IsPublishOK func(err error) bool

	// WarnLog is called when Publish returns an error that IsPublishOK
	// accepts, so the condition is logged rather than silently swallowed.
	WarnLog func(msg string)
}

// Batcher coalesces concurrent requests and executes them in batches,
// calling Publish once per batch instead of once per request.
type Batcher[Req, Res any] struct {
	config BatcherConfig[Req, Res]

	mu      sync.Mutex
	queue   []batchEntry[Req, Res]
	trigger chan struct{}
	flushMu sync.Mutex
}

// NewBatcher creates a new Batcher with the given configuration.
// If BatchSize or FlushDelay are <= 0, defaults from client.DefaultBatchSize
// and client.DefaultBatchFlushDelay are used.
func NewBatcher[Req, Res any](config BatcherConfig[Req, Res]) *Batcher[Req, Res] {
	if config.BatchSize <= 0 {
		config.BatchSize = client.DefaultBatchSize
	}
	if config.FlushDelay <= 0 {
		config.FlushDelay = client.DefaultBatchFlushDelay
	}
	return &Batcher[Req, Res]{config: config}
}

func (b *Batcher[Req, Res]) Enqueue(ctx context.Context, req Req) (Res, error) {
	resCh := make(chan batchResult[Res], 1)

	b.mu.Lock()
	b.queue = append(b.queue, batchEntry[Req, Res]{ctx: ctx, req: req, result: resCh})

	if len(b.queue) >= b.config.BatchSize {
		if b.trigger != nil {
			select {
			case b.trigger <- struct{}{}:
			default:
			}
		} else {
			b.trigger = make(chan struct{}, 1)
			b.trigger <- struct{}{}
			go b.waitForBatch(b.trigger)
		}
	} else if b.trigger == nil {
		b.trigger = make(chan struct{}, 1)
		go b.waitForBatch(b.trigger)
	}
	b.mu.Unlock()

	select {
	case <-ctx.Done():
		var zero Res
		return zero, ctx.Err()
	case result := <-resCh:
		return result.val, result.err
	}
}

// waitForBatch waits for either a trigger signal or FlushDelay timeout, then flushes the batch.
// Runs in a separate goroutine per batch window.
func (b *Batcher[Req, Res]) waitForBatch(trigger <-chan struct{}) {
	select {
	case <-trigger:
	case <-time.After(b.config.FlushDelay):
	}
	b.flush()
}

func (b *Batcher[Req, Res]) flush() {
	b.flushMu.Lock()
	defer b.flushMu.Unlock()

	b.mu.Lock()
	n := len(b.queue)
	if n == 0 {
		b.trigger = nil
		b.mu.Unlock()
		return
	}
	if n > b.config.BatchSize {
		n = b.config.BatchSize
	}
	batch := make([]batchEntry[Req, Res], n)
	copy(batch, b.queue[:n])
	b.queue = b.queue[n:]

	if len(b.queue) > 0 {
		b.trigger = make(chan struct{}, 1)
		if len(b.queue) >= b.config.BatchSize {
			b.trigger <- struct{}{}
		}
		go b.waitForBatch(b.trigger)
	} else {
		b.trigger = nil
	}
	b.mu.Unlock()

	items := make([]Req, 0, len(batch))
	for _, entry := range batch {
		items = append(items, entry.req)
	}

	execCtx := batchExecutionContext(batch)
	if err := execCtx.Err(); err != nil {
		b.failBatch(batch, err)
		return
	}

	results, err := b.config.ExecuteBatch(execCtx, items)
	if err != nil {
		var indexedErr IndexedBatchError
		if errors.As(err, &indexedErr) {
			b.failBatchIndexed(batch, indexedErr)
			return
		}
		if b.config.ShouldFallback != nil && b.config.ShouldFallback(err) && b.config.ExecuteOne != nil {
			results, err = b.executeIndividually(execCtx, batch)
			if err != nil {
				b.failBatch(batch, err)
				return
			}
		} else {
			b.failBatch(batch, err)
			return
		}
	}

	if len(results) != len(batch) {
		b.failBatch(batch, fmt.Errorf("batch execute returned %d results for %d items", len(results), len(batch)))
		return
	}

	if err := b.config.Publish(execCtx); err != nil {
		if b.config.IsPublishOK != nil && b.config.IsPublishOK(err) {
			if b.config.WarnLog != nil {
				b.config.WarnLog(err.Error())
			}
		} else {
			b.failBatch(batch, fmt.Errorf("failed to publish batch: %w", err))
			return
		}
	}

	for i, entry := range batch {
		entry.result <- batchResult[Res]{val: results[i]}
	}
}

// batchExecutionContext returns a context for executing a batch.
// It uses the first non-nil entry context from the batch. If any entry's context
// is canceled, the batch execution context will also be canceled, causing the entire
// batch to fail (unless ShouldFallback triggers individual retry). This is by design:
// batch operations are atomic, and partial failures are surfaced via IndexedBatchError
// rather than per-entry context cancellation.
func batchExecutionContext[Req, Res any](batch []batchEntry[Req, Res]) context.Context {
	for _, entry := range batch {
		if entry.ctx != nil {
			return entry.ctx
		}
	}

	return context.Background()
}

func (b *Batcher[Req, Res]) failBatch(batch []batchEntry[Req, Res], err error) {
	for _, entry := range batch {
		entry.result <- batchResult[Res]{err: err}
	}
}

func (b *Batcher[Req, Res]) failBatchIndexed(batch []batchEntry[Req, Res], indexedErr IndexedBatchError) {
	for i, entry := range batch {
		entry.result <- batchResult[Res]{err: indexedErr.ErrorForIndex(i)}
	}
}

// executeIndividually runs each batch entry through ExecuteOne sequentially.
// Used as a fallback when ExecuteBatch fails and ShouldFallback returns true.
// Returns a positionally-corresponding slice of results or an error if any individual execution fails.
func (b *Batcher[Req, Res]) executeIndividually(ctx context.Context, batch []batchEntry[Req, Res]) ([]Res, error) {
	results := make([]Res, 0, len(batch))
	for _, entry := range batch {
		res, err := b.config.ExecuteOne(ctx, entry.req)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}
