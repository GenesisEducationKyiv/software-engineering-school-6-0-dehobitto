package delivery

import "github.com/prometheus/client_golang/prometheus"

var (
	NotificationSentTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "subber_notification_sent_total",
			Help: "Total notifications successfully sent by notification-service.",
		},
	)
	NotificationDeadTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "subber_notification_dead_total",
			Help: "Total notifications moved to dead state by notification-service.",
		},
	)
)
