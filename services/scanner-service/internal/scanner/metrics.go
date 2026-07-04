package scanner

import "github.com/prometheus/client_golang/prometheus"

var ReleaseDetectedTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "subber_release_detected_total",
		Help: "Total releases detected by scanner-service.",
	},
)
