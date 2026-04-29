package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultPageSize = 100

// maxPaginationPages is the maximum number of pages fetched during a paginated
// list operation. Prevents infinite loops when the API returns inconsistent
// TotalCount values. At defaultPageSize=100 this allows up to 1,000,000 items.
var maxPaginationPages = 10_000

// DefaultRequestTimeout is the default HTTP request timeout in seconds.
const DefaultRequestTimeout = 30

// ErrWorksitesFeatureDisabled is returned when the worksites feature is not enabled on the Akamai Guardicore Segmentation instance.
var ErrWorksitesFeatureDisabled = errors.New("worksites feature is disabled on this Akamai Guardicore Segmentation instance; enable it in settings before managing worksites via Terraform")

// ErrPolicyGroupsFeatureDisabled is returned when the policy groups feature is not enabled on the Akamai Guardicore Segmentation instance.
var ErrPolicyGroupsFeatureDisabled = errors.New("policy groups feature is disabled on this Akamai Guardicore Segmentation instance; enable it in settings before managing policy groups via Terraform")

// APIError represents a structured error response from the Akamai Guardicore Segmentation API.
type APIError struct {
	StatusCode  int    `json:"-"`
	Description string `json:"description"`
	ErrorCode   string `json:"error_code"`
	ErrorDump   string `json:"error_dump"`
}

func (e *APIError) Error() string {
	if e.ErrorDump != "" {
		return fmt.Sprintf("API error %d (%s): %s", e.StatusCode, e.ErrorCode, e.ErrorDump)
	}
	if e.Description != "" {
		return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Description)
	}
	return fmt.Sprintf("API error %d", e.StatusCode)
}

// IsAlreadyExists returns true if the API error indicates a duplicate resource.
func (e *APIError) IsAlreadyExists() bool {
	return e.StatusCode == http.StatusBadRequest &&
		e.ErrorCode == "IllegalValue" &&
		strings.Contains(e.ErrorDump, "already in use")
}

// parseAPIError attempts to parse an API error response body into an APIError.
// If parsing fails, it returns a generic APIError with the raw body as Description.
func parseAPIError(statusCode int, body []byte) *APIError {
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil || (apiErr.Description == "" && apiErr.ErrorCode == "") {
		return &APIError{
			StatusCode:  statusCode,
			Description: string(body),
		}
	}
	apiErr.StatusCode = statusCode
	return &apiErr
}

func looksLikeMFAError(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest && statusCode != http.StatusForbidden {
		return false
	}
	s := strings.ToLower(string(body))
	return strings.Contains(s, "mfa") ||
		strings.Contains(s, "multi-factor") ||
		strings.Contains(s, "two-factor") ||
		strings.Contains(s, "otp") ||
		strings.Contains(s, "authorization header is malformed") ||
		strings.Contains(s, "additional verification")
}

func looksLikeJWT(token string) bool {
	parts := strings.Split(token, ".")
	return len(parts) == 3 && parts[0] != "" && parts[1] != "" && parts[2] != ""
}

// Config holds the configuration for the Akamai Guardicore Segmentation API client.
type Config struct {
	BaseURL            string
	Username           string
	Password           string
	AccessToken        string
	RefreshToken       string
	InsecureSkipVerify bool
	RequestTimeout     int64 // HTTP request timeout in seconds; 0 uses DefaultRequestTimeout
}

// Client is the Akamai Guardicore Segmentation API client.
type Client struct {
	config     Config
	httpClient *http.Client
	token      string
	tokenMu    sync.RWMutex
}

// NewClient creates a new Akamai Guardicore Segmentation API client.
func NewClient(config Config) (*Client, error) {
	timeout := time.Duration(config.RequestTimeout) * time.Second
	if config.RequestTimeout <= 0 {
		timeout = DefaultRequestTimeout * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	c := &Client{
		config:     config,
		httpClient: httpClient,
	}

	// Initialize token based on provided credentials
	if config.AccessToken != "" {
		c.token = config.AccessToken
	} else if err := c.authenticate(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	return c, nil
}

// authenticate obtains an access token using configured credentials.
func (c *Client) authenticate(ctx context.Context) error {
	return c.authenticateIfStale(ctx, "")
}

// authenticateIfStale re-authenticates only if the current token matches staleToken.
// If staleToken is empty, it always re-authenticates (used for initial auth).
// This prevents redundant re-authentication when multiple goroutines receive 401
// concurrently — the first one refreshes the token and the rest see it already changed.
func (c *Client) authenticateIfStale(ctx context.Context, staleToken string) error {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if staleToken != "" && c.token != staleToken {
		return nil
	}

	if c.config.RefreshToken != "" {
		return c.refreshAuthentication(ctx)
	}

	if c.config.Username != "" && c.config.Password != "" {
		return c.passwordAuthentication(ctx)
	}

	return fmt.Errorf("no valid authentication method configured")
}

// refreshAuthentication uses refresh token to get new access token.
func (c *Client) refreshAuthentication(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/api/v3.0/authenticate/refresh", c.config.BaseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.RefreshToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		apiErr := parseAPIError(resp.StatusCode, body)
		return fmt.Errorf("refresh token authentication failed: %w", apiErr)
	}

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode refresh response: %w", err)
	}

	if authResp.AccessToken == "" {
		return fmt.Errorf("refresh authentication succeeded but returned empty access token")
	}

	c.token = authResp.AccessToken
	return nil
}

// passwordAuthentication uses username/password to get access token.
func (c *Client) passwordAuthentication(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/api/v3.0/authenticate", c.config.BaseURL)

	authReq := AuthRequest{
		Username: c.config.Username,
		Password: c.config.Password,
	}

	body, err := json.Marshal(authReq)
	if err != nil {
		return fmt.Errorf("failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		apiErr := parseAPIError(resp.StatusCode, body)
		if looksLikeMFAError(resp.StatusCode, body) {
			return fmt.Errorf("password authentication failed (possible MFA issue — "+
				"if your account has multi-factor authentication enabled, "+
				"use \"access_token\" or \"refresh_token\" instead of username/password): %w", apiErr)
		}
		return fmt.Errorf("password authentication failed: %w", apiErr)
	}

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	if authResp.AccessToken == "" {
		return fmt.Errorf("password authentication succeeded but returned empty access token; " +
			"if your account has MFA enabled, use \"access_token\" or \"refresh_token\" instead of username/password")
	}

	if !looksLikeJWT(authResp.AccessToken) {
		return fmt.Errorf("password authentication succeeded but the returned token does not appear to be a valid JWT; " +
			"this may indicate that your account has MFA enabled and requires additional verification; " +
			"use \"access_token\" or \"refresh_token\" instead of username/password")
	}

	c.token = authResp.AccessToken
	return nil
}

// doRequest performs an HTTP request with authentication and retry on 401.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return c.doRequestWithRetry(ctx, method, path, body, true)
}

func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body interface{}, retry bool) (*http.Response, error) {
	reqURL := fmt.Sprintf("%s%s", c.config.BaseURL, path)

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.tokenMu.RLock()
	token := c.token
	c.tokenMu.RUnlock()

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Handle 401 by re-authenticating and retrying once
	if resp.StatusCode == http.StatusUnauthorized && retry {
		resp.Body.Close()

		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("request canceled before re-authentication: %w", err)
		}

		if err := c.authenticateIfStale(ctx, token); err != nil {
			return nil, fmt.Errorf("re-authentication failed: %w", err)
		}

		return c.doRequestWithRetry(ctx, method, path, body, false)
	}

	return resp, nil
}

// Label operations

// CreateLabel creates a new label.
func (c *Client) CreateLabel(ctx context.Context, label *LabelCreate) (*LabelCreate, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/labels", label)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create label: %w", parseAPIError(resp.StatusCode, body))
	}

	var createResp CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, fmt.Errorf("failed to decode create response: %w", err)
	}

	label.ID = createResp.ID
	return label, nil
}

// GetLabel retrieves a label by ID.
func (c *Client) GetLabel(ctx context.Context, id string) (*Label, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v4.0/labels/%s", id), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get label: status %d, body: %s", resp.StatusCode, string(body))
	}

	var getResp LabelGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResp); err != nil {
		return nil, fmt.Errorf("failed to decode label: %w", err)
	}

	if len(getResp.Objects) == 0 {
		return nil, nil
	}

	return &getResp.Objects[0], nil
}

// UpdateLabel updates an existing label.
func (c *Client) UpdateLabel(ctx context.Context, id string, label *LabelUpdate) (*LabelUpdate, error) {
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v4.0/labels/%s", id), label)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to update label: %w", parseAPIError(resp.StatusCode, body))
	}

	return label, nil
}

// DeleteLabel deletes a label.
func (c *Client) DeleteLabel(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v4.0/labels/%s", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 404 is acceptable for delete - resource already deleted
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete label: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListLabels lists labels with optional filtering by key and value.
func (c *Client) ListLabels(ctx context.Context, key, value string) ([]Label, error) {
	var allLabels []Label
	offset := 0
	page := 0

	for {
		if page >= maxPaginationPages {
			return nil, fmt.Errorf(
				"pagination safety limit reached: fetched %d pages (%d items) listing labels; "+
					"the API may be returning inconsistent results, or the dataset is very large; "+
					"consider narrowing your query with filters",
				page, len(allLabels),
			)
		}
		page++

		apiPath := "/api/v4.0/labels"
		params := url.Values{}
		if key != "" {
			params.Set("key", key)
		}
		if value != "" {
			params.Set("value", value)
		}
		params.Set("offset", fmt.Sprintf("%d", offset))
		params.Set("limit", fmt.Sprintf("%d", defaultPageSize))
		apiPath = fmt.Sprintf("%s?%s", apiPath, params.Encode())

		resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to list labels: status %d, body: %s", resp.StatusCode, string(body))
		}

		var listResp ListLabelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode labels list: %w", err)
		}
		resp.Body.Close()

		allLabels = append(allLabels, listResp.Objects...)

		if len(listResp.Objects) < defaultPageSize || len(allLabels) >= listResp.TotalCount {
			break
		}

		offset += len(listResp.Objects)
	}

	return allLabels, nil
}

// LabelGroup operations

// CreateLabelGroup creates a new label group.
func (c *Client) CreateLabelGroup(ctx context.Context, labelGroup *LabelGroupCreate) (*LabelGroupCreate, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/label-groups", labelGroup)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create label group: status %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, fmt.Errorf("failed to decode create response: %w", err)
	}

	labelGroup.ID = createResp.ID
	return labelGroup, nil
}

// GetLabelGroup retrieves a label group by ID.
func (c *Client) GetLabelGroup(ctx context.Context, id string) (*LabelGroup, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v4.0/label-groups/%s", id), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get label group: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read label group response: %w", err)
	}

	var getResp LabelGroupGetResponse
	if err := json.Unmarshal(body, &getResp); err == nil && len(getResp.Objects) > 0 {
		return &getResp.Objects[0], nil
	}

	var labelGroup LabelGroup
	if err := json.Unmarshal(body, &labelGroup); err != nil {
		return nil, fmt.Errorf("failed to decode label group: %w", err)
	}

	return &labelGroup, nil
}

// UpdateLabelGroup updates an existing label group.
func (c *Client) UpdateLabelGroup(ctx context.Context, id string, labelGroup *LabelGroupCreate) (*LabelGroupCreate, error) {
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v4.0/label-groups/%s", id), labelGroup)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to update label group: status %d, body: %s", resp.StatusCode, string(body))
	}

	labelGroup.ID = id
	return labelGroup, nil
}

// DeleteLabelGroup deletes a label group.
func (c *Client) DeleteLabelGroup(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v4.0/label-groups/%s", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 404 is acceptable for delete - resource already deleted
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete label group: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListLabelGroups lists label groups with optional filtering by key and value.
func (c *Client) ListLabelGroups(ctx context.Context, key, value string) ([]LabelGroup, error) {
	var allGroups []LabelGroup
	offset := 0
	page := 0

	for {
		if page >= maxPaginationPages {
			return nil, fmt.Errorf(
				"pagination safety limit reached: fetched %d pages (%d items) listing label groups; "+
					"the API may be returning inconsistent results, or the dataset is very large; "+
					"consider narrowing your query with filters",
				page, len(allGroups),
			)
		}
		page++

		apiPath := "/api/v4.0/label-groups"
		params := url.Values{}
		if key != "" {
			params.Set("key", key)
		}
		if value != "" {
			params.Set("value", value)
		}
		params.Set("offset", fmt.Sprintf("%d", offset))
		params.Set("limit", fmt.Sprintf("%d", defaultPageSize))
		apiPath = fmt.Sprintf("%s?%s", apiPath, params.Encode())

		resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to list label groups: status %d, body: %s", resp.StatusCode, string(body))
		}

		var listResp ListLabelGroupsResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode label groups list: %w", err)
		}
		resp.Body.Close()

		allGroups = append(allGroups, listResp.Objects...)

		if len(listResp.Objects) < defaultPageSize || len(allGroups) >= listResp.TotalCount {
			break
		}

		offset += len(listResp.Objects)
	}

	return allGroups, nil
}

// PublishLabelGroups publishes label group changes.
func (c *Client) PublishLabelGroups(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/label-groups/publish", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to publish label groups: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Policy Group operations

// isPolicyGroupFeatureDisabled checks if an API error response indicates the policy groups feature is disabled.
func isPolicyGroupFeatureDisabled(statusCode int, body []byte) bool {
	if statusCode == http.StatusNotFound {
		return true
	}
	if statusCode != http.StatusForbidden {
		return false
	}
	return strings.Contains(string(body), "policy groups feature is disabled") ||
		strings.Contains(string(body), "policy_groups feature is disabled")
}

// CreatePolicyGroup creates a new policy group.
func (c *Client) CreatePolicyGroup(ctx context.Context, group *PolicyGroupCreate) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/policy-groups", group)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		if isPolicyGroupFeatureDisabled(resp.StatusCode, body) {
			return "", ErrPolicyGroupsFeatureDisabled
		}
		return "", fmt.Errorf("failed to create policy group: status %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("failed to decode create policy group response: %w", err)
	}

	return createResp.ID, nil
}

// GetPolicyGroup retrieves a policy group by ID using list with ID filter.
func (c *Client) GetPolicyGroup(ctx context.Context, id string) (*PolicyGroup, error) {
	apiPath := fmt.Sprintf("/api/v4.0/policy-groups?id=%s&limit=1", url.QueryEscape(id))
	resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if isPolicyGroupFeatureDisabled(resp.StatusCode, body) {
			return nil, ErrPolicyGroupsFeatureDisabled
		}
		return nil, fmt.Errorf("failed to get policy group: status %d, body: %s", resp.StatusCode, string(body))
	}

	var listResp ListPolicyGroupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode policy group response: %w", err)
	}

	if len(listResp.Objects) == 0 {
		return nil, nil
	}

	return &listResp.Objects[0], nil
}

// UpdatePolicyGroup updates an existing policy group.
func (c *Client) UpdatePolicyGroup(ctx context.Context, id string, group *PolicyGroupCreate) error {
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v4.0/policy-groups/%s", url.PathEscape(id)), group)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		if isPolicyGroupFeatureDisabled(resp.StatusCode, body) {
			return ErrPolicyGroupsFeatureDisabled
		}
		return fmt.Errorf("failed to update policy group: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeletePolicyGroup deletes a policy group.
func (c *Client) DeletePolicyGroup(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v4.0/policy-groups/%s", url.PathEscape(id)), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		if isPolicyGroupFeatureDisabled(resp.StatusCode, body) {
			return ErrPolicyGroupsFeatureDisabled
		}
		return fmt.Errorf("failed to delete policy group: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListPolicyGroups lists policy groups with optional filtering by name and type.
func (c *Client) ListPolicyGroups(ctx context.Context, name, typ string) ([]PolicyGroup, error) {
	var allGroups []PolicyGroup
	offset := 0
	page := 0

	for {
		if page >= maxPaginationPages {
			return nil, fmt.Errorf(
				"pagination safety limit reached: fetched %d pages (%d items) listing policy groups; "+
					"the API may be returning inconsistent results, or the dataset is very large; "+
					"consider narrowing your query with filters",
				page, len(allGroups),
			)
		}
		page++

		apiPath := "/api/v4.0/policy-groups"
		params := url.Values{}
		if name != "" {
			params.Set("name", name)
		}
		if typ != "" {
			params.Set("type", typ)
		}
		params.Set("offset", fmt.Sprintf("%d", offset))
		params.Set("limit", fmt.Sprintf("%d", defaultPageSize))
		apiPath = fmt.Sprintf("%s?%s", apiPath, params.Encode())

		resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if isPolicyGroupFeatureDisabled(resp.StatusCode, body) {
				return nil, ErrPolicyGroupsFeatureDisabled
			}
			return nil, fmt.Errorf("failed to list policy groups: status %d, body: %s", resp.StatusCode, string(body))
		}

		var listResp ListPolicyGroupsResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode policy groups list: %w", err)
		}
		resp.Body.Close()

		allGroups = append(allGroups, listResp.Objects...)

		if len(listResp.Objects) < defaultPageSize || len(allGroups) >= listResp.TotalCount {
			break
		}

		offset += len(listResp.Objects)
	}

	return allGroups, nil
}

// PublishPolicyGroups publishes policy group changes.
func (c *Client) PublishPolicyGroups(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/policy-groups/publish", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		if isPolicyGroupFeatureDisabled(resp.StatusCode, body) {
			return ErrPolicyGroupsFeatureDisabled
		}
		return fmt.Errorf("failed to publish policy groups: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// PolicyRule operations

// CreatePolicyRule creates a new policy rule.
func (c *Client) CreatePolicyRule(ctx context.Context, spec map[string]interface{}) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/visibility/policy/rules", spec)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create policy rule: status %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("failed to decode create response: %w", err)
	}

	return createResp.ID, nil
}

// BulkCreatePolicyRules creates multiple policy rules in a single request.
func (c *Client) BulkCreatePolicyRules(ctx context.Context, specs []map[string]any) (*PolicyRulesBulkCreateResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/visibility/policy/rules/bulk", specs)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to bulk create policy rules: status %d, body: %s", resp.StatusCode, string(body))
	}

	var bulkResp PolicyRulesBulkCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&bulkResp); err != nil {
		return nil, fmt.Errorf("failed to decode bulk create policy rules response: %w", err)
	}

	return &bulkResp, nil
}

// GetPolicyRule retrieves a policy rule by ID.
func (c *Client) GetPolicyRule(ctx context.Context, id string) (map[string]interface{}, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v4.0/visibility/policy/rules/%s", id), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get policy rule: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy rule response: %w", err)
	}

	var getResp PolicyRuleGetResponse
	if err := json.Unmarshal(body, &getResp); err == nil && len(getResp.Objects) > 0 {
		return getResp.Objects[0], nil
	}

	var rule map[string]interface{}
	if err := json.Unmarshal(body, &rule); err != nil {
		return nil, fmt.Errorf("failed to decode policy rule: %w", err)
	}

	return rule, nil
}

// UpdatePolicyRule updates an existing policy rule.
func (c *Client) UpdatePolicyRule(ctx context.Context, id string, spec map[string]interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v4.0/visibility/policy/rules/%s", id), spec)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update policy rule: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeletePolicyRule deletes a policy rule (uses POST to /delete/{id} per API spec).
func (c *Client) DeletePolicyRule(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v4.0/visibility/policy/rules/delete/%s", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 404 is acceptable for delete - resource already deleted
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete policy rule: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListPolicyRules lists all policy rules with pagination.
func (c *Client) ListPolicyRules(ctx context.Context) ([]map[string]interface{}, error) {
	var allRules []map[string]interface{}
	offset := 0
	page := 0

	for {
		if page >= maxPaginationPages {
			return nil, fmt.Errorf(
				"pagination safety limit reached: fetched %d pages (%d items) listing policy rules; "+
					"the API may be returning inconsistent results, or the dataset is very large; "+
					"consider narrowing your query with filters",
				page, len(allRules),
			)
		}
		page++

		apiPath := "/api/v4.0/visibility/policy/rules"
		params := url.Values{}
		params.Set("offset", fmt.Sprintf("%d", offset))
		params.Set("limit", fmt.Sprintf("%d", defaultPageSize))
		apiPath = fmt.Sprintf("%s?%s", apiPath, params.Encode())

		resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to list policy rules: status %d, body: %s", resp.StatusCode, string(body))
		}

		var listResp ListPolicyRulesResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode policy rules list: %w", err)
		}
		resp.Body.Close()

		allRules = append(allRules, listResp.Objects...)

		if len(listResp.Objects) < defaultPageSize || len(allRules) >= listResp.TotalCount {
			break
		}

		offset += len(listResp.Objects)
	}

	return allRules, nil
}

// DNS Blocklist operations

// CreateDnsBlocklist creates a new DNS blocklist.
func (c *Client) CreateDnsBlocklist(ctx context.Context, blocklist *DnsBlocklistCreate) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/dns_security", blocklist)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create DNS blocklist: status %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("failed to decode create response: %w", err)
	}

	return createResp.ID, nil
}

// GetDnsBlocklist retrieves a DNS blocklist by ID.
// Uses the list endpoint with ids filter because the single-item GET endpoint
// does not return the id field or proper type string.
func (c *Client) GetDnsBlocklist(ctx context.Context, id string) (*DnsBlocklist, error) {
	apiPath := fmt.Sprintf("/api/v4.0/dns_security?ids=%s&domains_limit=-1", id)
	resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get DNS blocklist: status %d, body: %s", resp.StatusCode, string(body))
	}

	var listResp ListDnsBlocklistsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode DNS blocklist response: %w", err)
	}

	if len(listResp.Objects) == 0 {
		return nil, nil
	}

	return &listResp.Objects[0], nil
}

// UpdateDnsBlocklist updates an existing DNS blocklist using PATCH.
func (c *Client) UpdateDnsBlocklist(ctx context.Context, id string, blocklist *DnsBlocklistEdit) error {
	resp, err := c.doRequest(ctx, http.MethodPatch, fmt.Sprintf("/api/v4.0/dns_security/%s", id), blocklist)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update DNS blocklist: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteDnsBlocklist deletes a DNS blocklist.
func (c *Client) DeleteDnsBlocklist(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v4.0/dns_security/%s", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 404 is acceptable for delete - resource already deleted
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete DNS blocklist: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListDnsBlocklists lists DNS blocklists with optional filtering by name and type.
func (c *Client) ListDnsBlocklists(ctx context.Context, name, listType string) ([]DnsBlocklist, error) {
	var allBlocklists []DnsBlocklist
	startAt := 0
	page := 0

	for {
		if page >= maxPaginationPages {
			return nil, fmt.Errorf(
				"pagination safety limit reached: fetched %d pages (%d items) listing DNS blocklists; "+
					"the API may be returning inconsistent results, or the dataset is very large; "+
					"consider narrowing your query with filters",
				page, len(allBlocklists),
			)
		}
		page++

		apiPath := "/api/v4.0/dns_security"
		params := url.Values{}
		if name != "" {
			params.Set("name", name)
		}
		if listType != "" {
			params.Set("type", listType)
		}
		params.Set("start_at", fmt.Sprintf("%d", startAt))
		params.Set("max_results", fmt.Sprintf("%d", defaultPageSize))
		params.Set("domains_limit", "-1")
		apiPath = fmt.Sprintf("%s?%s", apiPath, params.Encode())

		resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to list DNS blocklists: status %d, body: %s", resp.StatusCode, string(body))
		}

		var listResp ListDnsBlocklistsResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode DNS blocklists list: %w", err)
		}
		resp.Body.Close()

		allBlocklists = append(allBlocklists, listResp.Objects...)

		if len(listResp.Objects) < defaultPageSize || len(allBlocklists) >= listResp.TotalCount {
			break
		}

		startAt += len(listResp.Objects)
	}

	return allBlocklists, nil
}

// BulkCreateDnsBlocklists creates multiple DNS blocklists in a single request.
func (c *Client) BulkCreateDnsBlocklists(ctx context.Context, blocklists []DnsBlocklistCreate) (*BulkCreateDnsBlocklistResponse, error) {
	req := &BulkCreateDnsBlocklistRequest{Items: blocklists}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/dns_security/bulk", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to bulk create DNS blocklists: status %d, body: %s", resp.StatusCode, string(body))
	}

	var bulkResp BulkCreateDnsBlocklistResponse
	if err := json.NewDecoder(resp.Body).Decode(&bulkResp); err != nil {
		return nil, fmt.Errorf("failed to decode bulk create response: %w", err)
	}

	return &bulkResp, nil
}

// BulkDeleteDnsBlocklists deletes multiple DNS blocklists in a single request.
func (c *Client) BulkDeleteDnsBlocklists(ctx context.Context, ids []string) error {
	apiPath := "/api/v4.0/dns_security/bulk"
	params := url.Values{}
	params.Set("ids", strings.Join(ids, ","))
	apiPath = fmt.Sprintf("%s?%s", apiPath, params.Encode())

	resp, err := c.doRequest(ctx, http.MethodDelete, apiPath, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to bulk delete DNS blocklists: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// BulkEditDnsBlocklists edits multiple DNS blocklists in a single request.
func (c *Client) BulkEditDnsBlocklists(ctx context.Context, req *BulkEditDnsBlocklistRequest) error {
	resp, err := c.doRequest(ctx, http.MethodPatch, "/api/v4.0/dns_security/bulk", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to bulk edit DNS blocklists: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ResetDnsBlocklistHitCount resets the hit counter for a DNS blocklist.
func (c *Client) ResetDnsBlocklistHitCount(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v4.0/dns_security/%s/hits", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to reset DNS blocklist hit count: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Incident operations

// CreateIncident creates a new incident.
// NOTE: The response uses "incident_id" (not "id" like other resources).
func (c *Client) CreateIncident(ctx context.Context, incident *IncidentCreate) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/incidents", incident)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create incident: status %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp CreateIncidentResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("failed to decode create incident response: %w", err)
	}

	return createResp.IncidentID, nil
}

// GetIncident retrieves an incident by ID.
// Uses the v3.0 generic-incidents list endpoint with id filter because there is no dedicated single-get endpoint.
// A wide time range is used to ensure the incident is found regardless of when it was created.
func (c *Client) GetIncident(ctx context.Context, id string) (map[string]interface{}, error) {
	nowMs := time.Now().UnixMilli()
	// Use a range from year 2000 to now+1day to cover all possible incident times.
	fromMs := int64(946684800000) // 2000-01-01T00:00:00Z in milliseconds
	toMs := nowMs + 86400000      // now + 1 day
	apiPath := fmt.Sprintf("/api/v3.0/generic-incidents?id=%s&from_time=%d&to_time=%d&limit=1", url.QueryEscape(id), fromMs, toMs)

	resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get incident: status %d, body: %s", resp.StatusCode, string(body))
	}

	var listResp ListIncidentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode incident response: %w", err)
	}

	if len(listResp.Objects) == 0 {
		return nil, nil
	}

	return listResp.Objects[0], nil
}

// ListIncidents lists incidents with pagination using the required time range.
func (c *Client) ListIncidents(ctx context.Context, fromTime, toTime int64) ([]map[string]interface{}, error) {
	var allIncidents []map[string]interface{}
	offset := 0
	page := 0

	for {
		if page >= maxPaginationPages {
			return nil, fmt.Errorf(
				"pagination safety limit reached: fetched %d pages (%d items) listing incidents; "+
					"the API may be returning inconsistent results, or the dataset is very large; "+
					"consider narrowing your query with filters",
				page, len(allIncidents),
			)
		}
		page++

		apiPath := "/api/v3.0/generic-incidents"
		params := url.Values{}
		params.Set("from_time", fmt.Sprintf("%d", fromTime))
		params.Set("to_time", fmt.Sprintf("%d", toTime))
		params.Set("offset", fmt.Sprintf("%d", offset))
		params.Set("limit", fmt.Sprintf("%d", defaultPageSize))
		apiPath = fmt.Sprintf("%s?%s", apiPath, params.Encode())

		resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to list incidents: status %d, body: %s", resp.StatusCode, string(body))
		}

		var listResp ListIncidentsResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode incidents list: %w", err)
		}
		resp.Body.Close()

		allIncidents = append(allIncidents, listResp.Objects...)

		if len(listResp.Objects) < defaultPageSize || len(allIncidents) >= listResp.TotalCount {
			break
		}

		offset += len(listResp.Objects)
	}

	return allIncidents, nil
}

// BulkCreateIncidents creates multiple incidents in a single request.
// NOTE: Uses "incidents" wrapper key (not "items" like DNS blocklists).
func (c *Client) BulkCreateIncidents(ctx context.Context, incidents []IncidentCreate) (*BulkCreateIncidentResponse, error) {
	req := &BulkCreateIncidentRequest{Incidents: incidents}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/incidents/bulk", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to bulk create incidents: status %d, body: %s", resp.StatusCode, string(body))
	}

	var bulkResp BulkCreateIncidentResponse
	if err := json.NewDecoder(resp.Body).Decode(&bulkResp); err != nil {
		return nil, fmt.Errorf("failed to decode bulk create incident response: %w", err)
	}

	return &bulkResp, nil
}

// Worksite operations

// isWorksiteFeatureDisabled checks if an API error response indicates the worksites feature is disabled.
func isWorksiteFeatureDisabled(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest {
		return false
	}
	return strings.Contains(string(body), "worksites feature is disabled")
}

// CreateWorksite creates a new worksite.
func (c *Client) CreateWorksite(ctx context.Context, worksite *WorksiteCreate) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/worksites", worksite)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		if isWorksiteFeatureDisabled(resp.StatusCode, body) {
			return "", ErrWorksitesFeatureDisabled
		}
		return "", fmt.Errorf("failed to create worksite: status %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("failed to decode create worksite response: %w", err)
	}

	return createResp.ID, nil
}

// GetWorksite retrieves a worksite by ID.
// It lists all worksites with pagination and filters for an exact ID match,
// since the API does not provide a dedicated single-item GET endpoint.
func (c *Client) GetWorksite(ctx context.Context, id string) (*Worksite, error) {
	worksites, err := c.ListWorksites(ctx, "")
	if err != nil {
		return nil, err
	}

	for _, w := range worksites {
		if w.ID == id {
			return &w, nil
		}
	}

	return nil, nil
}

// UpdateWorksite updates an existing worksite using PUT with id in body.
func (c *Client) UpdateWorksite(ctx context.Context, worksite *WorksiteUpdate) error {
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/v4.0/worksites", worksite)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if isWorksiteFeatureDisabled(resp.StatusCode, body) {
			return ErrWorksitesFeatureDisabled
		}
		return fmt.Errorf("failed to update worksite: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteWorksite deletes a worksite using the bulk delete endpoint.
func (c *Client) DeleteWorksite(ctx context.Context, id string) error {
	req := &DeleteWorksitesRequest{
		ComponentIDs: []string{id},
		NegateArgs:   nil,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/worksites/delete_worksites", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 404 is acceptable for delete - resource already deleted
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		if isWorksiteFeatureDisabled(resp.StatusCode, body) {
			return ErrWorksitesFeatureDisabled
		}
		return fmt.Errorf("failed to delete worksite: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListWorksites lists worksites with optional filtering by name.
func (c *Client) ListWorksites(ctx context.Context, name string) ([]Worksite, error) {
	var allWorksites []Worksite
	offset := 0
	page := 0

	for {
		if page >= maxPaginationPages {
			return nil, fmt.Errorf(
				"pagination safety limit reached: fetched %d pages (%d items) listing worksites; "+
					"the API may be returning inconsistent results, or the dataset is very large; "+
					"consider narrowing your query with filters",
				page, len(allWorksites),
			)
		}
		page++

		apiPath := "/api/v4.0/worksites"
		params := url.Values{}
		if name != "" {
			params.Set("gc_filter", name)
		}
		params.Set("offset", fmt.Sprintf("%d", offset))
		params.Set("limit", fmt.Sprintf("%d", defaultPageSize))
		apiPath = fmt.Sprintf("%s?%s", apiPath, params.Encode())

		resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if isWorksiteFeatureDisabled(resp.StatusCode, body) {
				return nil, ErrWorksitesFeatureDisabled
			}
			return nil, fmt.Errorf("failed to list worksites: status %d, body: %s", resp.StatusCode, string(body))
		}

		var listResp ListWorksitesResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode worksites list: %w", err)
		}
		resp.Body.Close()

		allWorksites = append(allWorksites, listResp.Objects...)

		if len(listResp.Objects) < defaultPageSize || len(allWorksites) >= listResp.TotalCount {
			break
		}

		offset += len(listResp.Objects)
	}

	return allWorksites, nil
}

// User Group operations

// CreateUserGroup creates a new user group.
func (c *Client) CreateUserGroup(ctx context.Context, userGroup *UserGroupCreate) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/visibility/user-groups", userGroup)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create user group: status %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("failed to decode create user group response: %w", err)
	}

	return createResp.ID, nil
}

// GetUserGroup retrieves a user group by ID.
// It lists all user groups and filters for an exact ID match,
// since the API does not provide a dedicated single-item GET endpoint.
func (c *Client) GetUserGroup(ctx context.Context, id string) (*UserGroup, error) {
	userGroups, err := c.ListUserGroups(ctx, "")
	if err != nil {
		return nil, err
	}

	for _, ug := range userGroups {
		if ug.ID == id {
			return &ug, nil
		}
	}

	return nil, nil
}

// UpdateUserGroup updates an existing user group using PUT with ID in URL.
func (c *Client) UpdateUserGroup(ctx context.Context, id string, userGroup *UserGroupCreate) error {
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/v4.0/visibility/user-groups/"+id, userGroup)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update user group: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteUserGroup deletes a user group by ID.
func (c *Client) DeleteUserGroup(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v4.0/visibility/user-groups/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 404 is acceptable for delete - resource already deleted
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete user group: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListUserGroups lists user groups with optional search filter.
func (c *Client) ListUserGroups(ctx context.Context, search string) ([]UserGroup, error) {
	var allUserGroups []UserGroup
	offset := 0
	page := 0

	for {
		if page >= maxPaginationPages {
			return nil, fmt.Errorf(
				"pagination safety limit reached: fetched %d pages (%d items) listing user groups; "+
					"the API may be returning inconsistent results, or the dataset is very large; "+
					"consider narrowing your query with filters",
				page, len(allUserGroups),
			)
		}
		page++

		apiPath := "/api/v4.0/visibility/user-groups"
		params := url.Values{}
		if search != "" {
			params.Set("search", search)
		}
		params.Set("offset", fmt.Sprintf("%d", offset))
		params.Set("limit", fmt.Sprintf("%d", defaultPageSize))
		apiPath = fmt.Sprintf("%s?%s", apiPath, params.Encode())

		resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to list user groups: status %d, body: %s", resp.StatusCode, string(body))
		}

		var listResp ListUserGroupsResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode user groups list: %w", err)
		}
		resp.Body.Close()

		allUserGroups = append(allUserGroups, listResp.Objects...)

		if len(listResp.Objects) < defaultPageSize || len(allUserGroups) >= listResp.TotalCount {
			break
		}

		offset += len(listResp.Objects)
	}

	return allUserGroups, nil
}

// CreateAsset creates a new asset.
func (c *Client) CreateAsset(ctx context.Context, asset *AssetCreate) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/assets/", asset)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create asset: status %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp CreateAssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("failed to decode create asset response: %w", err)
	}

	return createResp.AssetID, nil
}

// GetAsset retrieves a single asset by ID using the list endpoint with id filter.
// The single-item GET endpoint is not available (returns 405), so we use the list
// endpoint with an ID filter instead (similar to DNS blocklist pattern).
func (c *Client) GetAsset(ctx context.Context, id string) (*Asset, error) {
	apiPath := fmt.Sprintf("/api/v4.0/assets?id=%s&max_results=1", url.QueryEscape(id))

	resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get asset: status %d, body: %s", resp.StatusCode, string(body))
	}

	var listResp ListAssetsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode asset: %w", err)
	}

	if len(listResp.Objects) == 0 {
		return nil, nil
	}

	return &listResp.Objects[0], nil
}

// UpdateAsset updates an existing asset by ID.
func (c *Client) UpdateAsset(ctx context.Context, id string, asset *AssetUpdate) error {
	apiPath := fmt.Sprintf("/api/v4.0/assets/%s", url.PathEscape(id))

	resp, err := c.doRequest(ctx, http.MethodPut, apiPath, asset)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update asset: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteAsset deactivates an asset by ID.
// NOTE: The API DELETE endpoint deactivates the asset rather than permanently removing it.
func (c *Client) DeleteAsset(ctx context.Context, id string) error {
	apiPath := fmt.Sprintf("/api/v4.0/assets/%s", url.PathEscape(id))

	resp, err := c.doRequest(ctx, http.MethodDelete, apiPath, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to deactivate asset: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListAssets lists assets with optional name filter using start_at/max_results pagination.
func (c *Client) ListAssets(ctx context.Context, name string) ([]Asset, error) {
	var allAssets []Asset
	startAt := 0
	page := 0

	for {
		if page >= maxPaginationPages {
			return nil, fmt.Errorf(
				"pagination safety limit reached: fetched %d pages (%d items) listing assets; "+
					"the API may be returning inconsistent results, or the dataset is very large; "+
					"consider narrowing your query with filters",
				page, len(allAssets),
			)
		}
		page++

		apiPath := "/api/v4.0/assets"
		params := url.Values{}
		if name != "" {
			params.Set("name", name)
		}
		params.Set("start_at", fmt.Sprintf("%d", startAt))
		params.Set("max_results", fmt.Sprintf("%d", defaultPageSize))
		apiPath = fmt.Sprintf("%s?%s", apiPath, params.Encode())

		resp, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to list assets: status %d, body: %s", resp.StatusCode, string(body))
		}

		var listResp ListAssetsResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode assets list: %w", err)
		}
		resp.Body.Close()

		allAssets = append(allAssets, listResp.Objects...)

		if len(listResp.Objects) < defaultPageSize || len(allAssets) >= listResp.TotalCount {
			break
		}

		startAt += len(listResp.Objects)
	}

	return allAssets, nil
}

// BulkCreateAssets creates multiple assets in a single request.
// NOTE: The bulk create endpoint uses a plain array body (not wrapped in an object key).
func (c *Client) BulkCreateAssets(ctx context.Context, assets []AssetCreate) ([]string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/assets/bulk", assets)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to bulk create assets: status %d, body: %s", resp.StatusCode, string(body))
	}

	var ids []string
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("failed to decode bulk create assets response: %w", err)
	}

	return ids, nil
}

// BulkUpdateAssets updates multiple assets in a single request.
func (c *Client) BulkUpdateAssets(ctx context.Context, assets []AssetBulkUpdateItem) error {
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/v4.0/assets/bulk", assets)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to bulk update assets: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// BulkDeactivateAssets deactivates multiple assets in a single request.
func (c *Client) BulkDeactivateAssets(ctx context.Context, assetIDs []string) error {
	items := make([]BulkDeactivateAssetItem, len(assetIDs))
	for i, id := range assetIDs {
		items[i] = BulkDeactivateAssetItem{AssetID: id}
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/assets/bulk/deactivate", items)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to bulk deactivate assets: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreatePolicyRevision publishes policy changes.
func (c *Client) CreatePolicyRevision(ctx context.Context, req *PolicyRevisionRequest) error {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/visibility/policy/revisions", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create policy revision: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// AssignWorksite assigns entities (assets, agents, etc.) to a worksite.
func (c *Client) AssignWorksite(ctx context.Context, req *WorksiteAssignRequest) error {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v4.0/worksites/assign", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if isWorksiteFeatureDisabled(resp.StatusCode, body) {
			return ErrWorksitesFeatureDisabled
		}
		return fmt.Errorf("failed to assign worksite: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// MovePolicyRulesToWorksite moves policy rules to a worksite using the bulk move endpoint.
// Use worksiteID "all_worksites" to unassign rules from their current worksite.
func (c *Client) MovePolicyRulesToWorksite(ctx context.Context, worksiteID string, ruleIDs []string) error {
	apiPath := fmt.Sprintf("/api/v3.0/visibility/policy/rules-bulk/worksite/move/%s", url.PathEscape(worksiteID))

	req := &PolicyRuleBulkWorksiteMoveRequest{
		IDs:        ruleIDs,
		NegateArgs: &PolicyRuleBulkWorksiteMoveNegate{Filters: map[string]any{}},
	}

	resp, err := c.doRequest(ctx, http.MethodPost, apiPath, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if isWorksiteFeatureDisabled(resp.StatusCode, body) {
			return ErrWorksitesFeatureDisabled
		}
		return fmt.Errorf("failed to move policy rules to worksite: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
