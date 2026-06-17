package kafka

import "github.com/prometheus/client_golang/prometheus"

func NewConsumerLagGauge(service, topic string, consumer *Consumer) prometheus.Collector {
	return prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name:        "subber_kafka_consumer_lag",
			Help:        "Current Kafka consumer lag for a service/topic pair.",
			ConstLabels: prometheus.Labels{"service": service, "topic": topic},
		},
		func() float64 {
			if consumer == nil {
				return 0
			}
			return float64(consumer.Lag())
		},
	)
}
