package resources_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/xataio/xata-cnpg/api/v1"
	apiv1ac "github.com/xataio/xata-cnpg/pkg/client/applyconfiguration/api/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"

	"xata/services/branch-operator/pkg/reconciler/resources"
)

func TestPoolerSpec(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		clusterName      string
		instances        int32
		hibernated       bool
		poolMode         apiv1.PgBouncerPoolMode
		maxClientConn    string
		defaultPoolSize  string
		podLabels        map[string]string
		imagePullSecrets []string
		tolerations      []v1.Toleration
		nodeSelector     map[string]string
		expected         *apiv1ac.PoolerSpecApplyConfiguration
	}{
		"active branch": {
			clusterName:   "test-branch-1",
			instances:     1,
			hibernated:    false,
			poolMode:      apiv1.PgBouncerPoolModeSession,
			maxClientConn: "100",
			expected:      baseExpectedPoolerSpec("test-branch-1", 1, apiv1.PgBouncerPoolModeSession, "100"),
		},
		"hibernated branch": {
			clusterName:   "test-branch-2",
			instances:     1,
			hibernated:    true,
			poolMode:      apiv1.PgBouncerPoolModeSession,
			maxClientConn: "100",
			expected:      baseExpectedPoolerSpec("test-branch-2", 0, apiv1.PgBouncerPoolModeSession, "100"),
		},
		"with pod labels": {
			clusterName:   "test-branch-3",
			instances:     1,
			hibernated:    false,
			poolMode:      apiv1.PgBouncerPoolModeSession,
			maxClientConn: "100",
			podLabels: map[string]string{
				"xata.io/organizationID": "org-123",
				"xata.io/projectID":      "proj-456",
			},
			expected: baseExpectedPoolerSpec("test-branch-3", 1, apiv1.PgBouncerPoolModeSession, "100").
				WithTemplate(apiv1ac.PodTemplateSpec().
					WithObjectMeta(apiv1ac.Metadata().WithLabels(map[string]string{
						"xata.io/organizationID": "org-123",
						"xata.io/projectID":      "proj-456",
					})).
					WithSpec(basePoolerPodSpec())),
		},
		"with image pull secrets": {
			clusterName:      "test-branch-4",
			instances:        1,
			hibernated:       false,
			poolMode:         apiv1.PgBouncerPoolModeSession,
			maxClientConn:    "100",
			imagePullSecrets: []string{"ghcr-secret", "ecr-secret"},
			expected: baseExpectedPoolerSpec("test-branch-4", 1, apiv1.PgBouncerPoolModeSession, "100").
				WithTemplate(apiv1ac.PodTemplateSpec().WithSpec(func() v1.PodSpec {
					s := basePoolerPodSpec()
					s.ImagePullSecrets = []v1.LocalObjectReference{
						{Name: "ghcr-secret"},
						{Name: "ecr-secret"},
					}
					return s
				}())),
		},
		"with tolerations and node selector": {
			clusterName:   "test-branch-6",
			instances:     1,
			hibernated:    false,
			poolMode:      apiv1.PgBouncerPoolModeTransaction,
			maxClientConn: "100",
			tolerations: []v1.Toleration{
				{
					Key:      "xata.io/workload",
					Value:    "dataplane",
					Operator: v1.TolerationOpEqual,
					Effect:   v1.TaintEffectNoSchedule,
				},
			},
			nodeSelector: map[string]string{
				"xata.io/nodepool": "dataplane",
			},
			expected: baseExpectedPoolerSpec("test-branch-6", 1, apiv1.PgBouncerPoolModeTransaction, "100").
				WithTemplate(apiv1ac.PodTemplateSpec().WithSpec(func() v1.PodSpec {
					s := basePoolerPodSpec()
					s.Tolerations = []v1.Toleration{
						{
							Key:      "xata.io/workload",
							Value:    "dataplane",
							Operator: v1.TolerationOpEqual,
							Effect:   v1.TaintEffectNoSchedule,
						},
					}
					s.NodeSelector = map[string]string{
						"xata.io/nodepool": "dataplane",
					}
					return s
				}())),
		},
		"with default_pool_size override": {
			clusterName:     "test-branch-5",
			instances:       1,
			hibernated:      false,
			poolMode:        apiv1.PgBouncerPoolModeTransaction,
			maxClientConn:   "10000",
			defaultPoolSize: "180",
			expected: baseExpectedPoolerSpec("test-branch-5", 1, apiv1.PgBouncerPoolModeTransaction, "10000").
				WithPgBouncer(apiv1ac.PgBouncerSpec().
					WithPoolMode(apiv1.PgBouncerPoolModeTransaction).
					WithParameters(map[string]string{
						"max_client_conn":         "10000",
						"max_prepared_statements": "1000",
						"query_wait_timeout":      "120",
						"default_pool_size":       "180",
						"server_idle_timeout":     "60",
					})),
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			got := resources.PoolerSpec(tc.clusterName, tc.instances, tc.hibernated, tc.poolMode, tc.maxClientConn, tc.defaultPoolSize, tc.podLabels, tc.imagePullSecrets, tc.tolerations, tc.nodeSelector)

			require.Equal(t, tc.expected, got)
		})
	}
}

// baseExpectedPoolerSpec builds the apply-config that PoolerSpec is expected
// to return for the most common case: standard pgbouncer parameters, no pod
// labels, base pod spec. Test cases override individual fields via .WithX
// chaining.
func baseExpectedPoolerSpec(
	clusterName string,
	instances int32,
	poolMode apiv1.PgBouncerPoolMode,
	maxClientConn string,
) *apiv1ac.PoolerSpecApplyConfiguration {
	return apiv1ac.PoolerSpec().
		WithCluster(corev1ac.LocalObjectReference().WithName(clusterName)).
		WithType(apiv1.PoolerTypeRW).
		WithInstances(instances).
		WithPgBouncer(apiv1ac.PgBouncerSpec().
			WithPoolMode(poolMode).
			WithParameters(map[string]string{
				"max_client_conn":         maxClientConn,
				"max_prepared_statements": "1000",
				"query_wait_timeout":      "120",
				"server_idle_timeout":     "60",
			})).
		WithServiceTemplate(apiv1ac.ServiceTemplateSpec().
			WithObjectMeta(apiv1ac.Metadata().WithAnnotations(resources.InheritedAnnotations))).
		WithTemplate(apiv1ac.PodTemplateSpec().WithSpec(basePoolerPodSpec()))
}

// basePoolerPodSpec returns the standard pgbouncer pod spec that PoolerSpec
// produces when no overrides (image pull secrets, tolerations, node selector)
// are supplied.
func basePoolerPodSpec() v1.PodSpec {
	return v1.PodSpec{
		EnableServiceLinks: new(false),
		Containers: []v1.Container{
			{
				Name: "pgbouncer",
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("200m"),
						v1.ResourceMemory: resource.MustParse("100Mi"),
					},
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("500m"),
						v1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
		},
	}
}
