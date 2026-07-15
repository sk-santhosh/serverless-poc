package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RedisSpec describes the Redis Stream a QueueWorker watches.
type RedisSpec struct {
	// Address is host:port of the Redis instance, e.g. "redis-master.default.svc.cluster.local:6379".
	Address string `json:"address"`

	// Stream is the name of the Redis Stream to watch.
	Stream string `json:"stream"`

	// ConsumerGroup is the Redis consumer group used to coordinate delivery
	// across worker Pods. Created automatically if it does not exist.
	ConsumerGroup string `json:"consumerGroup"`
}

// WorkerSpec describes the container image and configuration used to
// process a single queue item. The operator has no knowledge of what the
// worker actually does — it is purely a Job template.
type WorkerSpec struct {
	// Image is the container image run once per queue item.
	Image string `json:"image"`

	// EnvFrom is injected into the worker container alongside the
	// Redis connection env vars the operator adds automatically
	// (REDIS_ADDRESS, REDIS_STREAM, REDIS_CONSUMER_GROUP).
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Resources are the compute resource requirements for the worker
	// container. Requests/limits are mandatory in this cluster's policy.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// RuntimeClassName selects the RuntimeClass for worker Pods (e.g.
	// "gvisor" to sandbox workers with runsc). Empty means the cluster's
	// default runtime.
	// +optional
	RuntimeClassName string `json:"runtimeClassName,omitempty"`
}

// QueueWorkerSpec defines the desired state of a QueueWorker.
type QueueWorkerSpec struct {
	// Redis identifies the stream this QueueWorker watches.
	Redis RedisSpec `json:"redis"`

	// MaxConcurrency is the maximum number of worker Jobs that may run at
	// once for this queue.
	// +kubebuilder:validation:Minimum=1
	MaxConcurrency int `json:"maxConcurrency"`

	// PollInterval is how often the controller checks queue depth, as a
	// Go duration string (e.g. "5s"). Parsed with time.ParseDuration.
	// +kubebuilder:validation:Pattern=`^[0-9]+(ms|s|m|h)$`
	PollInterval string `json:"pollInterval"`

	// Worker is the Job/Pod template used to process one queue item.
	Worker WorkerSpec `json:"worker"`
}

// QueueWorkerStatus defines the observed state of a QueueWorker.
type QueueWorkerStatus struct {
	// ActiveWorkers is the number of non-completed Jobs currently owned by
	// this QueueWorker.
	// +optional
	ActiveWorkers int `json:"activeWorkers,omitempty"`

	// QueueDepth is the last-observed length of the Redis stream.
	// +optional
	QueueDepth int64 `json:"queueDepth,omitempty"`

	// LastReconcileTime is the timestamp of the most recent reconcile loop.
	// +optional
	LastReconcileTime metav1.Time `json:"lastReconcileTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=qw
// +kubebuilder:printcolumn:name="Queue",type=string,JSONPath=".spec.redis.stream"
// +kubebuilder:printcolumn:name="MaxConcurrency",type=integer,JSONPath=".spec.maxConcurrency"
// +kubebuilder:printcolumn:name="Active",type=integer,JSONPath=".status.activeWorkers"
// +kubebuilder:printcolumn:name="Depth",type=integer,JSONPath=".status.queueDepth"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// QueueWorker is the Schema for the queueworkers API.
//
// A QueueWorker is intentionally generic: it points a Redis stream at a
// worker container image and a concurrency limit. Nothing about it is
// specific to sending email — the welcome-email behavior lives entirely in
// the image referenced by spec.worker.image.
type QueueWorker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueWorkerSpec   `json:"spec,omitempty"`
	Status QueueWorkerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QueueWorkerList contains a list of QueueWorker.
type QueueWorkerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QueueWorker `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QueueWorker{}, &QueueWorkerList{})
}
