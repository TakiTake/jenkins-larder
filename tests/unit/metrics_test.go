package unit

import (
	"testing"

	"github.com/yourorg/jenkins-larder/src/metrics"
)

func TestRecordDownload(t *testing.T) {
	// Should not panic
	metrics.RecordDownload("git", "1.0", "cache")
	metrics.RecordDownload("git", "1.0", "upstream")
	metrics.RecordDownload("git", "1.0", "stale")
}

func TestRecordBandwidthSaved(t *testing.T) {
	metrics.RecordBandwidthSaved(1024)
}

func TestRecordEviction(t *testing.T) {
	metrics.RecordEviction("storage_limit")
	metrics.RecordEviction("manual")
}

func TestUpdateStorageMetrics(t *testing.T) {
	metrics.UpdateStorageMetrics(5000, 10000)
}

func TestUpdateCachedPlugins(t *testing.T) {
	metrics.UpdateCachedPlugins(42)
}

func TestRecordUpstreamError(t *testing.T) {
	metrics.RecordUpstreamError("download_failed")
	metrics.RecordUpstreamError("timeout")
}

func TestRecordChecksumFailure(t *testing.T) {
	metrics.RecordChecksumFailure()
}
