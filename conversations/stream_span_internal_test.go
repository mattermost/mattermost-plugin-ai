// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"context"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/telemetry"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// A canceled reply stops reading mid-stream; releasing must still export the
// span and stop the forwarder instead of leaving it blocked on a send.
func TestEndSpanOnStreamCloseReleaseAfterAbandon(t *testing.T) {
	previous := otel.GetTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})

	upstream := make(chan llm.TextStreamEvent, 2)
	upstream <- llm.TextStreamEvent{Type: llm.EventTypeText, Value: "a"}
	upstream <- llm.TextStreamEvent{Type: llm.EventTypeText, Value: "b"}
	// upstream is never closed and never ends: the producer keeps going.

	_, span := telemetry.Tracer().Start(context.Background(), "process dm request")
	wrapped, release := endSpanOnStreamClose(&llm.TextStreamResult{Stream: upstream}, span)

	<-wrapped.Stream // the consumer reads one event, then is canceled
	require.Empty(t, exporter.GetSpans(), "span must stay open while the reply streams")

	release()

	require.Eventually(t, func() bool { return len(exporter.GetSpans()) == 1 }, time.Second, 5*time.Millisecond)
	select {
	case _, open := <-wrapped.Stream:
		if open {
			// At most the one event already in flight; then it must close.
			_, open = <-wrapped.Stream
		}
		require.False(t, open, "forwarder must stop after release")
	case <-time.After(time.Second):
		t.Fatal("forwarder still blocked after release")
	}
}
