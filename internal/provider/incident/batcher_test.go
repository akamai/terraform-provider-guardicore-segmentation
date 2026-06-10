package incident

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestIncidentCreateBatcher_BulkCall(t *testing.T) {
	bulkCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4.0/incidents/bulk" && r.Method == http.MethodPost {
			bulkCalls++
			writeBatcherJSON(t, w, map[string]any{"incident_ids": []string{"i1", "i2"}})
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewIncidentCreateBatcher(apiClient, client.BatcherTuning{})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), &client.IncidentCreate{Type: "CustomIncident", Severity: "LOW", Time: 1, Description: "d", Summary: "s", AffectedAssets: []byte("[]"), Tags: []string{"x"}})
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

func TestIncidentCreateBatcher_BulkFallbackToCreateOne(t *testing.T) {
	bulkCalls := 0
	createCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/incidents/bulk" && r.Method == http.MethodPost:
			bulkCalls++
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"bulk failed"}`))
			return
		case r.URL.Path == "/api/v4.0/incidents" && r.Method == http.MethodPost:
			createCalls++
			writeBatcherJSON(t, w, map[string]any{"incident_id": "single"})
			return
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewIncidentCreateBatcher(apiClient, client.BatcherTuning{})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := b.Enqueue(context.Background(), &client.IncidentCreate{Type: "CustomIncident", Severity: "LOW", Time: 1, Description: "d", Summary: "s", AffectedAssets: []byte("[]"), Tags: []string{"x"}})
			if err != nil {
				t.Errorf("enqueue: %v", err)
			}
		}()
	}
	wg.Wait()

	if bulkCalls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", bulkCalls)
	}
	if createCalls != 2 {
		t.Fatalf("expected 2 fallback create calls, got %d", createCalls)
	}
}

func TestIncidentCreateBatcher_BulkCountMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4.0/incidents/bulk" && r.Method == http.MethodPost {
			writeBatcherJSON(t, w, map[string]any{"incident_ids": []string{"i1"}})
			return
		}
		if r.URL.Path == "/api/v4.0/incidents" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"fallback should fail"}`))
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "token"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	b := NewIncidentCreateBatcher(apiClient, client.BatcherTuning{})

	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, e := b.Enqueue(context.Background(), &client.IncidentCreate{Type: "CustomIncident", Severity: "LOW", Time: 1, Description: "d", Summary: "s", AffectedAssets: []byte("[]"), Tags: []string{"x"}})
			errCh <- e
		}()
	}

	hadErr := false
	for i := 0; i < 2; i++ {
		err = <-errCh
		if err != nil {
			hadErr = true
			if !strings.Contains(err.Error(), "fallback should fail") {
				t.Fatalf("expected fallback error message, got: %v", err)
			}
		}
	}
	if !hadErr {
		t.Fatal("expected at least one error, got none")
	}
}
