package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostnameAllowed(t *testing.T) {
	tests := []struct {
		name            string
		hostname        string
		allowedPatterns []string
		want            bool
	}{
		{
			name:            "exact match",
			hostname:        "example.com",
			allowedPatterns: []string{"example.com"},
			want:            true,
		},
		{
			name:            "wildcard subdomain match",
			hostname:        "sub.example.com",
			allowedPatterns: []string{"*.example.com"},
			want:            true,
		},
		{
			name:            "wildcard subdomain no match",
			hostname:        "sub.different.com",
			allowedPatterns: []string{"*.example.com"},
			want:            false,
		},
		{
			name:            "global wildcard match",
			hostname:        "anything.com",
			allowedPatterns: []string{"*"},
			want:            true,
		},
		{
			name:            "multiple patterns with match",
			hostname:        "api.github.com",
			allowedPatterns: []string{"*.example.com", "api.github.com", "*.mattermost.com"},
			want:            true,
		},
		{
			name:            "multiple patterns no match",
			hostname:        "evil.com",
			allowedPatterns: []string{"*.example.com", "api.github.com", "*.mattermost.com"},
			want:            false,
		},
		{
			name:            "empty patterns list",
			hostname:        "example.com",
			allowedPatterns: []string{},
			want:            false,
		},
		{
			name:            "nil patterns list",
			hostname:        "example.com",
			allowedPatterns: nil,
			want:            false,
		},
		{
			name:            "deep subdomain match",
			hostname:        "deep.sub.example.com",
			allowedPatterns: []string{"*.example.com"},
			want:            true,
		},
		{
			name:            "partial suffix no match",
			hostname:        "notexample.com",
			allowedPatterns: []string{"*.example.com"},
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hostnameAllowed(tt.hostname, tt.allowedPatterns)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateRestrictedClient(t *testing.T) {
	// Start a test HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	// Parse the test server URL to get its hostname
	tsURL, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	testHostname := tsURL.Hostname()

	tests := []struct {
		name           string
		allowedHosts   []string
		targetURL      string
		expectError    bool
		errorSubstring string
	}{
		{
			name:         "allowed host",
			allowedHosts: []string{testHostname},
			targetURL:    ts.URL,
			expectError:  false,
		},
		{
			name:           "blocked host",
			allowedHosts:   []string{"allowed.com"},
			targetURL:      ts.URL,
			expectError:    true,
			errorSubstring: "not on allowed list",
		},
		{
			name:         "wildcard allowed",
			allowedHosts: []string{"*"},
			targetURL:    ts.URL,
			expectError:  false,
		},
		{
			name:         "multiple patterns with match",
			allowedHosts: []string{"other.com", testHostname, "another.com"},
			targetURL:    ts.URL,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createRestrictedClient(nil, tt.allowedHosts)

			req, err := http.NewRequest("GET", tt.targetURL, nil)
			assert.NoError(t, err)

			resp, err := client.Do(req)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorSubstring != "" {
					assert.Contains(t, err.Error(), tt.errorSubstring)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if resp != nil {
					resp.Body.Close()
					assert.Equal(t, http.StatusOK, resp.StatusCode)
				}
			}
		})
	}
}

func TestParseAllowedHostnames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple list",
			input:    "example.com,test.com",
			expected: []string{"example.com", "test.com"},
		},
		{
			name:     "with spaces",
			input:    " example.com , test.com ",
			expected: []string{"example.com", "test.com"},
		},
		{
			name:     "with empty entries",
			input:    "example.com,,test.com,",
			expected: []string{"example.com", "test.com"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: nil,
		},
		{
			name:     "with wildcards",
			input:    "*.example.com,api.github.com",
			expected: []string{"*.example.com", "api.github.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAllowedHostnames(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
