package pinboard

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// TestSuccessfulBookmarkCreation verifies that a bookmark is created successfully
// with all the correct query parameters sent via GET request.
func TestSuccessfulBookmarkCreation(t *testing.T) {
	var receivedQueryParams url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a GET request
		if r.Method != http.MethodGet {
			t.Errorf("expected GET request, got %s", r.Method)
		}

		// Extract query parameters
		receivedQueryParams = r.URL.Query()

		// Verify request body is empty (GET request has no body)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if len(body) > 0 {
			t.Errorf("expected empty request body for GET, got %q", string(body))
		}

		// Send v1 API success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result_code": "done"})
	}))
	defer server.Close()

	client := NewClient("test_auth_token")
	client.baseURL = server.URL

	bookmark := Bookmark{
		URL:         "https://example.com",
		Title:       "Example Site",
		Description: "A test bookmark",
		Tags:        []string{"test", "example"},
		Private:     true,
		ToRead:      false,
	}

	err := client.AddBookmark(context.Background(), bookmark)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify query parameters match v1 API format
	expectedFields := map[string]string{
		"url":         "https://example.com",
		"description": "Example Site",        // v1 API uses "description" for the title
		"extended":    "A test bookmark",     // v1 API uses "extended" for the description
		"tags":        "test example",
		"shared":      "no",                   // Private=true means shared=no
		"toread":      "no",                   // ToRead=false means toread=no
		"replace":     "yes",                  // Always yes for idempotent exports
		"format":      "json",
	}

	for field, expected := range expectedFields {
		actual := receivedQueryParams.Get(field)
		if actual != expected {
			t.Errorf("field %s: expected %q, got %q", field, expected, actual)
		}
	}
}

// TestAuthTokenInQueryParam verifies that the auth token is included in the query parameter.
func TestAuthTokenInQueryParam(t *testing.T) {
	expectedToken := "user:ABC123DEF456"
	var receivedToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract auth token from query parameter
		receivedToken = r.URL.Query().Get("auth_token")

		// Send v1 API success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result_code": "done"})
	}))
	defer server.Close()

	client := NewClient(expectedToken)
	client.baseURL = server.URL

	bookmark := Bookmark{
		URL:   "https://example.com",
		Title: "Test",
	}

	err := client.AddBookmark(context.Background(), bookmark)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if receivedToken != expectedToken {
		t.Errorf("expected auth token %q, got %q", expectedToken, receivedToken)
	}
}

// TestTagsAreSpaceSeparated verifies that multiple tags are joined with spaces in the query parameters.
func TestTagsAreSpaceSeparated(t *testing.T) {
	var receivedTags string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract tags from query parameters
		receivedTags = r.URL.Query().Get("tags")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result_code": "done"})
	}))
	defer server.Close()

	client := NewClient("test_token")
	client.baseURL = server.URL

	bookmark := Bookmark{
		URL:   "https://example.com",
		Title: "Test",
		Tags:  []string{"golang", "api", "testing"},
	}

	err := client.AddBookmark(context.Background(), bookmark)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedTags := "golang api testing"
	if receivedTags != expectedTags {
		t.Errorf("expected tags %q, got %q", expectedTags, receivedTags)
	}
}

// TestBooleanFields verifies that Private=true maps to shared=no and ToRead=false maps to toread=no.
func TestBooleanFields(t *testing.T) {
	var receivedQueryParams url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQueryParams = r.URL.Query()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result_code": "done"})
	}))
	defer server.Close()

	client := NewClient("test_token")
	client.baseURL = server.URL

	bookmark := Bookmark{
		URL:     "https://example.com",
		Title:   "Test",
		Private: true,
		ToRead:  false,
	}

	err := client.AddBookmark(context.Background(), bookmark)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// v1 API uses "shared" instead of "private", with inverted logic
	if receivedQueryParams.Get("shared") != "no" {
		t.Errorf("expected shared=no (when Private=true), got %q", receivedQueryParams.Get("shared"))
	}
	// v1 API uses "yes"/"no" instead of "true"/"false"
	if receivedQueryParams.Get("toread") != "no" {
		t.Errorf("expected toread=no, got %q", receivedQueryParams.Get("toread"))
	}
}

// TestServerErrorResponse verifies that server errors return meaningful error messages.
func TestServerErrorResponse(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedError  string
	}{
		{
			name:         "HTTP 500 error",
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"result_code": "internal server error"}`,
			expectedError: "API returned status 500",
		},
		{
			name:         "HTTP 401 unauthorised",
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"result_code": "invalid auth token"}`,
			expectedError: "API returned status 401",
		},
		{
			name:         "Error result_code in response",
			statusCode:   http.StatusOK,
			responseBody: `{"result_code": "something went wrong"}`,
			expectedError: "API error: result_code was \"something went wrong\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := NewClient("test_token")
			client.baseURL = server.URL

			bookmark := Bookmark{
				URL:   "https://example.com",
				Title: "Test",
			}

			err := client.AddBookmark(context.Background(), bookmark)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !containsSubstring(err.Error(), tt.expectedError) {
				t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Error())
			}
		})
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestFieldMapping verifies the confusing v1 API field mapping:
// - "description" query param should contain the bookmark Title
// - "extended" query param should contain the bookmark Description
func TestFieldMapping(t *testing.T) {
	var receivedQueryParams url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQueryParams = r.URL.Query()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result_code": "done"})
	}))
	defer server.Close()

	client := NewClient("test_token")
	client.baseURL = server.URL

	bookmark := Bookmark{
		URL:         "https://example.com",
		Title:       "My Bookmark Title",
		Description: "My bookmark description with more details",
	}

	err := client.AddBookmark(context.Background(), bookmark)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the confusing mapping: v1 "description" = our Title
	if receivedQueryParams.Get("description") != "My Bookmark Title" {
		t.Errorf("expected description param to contain Title %q, got %q",
			"My Bookmark Title", receivedQueryParams.Get("description"))
	}

	// Verify the confusing mapping: v1 "extended" = our Description
	if receivedQueryParams.Get("extended") != "My bookmark description with more details" {
		t.Errorf("expected extended param to contain Description %q, got %q",
			"My bookmark description with more details", receivedQueryParams.Get("extended"))
	}
}

// TestRateLimiting verifies that the client enforces a minimum delay between API calls.
func TestRateLimiting(t *testing.T) {
	callTimes := []time.Time{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callTimes = append(callTimes, time.Now())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result_code": "done"})
	}))
	defer server.Close()

	client := NewClient("test_token")
	client.baseURL = server.URL
	// Use a shorter delay for testing (100ms instead of 3 seconds)
	client.minDelay = 100 * time.Millisecond

	bookmark := Bookmark{
		URL:   "https://example.com",
		Title: "Test",
	}

	// Make first call
	err := client.AddBookmark(context.Background(), bookmark)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Make second call immediately
	err = client.AddBookmark(context.Background(), bookmark)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Verify we have two calls
	if len(callTimes) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(callTimes))
	}

	// Calculate the time between calls
	timeBetween := callTimes[1].Sub(callTimes[0])

	// The second call should have been delayed by at least minDelay
	if timeBetween < client.minDelay {
		t.Errorf("expected at least %v between calls, got %v", client.minDelay, timeBetween)
	}
}

// TestRateLimitingRespectsContext verifies that cancelling the context during
// a rate-limit wait returns promptly instead of blocking.
func TestRateLimitingRespectsContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result_code": "done"})
	}))
	defer server.Close()

	client := NewClient("test_token")
	client.baseURL = server.URL
	client.minDelay = 5 * time.Second

	bookmark := Bookmark{URL: "https://example.com", Title: "Test"}

	// First call succeeds normally.
	if err := client.AddBookmark(context.Background(), bookmark); err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Cancel the context before the second call's rate-limit wait finishes.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := client.AddBookmark(ctx, bookmark)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if elapsed >= 1*time.Second {
		t.Errorf("expected prompt return on cancel, but waited %v", elapsed)
	}
}

// TestGetBookmarkURLsByTag verifies that bookmarks are fetched and URLs are returned correctly.
func TestGetBookmarkURLsByTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send response with 3 bookmarks
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := []map[string]interface{}{
			{"href": "https://github.com/foo/bar", "description": "foo/bar"},
			{"href": "https://example.com/test", "description": "test"},
			{"href": "https://golang.org/doc", "description": "Go docs"},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient("test_token")
	client.baseURL = server.URL

	urls, err := client.GetBookmarkURLsByTag(context.Background(), "golang")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify all 3 URLs are in the returned map
	expectedURLs := []string{
		"https://github.com/foo/bar",
		"https://example.com/test",
		"https://golang.org/doc",
	}

	if len(urls) != 3 {
		t.Errorf("expected 3 URLs in map, got %d", len(urls))
	}

	for _, url := range expectedURLs {
		if !urls[url] {
			t.Errorf("expected URL %q to be in map", url)
		}
	}
}

// TestGetBookmarkURLsByTagEmpty verifies that an empty array returns an empty map.
func TestGetBookmarkURLsByTagEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send empty array response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient("test_token")
	client.baseURL = server.URL

	urls, err := client.GetBookmarkURLsByTag(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(urls) != 0 {
		t.Errorf("expected empty map, got %d entries", len(urls))
	}
}

// TestGetBookmarkURLsByTagSendsCorrectParams verifies that the correct query parameters are sent.
func TestGetBookmarkURLsByTagSendsCorrectParams(t *testing.T) {
	var receivedQueryParams url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQueryParams = r.URL.Query()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient("test_auth_token")
	client.baseURL = server.URL

	_, err := client.GetBookmarkURLsByTag(context.Background(), "golang")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify tag parameter
	if receivedQueryParams.Get("tag") != "golang" {
		t.Errorf("expected tag=golang, got %q", receivedQueryParams.Get("tag"))
	}

	// Verify format parameter
	if receivedQueryParams.Get("format") != "json" {
		t.Errorf("expected format=json, got %q", receivedQueryParams.Get("format"))
	}

	// Verify auth_token parameter
	if receivedQueryParams.Get("auth_token") != "test_auth_token" {
		t.Errorf("expected auth_token=test_auth_token, got %q", receivedQueryParams.Get("auth_token"))
	}
}

// TestGetBookmarkURLsByTagHTTPError verifies that HTTP errors are handled meaningfully.
func TestGetBookmarkURLsByTagHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	client := NewClient("test_token")
	client.baseURL = server.URL

	_, err := client.GetBookmarkURLsByTag(context.Background(), "golang")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify the error message contains meaningful information
	if !containsSubstring(err.Error(), "500") {
		t.Errorf("expected error to contain status code 500, got %q", err.Error())
	}
}
