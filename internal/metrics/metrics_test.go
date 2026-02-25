package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gimme-cdn/gimme/internal/metrics"
)

func TestHTTPRequestsTotal_Inc(t *testing.T) {
	// Use a fresh registry to avoid interference with the global default registry
	// (which already has the production metrics registered via init()).
	reg := prometheus.NewRegistry()

	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_http_requests_total",
		Help: "test",
	}, []string{"route", "method", "status_code"})

	require.NoError(t, reg.Register(counter))

	counter.WithLabelValues("/gimme/:package/*file", "GET", "200").Inc()
	counter.WithLabelValues("/gimme/:package/*file", "GET", "200").Inc()
	counter.WithLabelValues("/packages", "POST", "201").Inc()

	assert.Equal(t, float64(2), testutil.ToFloat64(counter.WithLabelValues("/gimme/:package/*file", "GET", "200")))
	assert.Equal(t, float64(1), testutil.ToFloat64(counter.WithLabelValues("/packages", "POST", "201")))
}

func TestS3OperationDuration_Observe(t *testing.T) {
	reg := prometheus.NewRegistry()

	hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "test_s3_op_duration_seconds",
		Help:    "test",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})

	require.NoError(t, reg.Register(hist))

	hist.WithLabelValues("GetObject").Observe(0.005)
	hist.WithLabelValues("GetObject").Observe(0.010)
	hist.WithLabelValues("AddObject").Observe(0.050)

	// Verify via text gathering that observations were recorded.
	// GatherAndCount returns the number of individual metric series (not families).
	// We have 2 label values (GetObject, AddObject) → 2 series.
	out, err := testutil.GatherAndCount(reg)
	require.NoError(t, err)
	assert.Equal(t, 2, out)
}

func TestCacheCounters(t *testing.T) {
	reg := prometheus.NewRegistry()

	hits := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_cache_hits_total", Help: "test"})
	misses := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_cache_misses_total", Help: "test"})
	require.NoError(t, reg.Register(hits))
	require.NoError(t, reg.Register(misses))

	hits.Inc()
	hits.Inc()
	misses.Inc()

	assert.Equal(t, float64(2), testutil.ToFloat64(hits))
	assert.Equal(t, float64(1), testutil.ToFloat64(misses))
}

func TestPackageLifecycleCounters(t *testing.T) {
	reg := prometheus.NewRegistry()

	uploaded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_packages_uploaded_total", Help: "test"})
	deleted := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_packages_deleted_total", Help: "test"})
	require.NoError(t, reg.Register(uploaded))
	require.NoError(t, reg.Register(deleted))

	uploaded.Inc()
	uploaded.Inc()
	uploaded.Inc()
	deleted.Inc()

	assert.Equal(t, float64(3), testutil.ToFloat64(uploaded))
	assert.Equal(t, float64(1), testutil.ToFloat64(deleted))
}

// descName extracts the fqName from a *prometheus.Desc string representation.
// The format is: `Desc{fqName: "<name>", help: "...", ...}`.
func descName(d *prometheus.Desc) string {
	s := d.String()
	const key = `fqName: "`
	for i := 0; i+len(key) <= len(s); i++ {
		if s[i:i+len(key)] == key {
			rest := s[i+len(key):]
			for j, ch := range rest {
				if ch == '"' {
					return rest[:j]
				}
			}
		}
	}
	return ""
}

// collectDescNames collects all metric names reported by a Collector via Describe.
func collectDescNames(c prometheus.Collector) []string {
	ch := make(chan *prometheus.Desc, 10)
	go func() {
		c.Describe(ch)
		close(ch)
	}()
	var names []string
	for d := range ch {
		names = append(names, descName(d))
	}
	return names
}

func TestMetricsRegisteredInDefaultRegistry(t *testing.T) {
	// Use Describe to read the actual metric name from each production Collector
	// without mutating any global state (no .Inc() / .Observe() calls needed).
	type namedCollector struct {
		name      string
		collector prometheus.Collector
	}
	cases := []namedCollector{
		{"gimme_http_requests_total", metrics.HTTPRequestsTotal},
		{"gimme_s3_operation_duration_seconds", metrics.S3OperationDuration},
		{"gimme_cache_hits_total", metrics.CacheHitsTotal},
		{"gimme_cache_misses_total", metrics.CacheMissesTotal},
		{"gimme_packages_uploaded_total", metrics.PackagesUploadedTotal},
		{"gimme_packages_deleted_total", metrics.PackagesDeletedTotal},
	}
	for _, tc := range cases {
		names := collectDescNames(tc.collector)
		require.NotEmpty(t, names, "Describe returned no descs for %s", tc.name)
		assert.Contains(t, names, tc.name, "expected metric name %s in Desc output", tc.name)
	}
}

func TestMetricNamesHaveGimmePrefix(t *testing.T) {
	// Verify that every production Collector's real metric name starts with "gimme_".
	collectors := []prometheus.Collector{
		metrics.HTTPRequestsTotal,
		metrics.S3OperationDuration,
		metrics.CacheHitsTotal,
		metrics.CacheMissesTotal,
		metrics.PackagesUploadedTotal,
		metrics.PackagesDeletedTotal,
	}
	for _, c := range collectors {
		for _, name := range collectDescNames(c) {
			assert.True(t, len(name) >= 6 && name[:6] == "gimme_",
				"metric %q must start with 'gimme_'", name)
		}
	}
}
