// Package metrics defines application Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	EmailsSentTotal     prometheus.Counter
	EmailsFailedTotal   prometheus.Counter
	ScanCyclesTotal     prometheus.Counter
	LogEntriesEnqueued  prometheus.Counter
	LogEntriesDropped   prometheus.Counter
	LogEntriesPublished prometheus.Counter
	LogPublishErrors    prometheus.Counter
}

// New registers application metrics against registerer.
func New(registerer prometheus.Registerer) *Metrics {
	factory := promauto.With(registerer)

	return &Metrics{
		HTTPRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "route", "status_code"},
		),

		HTTPRequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route"},
		),

		EmailsSentTotal: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "emails_sent_total",
				Help: "Total number of emails sent",
			},
		),

		EmailsFailedTotal: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "emails_failed_total",
				Help: "Total number of failed email sends",
			},
		),

		ScanCyclesTotal: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "scan_cycles_total",
				Help: "Total number of scan cycles completed",
			},
		),

		LogEntriesEnqueued: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "log_entries_enqueued_total",
				Help: "Total number of log entries queued for asynchronous publishing",
			},
		),

		LogEntriesDropped: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "log_entries_dropped_total",
				Help: "Total number of log entries dropped before publishing",
			},
		),

		LogEntriesPublished: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "log_entries_published_total",
				Help: "Total number of log entries successfully published to the log broker",
			},
		),

		LogPublishErrors: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "log_publish_errors_total",
				Help: "Total number of log broker publish errors",
			},
		),
	}
}

func (m *Metrics) IncLogEntriesEnqueued() {
	m.LogEntriesEnqueued.Inc()
}

func (m *Metrics) IncLogEntriesDropped() {
	m.LogEntriesDropped.Inc()
}

func (m *Metrics) IncLogEntriesPublished() {
	m.LogEntriesPublished.Inc()
}

func (m *Metrics) IncLogPublishErrors() {
	m.LogPublishErrors.Inc()
}

func RegisterRuntimeCollectors(registerer prometheus.Registerer) {
	registerer.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

func NewNoop() *Metrics {
	registry := prometheus.NewRegistry()
	return New(registry)
}
