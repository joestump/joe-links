// Governing: SPEC-0016 REQ "Prometheus Metrics Endpoint", ADR-0016
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RedirectsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "joelinks_redirects_total",
		Help: "Total slug resolution attempts.",
	}, []string{"status"})

	RedirectDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "joelinks_redirect_duration_seconds",
		Help:    "Time from request receipt to redirect response.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
	})

	ClicksRecordedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "joelinks_clicks_recorded_total",
		Help: "Click rows successfully written to the database.",
	})

	ClicksRecordErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "joelinks_clicks_record_errors_total",
		Help: "Click insert failures.",
	})

	LinksTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "joelinks_links_total",
		Help: "Total number of links in the database.",
	})

	UsersTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "joelinks_users_total",
		Help: "Total number of registered users in the database.",
	})
)
