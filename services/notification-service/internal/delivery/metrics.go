package delivery

import "github.com/prometheus/client_golang/prometheus"

var (
	notificationsSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "subber_notifications_sent_total",
			Help: "Notifications successfully sent.",
		},
	)
	notificationsFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "subber_notifications_failed_total",
			Help: "Notification send failures.",
		},
	)
	notificationsDead = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "subber_notifications_dead_total",
			Help: "Notifications marked dead after exhausting retries.",
		},
	)
	notificationsRetried = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "subber_notifications_retried_total",
			Help: "Notifications published to retry topics.",
		},
	)
)

func init() {
	prometheus.MustRegister(notificationsSent, notificationsFailed, notificationsDead, notificationsRetried)
}
