package outbox

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	pool     *pgxpool.Pool
	service  string
	pending  *prometheus.Desc
	failed   *prometheus.Desc
	attempts *prometheus.Desc
}

func NewCollector(pool *pgxpool.Pool, service string) *Collector {
	labels := []string{"service"}
	return &Collector{
		pool:    pool,
		service: service,
		pending: prometheus.NewDesc(
			"subber_outbox_pending",
			"Outbox events waiting to be published.",
			labels,
			nil,
		),
		failed: prometheus.NewDesc(
			"subber_outbox_failed",
			"Outbox events that have a recorded publish error and are not published yet.",
			labels,
			nil,
		),
		attempts: prometheus.NewDesc(
			"subber_outbox_publish_attempts",
			"Total recorded outbox publish attempts for unpublished events.",
			labels,
			nil,
		),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.pending
	ch <- c.failed
	ch <- c.attempts
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var pending, failed, attempts float64
	err := c.pool.QueryRow(ctx, `
SELECT
	COUNT(*) FILTER (WHERE published_at IS NULL)::float8 AS pending,
	COUNT(*) FILTER (WHERE published_at IS NULL AND last_error IS NOT NULL)::float8 AS failed,
	COALESCE(SUM(publish_attempts) FILTER (WHERE published_at IS NULL), 0)::float8 AS attempts
FROM outbox_events
`).Scan(&pending, &failed, &attempts)
	if err != nil {
		ch <- prometheus.NewInvalidMetric(c.pending, err)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.pending, prometheus.GaugeValue, pending, c.service)
	ch <- prometheus.MustNewConstMetric(c.failed, prometheus.GaugeValue, failed, c.service)
	ch <- prometheus.MustNewConstMetric(c.attempts, prometheus.GaugeValue, attempts, c.service)
}
