package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

func TestAssetCreateBatcher_BulkCall(t *testing.T) {
	bulkCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4.0/assets/bulk":
			bulkCalls++
			writeBatcherJSON(t, w, map[string]any{
				"number_of_succeeded": 2,
				"number_of_failed":    0,
				"total_number":        2,
				"created_asset_ids": map[string]string{
					"orch-1": "asset-1",
					"orch-2": "asset-2",
				},
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	b := NewAssetCreateBatcher(apiClient, client.BatcherTuning{})
	var wg sync.WaitGroup
	for i := 1; i <= 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), &client.AssetCreate{Name: fmt.Sprintf("a-%d", i), OrchestrationObjID: fmt.Sprintf("orch-%d", i), Nics: []client.AssetNIC{{IPAddresses: []string{"10.0.0.1"}}}})
			if err != nil {
				t.Errorf("enqueue: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
}

func TestAssetUpdateBatcher_BulkCall(t *testing.T) {
	bulkCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/assets/bulk" && r.Method == http.MethodPut:
			bulkCalls++
			writeBatcherJSON(t, w, map[string]any{
				"number_of_succeeded": 2,
				"number_of_failed":    0,
				"total_number":        2,
				"errors":              []any{},
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	b := NewAssetUpdateBatcher(apiClient, client.BatcherTuning{})
	var wg sync.WaitGroup
	for i := 1; i <= 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), &client.AssetBulkUpdateItem{AssetID: fmt.Sprintf("a-%d", i), Name: fmt.Sprintf("name-%d", i)})
			if err != nil {
				t.Errorf("enqueue: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
}

func TestAssetDeleteBatcher_BulkCall(t *testing.T) {
	bulkCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/assets/bulk/deactivate" && r.Method == http.MethodPost:
			bulkCalls++
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	b := NewAssetDeleteBatcher(apiClient, client.BatcherTuning{})
	var wg sync.WaitGroup
	for i := 1; i <= 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), fmt.Sprintf("asset-%d", i))
			if err != nil {
				t.Errorf("enqueue: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
}
