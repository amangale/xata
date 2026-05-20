package reconciler_test

import (
	"context"
	"testing"

	"xata/services/branch-operator/api/v1alpha1"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/xataio/xata-cnpg/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestXVolStatus(t *testing.T) {
	t.Parallel()

	t.Run("XVol info is unavailable when branch has no cluster", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		// Create a Branch with no associated Cluster
		branch := NewBranchBuilder().
			WithClusterName(nil).
			Build()

		withBranch(ctx, t, branch, func(t *testing.T, br *v1alpha1.Branch) {
			// Wait for the XVolInfoAvailable condition to be set
			requireEventuallyTrue(t, func() bool {
				err := getK8SObject(ctx, br.Name, br)
				if err != nil {
					return false
				}
				c := meta.FindStatusCondition(br.Status.Conditions, v1alpha1.XVolInfoAvailableConditionType)
				if c == nil {
					return false
				}
				return c.Status != metav1.ConditionUnknown
			})

			// Expect XVolInfoAvailable to be False because the Branch has no
			// Cluster associated with it
			c := meta.FindStatusCondition(br.Status.Conditions, v1alpha1.XVolInfoAvailableConditionType)
			require.NotNil(t, c)
			require.Equal(t, metav1.ConditionFalse, c.Status)
			require.Equal(t, v1alpha1.BranchHasNoClusterReason, c.Reason)

			// Assert PrimaryXVolName is empty
			require.Empty(t, br.Status.PrimaryXVolName)
		})
	})

	t.Run("XVol info available when Cluster exists", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		branch := NewBranchBuilder().Build()

		withBranch(ctx, t, branch, func(t *testing.T, br *v1alpha1.Branch) {
			clusterName := br.Name

			// Wait for the reconciler to create the CNPG Cluster
			cluster := apiv1.Cluster{}
			requireEventuallyNoErr(t, func() error {
				return getK8SObject(ctx, clusterName, &cluster)
			})

			xvolName, pvcName, _ := createPVCAndXVol(ctx, t, clusterName)

			// Set the Cluster's CurrentPrimary so getClusterPVC resolves
			setClusterStatus(ctx, t, &cluster, apiv1.ClusterStatus{
				CurrentPrimary: pvcName,
			})

			// Trigger re-reconciliation by updating a spec field
			err := retryOnConflict(ctx, br, func(b *v1alpha1.Branch) {
				b.Spec.ClusterSpec.Instances = 2
			})
			require.NoError(t, err)

			// Assert PrimaryXVolName is set and XVolInfoAvailable is True
			requireEventuallyTrue(t, func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(br), br)
				if err != nil {
					return false
				}
				return br.Status.PrimaryXVolName == xvolName &&
					meta.IsStatusConditionTrue(br.Status.Conditions, v1alpha1.XVolInfoAvailableConditionType)
			})
		})
	})

	t.Run("PrimaryXVolName is retained when cluster name is removed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		branch := NewBranchBuilder().Build()

		withBranch(ctx, t, branch, func(t *testing.T, br *v1alpha1.Branch) {
			clusterName := br.Name

			// Wait for the reconciler to create the CNPG Cluster
			cluster := apiv1.Cluster{}
			requireEventuallyNoErr(t, func() error {
				return getK8SObject(ctx, clusterName, &cluster)
			})

			xvolName, pvcName, _ := createPVCAndXVol(ctx, t, clusterName)

			// Set the Cluster's CurrentPrimary so getClusterPVC resolves
			setClusterStatus(ctx, t, &cluster, apiv1.ClusterStatus{
				CurrentPrimary: pvcName,
			})

			// Trigger re-reconciliation by updating a spec field
			err := retryOnConflict(ctx, br, func(b *v1alpha1.Branch) {
				b.Spec.ClusterSpec.SmartShutdownTimeout = new(int32(60))
			})
			require.NoError(t, err)

			// Wait for PrimaryXVolName to be set
			requireEventuallyTrue(t, func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(br), br)
				if err != nil {
					return false
				}
				return br.Status.PrimaryXVolName == xvolName
			})

			// Remove the cluster name from the branch
			err = retryOnConflict(ctx, br, func(b *v1alpha1.Branch) {
				b.Spec.ClusterSpec.Name = nil
			})
			require.NoError(t, err)

			// Wait for the XVolInfoAvailable condition to flip to False
			requireEventuallyTrue(t, func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(br), br)
				if err != nil {
					return false
				}
				return meta.IsStatusConditionFalse(br.Status.Conditions, v1alpha1.XVolInfoAvailableConditionType)
			})

			// PrimaryXVolName is retained after the cluster name is removed
			c := meta.FindStatusCondition(br.Status.Conditions, v1alpha1.XVolInfoAvailableConditionType)
			require.NotNil(t, c)
			require.Equal(t, metav1.ConditionFalse, c.Status)
			require.Equal(t, v1alpha1.BranchHasNoClusterReason, c.Reason)
			require.Equal(t, xvolName, br.Status.PrimaryXVolName)
		})
	})

	t.Run("PrimaryXVolName re-reconciles when primary PVC changes", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		branch := NewBranchBuilder().Build()

		withBranch(ctx, t, branch, func(t *testing.T, br *v1alpha1.Branch) {
			clusterName := br.Name

			cluster := apiv1.Cluster{}
			requireEventuallyNoErr(t, func() error {
				return getK8SObject(ctx, clusterName, &cluster)
			})

			// Initial primary: PVC/PV/XVol set A.
			xvolNameA, pvcNameA, _ := createPVCAndXVol(ctx, t, clusterName)
			setClusterStatus(ctx, t, &cluster, apiv1.ClusterStatus{
				CurrentPrimary: pvcNameA,
			})

			// Trigger a reconcile by scaling the cluster to 2 instances
			err := retryOnConflict(ctx, br, func(b *v1alpha1.Branch) {
				b.Spec.ClusterSpec.Instances = 2
			})
			require.NoError(t, err)

			requireEventuallyTrue(t, func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(br), br); err != nil {
					return false
				}
				return br.Status.PrimaryXVolName == xvolNameA
			})

			// Create a second PVC/PV/XVol set B and point the Cluster's
			// CurrentPrimary at it.
			xvolNameB := clusterName + "-xvol-b"
			pvcNameB := clusterName + "-2"
			createBoundPVCAndXVol(ctx, t, pvcNameB, xvolNameB, "")

			// Flip the Cluster's CurrentPrimary to PVC B and trigger a reconcile.
			setClusterStatus(ctx, t, &cluster, apiv1.ClusterStatus{
				CurrentPrimary: pvcNameB,
			})
			err = retryOnConflict(ctx, br, func(b *v1alpha1.Branch) {
				b.Spec.ClusterSpec.Instances = 3
			})
			require.NoError(t, err)

			// PrimaryXVolName re-reconciles to B.
			requireEventuallyTrue(t, func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(br), br); err != nil {
					return false
				}
				return br.Status.PrimaryXVolName == xvolNameB
			})
		})
	})

	t.Run("PrimaryXVolName is taken from PV annotation when present", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		branch := NewBranchBuilder().Build()

		withBranch(ctx, t, branch, func(t *testing.T, br *v1alpha1.Branch) {
			clusterName := br.Name

			cluster := apiv1.Cluster{}
			requireEventuallyNoErr(t, func() error {
				return getK8SObject(ctx, clusterName, &cluster)
			})

			// The XVol name differs from the PV name; the PV carries an annotation
			// that records the XVol it is backed by. This models xatastor-slot mode,
			// where the PV/XVol name relationship is not implicit.
			pvName := clusterName + "-pv"
			xvolName := clusterName + "-slot-xvol"
			pvcName := clusterName + "-1"
			createBoundPVCAndXVol(ctx, t, pvcName, xvolName, pvName)

			setClusterStatus(ctx, t, &cluster, apiv1.ClusterStatus{
				CurrentPrimary: pvcName,
			})

			err := retryOnConflict(ctx, br, func(b *v1alpha1.Branch) {
				b.Spec.ClusterSpec.Instances = 2
			})
			require.NoError(t, err)

			// PrimaryXVolName is the annotated name, not the PV name.
			requireEventuallyTrue(t, func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(br), br); err != nil {
					return false
				}
				return br.Status.PrimaryXVolName == xvolName &&
					meta.IsStatusConditionTrue(br.Status.Conditions, v1alpha1.XVolInfoAvailableConditionType)
			})
		})
	})
}
