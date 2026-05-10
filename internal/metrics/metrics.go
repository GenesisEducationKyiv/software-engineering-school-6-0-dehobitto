// Package metrics defines shared Prometheus counters for background workers.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EmailsSentTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "emails_sent_total",
			Help: "Total number of emails sent",
		},
	)

	EmailsFailedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "emails_failed_total",
			Help: "Total number of failed email sends",
		},
	)

	ScanCyclesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "scan_cycles_total",
			Help: "Total number of scan cycles completed",
		},
	)
)
