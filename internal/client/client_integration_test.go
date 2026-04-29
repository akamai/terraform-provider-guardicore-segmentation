package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestAccClient_ListIncidents verifies ListIncidents works against a real Guardicore API.
//
// Run with: TF_ACC=1 go test -v -count=1 -timeout 120s ./internal/client/ -run TestAccClient_ListIncidents.
func TestAccClient_ListIncidents(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set, skipping integration test")
	}

	config, err := loadIntegrationTestConfig()
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}
	t.Logf("Testing against: %s", config.BaseURL)

	apiClient, err := NewClient(Config{
		BaseURL:            config.BaseURL,
		Username:           config.Username,
		Password:           config.Password,
		AccessToken:        config.AccessToken,
		InsecureSkipVerify: config.InsecureSkipVerify,
		RequestTimeout:     120,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	nowMs := time.Now().UnixMilli()
	fromMs := int64(946684800000) // 2000-01-01

	incidents, err := apiClient.ListIncidents(context.Background(), fromMs, nowMs)
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}
	t.Logf("ListIncidents returned %d incidents", len(incidents))

	if len(incidents) > 0 {
		first := incidents[0]
		var keys []string
		for k := range first {
			keys = append(keys, k)
		}
		t.Logf("First incident keys: %v", keys)
		t.Logf("First incident id=%v type=%v severity=%v", first["id"], first["type"], first["severity"])
	}
}

// integrationTestConfig holds config loaded from testconfig.json for integration tests.
type integrationTestConfig struct {
	BaseURL            string `json:"base_url"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	AccessToken        string `json:"access_token"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

func loadIntegrationTestConfig() (*integrationTestConfig, error) {
	// Allow env var override for base URL
	if baseURL := os.Getenv("GUARDICORE_BASE_URL"); baseURL != "" {
		return &integrationTestConfig{
			BaseURL:            baseURL,
			Username:           os.Getenv("GUARDICORE_USERNAME"),
			Password:           os.Getenv("GUARDICORE_PASSWORD"),
			AccessToken:        os.Getenv("GUARDICORE_ACCESS_TOKEN"),
			InsecureSkipVerify: strings.EqualFold(os.Getenv("GUARDICORE_INSECURE_SKIP_VERIFY"), "true"),
		}, nil
	}

	paths := []string{"testconfig.json", "../../testconfig.json"}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var config integrationTestConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}
		if config.BaseURL == "" {
			continue
		}
		return &config, nil
	}

	return nil, fmt.Errorf("no testconfig.json found and GUARDICORE_BASE_URL not set")
}
