package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func (p *Plugin) ServeMetrics(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.metricsHandler.ServeHTTP(w, r)
}

func (p *Plugin) GetMetrics() metrics.Metrics {
	return p.metricsService
}

func (p *Plugin) metricsMiddleware(c *gin.Context) {
	metrics := p.GetMetrics()
	if metrics == nil {
		c.Next()
		return
	}
	p.GetMetrics().IncrementHTTPRequests()
	now := time.Now()

	c.Next()

	elapsed := float64(time.Since(now)) / float64(time.Second)

	status := c.Writer.Status()

	if status < 200 || status > 299 {
		p.GetMetrics().IncrementHTTPErrors()
	}

	endpoint := c.HandlerName()
	p.GetMetrics().ObserveAPIEndpointDuration(endpoint, c.Request.Method, strconv.Itoa(status), elapsed)
}
