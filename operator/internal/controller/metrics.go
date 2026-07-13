package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Custom metrics for the QueueWorker controller, registered into
// controller-runtime's global registry so they are served from the same
// /metrics endpoint as the built-in controller_runtime_* metrics.
var (
	queueDepthGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "queueworker_queue_depth",
		Help: "Backlog of the watched Redis stream (consumer group lag + pending), as seen at the last reconcile.",
	}, []string{"queueworker", "stream"})

	runningJobsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "queueworker_running_jobs",
		Help: "Worker Jobs currently running for this QueueWorker, as seen at the last reconcile.",
	}, []string{"queueworker", "stream"})

	jobsCreatedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "queueworker_jobs_created_total",
		Help: "Worker Jobs created by the controller.",
	}, []string{"queueworker", "stream"})
)

func init() {
	metrics.Registry.MustRegister(queueDepthGauge, runningJobsGauge, jobsCreatedCounter)
}
