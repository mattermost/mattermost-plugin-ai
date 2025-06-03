// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	"fmt"
	"sync"
	"time"

	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

// ClientManager manages MCP clients for multiple users
type ClientManager struct {
	config        Config
	log           pluginapi.LogService
	clientsMu     sync.RWMutex
	clients       map[string]*UserClient // Map of userID to UserClient
	cleanupTicker *time.Ticker
	closeChan     chan struct{}
	clientTimeout time.Duration
}

// Config contains the configuration for the MCP clients
type Config struct {
	Enabled            bool                    `json:"enabled"`
	Servers            map[string]ServerConfig `json:"servers"`
	IdleTimeoutMinutes int                     `json:"idleTimeoutMinutes"`
}

// NewClientManager creates a new MCP client manager
func NewClientManager(config Config, log pluginapi.LogService) *ClientManager {
	manager := &ClientManager{
		log: log,
	}
	manager.ReInit(config)
	return manager
}

// cleanupInactiveClients periodically checks for and closes inactive client connections
func (m *ClientManager) cleanupInactiveClients() {
	for {
		select {
		case <-m.cleanupTicker.C:
			m.clientsMu.Lock()
			now := time.Now()
			for userID, client := range m.clients {
				if now.Sub(client.lastActivity) > m.clientTimeout {
					m.log.Debug("Closing inactive MCP client", "userID", userID, "idleTime", now.Sub(client.lastActivity))
					client.Close()
					delete(m.clients, userID)
				}
			}
			m.clientsMu.Unlock()
		case <-m.closeChan:
			m.cleanupTicker.Stop()
			return
		}
	}
}

// ReInit re-initializes the client manager with a new configuration
func (m *ClientManager) ReInit(config Config) {
	m.Close()

	if config.IdleTimeoutMinutes <= 0 {
		config.IdleTimeoutMinutes = 30
	}

	m.config = config
	m.clients = make(map[string]*UserClient)
	m.clientTimeout = time.Duration(config.IdleTimeoutMinutes) * time.Minute
	m.closeChan = make(chan struct{})

	// Start cleanup ticker to remove inactive clients
	m.cleanupTicker = time.NewTicker(5 * time.Minute)
	go m.cleanupInactiveClients()
}

// Close closes the client manager and all managed clients
// The client manger should not be used after Close is called
func (m *ClientManager) Close() {
	// If already closed, do nothing
	if m.closeChan == nil {
		return
	}
	// Stop the cleanup goroutine
	close(m.closeChan)
	m.closeChan = nil
	m.cleanupTicker.Stop()

	// Close all client connections
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	for _, client := range m.clients {
		client.Close()
	}

	// Clear the clients map
	m.clients = make(map[string]*UserClient)
}

// createAndStoreUserClient creates a new UserClient instance and stores it in the manager
func (m *ClientManager) createAndStoreUserClient(userID string) (*UserClient, error) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	// Check again in case another goroutine created the client while we were waiting for the lock
	client, exists := m.clients[userID]
	if exists {
		client.lastActivity = time.Now()
		return client, nil
	}

	// Create a new user client
	userClient := &UserClient{
		log:          m.log,
		clients:      make(map[string]*ServerConnection),
		toolDefs:     make(map[string]ToolDefinition),
		lastActivity: time.Now(),
		userID:       userID,
	}

	// Let user client connect to all servers
	if err := userClient.ConnectToAllServers(m.config.Servers); err != nil {
		return nil, fmt.Errorf("failed to initialize MCP client for user %s: %w", userID, err)
	}

	m.clients[userID] = userClient

	return userClient, nil
}

// getClientForUser gets or creates an MCP client for a specific user
func (m *ClientManager) getClientForUser(userID string) (*UserClient, error) {
	m.clientsMu.RLock()
	client, exists := m.clients[userID]
	m.clientsMu.RUnlock()
	if exists {
		client.lastActivity = time.Now()
		return client, nil
	}

	newUserClient, err := m.createAndStoreUserClient(userID)
	if err != nil {
		return nil, err
	}

	return newUserClient, nil
}

// GetToolsForUser returns the tools available for a specific user
func (m *ClientManager) GetToolsForUser(userID string) ([]llm.Tool, error) {
	// If not enabled or no servers configured return no tools
	if !m.config.Enabled || len(m.config.Servers) == 0 {
		return []llm.Tool{}, nil
	}

	// Get or create client for this user
	userClient, err := m.getClientForUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP client for user %s: %w", userID, err)
	}

	// Return the user's tools
	return userClient.GetTools(), nil
}
