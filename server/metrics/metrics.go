//go:generate mockery --name=Metrics
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const (
	MetricsNamespace       = "copilot"
	MetricsSubsystemSystem = "system"
	MetricsSubsystemApp    = "app"
	MetricsSubsystemHTTP   = "http"
	MetricsSubsystemAPI    = "api"
	MetricsSubsystemEvents = "events"
	MetricsSubsystemDB     = "db"
	MetricsSubsystemLLM    = "llm"

	MetricsCloudInstallationLabel = "installationId"
	MetricsVersionLabel           = "version"

	ActionSourceMSTeams     = "msteams"
	ActionSourceMattermost  = "mattermost"
	ActionCreated           = "created"
	ActionUpdated           = "updated"
	ActionDeleted           = "deleted"
	ReactionSetAction       = "set"
	ReactionUnsetAction     = "unset"
	SubscriptionRefreshed   = "refreshed"
	SubscriptionConnected   = "connected"
	SubscriptionReconnected = "reconnected"
)

type Metrics interface {
	GetRegistry() *prometheus.Registry

	ObserveAPIEndpointDuration(handler, method, statusCode string, elapsed float64)

	IncrementHTTPRequests()
	IncrementHTTPErrors()

	ObserveLLMRequest(llmID string)
	ObserveLLMTokensSent(llmID string, count int64)
	ObserveLLMTokensReceived(llmID string, count int64)
	ObserveLLMBytesSent(llmID string, count int64)
	ObserveLLMBytesReceived(llmID string, count int64)
}

type InstanceInfo struct {
	InstallationID      string
	ConnectedUsersLimit int
	PluginVersion       string
}

// metrics used to instrumentate metrics in prometheus.
type metrics struct {
	registry *prometheus.Registry

	pluginStartTime prometheus.Gauge
	pluginInfo      prometheus.Gauge

	apiTime *prometheus.HistogramVec

	httpRequestsTotal prometheus.Counter
	httpErrorsTotal   prometheus.Counter

	llmRequestsTotal  *prometheus.CounterVec
	llmTokensSent     *prometheus.CounterVec
	llmTokensReceived *prometheus.CounterVec
	llmBytesSent      *prometheus.CounterVec
	llmBytesReceived  *prometheus.CounterVec
}

// NewMetrics Factory method to create a new metrics collector.
func NewMetrics(info InstanceInfo) Metrics {
	m := &metrics{}

	m.registry = prometheus.NewRegistry()
	options := collectors.ProcessCollectorOpts{
		Namespace: MetricsNamespace,
	}
	m.registry.MustRegister(collectors.NewProcessCollector(options))
	m.registry.MustRegister(collectors.NewGoCollector())

	additionalLabels := map[string]string{}
	if info.InstallationID != "" {
		additionalLabels[MetricsCloudInstallationLabel] = info.InstallationID
	}

	m.pluginStartTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemSystem,
		Name:        "plugin_start_timestamp_seconds",
		Help:        "The time the plugin started.",
		ConstLabels: additionalLabels,
	})
	m.pluginStartTime.SetToCurrentTime()
	m.registry.MustRegister(m.pluginStartTime)

	m.pluginInfo = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: MetricsNamespace,
		Subsystem: MetricsSubsystemSystem,
		Name:      "plugin_info",
		Help:      "The plugin version.",
		ConstLabels: map[string]string{
			MetricsCloudInstallationLabel: info.InstallationID,
			MetricsVersionLabel:           info.PluginVersion,
		},
	})
	m.pluginInfo.Set(1)
	m.registry.MustRegister(m.pluginInfo)

	m.apiTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   MetricsNamespace,
			Subsystem:   MetricsSubsystemAPI,
			Name:        "time_seconds",
			Help:        "Time to execute the api handler",
			ConstLabels: additionalLabels,
		},
		[]string{"handler", "method", "status_code"},
	)
	m.registry.MustRegister(m.apiTime)

	m.httpRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemHTTP,
		Name:        "requests_total",
		Help:        "The total number of http API requests.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.httpRequestsTotal)

	m.httpErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemHTTP,
		Name:        "errors_total",
		Help:        "The total number of http API errors.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.httpErrorsTotal)

	m.llmRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemLLM,
		Name:        "requests_total",
		Help:        "The total number of LLM requets made.",
		ConstLabels: additionalLabels,
	}, []string{"llm_name"})
	m.registry.MustRegister(m.llmRequestsTotal)

	m.llmTokensSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemLLM,
		Name:        "tokens_sent_total",
		Help:        "The total number of tokens sent.",
		ConstLabels: additionalLabels,
	}, []string{"llm_name"})
	m.registry.MustRegister(m.llmTokensSent)

	m.llmTokensReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemLLM,
		Name:        "tokens_received_total",
		Help:        "The total number of tokens received.",
		ConstLabels: additionalLabels,
	}, []string{"llm_name"})
	m.registry.MustRegister(m.llmTokensReceived)

	m.llmBytesSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemLLM,
		Name:        "bytes_sent_total",
		Help:        "The total number of bytes sent.",
		ConstLabels: additionalLabels,
	}, []string{"llm_name"})
	m.registry.MustRegister(m.llmBytesSent)

	m.llmBytesReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemLLM,
		Name:        "bytes_received_total",
		Help:        "The total number of bytes received.",
		ConstLabels: additionalLabels,
	}, []string{"llm_name"})
	m.registry.MustRegister(m.llmBytesReceived)

	return m
}

func (m *metrics) GetRegistry() *prometheus.Registry {
	return m.registry
}

func (m *metrics) ObserveAPIEndpointDuration(handler, method, statusCode string, elapsed float64) {
	if m != nil {
		m.apiTime.With(prometheus.Labels{"handler": handler, "method": method, "status_code": statusCode}).Observe(elapsed)
	}
}

func (m *metrics) ObserveLLMRequest(llmID string) {
	if m != nil {
		m.llmRequestsTotal.With(prometheus.Labels{"llm_name": llmID}).Inc()
	}
}

func (m *metrics) ObserveLLMTokensSent(llmID string, count int64) {
	if m != nil {
		m.llmTokensSent.With(prometheus.Labels{"llm_name": llmID}).Add(float64(count))
	}
}

func (m *metrics) ObserveLLMTokensReceived(llmID string, count int64) {
	if m != nil {
		m.llmTokensReceived.With(prometheus.Labels{"llm_name": llmID}).Add(float64(count))
	}
}

func (m *metrics) ObserveLLMBytesSent(llmID string, count int64) {
	if m != nil {
		m.llmBytesSent.With(prometheus.Labels{"llm_name": llmID}).Add(float64(count))
	}
}

func (m *metrics) ObserveLLMBytesReceived(llmID string, count int64) {
	if m != nil {
		m.llmBytesReceived.With(prometheus.Labels{"llm_name": llmID}).Add(float64(count))
	}
}

func (m *metrics) IncrementHTTPRequests() {
	if m != nil {
		m.httpRequestsTotal.Inc()
	}
}

func (m *metrics) IncrementHTTPErrors() {
	if m != nil {
		m.httpErrorsTotal.Inc()
	}
}
