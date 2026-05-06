package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// T055: mirror_requests_total counter with status label
	PluginDownloads = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jenkins_larder_plugin_downloads_total",
			Help: "Total number of plugin downloads",
		},
		[]string{"name", "version", "source"},
	)

	// T054: Cache hit/miss tracking
	CacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "jenkins_larder_cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	CacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "jenkins_larder_cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	// T057: mirror_storage_bytes_used and mirror_storage_bytes_limit gauges
	StorageUsageBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "jenkins_larder_storage_usage_bytes",
			Help: "Current storage usage in bytes",
		},
	)

	StorageLimitBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "jenkins_larder_storage_limit_bytes",
			Help: "Storage limit in bytes",
		},
	)

	// T056: mirror_request_duration_seconds histogram
	DownloadDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "jenkins_larder_download_duration_seconds",
			Help:    "Plugin download duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"source"},
	)

	// T058: mirror_cached_plugins_total gauge
	CachedPluginsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "jenkins_larder_cached_plugins_total",
			Help: "Total number of cached plugins",
		},
	)

	// T059: mirror_bandwidth_saved_bytes_total counter
	BandwidthSavedBytes = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "jenkins_larder_bandwidth_saved_bytes_total",
			Help: "Total bandwidth saved by cache hits (bytes not fetched from upstream)",
		},
	)

	// T060: mirror_evictions_total counter with reason label
	EvictionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jenkins_larder_evictions_total",
			Help: "Total number of cache evictions",
		},
		[]string{"reason"},
	)

	// T061: mirror_upstream_failures_total and mirror_checksum_failures_total
	UpstreamErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jenkins_larder_upstream_errors_total",
			Help: "Total number of upstream fetch errors",
		},
		[]string{"type"},
	)

	ChecksumFailures = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "jenkins_larder_checksum_failures_total",
			Help: "Total number of checksum validation failures",
		},
	)
)

func init() {
	// Initialize label combinations so metrics appear in output even at zero
	EvictionsTotal.WithLabelValues("storage_limit")
	EvictionsTotal.WithLabelValues("manual")
	UpstreamErrors.WithLabelValues("download_failed")
	UpstreamErrors.WithLabelValues("timeout")
}

// RecordDownload records a plugin download
func RecordDownload(name, version, source string) {
	PluginDownloads.WithLabelValues(name, version, source).Inc()

	if source == "cache" {
		CacheHits.Inc()
	} else {
		CacheMisses.Inc()
	}
}

// RecordBandwidthSaved records bytes saved by serving from cache
func RecordBandwidthSaved(bytes int64) {
	BandwidthSavedBytes.Add(float64(bytes))
}

// RecordEviction records a cache eviction
func RecordEviction(reason string) {
	EvictionsTotal.WithLabelValues(reason).Inc()
}

// UpdateStorageMetrics updates storage usage metrics
func UpdateStorageMetrics(usageBytes, limitBytes int64) {
	StorageUsageBytes.Set(float64(usageBytes))
	StorageLimitBytes.Set(float64(limitBytes))
}

// UpdateCachedPlugins updates the cached plugins gauge
func UpdateCachedPlugins(count int) {
	CachedPluginsTotal.Set(float64(count))
}

// RecordUpstreamError records an upstream error
func RecordUpstreamError(errorType string) {
	UpstreamErrors.WithLabelValues(errorType).Inc()
}

// RecordChecksumFailure records a checksum validation failure
func RecordChecksumFailure() {
	ChecksumFailures.Inc()
}
