package reconciler

import (
	"context"
	"strconv"

	apiv1 "github.com/xataio/xata-cnpg/api/v1"
	apiv1ac "github.com/xataio/xata-cnpg/pkg/client/applyconfiguration/api/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"xata/services/branch-operator/api/v1alpha1"
	"xata/services/branch-operator/pkg/reconciler/resources"
)

const PoolerSuffix = "-pooler"

// desiredPoolerName returns the name of the Pooler the Branch should have, or
// "" when no Pooler is desired. The Pooler is named after the cluster it fronts
// so it matches the Cluster, and only exists when the Branch both specifies a
// pooler configuration and has a cluster.
func desiredPoolerName(branch *v1alpha1.Branch) string {
	if !branch.Spec.Pooler.IsEnabled() || !branch.HasClusterName() {
		return ""
	}
	return branch.ClusterName() + PoolerSuffix
}

// reconcilePooler ensures that the correct Pooler exists for the given Branch
// when a pooler is configured. The Pooler is named after the Branch's cluster
// so it matches the Cluster it fronts. Poolers left behind by a cluster rename,
// a disabled pooler or an unset cluster are removed by reconcileOwnedPoolers.
func (r *BranchReconciler) reconcilePooler(
	ctx context.Context,
	branch *v1alpha1.Branch,
) error {
	name := desiredPoolerName(branch)
	if name == "" {
		return nil
	}

	ac := apiv1ac.Pooler(name, r.ClustersNamespace).
		WithLabels(clusterLabels(branch.Spec.InheritedMetadata)).
		WithOwnerReferences(metav1ac.OwnerReference().
			WithAPIVersion(v1alpha1.GroupVersion.String()).
			WithKind("Branch").
			WithName(branch.Name).
			WithUID(branch.UID).
			WithBlockOwnerDeletion(true).
			WithController(true)).
		WithSpec(resources.PoolerSpec(
			branch.ClusterName(),
			branch.Spec.Pooler.Instances,
			branch.Spec.ClusterSpec.Hibernation.IsEnabled(),
			apiv1.PgBouncerPoolMode(branch.Spec.Pooler.Mode),
			branch.Spec.Pooler.MaxClientConn,
			defaultPoolSize(branch),
			branch.Spec.InheritedMetadata.GetLabels(),
			r.ImagePullSecrets,
			r.Tolerations,
			branch.Spec.ClusterSpec.Affinity.GetNodeSelector(),
		))

	return r.Apply(ctx, ac, client.FieldOwner(OperatorName), client.ForceOwnership)
}

// reconcileOwnedPoolers ensures that only the Pooler named after the Branch's
// cluster is owned by the Branch, deleting any other Poolers owned by the
// Branch. This cleans up poolers left behind by a cluster rename, a disabled
// pooler or an unset cluster
func (r *BranchReconciler) reconcileOwnedPoolers(
	ctx context.Context,
	branch *v1alpha1.Branch,
) (controllerutil.OperationResult, error) {
	keepName := desiredPoolerName(branch)

	var poolerList apiv1.PoolerList
	err := r.List(ctx, &poolerList,
		client.InNamespace(r.ClustersNamespace),
		client.MatchingFields{PoolerOwnerKey: branch.Name},
	)
	if err != nil {
		return "", err
	}

	result := controllerutil.OperationResultNone
	for i := range poolerList.Items {
		pooler := &poolerList.Items[i]

		// Don't delete the Pooler we should keep
		if pooler.Name == keepName {
			continue
		}

		// Don't delete Poolers that are already being deleted
		if !pooler.DeletionTimestamp.IsZero() {
			continue
		}

		if err := r.Delete(ctx, pooler); err != nil {
			return "", client.IgnoreNotFound(err)
		}
		result = controllerutil.OperationResultUpdated
	}

	return result, nil
}

// defaultPoolSize returns the PgBouncer default_pool_size to set for the
// branch. An operator-supplied override on the PoolerSpec wins; otherwise
// the value is derived as floor(0.9 * max_connections) from the branch's
// Postgres parameters. Returns "" when neither is available, which leaves
// the PgBouncer default in place.
func defaultPoolSize(branch *v1alpha1.Branch) string {
	if v := branch.Spec.Pooler.DefaultPoolSize; v != "" {
		// CRD validation enforces ^[1-9][0-9]*$; re-check in case an
		// older CRD let a malformed value through.
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return v
		}
	}
	maxConns := maxConnectionsFromBranch(branch)
	if maxConns <= 0 {
		return ""
	}
	return strconv.Itoa(maxConns * 9 / 10)
}

// maxConnectionsFromBranch extracts the max_connections value from the
// Branch's Postgres parameters. Returns 0 when unset or unparseable.
func maxConnectionsFromBranch(branch *v1alpha1.Branch) int {
	if branch.Spec.ClusterSpec.Postgres == nil {
		return 0
	}
	for _, p := range branch.Spec.ClusterSpec.Postgres.Parameters {
		if p.Name != "max_connections" {
			continue
		}
		v, err := strconv.Atoi(p.Value)
		if err != nil || v <= 0 {
			return 0
		}
		return v
	}
	return 0
}
