package reconciler

import (
	"context"
	"fmt"

	apiv1 "github.com/xataio/xata-cnpg/api/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"xata/services/branch-operator/api/v1alpha1"
)

// updateXVolStatus resolves the XVol associated with the Branch and reports
// its availability via the XVolInfoAvailable condition
func (r *BranchReconciler) updateXVolStatus(ctx context.Context, branch *v1alpha1.Branch) error {
	// The XVol info is unavailable if the Branch has no associated Cluster
	if !branch.HasClusterName() {
		return r.setXVolInfoConditionToFalse(ctx, branch, v1alpha1.BranchHasNoClusterReason)
	}

	// Get the Cluster associated with the Branch
	cluster := &apiv1.Cluster{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      branch.ClusterName(),
		Namespace: r.ClustersNamespace,
	}, cluster)
	if err != nil {
		return fmt.Errorf("get cluster: %w", err)
	}

	// Get the primary PVC name for the Cluster
	pvcName, err := getClusterPVC(cluster)
	if err != nil {
		return r.setXVolInfoConditionToFalse(ctx, branch, v1alpha1.ClusterPVCNotAvailableReason)
	}

	// Get the PVC for the Cluster's current primary instance
	pvc := &v1.PersistentVolumeClaim{}
	err = r.Get(ctx, client.ObjectKey{
		Name:      pvcName,
		Namespace: r.ClustersNamespace,
	}, pvc)
	if err != nil {
		return fmt.Errorf("get pvc %q: %w", pvcName, err)
	}

	// If the PVC does not have a bound PV there is nothing to look up
	pvName := pvc.Spec.VolumeName
	if pvName == "" {
		return r.setXVolInfoConditionToFalse(ctx, branch, v1alpha1.PVNotBoundReason)
	}

	// Get the PV for the PVC
	pv := &v1.PersistentVolume{}
	err = r.Get(ctx, client.ObjectKey{Name: pvName}, pv)
	if err != nil {
		return fmt.Errorf("get pv %q: %w", pvName, err)
	}

	// Record the XVol name on the Branch's status subresource
	return r.recordXVolStatus(ctx, branch, pv)
}

// recordXVolStatus looks up the XVol corresponding to a PV and sets the
// XVolInfoAvailable condition based on the result. On success the name is
// recorded in PrimaryXVolName.
func (r *BranchReconciler) recordXVolStatus(ctx context.Context, branch *v1alpha1.Branch, pv *v1.PersistentVolume) error {
	xvol := &unstructured.Unstructured{}
	xvol.SetGroupVersionKind(xvolGVK)

	// Determine the name of the XVol corresponding to the PV. This defaults to
	// the PV name but can be overridden by an annotation on the PV
	xVolName := pv.Name
	if n, ok := pv.Annotations[v1alpha1.AwokenByXVolAnnotation]; ok {
		xVolName = n
	}

	// Try to get XVol. If the API is not found (ie the CRD is not installed) set
	// the condition to False with an appropriate reason
	err := r.Get(ctx, client.ObjectKey{Name: xVolName}, xvol)
	if meta.IsNoMatchError(err) {
		return r.setXVolInfoConditionToFalse(ctx, branch, v1alpha1.XVolCRDNotInstalledReason)
	}

	// If there is no XVol corresponding to the PV set the condition to False
	// with an appropriate reason.
	if apierrors.IsNotFound(err) {
		return r.setXVolInfoConditionToFalse(ctx, branch, v1alpha1.XVolNotFoundReason)
	}
	if err != nil {
		return err
	}

	// The XVol exists, record the name and set the condition to True
	branch.Status.PrimaryXVolName = xVolName
	return r.setXVolInfoConditionToTrue(ctx, branch, v1alpha1.XVolInfoCollectedReason)
}
