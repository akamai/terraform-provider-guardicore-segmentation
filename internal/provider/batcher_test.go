package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

func TestBatcher_CoalescesAndPublishesOnce(t *testing.T) {
	var executeCalls atomic.Int32
	var publishCalls atomic.Int32

	batcher := NewBatcher(BatcherConfig[string, string]{
		ExecuteBatch: func(_ context.Context, items []string) ([]string, error) {
			executeCalls.Add(1)
			results := make([]string, len(items))
			for i, item := range items {
				results[i] = "id-" + item
			}
			return results, nil
		},
		Publish: func(_ context.Context) error {
			publishCalls.Add(1)
			return nil
		},
	})

	const n = 3
	ids := make([]string, n)
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := batcher.Enqueue(context.Background(), fmt.Sprintf("item-%d", i))
			if err != nil {
				errCh <- err
				return
			}
			ids[i] = id
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("unexpected error: %v", err)
	}

	if c := executeCalls.Load(); c != 1 {
		t.Fatalf("expected 1 execute call, got %d", c)
	}
	if c := publishCalls.Load(); c != 1 {
		t.Fatalf("expected 1 publish call, got %d", c)
	}
	for i, id := range ids {
		if id == "" {
			t.Fatalf("expected non-empty id for index %d", i)
		}
	}
}

func TestBatcher_BatchExecuteFailure(t *testing.T) {
	var publishCalls atomic.Int32
	batchErr := errors.New("batch execution failed")

	batcher := NewBatcher(BatcherConfig[string, string]{
		ExecuteBatch: func(_ context.Context, _ []string) ([]string, error) {
			return nil, batchErr
		},
		Publish: func(_ context.Context) error {
			publishCalls.Add(1)
			return nil
		},
	})

	const n = 2
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), "item")
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	}
	if c := publishCalls.Load(); c != 0 {
		t.Fatalf("expected 0 publish calls on batch failure, got %d", c)
	}
}

func TestBatcher_PublishFailure(t *testing.T) {
	batcher := NewBatcher(BatcherConfig[string, string]{
		ExecuteBatch: func(_ context.Context, items []string) ([]string, error) {
			results := make([]string, len(items))
			for i := range items {
				results[i] = fmt.Sprintf("id-%d", i)
			}
			return results, nil
		},
		Publish: func(_ context.Context) error {
			return errors.New("publish failed")
		},
	})

	const n = 2
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), "item")
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err == nil {
			t.Fatal("expected error when publish fails, got nil")
		}
	}
}

func TestBatcher_FallbackToSingleExecution(t *testing.T) {
	var executeBatchCalls atomic.Int32
	var executeOneCalls atomic.Int32

	batcher := NewBatcher(BatcherConfig[string, string]{
		ExecuteBatch: func(_ context.Context, _ []string) ([]string, error) {
			executeBatchCalls.Add(1)
			return nil, errors.New("unknown field in batch")
		},
		Publish: func(_ context.Context) error { return nil },
		ShouldFallback: func(err error) bool {
			return err.Error() == "unknown field in batch"
		},
		ExecuteOne: func(_ context.Context, item string) (string, error) {
			executeOneCalls.Add(1)
			return "id-" + item, nil
		},
	})

	const n = 2
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), "item")
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("expected fallback success, got: %v", err)
		}
	}
	if c := executeBatchCalls.Load(); c != 1 {
		t.Fatalf("expected 1 batch call, got %d", c)
	}
	if c := executeOneCalls.Load(); c != int32(n) {
		t.Fatalf("expected %d single calls, got %d", n, c)
	}
}

func TestBatcher_FallbackExecuteOneFailure(t *testing.T) {
	batcher := NewBatcher(BatcherConfig[string, string]{
		ExecuteBatch: func(_ context.Context, _ []string) ([]string, error) {
			return nil, errors.New("batch error")
		},
		Publish: func(_ context.Context) error { return nil },
		ShouldFallback: func(_ error) bool {
			return true
		},
		ExecuteOne: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("single error")
		},
	})

	_, err := batcher.Enqueue(context.Background(), "item")
	if err == nil {
		t.Fatal("expected error from fallback failure, got nil")
	}
}

func TestBatcher_IsPublishOKWithWarning(t *testing.T) {
	var warnMsg string
	var mu sync.Mutex

	batcher := NewBatcher(BatcherConfig[string, string]{
		ExecuteBatch: func(_ context.Context, items []string) ([]string, error) {
			results := make([]string, len(items))
			for i := range items {
				results[i] = fmt.Sprintf("id-%d", i)
			}
			return results, nil
		},
		Publish: func(_ context.Context) error {
			return errors.New("no changes to publish")
		},
		IsPublishOK: func(err error) bool {
			return err.Error() == "no changes to publish"
		},
		WarnLog: func(msg string) {
			mu.Lock()
			warnMsg = msg
			mu.Unlock()
		},
	})

	id, err := batcher.Enqueue(context.Background(), "item")
	if err != nil {
		t.Fatalf("expected success when IsPublishOK returns true, got: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	mu.Lock()
	defer mu.Unlock()
	if warnMsg != "no changes to publish" {
		t.Fatalf("expected warning message 'no changes to publish', got %q", warnMsg)
	}
}

func TestBatcher_FlushesAtBatchSize(t *testing.T) {
	var executeCalls atomic.Int32
	var publishCalls atomic.Int32

	batcher := NewBatcher(BatcherConfig[string, string]{
		BatchSize: 5,
		ExecuteBatch: func(_ context.Context, items []string) ([]string, error) {
			executeCalls.Add(1)
			results := make([]string, len(items))
			for i := range items {
				results[i] = fmt.Sprintf("id-%d", i)
			}
			return results, nil
		},
		Publish: func(_ context.Context) error {
			publishCalls.Add(1)
			return nil
		},
	})

	const n = 5
	errCh := make(chan error, n)
	var wg sync.WaitGroup

	start := time.Now()
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), "item")
			errCh <- err
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if elapsed >= client.DefaultBatchFlushDelay {
		t.Fatalf("expected flush before timer (%v), but took %v", client.DefaultBatchFlushDelay, elapsed)
	}
	if c := executeCalls.Load(); c != 1 {
		t.Fatalf("expected 1 execute call, got %d", c)
	}
	if c := publishCalls.Load(); c != 1 {
		t.Fatalf("expected 1 publish call, got %d", c)
	}
}

func TestBatcher_ConcurrentFlushesSerialize(t *testing.T) {
	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32
	var executeCalls atomic.Int32

	batcher := NewBatcher(BatcherConfig[string, string]{
		BatchSize: 3,
		ExecuteBatch: func(_ context.Context, items []string) ([]string, error) {
			executeCalls.Add(1)
			results := make([]string, len(items))
			for i := range items {
				results[i] = fmt.Sprintf("id-%d", i)
			}
			return results, nil
		},
		Publish: func(_ context.Context) error {
			cur := currentConcurrent.Add(1)
			for {
				prev := maxConcurrent.Load()
				if cur <= prev || maxConcurrent.CompareAndSwap(prev, cur) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			currentConcurrent.Add(-1)
			return nil
		},
	})

	const n = 6
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), "item")
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if mc := maxConcurrent.Load(); mc > 1 {
		t.Fatalf("expected max 1 concurrent publish, got %d", mc)
	}
	if c := executeCalls.Load(); c < 2 {
		t.Fatalf("expected at least 2 execute calls, got %d", c)
	}
}

func TestBatcher_LargeBatchChunked(t *testing.T) {
	var mu sync.Mutex
	totalProcessed := 0
	executeCalls := 0
	publishCalls := 0

	batcher := NewBatcher(BatcherConfig[string, string]{
		BatchSize: 5,
		ExecuteBatch: func(_ context.Context, items []string) ([]string, error) {
			mu.Lock()
			executeCalls++
			totalProcessed += len(items)
			mu.Unlock()
			results := make([]string, len(items))
			for i := range items {
				results[i] = fmt.Sprintf("id-%d", i)
			}
			return results, nil
		},
		Publish: func(_ context.Context) error {
			mu.Lock()
			publishCalls++
			mu.Unlock()
			return nil
		},
	})

	const n = 12
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), "item")
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if totalProcessed != n {
		t.Fatalf("expected %d total processed, got %d", n, totalProcessed)
	}
	if executeCalls < 2 {
		t.Fatalf("expected at least 2 execute calls for %d items with batch size 5, got %d", n, executeCalls)
	}
	if publishCalls != executeCalls {
		t.Fatalf("expected publish calls (%d) to match execute calls (%d)", publishCalls, executeCalls)
	}
}

func TestBatcher_VoidResultType(t *testing.T) {
	var executeCalls atomic.Int32

	batcher := NewBatcher(BatcherConfig[string, struct{}]{
		ExecuteBatch: func(_ context.Context, items []string) ([]struct{}, error) {
			executeCalls.Add(1)
			return make([]struct{}, len(items)), nil
		},
		Publish: func(_ context.Context) error { return nil },
	})

	const n = 3
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), "delete-id")
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if c := executeCalls.Load(); c != 1 {
		t.Fatalf("expected 1 execute call, got %d", c)
	}
}

func TestBatcher_ResultLengthMismatch(t *testing.T) {
	batcher := NewBatcher(BatcherConfig[string, string]{
		ExecuteBatch: func(_ context.Context, _ []string) ([]string, error) {
			return []string{"only-one"}, nil
		},
		Publish: func(_ context.Context) error { return nil },
	})

	const n = 2
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), "item")
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err == nil {
			t.Fatal("expected error for result length mismatch, got nil")
		}
	}
}

func TestBatcher_NoFallbackWhenNil(t *testing.T) {
	batcher := NewBatcher(BatcherConfig[string, string]{
		ExecuteBatch: func(_ context.Context, _ []string) ([]string, error) {
			return nil, errors.New("batch error")
		},
		Publish: func(_ context.Context) error { return nil },
	})

	_, err := batcher.Enqueue(context.Background(), "item")
	if err == nil {
		t.Fatal("expected error when ShouldFallback is nil, got nil")
	}
}

type testIndexedError struct {
	perIndex map[int]error
}

func (e *testIndexedError) Error() string { return "indexed batch error" }

func (e *testIndexedError) ErrorForIndex(i int) error {
	if err, ok := e.perIndex[i]; ok {
		return err
	}
	return fmt.Errorf("batch failed due to other items")
}

func TestBatcher_IndexedBatchErrorRouting(t *testing.T) {
	specificErr := errors.New("validation failed for this item")

	batcher := NewBatcher(BatcherConfig[string, string]{
		BatchSize: 3,
		ExecuteBatch: func(_ context.Context, items []string) ([]string, error) {
			errs := make(map[int]error)
			for i, item := range items {
				if item == "bad" {
					errs[i] = specificErr
				}
			}
			return nil, &testIndexedError{perIndex: errs}
		},
		Publish: func(_ context.Context) error { return nil },
	})

	const n = 3
	type result struct {
		idx int
		err error
	}
	results := make(chan result, n)
	var wg sync.WaitGroup
	items := []string{"good-0", "bad", "good-2"}
	for i, item := range items {
		wg.Add(1)
		go func(i int, item string) {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), item)
			results <- result{idx: i, err: err}
		}(i, item)
	}
	wg.Wait()
	close(results)

	errs := make(map[int]error, n)
	for r := range results {
		errs[r.idx] = r.err
	}

	for i := range n {
		if errs[i] == nil {
			t.Fatalf("item %d: expected error, got nil", i)
		}
	}

	hasSpecific := 0
	hasGeneric := 0
	for _, err := range errs {
		if err.Error() == specificErr.Error() {
			hasSpecific++
		} else if strings.Contains(err.Error(), "batch failed due to other items") {
			hasGeneric++
		}
	}
	if hasSpecific != 1 {
		t.Fatalf("expected 1 specific error, got %d", hasSpecific)
	}
	if hasGeneric != 2 {
		t.Fatalf("expected 2 generic errors, got %d", hasGeneric)
	}
}

func TestBatcher_IndexedBatchErrorSkipsFallback(t *testing.T) {
	var executeOneCalls atomic.Int32

	batcher := NewBatcher(BatcherConfig[string, string]{
		ExecuteBatch: func(_ context.Context, _ []string) ([]string, error) {
			return nil, &testIndexedError{perIndex: map[int]error{0: errors.New("fail")}}
		},
		Publish:        func(_ context.Context) error { return nil },
		ShouldFallback: func(_ error) bool { return true },
		ExecuteOne: func(_ context.Context, _ string) (string, error) {
			executeOneCalls.Add(1)
			return "id", nil
		},
	})

	_, err := batcher.Enqueue(context.Background(), "item")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if c := executeOneCalls.Load(); c != 0 {
		t.Fatalf("expected 0 fallback calls (indexed errors skip fallback), got %d", c)
	}
}
