package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	serverlesspocv1alpha1 "github.com/sk-santhosh/serverless-poc/operator/api/v1alpha1"
)

const (
	// queueWorkerLabel is set on every Job the controller creates so it can
	// later list "Jobs belonging to this QueueWorker" via a label selector,
	// without needing to touch the Jobs' Pods or the queue messages they
	// process.
	queueWorkerLabel = "queueworker"

	defaultPollInterval = 5 * time.Second
)

// QueueWorkerReconciler reconciles a QueueWorker object.
//
// Design note: this controller never reads from or acknowledges messages on
// the Redis stream it watches. It only ever asks "how many messages are
// waiting?" (XINFO GROUPS lag + pending) and "how many Jobs are already
// running?", then creates the difference as new Jobs. Message-level delivery coordination (at-least
// -once semantics, retries, acking) is entirely the Redis consumer group's
// job, handled by the worker container itself.
type QueueWorkerReconciler struct {
	client.Client

	// newRedisClient is overridable in tests; defaults to a real go-redis client.
	newRedisClient func(address string) redisClient
}

// redisClient is the subset of go-redis functionality the controller needs.
// Narrowing the interface keeps the reconciler easy to unit test.
type redisClient interface {
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XInfoGroups(ctx context.Context, stream string) *redis.XInfoGroupsCmd
	Close() error
}

// NewQueueWorkerReconciler constructs a reconciler with the default
// (real) Redis client factory.
func NewQueueWorkerReconciler(c client.Client) *QueueWorkerReconciler {
	return &QueueWorkerReconciler{
		Client: c,
		newRedisClient: func(address string) redisClient {
			return redis.NewClient(&redis.Options{Addr: address})
		},
	}
}

// +kubebuilder:rbac:groups=serverless-poc.sk-santhosh.io,resources=queueworkers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=serverless-poc.sk-santhosh.io,resources=queueworkers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete

func (r *QueueWorkerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var qw serverlesspocv1alpha1.QueueWorker
	if err := r.Get(ctx, req.NamespacedName, &qw); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	pollInterval, err := time.ParseDuration(qw.Spec.PollInterval)
	if err != nil {
		logger.Error(err, "invalid pollInterval, falling back to default", "pollInterval", qw.Spec.PollInterval)
		pollInterval = defaultPollInterval
	}

	rdb := r.newRedisClient(qw.Spec.Redis.Address)
	defer rdb.Close()

	// Ensure the consumer group exists. BUSYGROUP means it already does —
	// that's the expected steady state, not an error.
	if err := rdb.XGroupCreateMkStream(ctx, qw.Spec.Redis.Stream, qw.Spec.Redis.ConsumerGroup, "0").Err(); err != nil && !isBusyGroupErr(err) {
		logger.Error(err, "failed to ensure consumer group exists")
		return ctrl.Result{}, err
	}

	// Backlog = entries never delivered to this group (lag) + delivered but
	// not yet acked (pending). XLEN is deliberately NOT used: XACK does not
	// remove entries from a stream, so its length never shrinks as work
	// completes and would keep this controller scaling up forever.
	groups, err := rdb.XInfoGroups(ctx, qw.Spec.Redis.Stream).Result()
	if err != nil {
		logger.Error(err, "failed to read queue depth")
		return ctrl.Result{}, err
	}
	var depth int64
	for _, g := range groups {
		if g.Name == qw.Spec.Redis.ConsumerGroup {
			depth = g.Pending
			// Lag is unknown (reported <= 0) after XDEL/XSETID; fall back
			// to pending-only rather than over- or under-counting.
			if g.Lag > 0 {
				depth += g.Lag
			}
			break
		}
	}

	running, err := r.countRunningJobs(ctx, qw.Namespace, qw.Name)
	if err != nil {
		logger.Error(err, "failed to count running jobs")
		return ctrl.Result{}, err
	}

	queueDepthGauge.WithLabelValues(qw.Name, qw.Spec.Redis.Stream).Set(float64(depth))
	runningJobsGauge.WithLabelValues(qw.Name, qw.Spec.Redis.Stream).Set(float64(running))

	desired := int(minInt64(depth, int64(qw.Spec.MaxConcurrency))) - running
	if desired > 0 {
		logger.Info("scaling up workers", "queueworker", qw.Name, "depth", depth, "running", running, "creating", desired)
		for i := 0; i < desired; i++ {
			if err := r.createWorkerJob(ctx, &qw); err != nil {
				logger.Error(err, "failed to create worker job")
				return ctrl.Result{}, err
			}
			jobsCreatedCounter.WithLabelValues(qw.Name, qw.Spec.Redis.Stream).Inc()
		}
	}

	qw.Status.ActiveWorkers = running + maxInt(desired, 0)
	qw.Status.QueueDepth = depth
	qw.Status.LastReconcileTime = metav1.Now()
	if err := r.Status().Update(ctx, &qw); err != nil {
		// A conflict just means our cached copy went stale mid-reconcile
		// (e.g. a Job watch event landed); the next poll writes fresh
		// status, so retry quietly instead of logging an error.
		if apierrors.IsConflict(err) {
			return ctrl.Result{RequeueAfter: pollInterval}, nil
		}
		logger.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: pollInterval}, nil
}

// countRunningJobs lists Jobs labeled as belonging to this QueueWorker and
// counts the ones that have not yet finished (succeeded or failed).
func (r *QueueWorkerReconciler) countRunningJobs(ctx context.Context, namespace, name string) (int, error) {
	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(labels.Set{queueWorkerLabel: name})},
	); err != nil {
		return 0, err
	}

	running := 0
	for _, job := range jobs.Items {
		if job.Status.CompletionTime == nil && !jobFailed(&job) {
			running++
		}
	}
	return running, nil
}

func jobFailed(job *batchv1.Job) bool {
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// createWorkerJob builds and creates a single-Pod Job that will process
// exactly one queue item, per the spec's worker.image contract.
func (r *QueueWorkerReconciler) createWorkerJob(ctx context.Context, qw *serverlesspocv1alpha1.QueueWorker) error {
	backoffLimit := int32(2)
	ttlSeconds := int32(60)

	env := []corev1.EnvVar{
		{Name: "REDIS_ADDRESS", Value: qw.Spec.Redis.Address},
		{Name: "REDIS_STREAM", Value: qw.Spec.Redis.Stream},
		{Name: "REDIS_CONSUMER_GROUP", Value: qw.Spec.Redis.ConsumerGroup},
	}

	runAsNonRoot := true
	runAsUser := int64(1001)
	readOnlyRootFS := true
	allowPrivilegeEscalation := false

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", qw.Name),
			Namespace:    qw.Namespace,
			Labels: map[string]string{
				queueWorkerLabel: qw.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						queueWorkerLabel: qw.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:    corev1.RestartPolicyNever,
					RuntimeClassName: runtimeClassNameOrNil(qw.Spec.Worker.RuntimeClassName),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
					},
					Containers: []corev1.Container{
						{
							Name:  "worker",
							Image: qw.Spec.Worker.Image,
							// IfNotPresent (not the k8s :latest default of
							// Always) so images side-loaded into kind via
							// `kind load docker-image` are usable without a
							// registry.
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env:             env,
							EnvFrom:         qw.Spec.Worker.EnvFrom,
							Resources:       qw.Spec.Worker.Resources,
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem:   &readOnlyRootFS,
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(qw, job, r.Client.Scheme()); err != nil {
		return err
	}

	return r.Create(ctx, job)
}

func (r *QueueWorkerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&serverlesspocv1alpha1.QueueWorker{}).
		Owns(&batchv1.Job{}).
		Named("queueworker").
		Complete(r)
}

func isBusyGroupErr(err error) bool {
	return err != nil && len(err.Error()) >= 9 && err.Error()[:9] == "BUSYGROUP"
}

// runtimeClassNameOrNil maps the CRD's plain-string field onto PodSpec's
// *string: empty string means "cluster default runtime", i.e. leave nil.
func runtimeClassNameOrNil(name string) *string {
	if name == "" {
		return nil
	}
	return &name
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
