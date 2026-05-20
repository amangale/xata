package reconciler_test

import (
	"context"
	"testing"

	"xata/services/branch-operator/api/v1alpha1"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestXVolCloneReconciliation(t *testing.T) {
	t.Parallel()

	t.Run("cloned XVol is created from parent's primary XVol", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		parentBranch := NewBranchBuilder().Build()
		parentXVolName := randomString(10)

		withBranch(ctx, t, parentBranch, func(t *testing.T, parentBr *v1alpha1.Branch) {
			// Set the parent Branch's PrimaryXVolName so the child has something
			// to clone from.
			err := retryStatusOnConflict(ctx, parentBr, func(b *v1alpha1.Branch) {
				b.Status.PrimaryXVolName = parentXVolName
			})
			require.NoError(t, err)

			// Create a child Branch with a Restore of type XVolClone that references
			// the parent Branch
			childBranch := NewBranchBuilder().
				WithClusterName(nil).
				WithRestore(v1alpha1.RestoreTypeXVolClone, parentBr.Name).
				Build()

			withBranch(ctx, t, childBranch, func(t *testing.T, childBr *v1alpha1.Branch) {
				cloneName := v1alpha1.XVolCloneName(parentBr.Name, childBr.Name)

				// Expect the clone XVol to be created
				clone := &unstructured.Unstructured{}
				clone.SetGroupVersionKind(xvolGVK)
				requireEventuallyNoErr(t, func() error {
					return k8sClient.Get(ctx, client.ObjectKey{Name: cloneName}, clone)
				})

				// spec.source.volume points at the parent's primary XVol
				gotSource, found, err := unstructured.NestedString(clone.Object, "spec", "source", "volume")
				require.NoError(t, err)
				require.True(t, found, "spec.source.volume not set")
				require.Equal(t, parentXVolName, gotSource)

				// spec.xvolReclaimPolicy is Retain
				gotReclaim, found, err := unstructured.NestedString(clone.Object, "spec", "xvolReclaimPolicy")
				require.NoError(t, err)
				require.True(t, found, "spec.xvolReclaimPolicy not set")
				require.Equal(t, "Retain", gotReclaim)

				// The child Branch is an owner of the clone
				ownerRefs := clone.GetOwnerReferences()
				require.Len(t, ownerRefs, 1)
				require.Equal(t, childBr.Name, ownerRefs[0].Name)
				require.Equal(t, "Branch", ownerRefs[0].Kind)
			})
		})
	})

	t.Run("cloned XVol reconciliation fails when parent branch has no XVol", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		// Parent Branch with no PrimaryXVolName set on its status
		parentBranch := NewBranchBuilder().Build()

		withBranch(ctx, t, parentBranch, func(t *testing.T, parentBr *v1alpha1.Branch) {
			childBranch := NewBranchBuilder().
				WithClusterName(nil).
				WithRestore(v1alpha1.RestoreTypeXVolClone, parentBr.Name).
				Build()

			withBranch(ctx, t, childBranch, func(t *testing.T, childBr *v1alpha1.Branch) {
				cloneName := v1alpha1.XVolCloneName(parentBr.Name, childBr.Name)

				// Expect the Ready condition to reflect the parent's missing XVol
				// and no clone XVol to have been created
				requireEventuallyTrue(t, func() bool {
					br := v1alpha1.Branch{}
					if err := getK8SObject(ctx, childBr.Name, &br); err != nil {
						return false
					}
					if !isReadyConditionFalseWithReason(br, v1alpha1.ParentBranchHasNoXVolReason) {
						return false
					}

					// Expect no clone XVol to have been created
					clone := &unstructured.Unstructured{}
					clone.SetGroupVersionKind(xvolGVK)
					err := k8sClient.Get(ctx, client.ObjectKey{Name: cloneName}, clone)
					return apierrors.IsNotFound(err)
				})
			})
		})
	})

	t.Run("cloned XVol reconciliation fails when parent branch is missing", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		// Create a child Branch that clones from a non-existent parent
		childBranch := NewBranchBuilder().
			WithClusterName(nil).
			WithRestore(v1alpha1.RestoreTypeXVolClone, "does-not-exist").
			Build()

		withBranch(ctx, t, childBranch, func(t *testing.T, childBr *v1alpha1.Branch) {
			cloneName := v1alpha1.XVolCloneName("does-not-exist", childBr.Name)

			// Expect the Ready condition to reflect the missing parent and no
			// clone XVol to have been created
			requireEventuallyTrue(t, func() bool {
				br := v1alpha1.Branch{}
				if err := getK8SObject(ctx, childBr.Name, &br); err != nil {
					return false
				}
				if !isReadyConditionFalseWithReason(br, v1alpha1.ParentBranchNotFoundReason) {
					return false
				}

				// Expect no clone XVol to have been created
				clone := &unstructured.Unstructured{}
				clone.SetGroupVersionKind(xvolGVK)
				err := k8sClient.Get(ctx, client.ObjectKey{Name: cloneName}, clone)
				return apierrors.IsNotFound(err)
			})
		})
	})

	t.Run("cloned XVol reconciliation succeeds after parent branch is deleted", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		parentBranch := NewBranchBuilder().Build()
		parentXVolName := randomString(10)

		withBranch(ctx, t, parentBranch, func(t *testing.T, parentBr *v1alpha1.Branch) {
			// Set the parent Branch's PrimaryXVolName so the child has something
			// to clone from.
			err := retryStatusOnConflict(ctx, parentBr, func(b *v1alpha1.Branch) {
				b.Status.PrimaryXVolName = parentXVolName
			})
			require.NoError(t, err)

			// Create a child Branch with a Restore of type XVolClone that references
			// the parent Branch
			childBranch := NewBranchBuilder().
				WithClusterName(nil).
				WithRestore(v1alpha1.RestoreTypeXVolClone, parentBr.Name).
				Build()

			withBranch(ctx, t, childBranch, func(t *testing.T, childBr *v1alpha1.Branch) {
				cloneName := v1alpha1.XVolCloneName(parentBr.Name, childBr.Name)

				// Wait for the clone XVol to be created from the parent
				clone := &unstructured.Unstructured{}
				clone.SetGroupVersionKind(xvolGVK)
				requireEventuallyNoErr(t, func() error {
					return k8sClient.Get(ctx, client.ObjectKey{Name: cloneName}, clone)
				})

				// Delete the parent Branch.
				require.NoError(t, k8sClient.Delete(ctx, parentBr))

				// Force a reconcile of the child by clearing the cluster name
				err := retryOnConflict(ctx, childBr, func(b *v1alpha1.Branch) {
					b.Spec.ClusterSpec.Name = nil
				})
				require.NoError(t, err)

				// Wait for the child Branch to be reconciled to Ready=True. The
				// post-deletion reconcile succeeds despite the parent Branch no longer
				// existing.
				requireEventuallyTrue(t, func() bool {
					br := v1alpha1.Branch{}
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: childBr.Name}, &br); err != nil {
						return false
					}
					if br.Status.ObservedGeneration != br.Generation {
						return false
					}
					return meta.IsStatusConditionTrue(br.Status.Conditions, v1alpha1.BranchReadyConditionType)
				})
			})
		})
	})
}
