package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsCollection struct {
	registerer prometheus.Registerer

	// global statistics
	BackupsTotal      prometheus.Counter
	BackupsSuccessful prometheus.Counter
	BackupsFailed     prometheus.Counter

	// repository statistics
	DataBlobs prometheus.Gauge
	TreeBlobs prometheus.Gauge

	// snapshot statistics
	FilesNew        prometheus.Gauge
	FilesChanged    prometheus.Gauge
	FilesUnmodified prometheus.Gauge
	DirsNew         prometheus.Gauge
	DirsChanged     prometheus.Gauge
	DirsUnmodified  prometheus.Gauge
	FilesProcessed  prometheus.Gauge // `total_files_processed` in summary, `total_files` in status messages
	BytesProcessed  prometheus.Gauge // `total_bytes_processed` in summary, `total_bytes` in status messages
	BytesAdded      prometheus.Gauge
	BackupDuration  prometheus.Gauge // `total_duration` in summary message
}

func (m *MetricsCollection) Initialize() {
	// global statistics
	m.BackupsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "backup",
		Name:      "backups_all_total",
		Help:      "The total number of backups attempted, including failures.",
	})
	m.BackupsSuccessful = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "backup",
		Name:      "backups_successful_total",
		Help:      "The total number of backups that succeeded.",
	})
	m.BackupsFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "backup",
		Name:      "backups_failed_total",
		Help:      "The total number of backups that failed.",
	})

	// `restic backup --json` response:
	// repository statistics
	m.DataBlobs = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_blobs_data",
		Help:      "The number of data blobs in the repository.",
	})
	m.TreeBlobs = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_blobs_tree",
		Help:      "The number of tree blobs in the repository.",
	})

	// snapshot statistics
	m.FilesNew = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_files_new",
		Help:      "Amount of new files.",
	})
	m.FilesChanged = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_files_changed",
		Help:      "Amount of files with changes.",
	})
	m.FilesUnmodified = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_files_unmodified",
		Help:      "Amount of files unmodified since last backup.",
	})
	m.DirsNew = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_dirs_new",
		Help:      "Amount of new directories.",
	})
	m.DirsChanged = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_dirs_changed",
		Help:      "Amount of directories with changes.",
	})
	m.DirsUnmodified = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_dirs_unmodified",
		Help:      "Amount of directories unmodified since last backup.",
	})
	m.FilesProcessed = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_files_processed",
		Help:      "Total number of files scanned by the backup for changes.",
	})
	m.BytesProcessed = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_processed_bytes",
		Help:      "Total number of bytes scanned by the backup for changes.",
	})
	m.BytesAdded = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_added_bytes",
		Help:      "Total number of bytes added to the repository.",
	})
	m.BackupDuration = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "backup",
		Name:      "restic_duration_milliseconds",
		Help:      "The duration of backups in milliseconds.",
	})
}

func (m *MetricsCollection) Register(r prometheus.Registerer) {
	if r == nil {
		r = prometheus.DefaultRegisterer
	}
	m.registerer = r

	r.MustRegister(
		m.BackupsTotal,
		m.BackupsSuccessful,
		m.BackupsFailed,
		m.DataBlobs,
		m.TreeBlobs,
		m.FilesNew,
		m.FilesChanged,
		m.FilesUnmodified,
		m.DirsNew,
		m.DirsChanged,
		m.DirsUnmodified,
		m.FilesProcessed,
		m.BytesProcessed,
		m.BytesAdded,
		m.BackupDuration,
	)
}

func (m *MetricsCollection) getHandler() http.Handler {
	if m.registerer == nil {
		return promhttp.Handler()
	}

	// from promhttp.Handler()
	return promhttp.InstrumentMetricHandler(
		m.registerer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}),
	)
}
