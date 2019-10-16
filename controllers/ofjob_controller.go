/*
Copyright 2019 like.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	infrav1 "my.domain/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
	//	"time"
)

// OfJobReconciler reconciles a OfJob object
type OfJobReconciler struct {
	client.Client
	Log   logr.Logger
	Queue workqueue.RateLimitingInterface
}

type stopCh struct {
}

// +kubebuilder:rbac:groups=infra.oneflow.org,resources=ofjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.oneflow.org,resources=ofjobs/status,verbs=get;update;patch

func (r *OfJobReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {

	ctx := context.Background()
	_ = r.Log.WithValues("ofjob", req.NamespacedName)
	nfJob := &infrav1.OfJob{}
	if err := r.Get(ctx, req.NamespacedName, nfJob); err != nil {
		fmt.Printf("delete nfjob")
		key := req.NamespacedName.Namespace + "/" + req.NamespacedName.Name + "/delete"
		r.Queue.Add(key)
	} else {
		fmt.Printf("create nfjob")
		key := req.NamespacedName.Namespace + "/" + req.NamespacedName.Name + "/create"
		r.Queue.Add(key)
	}
	return ctrl.Result{}, nil
}

func (r *OfJobReconciler) createStatefulset(ofJob *infrav1.OfJob) error {
	ctx := context.Background()
	var workerReplicas int32
	if ofJob.Spec.Replicas == nil {
		workerReplicas = 1
	} else {
		workerReplicas = int32(*ofJob.Spec.Replicas)
	}
	satefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			// TODO
			Name:      ofJob.Name,
			Namespace: ofJob.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			// TODO: should be point
			Replicas:    &workerReplicas,
			ServiceName: "service-" + ofJob.Name,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": ofJob.Name + "-app"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": ofJob.Name + "-app"},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "Always",
					Subdomain:     "busybox-subdomain",
					Containers: []corev1.Container{
						{
							Name:       "myjob",
							Image:      "golang:1.12.5",
							WorkingDir: "",
							Command:    []string{"sh", "-c"},
							Args:       []string{"sleep 365d"},
						},
					},
				},
			},
		},
	}

	fmt.Println("create statefulSet")
	err := r.Create(ctx, satefulSet)
	if err != nil {
		fmt.Println(err)
		r.Log.WithValues("error", "create statefulSet error")
		return err
	}
	fmt.Println("create statefulSet" + satefulSet.Name)
	return nil
}
func (r *OfJobReconciler) createJob(ofJob *infrav1.OfJob, namespaceName types.NamespacedName) error {
	ctx := context.Background()
	job := &batchv1.Job{}
	err := r.Get(ctx, namespaceName, job)
	if err != nil {
		if err := r.Get(ctx, namespaceName, ofJob); err != nil {
			fmt.Println(err)
		} else {
			job := &batchv1.Job{}
			fmt.Println("find job")
			if err := r.Get(ctx, namespaceName, job); err != nil {
				fmt.Println("create job")
				job = &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						// TODO
						Name:      ofJob.Name,
						Namespace: ofJob.Namespace,
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: "Never",
								Subdomain:     "busybox-subdomain",
								Containers: []corev1.Container{
									{
										Name:       "myjob",
										Image:      "golang:1.12.5",
										WorkingDir: "",
										Command:    []string{"sh", "-c"},
										Args:       []string{"sleep 60s"},
									},
								},
							},
						},
					},
				}
			}
			//job.Spec = nfJob.Spec
			fmt.Println("create job")
			err := r.Create(ctx, job)
			if err != nil {
				fmt.Println(err)
				r.Log.WithValues("error", "create job error")
				return err
			}
		}
	}
	return nil
}
func (r *OfJobReconciler) delete(namespaceName types.NamespacedName) error {
	ctx := context.Background()
	job := &batchv1.Job{}
	err := r.Get(ctx, namespaceName, job)
	if err != nil {
		r.Log.WithValues("delete", "unable to get  job")
	}
	r.Delete(ctx, job)

	satefulSet := &appsv1.StatefulSet{}
	err = r.Get(ctx, namespaceName, satefulSet)
	if err != nil {
		r.Log.WithValues("delete", "unable to get  satefulSet")
	}
	r.Delete(ctx, satefulSet)

	return nil
}
func (r *OfJobReconciler) SetupWithManager(mgr ctrl.Manager) error {

	r.run()
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.OfJob{}).
		Complete(r)
}

func (r *OfJobReconciler) create(ofJob *infrav1.OfJob, namespaceName types.NamespacedName) error {
	ctx := context.Background()
	var workerReplicas int
	if ofJob.Spec.Replicas == nil {
		workerReplicas = 1
	} else {
		workerReplicas = int(*ofJob.Spec.Replicas)
	}
	fmt.Printf("workerReplicas=%d", workerReplicas)
	if workerReplicas > 1 {
		fmt.Printf("create statefulset")
		satefulSet := &appsv1.StatefulSet{}
		err := r.Get(ctx, namespaceName, satefulSet)
		if err != nil {
			err := r.createStatefulset(ofJob)
			if err != nil {
				r.setFail(namespaceName)
			}
		} else {
			if satefulSet.Status.Replicas == satefulSet.Status.ReadyReplicas {
				err = r.createJob(ofJob, namespaceName)
				if err != nil {
					r.setFail(namespaceName)
				}
			}
		}

	} else {
		err := r.createJob(ofJob, namespaceName)
		if err != nil {
			r.setFail(namespaceName)
		}
	}
	return nil
}
func (r *OfJobReconciler) update(ofJob *infrav1.OfJob, namespaceName types.NamespacedName) error {
	ctx := context.Background()
	job := &batchv1.Job{}
	err := r.Get(ctx, namespaceName, job)
	if err != nil {
		statefulSet := &appsv1.StatefulSet{}
		err = r.Get(ctx, namespaceName, statefulSet)
		if err != nil {
			fmt.Printf("replicas= %d %d ", statefulSet.Status.ReadyReplicas, statefulSet.Status.Replicas)
			if statefulSet.Status.ReadyReplicas == statefulSet.Status.Replicas {
				err = r.createJob(ofJob, namespaceName)
				if err != nil {
					r.setFail(namespaceName)
				}
			}
			return err
		}
	}
	fmt.Printf("success=%d", job.Status.Succeeded)
	if job.Status.Active > 0 {

	}
	if job.Status.Succeeded > 0 {
		r.setSuccess(namespaceName)
		return nil
	}
	if job.Status.Failed > 0 {
		r.setFail(namespaceName)
		return nil
	}
	return nil

}
func (r *OfJobReconciler) check(ofJob *infrav1.OfJob, namespaceName types.NamespacedName) bool {
	fmt.Printf("check %s %s\n", namespaceName.Namespace, namespaceName.Name)
	ctx := context.Background()
	job := &batchv1.Job{}
	err := r.Get(ctx, namespaceName, job)
	if err != nil {
		fmt.Printf("check not found job\n")
		return false
	}
	if job.Status.Succeeded > 0 {
		r.setSuccess(namespaceName)

	}
	if job.Status.Failed > 0 {
		r.setFail(namespaceName)

	}
	if job.Status.Active > 0 {
		r.setRunning(namespaceName)

	}
	r.setFail(namespaceName)
	return false

}
func (r *OfJobReconciler) setSuccess(namespaceName types.NamespacedName) {

	nfJob := &infrav1.OfJob{}
	ctx := context.Background()
	if err := r.Get(ctx, namespaceName, nfJob); err != nil {
		return
	}
	fmt.Printf("complete %s \n", nfJob.Name)
	nfJob.Status.Status = "complete"
	nfJob.Spec.Template.Spec.Containers = make([]corev1.Container, 1)
	err := r.Update(ctx, nfJob)
	if err != nil {
		fmt.Println(err)
	}
	r.recover(namespaceName)

}
func (r *OfJobReconciler) setRunning(namespaceName types.NamespacedName) {

	nfJob := &infrav1.OfJob{}
	ctx := context.Background()
	if err := r.Get(ctx, namespaceName, nfJob); err != nil {
		return
	}
	fmt.Printf("running %s \n", nfJob.Name)
	nfJob.Status.Status = "running"
	nfJob.Spec.Template.Spec.Containers = make([]corev1.Container, 1)
	err := r.Update(ctx, nfJob)
	if err != nil {
		fmt.Println(err)
	}

}
func (r *OfJobReconciler) recover(namespaceName types.NamespacedName) {

	/*
		ctx := context.Background()
		var replicas int32 = 0
		satefulSet := &appsv1.StatefulSet{}
		err := r.Get(ctx, namespaceName, satefulSet)
		if err != nil {
			return
		}
		fmt.Printf(" recover %s %s\n", namespaceName.Name, namespaceName.Namespace)
		satefulSet.Spec.Replicas = &replicas
		err = r.Update(ctx, satefulSet)
		if err != nil {

			fmt.Println("recover error", err)
		}
	*/
	fmt.Printf(" recover %s %s\n", namespaceName.Name, namespaceName.Namespace)
	r.delete(namespaceName)

}
func (r *OfJobReconciler) setFail(namespaceName types.NamespacedName) {
	nfJob := &infrav1.OfJob{}
	ctx := context.Background()
	if err := r.Get(ctx, namespaceName, nfJob); err != nil {

	}
	nfJob.Status.Status = "fail"
	r.Update(ctx, nfJob)
	r.recover(namespaceName)
}
func (r *OfJobReconciler) run() error {
	stopCh := make(chan struct{}, 1)
	//go r.runWorker()
	go wait.Until(r.runWorker, time.Second, stopCh)
	return nil
}

func (r *OfJobReconciler) runWorker() {
	ctx := context.Background()
	obj, shutdown := r.Queue.Get()
	if shutdown {
		return
	}
	key := obj.(string)
	namespace, name, action, err := r.splitMetaNamespaceKey(key)
	if err != nil {
		fmt.Printf("invalid resource key: %s\n", obj)
		r.Queue.Forget(obj)
	}
	namespaceName := types.NamespacedName{}
	namespaceName.Name = name
	namespaceName.Namespace = namespace
	ofJob := &infrav1.OfJob{}
	if err := r.Get(ctx, namespaceName, ofJob); err != nil {
		r.Log.WithValues("get", "unable to get nfjob")
		r.Queue.Forget(obj)
	}
	fmt.Printf("action %s %s %s \n", action, namespaceName, name)
	switch action {
	case "create":
		{
			err = r.create(ofJob, namespaceName)
			if err != nil {
				r.Queue.Forget(obj)
				return
			} else {
				r.Queue.Done(obj)
				key := namespace + "/" + name + "/check"
				r.Queue.AddAfter(key, 1000*1000*1000*60*5)
				return
			}
		}
	case "delete":
		{
			err := r.delete(namespaceName)
			if err != nil {

			} else {
				r.Queue.Done(obj)
				return
			}
		}
	case "update":
		{
			fmt.Printf("update")
			err := r.update(ofJob, namespaceName)
			if err != nil {

			} else {
				r.Queue.Done(obj)
				return
			}
		}
	case "check":
		{
			ok := r.check(ofJob, namespaceName)
			if ok {
				r.Queue.Done(obj)
				return
			} else {
				r.setFail(namespaceName)
				r.Queue.Done(obj)
				key := namespace + "/" + name + "/create"
				r.Queue.AddAfter(key, 1000*1000*1000*60*5)
				return
			}
		}
	}
	return

}

func (r *OfJobReconciler) splitMetaNamespaceKey(key string) (namespace, name string, action string, err error) {
	parts := strings.Split(key, "/")
	if len(parts) == 3 {
		// namespace and name
		return parts[0], parts[1], parts[2], nil
	}

	return "", "", "", fmt.Errorf("unexpected key format: %q", key)
}
