package reconciler

import (
	"context"
	"fmt"

	apiv1 "github.com/xataio/xata-cnpg/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"xata/services/branch-operator/api/v1alpha1"
)

var xvolGVK = schema.GroupVersionKind{
	Group:   "xata.io",
	Version: "v1alpha1",
	Kind:    "Xvol",
}

// reconcileXVolOwnership ensures that XVols backing the Branch's owned Cluster
// have their reclaim policy set to Retain and their owner reference set to the
// Branch when the Branch does not specify a Cluster name.
//
// This step ensures that the XVols will be retained after a subsequent
// reconciliation step (reconcileOwnedClusters) deletes the Cluster.
func (r *BranchReconciler) reconcileXVolOwnership(
	ctx context.Context,
	branch *v1alpha1.Branch,
) (controllerutil.OperationResult, error) {
	// If the Branch has a Cluster name, there is nothing to do - the Cluster
	// will not be deleted later during reconciliation, so there is no need to
	// protect the XVols.
	if branch.HasClusterName() {
		return controllerutil.OperationResultNone, nil
	}

	// List all Clusters owned by the Branch.
	var clusterList apiv1.ClusterList
	err := r.List(ctx, &clusterList,
		client.InNamespace(r.ClustersNamespace),
		client.MatchingFields{ClusterOwnerKey: branch.Name},
	)
	if err != nil {
		return "", fmt.Errorf("list owned clusters: %w", err)
	}

	// Set the reclaim policy to Retain for XVols used by the owned Cluster.
	result := controllerutil.OperationResultNone
	for _, cluster := range clusterList.Items {
		// Ignore any Clusters that are already being deleted
		if !cluster.DeletionTimestamp.IsZero() {
			continue
		}

		// Patch the reclaim policy on the Cluster XVols
		for _, pvcName := range cluster.Status.HealthyPVC {
			patched, err := r.ensureXVolRetained(ctx, branch, pvcName)
			if err != nil {
				return "", fmt.Errorf("ensure XVol retained for PVC %s: %w", pvcName, err)
			}
			if patched {
				result = controllerutil.OperationResultUpdated
			}
		}
	}

	return result, nil
}

// ensureXVolRetained patches the XVol backing the given PVC to have a reclaim
// policy of Retain and sets the Branch as an owner of the XVol. The owner
// reference ensures the XVol is cleaned up by Kubernetes garbage collection
// when the Branch is deleted. It returns true if the XVol was patched, or
// false if the XVol did not need to be patched.
func (r *BranchReconciler) ensureXVolRetained(ctx context.Context, branch *v1alpha1.Branch, pvcName string) (bool, error) {
	// Get the PVC
	var pvc corev1.PersistentVolumeClaim
	if err := r.Get(ctx, client.ObjectKey{
		Name:      pvcName,
		Namespace: r.ClustersNamespace,
	}, &pvc); err != nil {
		return false, client.IgnoreNotFound(err)
	}

	// If the PVC does not have a bound PV there is nothing to do
	pvName := pvc.Spec.VolumeName
	if pvName == "" {
		return false, nil
	}

	// Get the name of the XVol corresponding to the PV
	xVolName, err := r.xVolNameForPV(ctx, pvName)
	if err != nil {
		return false, fmt.Errorf("get xvol name for pv %q: %w", pvName, err)
	}

	// Get the XVol. We have to use Unstructured here because the XVol types are
	// in the private xatastor repository so we can't import them directly.
	xvol := &unstructured.Unstructured{}
	xvol.SetGroupVersionKind(xvolGVK)
	err = r.Get(ctx, client.ObjectKey{Name: xVolName}, xvol)
	if err != nil {
		// If the XVol CRD is not installed, treat it the same as if the XVol did
		// not exist - there is nothing to protect.
		if meta.IsNoMatchError(err) {
			return false, nil
		}
		return false, client.IgnoreNotFound(err)
	}

	// Get the current reclaim policy and check if the Branch is already an
	// owner of the XVol.
	reclaimPolicy, _, _ := unstructured.NestedString(xvol.Object, "spec", "xvolReclaimPolicy")
	hasRetainReclaimPolicy := reclaimPolicy == "Retain"
	hasOwnerRef, err := controllerutil.HasOwnerReference(xvol.GetOwnerReferences(), branch, r.Scheme)
	if err != nil {
		return false, fmt.Errorf("check owner reference on XVol %s: %w", pvName, err)
	}

	// If the XVol already has Retain reclaim policy and the Branch owner
	// reference there is nothing to do
	if hasRetainReclaimPolicy && hasOwnerRef {
		return false, nil
	}

	// Create a merge patch to update the XVol
	patch := client.MergeFrom(xvol.DeepCopy())

	// Patch the XVol to have a reclaim policy of Retain.
	err = unstructured.SetNestedField(xvol.Object, "Retain", "spec", "xvolReclaimPolicy")
	if err != nil {
		return false, fmt.Errorf("set reclaim policy on XVol %s: %w", pvName, err)
	}

	// Set the Branch as an owner of the XVol so it is cleaned up by Kubernetes
	// garbage collection if the branch is deleted before it is woken up
	err = controllerutil.SetOwnerReference(branch, xvol, r.Scheme)
	if err != nil {
		return false, fmt.Errorf("set owner reference on XVol %s: %w", pvName, err)
	}

	// Apply the patch to update the XVol
	err = r.Patch(ctx, xvol, patch)
	if err != nil {
		return false, fmt.Errorf("patch XVol %s: %w", pvName, err)
	}

	return true, nil
}
