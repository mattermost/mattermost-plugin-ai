// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	cacheKeyPrefix = "mcp_tools_cache_v1_"
	cacheTTL       = 8 * 24 * time.Hour // 8 days

	serverProfileKeyPrefix = "mcp_server_profile_v1_"
	// serverProfileRefreshAge bounds how long an unchanged profile goes
	// without being rewritten, so active servers never reach cacheTTL.
	serverProfileRefreshAge = 24 * time.Hour
)

// ServerProfile is how a remote MCP server identified itself when a session
// was initialized. It lets users who have not connected yet be told what the
// server is for, so it holds only server-wide identity, never text that may be
// written for the connected user (such as initialize instructions). Fields are
// sanitized and length-capped before they are stored.
type ServerProfile struct {
	ServerURL   string    `json:"server_url"`
	Name        string    `json:"name,omitempty"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

func (p ServerProfile) sameContent(other ServerProfile) bool {
	p.Timestamp, other.Timestamp = time.Time{}, time.Time{}
	return p == other
}

// CachedTools represents a cached set of tools for a specific MCP server
type CachedTools struct {
	Tools      map[string]*mcp.Tool `json:"tools"`
	Timestamp  time.Time            `json:"timestamp"`
	ServerURL  string               `json:"server_url"`
	ServerName string               `json:"server_name"`
}

// KVStore interface for key-value operations
type KVStore interface {
	Get(key string, o any) error
	Set(key string, value any, options ...pluginapi.KVSetOption) (bool, error)
	Delete(key string) error
	ListKeys(page, count int, options ...pluginapi.ListKeysOption) ([]string, error)
}

// Logger interface for logging operations
type Logger interface {
	Debug(msg string, keyValuePairs ...any)
	Info(msg string, keyValuePairs ...any)
	Warn(msg string, keyValuePairs ...any)
	Error(msg string, keyValuePairs ...any)
}

// ToolsCache manages the global cache of MCP tools across all users
// It uses the KV store directly for HA mode compatibility
type ToolsCache struct {
	kvAPI KVStore
	log   Logger
}

// NewToolsCache creates a new ToolsCache instance
func NewToolsCache(kvAPI KVStore, log Logger) *ToolsCache {
	return &ToolsCache{
		kvAPI: kvAPI,
		log:   log,
	}
}

// GetTools retrieves cached tools for a server, returns nil if missing
func (tc *ToolsCache) GetTools(serverID string) map[string]*mcp.Tool {
	cached := tc.getEntry(serverID)
	if cached == nil {
		return nil
	}
	return cached.Tools
}

// GetToolsForURL is GetTools restricted to an entry recorded for serverURL, so
// a server renamed onto a different URL never reports the old URL's tools.
func (tc *ToolsCache) GetToolsForURL(serverID, serverURL string) map[string]*mcp.Tool {
	cached := tc.getEntry(serverID)
	if cached == nil || cached.ServerURL != serverURL {
		return nil
	}
	return cached.Tools
}

func (tc *ToolsCache) getEntry(serverID string) *CachedTools {
	key := tc.buildCacheKey(serverID)

	var cached CachedTools
	err := tc.kvAPI.Get(key, &cached)
	if err != nil {
		tc.log.Error("Failed to retrieve tools cache from KV store", "serverID", serverID, "error", err)
		return nil
	}

	// Get() returns no error for non-existent keys, just doesn't populate the struct
	// Check if data was actually populated
	if cached.Tools == nil || cached.Timestamp.IsZero() {
		tc.log.Debug("Cache miss for server", "serverID", serverID)
		return nil
	}

	tc.log.Debug("Cache hit for server", "serverID", serverID, "tools", len(cached.Tools))
	return &cached
}

// GetServerProfile returns the stored profile for a server recorded at
// serverURL, or nil when none is stored.
func (tc *ToolsCache) GetServerProfile(serverID, serverURL string) *ServerProfile {
	var profile ServerProfile
	if err := tc.kvAPI.Get(serverProfileKeyPrefix+serverID, &profile); err != nil {
		tc.log.Warn("Failed to retrieve MCP server profile from KV store", "serverID", serverID, "error", err)
		return nil
	}
	if profile.Timestamp.IsZero() || profile.ServerURL != serverURL {
		return nil
	}
	return &profile
}

// SetServerProfile stores profile for a server unless an identical profile was
// stored recently.
func (tc *ToolsCache) SetServerProfile(serverID string, profile ServerProfile, now time.Time) error {
	if existing := tc.GetServerProfile(serverID, profile.ServerURL); existing != nil &&
		existing.sameContent(profile) && now.Sub(existing.Timestamp) < serverProfileRefreshAge {
		return nil
	}

	profile.Timestamp = now
	success, err := tc.kvAPI.Set(serverProfileKeyPrefix+serverID, profile, pluginapi.SetExpiry(cacheTTL))
	if err != nil {
		return fmt.Errorf("failed to persist MCP server profile: %w", err)
	}
	if !success {
		return fmt.Errorf("MCP server profile was not persisted for server %s", serverID)
	}
	return nil
}

// SetTools updates the cache for a server and persists to KV store
func (tc *ToolsCache) SetTools(serverID string, serverName string, serverURL string, tools map[string]*mcp.Tool, timestamp time.Time) error {
	cached := &CachedTools{
		Tools:      tools,
		Timestamp:  timestamp,
		ServerURL:  serverURL,
		ServerName: serverName,
	}

	// Persist directly to KV store
	// Set returns (false, err) if DB error occurred
	// Set returns (false, nil) if the value was not set
	// Set returns (true, nil) if the value was set
	key := tc.buildCacheKey(serverID)
	success, err := tc.kvAPI.Set(key, cached, pluginapi.SetExpiry(cacheTTL))
	if err != nil {
		tc.log.Error("Failed to persist tools cache to KV store (DB error)", "serverID", serverID, "error", err)
		return fmt.Errorf("failed to persist cache: %w", err)
	}
	if !success {
		tc.log.Warn("Tools cache was not persisted to KV store", "serverID", serverID)
		return fmt.Errorf("cache was not persisted for server %s", serverID)
	}

	tc.log.Debug("Updated tools cache for server", "serverID", serverID, "tools", len(tools))
	return nil
}

// InvalidateServer removes a server's cache entry from KV store
// Delete returns no error for non-existent keys, only errors on actual failures
func (tc *ToolsCache) InvalidateServer(serverID string) error {
	key := tc.buildCacheKey(serverID)
	err := tc.kvAPI.Delete(key)
	if err != nil {
		tc.log.Error("Failed to delete tools cache from KV store", "serverID", serverID, "error", err)
		return fmt.Errorf("failed to delete cache: %w", err)
	}

	tc.log.Debug("Invalidated tools cache for server (deleted if existed)", "serverID", serverID)
	return nil
}

// InvalidateServerProfile removes a server's stored profile from KV store.
func (tc *ToolsCache) InvalidateServerProfile(serverID string) error {
	if err := tc.kvAPI.Delete(serverProfileKeyPrefix + serverID); err != nil {
		return fmt.Errorf("failed to delete MCP server profile: %w", err)
	}
	return nil
}

// ClearAll removes all cached tools and server profiles from KV store. The
// returned count covers cached tool lists only.
func (tc *ToolsCache) ClearAll() (int, error) {
	tc.log.Info("Clearing all MCP tools cache entries")

	keys, err := tc.kvAPI.ListKeys(0, 1000)
	if err != nil {
		tc.log.Error("Failed to list cache keys from KV store", "error", err)
		return 0, fmt.Errorf("failed to list cache keys: %w", err)
	}

	clearedCount := 0
	for _, key := range keys {
		isToolsEntry := strings.HasPrefix(key, cacheKeyPrefix)
		if !isToolsEntry && !strings.HasPrefix(key, serverProfileKeyPrefix) {
			continue
		}
		if err := tc.kvAPI.Delete(key); err != nil {
			tc.log.Warn("Failed to delete cache key", "key", key, "error", err)
			continue
		}
		if isToolsEntry {
			clearedCount++
		}
	}

	tc.log.Info("Cleared MCP tools cache", "clearedCount", clearedCount)
	return clearedCount, nil
}

// buildCacheKey constructs the KV store key for a server
func (tc *ToolsCache) buildCacheKey(serverID string) string {
	return cacheKeyPrefix + serverID
}
