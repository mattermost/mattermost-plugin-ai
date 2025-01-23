// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/shared/httpservice"
)

// Hostname matching rules:
// 1. Exact matches: A hostname must exactly match an allowed pattern
// 2. Wildcard matches: Patterns starting with "*." only match subdomains
//    - "*.example.com" matches "sub.example.com" and "deep.sub.example.com"
//    - "*.example.com" does NOT match "example.com" itself
// 3. Global wildcard: A pattern of "*" matches all hostnames
// 4. IPv6 zones: Hostnames containing zone IDs (%) require exact matches
//    - Wildcard patterns never match hostnames containing zone IDs

// hostnameAllowed checks if a hostname matches any of the allowed patterns
func hostnameAllowed(hostname string, allowedPatterns []string) bool {
	for _, pattern := range allowedPatterns {
		if pattern == "*" {
			return true
		}

		if strings.HasPrefix(pattern, "*.") {
			// Reject hosts with ipv6 zones
			if strings.ContainsAny(hostname, "%") {
				return false
			}

			suffix := pattern[1:] // Remove the *
			if strings.HasSuffix(hostname, suffix) {
				return true
			}
		} else if hostname == pattern {
			return true
		}
	}
	return false
}

// parseAllowedHostnames splits the comma-separated string into cleaned hostname patterns
func parseAllowedHostnames(allowedHostnames string) []string {
	allowedHostnames = strings.TrimSpace(allowedHostnames)
	if allowedHostnames == "" {
		return nil
	}

	patterns := strings.Split(allowedHostnames, ",")
	cleaned := make([]string, 0, len(patterns))

	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}

	return cleaned
}

// restrictedTransport wraps an http.RoundTripper to enforce hostname restrictions
type restrictedTransport struct {
	wrapped      http.RoundTripper
	allowedHosts []string
}

func (t *restrictedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL == nil {
		return nil, fmt.Errorf("request has no URL")
	}

	hostname := req.URL.Hostname()
	if !hostnameAllowed(hostname, t.allowedHosts) {
		return nil, fmt.Errorf("hostname %q is not on allowed list, add this host to allowed upstream hosts", hostname)
	}

	return t.wrapped.RoundTrip(req)
}

// wrapTransportWithHostRestrictions wraps an existing transport with hostname restrictions
func wrapTransportWithHostRestrictions(base http.RoundTripper, allowedHostnames []string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	return &restrictedTransport{
		wrapped:      base,
		allowedHosts: allowedHostnames,
	}
}

// createRestrictedClient creates an http.Client with hostname restrictions
func createRestrictedClient(client *http.Client, allowedHostnames []string) *http.Client {
	if client == nil {
		client = &http.Client{}
	}

	// Wrap the existing transport or create new one
	client.Transport = wrapTransportWithHostRestrictions(client.Transport, allowedHostnames)

	return client
}

func (p *Plugin) createExternalHTTPClient() *http.Client {
	baseClient := httpservice.MakeHTTPServicePlugin(p.API).MakeClient(false)
	return createRestrictedClient(baseClient, parseAllowedHostnames(p.getConfiguration().AllowedUpstreamHostnames))
}
