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

package v1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// nolint:unused
// log is for logging in this package.
var persistentvolumelog = logf.Log.WithName("persistentvolume-resource")

// SetupPersistentVolumeWebhookWithManager registers the webhook for PersistentVolume in the manager.
func SetupPersistentVolumeWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&corev1.PersistentVolume{}).
		WithValidator(&PersistentVolumeCustomValidator{}).
		WithDefaulter(&PersistentVolumeCustomDefaulter{Client: mgr.GetClient()}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate--v1-persistentvolume,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=persistentvolumes,verbs=create;update,versions=v1,name=mpersistentvolume-v1.kb.io,admissionReviewVersions=v1

// PersistentVolumeCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind PersistentVolume when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type PersistentVolumeCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
	Client client.Client
}

var _ webhook.CustomDefaulter = &PersistentVolumeCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind PersistentVolume.
func (d *PersistentVolumeCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	persistentvolume, ok := obj.(*corev1.PersistentVolume)

	if !ok {
		return fmt.Errorf("expected an PersistentVolume object but got %T", obj)
	}

	if persistentvolume.DeletionTimestamp != nil {
		persistentvolumelog.Info("Skipping defaulting for PersistentVolume being deleted", "name", persistentvolume.Name)
		return nil
	}

	persistentvolumelog.Info("Defaulting for PersistentVolume", "name", persistentvolume.Name)

	// TODO(user): fill in your defaulting logic.

	// only perform defauting if csi driver is "exa.csi.ddn.com"
	if persistentvolume.Spec.CSI == nil || persistentvolume.Spec.CSI.Driver != "exa.csi.ddn.com" {
		persistentvolumelog.Info("Skipping defaulting for PersistentVolume", "name", persistentvolume.Name,
			"reason", "not exa.csi.ddn.com driver")
		return nil
	}

	// if the PersistentVolume is bound to a PersistentVolumeClaim, set the "origin" annotation to the namespace of the PVC
	if persistentvolume.Spec.ClaimRef != nil && persistentvolume.Spec.ClaimRef.Namespace != "" {
		if persistentvolume.Annotations == nil {
			persistentvolume.Annotations = make(map[string]string)
		}
		if _, exists := persistentvolume.Annotations["origin"]; !exists {
			persistentvolume.Annotations["origin"] = persistentvolume.Spec.ClaimRef.Namespace
			persistentvolumelog.Info("Set default annotation origin", "namespace", persistentvolume.Spec.ClaimRef.Namespace)
		}

	} else {
		// if the pv os not bound to a pvc ,we will check if pv has same volumehandle has pvc refer.
		// the match policy is prefix patch .
		// volume Handle is in the format:
		// volumeHandle: exa1:192.168.2.103@tcp;192.168.2.102@tcp;/testfs:/mnt:/nginx-persistent
		// volumeHandle: exa1:192.168.2.103@tcp;192.168.2.102@tcp;/testfs:/exaFS:pvc-exa-c593b5f3-4bff-4f8f-9643-45417de93727-projectId-1241985291
		// first part exa1 should be same , the last part /nginx-persistent should have a prefix match

		exapvList := &corev1.PersistentVolumeList{}
		if err := d.Client.List(ctx, exapvList, client.MatchingFields{"spec.csi.driver": "exa.csi.ddn.com"}); err != nil {
			persistentvolumelog.Error(err, "Failed to list PersistentVolumes")
			return err
		}
		// get the last part of the volume handle and check if it prefix matches with any of the exapvList
		// get volume handle part1 and last part
		volumeHandleParts := strings.Split(persistentvolume.Spec.CSI.VolumeHandle, ":")
		if len(volumeHandleParts) < 2 {
			persistentvolumelog.Info("PersistentVolume does not have a valid VolumeHandle, skipping defaulting",
				"pvname", persistentvolume.Name)
			return nil
		}
		// split the volume handle by ":" and get the first part and last part

		config := volumeHandleParts[0]
		path := volumeHandleParts[len(volumeHandleParts)-1]
		// check if the volume handle has a prefix match with any of the exapv

		for _, exapv := range exapvList.Items {
			if exapv.Spec.CSI != nil && exapv.Spec.CSI.VolumeHandle != "" {

				exaVolumeHandleParts := strings.Split(exapv.Spec.CSI.VolumeHandle, ":")
				if len(exaVolumeHandleParts) < 2 {
					persistentvolumelog.Info("Exa PersistentVolume does not have a valid VolumeHandle, skipping defaulting",
						"pvname", exapv.Name)
					continue
				}
				exaConfig := exaVolumeHandleParts[0]
				exaPath := exaVolumeHandleParts[len(exaVolumeHandleParts)-1]
				if exaConfig == config && hasPathPrefix(path, exaPath) {
					// if the exaPath is a prefix of the path, then set the "origin" annotation to the namespace of the PVC
					if persistentvolume.Annotations == nil {
						persistentvolume.Annotations = make(map[string]string)
					}
					if _, exists := persistentvolume.Annotations["origin"]; !exists {

						if exapv.Spec.ClaimRef != nil && exapv.Spec.ClaimRef.Namespace != "" {
							persistentvolume.Annotations["origin"] = exapv.Spec.ClaimRef.Namespace
							persistentvolumelog.Info("Found binding information with the same prefix path.", "namespace", exapv.Spec.ClaimRef.Namespace, "pvc", exapv.Spec.ClaimRef.Name, "VolumeHandle", exapv.Spec.CSI.VolumeHandle)
							return nil
						}

					}

				}

			}
		}

	}

	return nil
}

func hasPathPrefix(newPath, existingPath string) bool {
	if !strings.HasPrefix(newPath, existingPath) {
		return false
	}

	if newPath == existingPath {
		return true
	}

	suffix := strings.TrimPrefix(newPath, existingPath)
	return strings.HasPrefix(suffix, "/") || strings.HasPrefix(suffix, "-")
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate--v1-persistentvolume,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=persistentvolumes,verbs=create;update,versions=v1,name=vpersistentvolume-v1.kb.io,admissionReviewVersions=v1

// PersistentVolumeCustomValidator struct is responsible for validating the PersistentVolume resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type PersistentVolumeCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &PersistentVolumeCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type PersistentVolume.
func (v *PersistentVolumeCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	persistentvolume, ok := obj.(*corev1.PersistentVolume)
	if !ok {
		return nil, fmt.Errorf("expected a PersistentVolume object but got %T", obj)
	}
	persistentvolumelog.Info("Validation for PersistentVolume upon creation", "name", persistentvolume.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type PersistentVolume.
func (v *PersistentVolumeCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	persistentvolume, ok := newObj.(*corev1.PersistentVolume)
	if !ok {
		return nil, fmt.Errorf("expected a PersistentVolume object for the newObj but got %T", newObj)
	}
	persistentvolumelog.Info("Validation for PersistentVolume upon update", "name", persistentvolume.GetName())

	if persistentvolume.DeletionTimestamp != nil {
		persistentvolumelog.Info("Skipping defaulting for PersistentVolume being deleted", "name", persistentvolume.Name)
		return nil, nil
	}

	if persistentvolume.Spec.CSI == nil || persistentvolume.Spec.CSI.Driver != "exa.csi.ddn.com" {
		persistentvolumelog.Info("Skipping defaulting for PersistentVolume", "name", persistentvolume.Name,
			"reason", "not exa.csi.ddn.com driver")
		return nil, nil
	}

	if oldObj == nil {
		return nil, fmt.Errorf("expected a PersistentVolume object for the oldObj but got nil")
	}
	oldPersistentVolume, ok := oldObj.(*corev1.PersistentVolume)
	if !ok {
		return nil, fmt.Errorf("expected a PersistentVolume object for the oldObj but got %T", oldObj)
	}

	// if old has origin annotation ,we will reject the update /delete of annotation
	if oldPersistentVolume.Annotations != nil {
		if _, exists := oldPersistentVolume.Annotations["origin"]; exists {
			if persistentvolume.Annotations == nil || persistentvolume.Annotations["origin"] != oldPersistentVolume.Annotations["origin"] {
				return nil, fmt.Errorf("cannot update/delete 'origin' annotation, it is immutable")
			}

			if persistentvolume.Spec.ClaimRef != nil && persistentvolume.Spec.ClaimRef.Namespace != "" {
				if oldPersistentVolume.Annotations["origin"] != persistentvolume.Spec.ClaimRef.Namespace {
					return nil, fmt.Errorf("cannot update ClaimRef namespace, it must match the 'origin' annotation")
				}
			}
		} else {
			// if old has no origin annotation, we will check if new pvc claimref must match the new origin annotation
			if persistentvolume.Spec.ClaimRef != nil && persistentvolume.Spec.ClaimRef.Namespace != "" {
				if persistentvolume.Annotations == nil || persistentvolume.Annotations["origin"] != persistentvolume.Spec.ClaimRef.Namespace {
					return nil, fmt.Errorf("cannot update ClaimRef namespace, it must match the 'origin' annotation")
				}
			}

		}

		// if new pvc claimref is not nil and not equal to old origin annotation, we will reject the update

	} else {

		// if old has no origin annotation, we will check if new pvc claimref must match the new origin annotation
		if persistentvolume.Spec.ClaimRef != nil && persistentvolume.Spec.ClaimRef.Namespace != "" {
			if persistentvolume.Annotations == nil || persistentvolume.Annotations["origin"] != persistentvolume.Spec.ClaimRef.Namespace {
				return nil, fmt.Errorf("cannot update ClaimRef namespace, it must match the 'origin' annotation")
			}
		}
	}

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type PersistentVolume.
func (v *PersistentVolumeCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	persistentvolume, ok := obj.(*corev1.PersistentVolume)
	if !ok {
		return nil, fmt.Errorf("expected a PersistentVolume object but got %T", obj)
	}
	persistentvolumelog.Info("Validation for PersistentVolume upon deletion", "name", persistentvolume.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
