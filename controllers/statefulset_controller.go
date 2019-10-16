package controllers

import (
	"context"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OfJobReconciler reconciles a OfJob object
type SatefulSetReconciler struct {
	client.Client
	Log   logr.Logger
	Queue workqueue.RateLimitingInterface
}

// +kubebuilder:rbac:groups=infra.oneflow.org,resources=ofjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.oneflow.org,resources=ofjobs/status,verbs=get;update;patch

func (r *SatefulSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	//	_ = r.Log.WithValues("ofjob", req.NamespacedName)
	ctx := context.Background()
	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, req.NamespacedName, statefulSet)
	if err != nil {
		r.Log.WithValues("get", "unable to get  job")
	} else {
		key := req.NamespacedName.Namespace + "/" + req.NamespacedName.Name + "/update"
		r.Queue.Add(key)
	}
	return ctrl.Result{}, nil
}

func (r *SatefulSetReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		Complete(r)
}
