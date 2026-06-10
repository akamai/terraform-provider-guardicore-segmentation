package common

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var ReadAfterCreateMaxAttempts = 5

var ReadAfterCreateInitialDelay = 200 * time.Millisecond

var ReadAfterCreateMaxDelay = 2 * time.Second

var readAfterCreateHookMu sync.RWMutex

var readAfterCreateVisibilityHook func(resourceName string, attempt int) bool

// waitForReadAfterCreate polls a read function until a resource becomes readable.
// It handles eventual consistency windows after create operations.
func WaitForReadAfterCreate[T any](ctx context.Context, resourceName string, read func(context.Context) (*T, error)) (*T, error) {
	delay := ReadAfterCreateInitialDelay

	for attempt := 1; attempt <= ReadAfterCreateMaxAttempts; attempt++ {
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

		if attempt == ReadAfterCreateMaxAttempts {
			break
		}

		if err := SleepWithContext(ctx, delay); err != nil {
			return nil, err
		}

		delay *= 2
		if delay > ReadAfterCreateMaxDelay {
			delay = ReadAfterCreateMaxDelay
		}
	}

	return nil, fmt.Errorf("%s created but not yet readable after %d attempts", resourceName, ReadAfterCreateMaxAttempts)
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

func SetReadAfterCreateVisibilityHookForTest(hook func(resourceName string, attempt int) bool) func() {
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

func SleepWithContext(ctx context.Context, delay time.Duration) error {
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
