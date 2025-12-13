package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// TODO (T052): Implement Prometheus metrics

var (
	// PluginDownloads tracks total plugin downloads
	// TODO (T053): Implement download counter with labels (name, version, source)
	PluginDownloads = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jenkins_larder_plugin_downloads_total",
			Help: "Total number of plugin downloads",
		},
		[]string{"name", "version", "source"}, // source: cache or upstream
	)

	// CacheHitRatio tracks cache hit rate
	// TODO (T054): Implement cache hit/miss tracking
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

	// StorageUsage tracks current storage utilization
	// TODO (T055): Implement storage usage gauge
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

	// DownloadDuration tracks download latency
	// TODO (T056): Implement download duration histogram
	DownloadDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "jenkins_larder_download_duration_seconds",
			Help:    "Plugin download duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"source"}, // cache or upstream
	)

	// UpstreamErrors tracks upstream fetch errors
	// TODO (T057): Implement upstream error counter
	UpstreamErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jenkins_larder_upstream_errors_total",
			Help: "Total number of upstream fetch errors",
		},
		[]string{"type"}, // timeout, not_found, server_error, etc.
	)
)

// RecordDownload records a plugin download
func RecordDownload(name, version, source string) {
	PluginDownloads.WithLabelValues(name, version, source).Inc()

	if source == "cache" {
		CacheHits.Inc()
	} else {
		CacheMisses.Inc()
	}
}

// UpdateStorageMetrics updates storage usage metrics
// TODO (T058): Implement periodic storage metrics update
func UpdateStorageMetrics(usageBytes, limitBytes int64) {
	StorageUsageBytes.Set(float64(usageBytes))
	StorageLimitBytes.Set(float64(limitBytes))
}

// RecordUpstreamError records an upstream error
func RecordUpstreamError(errorType string) {
	UpstreamErrors.WithLabelValues(errorType).Inc()
}
