// Package metrics centralises all Prometheus metric definitions for Gimme.
// All metrics are registered against the default Prometheus registry so they
// are automatically exposed on the /metrics endpoint provided by promhttp.Handler().
package metrics

import "github.com/prometheus/client_golang/prometheus"

// HTTP request counter — incremented once per handled request.
// Labels: route (matched Gin pattern, e.g. "/gimme/:package/*file"), method, status_code.
var HTTPRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "gimme_http_requests_total",
		Help: "Total number of HTTP requests handled, partitioned by route, method and status code.",
	},
	[]string{"route", "method", "status_code"},
)

// S3 operation latency histogram.
// Labels: operation (AddObject, GetObject, ListObjects, RemoveObjects, ObjectExists, Ping).
var S3OperationDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "gimme_s3_operation_duration_seconds",
		Help:    "Latency of S3 operations in seconds, partitioned by operation name.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"operation"},
)

// Cache lookup counters.
var CacheHitsTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gimme_cache_hits_total",
		Help: "Total number of internal cache hits (partial-version resolution).",
	},
)

var CacheMissesTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gimme_cache_misses_total",
		Help: "Total number of internal cache misses (partial-version resolution).",
	},
)

// Package lifecycle counters.
var PackagesUploadedTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gimme_packages_uploaded_total",
		Help: "Total number of packages successfully uploaded.",
	},
)

var PackagesDeletedTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "gimme_packages_deleted_total",
		Help: "Total number of packages successfully deleted.",
	},
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		S3OperationDuration,
		CacheHitsTotal,
		CacheMissesTotal,
		PackagesUploadedTotal,
		PackagesDeletedTotal,
	)
}
