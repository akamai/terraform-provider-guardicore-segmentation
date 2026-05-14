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

func TestLabelGroupCreateBatcher_CoalescesAndPublishesOnce(t *testing.T) {
	var mu sync.Mutex
	createCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/label-groups" && r.Method == http.MethodPost:
			mu.Lock()
			createCalls++
			n := createCalls
			mu.Unlock()

			var lg client.LabelGroupCreate
			if err := json.NewDecoder(r.Body).Decode(&lg); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			lg.ID = fmt.Sprintf("lg-%d", n)
			writeBatcherJSON(t, w, lg)
		case r.URL.Path == "/api/v4.0/label-groups/publish":
			mu.Lock()
			publishCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewLabelGroupCreateBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	results := make([]*client.LabelGroupCreate, n)
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := batcher.Enqueue(context.Background(), &client.LabelGroupCreate{
				Key:   "env",
				Value: fmt.Sprintf("val-%d", i),
			})
			if err != nil {
				errCh <- err
				return
			}
			results[i] = res
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if createCalls != n {
		t.Fatalf("expected %d individual create calls, got %d", n, createCalls)
	}
	if publishCalls != 1 {
		t.Fatalf("expected 1 publish call, got %d", publishCalls)
	}
	for i, res := range results {
		if res == nil || res.ID == "" {
			t.Fatalf("expected non-nil result with ID for index %d", i)
		}
	}
}

func TestLabelGroupUpdateBatcher_CoalescesAndPublishesOnce(t *testing.T) {
	var mu sync.Mutex
	updateCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && len(r.URL.Path) > len("/api/v4.0/label-groups/"):
			mu.Lock()
			updateCalls++
			mu.Unlock()

			var lg client.LabelGroupCreate
			if err := json.NewDecoder(r.Body).Decode(&lg); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			writeBatcherJSON(t, w, lg)
		case r.URL.Path == "/api/v4.0/label-groups/publish":
			mu.Lock()
			publishCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewLabelGroupUpdateBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), labelGroupUpdateReq{
				id: fmt.Sprintf("lg-%d", i),
				labelGroup: &client.LabelGroupCreate{
					Key:   "env",
					Value: fmt.Sprintf("updated-%d", i),
				},
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
	if updateCalls != n {
		t.Fatalf("expected %d update calls, got %d", n, updateCalls)
	}
	if publishCalls != 1 {
		t.Fatalf("expected 1 publish call, got %d", publishCalls)
	}
}

func TestLabelGroupDeleteBatcher_CoalescesAndPublishesOnce(t *testing.T) {
	var mu sync.Mutex
	deleteCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && len(r.URL.Path) > len("/api/v4.0/label-groups/"):
			mu.Lock()
			deleteCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/v4.0/label-groups/publish":
			mu.Lock()
			publishCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewLabelGroupDeleteBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), fmt.Sprintf("lg-%d", i))
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
	if deleteCalls != n {
		t.Fatalf("expected %d delete calls, got %d", n, deleteCalls)
	}
	if publishCalls != 1 {
		t.Fatalf("expected 1 publish call, got %d", publishCalls)
	}
}

func TestLabelGroupCreateBatcher_PublishFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/label-groups" && r.Method == http.MethodPost:
			var lg client.LabelGroupCreate
			if err := json.NewDecoder(r.Body).Decode(&lg); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			lg.ID = "lg-1"
			writeBatcherJSON(t, w, lg)
		case r.URL.Path == "/api/v4.0/label-groups/publish":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"publish failed"}`))
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewLabelGroupCreateBatcher(apiClient, client.BatcherTuning{})

	_, err = batcher.Enqueue(context.Background(), &client.LabelGroupCreate{
		Key:   "env",
		Value: "test",
	})
	if err == nil {
		t.Fatal("expected error when publish fails, got nil")
	}
}

func TestLabelGroupDeleteBatcher_IndividualFailure(t *testing.T) {
	var mu sync.Mutex
	deleteCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && len(r.URL.Path) > len("/api/v4.0/label-groups/"):
			mu.Lock()
			deleteCalls++
			n := deleteCalls
			mu.Unlock()
			if n == 2 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"delete failed"}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/v4.0/label-groups/publish":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewLabelGroupDeleteBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), fmt.Sprintf("lg-%d", i))
			errCh <- err
		}(i)
	}

	wg.Wait()
	close(errCh)

	hasError := false
	for err := range errCh {
		if err != nil {
			hasError = true
		}
	}
	if !hasError {
		t.Fatal("expected at least one error when individual delete fails")
	}
}
