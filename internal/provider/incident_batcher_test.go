package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

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
