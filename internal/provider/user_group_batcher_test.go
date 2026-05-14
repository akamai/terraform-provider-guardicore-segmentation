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

func TestUserGroupCreateBatcher_CoalescesAndPublishesOnce(t *testing.T) {
	var mu sync.Mutex
	createCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/visibility/user-groups" && r.Method == http.MethodPost:
			mu.Lock()
			createCalls++
			n := createCalls
			mu.Unlock()

			writeBatcherJSON(t, w, map[string]string{"id": fmt.Sprintf("ug-%d", n)})
		case r.URL.Path == "/api/v3.0/visibility/user-groups/revisions" && r.Method == http.MethodPost:
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

	batcher := NewUserGroupCreateBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	results := make([]string, n)
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := batcher.Enqueue(context.Background(), &client.UserGroupCreate{
				Title: fmt.Sprintf("group-%d", i),
				OrchestrationsGroups: []client.OrchestrationGroup{
					{OrchestrationID: "orch-1", Groups: []string{"g1"}},
				},
			})
			if err != nil {
				errCh <- err
				return
			}
			results[i] = id
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
	for i, id := range results {
		if id == "" {
			t.Fatalf("expected non-empty ID for index %d", i)
		}
	}
}

func TestUserGroupUpdateBatcher_CoalescesAndPublishesOnce(t *testing.T) {
	var mu sync.Mutex
	updateCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && len(r.URL.Path) > len("/api/v4.0/visibility/user-groups/"):
			mu.Lock()
			updateCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v3.0/visibility/user-groups/revisions" && r.Method == http.MethodPost:
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

	batcher := NewUserGroupUpdateBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), userGroupUpdateReq{
				id: fmt.Sprintf("ug-%d", i),
				userGroup: &client.UserGroupCreate{
					Title: fmt.Sprintf("updated-%d", i),
					OrchestrationsGroups: []client.OrchestrationGroup{
						{OrchestrationID: "orch-1", Groups: []string{"g1"}},
					},
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

func TestUserGroupDeleteBatcher_CoalescesAndPublishesOnce(t *testing.T) {
	var mu sync.Mutex
	deleteCalls := 0
	publishCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && len(r.URL.Path) > len("/api/v4.0/visibility/user-groups/"):
			mu.Lock()
			deleteCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/v3.0/visibility/user-groups/revisions" && r.Method == http.MethodPost:
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

	batcher := NewUserGroupDeleteBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), fmt.Sprintf("ug-%d", i))
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

func TestUserGroupCreateBatcher_PublishFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/visibility/user-groups" && r.Method == http.MethodPost:
			writeBatcherJSON(t, w, map[string]string{"id": "ug-1"})
		case r.URL.Path == "/api/v3.0/visibility/user-groups/revisions" && r.Method == http.MethodPost:
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

	batcher := NewUserGroupCreateBatcher(apiClient, client.BatcherTuning{})

	_, err = batcher.Enqueue(context.Background(), &client.UserGroupCreate{
		Title: "test",
		OrchestrationsGroups: []client.OrchestrationGroup{
			{OrchestrationID: "orch-1", Groups: []string{"g1"}},
		},
	})
	if err == nil {
		t.Fatal("expected error when publish fails, got nil")
	}
}

func TestUserGroupDeleteBatcher_IndividualFailure(t *testing.T) {
	var mu sync.Mutex
	deleteCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && len(r.URL.Path) > len("/api/v4.0/visibility/user-groups/"):
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
		case r.URL.Path == "/api/v3.0/visibility/user-groups/revisions" && r.Method == http.MethodPost:
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

	batcher := NewUserGroupDeleteBatcher(apiClient, client.BatcherTuning{})

	const n = 3
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := batcher.Enqueue(context.Background(), fmt.Sprintf("ug-%d", i))
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

func TestUserGroupCreateBatcher_RevisionUnchangedTreatedAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/visibility/user-groups" && r.Method == http.MethodPost:
			writeBatcherJSON(t, w, map[string]string{"id": "ug-1"})
		case r.URL.Path == "/api/v3.0/visibility/user-groups/revisions" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"Revision hasn't been changed"}`))
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	batcher := NewUserGroupCreateBatcher(apiClient, client.BatcherTuning{})

	id, err := batcher.Enqueue(context.Background(), &client.UserGroupCreate{
		Title: "test",
		OrchestrationsGroups: []client.OrchestrationGroup{
			{OrchestrationID: "orch-1", Groups: []string{"g1"}},
		},
	})
	if err != nil {
		t.Fatalf("expected no error when revision unchanged, got: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
}
