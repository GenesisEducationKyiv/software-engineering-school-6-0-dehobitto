package outbox

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

func NewBacklogGauge(pool *pgxpool.Pool, service string) prometheus.Collector {
	return prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name:        "subber_outbox_backlog",
			Help:        "Current number of unpublished outbox events.",
			ConstLabels: prometheus.Labels{"service": service},
		},
		func() float64 {
			if pool == nil {
				return 0
			}
			var count float64
			if err := pool.QueryRow(context.Background(), `
SELECT COUNT(*)
FROM outbox_events
WHERE published_at IS NULL
`).Scan(&count); err != nil {
				return 0
			}
			return count
		},
	)
}
