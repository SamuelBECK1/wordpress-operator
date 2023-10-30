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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	wordpressv1alpha1 "github.com/SamuelBECK1/wordpress-operator/api/v1alpha1"
)

// WordpressReconciler reconciles a Wordpress object
type WordpressReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=wordpress.wordpress.com,resources=wordpresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=wordpress.wordpress.com,resources=wordpresses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=wordpress.wordpress.com,resources=wordpresses/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;
//+kubebuilder:rbac:groups=core,resources=services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Wordpress object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *WordpressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	wordpress := &wordpressv1alpha1.Wordpress{}
	err := r.Get(context.TODO(), req.NamespacedName, wordpress)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	found := &appsv1.Deployment{}
	foundSvc := &corev1.Service{}
	foundPvc := &corev1.PersistentVolumeClaim{}
	err = r.Get(ctx, types.NamespacedName{Name: "mysql-pvc-" + wordpress.Name, Namespace: wordpress.Namespace}, foundPvc)
	if err != nil && errors.IsNotFound(err) {
		// Define a new pvc
		pvc := r.pvcForMysql(wordpress)
		log.Info("Creating a new PVC", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
		err = r.Create(ctx, pvc)
		if err != nil {
			log.Error(err, "Failed to create new PVC", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
			return ctrl.Result{}, err
		}
		// pvc created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get PVC")
		return ctrl.Result{}, err
	}

	err = r.Get(ctx, types.NamespacedName{Name: "pvc-" + wordpress.Name, Namespace: wordpress.Namespace}, foundPvc)
	if err != nil && errors.IsNotFound(err) {
		// Define a new pvc
		pvc := r.pvcForWordpress(wordpress)
		log.Info("Creating a new PVC", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
		err = r.Create(ctx, pvc)
		if err != nil {
			log.Error(err, "Failed to create new PVC", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
			return ctrl.Result{}, err
		}
		// pvc created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get PVC")
		return ctrl.Result{}, err
	}

	err = r.Get(ctx, types.NamespacedName{Name: "mysql-" + wordpress.Name, Namespace: wordpress.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForPostgresql(wordpress)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}
	err = r.Get(ctx, types.NamespacedName{Name: "mysql-" + wordpress.Name + "-svc", Namespace: wordpress.Namespace}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		svc := r.serviceForPostgresql(wordpress)
		log.Info("Creating a new service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	err = r.Get(ctx, types.NamespacedName{Name: wordpress.Name + "-svc", Namespace: wordpress.Namespace}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		svc := r.serviceForWordpress(wordpress)
		log.Info("Creating a new service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	err = r.Get(ctx, types.NamespacedName{Name: wordpress.Name, Namespace: wordpress.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForWordpress(wordpress)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// deploymentForMemcached returns a memcached Deployment object
func (r *WordpressReconciler) deploymentForPostgresql(m *wordpressv1alpha1.Wordpress) *appsv1.Deployment {
	replicas := int32(1)
	dbUser := m.Spec.SqlDbUsername
	dbPassword := m.Spec.SqlDbPassword
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-" + m.Name,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app": "mysql-" + m.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"wordpress_cr": "mysql-" + m.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"wordpress_cr": "mysql-" + m.Name,
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: "mysql-pvc-" + m.Name,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "mysql-pvc-" + m.Name,
							},
						},
					}},
					Containers: []corev1.Container{{
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "mysql-pvc-" + m.Name,
							MountPath: "/var/lib/mysql",
						}},
						Env: []corev1.EnvVar{{
							Name:  "MYSQL_DATABASE",
							Value: "wordpress",
						}, {
							Name:  "MYSQL_USER",
							Value: dbUser,
						}, {
							Name:  "MYSQL_ROOT_PASSWORD",
							Value: dbPassword,
						}, {
							Name:  "MYSQL_PASSWORD",
							Value: dbPassword,
						}},
						Image: "mysql:latest",
						Name:  "mysql",
						Ports: []corev1.ContainerPort{{
							ContainerPort: int32(3306),
							Name:          "mysql",
						}},
					}},
				},
			},
		},
	}
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

func (r *WordpressReconciler) serviceForPostgresql(m *wordpressv1alpha1.Wordpress) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-" + m.Name + "-svc",
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app": "mysql-" + m.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"wordpress_cr": "mysql-" + m.Name,
			},
			Ports: []corev1.ServicePort{{
				Name: "mysql",
				Port: int32(3306),
			}},
		},
	}
	ctrl.SetControllerReference(m, svc, r.Scheme)
	return svc
}

func (r *WordpressReconciler) serviceForWordpress(m *wordpressv1alpha1.Wordpress) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name + "-svc",
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app": m.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"wordpress_cr": m.Name,
			},
			Ports: []corev1.ServicePort{{
				Name: "http",
				Port: int32(80),
			}},
		},
	}
	ctrl.SetControllerReference(m, svc, r.Scheme)
	return svc
}

func (r *WordpressReconciler) deploymentForWordpress(m *wordpressv1alpha1.Wordpress) *appsv1.Deployment {
	replicas := int32(1)
	dbUser := m.Spec.SqlDbUsername
	dbPassword := m.Spec.SqlDbPassword
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app": m.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"wordpress_cr": m.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"wordpress_cr": m.Name,
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: "pvc-" + m.Name,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc-" + m.Name,
							},
						},
					}},
					Containers: []corev1.Container{{
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "pvc-" + m.Name,
							MountPath: "/var/www/html",
						}},

						Env: []corev1.EnvVar{{
							Name:  "WORDPRESS_DB_NAME",
							Value: "wordpress",
						}, {
							Name:  "WORDPRESS_DB_USER",
							Value: dbUser,
						}, {
							Name:  "WORDPRESS_DB_PASSWORD",
							Value: dbPassword,
						}, {
							Name:  "WORDPRESS_DB_HOST",
							Value: "mysql-" + m.Name + "-svc",
						}},
						Image: "wordpress:latest",
						Name:  "wordpress",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 80,
							Name:          "http",
						}},
					}},
				},
			},
		},
	}
	// Set Memcached instance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

func (r *WordpressReconciler) pvcForWordpress(m *wordpressv1alpha1.Wordpress) *corev1.PersistentVolumeClaim {
	svc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-" + m.Name,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app": m.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				"ReadWriteOnce",
			},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	ctrl.SetControllerReference(m, svc, r.Scheme)
	return svc
}

func (r *WordpressReconciler) pvcForMysql(m *wordpressv1alpha1.Wordpress) *corev1.PersistentVolumeClaim {
	svc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-pvc-" + m.Name,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app": m.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				"ReadWriteOnce",
			},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	ctrl.SetControllerReference(m, svc, r.Scheme)
	return svc
}

func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

func labelsForWordpress(name string) map[string]string {
	return map[string]string{"app": "wordpress", "wordpress_cr": name}
}

// SetupWithManager sets up the controller with the Manager.
func (r *WordpressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wordpressv1alpha1.Wordpress{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
