package main

import (
	"net/http"

	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func (p *Plugin) ServeMetrics(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.metricsHandler.ServeHTTP(w, r)
}

func (p *Plugin) GetMetrics() metrics.Metrics {
	return p.metricsService
}
