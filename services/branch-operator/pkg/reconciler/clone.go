package reconciler

import (
	"context"
	"fmt"

	"xata/services/branch-operator/api/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileXVolClone ensures that the cloned XVol exists for the given Branch
// if it has a Restore field of type XVolClone.
func (r *BranchReconciler) reconcileXVolClone(ctx context.Context,
	branch *v1alpha1.Branch,
) (controllerutil.OperationResult, error) {
	// If there is no XVolClone-based restore configured for this branch there is
	// nothing to do
	if !branch.Spec.Restore.IsXVolCloneType() {
		return controllerutil.OperationResultNone, nil
	}

	// Construct the expected name of the cloned XVol based on the parent and
	// child branch names
	cloneName := v1alpha1.XVolCloneName(branch.Spec.Restore.Name, branch.Name)

	// Try to get the cloned XVol
	xvol := &unstructured.Unstructured{}
	xvol.SetGroupVersionKind(xvolGVK)
	err := r.Get(ctx, types.NamespacedName{Name: cloneName}, xvol)
	if err != nil && !apierrors.IsNotFound(err) {
		return controllerutil.OperationResultNone, err
	}

	// If the clone doesn't exist, create it
	if apierrors.IsNotFound(err) {
		return r.createClonedXVol(ctx, branch, cloneName)
	}

	// If the cloned XVol exists there is nothing to do
	return controllerutil.OperationResultNone, nil
}

// createClonedXVol creates the cloned XVol for the given child Branch,
// setting the target of the clone to the the parent Branch's XVol
func (r *BranchReconciler) createClonedXVol(
	ctx context.Context,
	branch *v1alpha1.Branch,
	cloneName string,
) (controllerutil.OperationResult, error) {
	// Fetch the parent Branch resource
	parentBranch := v1alpha1.Branch{}
	err := r.Get(ctx, types.NamespacedName{
		Name: branch.Spec.Restore.Name,
	}, &parentBranch)
	if apierrors.IsNotFound(err) {
		return controllerutil.OperationResultNone, &ConditionError{
			ConditionType:   v1alpha1.BranchReadyConditionType,
			ConditionReason: v1alpha1.ParentBranchNotFoundReason,
			Err:             err,
		}
	}
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Get the parent Branch's primary XVol name from its status
	parentXVolName := parentBranch.Status.PrimaryXVolName
	if parentXVolName == "" {
		return controllerutil.OperationResultNone, &ConditionError{
			ConditionType:   v1alpha1.BranchReadyConditionType,
			ConditionReason: v1alpha1.ParentBranchHasNoXVolReason,
			Err:             fmt.Errorf("parent branch %q has no XVol", branch.Spec.Restore.Name),
		}
	}

	// Build the desired XVol
	xvol := &unstructured.Unstructured{}
	xvol.SetGroupVersionKind(xvolGVK)
	xvol.SetName(cloneName)

	// Set the labels on the cloned XVol
	ensureLabels(xvol, branch.Spec.InheritedMetadata)

	// Set the source volume on the cloned XVol to be the parent Branch's XVol
	if err := unstructured.SetNestedField(xvol.Object, parentXVolName, "spec", "source", "volume"); err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Set the reclaim policy on the cloned XVol to Retain
	if err := unstructured.SetNestedField(xvol.Object, "Retain", "spec", "xvolReclaimPolicy"); err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Set the owner reference on the cloned XVol to be the child Branch
	if err := controllerutil.SetOwnerReference(branch, xvol, r.Scheme); err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Create the cloned XVol
	if err := r.Create(ctx, xvol); err != nil && !apierrors.IsAlreadyExists(err) {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultCreated, nil
}
