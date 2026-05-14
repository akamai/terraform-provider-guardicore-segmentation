package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var readAfterCreateMaxAttempts = 5

var readAfterCreateInitialDelay = 200 * time.Millisecond

var readAfterCreateMaxDelay = 2 * time.Second

var readAfterCreateHookMu sync.RWMutex

var readAfterCreateVisibilityHook func(resourceName string, attempt int) bool

// waitForReadAfterCreate polls a read function until a resource becomes readable.
// It handles eventual consistency windows after create operations.
func waitForReadAfterCreate[T any](ctx context.Context, resourceName string, read func(context.Context) (*T, error)) (*T, error) {
	delay := readAfterCreateInitialDelay

	for attempt := 1; attempt <= readAfterCreateMaxAttempts; attempt++ {
		obj, err := read(ctx)
		if err != nil {
			return nil, err
		}

		if obj != nil {
			if shouldForceReadAfterCreateNotVisible(resourceName, attempt) {
				obj = nil
			}
		}

		if obj != nil {
			return obj, nil
		}

		if attempt == readAfterCreateMaxAttempts {
			break
		}

		if err := sleepWithContext(ctx, delay); err != nil {
			return nil, err
		}

		delay *= 2
		if delay > readAfterCreateMaxDelay {
			delay = readAfterCreateMaxDelay
		}
	}

	return nil, fmt.Errorf("%s created but not yet readable after %d attempts", resourceName, readAfterCreateMaxAttempts)
}

func shouldForceReadAfterCreateNotVisible(resourceName string, attempt int) bool {
	readAfterCreateHookMu.RLock()
	hook := readAfterCreateVisibilityHook
	readAfterCreateHookMu.RUnlock()

	if hook == nil {
		return false
	}

	return hook(resourceName, attempt)
}

func setReadAfterCreateVisibilityHookForTest(hook func(resourceName string, attempt int) bool) func() {
	readAfterCreateHookMu.Lock()
	previous := readAfterCreateVisibilityHook
	readAfterCreateVisibilityHook = hook
	readAfterCreateHookMu.Unlock()

	return func() {
		readAfterCreateHookMu.Lock()
		readAfterCreateVisibilityHook = previous
		readAfterCreateHookMu.Unlock()
	}
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
