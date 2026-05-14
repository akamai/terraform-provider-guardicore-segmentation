package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

func writeBatcherJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("failed to write JSON response: %v", err)
	}
}

func TestPolicyRuleCreateBatcher_CoalescesAndPublishesOnce(t *testing.T) {
	var mu sync.Mutex
	bulkCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/bulk":
			mu.Lock()
			bulkCalls++
			mu.Unlock()

			var specs []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&specs); err != nil {
				t.Fatalf("failed to decode bulk request: %v", err)
			}

			succeeded := make([]string, len(specs))
			for i := range specs {
				succeeded[i] = fmt.Sprintf("rule-%d", i+1)
			}

			writeBatcherJSON(t, w, client.PolicyRulesBulkCreateResponse{
				NumberOfFailed:    0,
				NumberOfSucceeded: len(specs),
				Result:            "success",
				Succeeded:         succeeded,
				TotalNumber:       len(specs),
			})
		case "/api/v4.0/visibility/policy/revisions":
			mu.Lock()
			publishCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewPolicyRuleCreateBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	ids := make([]string, n)
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := batcher.Enqueue(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}})
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
		t.Fatalf("unexpected enqueue error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
	if publishCalls != 1 {
		t.Fatalf("expected 1 publish call, got %d", publishCalls)
	}

	for i, id := range ids {
		if id == "" {
			t.Fatalf("expected non-empty id for index %d", i)
		}
	}
}

func TestPolicyRuleCreateBatcher_PartialFailureFailsAll(t *testing.T) {
	var mu sync.Mutex
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/bulk":
			writeBatcherJSON(t, w, client.PolicyRulesBulkCreateResponse{
				NumberOfFailed:    1,
				NumberOfSucceeded: 1,
				Result:            "partial failure",
				Succeeded:         []string{"rule-1"},
				TotalNumber:       2,
			})
		case "/api/v4.0/visibility/policy/revisions":
			mu.Lock()
			publishCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewPolicyRuleCreateBatcher(apiClient, client.BatcherTuning{})

	const n = 2
	ids := make([]string, n)
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := batcher.Enqueue(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}})
			ids[i] = id
			errCh <- err
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err == nil {
			t.Fatal("expected error for partial bulk failure, got nil")
		}
	}

	for i, id := range ids {
		if id != "" {
			t.Fatalf("expected empty ID for failed item %d, got %q", i, id)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if publishCalls != 0 {
		t.Fatalf("expected 0 publish calls on partial failure, got %d", publishCalls)
	}
}

func TestPolicyRuleCreateBatcher_PublishFailureFailsAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/bulk":
			writeBatcherJSON(t, w, client.PolicyRulesBulkCreateResponse{
				NumberOfFailed:    0,
				NumberOfSucceeded: 2,
				Result:            "success",
				Succeeded:         []string{"rule-1", "rule-2"},
				TotalNumber:       2,
			})
		case "/api/v4.0/visibility/policy/revisions":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"publish failed"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewPolicyRuleCreateBatcher(apiClient, client.BatcherTuning{})

	const n = 2
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}})
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

func TestPolicyRuleCreateBatcher_FallbackToSingleCreateOnUnknownField(t *testing.T) {
	var mu sync.Mutex
	bulkCalls := 0
	singleCreateCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/bulk":
			mu.Lock()
			bulkCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":{"json":{"0":{"priority":["Unknown field."]}}}}`))
		case "/api/v4.0/visibility/policy/rules":
			mu.Lock()
			singleCreateCalls++
			n := singleCreateCalls
			mu.Unlock()
			writeBatcherJSON(t, w, client.CreateResponse{ID: fmt.Sprintf("rule-%d", n)})
		case "/api/v4.0/visibility/policy/revisions":
			mu.Lock()
			publishCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewPolicyRuleCreateBatcher(apiClient, client.BatcherTuning{})

	const n = 2
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}, "priority": 10})
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("expected fallback success, got error: %v", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
	if singleCreateCalls != n {
		t.Fatalf("expected %d single create calls, got %d", n, singleCreateCalls)
	}
	if publishCalls != 1 {
		t.Fatalf("expected 1 publish call, got %d", publishCalls)
	}
}

func TestPolicyRuleCreateBatcher_RevisionUnchangedTreatedAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/bulk":
			var specs []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&specs); err != nil {
				t.Fatalf("failed to decode bulk request: %v", err)
			}
			succeeded := make([]string, len(specs))
			for i := range specs {
				succeeded[i] = fmt.Sprintf("rule-%d", i+1)
			}
			writeBatcherJSON(t, w, client.PolicyRulesBulkCreateResponse{
				NumberOfFailed:    0,
				NumberOfSucceeded: len(specs),
				Result:            "success",
				Succeeded:         succeeded,
				TotalNumber:       len(specs),
			})
		case "/api/v4.0/visibility/policy/revisions":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"description":"Revision hasn't been changed.","error_code":"BAD_REQUEST"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewPolicyRuleCreateBatcher(apiClient, client.BatcherTuning{})

	const n = 2
	ids := make([]string, n)
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := batcher.Enqueue(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}})
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
		t.Fatalf("expected no error when revision unchanged, got: %v", err)
	}

	for i, id := range ids {
		if id == "" {
			t.Fatalf("expected non-empty id for index %d", i)
		}
	}
}

func TestPolicyRuleCreateBatcher_FallbackRevisionUnchangedTreatedAsSuccess(t *testing.T) {
	var mu sync.Mutex
	singleCreateCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/bulk":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":{"json":{"0":{"priority":["Unknown field."]}}}}`))
		case "/api/v4.0/visibility/policy/rules":
			mu.Lock()
			singleCreateCalls++
			n := singleCreateCalls
			mu.Unlock()
			writeBatcherJSON(t, w, client.CreateResponse{ID: fmt.Sprintf("rule-%d", n)})
		case "/api/v4.0/visibility/policy/revisions":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"description":"Revision hasn't been changed.","error_code":"BAD_REQUEST"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewPolicyRuleCreateBatcher(apiClient, client.BatcherTuning{})

	const n = 2
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}, "priority": 10})
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("expected no error on fallback with revision unchanged, got: %v", err)
		}
	}
}

func TestPolicyRuleCreateBatcher_FlushesAtBatchSize(t *testing.T) {
	var mu sync.Mutex
	bulkCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/bulk":
			mu.Lock()
			bulkCalls++
			mu.Unlock()

			var specs []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&specs); err != nil {
				t.Fatalf("failed to decode bulk request: %v", err)
			}
			succeeded := make([]string, len(specs))
			for i := range specs {
				succeeded[i] = fmt.Sprintf("rule-%d", i+1)
			}
			writeBatcherJSON(t, w, client.PolicyRulesBulkCreateResponse{
				NumberOfFailed:    0,
				NumberOfSucceeded: len(specs),
				Result:            "success",
				Succeeded:         succeeded,
				TotalNumber:       len(specs),
			})
		case "/api/v4.0/visibility/policy/revisions":
			mu.Lock()
			publishCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewBatcher(BatcherConfig[map[string]any, string]{
		BatchSize: 5,
		ExecuteBatch: func(ctx context.Context, items []map[string]any) ([]string, error) {
			bulkResp, err := apiClient.BulkCreatePolicyRules(ctx, items)
			if err != nil {
				return nil, err
			}
			return bulkResp.Succeeded, nil
		},
		Publish:     policyRulePublish(apiClient),
		IsPublishOK: policyRuleIsPublishOK,
		WarnLog:     policyRuleWarnLog,
	})

	const n = 5
	ids := make([]string, n)
	errCh := make(chan error, n)
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := batcher.Enqueue(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}})
			if err != nil {
				errCh <- err
				return
			}
			ids[i] = id
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	close(errCh)
	for err := range errCh {
		t.Fatalf("unexpected error: %v", err)
	}

	if elapsed >= client.DefaultBatchFlushDelay {
		t.Fatalf("expected flush before timer (%v), but took %v", client.DefaultBatchFlushDelay, elapsed)
	}

	mu.Lock()
	defer mu.Unlock()
	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
	if publishCalls != 1 {
		t.Fatalf("expected 1 publish call, got %d", publishCalls)
	}
}

func TestPolicyRuleCreateBatcher_ConcurrentFlushesSerialize(t *testing.T) {
	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32
	var mu sync.Mutex
	bulkCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/bulk":
			mu.Lock()
			bulkCalls++
			mu.Unlock()

			var specs []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&specs); err != nil {
				t.Fatalf("failed to decode bulk request: %v", err)
			}
			succeeded := make([]string, len(specs))
			for i := range specs {
				succeeded[i] = fmt.Sprintf("rule-%d", i+1)
			}
			writeBatcherJSON(t, w, client.PolicyRulesBulkCreateResponse{
				NumberOfFailed:    0,
				NumberOfSucceeded: len(specs),
				Result:            "success",
				Succeeded:         succeeded,
				TotalNumber:       len(specs),
			})
		case "/api/v4.0/visibility/policy/revisions":
			cur := currentConcurrent.Add(1)
			for {
				prev := maxConcurrent.Load()
				if cur <= prev || maxConcurrent.CompareAndSwap(prev, cur) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			currentConcurrent.Add(-1)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewBatcher(BatcherConfig[map[string]any, string]{
		BatchSize: 3,
		ExecuteBatch: func(ctx context.Context, items []map[string]any) ([]string, error) {
			bulkResp, err := apiClient.BulkCreatePolicyRules(ctx, items)
			if err != nil {
				return nil, err
			}
			return bulkResp.Succeeded, nil
		},
		Publish:     policyRulePublish(apiClient),
		IsPublishOK: policyRuleIsPublishOK,
		WarnLog:     policyRuleWarnLog,
	})

	const n = 6
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}})
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

	mu.Lock()
	defer mu.Unlock()
	if bulkCalls < 2 {
		t.Fatalf("expected at least 2 bulk calls (to verify serialization), got %d", bulkCalls)
	}
}

func TestPolicyRuleCreateBatcher_LargeBatchChunked(t *testing.T) {
	var mu sync.Mutex
	bulkCalls := 0
	publishCalls := 0
	totalRulesCreated := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/bulk":
			mu.Lock()
			bulkCalls++
			mu.Unlock()

			var specs []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&specs); err != nil {
				t.Fatalf("failed to decode bulk request: %v", err)
			}

			mu.Lock()
			baseIdx := totalRulesCreated
			totalRulesCreated += len(specs)
			mu.Unlock()

			succeeded := make([]string, len(specs))
			for i := range specs {
				succeeded[i] = fmt.Sprintf("rule-%d", baseIdx+i+1)
			}
			writeBatcherJSON(t, w, client.PolicyRulesBulkCreateResponse{
				NumberOfFailed:    0,
				NumberOfSucceeded: len(specs),
				Result:            "success",
				Succeeded:         succeeded,
				TotalNumber:       len(specs),
			})
		case "/api/v4.0/visibility/policy/revisions":
			mu.Lock()
			publishCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewBatcher(BatcherConfig[map[string]any, string]{
		BatchSize: 5,
		ExecuteBatch: func(ctx context.Context, items []map[string]any) ([]string, error) {
			bulkResp, err := apiClient.BulkCreatePolicyRules(ctx, items)
			if err != nil {
				return nil, err
			}
			return bulkResp.Succeeded, nil
		},
		Publish:     policyRulePublish(apiClient),
		IsPublishOK: policyRuleIsPublishOK,
		WarnLog:     policyRuleWarnLog,
	})

	const n = 12
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}})
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
	if totalRulesCreated != n {
		t.Fatalf("expected %d total rules created, got %d", n, totalRulesCreated)
	}
	if bulkCalls < 2 {
		t.Fatalf("expected at least 2 bulk calls for %d items with batch size 5, got %d", n, bulkCalls)
	}
	if publishCalls != bulkCalls {
		t.Fatalf("expected publish calls (%d) to match bulk calls (%d)", publishCalls, bulkCalls)
	}
}

func TestPolicyRuleUpdateBatcher_CoalescesAndPublishesOnce(t *testing.T) {
	var mu sync.Mutex
	bulkCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/bulk":
			if r.Method != http.MethodPut {
				t.Fatalf("expected PUT, got %s", r.Method)
			}
			mu.Lock()
			bulkCalls++
			mu.Unlock()

			var items []client.PolicyRuleBulkUpdateItem
			if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
				t.Fatalf("failed to decode bulk request: %v", err)
			}

			succeeded := make([]string, len(items))
			for i, item := range items {
				succeeded[i] = item.ID
			}

			writeBatcherJSON(t, w, client.PolicyRulesBulkCreateResponse{
				NumberOfFailed:    0,
				NumberOfSucceeded: len(items),
				Result:            "success",
				Succeeded:         succeeded,
				TotalNumber:       len(items),
			})
		case "/api/v4.0/visibility/policy/revisions":
			mu.Lock()
			publishCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewPolicyRuleUpdateBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), policyRuleUpdateReq{
				id:   fmt.Sprintf("rule-%d", i),
				spec: map[string]any{"action": "BLOCK"},
			})
			errCh <- err
		}(i)
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
	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
	if publishCalls != 1 {
		t.Fatalf("expected 1 publish call, got %d", publishCalls)
	}
}

func TestPolicyRuleDeleteBatcher_CoalescesAndPublishesOnce(t *testing.T) {
	var mu sync.Mutex
	bulkCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/visibility/policy/rules/delete/bulk":
			mu.Lock()
			bulkCalls++
			mu.Unlock()

			var items []client.PolicyRuleBulkDeleteItem
			if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
				t.Fatalf("failed to decode bulk request: %v", err)
			}

			succeeded := make([]string, len(items))
			for i, item := range items {
				succeeded[i] = item.ID
			}

			writeBatcherJSON(t, w, client.PolicyRulesBulkCreateResponse{
				NumberOfFailed:    0,
				NumberOfSucceeded: len(items),
				Result:            "success",
				Succeeded:         succeeded,
				TotalNumber:       len(items),
			})
		case "/api/v4.0/visibility/policy/revisions":
			mu.Lock()
			publishCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewPolicyRuleDeleteBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), fmt.Sprintf("rule-%d", i))
			errCh <- err
		}(i)
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
	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
	if publishCalls != 1 {
		t.Fatalf("expected 1 publish call, got %d", publishCalls)
	}
}
