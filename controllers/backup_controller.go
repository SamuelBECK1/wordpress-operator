/*
Copyright 2023.

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
	wordpressv1alpha1 "github.com/SamuelBECK1/wordpress-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=wordpress.wordpress.com,resources=backups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=wordpress.wordpress.com,resources=backups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=wordpress.wordpress.com,resources=backups/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Backup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	backup := &wordpressv1alpha1.Backup{}
	err := r.Get(context.TODO(), req.NamespacedName, backup)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	foundCronJobs := &batchv1.CronJob{}
	err = r.Get(ctx, types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace}, foundCronJobs)
	if err != nil && errors.IsNotFound(err) {
		// Define a new pvc
		crj := r.cronjobForBackup(backup)
		log.Info("Creating a new Backup definition", "backup.Namespace", backup.Namespace, "backup.Name", backup.Name)
		err = r.Create(ctx, crj)
		if err != nil {
			log.Error(err, "Failed to create new Backup definition", "backup.Namespace", backup.Namespace, "backup.Name", backup.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *BackupReconciler) podTemplate(b *wordpressv1alpha1.Backup) corev1.PodTemplateSpec {
	tpl := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  b.Name,
				Image: "nginx",
				Command: []string{
					"/bin/sh",
					"-c",
					"/usr/bin/curl https://www.google.com",
				},
			}},
			RestartPolicy: "OnFailure",
		},
	}
	return tpl
}

func (r *BackupReconciler) cronjobForBackup(b *wordpressv1alpha1.Backup) *batchv1.CronJob {
	cronExpression := b.Spec.CronJobExpression
	crj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.Name,
			Namespace: b.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: cronExpression,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: r.podTemplate(b),
				},
			},
		},
	}
	ctrl.SetControllerReference(b, crj, r.Scheme)
	return crj
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wordpressv1alpha1.Backup{}).
		Complete(r)
}
