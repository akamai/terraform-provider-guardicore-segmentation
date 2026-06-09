package dnssecurity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
)

func writeBatcherJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("failed to write JSON response: %v", err)
	}
}

func TestDnsSecurityCreateBatcher_BulkCall(t *testing.T) {
	bulkCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4.0/dns_security/bulk" && r.Method == http.MethodPost {
			bulkCalls++
			writeBatcherJSON(t, w, map[string]any{"ids": []string{"d1", "d2"}})
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewDnsSecurityCreateBatcher(apiClient, client.BatcherTuning{})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), &client.DnsBlocklistCreate{Name: "n", Type: "CUSTOM_BLOCK", Domains: []string{"a.com"}})
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

func TestDnsSecurityUpdateBatcher_BulkCall(t *testing.T) {
	bulkCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4.0/dns_security/bulk" && r.Method == http.MethodPatch {
			bulkCalls++
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
	b := NewDnsSecurityUpdateBatcher(apiClient, client.BatcherTuning{})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "updated"
			_, err := b.Enqueue(context.Background(), common.DnsSecurityUpdateReq{
				ID:   fmt.Sprintf("d-%d", i),
				Edit: &client.DnsBlocklistEdit{Name: &name},
			})
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

func TestDnsSecurityDeleteBatcher_BulkCall(t *testing.T) {
	bulkCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4.0/dns_security/bulk" && r.Method == http.MethodDelete {
			bulkCalls++
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
	b := NewDnsSecurityDeleteBatcher(apiClient, client.BatcherTuning{})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), fmt.Sprintf("d-%d", i))
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

func TestDnsSecurityCreateBatcher_BulkErrorFallsBack(t *testing.T) {
	bulkCalls := 0
	createCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/dns_security/bulk" && r.Method == http.MethodPost:
			bulkCalls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"description":"bulk failed","error_code":"ServerError"}`))
		case r.URL.Path == "/api/v4.0/dns_security" && r.Method == http.MethodPost:
			createCalls++
			writeBatcherJSON(t, w, map[string]any{"id": "fallback-id"})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewDnsSecurityCreateBatcher(apiClient, client.BatcherTuning{})

	id, err := b.Enqueue(context.Background(), &client.DnsBlocklistCreate{Name: "n", Type: "CUSTOM_BLOCK", Domains: []string{"a.com"}})
	if err != nil {
		t.Fatalf("expected fallback create success, got %v", err)
	}
	if id != "fallback-id" {
		t.Fatalf("expected fallback id, got %q", id)
	}
	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
	if createCalls != 1 {
		t.Fatalf("expected 1 fallback create call, got %d", createCalls)
	}
}

func TestDnsSecurityUpdateBatcher_BulkErrorFallsBack(t *testing.T) {
	bulkCalls := 0
	patchCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/dns_security/bulk" && r.Method == http.MethodPatch:
			bulkCalls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"description":"bulk failed","error_code":"ServerError"}`))
		case r.URL.Path == "/api/v4.0/dns_security/d-1" && r.Method == http.MethodPatch:
			patchCalls++
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
	b := NewDnsSecurityUpdateBatcher(apiClient, client.BatcherTuning{})

	name := "updated"
	_, err = b.Enqueue(context.Background(), common.DnsSecurityUpdateReq{
		ID:   "d-1",
		Edit: &client.DnsBlocklistEdit{Name: &name},
	})
	if err != nil {
		t.Fatalf("expected fallback update success, got %v", err)
	}
	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
	if patchCalls != 1 {
		t.Fatalf("expected 1 fallback patch call, got %d", patchCalls)
	}
}

func TestDnsSecurityDeleteBatcher_BulkErrorFallsBack(t *testing.T) {
	bulkCalls := 0
	deleteCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/dns_security/bulk" && r.Method == http.MethodDelete:
			bulkCalls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"description":"bulk failed","error_code":"ServerError"}`))
		case r.URL.Path == "/api/v4.0/dns_security/d-1" && r.Method == http.MethodDelete:
			deleteCalls++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewDnsSecurityDeleteBatcher(apiClient, client.BatcherTuning{})

	_, err = b.Enqueue(context.Background(), "d-1")
	if err != nil {
		t.Fatalf("expected fallback delete success, got %v", err)
	}
	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
	if deleteCalls != 1 {
		t.Fatalf("expected 1 fallback delete call, got %d", deleteCalls)
	}
}
