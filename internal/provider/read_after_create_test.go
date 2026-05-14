package provider

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func withReadAfterCreateTiming(maxAttempts int, initialDelay, maxDelay time.Duration, fn func()) {
	oldAttempts := readAfterCreateMaxAttempts
	oldInitial := readAfterCreateInitialDelay
	oldMax := readAfterCreateMaxDelay

	readAfterCreateMaxAttempts = maxAttempts
	readAfterCreateInitialDelay = initialDelay
	readAfterCreateMaxDelay = maxDelay

	defer func() {
		readAfterCreateMaxAttempts = oldAttempts
		readAfterCreateInitialDelay = oldInitial
		readAfterCreateMaxDelay = oldMax
	}()

	fn()
}

func TestWaitForReadAfterCreate_SucceedsAfterTransientNotFound(t *testing.T) {
	withReadAfterCreateTiming(5, time.Millisecond, 2*time.Millisecond, func() {
		var calls int32
		type sample struct{ ID string }

		obj, err := waitForReadAfterCreate(context.Background(), "asset", func(context.Context) (*sample, error) {
			if atomic.AddInt32(&calls, 1) < 3 {
				return nil, nil
			}
			return &sample{ID: "asset-1"}, nil
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if obj == nil || obj.ID != "asset-1" {
			t.Fatalf("expected asset-1, got %#v", obj)
		}
		if got := atomic.LoadInt32(&calls); got != 3 {
			t.Fatalf("expected 3 calls, got %d", got)
		}
	})
}

func TestWaitForReadAfterCreate_TimesOut(t *testing.T) {
	withReadAfterCreateTiming(3, time.Millisecond, 2*time.Millisecond, func() {
		type sample struct{ ID string }

		_, err := waitForReadAfterCreate(context.Background(), "dns blocklist", func(context.Context) (*sample, error) {
			return nil, nil
		})
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "dns blocklist created but not yet readable") {
			t.Fatalf("expected eventual consistency error, got %v", err)
		}
	})
}

func TestWaitForReadAfterCreate_StopsOnHardError(t *testing.T) {
	withReadAfterCreateTiming(5, time.Millisecond, 2*time.Millisecond, func() {
		expected := errors.New("boom")
		var calls int32
		type sample struct{ ID string }

		_, err := waitForReadAfterCreate(context.Background(), "asset", func(context.Context) (*sample, error) {
			if atomic.AddInt32(&calls, 1) == 2 {
				return nil, expected
			}
			return nil, nil
		})
		if !errors.Is(err, expected) {
			t.Fatalf("expected hard error to be returned, got %v", err)
		}
		if got := atomic.LoadInt32(&calls); got != 2 {
			t.Fatalf("expected 2 calls before hard stop, got %d", got)
		}
	})
}

func TestWaitForReadAfterCreate_RespectsContextCancel(t *testing.T) {
	withReadAfterCreateTiming(5, 50*time.Millisecond, 100*time.Millisecond, func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		type sample struct{ ID string }
		_, err := waitForReadAfterCreate(ctx, "asset", func(context.Context) (*sample, error) {
			return nil, nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled error, got %v", err)
		}
	})
}

func TestAssetResource_Create_EventualConsistencyRead(t *testing.T) {
	withReadAfterCreateTiming(5, time.Millisecond, 2*time.Millisecond, func() {
		type asset struct{ ID string }
		var calls int32

		obj, err := waitForReadAfterCreate(context.Background(), "asset", func(context.Context) (*asset, error) {
			if atomic.AddInt32(&calls, 1) < 4 {
				return nil, nil
			}
			return &asset{ID: "asset-created"}, nil
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if obj == nil || obj.ID != "asset-created" {
			t.Fatalf("expected asset-created, got %#v", obj)
		}
	})
}

func TestDnsSecurityResource_Create_EventualConsistencyRead(t *testing.T) {
	withReadAfterCreateTiming(5, time.Millisecond, 2*time.Millisecond, func() {
		type dns struct{ ID string }
		var calls int32

		obj, err := waitForReadAfterCreate(context.Background(), "dns blocklist", func(context.Context) (*dns, error) {
			if atomic.AddInt32(&calls, 1) < 3 {
				return nil, nil
			}
			return &dns{ID: "dns-created"}, nil
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if obj == nil || obj.ID != "dns-created" {
			t.Fatalf("expected dns-created, got %#v", obj)
		}
	})
}

func TestSetReadAfterCreateVisibilityHookForTest(t *testing.T) {
	restore := setReadAfterCreateVisibilityHookForTest(func(resourceName string, attempt int) bool {
		return resourceName == "asset" && attempt == 1
	})

	if !shouldForceReadAfterCreateNotVisible("asset", 1) {
		t.Fatal("expected hook to force not visible")
	}
	if shouldForceReadAfterCreateNotVisible("asset", 2) {
		t.Fatal("expected hook to not force for attempt 2")
	}

	restore()

	if shouldForceReadAfterCreateNotVisible("asset", 1) {
		t.Fatal("expected hook to be restored")
	}
}
