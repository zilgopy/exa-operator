/*
Copyright 2025.

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// DummyReconciler reconciles a Dummy object
type DummyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=zilgopy,resources=dummies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=zilgopy,resources=dummies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=zilgopy,resources=dummies/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Dummy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *DummyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the PersistentVolume instance
	pv := &corev1.PersistentVolume{}
	if err := r.Get(ctx, req.NamespacedName, pv); err != nil {
		if errors.IsNotFound(err) {
			log.Info("PersistentVolume not found ignoring.", "pvname", req.Name)
			// If the PersistentVolume is not found, we can ignore the error.
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PersistentVolume")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if the PersistentVolume has the "origin" annotation
	_, exists := pv.Annotations["origin"]
	if !exists {
		log.Info("PersistentVolume does not have 'origin' annotation.", "pvname", req.Name)
		// If the annotation does not exist, we can read from its bound pvc to get the ns
		//check if the PersistentVolume is bound to a PersistentVolumeClaim
		if pv.Spec.ClaimRef == nil {
			log.Info("PersistentVolume is not bound to any PersistentVolumeClaim.", "pvname", req.Name)
			return ctrl.Result{}, nil
		}
		pvcNamespace := pv.Spec.ClaimRef.Namespace
		log.Info("Reading PVC namespace from PersistentVolume.", "pvname", req.Name)
		// then update annotation origin = pvcNamespace
		if pv.Annotations == nil {
			pv.Annotations = make(map[string]string)
		}
		pv.Annotations["origin"] = pvcNamespace
		if err := r.Update(ctx, pv); err != nil {
			log.Error(err, "Failed to update PersistentVolume with 'origin' annotation name.", "pvname", req.Name)
			return ctrl.Result{}, err
		}
		log.Info("Updated PersistentVolume with 'origin' annotation name", "pvname", pv.Name, "origin namespace", pvcNamespace)

	}

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
// only filter events for PersistentVolume with annotation "origin" are processed
func (r *DummyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&corev1.PersistentVolume{}).WithEventFilter(predicate.AnnotationChangedPredicate{}).
		// Uncomment the following line to enable leader election for controller manager.
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		// For().
		Named("dummy").
		Complete(r)
}
