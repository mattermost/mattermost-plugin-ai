// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package telemetry

import (
	"fmt"

	"github.com/rudderlabs/analytics-go"
)

type ClientConfig struct {
	WriteKey     string
	DataplaneURL string
	DiagnosticID string
	DefaultProps map[string]any
}

func (c *ClientConfig) isValid() error {
	if c.WriteKey == "" {
		return fmt.Errorf("WriteKey should not be empty")
	}

	if c.DataplaneURL == "" {
		return fmt.Errorf("DataplaneURL should not be empty")
	}

	if c.DiagnosticID == "" {
		return fmt.Errorf("DiagnosticID should not be empty")
	}

	return nil
}

type Client struct {
	config ClientConfig
	client analytics.Client
}

func NewClient(config ClientConfig) (*Client, error) {
	if err := config.isValid(); err != nil {
		return nil, fmt.Errorf("telemetry: config validation failed: %w", err)
	}

	return &Client{
		config: config,
		client: analytics.New(config.WriteKey, config.DataplaneURL),
	}, nil
}

func (c *Client) Track(event string, props map[string]any, ctx *analytics.Context) error {
	if props == nil {
		props = map[string]any{}
	}

	for k, v := range c.config.DefaultProps {
		props[k] = v
	}

	if err := c.client.Enqueue(analytics.Track{
		Event:      event,
		UserId:     c.config.DiagnosticID,
		Properties: props,
		Context:    ctx,
	}); err != nil {
		return fmt.Errorf("telemetry: failed to track event: %w", err)
	}

	return nil
}

func (c *Client) Close() error {
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("telemetry: failed to close client: %w", err)
	}
	return nil
}
