package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *StatusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

func (p *Plugin) metricsMiddleware(c *gin.Context) {
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
