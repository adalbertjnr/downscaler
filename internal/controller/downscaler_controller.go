/*
Copyright 2024.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	downscalergov1alpha1 "github.com/adalbertjnr/downscaler/api/v1alpha1"
	"github.com/adalbertjnr/downscaler/internal/manager"
	"github.com/go-logr/logr"
)

// DownscalerReconciler reconciles a Downscaler object
type DownscalerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger logr.Logger

	DownscalerScheduler *manager.Downscaler
}

//+kubebuilder:rbac:groups=downscaler.go,resources=downscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=downscaler.go,resources=downscalers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=downscaler.go,resources=downscalers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Downscaler object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *DownscalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	dc := r.DownscalerScheduler

	var app downscalergov1alpha1.Downscaler
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		ctrl.Log.Error(err, "reconcile", "error", err)
		return ctrl.Result{}, err
	}

	r.Logger.Info("reconcile", "kind", app.Kind, "name", app.Name, "status", "updated")

	if valid := dc.Add(ctx, app).
		Validate(); !valid {
		return ctrl.Result{}, nil
	}

	return dc.Run()
}

// SetupWithManager sets up the controller with the Manager.
func (r *DownscalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&downscalergov1alpha1.Downscaler{}).
		Complete(r)
}
