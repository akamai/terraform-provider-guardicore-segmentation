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

func TestWorksiteDeleteBatcher_BulkCall(t *testing.T) {
	bulkCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4.0/worksites/delete_worksites" && r.Method == http.MethodPost {
			bulkCalls++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewWorksiteDeleteBatcher(apiClient, client.BatcherTuning{})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), "w")
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

func TestWorksiteDeleteBatcher_PropagatesSkipAsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4.0/worksites/delete_worksites" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"details":"Worksites: w-1 not deleted, these worksites are assigned","failures":0,"skips":1,"successes":0,"assigned_worksites":1,"assigned_details":"Reassign dependencies and retry."}`)
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewWorksiteDeleteBatcher(apiClient, client.BatcherTuning{})

	_, err = b.Enqueue(context.Background(), "w-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete worksite") {
		t.Fatalf("expected delete error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "skips=1") {
		t.Fatalf("expected skips in error, got: %v", err)
	}
}
