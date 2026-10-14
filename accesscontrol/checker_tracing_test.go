// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"context"
	"errors"
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/telemetry"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func installTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	previous := otel.GetTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})
	return exporter
}

func spanAttr(s tracetest.SpanStub, key string) (any, bool) {
	for _, a := range s.Attributes {
		if string(a.Key) == key {
			return a.Value.AsInterface(), true
		}
	}
	return nil, false
}

func TestCheckerSpanStatus(t *testing.T) {
	serverID := model.NewId()
	agentID := model.NewId()
	tests := []struct {
		name        string
		client      *stubDecisionClient
		check       func(c *Checker) error
		wantDenied  bool
		wantErrCode codes.Code
	}{
		{
			name:   "mcp deny is not an error",
			client: &stubDecisionClient{decision: denyDecision()},
			check: func(c *Checker) error {
				return c.CanUseMCPServer(WithResourceName(context.Background(), "jira"), testUserID, serverID)
			},
			wantDenied:  true,
			wantErrCode: codes.Unset,
		},
		{
			name:   "agent deny is not an error",
			client: &stubDecisionClient{decision: denyDecision()},
			check: func(c *Checker) error {
				return c.CanUseAgent(context.Background(), testUserID, &llm.BotConfig{ID: agentID}, nil)
			},
			wantDenied:  true,
			wantErrCode: codes.Unset,
		},
		{
			name:   "evaluation failure is an error",
			client: &stubDecisionClient{err: errors.New("rpc down")},
			check: func(c *Checker) error {
				return c.CanUseMCPServer(context.Background(), testUserID, serverID)
			},
			wantDenied:  true,
			wantErrCode: codes.Error,
		},
		{
			name:   "allow",
			client: &stubDecisionClient{decision: allowDecision()},
			check: func(c *Checker) error {
				return c.CanUseMCPServer(context.Background(), testUserID, serverID)
			},
			wantErrCode: codes.Unset,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exporter := installTestTracer(t)
			err := tc.check(New(tc.client, nil, NoMCPServerIDs, nil))
			require.Equal(t, tc.wantDenied, errors.Is(err, ErrAccessDenied))

			spans := exporter.GetSpans()
			require.Len(t, spans, 1)
			require.Equal(t, tc.wantErrCode, spans[0].Status.Code)
			denied, _ := spanAttr(spans[0], string(telemetry.ABACDenied))
			require.Equal(t, tc.wantDenied, denied == true)
		})
	}
}

func TestCheckerSpanCarriesResourceName(t *testing.T) {
	exporter := installTestTracer(t)
	c := New(&stubDecisionClient{decision: denyDecision()}, nil, NoMCPServerIDs, nil)

	_ = c.CanUseMCPServer(WithResourceName(context.Background(), "jira"), testUserID, model.NewId())

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	name, ok := spanAttr(spans[0], string(telemetry.ABACResourceName))
	require.True(t, ok)
	require.Equal(t, "jira", name)
}
