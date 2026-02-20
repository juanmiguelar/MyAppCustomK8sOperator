package controller

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	myappsv1 "github.com/juanmiguelar/myapp-operator/api/v1"
)

type MyAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.example.com,resources=myapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.example.com,resources=myapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *MyAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch — get my CRD myapp
	myApp := &myappsv1.MyApp{}
	if err := r.Get(ctx, req.NamespacedName, myApp); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil // was removed, nothing to do
		}
		return ctrl.Result{}, err
	}

	// 2. Config of deployment
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      myApp.Name,
			Namespace: myApp.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &myApp.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": myApp.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": myApp.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  myApp.Name,
						Image: myApp.Spec.Image,
					}},
				},
			},
		},
	}

	// 3. Ownership — if myApp gets deleted also the deployment
	if err := controllerutil.SetControllerReference(myApp, deploy, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// 4. Create or Update — idempotente
	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, existing)
	if errors.IsNotFound(err) {
		log.Info("Creating Deployment", "name", deploy.Name)
		return ctrl.Result{}, r.Create(ctx, deploy)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// 5. Update status with the state of deplyment
	myApp.Status.ReadyReplicas = existing.Status.ReadyReplicas
	if err := r.Status().Update(ctx, myApp); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func (r *MyAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&myappsv1.MyApp{}).
		Named("myapp").
		Complete(r)
}
