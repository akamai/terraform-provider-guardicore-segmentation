package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

func TestLabelCreateBatcher_BulkCall(t *testing.T) {
	bulkCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4.0/labels/bulk" && r.Method == http.MethodPost {
			bulkCalls++
			writeBatcherJSON(t, w, map[string]any{"succeeded": []string{"l1", "l2"}, "failed": []string{}, "missing": []string{}})
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewLabelCreateBatcher(apiClient, client.BatcherTuning{})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), &client.LabelCreate{Key: "k", Value: "v"})
			if err != nil {
				t.Errorf("enqueue: %v", err)
			}
		}()
	}
	wg.Wait()

	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
}

func TestLabelUpdateBatcher_BulkCall(t *testing.T) {
	updateCalls := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v4.0/labels/") && r.Method == http.MethodPut {
			mu.Lock()
			updateCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewLabelUpdateBatcher(apiClient, client.BatcherTuning{})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), labelUpdateReq{
				id:    fmt.Sprintf("l%d", i+1),
				label: &client.LabelUpdate{Key: "k", Value: "v"},
			})
			if err != nil {
				t.Errorf("enqueue: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if updateCalls != 2 {
		t.Fatalf("expected 2 update calls, got %d", updateCalls)
	}
}

func TestLabelDeleteBatcher_BulkCall(t *testing.T) {
	bulkDeleteCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/labels/bulk_delete" && r.Method == http.MethodDelete:
			bulkDeleteCalls++
			writeBatcherJSON(t, w, map[string]any{"succeeded": []string{"l1", "l2"}, "failed": []string{}, "missing": []string{}})
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/labels/") && r.Method == http.MethodGet:
			// Verification: label no longer exists
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewLabelDeleteBatcher(apiClient, client.BatcherTuning{})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), fmt.Sprintf("l%d", i+1))
			if err != nil {
				t.Errorf("enqueue: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if bulkDeleteCalls != 1 {
		t.Fatalf("expected 1 bulk delete call, got %d", bulkDeleteCalls)
	}
}

func TestLabelCreateBatcher_ValidationErrorRouting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4.0/labels/bulk" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"message":{"json":{"1":{"criteria":{"0":{"op":["Must be one of EQUALS, CONTAINS"]}}}}}}`)
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewLabelCreateBatcher(apiClient, client.BatcherTuning{})

	type result struct {
		idx int
		err error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), &client.LabelCreate{Key: "k", Value: fmt.Sprintf("v%d", i)})
			results <- result{idx: i, err: err}
		}(i)
	}
	wg.Wait()
	close(results)

	errs := make(map[int]error, 2)
	for r := range results {
		errs[r.idx] = r.err
	}

	for i := 0; i < 2; i++ {
		if errs[i] == nil {
			t.Fatalf("item %d: expected error, got nil", i)
		}
	}

	hasSpecific := 0
	hasGeneric := 0
	for _, err := range errs {
		errStr := err.Error()
		if strings.Contains(errStr, "criteria") && strings.Contains(errStr, "op") {
			hasSpecific++
		} else if strings.Contains(errStr, "batch failed") {
			hasGeneric++
		}
	}
	if hasSpecific != 1 {
		t.Fatalf("expected 1 specific validation error, got %d", hasSpecific)
	}
	if hasGeneric != 1 {
		t.Fatalf("expected 1 generic batch error, got %d", hasGeneric)
	}
}

func TestLabelCreateBatcher_NonValidationErrorUnchanged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4.0/labels/bulk" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"description":"internal server error","error_code":"ServerError"}`)
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewLabelCreateBatcher(apiClient, client.BatcherTuning{})

	_, err = b.Enqueue(context.Background(), &client.LabelCreate{Key: "k", Value: "v"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "batch failed due to validation errors") {
		t.Fatalf("non-validation errors should not use indexed routing, got: %s", err)
	}
}
