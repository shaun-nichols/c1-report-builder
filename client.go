package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// C1Client is a lightweight ConductorOne API client.
type C1Client struct {
	clientID     string
	clientSecret string
	tenant       string
	baseURL      string
	httpClient   *http.Client

	mu             sync.Mutex
	token          string
	tokenExpiresAt time.Time
}

func NewC1Client(clientID, clientSecret string) (*C1Client, error) {
	tenant, err := extractTenant(clientID)
	if err != nil {
		return nil, err
	}
	return &C1Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		tenant:       tenant,
		baseURL:      fmt.Sprintf("https://%s.conductor.one", tenant),
		httpClient:   &http.Client{Timeout: 60 * time.Second},
	}, nil
}

var tenantRe = regexp.MustCompile(`@(.+?)\.conductor\.one`)

func extractTenant(clientID string) (string, error) {
	m := tenantRe.FindStringSubmatch(clientID)
	if len(m) < 2 {
		return "", fmt.Errorf("invalid Client ID format — expected something like name@tenant.conductor.one/spc")
	}
	return m[1], nil
}

func (c *C1Client) ensureToken() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExpiresAt.Add(-60*time.Second)) {
		return nil
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/auth/v1/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed: %s", friendlyAPIError(resp.StatusCode, body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	c.token = tokenResp.AccessToken
	c.tokenExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return nil
}

func (c *C1Client) doGet(path string, params map[string]string) (json.RawMessage, error) {
	if err := c.ensureToken(); err != nil {
		return nil, err
	}

	u := c.baseURL + path
	if len(params) > 0 {
		v := url.Values{}
		for k, val := range params {
			v.Set(k, val)
		}
		u += "?" + v.Encode()
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", friendlyAPIError(resp.StatusCode, body))
	}
	return body, nil
}

func (c *C1Client) doPost(path string, payload any) (json.RawMessage, error) {
	if err := c.ensureToken(); err != nil {
		return nil, err
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+path, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", friendlyAPIError(resp.StatusCode, body))
	}
	return body, nil
}

// friendlyAPIError extracts a user-safe message from an API error response.
func friendlyAPIError(statusCode int, body []byte) string {
	// Try to parse structured error
	var apiErr struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		ErrorDesc string `json:"error_description"`
	}
	if json.Unmarshal(body, &apiErr) == nil {
		if apiErr.ErrorDesc != "" {
			return apiErr.ErrorDesc
		}
		if apiErr.Message != "" {
			// Strip internal details like request-id
			msg := apiErr.Message
			if idx := strings.Index(msg, " (request-id:"); idx > 0 {
				msg = msg[:idx]
			}
			return msg
		}
		if apiErr.Error != "" {
			return apiErr.Error
		}
	}

	// Fallback to status code descriptions
	switch statusCode {
	case 400:
		return "Invalid request — check your parameters"
	case 401:
		return "Authentication failed — check your credentials"
	case 403:
		return "Access denied — your service principal may not have permission"
	case 404:
		return "Resource not found"
	case 429:
		return "Rate limited — too many requests, try again shortly"
	default:
		if statusCode >= 500 {
			return "ConductorOne server error — try again later"
		}
		return fmt.Sprintf("API request failed (HTTP %d)", statusCode)
	}
}

type paginatedResponse struct {
	List          json.RawMessage `json:"list"`
	NextPageToken string          `json:"nextPageToken"`
}

// paginateGet fetches all pages from a GET endpoint. If limit > 0, stops after collecting that many items.
func (c *C1Client) paginateGet(path string, limit int) ([]json.RawMessage, error) {
	var all []json.RawMessage
	pageToken := ""
	for {
		params := map[string]string{"page_size": "100"}
		if pageToken != "" {
			params["page_token"] = pageToken
		}
		body, err := c.doGet(path, params)
		if err != nil {
			return nil, err
		}
		var pr paginatedResponse
		if err := json.Unmarshal(body, &pr); err != nil {
			return nil, err
		}
		var items []json.RawMessage
		if err := json.Unmarshal(pr.List, &items); err != nil {
			return nil, err
		}
		all = append(all, items...)
		if limit > 0 && len(all) >= limit {
			return all[:limit], nil
		}
		if pr.NextPageToken == "" {
			break
		}
		pageToken = pr.NextPageToken
	}
	return all, nil
}

// paginatePost fetches all pages from a POST endpoint. If limit > 0, stops after collecting that many items.
func (c *C1Client) paginatePost(path string, baseBody map[string]any, limit int) ([]json.RawMessage, error) {
	var all []json.RawMessage
	pageToken := ""
	for {
		body := make(map[string]any)
		for k, v := range baseBody {
			body[k] = v
		}
		body["pageSize"] = 100
		body["pageToken"] = pageToken

		respBody, err := c.doPost(path, body)
		if err != nil {
			return nil, err
		}
		var pr paginatedResponse
		if err := json.Unmarshal(respBody, &pr); err != nil {
			return nil, err
		}
		var items []json.RawMessage
		if err := json.Unmarshal(pr.List, &items); err != nil {
			return nil, err
		}
		all = append(all, items...)
		if limit > 0 && len(all) >= limit {
			return all[:limit], nil
		}
		if pr.NextPageToken == "" {
			break
		}
		pageToken = pr.NextPageToken
	}
	return all, nil
}

// --- High-level API methods ---

type AppView map[string]any

func (av AppView) App() AppInfo {
	var info AppInfo
	if nested, ok := av["app"].(map[string]any); ok {
		info.fromMap(nested)
	} else {
		info.fromMap(av)
	}
	return info
}

type AppInfo struct {
	ID          string
	DisplayName string
	UserCount   any
}

func (a *AppInfo) fromMap(m map[string]any) {
	a.ID, _ = m["id"].(string)
	a.DisplayName, _ = m["displayName"].(string)
	a.UserCount = m["userCount"]
}

func (c *C1Client) ListApps() ([]AppView, error) {
	items, err := c.paginateGet("/api/v1/apps", 0)
	if err != nil {
		return nil, err
	}
	return rawToAppViews(items)
}

func (c *C1Client) ListAppUsers(appID string, limit int) ([]json.RawMessage, error) {
	return c.paginateGet(fmt.Sprintf("/api/v1/apps/%s/app_users", appID), limit)
}

func (c *C1Client) ListEntitlements(appID string, limit int) ([]json.RawMessage, error) {
	return c.paginateGet(fmt.Sprintf("/api/v1/apps/%s/entitlements", appID), limit)
}

func (c *C1Client) SearchGrants(appID string, limit int) ([]json.RawMessage, error) {
	return c.paginatePost("/api/v1/search/grants", map[string]any{"appIds": []string{appID}}, limit)
}

func (c *C1Client) ListConnectors(appID string) ([]json.RawMessage, error) {
	return c.paginateGet(fmt.Sprintf("/api/v1/apps/%s/connectors", appID), 0)
}

func (c *C1Client) SearchTasks(body map[string]any, limit int) ([]json.RawMessage, error) {
	return c.paginatePost("/api/v1/search/tasks", body, limit)
}

func (c *C1Client) ListSystemLogEvents(limit int) ([]json.RawMessage, error) {
	return c.paginatePost("/api/v1/systemlog/events", map[string]any{}, limit)
}

func (c *C1Client) SearchGrantFeed(body map[string]any, limit int) ([]json.RawMessage, error) {
	return c.paginatePost("/api/v1/grants/feed", body, limit)
}

func (c *C1Client) SearchPastGrants(appID string, limit int) ([]json.RawMessage, error) {
	return c.paginatePost("/api/v1/search/past-grants", map[string]any{"appIds": []string{appID}}, limit)
}

func (c *C1Client) ListAppOwners(appID string) ([]json.RawMessage, error) {
	return c.paginateGet(fmt.Sprintf("/api/v1/apps/%s/owners", appID), 0)
}

func rawToAppViews(items []json.RawMessage) ([]AppView, error) {
	views := make([]AppView, 0, len(items))
	for _, raw := range items {
		var av AppView
		if err := json.Unmarshal(raw, &av); err != nil {
			return nil, err
		}
		views = append(views, av)
	}
	return views, nil
}
