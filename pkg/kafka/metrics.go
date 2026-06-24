package kafka

import "github.com/prometheus/client_golang/prometheus"

var (
	consumerProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subber_kafka_consumer_processed_total",
			Help: "Kafka messages successfully processed and committed.",
		},
		[]string{"consumer_group", "topic"},
	)
	consumerFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subber_kafka_consumer_failed_total",
			Help: "Kafka consumer failures that stop message processing without committing the offset.",
		},
		[]string{"consumer_group", "topic", "stage"},
	)
	consumerSkipped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subber_kafka_consumer_skipped_total",
			Help: "Kafka messages skipped as non-retryable after DLQ handling.",
		},
		[]string{"consumer_group", "topic"},
	)
)

func init() {
	prometheus.MustRegister(consumerProcessed, consumerFailed, consumerSkipped)
}

func observeProcessed(groupID, topic string) {
	consumerProcessed.WithLabelValues(groupID, topic).Inc()
}

func observeFailed(groupID, topic, stage string) {
	consumerFailed.WithLabelValues(groupID, topic, stage).Inc()
}

func observeSkipped(groupID, topic string) {
	consumerSkipped.WithLabelValues(groupID, topic).Inc()
}

func NewConsumerLagGauge(service, topic string, consumer *Consumer) prometheus.Collector {
	return prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "subber_kafka_consumer_lag",
			Help: "Current Kafka consumer group lag reported by kafka-go.",
			ConstLabels: prometheus.Labels{
				"service": service,
				"topic":   topic,
			},
		},
		func() float64 {
			return float64(consumer.Lag())
		},
	)
}
