package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testJWTGenerated = "eyJ0.generated.token"
	testJWTRefreshed = "eyJ0.refreshed.token"
	testJWTNew       = "eyJ0.new.token"
	testJWTOld       = "eyJ0.old.token"
)

// writeJSON is a test helper that writes JSON response and handles encoding errors.
func writeJSON(t *testing.T, w http.ResponseWriter, v interface{}) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("failed to encode response: %v", err)
	}
}

func TestNewClient_WithAccessToken(t *testing.T) {
	config := Config{
		BaseURL:     "https://guardicore.example.com",
		AccessToken: "test-access-token",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if client.token != "test-access-token" {
		t.Errorf("expected token to be 'test-access-token', got '%s'", client.token)
	}
}

func TestNewClient_DefaultTimeout(t *testing.T) {
	c, err := NewClient(Config{
		BaseURL:     "https://guardicore.example.com",
		AccessToken: "test-token",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := DefaultRequestTimeout * time.Second
	if c.httpClient.Timeout != expected {
		t.Errorf("expected timeout %v, got %v", expected, c.httpClient.Timeout)
	}
}

func TestNewClient_CustomTimeout(t *testing.T) {
	c, err := NewClient(Config{
		BaseURL:        "https://guardicore.example.com",
		AccessToken:    "test-token",
		RequestTimeout: 120,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if c.httpClient.Timeout != 120*time.Second {
		t.Errorf("expected timeout 120s, got %v", c.httpClient.Timeout)
	}
}

func TestNewClient_ZeroTimeoutUsesDefault(t *testing.T) {
	c, err := NewClient(Config{
		BaseURL:        "https://guardicore.example.com",
		AccessToken:    "test-token",
		RequestTimeout: 0,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := DefaultRequestTimeout * time.Second
	if c.httpClient.Timeout != expected {
		t.Errorf("expected default timeout %v, got %v", expected, c.httpClient.Timeout)
	}
}

func TestNewClient_WithUsernamePassword(t *testing.T) {
	// Create a mock server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3.0/authenticate" {
			t.Errorf("expected path '/api/v3.0/authenticate', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var authReq AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if authReq.Username != "testuser" {
			t.Errorf("expected username 'testuser', got '%s'", authReq.Username)
		}
		if authReq.Password != "testpass" {
			t.Errorf("expected password 'testpass', got '%s'", authReq.Password)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, AuthResponse{
			AccessToken: testJWTGenerated,
		})
	}))
	defer server.Close()

	config := Config{
		BaseURL:  server.URL,
		Username: "testuser",
		Password: "testpass",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if client.token != testJWTGenerated {
		t.Errorf("expected token to be %q, got '%s'", testJWTGenerated, client.token)
	}
}

func TestNewClient_WithRefreshToken(t *testing.T) {
	// Create a mock server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3.0/authenticate/refresh" {
			t.Errorf("expected path '/api/v3.0/authenticate/refresh', got '%s'", r.URL.Path)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer refresh-token-123" {
			t.Errorf("expected Authorization header 'Bearer refresh-token-123', got '%s'", authHeader)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, AuthResponse{
			AccessToken: testJWTRefreshed,
		})
	}))
	defer server.Close()

	config := Config{
		BaseURL:      server.URL,
		RefreshToken: "refresh-token-123",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if client.token != testJWTRefreshed {
		t.Errorf("expected token to be 'refreshed-token', got '%s'", client.token)
	}
}

func TestNewClient_NoAuthMethod(t *testing.T) {
	config := Config{
		BaseURL: "https://guardicore.example.com",
	}

	_, err := NewClient(config)
	if err == nil {
		t.Fatal("expected error when no auth method provided")
	}
}

func TestClient_CreateLabel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/labels/bulk" {
			t.Errorf("expected path '/api/v4.0/labels/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var labels []LabelCreate
		if err := json.NewDecoder(r.Body).Decode(&labels); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if len(labels) != 1 {
			t.Fatalf("expected 1 label in bulk request, got %d", len(labels))
		}

		if labels[0].Key != "Environment" || labels[0].Value != "Production" {
			t.Errorf("unexpected label data: %+v", labels[0])
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, LabelBulkResponse{Succeeded: []string{"label-123"}})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	label := &LabelCreate{
		Key:   "Environment",
		Value: "Production",
	}

	result, err := client.CreateLabel(context.Background(), label)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Key != "Environment" {
		t.Errorf("expected Key 'Environment', got '%s'", result.Key)
	}
}

func TestClient_GetLabel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/labels/label-123" {
			t.Errorf("expected path '/api/v4.0/labels/label-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		// API returns wrapped response format
		writeJSON(t, w, LabelGetResponse{
			Objects: []Label{
				{
					ID:    "label-123",
					Key:   "Environment",
					Value: "Production",
				},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	label, err := client.GetLabel(context.Background(), "label-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if label.ID != "label-123" {
		t.Errorf("expected ID 'label-123', got '%s'", label.ID)
	}
	if label.Key != "Environment" {
		t.Errorf("expected Key 'Environment', got '%s'", label.Key)
	}
}

func TestClient_GetLabel_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	label, err := client.GetLabel(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}

	if label != nil {
		t.Error("expected nil label for not found")
	}
}

func TestClient_UpdateLabel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/labels/label-123" {
			t.Errorf("expected path '/api/v4.0/labels/label-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected method PUT, got '%s'", r.Method)
		}

		var label LabelUpdate
		if err := json.NewDecoder(r.Body).Decode(&label); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if label.Key != "Environment" {
			t.Errorf("expected Key 'Environment', got '%s'", label.Key)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	label := &LabelUpdate{
		Key:   "Environment",
		Value: "Staging",
	}

	result, err := client.UpdateLabel(context.Background(), "label-123", label)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Key != "Environment" {
		t.Errorf("expected Key 'Environment', got '%s'", result.Key)
	}
}

func TestClient_UpdateLabel_DynamicCriteriaAddSingle(t *testing.T) {
	var gotChanges LabelDynamicCriteriaChangesRequest
	seenChanges := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodGet:
			writeJSON(t, w, LabelGetResponse{Objects: []Label{{ID: "label-123", Key: "Environment", Value: "Production", DynamicCriteria: []LabelCriteria{}}}})
		case r.URL.Path == "/api/v3.0/visibility/labels/label-123/dynamic-criteria/changes" && r.Method == http.MethodPost:
			seenChanges = true
			if err := json.NewDecoder(r.Body).Decode(&gotChanges); err != nil {
				t.Fatalf("decode changes: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodPut:
			var req LabelUpdate
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode label update: %v", err)
			}
			if req.Key != "Environment" || req.Value != "Staging" {
				t.Fatalf("unexpected update payload: %+v", req)
			}
			if req.Criteria != nil {
				t.Fatalf("expected generic update endpoint without criteria, got %+v", req.Criteria)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}

	_, err := c.UpdateLabel(context.Background(), "label-123", &LabelUpdate{
		Key:   "Environment",
		Value: "Staging",
		Criteria: []LabelCriteria{
			{Field: "name", Op: "STARTSWITH", Argument: "web"},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !seenChanges {
		t.Fatal("expected dynamic criteria changes call")
	}
	if len(gotChanges.Added) != 1 || len(gotChanges.Modified) != 0 || len(gotChanges.Deleted) != 0 {
		t.Fatalf("unexpected changes payload: %+v", gotChanges)
	}
	if gotChanges.Added[0].Source != "User" {
		t.Fatalf("expected source User, got %q", gotChanges.Added[0].Source)
	}
	if gotChanges.Added[0].Field != "name" || gotChanges.Added[0].Op != "STARTSWITH" || gotChanges.Added[0].Argument != "web" {
		t.Fatalf("unexpected added criterion: %+v", gotChanges.Added[0])
	}
	if matched, _ := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, gotChanges.Added[0].ID); !matched {
		t.Fatalf("expected generated UUID id, got %q", gotChanges.Added[0].ID)
	}
}

func TestClient_UpdateLabel_DynamicCriteriaAddCompound(t *testing.T) {
	var gotChanges LabelDynamicCriteriaChangesRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodGet:
			writeJSON(t, w, LabelGetResponse{Objects: []Label{{ID: "label-123", Key: "Environment", Value: "Production"}}})
		case r.URL.Path == "/api/v3.0/visibility/labels/label-123/dynamic-criteria/changes" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotChanges); err != nil {
				t.Fatalf("decode changes: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}
	_, err := c.UpdateLabel(context.Background(), "label-123", &LabelUpdate{
		Key:   "Environment",
		Value: "Staging",
		Criteria: []LabelCriteria{
			{CompoundCriteria: []LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "prod"}, {Field: "image_name", Op: "CONTAINS", Argument: "nginx"}}},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(gotChanges.Added) != 1 || len(gotChanges.Added[0].CompoundCriteria) != 2 {
		t.Fatalf("unexpected compound add payload: %+v", gotChanges)
	}
}

func TestClient_UpdateLabel_DynamicCriteriaModifySingle(t *testing.T) {
	var gotChanges LabelDynamicCriteriaChangesRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodGet:
			writeJSON(t, w, LabelGetResponse{Objects: []Label{{ID: "label-123", Key: "Environment", Value: "Production", DynamicCriteria: []LabelCriteria{{ID: "crit-1", Field: "name", Op: "STARTSWITH", Argument: "old"}}}}})
		case r.URL.Path == "/api/v3.0/visibility/labels/label-123/dynamic-criteria/changes" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotChanges); err != nil {
				t.Fatalf("decode changes: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}
	_, err := c.UpdateLabel(context.Background(), "label-123", &LabelUpdate{Key: "Environment", Value: "Staging", Criteria: []LabelCriteria{{Field: "name", Op: "STARTSWITH", Argument: "new"}}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(gotChanges.Modified) != 1 || gotChanges.Modified[0].ID != "crit-1" {
		t.Fatalf("expected one modify on crit-1, got %+v", gotChanges)
	}
	if len(gotChanges.Added) != 0 || len(gotChanges.Deleted) != 0 {
		t.Fatalf("expected modify-only payload, got %+v", gotChanges)
	}
}

func TestClient_UpdateLabel_DynamicCriteriaModifyCompoundReplaceArray(t *testing.T) {
	var gotChanges LabelDynamicCriteriaChangesRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodGet:
			writeJSON(t, w, LabelGetResponse{Objects: []Label{{
				ID:              "label-123",
				Key:             "Environment",
				Value:           "Production",
				DynamicCriteria: []LabelCriteria{{ID: "crit-comp", CompoundCriteria: []LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "old"}, {Field: "image_name", Op: "CONTAINS", Argument: "old-image"}}}},
			}}})
		case r.URL.Path == "/api/v3.0/visibility/labels/label-123/dynamic-criteria/changes" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotChanges); err != nil {
				t.Fatalf("decode changes: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}
	_, err := c.UpdateLabel(context.Background(), "label-123", &LabelUpdate{
		Key:   "Environment",
		Value: "Staging",
		Criteria: []LabelCriteria{{
			CompoundCriteria: []LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "new"}, {Field: "container_command", Op: "CONTAINS", Argument: "run"}},
		}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(gotChanges.Modified) != 1 || gotChanges.Modified[0].ID != "crit-comp" {
		t.Fatalf("expected compound modify with existing id, got %+v", gotChanges)
	}
	if len(gotChanges.Modified[0].CompoundCriteria) != 2 {
		t.Fatalf("expected full compound replacement, got %+v", gotChanges.Modified[0])
	}
}

func TestClient_UpdateLabel_DynamicCriteriaDeleteSingle(t *testing.T) {
	var gotChanges LabelDynamicCriteriaChangesRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodGet:
			writeJSON(t, w, LabelGetResponse{Objects: []Label{{ID: "label-123", Key: "Environment", Value: "Production", DynamicCriteria: []LabelCriteria{{ID: "crit-delete", Field: "name", Op: "CONTAINS", Argument: "x"}}}}})
		case r.URL.Path == "/api/v3.0/visibility/labels/label-123/dynamic-criteria/changes" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotChanges); err != nil {
				t.Fatalf("decode changes: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}
	_, err := c.UpdateLabel(context.Background(), "label-123", &LabelUpdate{Key: "Environment", Value: "Staging", Criteria: []LabelCriteria{}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(gotChanges.Deleted) != 1 || gotChanges.Deleted[0] != "crit-delete" {
		t.Fatalf("expected delete crit-delete, got %+v", gotChanges)
	}
}

func TestClient_UpdateLabel_DynamicCriteriaDeleteCompound(t *testing.T) {
	var gotChanges LabelDynamicCriteriaChangesRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodGet:
			writeJSON(t, w, LabelGetResponse{Objects: []Label{{ID: "label-123", Key: "Environment", Value: "Production", DynamicCriteria: []LabelCriteria{{ID: "crit-comp-del", CompoundCriteria: []LabelCriteria{{Field: "image_name", Op: "CONTAINS", Argument: "nginx"}}}}}}})
		case r.URL.Path == "/api/v3.0/visibility/labels/label-123/dynamic-criteria/changes" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotChanges); err != nil {
				t.Fatalf("decode changes: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}
	_, err := c.UpdateLabel(context.Background(), "label-123", &LabelUpdate{Key: "Environment", Value: "Staging", Criteria: []LabelCriteria{}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(gotChanges.Deleted) != 1 || gotChanges.Deleted[0] != "crit-comp-del" {
		t.Fatalf("expected delete crit-comp-del, got %+v", gotChanges)
	}
}

func TestClient_UpdateLabel_DynamicCriteriaMixedAddModifyDelete(t *testing.T) {
	var gotChanges LabelDynamicCriteriaChangesRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodGet:
			writeJSON(t, w, LabelGetResponse{Objects: []Label{{
				ID:    "label-123",
				Key:   "Environment",
				Value: "Production",
				DynamicCriteria: []LabelCriteria{
					{ID: "crit-mod", Field: "name", Op: "STARTSWITH", Argument: "old"},
					{ID: "crit-del", Field: "name", Op: "CONTAINS", Argument: "to-delete"},
					{ID: "crit-keep", Field: "name", Op: "EQUALS", Argument: "keep"},
				},
			}}})
		case r.URL.Path == "/api/v3.0/visibility/labels/label-123/dynamic-criteria/changes" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotChanges); err != nil {
				t.Fatalf("decode changes: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}
	_, err := c.UpdateLabel(context.Background(), "label-123", &LabelUpdate{
		Key:   "Environment",
		Value: "Staging",
		Criteria: []LabelCriteria{
			{Field: "name", Op: "STARTSWITH", Argument: "new"},
			{Field: "name", Op: "EQUALS", Argument: "keep"},
			{Field: "name", Op: "ENDSWITH", Argument: "added"},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(gotChanges.Added) != 1 || len(gotChanges.Modified) != 1 || len(gotChanges.Deleted) != 1 {
		t.Fatalf("expected add+modify+delete, got %+v", gotChanges)
	}
	if gotChanges.Modified[0].ID != "crit-mod" {
		t.Fatalf("expected modify id crit-mod, got %+v", gotChanges.Modified)
	}
	if gotChanges.Deleted[0] != "crit-del" {
		t.Fatalf("expected delete id crit-del, got %+v", gotChanges.Deleted)
	}
}

func TestClient_UpdateLabelDynamicCriteriaChanges_UnknownModifiedOrDeletedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3.0/visibility/labels/label-123/dynamic-criteria/changes" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"description":"criterion id not found","error_code":"IllegalValue"}`)
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}
	err := c.UpdateLabelDynamicCriteriaChanges(context.Background(), "label-123", &LabelDynamicCriteriaChangesRequest{
		Added:    []LabelDynamicCriterionChange{},
		Modified: []LabelDynamicCriterionChange{{ID: "missing", Source: "User", Field: "name", Op: "EQUALS", Argument: "x"}},
		Deleted:  []string{"missing-delete"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
}

func TestClient_UpdateLabelDynamicCriteriaChanges_RejectsMalformedCriteria(t *testing.T) {
	c := &Client{config: Config{BaseURL: "https://example.invalid"}, httpClient: http.DefaultClient, token: "test-token"}
	err := c.UpdateLabelDynamicCriteriaChanges(context.Background(), "label-123", &LabelDynamicCriteriaChangesRequest{
		Added: []LabelDynamicCriterionChange{{
			ID:               "id-1",
			Source:           "User",
			Field:            "name",
			Op:               "EQUALS",
			Argument:         "x",
			CompoundCriteria: []LabelDynamicCompoundCriterion{{Field: "container_labels", Op: "STARTSWITH", Argument: "prod"}},
		}},
		Modified: []LabelDynamicCriterionChange{},
		Deleted:  []string{},
	})
	if err == nil {
		t.Fatal("expected malformed criterion error")
	}
	if !strings.Contains(err.Error(), "cannot set both") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_UpdateLabelDynamicCriteriaChanges_CompoundDoesNotRequireInnerIDs(t *testing.T) {
	seen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3.0/visibility/labels/label-123/dynamic-criteria/changes" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}

		var raw map[string]any
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode raw: %v", err)
		}

		added, ok := raw["added"].([]any)
		if !ok || len(added) != 1 {
			t.Fatalf("expected one added, got %+v", raw)
		}
		entry, ok := added[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected added entry: %#v", added[0])
		}
		compound, ok := entry["compound_criteria"].([]any)
		if !ok || len(compound) != 1 {
			t.Fatalf("expected one compound row, got %+v", entry)
		}
		inner, ok := compound[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected inner row: %#v", compound[0])
		}
		if _, exists := inner["id"]; exists {
			t.Fatalf("compound inner rows must not include id: %+v", inner)
		}

		seen = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}
	err := c.UpdateLabelDynamicCriteriaChanges(context.Background(), "label-123", &LabelDynamicCriteriaChangesRequest{
		Added: []LabelDynamicCriterionChange{{
			ID:     "crit-1",
			Source: "User",
			CompoundCriteria: []LabelDynamicCompoundCriterion{{
				Field:    "container_labels",
				Op:       "STARTSWITH",
				Argument: "prod",
			}},
		}},
		Modified: []LabelDynamicCriterionChange{},
		Deleted:  []string{},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !seen {
		t.Fatal("expected endpoint call")
	}
}

func TestClient_UpdateLabel_DynamicCriteriaCounterMatchesRequestSize(t *testing.T) {
	var gotChanges LabelDynamicCriteriaChangesRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodGet:
			writeJSON(t, w, LabelGetResponse{Objects: []Label{{
				ID:              "label-123",
				Key:             "Environment",
				Value:           "Production",
				DynamicCriteria: []LabelCriteria{{ID: "keep-1", Field: "name", Op: "EQUALS", Argument: "keep"}, {ID: "del-1", Field: "name", Op: "EQUALS", Argument: "del"}},
			}}})
		case r.URL.Path == "/api/v3.0/visibility/labels/label-123/dynamic-criteria/changes" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotChanges); err != nil {
				t.Fatalf("decode changes: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}
	desired := []LabelCriteria{{Field: "name", Op: "EQUALS", Argument: "keep"}, {Field: "name", Op: "STARTSWITH", Argument: "new-a"}, {Field: "name", Op: "ENDSWITH", Argument: "new-b"}}
	_, err := c.UpdateLabel(context.Background(), "label-123", &LabelUpdate{Key: "Environment", Value: "Staging", Criteria: desired})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	currentCount := 2
	computed := currentCount + len(gotChanges.Added) - len(gotChanges.Deleted)
	if computed != len(desired) {
		t.Fatalf("expected computed dynamic criteria count %d, got %d (changes=%+v)", len(desired), computed, gotChanges)
	}
}

func TestClient_CreateLabel_WithInitialDynamicCriteria(t *testing.T) {
	seenCriteria := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/labels/bulk" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}

		var labels []LabelCreate
		if err := json.NewDecoder(r.Body).Decode(&labels); err != nil {
			t.Fatalf("decode labels: %v", err)
		}
		if len(labels) != 1 {
			t.Fatalf("expected 1 label, got %d", len(labels))
		}
		if len(labels[0].Criteria) != 2 {
			t.Fatalf("expected 2 initial criteria, got %+v", labels[0].Criteria)
		}
		seenCriteria = true
		writeJSON(t, w, LabelBulkResponse{Succeeded: []string{"label-123"}})
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}
	_, err := c.CreateLabel(context.Background(), &LabelCreate{
		Key:   "Environment",
		Value: "Production",
		Criteria: []LabelCriteria{
			{Field: "name", Op: "STARTSWITH", Argument: "web"},
			{CompoundCriteria: []LabelCriteria{{Field: "container_labels", Op: "CONTAINS", Argument: "prod"}}},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !seenCriteria {
		t.Fatal("expected create payload to include initial criteria")
	}
}

func TestClient_DeleteLabel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4.0/labels/bulk_delete" && r.Method == http.MethodDelete:
			var items []LabelBulkDeleteItem
			if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}
			if len(items) != 1 || items[0].ID != "label-123" {
				t.Fatalf("unexpected bulk delete body: %+v", items)
			}
			writeJSON(t, w, LabelBulkResponse{Succeeded: []string{"label-123"}})
		case r.URL.Path == "/api/v4.0/labels/label-123" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteLabel(context.Background(), "label-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_ListLabels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/labels" {
			t.Errorf("expected path '/api/v4.0/labels', got '%s'", r.URL.Path)
		}

		key := r.URL.Query().Get("key")
		value := r.URL.Query().Get("value")

		if key != "Environment" || value != "Production" {
			t.Errorf("expected key=Environment&value=Production, got key=%s&value=%s", key, value)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListLabelsResponse{
			Objects: []Label{
				{ID: "label-1", Key: "Environment", Value: "Production"},
			},
			TotalCount: 1,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	labels, err := client.ListLabels(context.Background(), "Environment", "Production")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(labels) != 1 {
		t.Errorf("expected 1 label, got %d", len(labels))
	}
}

func TestClient_RetryOn401(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/api/v3.0/authenticate" {
			w.Header().Set("Content-Type", "application/json")
			writeJSON(t, w, AuthResponse{AccessToken: testJWTNew})
			return
		}

		if callCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// API returns wrapped response format
		writeJSON(t, w, LabelGetResponse{
			Objects: []Label{
				{ID: "label-123", Key: "Test", Value: "Value"},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		config: Config{
			BaseURL:  server.URL,
			Username: "user",
			Password: "pass",
		},
		httpClient: http.DefaultClient,
		token:      testJWTOld,
	}

	label, err := client.GetLabel(context.Background(), "label-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if label == nil {
		t.Fatal("expected label, got nil")
	}

	if client.token != testJWTNew {
		t.Errorf("expected token to be updated to 'new-token', got '%s'", client.token)
	}
}

func TestClient_CreateLabelGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/label-groups" {
			t.Errorf("expected path '/api/v4.0/label-groups', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, CreateResponse{ID: "group-123"})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	group := &LabelGroupCreate{
		Key:      "Server Group",
		Value:    "Web Servers",
		Comments: "All web servers",
	}

	result, err := client.CreateLabelGroup(context.Background(), group)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.ID != "group-123" {
		t.Errorf("expected ID 'group-123', got '%s'", result.ID)
	}
}

func TestClient_UpdateLabelGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/label-groups/group-123" {
			t.Errorf("expected path '/api/v4.0/label-groups/group-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected method PUT, got '%s'", r.Method)
		}

		var group LabelGroupCreate
		if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if group.Key != "Server Group" {
			t.Errorf("expected Key 'Server Group', got '%s'", group.Key)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	group := &LabelGroupCreate{
		Key:      "Server Group",
		Value:    "Updated Web",
		Comments: "Updated comment",
	}

	result, err := client.UpdateLabelGroup(context.Background(), "group-123", group)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.ID != "group-123" {
		t.Errorf("expected ID 'group-123', got '%s'", result.ID)
	}
	if result.Key != "Server Group" {
		t.Errorf("expected Key 'Server Group', got '%s'", result.Key)
	}
}

func TestClient_UpdateLabelGroup_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.UpdateLabelGroup(context.Background(), "group-123", &LabelGroupCreate{Key: "k", Value: "v"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "failed to update label group") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClient_DeleteLabelGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/label-groups/group-123" {
			t.Errorf("expected path '/api/v4.0/label-groups/group-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected method DELETE, got '%s'", r.Method)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteLabelGroup(context.Background(), "group-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_DeleteLabelGroup_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteLabelGroup(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}
}

func TestClient_DeleteLabelGroup_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteLabelGroup(context.Background(), "group-123")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "failed to delete label group") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClient_ListLabelGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/label-groups" {
			t.Errorf("expected path '/api/v4.0/label-groups', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		key := r.URL.Query().Get("key")
		value := r.URL.Query().Get("value")

		if key != "Server Group" || value != "Web" {
			t.Errorf("expected key=Server Group&value=Web, got key=%s&value=%s", key, value)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListLabelGroupsResponse{
			Objects: []LabelGroup{
				{ID: "group-1", Key: "Server Group", Value: "Web"},
			},
			TotalCount: 1,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	groups, err := client.ListLabelGroups(context.Background(), "Server Group", "Web")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
	if groups[0].ID != "group-1" {
		t.Errorf("expected ID 'group-1', got '%s'", groups[0].ID)
	}
}

func TestClient_ListLabelGroups_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		offset := r.URL.Query().Get("offset")

		if offset == "0" {
			groups := make([]LabelGroup, defaultPageSize)
			for i := 0; i < defaultPageSize; i++ {
				groups[i] = LabelGroup{
					ID:    fmt.Sprintf("group-%d", i),
					Key:   "Key",
					Value: fmt.Sprintf("Value-%d", i),
				}
			}
			writeJSON(t, w, ListLabelGroupsResponse{
				Objects:    groups,
				TotalCount: defaultPageSize + 3,
			})
		} else {
			groups := make([]LabelGroup, 3)
			for i := 0; i < 3; i++ {
				groups[i] = LabelGroup{
					ID:    fmt.Sprintf("group-%d", defaultPageSize+i),
					Key:   "Key",
					Value: fmt.Sprintf("Value-%d", defaultPageSize+i),
				}
			}
			writeJSON(t, w, ListLabelGroupsResponse{
				Objects:    groups,
				TotalCount: defaultPageSize + 3,
			})
		}
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	groups, err := client.ListLabelGroups(context.Background(), "", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedTotal := defaultPageSize + 3
	if len(groups) != expectedTotal {
		t.Errorf("expected %d groups, got %d", expectedTotal, len(groups))
	}

	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestClient_ListLabelGroups_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.ListLabelGroups(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "failed to list label groups") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClient_PublishLabelGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/label-groups/publish" {
			t.Errorf("expected path '/api/v4.0/label-groups/publish', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.PublishLabelGroups(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_CreatePolicyRule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/policy/rules" {
			t.Errorf("expected path '/api/v4.0/visibility/policy/rules', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, CreateResponse{ID: "rule-123"})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	spec := map[string]interface{}{
		"action":  "ALLOW",
		"enabled": true,
	}

	id, err := client.CreatePolicyRule(context.Background(), spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if id != "rule-123" {
		t.Errorf("expected ID 'rule-123', got '%s'", id)
	}
}

func TestClient_BulkCreatePolicyRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/policy/rules/bulk" {
			t.Errorf("expected path '/api/v4.0/visibility/policy/rules/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var body []map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if len(body) != 2 {
			t.Fatalf("expected 2 policy rules in request body, got %d", len(body))
		}

		writeJSON(t, w, PolicyRulesBulkCreateResponse{
			NumberOfFailed:    0,
			NumberOfSucceeded: 2,
			Result:            "success",
			Succeeded:         []string{"rule-1", "rule-2"},
			TotalNumber:       2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	specs := []map[string]any{
		{"action": "ALLOW", "enabled": true},
		{"action": "BLOCK", "enabled": true},
	}

	resp, err := client.BulkCreatePolicyRules(context.Background(), specs)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Succeeded) != 2 {
		t.Fatalf("expected 2 succeeded IDs, got %d", len(resp.Succeeded))
	}
}

func TestClient_GetPolicyRule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/policy/rules/rule-123" {
			t.Errorf("expected path '/api/v4.0/visibility/policy/rules/rule-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, map[string]interface{}{
			"id":      "rule-123",
			"action":  "ALLOW",
			"enabled": true,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	rule, err := client.GetPolicyRule(context.Background(), "rule-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if rule == nil {
		t.Fatal("expected rule, got nil")
	}
	if rule["id"] != "rule-123" {
		t.Errorf("expected id 'rule-123', got '%v'", rule["id"])
	}
	if rule["action"] != "ALLOW" {
		t.Errorf("expected action 'ALLOW', got '%v'", rule["action"])
	}
}

func TestClient_GetPolicyRule_WrappedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/policy/rules/rule-123" {
			t.Errorf("expected path '/api/v4.0/visibility/policy/rules/rule-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, PolicyRuleGetResponse{
			Objects: []map[string]interface{}{
				{"id": "rule-123", "comments": "wrapped rule"},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	rule, err := client.GetPolicyRule(context.Background(), "rule-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if rule == nil {
		t.Fatal("expected rule, got nil")
	}
	if rule["id"] != "rule-123" {
		t.Errorf("expected id 'rule-123', got '%v'", rule["id"])
	}
	if rule["comments"] != "wrapped rule" {
		t.Errorf("expected comments 'wrapped rule', got '%v'", rule["comments"])
	}
}

func TestClient_GetPolicyRule_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	rule, err := client.GetPolicyRule(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}

	if rule != nil {
		t.Error("expected nil rule for not found")
	}
}

func TestClient_GetPolicyRule_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.GetPolicyRule(context.Background(), "rule-123")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "failed to get policy rule") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClient_UpdatePolicyRule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/policy/rules/rule-123" {
			t.Errorf("expected path '/api/v4.0/visibility/policy/rules/rule-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected method PUT, got '%s'", r.Method)
		}

		var spec map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if spec["action"] != "BLOCK" {
			t.Errorf("expected action 'BLOCK', got '%v'", spec["action"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	spec := map[string]interface{}{
		"action":  "BLOCK",
		"enabled": true,
	}

	err := client.UpdatePolicyRule(context.Background(), "rule-123", spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_UpdatePolicyRule_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.UpdatePolicyRule(context.Background(), "rule-123", map[string]interface{}{"action": "BLOCK"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "failed to update policy rule") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClient_BulkEditDnsBlocklists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/dns_security/bulk" {
			t.Errorf("expected path '/api/v4.0/dns_security/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Errorf("expected method PATCH, got '%s'", r.Method)
		}

		var req BulkEditDnsBlocklistRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if len(req.Items) != 2 {
			t.Errorf("expected 2 items, got %d", len(req.Items))
		}
		if req.Items[0].ID != "bl-1" {
			t.Errorf("expected first item ID 'bl-1', got '%s'", req.Items[0].ID)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	name1 := "Updated 1"
	name2 := "Updated 2"
	req := &BulkEditDnsBlocklistRequest{
		Items: []BulkEditDnsBlocklistItem{
			{ID: "bl-1", Name: &name1},
			{ID: "bl-2", Name: &name2},
		},
	}

	err := client.BulkEditDnsBlocklists(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_BulkEditDnsBlocklists_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("bulk edit failed"))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.BulkEditDnsBlocklists(context.Background(), &BulkEditDnsBlocklistRequest{})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "failed to bulk edit DNS blocklists") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClient_CreatePolicyRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/policy/revisions" {
			t.Errorf("expected path '/api/v4.0/visibility/policy/revisions', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var req PolicyRevisionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if req.Comments != "Published via Terraform" {
			t.Errorf("expected comments to be 'Published via Terraform', got '%s'", req.Comments)
		}
		if req.Origin == nil || *req.Origin != "API_CALL" {
			if req.Origin == nil {
				t.Errorf("expected origin to be set, got nil")
			} else {
				t.Errorf("expected origin 'API_CALL', got '%s'", *req.Origin)
			}
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	origin := "API_CALL"
	req := &PolicyRevisionRequest{
		Comments: "Published via Terraform",
		Rulesets: []string{},
		Origin:   &origin,
	}

	err := client.CreatePolicyRevision(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_DeletePolicyRule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Policy rules use POST to /delete/{id} endpoint.
		if r.URL.Path != "/api/v4.0/visibility/policy/rules/delete/rule-123" {
			t.Errorf("expected path '/api/v4.0/visibility/policy/rules/delete/rule-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeletePolicyRule(context.Background(), "rule-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_ListPolicyRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/policy/rules" {
			t.Errorf("expected path '/api/v4.0/visibility/policy/rules', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		offset := r.URL.Query().Get("offset")
		if offset != "0" {
			t.Errorf("expected offset=0, got %s", offset)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListPolicyRulesResponse{
			Objects: []map[string]interface{}{
				{"id": "rule-1", "action": "ALLOW", "enabled": true},
				{"id": "rule-2", "action": "BLOCK", "enabled": false},
			},
			TotalCount: 2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	rules, err := client.ListPolicyRules(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}

	if rules[0]["id"] != "rule-1" {
		t.Errorf("expected first rule ID 'rule-1', got '%v'", rules[0]["id"])
	}
}

func TestClient_ListPolicyRules_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		offset := r.URL.Query().Get("offset")

		if offset == "0" {
			// First page: return defaultPageSize items
			rules := make([]map[string]interface{}, defaultPageSize)
			for i := 0; i < defaultPageSize; i++ {
				rules[i] = map[string]interface{}{
					"id":     fmt.Sprintf("rule-%d", i),
					"action": "ALLOW",
				}
			}
			writeJSON(t, w, ListPolicyRulesResponse{
				Objects:    rules,
				TotalCount: defaultPageSize + 5,
			})
		} else {
			// Second page: return remaining 5 items
			rules := make([]map[string]interface{}, 5)
			for i := 0; i < 5; i++ {
				rules[i] = map[string]interface{}{
					"id":     fmt.Sprintf("rule-%d", defaultPageSize+i),
					"action": "BLOCK",
				}
			}
			writeJSON(t, w, ListPolicyRulesResponse{
				Objects:    rules,
				TotalCount: defaultPageSize + 5,
			})
		}
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	rules, err := client.ListPolicyRules(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedTotal := defaultPageSize + 5
	if len(rules) != expectedTotal {
		t.Errorf("expected %d rules, got %d", expectedTotal, len(rules))
	}

	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestClient_GetLabelGroup_WithNestedLabels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/label-groups/group-123" {
			t.Errorf("expected path '/api/v4.0/label-groups/group-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		// Simulate actual API response with full nested label objects
		response := `{
			"objects": [{
				"id": "group-123",
				"key": "Server Group",
				"value": "Production Web",
				"comments": "Test group",
				"include_labels": {
					"or_labels": [{
						"and_labels": [{
							"id": "label-1",
							"key": "Environment",
							"value": "Production",
							"name": "Environment: Production",
							"color_index": 0
						}, {
							"id": "label-2",
							"key": "Application",
							"value": "Web",
							"name": "Application: Web",
							"color_index": 1
						}]
					}]
				},
				"exclude_labels": {
					"or_labels": [{
						"and_labels": [{
							"id": "label-3",
							"key": "Status",
							"value": "Deprecated",
							"name": "Status: Deprecated",
							"color_index": 2
						}]
					}]
				}
			}]
		}`
		if _, err := w.Write([]byte(response)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	labelGroup, err := client.GetLabelGroup(context.Background(), "group-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if labelGroup == nil {
		t.Fatal("expected label group, got nil")
		return
	}

	if labelGroup.ID != "group-123" {
		t.Errorf("expected ID 'group-123', got '%s'", labelGroup.ID)
	}

	// Verify include_labels structure
	if labelGroup.IncludeLabels == nil {
		t.Fatal("expected include_labels, got nil")
	}

	if len(labelGroup.IncludeLabels.OrLabels) != 1 {
		t.Errorf("expected 1 or_label, got %d", len(labelGroup.IncludeLabels.OrLabels))
	}

	if len(labelGroup.IncludeLabels.OrLabels[0].AndLabels) != 2 {
		t.Errorf("expected 2 and_labels, got %d", len(labelGroup.IncludeLabels.OrLabels[0].AndLabels))
	}

	firstLabel := labelGroup.IncludeLabels.OrLabels[0].AndLabels[0]
	if firstLabel.ID != "label-1" {
		t.Errorf("expected label ID 'label-1', got '%s'", firstLabel.ID)
	}
	if firstLabel.Key != "Environment" {
		t.Errorf("expected label key 'Environment', got '%s'", firstLabel.Key)
	}

	// Verify exclude_labels structure
	if labelGroup.ExcludeLabels == nil {
		t.Fatal("expected exclude_labels, got nil")
	}

	if len(labelGroup.ExcludeLabels.OrLabels) != 1 {
		t.Errorf("expected 1 exclude or_label, got %d", len(labelGroup.ExcludeLabels.OrLabels))
	}

	if len(labelGroup.ExcludeLabels.OrLabels[0].AndLabels) != 1 {
		t.Errorf("expected 1 exclude and_label, got %d", len(labelGroup.ExcludeLabels.OrLabels[0].AndLabels))
	}

	excludeLabel := labelGroup.ExcludeLabels.OrLabels[0].AndLabels[0]
	if excludeLabel.ID != "label-3" {
		t.Errorf("expected exclude label ID 'label-3', got '%s'", excludeLabel.ID)
	}
}

func TestClient_CreateDnsBlocklist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/dns_security" {
			t.Errorf("expected path '/api/v4.0/dns_security', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var blocklist DnsBlocklistCreate
		if err := json.NewDecoder(r.Body).Decode(&blocklist); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if blocklist.Name != "Test Block" {
			t.Errorf("expected Name 'Test Block', got '%s'", blocklist.Name)
		}
		if blocklist.Type != "CUSTOM_BLOCK" {
			t.Errorf("expected Type 'CUSTOM_BLOCK', got '%s'", blocklist.Type)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, CreateResponse{ID: "blocklist-123"})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	blocklist := &DnsBlocklistCreate{
		Name: "Test Block",
		Type: "CUSTOM_BLOCK",
	}

	id, err := client.CreateDnsBlocklist(context.Background(), blocklist)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if id != "blocklist-123" {
		t.Errorf("expected ID 'blocklist-123', got '%s'", id)
	}
}

func TestClient_GetDnsBlocklist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/dns_security" {
			t.Errorf("expected path '/api/v4.0/dns_security', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}
		ids := r.URL.Query().Get("ids")
		if ids != "blocklist-123" {
			t.Errorf("expected ids query param 'blocklist-123', got '%s'", ids)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListDnsBlocklistsResponse{
			Objects: []DnsBlocklist{
				{
					ID:      "blocklist-123",
					Name:    "Test Block",
					Type:    "CUSTOM_BLOCK",
					Enabled: true,
				},
			},
			TotalCount: 1,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	blocklist, err := client.GetDnsBlocklist(context.Background(), "blocklist-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if blocklist == nil {
		t.Fatal("expected blocklist, got nil")
		return
	}
	if blocklist.ID != "blocklist-123" {
		t.Errorf("expected ID 'blocklist-123', got '%s'", blocklist.ID)
	}
	if blocklist.Name != "Test Block" {
		t.Errorf("expected Name 'Test Block', got '%s'", blocklist.Name)
	}
	if blocklist.Type != "CUSTOM_BLOCK" {
		t.Errorf("expected Type 'CUSTOM_BLOCK', got '%s'", blocklist.Type)
	}
	if !blocklist.Enabled {
		t.Error("expected Enabled to be true")
	}
}

func TestClient_GetDnsBlocklist_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	blocklist, err := client.GetDnsBlocklist(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}

	if blocklist != nil {
		t.Error("expected nil blocklist for not found")
	}
}

func TestClient_UpdateDnsBlocklist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/dns_security/blocklist-123" {
			t.Errorf("expected path '/api/v4.0/dns_security/blocklist-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Errorf("expected method PATCH, got '%s'", r.Method)
		}

		var edit DnsBlocklistEdit
		if err := json.NewDecoder(r.Body).Decode(&edit); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	name := "Updated Block"
	edit := &DnsBlocklistEdit{
		Name: &name,
	}

	err := client.UpdateDnsBlocklist(context.Background(), "blocklist-123", edit)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_DeleteDnsBlocklist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/dns_security/blocklist-123" {
			t.Errorf("expected path '/api/v4.0/dns_security/blocklist-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected method DELETE, got '%s'", r.Method)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteDnsBlocklist(context.Background(), "blocklist-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_DeleteDnsBlocklist_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteDnsBlocklist(context.Background(), "blocklist-123")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}
}

func TestClient_ListDnsBlocklists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/dns_security" {
			t.Errorf("expected path '/api/v4.0/dns_security', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		startAt := r.URL.Query().Get("start_at")
		if startAt != "0" {
			t.Errorf("expected start_at=0, got %s", startAt)
		}

		maxResults := r.URL.Query().Get("max_results")
		if maxResults != fmt.Sprintf("%d", defaultPageSize) {
			t.Errorf("expected max_results=%d, got %s", defaultPageSize, maxResults)
		}

		name := r.URL.Query().Get("name")
		if name != "Test" {
			t.Errorf("expected name=Test, got %s", name)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListDnsBlocklistsResponse{
			Objects: []DnsBlocklist{
				{ID: "bl-1", Name: "Test Block 1", Type: "CUSTOM_BLOCK", Enabled: true},
				{ID: "bl-2", Name: "Test Block 2", Type: "CUSTOM_BLOCK", Enabled: false},
			},
			TotalCount: 2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	blocklists, err := client.ListDnsBlocklists(context.Background(), "Test", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(blocklists) != 2 {
		t.Errorf("expected 2 blocklists, got %d", len(blocklists))
	}
}

func TestClient_ListDnsBlocklists_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		startAt := r.URL.Query().Get("start_at")

		if startAt == "0" {
			// First page: return defaultPageSize items
			items := make([]DnsBlocklist, defaultPageSize)
			for i := 0; i < defaultPageSize; i++ {
				items[i] = DnsBlocklist{
					ID:      fmt.Sprintf("bl-%d", i),
					Name:    fmt.Sprintf("Block %d", i),
					Type:    "CUSTOM_BLOCK",
					Enabled: true,
				}
			}
			writeJSON(t, w, ListDnsBlocklistsResponse{
				Objects:    items,
				TotalCount: defaultPageSize + 5,
			})
		} else {
			// Second page: return remaining 5 items
			items := make([]DnsBlocklist, 5)
			for i := 0; i < 5; i++ {
				items[i] = DnsBlocklist{
					ID:      fmt.Sprintf("bl-%d", defaultPageSize+i),
					Name:    fmt.Sprintf("Block %d", defaultPageSize+i),
					Type:    "CUSTOM_BLOCK",
					Enabled: true,
				}
			}
			writeJSON(t, w, ListDnsBlocklistsResponse{
				Objects:    items,
				TotalCount: defaultPageSize + 5,
			})
		}
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	blocklists, err := client.ListDnsBlocklists(context.Background(), "", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedTotal := defaultPageSize + 5
	if len(blocklists) != expectedTotal {
		t.Errorf("expected %d blocklists, got %d", expectedTotal, len(blocklists))
	}

	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestClient_BulkCreateDnsBlocklists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/dns_security/bulk" {
			t.Errorf("expected path '/api/v4.0/dns_security/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, BulkCreateDnsBlocklistResponse{IDs: []string{"id-1", "id-2"}})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	items := []DnsBlocklistCreate{
		{Name: "Block 1", Type: "CUSTOM_BLOCK"},
		{Name: "Block 2", Type: "CUSTOM_BLOCK"},
	}

	result, err := client.BulkCreateDnsBlocklists(context.Background(), items)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.IDs) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(result.IDs))
	}
	if result.IDs[0] != "id-1" || result.IDs[1] != "id-2" {
		t.Errorf("expected IDs ['id-1', 'id-2'], got %v", result.IDs)
	}
}

func TestClient_BulkDeleteDnsBlocklists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/dns_security/bulk" {
			t.Errorf("expected path '/api/v4.0/dns_security/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected method DELETE, got '%s'", r.Method)
		}

		ids := r.URL.Query().Get("ids")
		if !strings.Contains(ids, "id-1") || !strings.Contains(ids, "id-2") {
			t.Errorf("expected ids query param to contain 'id-1' and 'id-2', got '%s'", ids)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.BulkDeleteDnsBlocklists(context.Background(), []string{"id-1", "id-2"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_ResetDnsBlocklistHitCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/dns_security/blocklist-123/hits" {
			t.Errorf("expected path '/api/v4.0/dns_security/blocklist-123/hits', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected method DELETE, got '%s'", r.Method)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.ResetDnsBlocklistHitCount(context.Background(), "blocklist-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// Incident tests

func TestClient_CreateIncident(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/incidents" {
			t.Errorf("expected path '/api/v4.0/incidents', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var incident IncidentCreate
		if err := json.NewDecoder(r.Body).Decode(&incident); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if incident.Type != "CustomIncident" {
			t.Errorf("expected Type 'CustomIncident', got '%s'", incident.Type)
		}
		if incident.Severity != "HIGH" {
			t.Errorf("expected Severity 'HIGH', got '%s'", incident.Severity)
		}
		if incident.Description != "Test incident" {
			t.Errorf("expected Description 'Test incident', got '%s'", incident.Description)
		}
		if len(incident.Tags) != 1 || incident.Tags[0] != "test" {
			t.Errorf("expected Tags ['test'], got %v", incident.Tags)
		}

		w.Header().Set("Content-Type", "application/json")
		// NOTE: incident API returns incident_id, not id
		writeJSON(t, w, CreateIncidentResponse{IncidentID: "incident-123"})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	incident := &IncidentCreate{
		Type:           "CustomIncident",
		Severity:       "HIGH",
		AffectedAssets: json.RawMessage(`[{"ip":"10.0.0.1"}]`),
		Time:           1621957270000,
		Tags:           []string{"test"},
		Description:    "Test incident",
		Summary:        "### Test summary",
	}

	id, err := client.CreateIncident(context.Background(), incident)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if id != "incident-123" {
		t.Errorf("expected ID 'incident-123', got '%s'", id)
	}
}

func TestClient_GetIncident(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3.0/generic-incidents" {
			t.Errorf("expected path '/api/v3.0/generic-incidents', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		idParam := r.URL.Query().Get("id")
		if idParam != "incident-123" {
			t.Errorf("expected id query param 'incident-123', got '%s'", idParam)
		}
		fromTime := r.URL.Query().Get("from_time")
		if fromTime != "946684800000" {
			t.Errorf("expected from_time '946684800000', got '%s'", fromTime)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, map[string]interface{}{
			"objects": []map[string]interface{}{
				{
					"id":          "incident-123",
					"type":        "Reveal",
					"severity":    "MEDIUM",
					"time":        1504688829035,
					"is_legacy":   false,
					"description": "Detected by generic incidents",
				},
			},
			"total_count": 1,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	incident, err := client.GetIncident(context.Background(), "incident-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if incident == nil {
		t.Fatal("expected incident, got nil")
	}
	if incident["id"] != "incident-123" {
		t.Errorf("expected id 'incident-123', got '%v'", incident["id"])
	}
	if incident["type"] != "Reveal" {
		t.Errorf("expected type 'Reveal', got '%v'", incident["type"])
	}
	if incident["severity"] != "MEDIUM" {
		t.Errorf("expected severity 'MEDIUM', got '%v'", incident["severity"])
	}
}

func TestClient_GetIncident_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, map[string]interface{}{
			"objects":     []interface{}{},
			"total_count": 0,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	incident, err := client.GetIncident(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if incident != nil {
		t.Errorf("expected nil, got %v", incident)
	}
}

func TestClient_ListIncidents(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3.0/generic-incidents" {
			t.Errorf("expected path '/api/v3.0/generic-incidents', got '%s'", r.URL.Path)
		}

		fromTime := r.URL.Query().Get("from_time")
		if fromTime != "1000" {
			t.Errorf("expected from_time '1000', got '%s'", fromTime)
		}
		toTime := r.URL.Query().Get("to_time")
		if toTime != "2000" {
			t.Errorf("expected to_time '2000', got '%s'", toTime)
		}

		requestCount++
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, map[string]interface{}{
			"objects": []map[string]interface{}{
				{"id": fmt.Sprintf("incident-%d", requestCount), "type": "Reveal", "severity": "LOW", "time": 1504688829035},
			},
			"total_count": 1,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	incidents, err := client.ListIncidents(context.Background(), 1000, 2000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(incidents) != 1 {
		t.Errorf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0]["type"] != "Reveal" {
		t.Errorf("expected type 'Reveal', got '%v'", incidents[0]["type"])
	}
}

func TestClient_BulkCreateIncidents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/incidents/bulk" {
			t.Errorf("expected path '/api/v4.0/incidents/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var req BulkCreateIncidentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if len(req.Incidents) != 2 {
			t.Errorf("expected 2 incidents, got %d", len(req.Incidents))
		}

		w.Header().Set("Content-Type", "application/json")
		// NOTE: bulk response uses incident_ids, not ids
		writeJSON(t, w, BulkCreateIncidentResponse{
			IncidentIDs: []string{"incident-1", "incident-2"},
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	incidents := []IncidentCreate{
		{
			Type:           "CustomIncident",
			Severity:       "HIGH",
			AffectedAssets: json.RawMessage(`[{"ip":"10.0.0.1"}]`),
			Time:           1621957270000,
			Tags:           []string{"test"},
			Description:    "Incident 1",
			Summary:        "Summary 1",
		},
		{
			Type:           "CustomIncident",
			Severity:       "LOW",
			AffectedAssets: json.RawMessage(`[{"ip":"10.0.0.2"}]`),
			Time:           1621957270001,
			Tags:           []string{"test"},
			Description:    "Incident 2",
			Summary:        "Summary 2",
		},
	}

	resp, err := client.BulkCreateIncidents(context.Background(), incidents)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.IncidentIDs) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(resp.IncidentIDs))
	}
	if resp.IncidentIDs[0] != "incident-1" {
		t.Errorf("expected first ID 'incident-1', got '%s'", resp.IncidentIDs[0])
	}
}

func TestClient_GetAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/assets" {
			t.Errorf("expected path '/api/v4.0/assets', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}
		if r.URL.Query().Get("id") != "asset-456" {
			t.Errorf("expected id query param 'asset-456', got '%s'", r.URL.Query().Get("id"))
		}
		if r.URL.Query().Get("max_results") != "1" {
			t.Errorf("expected max_results query param '1', got '%s'", r.URL.Query().Get("max_results"))
		}
		if r.URL.Query().Get("expand") != "labels" {
			t.Errorf("expected expand query param 'labels', got '%s'", r.URL.Query().Get("expand"))
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListAssetsResponse{
			Objects: []Asset{
				{
					ID:     "asset-456",
					Name:   "test-asset",
					Status: "on",
					Nics: []AssetNIC{
						{IPAddresses: []string{"10.0.0.1"}, MacAddress: "00:11:22:33:44:55"},
					},
				},
			},
			TotalCount: 1,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	asset, err := client.GetAsset(context.Background(), "asset-456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if asset.ID != "asset-456" {
		t.Errorf("expected ID 'asset-456', got '%s'", asset.ID)
	}
	if asset.Name != "test-asset" {
		t.Errorf("expected name 'test-asset', got '%s'", asset.Name)
	}
	if asset.Status != "on" {
		t.Errorf("expected status 'on', got '%s'", asset.Status)
	}
	if len(asset.Nics) != 1 {
		t.Fatalf("expected 1 NIC, got %d", len(asset.Nics))
	}
	if asset.Nics[0].MacAddress != "00:11:22:33:44:55" {
		t.Errorf("expected MAC '00:11:22:33:44:55', got '%s'", asset.Nics[0].MacAddress)
	}
}

func TestClient_GetAsset_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return empty list to simulate not found
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListAssetsResponse{
			Objects:    []Asset{},
			TotalCount: 0,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	asset, err := client.GetAsset(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}

	if asset != nil {
		t.Error("expected nil asset for not found")
	}
}

func TestClient_GetAsset_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.GetAsset(context.Background(), "asset-456")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get asset") {
		t.Errorf("expected 'failed to get asset' error, got: %v", err)
	}
}

func TestClient_DeleteAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/assets/asset-456" {
			t.Errorf("expected path '/api/v4.0/assets/asset-456', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected method DELETE, got '%s'", r.Method)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteAsset(context.Background(), "asset-456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_DeleteAsset_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteAsset(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}
}

func TestClient_DeleteAsset_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteAsset(context.Background(), "asset-456")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to deactivate asset") {
		t.Errorf("expected 'failed to deactivate asset' error, got: %v", err)
	}
}

func TestClient_ListAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/assets" {
			t.Errorf("expected path '/api/v4.0/assets', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		// Verify pagination params
		if r.URL.Query().Get("start_at") == "" {
			t.Error("expected start_at query parameter")
		}
		if r.URL.Query().Get("max_results") == "" {
			t.Error("expected max_results query parameter")
		}
		if r.URL.Query().Get("expand") != "labels" {
			t.Errorf("expected expand query parameter 'labels', got '%s'", r.URL.Query().Get("expand"))
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListAssetsResponse{
			Objects: []Asset{
				{ID: "asset-1", Name: "asset-one", Status: "on"},
				{ID: "asset-2", Name: "asset-two", Status: "off"},
			},
			TotalCount: 2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	assets, err := client.ListAssets(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}
	if assets[0].ID != "asset-1" {
		t.Errorf("expected first ID 'asset-1', got '%s'", assets[0].ID)
	}
	if assets[1].Name != "asset-two" {
		t.Errorf("expected second name 'asset-two', got '%s'", assets[1].Name)
	}
}

func TestClient_ListAssets_WithNameFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nameFilter := r.URL.Query().Get("name")
		if nameFilter != "web-server" {
			t.Errorf("expected name filter 'web-server', got '%s'", nameFilter)
		}
		if r.URL.Query().Get("expand") != "labels" {
			t.Errorf("expected expand query parameter 'labels', got '%s'", r.URL.Query().Get("expand"))
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListAssetsResponse{
			Objects: []Asset{
				{ID: "asset-1", Name: "web-server"},
			},
			TotalCount: 1,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	assets, err := client.ListAssets(context.Background(), "web-server")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
}

func TestClient_ListAssets_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// First page: return full page to trigger pagination
			assets := make([]Asset, defaultPageSize)
			for i := range assets {
				assets[i] = Asset{ID: fmt.Sprintf("asset-%d", i), Name: fmt.Sprintf("asset-%d", i)}
			}
			writeJSON(t, w, ListAssetsResponse{
				Objects:    assets,
				TotalCount: defaultPageSize + 1,
			})
		} else {
			// Second page: return remaining
			writeJSON(t, w, ListAssetsResponse{
				Objects:    []Asset{{ID: "asset-last", Name: "asset-last"}},
				TotalCount: defaultPageSize + 1,
			})
		}
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	assets, err := client.ListAssets(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(assets) != defaultPageSize+1 {
		t.Errorf("expected %d assets, got %d", defaultPageSize+1, len(assets))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestClient_ListAssets_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":"forbidden"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.ListAssets(context.Background(), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list assets") {
		t.Errorf("expected 'failed to list assets' error, got: %v", err)
	}
}

func TestClient_ListAssets_PaginationSafetyLimit(t *testing.T) {
	origLimit := maxPaginationPages
	maxPaginationPages = 3
	defer func() { maxPaginationPages = origLimit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assets := make([]Asset, defaultPageSize)
		for i := range assets {
			assets[i] = Asset{ID: fmt.Sprintf("asset-%d", i), Name: fmt.Sprintf("asset-%d", i)}
		}
		writeJSON(t, w, ListAssetsResponse{
			Objects:    assets,
			TotalCount: 999999,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.ListAssets(context.Background(), "")
	if err == nil {
		t.Fatal("expected pagination safety limit error, got nil")
	}
	if !strings.Contains(err.Error(), "pagination safety limit reached") {
		t.Errorf("expected 'pagination safety limit reached' error, got: %v", err)
	}
}

func TestClient_ListAssets_TotalFieldName(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// Simulate real API: uses "total" not "total_count"
			fmt.Fprint(w, `{"objects":[{"id":"a1","name":"first"},{"id":"a2","name":"second"}],"total":3}`)
		} else {
			fmt.Fprint(w, `{"objects":[{"id":"a3","name":"third"}],"total":3}`)
		}
	}))
	defer server.Close()

	origPageSize := defaultPageSize
	defaultPageSize = 2
	defer func() { defaultPageSize = origPageSize }()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	assets, err := client.ListAssets(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(assets) != 3 {
		t.Fatalf("expected 3 assets (pagination across 2 pages), got %d — 'total' field may not be parsed correctly", len(assets))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestClient_BulkCreateAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/assets/bulk" {
			t.Errorf("expected path '/api/v4.0/assets/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		// Verify plain array body (not wrapped in object)
		var assets []AssetCreate
		if err := json.NewDecoder(r.Body).Decode(&assets); err != nil {
			t.Fatalf("failed to decode request body as array: %v", err)
		}

		if len(assets) != 2 {
			t.Errorf("expected 2 assets, got %d", len(assets))
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, BulkCreateAssetsResponse{
			NumberOfSucceeded: 2,
			NumberOfFailed:    0,
			TotalNumber:       2,
			CreatedAssetIDs:   map[string]string{"orch-1": "asset-1", "orch-2": "asset-2"},
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	assets := []AssetCreate{
		{Name: "asset-1", OrchestrationObjID: "orch-1", Nics: []AssetNIC{{IPAddresses: []string{"10.0.0.1"}}}},
		{Name: "asset-2", OrchestrationObjID: "orch-2", Nics: []AssetNIC{{IPAddresses: []string{"10.0.0.2"}}}},
	}

	bulkResp, err := client.BulkCreateAssets(context.Background(), assets)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if bulkResp.NumberOfSucceeded != 2 {
		t.Fatalf("expected 2 succeeded, got %d", bulkResp.NumberOfSucceeded)
	}
	if bulkResp.CreatedAssetIDs["orch-1"] != "asset-1" {
		t.Errorf("expected orch-1 -> asset-1, got %s", bulkResp.CreatedAssetIDs["orch-1"])
	}
	if bulkResp.CreatedAssetIDs["orch-2"] != "asset-2" {
		t.Errorf("expected orch-2 -> asset-2, got %s", bulkResp.CreatedAssetIDs["orch-2"])
	}
}

func TestClient_BulkCreateAssets_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"validation failed"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.BulkCreateAssets(context.Background(), []AssetCreate{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to bulk create assets") {
		t.Errorf("expected 'failed to bulk create assets' error, got: %v", err)
	}
}

func TestClient_BulkUpdateAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/assets/bulk" {
			t.Errorf("expected path '/api/v4.0/assets/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected method PUT, got '%s'", r.Method)
		}

		var items []AssetBulkUpdateItem
		if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
		if items[0].AssetID != "asset-1" {
			t.Errorf("expected first asset_id 'asset-1', got '%s'", items[0].AssetID)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, BulkUpdateAssetsResponse{
			NumberOfSucceeded: 2,
			NumberOfFailed:    0,
			TotalNumber:       2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	items := []AssetBulkUpdateItem{
		{AssetID: "asset-1", Name: "updated-1"},
		{AssetID: "asset-2", Name: "updated-2"},
	}

	bulkResp, err := client.BulkUpdateAssets(context.Background(), items)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if bulkResp.NumberOfSucceeded != 2 {
		t.Fatalf("expected 2 succeeded, got %d", bulkResp.NumberOfSucceeded)
	}
}

func TestClient_BulkUpdateAssets_LabelsNilOmittedAndEmptyIncluded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/assets/bulk" {
			t.Errorf("expected path '/api/v4.0/assets/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected method PUT, got '%s'", r.Method)
		}

		var raw []map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("failed to decode raw request body: %v", err)
		}
		if len(raw) != 2 {
			t.Fatalf("expected 2 items, got %d", len(raw))
		}

		if _, ok := raw[0]["labels"]; ok {
			t.Fatalf("expected labels field omitted when labels pointer is nil")
		}

		labelsVal, ok := raw[1]["labels"]
		if !ok {
			t.Fatalf("expected labels field present for explicit empty labels")
		}
		labelsArr, ok := labelsVal.([]interface{})
		if !ok {
			t.Fatalf("expected labels to be an array, got %T", labelsVal)
		}
		if len(labelsArr) != 0 {
			t.Fatalf("expected explicit empty labels array, got len=%d", len(labelsArr))
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, BulkUpdateAssetsResponse{NumberOfSucceeded: 2, NumberOfFailed: 0, TotalNumber: 2})
	}))
	defer server.Close()

	c := &Client{config: Config{BaseURL: server.URL}, httpClient: http.DefaultClient, token: "test-token"}

	emptyLabels := []AssetLabelRef{}
	items := []AssetBulkUpdateItem{
		{AssetID: "asset-1", Name: "n1", Labels: nil},
		{AssetID: "asset-2", Name: "n2", Labels: &emptyLabels},
	}

	if _, err := c.BulkUpdateAssets(context.Background(), items); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_BulkUpdateAssets_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"validation failed"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.BulkUpdateAssets(context.Background(), []AssetBulkUpdateItem{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to bulk update assets") {
		t.Errorf("expected 'failed to bulk update assets' error, got: %v", err)
	}
}

func TestClient_BulkDeactivateAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/assets/bulk/deactivate" {
			t.Errorf("expected path '/api/v4.0/assets/bulk/deactivate', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var items []BulkDeactivateAssetItem
		if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
		if items[0].AssetID != "asset-1" {
			t.Errorf("expected first asset_id 'asset-1', got '%s'", items[0].AssetID)
		}
		if items[1].AssetID != "asset-2" {
			t.Errorf("expected second asset_id 'asset-2', got '%s'", items[1].AssetID)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.BulkDeactivateAssets(context.Background(), []string{"asset-1", "asset-2"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_BulkDeactivateAssets_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"validation failed"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.BulkDeactivateAssets(context.Background(), []string{"asset-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to bulk deactivate assets") {
		t.Errorf("expected 'failed to bulk deactivate assets' error, got: %v", err)
	}
}

// isWorksiteFeatureDisabled tests

func TestIsDnsSecurityFeatureDisabled(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		expected   bool
	}{
		{
			name:       "403 with DNS feature disabled API error",
			statusCode: http.StatusForbidden,
			body:       `{"description":"Could not complete the operation due to a server error. See error for more details","error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`,
			expected:   true,
		},
		{
			name:       "403 with string fallback match",
			statusCode: http.StatusForbidden,
			body:       `{"error":"DNS Security is not enabled"}`,
			expected:   true,
		},
		{
			name:       "403 with unrelated error",
			statusCode: http.StatusForbidden,
			body:       `{"error_code":"OperationFailed","error_dump":"something else"}`,
			expected:   false,
		},
		{
			name:       "400 with DNS disabled message",
			statusCode: http.StatusBadRequest,
			body:       `{"error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`,
			expected:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isDnsSecurityFeatureDisabled(tc.statusCode, []byte(tc.body))
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestIsWorksiteFeatureDisabled(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		expected   bool
	}{
		{"400 with feature disabled message", http.StatusBadRequest, `{"error":"worksites feature is disabled"}`, true},
		{"400 with other message", http.StatusBadRequest, `{"error":"validation failed"}`, false},
		{"500 with feature disabled message", http.StatusInternalServerError, `{"error":"worksites feature is disabled"}`, false},
		{"200 with empty body", http.StatusOK, "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isWorksiteFeatureDisabled(tc.statusCode, []byte(tc.body))
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// Worksite CRUD tests

func TestClient_CreateWorksite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/worksites" {
			t.Errorf("expected path '/api/v4.0/worksites', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var ws WorksiteCreate
		if err := json.NewDecoder(r.Body).Decode(&ws); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if ws.Name != "test-worksite" {
			t.Errorf("expected name 'test-worksite', got '%s'", ws.Name)
		}
		if ws.Comment != "test comment" {
			t.Errorf("expected comment 'test comment', got '%s'", ws.Comment)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, CreateResponse{ID: "ws-123"})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	id, err := client.CreateWorksite(context.Background(), &WorksiteCreate{
		Name:    "test-worksite",
		Comment: "test comment",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if id != "ws-123" {
		t.Errorf("expected ID 'ws-123', got '%s'", id)
	}
}

func TestClient_CreateWorksite_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.CreateWorksite(context.Background(), &WorksiteCreate{Name: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create worksite") {
		t.Errorf("expected 'failed to create worksite' error, got: %v", err)
	}
}

func TestClient_CreateWorksite_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"worksites feature is disabled"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.CreateWorksite(context.Background(), &WorksiteCreate{Name: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrWorksitesFeatureDisabled {
		t.Errorf("expected ErrWorksitesFeatureDisabled, got: %v", err)
	}
}

func TestClient_GetWorksite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListWorksitesResponse{
			Objects: []Worksite{
				{ID: "ws-1", Name: "worksite-one", Comment: "first"},
				{ID: "ws-2", Name: "worksite-two", Comment: "second"},
			},
			TotalCount: 2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	ws, err := client.GetWorksite(context.Background(), "ws-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ws == nil {
		t.Fatal("expected worksite, got nil")
		return
	}
	if ws.ID != "ws-2" {
		t.Errorf("expected ID 'ws-2', got '%s'", ws.ID)
	}
	if ws.Name != "worksite-two" {
		t.Errorf("expected name 'worksite-two', got '%s'", ws.Name)
	}
}

func TestClient_GetWorksite_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListWorksitesResponse{
			Objects:    []Worksite{{ID: "ws-other", Name: "other"}},
			TotalCount: 1,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	ws, err := client.GetWorksite(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ws != nil {
		t.Errorf("expected nil, got %v", ws)
	}
}

func TestClient_UpdateWorksite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/worksites" {
			t.Errorf("expected path '/api/v4.0/worksites', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected method PUT, got '%s'", r.Method)
		}

		// Verify ID is in body (API quirk: PUT with ID in body, not URL)
		var ws WorksiteUpdate
		if err := json.NewDecoder(r.Body).Decode(&ws); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if ws.ID != "ws-123" {
			t.Errorf("expected ID 'ws-123' in body, got '%s'", ws.ID)
		}
		if ws.Name != "updated-name" {
			t.Errorf("expected name 'updated-name', got '%s'", ws.Name)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.UpdateWorksite(context.Background(), &WorksiteUpdate{
		ID:      "ws-123",
		Name:    "updated-name",
		Comment: "updated comment",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_UpdateWorksite_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.UpdateWorksite(context.Background(), &WorksiteUpdate{ID: "ws-123", Name: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update worksite") {
		t.Errorf("expected 'failed to update worksite' error, got: %v", err)
	}
}

func TestClient_UpdateWorksite_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"worksites feature is disabled"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.UpdateWorksite(context.Background(), &WorksiteUpdate{ID: "ws-123", Name: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrWorksitesFeatureDisabled {
		t.Errorf("expected ErrWorksitesFeatureDisabled, got: %v", err)
	}
}

func TestClient_DeleteWorksite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/worksites/delete_worksites" {
			t.Errorf("expected path '/api/v4.0/worksites/delete_worksites', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var req DeleteWorksitesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if len(req.ComponentIDs) != 1 || req.ComponentIDs[0] != "ws-123" {
			t.Errorf("expected component_ids ['ws-123'], got %v", req.ComponentIDs)
		}
		if req.NegateArgs != nil {
			t.Errorf("expected negate_args nil, got %v", req.NegateArgs)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, DeleteWorksitesResponse{Successes: 1, Failures: 0, Skips: 0})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteWorksite(context.Background(), "ws-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_DeleteWorksite_SkippedIn200Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, DeleteWorksitesResponse{
			AssignedDetails:   "Reassign any assigned Agent, Asset, Rule, Installation Profile, Permission Scheme or Orchestration to another Worksite and try again.",
			AssignedWorksites: 1,
			Details:           "Worksites: ws-123 not deleted, these worksites are assigned",
			Failures:          0,
			Skips:             1,
			Successes:         0,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteWorksite(context.Background(), "ws-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete worksite") {
		t.Fatalf("expected delete error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "skips=1") {
		t.Fatalf("expected skips count in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not deleted") {
		t.Fatalf("expected API details in error, got: %v", err)
	}
}

func TestClient_DeleteWorksite_FailedIn200Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, DeleteWorksitesResponse{
			Details:   "Worksites: ws-123 not deleted",
			Failures:  1,
			Skips:     0,
			Successes: 0,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteWorksite(context.Background(), "ws-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failures=1") {
		t.Fatalf("expected failures count in error, got: %v", err)
	}
}

func TestClient_DeleteWorksite_200ResponseZeroSkipsAndFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, DeleteWorksitesResponse{Failures: 0, Skips: 0, Successes: 1})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteWorksite(context.Background(), "ws-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_DeleteWorksite_NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteWorksite(context.Background(), "ws-123")
	if err != nil {
		t.Fatalf("expected no error for 204 response, got %v", err)
	}
}

func TestClient_DeleteWorksite_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteWorksite(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}
}

func TestClient_DeleteWorksite_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteWorksite(context.Background(), "ws-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete worksite") {
		t.Errorf("expected 'failed to delete worksite' error, got: %v", err)
	}
}

func TestClient_DeleteWorksite_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"worksites feature is disabled"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteWorksite(context.Background(), "ws-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrWorksitesFeatureDisabled {
		t.Errorf("expected ErrWorksitesFeatureDisabled, got: %v", err)
	}
}

func TestClient_ListWorksites(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		gcFilter := r.URL.Query().Get("gc_filter")
		if gcFilter != "test-filter" {
			t.Errorf("expected gc_filter 'test-filter', got '%s'", gcFilter)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListWorksitesResponse{
			Objects: []Worksite{
				{ID: "ws-1", Name: "worksite-one"},
				{ID: "ws-2", Name: "worksite-two"},
			},
			TotalCount: 2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	worksites, err := client.ListWorksites(context.Background(), "test-filter")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(worksites) != 2 {
		t.Fatalf("expected 2 worksites, got %d", len(worksites))
	}
	if worksites[0].ID != "ws-1" {
		t.Errorf("expected first ID 'ws-1', got '%s'", worksites[0].ID)
	}
}

func TestClient_ListWorksites_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			worksites := make([]Worksite, defaultPageSize)
			for i := range worksites {
				worksites[i] = Worksite{ID: fmt.Sprintf("ws-%d", i), Name: fmt.Sprintf("worksite-%d", i)}
			}
			writeJSON(t, w, ListWorksitesResponse{
				Objects:    worksites,
				TotalCount: defaultPageSize + 1,
			})
		} else {
			writeJSON(t, w, ListWorksitesResponse{
				Objects:    []Worksite{{ID: "ws-last", Name: "worksite-last"}},
				TotalCount: defaultPageSize + 1,
			})
		}
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	worksites, err := client.ListWorksites(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(worksites) != defaultPageSize+1 {
		t.Errorf("expected %d worksites, got %d", defaultPageSize+1, len(worksites))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestClient_ListWorksites_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.ListWorksites(context.Background(), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list worksites") {
		t.Errorf("expected 'failed to list worksites' error, got: %v", err)
	}
}

func TestClient_ListWorksites_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"worksites feature is disabled"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.ListWorksites(context.Background(), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrWorksitesFeatureDisabled {
		t.Errorf("expected ErrWorksitesFeatureDisabled, got: %v", err)
	}
}

// UserGroup CRUD tests

func TestClient_CreateUserGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/user-groups" {
			t.Errorf("expected path '/api/v4.0/visibility/user-groups', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var ug UserGroupCreate
		if err := json.NewDecoder(r.Body).Decode(&ug); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if ug.Title != "Test Group" {
			t.Errorf("expected title 'Test Group', got '%s'", ug.Title)
		}
		if len(ug.OrchestrationsGroups) != 1 {
			t.Fatalf("expected 1 orchestration group, got %d", len(ug.OrchestrationsGroups))
		}
		if ug.OrchestrationsGroups[0].OrchestrationID != "orch-1" {
			t.Errorf("expected orchestration_id 'orch-1', got '%s'", ug.OrchestrationsGroups[0].OrchestrationID)
		}
		if len(ug.OrchestrationsGroups[0].Groups) != 2 {
			t.Errorf("expected 2 groups, got %d", len(ug.OrchestrationsGroups[0].Groups))
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, CreateResponse{ID: "ug-123"})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	id, err := client.CreateUserGroup(context.Background(), &UserGroupCreate{
		Title: "Test Group",
		OrchestrationsGroups: []OrchestrationGroup{
			{OrchestrationID: "orch-1", Groups: []string{"group-a", "group-b"}},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if id != "ug-123" {
		t.Errorf("expected ID 'ug-123', got '%s'", id)
	}
}

func TestClient_CreateUserGroup_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.CreateUserGroup(context.Background(), &UserGroupCreate{Title: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create user group") {
		t.Errorf("expected 'failed to create user group' error, got: %v", err)
	}
}

func TestClient_GetUserGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListUserGroupsResponse{
			Objects: []UserGroup{
				{ID: "ug-1", Title: "Group One", OrchestrationsGroups: []OrchestrationGroup{{OrchestrationID: "orch-1", Groups: []string{"g1"}}}},
				{ID: "ug-2", Title: "Group Two"},
			},
			TotalCount: 2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	ug, err := client.GetUserGroup(context.Background(), "ug-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ug == nil {
		t.Fatal("expected user group, got nil")
		return
	}
	if ug.ID != "ug-1" {
		t.Errorf("expected ID 'ug-1', got '%s'", ug.ID)
	}
	if ug.Title != "Group One" {
		t.Errorf("expected title 'Group One', got '%s'", ug.Title)
	}
	if len(ug.OrchestrationsGroups) != 1 {
		t.Errorf("expected 1 orchestration group, got %d", len(ug.OrchestrationsGroups))
	}
}

func TestClient_GetUserGroup_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListUserGroupsResponse{
			Objects:    []UserGroup{},
			TotalCount: 0,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	ug, err := client.GetUserGroup(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ug != nil {
		t.Errorf("expected nil, got %v", ug)
	}
}

func TestClient_UpdateUserGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/user-groups/ug-123" {
			t.Errorf("expected path '/api/v4.0/visibility/user-groups/ug-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected method PUT, got '%s'", r.Method)
		}

		var ug UserGroupCreate
		if err := json.NewDecoder(r.Body).Decode(&ug); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if ug.Title != "Updated Group" {
			t.Errorf("expected title 'Updated Group', got '%s'", ug.Title)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.UpdateUserGroup(context.Background(), "ug-123", &UserGroupCreate{
		Title:                "Updated Group",
		OrchestrationsGroups: []OrchestrationGroup{{OrchestrationID: "orch-1", Groups: []string{"g1"}}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_UpdateUserGroup_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.UpdateUserGroup(context.Background(), "ug-123", &UserGroupCreate{Title: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update user group") {
		t.Errorf("expected 'failed to update user group' error, got: %v", err)
	}
}

func TestClient_DeleteUserGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/user-groups/ug-123" {
			t.Errorf("expected path '/api/v4.0/visibility/user-groups/ug-123', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected method DELETE, got '%s'", r.Method)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteUserGroup(context.Background(), "ug-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_DeleteUserGroup_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteUserGroup(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}
}

func TestClient_DeleteUserGroup_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteUserGroup(context.Background(), "ug-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete user group") {
		t.Errorf("expected 'failed to delete user group' error, got: %v", err)
	}
}

func TestClient_ListUserGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got '%s'", r.Method)
		}

		search := r.URL.Query().Get("search")
		if search != "test-search" {
			t.Errorf("expected search 'test-search', got '%s'", search)
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, ListUserGroupsResponse{
			Objects: []UserGroup{
				{ID: "ug-1", Title: "Group One"},
				{ID: "ug-2", Title: "Group Two"},
			},
			TotalCount: 2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	groups, err := client.ListUserGroups(context.Background(), "test-search")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 user groups, got %d", len(groups))
	}
	if groups[0].ID != "ug-1" {
		t.Errorf("expected first ID 'ug-1', got '%s'", groups[0].ID)
	}
}

func TestClient_ListUserGroups_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			groups := make([]UserGroup, defaultPageSize)
			for i := range groups {
				groups[i] = UserGroup{ID: fmt.Sprintf("ug-%d", i), Title: fmt.Sprintf("Group %d", i)}
			}
			writeJSON(t, w, ListUserGroupsResponse{
				Objects:    groups,
				TotalCount: defaultPageSize + 1,
			})
		} else {
			writeJSON(t, w, ListUserGroupsResponse{
				Objects:    []UserGroup{{ID: "ug-last", Title: "Last Group"}},
				TotalCount: defaultPageSize + 1,
			})
		}
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	groups, err := client.ListUserGroups(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(groups) != defaultPageSize+1 {
		t.Errorf("expected %d user groups, got %d", defaultPageSize+1, len(groups))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestClient_ListUserGroups_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.ListUserGroups(context.Background(), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list user groups") {
		t.Errorf("expected 'failed to list user groups' error, got: %v", err)
	}
}

// CreatePolicyRevision error test

func TestClient_CreatePolicyRevision_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.CreatePolicyRevision(context.Background(), &PolicyRevisionRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create policy revision") {
		t.Errorf("expected 'failed to create policy revision' error, got: %v", err)
	}
}

func TestClient_CreatePolicyRevision_RevisionUnchanged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"description":"Revision hasn't been changed.","error_code":"BAD_REQUEST"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.CreatePolicyRevision(context.Background(), &PolicyRevisionRequest{})
	if !errors.Is(err, ErrPolicyRevisionUnchanged) {
		t.Fatalf("expected ErrPolicyRevisionUnchanged, got: %v", err)
	}
}

func TestIsPolicyRevisionUnchanged(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{"matching", http.StatusBadRequest, `{"description":"Revision hasn't been changed.","error_code":"BAD_REQUEST"}`, true},
		{"wrong status", http.StatusInternalServerError, `{"description":"Revision hasn't been changed."}`, false},
		{"different error", http.StatusBadRequest, `{"description":"Something else went wrong."}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPolicyRevisionUnchanged(tt.statusCode, []byte(tt.body))
			if got != tt.want {
				t.Errorf("isPolicyRevisionUnchanged(%d, %q) = %v, want %v", tt.statusCode, tt.body, got, tt.want)
			}
		})
	}
}

// Error path tests for existing functions

func TestClient_CreateLabel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.CreateLabel(context.Background(), &LabelCreate{Key: "env", Value: "prod"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create label") {
		t.Errorf("expected 'failed to create label' error, got: %v", err)
	}
}

func TestClient_GetLabel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.GetLabel(context.Background(), "label-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get label") {
		t.Errorf("expected 'failed to get label' error, got: %v", err)
	}
}

func TestClient_UpdateLabel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.UpdateLabel(context.Background(), "label-123", &LabelUpdate{Key: "env", Value: "prod"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update label") {
		t.Errorf("expected 'failed to update label' error, got: %v", err)
	}
}

func TestClient_DeleteLabel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteLabel(context.Background(), "label-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete label") {
		t.Errorf("expected 'failed to delete label' error, got: %v", err)
	}
}

func TestClient_ListLabels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.ListLabels(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list labels") {
		t.Errorf("expected 'failed to list labels' error, got: %v", err)
	}
}

func TestClient_ListLabels_PaginationSafetyLimit(t *testing.T) {
	origLimit := maxPaginationPages
	maxPaginationPages = 3
	defer func() { maxPaginationPages = origLimit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		labels := make([]Label, defaultPageSize)
		for i := range labels {
			labels[i] = Label{ID: fmt.Sprintf("lbl-%d", i), Key: "k", Value: "v"}
		}
		writeJSON(t, w, ListLabelsResponse{
			Objects:    labels,
			TotalCount: 999999,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.ListLabels(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected pagination safety limit error, got nil")
	}
	if !strings.Contains(err.Error(), "pagination safety limit reached") {
		t.Errorf("expected 'pagination safety limit reached' error, got: %v", err)
	}
}

func TestClient_CreateLabelGroup_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.CreateLabelGroup(context.Background(), &LabelGroupCreate{Key: "env", Value: "prod"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create label group") {
		t.Errorf("expected 'failed to create label group' error, got: %v", err)
	}
}

func TestClient_PublishLabelGroups_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.PublishLabelGroups(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to publish label groups") {
		t.Errorf("expected 'failed to publish label groups' error, got: %v", err)
	}
}

func TestClient_CreatePolicyRule_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.CreatePolicyRule(context.Background(), map[string]interface{}{"action": "ALLOW"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create policy rule") {
		t.Errorf("expected 'failed to create policy rule' error, got: %v", err)
	}
}

func TestClient_BulkCreatePolicyRules_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.BulkCreatePolicyRules(context.Background(), []map[string]any{{"action": "ALLOW"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to bulk create policy rules") {
		t.Errorf("expected 'failed to bulk create policy rules' error, got: %v", err)
	}
}

func TestClient_BulkUpdatePolicyRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/policy/rules/bulk" {
			t.Errorf("expected path '/api/v4.0/visibility/policy/rules/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected method PUT, got '%s'", r.Method)
		}

		var body []PolicyRuleBulkUpdateItem
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if len(body) != 2 {
			t.Fatalf("expected 2 items in request body, got %d", len(body))
		}

		writeJSON(t, w, PolicyRulesBulkCreateResponse{
			NumberOfFailed:    0,
			NumberOfSucceeded: 2,
			Result:            "success",
			Succeeded:         []string{body[0].ID, body[1].ID},
			TotalNumber:       2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	items := []PolicyRuleBulkUpdateItem{
		{ID: "rule-1", Rule: map[string]any{"action": "ALLOW"}},
		{ID: "rule-2", Rule: map[string]any{"action": "BLOCK"}},
	}

	resp, err := client.BulkUpdatePolicyRules(context.Background(), items)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.NumberOfSucceeded != 2 {
		t.Fatalf("expected 2 succeeded, got %d", resp.NumberOfSucceeded)
	}
}

func TestClient_BulkUpdatePolicyRules_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.BulkUpdatePolicyRules(context.Background(), []PolicyRuleBulkUpdateItem{{ID: "rule-1", Rule: map[string]any{"action": "ALLOW"}}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to bulk update policy rules") {
		t.Errorf("expected 'failed to bulk update policy rules' error, got: %v", err)
	}
}

func TestClient_BulkDeletePolicyRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/visibility/policy/rules/delete/bulk" {
			t.Errorf("expected path '/api/v4.0/visibility/policy/rules/delete/bulk', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got '%s'", r.Method)
		}

		var body []PolicyRuleBulkDeleteItem
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if len(body) != 2 {
			t.Fatalf("expected 2 items in request body, got %d", len(body))
		}

		writeJSON(t, w, PolicyRulesBulkCreateResponse{
			NumberOfFailed:    0,
			NumberOfSucceeded: 2,
			Result:            "success",
			Succeeded:         []string{body[0].ID, body[1].ID},
			TotalNumber:       2,
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	items := []PolicyRuleBulkDeleteItem{
		{ID: "rule-1"},
		{ID: "rule-2"},
	}

	resp, err := client.BulkDeletePolicyRules(context.Background(), items)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.NumberOfSucceeded != 2 {
		t.Fatalf("expected 2 succeeded, got %d", resp.NumberOfSucceeded)
	}
}

func TestClient_BulkDeletePolicyRules_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.BulkDeletePolicyRules(context.Background(), []PolicyRuleBulkDeleteItem{{ID: "rule-1"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to bulk delete policy rules") {
		t.Errorf("expected 'failed to bulk delete policy rules' error, got: %v", err)
	}
}

func TestClient_DeletePolicyRule_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeletePolicyRule(context.Background(), "rule-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete policy rule") {
		t.Errorf("expected 'failed to delete policy rule' error, got: %v", err)
	}
}

func TestClient_GetLabelGroup_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.GetLabelGroup(context.Background(), "group-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get label group") {
		t.Errorf("expected 'failed to get label group' error, got: %v", err)
	}
}

func TestClient_GetLabelGroup_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	lg, err := client.GetLabelGroup(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if lg != nil {
		t.Errorf("expected nil, got %v", lg)
	}
}

func TestClient_GetLabelGroup_DirectObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a direct label group object (not wrapped in objects array)
		writeJSON(t, w, map[string]interface{}{
			"id":    "group-1",
			"key":   "Role",
			"value": "Web",
		})
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	lg, err := client.GetLabelGroup(context.Background(), "group-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if lg == nil {
		t.Fatal("expected label group, got nil")
		return
	}
	if lg.ID != "group-1" {
		t.Errorf("expected ID 'group-1', got '%s'", lg.ID)
	}
}

func TestClient_UpdateDnsBlocklist_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.UpdateDnsBlocklist(context.Background(), "dns-123", &DnsBlocklistEdit{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update DNS blocklist") {
		t.Errorf("expected 'failed to update DNS blocklist' error, got: %v", err)
	}
}

func TestClient_DeleteDnsBlocklist_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteDnsBlocklist(context.Background(), "dns-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete DNS blocklist") {
		t.Errorf("expected 'failed to delete DNS blocklist' error, got: %v", err)
	}
}

func TestClient_ResetDnsBlocklistHitCount_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.ResetDnsBlocklistHitCount(context.Background(), "dns-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to reset DNS blocklist hit count") {
		t.Errorf("expected 'failed to reset DNS blocklist hit count' error, got: %v", err)
	}
}

func TestClient_CreateIncident_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.CreateIncident(context.Background(), &IncidentCreate{Type: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create incident") {
		t.Errorf("expected 'failed to create incident' error, got: %v", err)
	}
}

func TestClient_BulkCreateIncidents_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.BulkCreateIncidents(context.Background(), []IncidentCreate{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to bulk create incidents") {
		t.Errorf("expected 'failed to bulk create incidents' error, got: %v", err)
	}
}

func TestClient_BulkCreateDnsBlocklists_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.BulkCreateDnsBlocklists(context.Background(), []DnsBlocklistCreate{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to bulk create DNS blocklists") {
		t.Errorf("expected 'failed to bulk create DNS blocklists' error, got: %v", err)
	}
}

func TestClient_CreateDnsBlocklist_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.CreateDnsBlocklist(context.Background(), &DnsBlocklistCreate{Name: "test", Type: "CUSTOM_BLOCK"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create DNS blocklist") {
		t.Errorf("expected 'failed to create DNS blocklist' error, got: %v", err)
	}
}

func TestClient_CreateDnsBlocklist_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"Could not complete the operation due to a server error. See error for more details","error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.CreateDnsBlocklist(context.Background(), &DnsBlocklistCreate{Name: "test", Type: "CUSTOM_BLOCK"})
	if err != ErrDnsSecurityFeatureDisabled {
		t.Errorf("expected ErrDnsSecurityFeatureDisabled, got: %v", err)
	}
}

func TestClient_GetDnsBlocklist_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"Could not complete the operation due to a server error. See error for more details","error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.GetDnsBlocklist(context.Background(), "dns-123")
	if err != ErrDnsSecurityFeatureDisabled {
		t.Errorf("expected ErrDnsSecurityFeatureDisabled, got: %v", err)
	}
}

func TestClient_UpdateDnsBlocklist_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"Could not complete the operation due to a server error. See error for more details","error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.UpdateDnsBlocklist(context.Background(), "dns-123", &DnsBlocklistEdit{})
	if err != ErrDnsSecurityFeatureDisabled {
		t.Errorf("expected ErrDnsSecurityFeatureDisabled, got: %v", err)
	}
}

func TestClient_DeleteDnsBlocklist_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"Could not complete the operation due to a server error. See error for more details","error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.DeleteDnsBlocklist(context.Background(), "dns-123")
	if err != ErrDnsSecurityFeatureDisabled {
		t.Errorf("expected ErrDnsSecurityFeatureDisabled, got: %v", err)
	}
}

func TestClient_ListDnsBlocklists_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"Could not complete the operation due to a server error. See error for more details","error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.ListDnsBlocklists(context.Background(), "", "")
	if err != ErrDnsSecurityFeatureDisabled {
		t.Errorf("expected ErrDnsSecurityFeatureDisabled, got: %v", err)
	}
}

func TestClient_BulkCreateDnsBlocklists_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"Could not complete the operation due to a server error. See error for more details","error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := client.BulkCreateDnsBlocklists(context.Background(), []DnsBlocklistCreate{{Name: "test", Type: "CUSTOM_BLOCK"}})
	if err != ErrDnsSecurityFeatureDisabled {
		t.Errorf("expected ErrDnsSecurityFeatureDisabled, got: %v", err)
	}
}

func TestClient_BulkDeleteDnsBlocklists_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"Could not complete the operation due to a server error. See error for more details","error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.BulkDeleteDnsBlocklists(context.Background(), []string{"id-1"})
	if err != ErrDnsSecurityFeatureDisabled {
		t.Errorf("expected ErrDnsSecurityFeatureDisabled, got: %v", err)
	}
}

func TestClient_BulkEditDnsBlocklists_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"Could not complete the operation due to a server error. See error for more details","error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.BulkEditDnsBlocklists(context.Background(), &BulkEditDnsBlocklistRequest{Items: []BulkEditDnsBlocklistItem{{ID: "id-1"}}})
	if err != ErrDnsSecurityFeatureDisabled {
		t.Errorf("expected ErrDnsSecurityFeatureDisabled, got: %v", err)
	}
}

func TestClient_ResetDnsBlocklistHitCount_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"Could not complete the operation due to a server error. See error for more details","error_code":"OperationFailed","error_dump":"('%s is not enabled', 'DNS Security')"}`))
	}))
	defer server.Close()

	client := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := client.ResetDnsBlocklistHitCount(context.Background(), "id-1")
	if err != ErrDnsSecurityFeatureDisabled {
		t.Errorf("expected ErrDnsSecurityFeatureDisabled, got: %v", err)
	}
}

// Worksite assignment tests

func TestClient_AssignWorksite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v4.0/worksites/assign" {
			t.Errorf("expected path '/api/v4.0/worksites/assign', got '%s'", r.URL.Path)
		}

		var req WorksiteAssignRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.ID != "ws-1" {
			t.Errorf("expected worksite ID 'ws-1', got '%s'", req.ID)
		}
		if req.EntityType != "asset" {
			t.Errorf("expected entity type 'asset', got '%s'", req.EntityType)
		}
		if len(req.EntityIDs) != 2 || req.EntityIDs[0] != "a-1" || req.EntityIDs[1] != "a-2" {
			t.Errorf("unexpected entity IDs: %v", req.EntityIDs)
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"ws-1","name":"HQ"}`)
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := c.AssignWorksite(context.Background(), &WorksiteAssignRequest{
		ID:         "ws-1",
		EntityType: "asset",
		EntityIDs:  []string{"a-1", "a-2"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_AssignWorksite_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := c.AssignWorksite(context.Background(), &WorksiteAssignRequest{
		ID:         "ws-1",
		EntityType: "asset",
		EntityIDs:  []string{"a-1"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to assign worksite") {
		t.Errorf("expected 'failed to assign worksite' error, got: %v", err)
	}
}

func TestClient_AssignWorksite_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"worksites feature is disabled"}`)
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := c.AssignWorksite(context.Background(), &WorksiteAssignRequest{
		ID:         "ws-1",
		EntityType: "asset",
		EntityIDs:  []string{"a-1"},
	})
	if err != ErrWorksitesFeatureDisabled {
		t.Errorf("expected ErrWorksitesFeatureDisabled, got: %v", err)
	}
}

func TestClient_MovePolicyRulesToWorksite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		expectedPath := "/api/v3.0/visibility/policy/rules-bulk/worksite/move/ws-1"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path '%s', got '%s'", expectedPath, r.URL.Path)
		}

		var req PolicyRuleBulkWorksiteMoveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(req.IDs) != 2 || req.IDs[0] != "r-1" || req.IDs[1] != "r-2" {
			t.Errorf("unexpected rule IDs: %v", req.IDs)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"inserted_count":0,"modified_count":2,"worksite":{"id":"ws-1","name":"HQ"}}`)
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := c.MovePolicyRulesToWorksite(context.Background(), "ws-1", []string{"r-1", "r-2"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_MovePolicyRulesToWorksite_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := c.MovePolicyRulesToWorksite(context.Background(), "ws-1", []string{"r-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to move policy rules to worksite") {
		t.Errorf("expected 'failed to move policy rules to worksite' error, got: %v", err)
	}
}

func TestClient_MovePolicyRulesToWorksite_FeatureDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"worksites feature is disabled"}`)
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := c.MovePolicyRulesToWorksite(context.Background(), "ws-1", []string{"r-1"})
	if err != ErrWorksitesFeatureDisabled {
		t.Errorf("expected ErrWorksitesFeatureDisabled, got: %v", err)
	}
}

func TestClient_MovePolicyRulesToWorksite_AllWorksites(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v3.0/visibility/policy/rules-bulk/worksite/move/all_worksites"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path '%s', got '%s'", expectedPath, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"inserted_count":0,"modified_count":1,"worksite":{"id":"","name":"All Worksites"}}`)
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	err := c.MovePolicyRulesToWorksite(context.Background(), "all_worksites", []string{"r-1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_AuthenticateIfStale_EmptyStaleToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3.0/authenticate" {
			t.Errorf("expected path '/api/v3.0/authenticate', got '%s'", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, AuthResponse{AccessToken: testJWTNew})
	}))
	defer server.Close()

	c := &Client{
		config: Config{
			BaseURL:  server.URL,
			Username: "user",
			Password: "pass",
		},
		httpClient: http.DefaultClient,
		token:      testJWTOld,
	}

	if err := c.authenticateIfStale(context.Background(), ""); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if c.token != testJWTNew {
		t.Errorf("expected token %q, got '%s'", testJWTNew, c.token)
	}
}

func TestClient_AuthenticateIfStale_MatchingStaleToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3.0/authenticate" {
			t.Errorf("expected path '/api/v3.0/authenticate', got '%s'", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, AuthResponse{AccessToken: testJWTNew})
	}))
	defer server.Close()

	c := &Client{
		config: Config{
			BaseURL:  server.URL,
			Username: "user",
			Password: "pass",
		},
		httpClient: http.DefaultClient,
		token:      "stale-token",
	}

	if err := c.authenticateIfStale(context.Background(), "stale-token"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if c.token != testJWTNew {
		t.Errorf("expected token 'new-token', got '%s'", c.token)
	}
}

func TestClient_AuthenticateIfStale_NonMatchingStaleToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("authenticate should not have been called — token was already refreshed")
	}))
	defer server.Close()

	c := &Client{
		config: Config{
			BaseURL:  server.URL,
			Username: "user",
			Password: "pass",
		},
		httpClient: http.DefaultClient,
		token:      "already-refreshed-token",
	}

	if err := c.authenticateIfStale(context.Background(), "old-stale-token"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if c.token != "already-refreshed-token" {
		t.Errorf("expected token 'already-refreshed-token', got '%s'", c.token)
	}
}

func TestClient_AuthenticateIfStale_RefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3.0/authenticate/refresh" {
			t.Errorf("expected path '/api/v3.0/authenticate/refresh', got '%s'", r.URL.Path)
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer my-refresh-token" {
			t.Errorf("expected Authorization 'Bearer my-refresh-token', got '%s'", authHeader)
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, AuthResponse{AccessToken: testJWTRefreshed})
	}))
	defer server.Close()

	c := &Client{
		config: Config{
			BaseURL:      server.URL,
			RefreshToken: "my-refresh-token",
		},
		httpClient: http.DefaultClient,
		token:      "stale",
	}

	if err := c.authenticateIfStale(context.Background(), "stale"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if c.token != testJWTRefreshed {
		t.Errorf("expected token 'refreshed-token', got '%s'", c.token)
	}
}

func TestClient_AuthenticateIfStale_NoAuthMethod(t *testing.T) {
	c := &Client{
		config:     Config{BaseURL: "https://guardicore.example.com"},
		httpClient: http.DefaultClient,
		token:      "stale",
	}

	err := c.authenticateIfStale(context.Background(), "stale")
	if err == nil {
		t.Fatal("expected error when no auth method configured")
	}

	if !strings.Contains(err.Error(), "no valid authentication method configured") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAuthenticateWithRetry_SucceedsAfterTransientFailure(t *testing.T) {
	origDelay := authRetryBaseDelay
	authRetryBaseDelay = time.Millisecond
	defer func() { authRetryBaseDelay = origDelay }()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 1 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"message":"Internal Server Error"}`)
			return
		}
		writeJSON(t, w, AuthResponse{AccessToken: testJWTGenerated})
	}))
	defer server.Close()

	c := &Client{
		config: Config{
			BaseURL:  server.URL,
			Username: "admin",
			Password: "password",
		},
		httpClient: server.Client(),
	}

	err := c.authenticateWithRetry(context.Background(), "")
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if c.token != testJWTGenerated {
		t.Errorf("expected token %q, got %q", testJWTGenerated, c.token)
	}
	if atomic.LoadInt32(&attempts) < 2 {
		t.Errorf("expected at least 2 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestAuthenticateWithRetry_ExhaustsRetries(t *testing.T) {
	origDelay := authRetryBaseDelay
	authRetryBaseDelay = time.Millisecond
	defer func() { authRetryBaseDelay = origDelay }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"Internal Server Error"}`)
	}))
	defer server.Close()

	c := &Client{
		config: Config{
			BaseURL:  server.URL,
			Username: "admin",
			Password: "password",
		},
		httpClient: server.Client(),
	}

	err := c.authenticateWithRetry(context.Background(), "")
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "authentication failed after") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuthenticateWithRetry_NoRetryOnBadCredentials(t *testing.T) {
	origDelay := authRetryBaseDelay
	authRetryBaseDelay = time.Millisecond
	defer func() { authRetryBaseDelay = origDelay }()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, AuthResponse{AccessToken: ""})
	}))
	defer server.Close()

	c := &Client{
		config: Config{
			BaseURL:  server.URL,
			Username: "admin",
			Password: "wrong",
		},
		httpClient: server.Client(),
	}

	err := c.authenticateWithRetry(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected exactly 1 attempt (no retry for non-transient error), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestIsTransientAuthError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{"nil", nil, false},
		{"eof", fmt.Errorf("failed to authenticate: Post \"https://example.com\": EOF"), true},
		{"tls error", fmt.Errorf("tls: server did not echo the legacy session ID"), true},
		{"connection reset", fmt.Errorf("connection reset by peer"), true},
		{"timeout", fmt.Errorf("request timeout"), true},
		{"internal server error", fmt.Errorf("password authentication failed: internal server error"), true},
		{"empty access token", fmt.Errorf("password authentication succeeded but returned empty access token"), false},
		{"invalid jwt", fmt.Errorf("does not appear to be a valid jwt"), false},
		{"mfa", fmt.Errorf("multi-factor authentication required"), false},
		{"no auth method", fmt.Errorf("no valid authentication method configured"), false},
		{"refresh expired", fmt.Errorf("refresh token may be expired or invalid"), false},
		{"generic error", fmt.Errorf("some unknown error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientAuthError(tt.err)
			if got != tt.transient {
				t.Errorf("isTransientAuthError(%v) = %v, want %v", tt.err, got, tt.transient)
			}
		})
	}
}

func TestClient_ConcurrentRetryOn401(t *testing.T) {
	var authCallCount int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3.0/authenticate" {
			atomic.AddInt64(&authCallCount, 1)
			w.Header().Set("Content-Type", "application/json")
			writeJSON(t, w, AuthResponse{AccessToken: testJWTNew})
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "Bearer "+testJWTOld {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, LabelGetResponse{
			Objects: []Label{{ID: "label-1", Key: "K", Value: "V"}},
		})
	}))
	defer server.Close()

	c := &Client{
		config: Config{
			BaseURL:  server.URL,
			Username: "user",
			Password: "pass",
		},
		httpClient: http.DefaultClient,
		token:      testJWTOld,
	}

	const goroutineCount = 10
	var wg sync.WaitGroup
	wg.Add(goroutineCount)
	errs := make([]error, goroutineCount)
	results := make([]*Label, goroutineCount)

	for i := 0; i < goroutineCount; i++ {
		go func(idx int) {
			defer wg.Done()
			label, err := c.GetLabel(context.Background(), "label-1")
			errs[idx] = err
			results[idx] = label
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}
	}
	for i, label := range results {
		if label == nil {
			t.Errorf("goroutine %d returned nil label", i)
		}
	}

	// With the stale-token check, only 1-2 auth calls should occur.
	// Without the fix, all goroutines would re-authenticate (~10 calls).
	count := atomic.LoadInt64(&authCallCount)
	if count < 1 {
		t.Error("expected at least 1 auth call")
	}
	if count > 2 {
		t.Errorf("expected at most 2 auth calls (stale-token dedup), got %d", count)
	}

	if c.token != testJWTNew {
		t.Errorf("expected token 'new-token', got '%s'", c.token)
	}
}

func TestClient_RetryOn429_WithRetryAfter(t *testing.T) {
	oldRetries := maxRequestRetries
	maxRequestRetries = 1
	t.Cleanup(func() {
		maxRequestRetries = oldRetries
	})

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4.0/labels/label-429" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		current := atomic.AddInt32(&callCount, 1)
		if current == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"description":"rate limited"}`))
			return
		}

		writeJSON(t, w, LabelGetResponse{Objects: []Label{{ID: "label-429", Key: "K", Value: "V"}}})
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	label, err := c.GetLabel(ctx, "label-429")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if label == nil || label.ID != "label-429" {
		t.Fatalf("expected label label-429, got %#v", label)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

func TestClient_RetryOn5xx_TransientSuccess(t *testing.T) {
	oldRetries := maxRequestRetries
	oldBaseDelay := retryBaseDelay
	oldMaxDelay := retryMaxDelay
	maxRequestRetries = 3
	retryBaseDelay = time.Millisecond
	retryMaxDelay = 5 * time.Millisecond
	t.Cleanup(func() {
		maxRequestRetries = oldRetries
		retryBaseDelay = oldBaseDelay
		retryMaxDelay = oldMaxDelay
	})

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&callCount, 1)
		switch current {
		case 1:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server error"))
		case 2:
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("gateway error"))
		default:
			writeJSON(t, w, LabelGetResponse{Objects: []Label{{ID: "label-5xx", Key: "K", Value: "V"}}})
		}
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	label, err := c.GetLabel(context.Background(), "label-5xx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if label == nil || label.ID != "label-5xx" {
		t.Fatalf("expected label label-5xx, got %#v", label)
	}
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Fatalf("expected 3 calls, got %d", got)
	}
}

func TestClient_RetryOn429_StopsAfterMaxRetries(t *testing.T) {
	oldRetries := maxRequestRetries
	oldBaseDelay := retryBaseDelay
	oldMaxDelay := retryMaxDelay
	maxRequestRetries = 2
	retryBaseDelay = time.Millisecond
	retryMaxDelay = 5 * time.Millisecond
	t.Cleanup(func() {
		maxRequestRetries = oldRetries
		retryBaseDelay = oldBaseDelay
		retryMaxDelay = oldMaxDelay
	})

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"description":"rate limited"}`))
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := c.GetLabel(context.Background(), "label-429-loop")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get label: status 429") {
		t.Fatalf("expected status 429 error, got %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Fatalf("expected 3 calls (initial + 2 retries), got %d", got)
	}
}

func TestClient_NoRetryOn403(t *testing.T) {
	oldRetries := maxRequestRetries
	maxRequestRetries = 3
	t.Cleanup(func() {
		maxRequestRetries = oldRetries
	})

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"forbidden"}`))
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := c.GetLabel(context.Background(), "label-403")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get label: status 403") {
		t.Fatalf("expected status 403 error, got %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

func TestClient_RequestTimeout_ClassifiedCorrectly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(120 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, LabelGetResponse{Objects: []Label{{ID: "slow"}}})
	}))
	defer server.Close()

	c := &Client{
		config: Config{BaseURL: server.URL},
		httpClient: &http.Client{
			Timeout: 20 * time.Millisecond,
		},
		token: "test-token",
	}

	_, err := c.GetLabel(context.Background(), "slow")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "request GET /api/v4.0/labels/slow failed") {
		t.Fatalf("expected contextual request failure, got %v", err)
	}
}

func TestClient_RefreshTokenExpired_ReturnsActionableError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3.0/authenticate/refresh" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"description":"refresh expired"}`))
	}))
	defer server.Close()

	_, err := NewClient(Config{BaseURL: server.URL, RefreshToken: "expired"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "refresh token may be expired or invalid") {
		t.Fatalf("expected actionable refresh token message, got %v", err)
	}
	if !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("expected access_token guidance, got %v", err)
	}
}

func TestClient_MissingCredentials_FailsFastWithoutRequest(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := NewClient(Config{BaseURL: server.URL})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no valid authentication method configured") {
		t.Fatalf("expected missing auth error, got %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 0 {
		t.Fatalf("expected 0 requests, got %d", got)
	}
}

func TestClient_ForbiddenOnResourceOperation_NotRetried(t *testing.T) {
	oldRetries := maxRequestRetries
	maxRequestRetries = 3
	t.Cleanup(func() {
		maxRequestRetries = oldRetries
	})

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"forbidden"}`))
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := c.BulkUpdateAssets(context.Background(), []AssetBulkUpdateItem{{AssetID: "asset-1", Comments: "updated"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to bulk update assets: status 403") {
		t.Fatalf("expected status 403 error, got %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected 1 request, got %d", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	if got, ok := parseRetryAfter("2"); !ok || got != 2*time.Second {
		t.Fatalf("expected 2s, got %v ok=%v", got, ok)
	}

	if _, ok := parseRetryAfter("0"); ok {
		t.Fatal("expected zero retry-after to be ignored")
	}

	if _, ok := parseRetryAfter("not-a-time"); ok {
		t.Fatal("expected invalid retry-after to be ignored")
	}
}

func TestParseAPIError_ValidJSON(t *testing.T) {
	body := []byte(`{"description":"Illegal value provided for 'Label name'","error_code":"IllegalValue","error_dump":"Label name already in use"}`)
	apiErr := parseAPIError(400, body)

	if apiErr.StatusCode != 400 {
		t.Errorf("expected StatusCode 400, got %d", apiErr.StatusCode)
	}
	if apiErr.ErrorCode != "IllegalValue" {
		t.Errorf("expected ErrorCode 'IllegalValue', got %q", apiErr.ErrorCode)
	}
	if apiErr.ErrorDump != "Label name already in use" {
		t.Errorf("expected ErrorDump 'Label name already in use', got %q", apiErr.ErrorDump)
	}
	if apiErr.Description != "Illegal value provided for 'Label name'" {
		t.Errorf("expected Description to match, got %q", apiErr.Description)
	}
}

func TestParseAPIError_InvalidJSON(t *testing.T) {
	body := []byte(`not valid json`)
	apiErr := parseAPIError(500, body)

	if apiErr.StatusCode != 500 {
		t.Errorf("expected StatusCode 500, got %d", apiErr.StatusCode)
	}
	if apiErr.Description != "not valid json" {
		t.Errorf("expected Description 'not valid json', got %q", apiErr.Description)
	}
	if apiErr.ErrorCode != "" {
		t.Errorf("expected empty ErrorCode, got %q", apiErr.ErrorCode)
	}
}

func TestAPIError_IsAlreadyExists(t *testing.T) {
	tests := []struct {
		name     string
		err      *APIError
		expected bool
	}{
		{
			name: "duplicate label",
			err: &APIError{
				StatusCode:  400,
				ErrorCode:   "IllegalValue",
				ErrorDump:   "Label name already in use",
				Description: "Illegal value provided for 'Label name'",
			},
			expected: true,
		},
		{
			name: "different error code",
			err: &APIError{
				StatusCode:  400,
				ErrorCode:   "ValidationError",
				ErrorDump:   "something already in use",
				Description: "Validation failed",
			},
			expected: false,
		},
		{
			name: "wrong status code",
			err: &APIError{
				StatusCode: 500,
				ErrorCode:  "IllegalValue",
				ErrorDump:  "Label name already in use",
			},
			expected: false,
		},
		{
			name: "no already in use",
			err: &APIError{
				StatusCode: 400,
				ErrorCode:  "IllegalValue",
				ErrorDump:  "some other error",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsAlreadyExists(); got != tt.expected {
				t.Errorf("IsAlreadyExists() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClient_CreateLabel_DuplicateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"description":"Illegal value provided for 'Label name'","error_code":"IllegalValue","error_dump":"Label name already in use"}`)
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := c.CreateLabel(context.Background(), &LabelCreate{Key: "env", Value: "prod"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected error to unwrap to *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsAlreadyExists() {
		t.Errorf("expected IsAlreadyExists() to be true, got false; error: %v", apiErr)
	}
	if !strings.Contains(err.Error(), "failed to create label") {
		t.Errorf("expected error to contain 'failed to create label', got: %v", err)
	}
}

func TestClient_UpdateLabel_DuplicateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"description":"Illegal value provided for 'Label name'","error_code":"IllegalValue","error_dump":"Label name already in use"}`)
	}))
	defer server.Close()

	c := &Client{
		config:     Config{BaseURL: server.URL},
		httpClient: http.DefaultClient,
		token:      "test-token",
	}

	_, err := c.UpdateLabel(context.Background(), "label-123", &LabelUpdate{Key: "env", Value: "prod"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected error to unwrap to *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsAlreadyExists() {
		t.Errorf("expected IsAlreadyExists() to be true, got false; error: %v", apiErr)
	}
	if !strings.Contains(err.Error(), "failed to update label") {
		t.Errorf("expected error to contain 'failed to update label', got: %v", err)
	}
}

func TestPasswordAuth_ErrorIncludesBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(t, w, map[string]string{"description": "Invalid credentials"})
	}))
	defer server.Close()

	_, err := NewClient(Config{
		BaseURL:  server.URL,
		Username: "user",
		Password: "wrong",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Invalid credentials") {
		t.Errorf("expected error to contain API body, got: %v", err)
	}
}

func TestPasswordAuth_MFAHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"description":"The Authorization header is malformed. Expected format: \"Authorization: Bearer <JWT>\""}`)
	}))
	defer server.Close()

	_, err := NewClient(Config{
		BaseURL:  server.URL,
		Username: "mfa-user",
		Password: "pass",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "MFA") {
		t.Errorf("expected MFA hint in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "access_token") {
		t.Errorf("expected access_token suggestion in error, got: %v", err)
	}
}

func TestPasswordAuth_NonMFAError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error")
	}))
	defer server.Close()

	_, err := NewClient(Config{
		BaseURL:  server.URL,
		Username: "user",
		Password: "pass",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("expected body in error, got: %v", err)
	}
	if strings.Contains(err.Error(), "MFA") {
		t.Errorf("expected no MFA hint for 500 error, got: %v", err)
	}
}

func TestPasswordAuth_EmptyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, AuthResponse{AccessToken: ""})
	}))
	defer server.Close()

	_, err := NewClient(Config{
		BaseURL:  server.URL,
		Username: "user",
		Password: "pass",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "empty access token") {
		t.Errorf("expected empty token message, got: %v", err)
	}
}

func TestPasswordAuth_InvalidJWTToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, AuthResponse{AccessToken: "not-a-jwt"})
	}))
	defer server.Close()

	_, err := NewClient(Config{
		BaseURL:  server.URL,
		Username: "user",
		Password: "pass",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "valid JWT") {
		t.Errorf("expected JWT validation message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "access_token") {
		t.Errorf("expected access_token suggestion, got: %v", err)
	}
}

func TestRefreshAuth_ErrorIncludesBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(t, w, map[string]string{"description": "Token expired"})
	}))
	defer server.Close()

	_, err := NewClient(Config{
		BaseURL:      server.URL,
		RefreshToken: "expired-token",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Token expired") {
		t.Errorf("expected API body in error, got: %v", err)
	}
}

func TestRefreshAuth_EmptyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, AuthResponse{AccessToken: ""})
	}))
	defer server.Close()

	_, err := NewClient(Config{
		BaseURL:      server.URL,
		RefreshToken: "some-token",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "empty access token") {
		t.Errorf("expected empty token message, got: %v", err)
	}
}

func TestLooksLikeMFAError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{"MFA keyword", 400, `{"description":"MFA required"}`, true},
		{"multi-factor keyword", 400, `{"description":"multi-factor auth needed"}`, true},
		{"two-factor keyword", 403, `{"description":"two-factor verification required"}`, true},
		{"OTP keyword", 400, `{"description":"OTP verification pending"}`, true},
		{"malformed JWT", 400, `{"description":"The Authorization header is malformed"}`, true},
		{"additional verification", 403, `{"description":"additional verification required"}`, true},
		{"wrong status code", 500, `{"description":"MFA required"}`, false},
		{"generic error", 400, `{"description":"Invalid credentials"}`, false},
		{"empty body", 400, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeMFAError(tt.statusCode, []byte(tt.body))
			if got != tt.want {
				t.Errorf("looksLikeMFAError(%d, %q) = %v, want %v", tt.statusCode, tt.body, got, tt.want)
			}
		})
	}
}

func TestLooksLikeJWT(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"valid JWT structure", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U", true},
		{"three parts", "a.b.c", true},
		{"empty string", "", false},
		{"single segment", "not-a-jwt", false},
		{"two segments", "header.payload", false},
		{"four segments", "a.b.c.d", false},
		{"empty first part", ".b.c", false},
		{"empty middle part", "a..c", false},
		{"empty last part", "a.b.", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeJWT(tt.token)
			if got != tt.want {
				t.Errorf("looksLikeJWT(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestFlattenValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		input    any
		expected []string
	}{
		{
			name:     "simple string array leaf",
			prefix:   "field",
			input:    []any{"error message"},
			expected: []string{"field: error message"},
		},
		{
			name:   "nested object with numeric keys",
			prefix: "",
			input: map[string]any{
				"criteria": map[string]any{
					"1": map[string]any{
						"compound_criteria": []any{"Unknown field"},
						"op":                []any{"Must be one of EQUALS, STARTSWITH"},
					},
				},
			},
			expected: []string{
				"criteria[1].compound_criteria: Unknown field",
				"criteria[1].op: Must be one of EQUALS, STARTSWITH",
			},
		},
		{
			name:   "multiple numeric indices",
			prefix: "",
			input: map[string]any{
				"criteria": map[string]any{
					"0": map[string]any{
						"field": []any{"Required"},
					},
					"1": map[string]any{
						"op": []any{"Invalid value"},
					},
				},
			},
			expected: []string{
				"criteria[0].field: Required",
				"criteria[1].op: Invalid value",
			},
		},
		{
			name:     "nil input",
			prefix:   "",
			input:    nil,
			expected: nil,
		},
		{
			name:     "string leaf value",
			prefix:   "field",
			input:    "single error",
			expected: []string{"field: single error"},
		},
		{
			name:     "empty map",
			prefix:   "",
			input:    map[string]any{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flattenValidationErrors(tt.prefix, tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d results, got %d: %v", len(tt.expected), len(got), got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("result[%d]: expected %q, got %q", i, tt.expected[i], got[i])
				}
			}
		})
	}
}

func TestParseBulkLabelValidationError(t *testing.T) {
	t.Run("valid validation error", func(t *testing.T) {
		body := []byte(`{"message":{"json":{"3":{"criteria":{"1":{"compound_criteria":["Unknown field"],"op":["Must be one of EQUALS"]}}},"4":{"criteria":{"2":{"compound_criteria":["Unknown field"]}}}}}}`)
		result := parseBulkLabelValidationError(400, body)
		if result == nil {
			t.Fatal("expected non-nil result")
			return
		}
		if len(result.ItemErrors) != 2 {
			t.Fatalf("expected 2 item errors, got %d", len(result.ItemErrors))
		}
		item3, ok := result.ItemErrors[3]
		if !ok {
			t.Fatal("expected error for index 3")
		}
		if len(item3.Messages) != 2 {
			t.Fatalf("expected 2 messages for index 3, got %d: %v", len(item3.Messages), item3.Messages)
		}
		item4, ok := result.ItemErrors[4]
		if !ok {
			t.Fatal("expected error for index 4")
		}
		if len(item4.Messages) != 1 {
			t.Fatalf("expected 1 message for index 4, got %d: %v", len(item4.Messages), item4.Messages)
		}
	})

	t.Run("non-validation JSON", func(t *testing.T) {
		body := []byte(`{"description":"server error","error_code":"InternalError"}`)
		result := parseBulkLabelValidationError(400, body)
		if result != nil {
			t.Fatal("expected nil for non-validation error JSON")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		result := parseBulkLabelValidationError(400, []byte(`not json`))
		if result != nil {
			t.Fatal("expected nil for invalid JSON")
		}
	})

	t.Run("empty message.json", func(t *testing.T) {
		body := []byte(`{"message":{"json":{}}}`)
		result := parseBulkLabelValidationError(400, body)
		if result != nil {
			t.Fatal("expected nil for empty json map")
		}
	})

	t.Run("non-numeric keys skipped", func(t *testing.T) {
		body := []byte(`{"message":{"json":{"abc":{"field":["error"]}}}}`)
		result := parseBulkLabelValidationError(400, body)
		if result != nil {
			t.Fatal("expected nil when all keys are non-numeric")
		}
	})
}

func TestBulkValidationError_ErrorForIndex(t *testing.T) {
	bve := &BulkValidationError{
		StatusCode: 400,
		ItemErrors: map[int]*BulkItemError{
			2: {Index: 2, Messages: []string{"criteria[0].op: Invalid"}},
		},
	}

	err := bve.ErrorForIndex(2)
	if err == nil {
		t.Fatal("expected non-nil error for index 2")
	}
	if !strings.Contains(err.Error(), "criteria[0].op: Invalid") {
		t.Fatalf("expected specific error, got: %s", err)
	}

	err = bve.ErrorForIndex(0)
	if err == nil {
		t.Fatal("expected non-nil error for index 0")
	}
	if !strings.Contains(err.Error(), "batch failed due to validation errors") {
		t.Fatalf("expected generic error, got: %s", err)
	}
	if !strings.Contains(err.Error(), "2") {
		t.Fatalf("expected failing index in message, got: %s", err)
	}
}

func TestBulkValidationError_Error(t *testing.T) {
	bve := &BulkValidationError{
		StatusCode: 400,
		ItemErrors: map[int]*BulkItemError{
			1: {Index: 1, Messages: []string{"field: Required"}},
			3: {Index: 3, Messages: []string{"op: Invalid", "criteria: Unknown"}},
		},
	}
	errStr := bve.Error()
	if !strings.Contains(errStr, "item[1]") || !strings.Contains(errStr, "item[3]") {
		t.Fatalf("expected both item indices in error string, got: %s", errStr)
	}
	if !strings.Contains(errStr, "bulk validation error") {
		t.Fatalf("expected 'bulk validation error' prefix, got: %s", errStr)
	}
}
