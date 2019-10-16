package controllers

import (
	"context"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OfJobReconciler reconciles a OfJob object
type JobReconciler struct {
	client.Client
	Log   logr.Logger
	Queue workqueue.RateLimitingInterface
}

// +kubebuilder:rbac:groups=infra.oneflow.org,resources=ofjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.oneflow.org,resources=ofjobs/status,verbs=get;update;patch

func (r *JobReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	//	_ = r.Log.WithValues("ofjob", req.NamespacedName)
	ctx := context.Background()
	job := &batchv1.Job{}
	err := r.Get(ctx, req.NamespacedName, job)
	if err != nil {
		r.Log.WithValues("get", "unable to get  job")
	}
	if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
		key := req.NamespacedName.Namespace + "/" + req.NamespacedName.Name + "/update"
		r.Queue.Add(key)
	}
	return ctrl.Result{}, nil
}

func (r *JobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		Complete(r)
}
