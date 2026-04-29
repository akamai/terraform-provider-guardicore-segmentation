package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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

	batcher := NewPolicyRuleCreateBatcher(apiClient)

	const n = 3
	ids := make([]string, n)
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := batcher.EnqueueCreate(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}})
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

	batcher := NewPolicyRuleCreateBatcher(apiClient)

	const n = 2
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.EnqueueCreate(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}})
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err == nil {
			t.Fatal("expected error for partial bulk failure, got nil")
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

	batcher := NewPolicyRuleCreateBatcher(apiClient)

	const n = 2
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.EnqueueCreate(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}})
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

	batcher := NewPolicyRuleCreateBatcher(apiClient)

	const n = 2
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.EnqueueCreate(context.Background(), map[string]any{"action": "ALLOW", "section_position": "ALLOW", "source": map[string]any{}, "destination": map[string]any{}, "priority": 10})
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
