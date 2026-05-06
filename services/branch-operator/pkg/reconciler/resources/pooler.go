package resources

import (
	apiv1 "github.com/xataio/xata-cnpg/api/v1"
	apiv1ac "github.com/xataio/xata-cnpg/pkg/client/applyconfiguration/api/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

// PoolerSpec defines the PoolerSpec for a CNPG PgBouncer connection pooler.
// It creates a PgBouncer instance in transaction mode with max_client_conn
// set to the maximum. When hibernated, instances is set to 0.
// When defaultPoolSize is non-empty, it is set verbatim on PgBouncer.
func PoolerSpec(clusterName string,
	instances int32,
	hibernated bool,
	poolMode apiv1.PgBouncerPoolMode,
	maxClientConn, defaultPoolSize string,
	podLabels map[string]string,
	imagePullSecrets []string,
	tolerations []v1.Toleration,
	nodeSelector map[string]string,
) *apiv1ac.PoolerSpecApplyConfiguration {
	if hibernated {
		instances = 0
	}

	params := map[string]string{
		"max_client_conn":         maxClientConn,
		"max_prepared_statements": "1000",
		"query_wait_timeout":      "120",
		"server_idle_timeout":     "60",
	}
	if defaultPoolSize != "" {
		params["default_pool_size"] = defaultPoolSize
	}

	var pullSecrets []v1.LocalObjectReference
	for _, name := range imagePullSecrets {
		pullSecrets = append(pullSecrets, v1.LocalObjectReference{Name: name})
	}

	podSpec := v1.PodSpec{
		EnableServiceLinks: new(false),
		ImagePullSecrets:   pullSecrets,
		Tolerations:        tolerations,
		NodeSelector:       nodeSelector,
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

	template := apiv1ac.PodTemplateSpec().WithSpec(podSpec)
	if len(podLabels) > 0 {
		template = template.WithObjectMeta(apiv1ac.Metadata().WithLabels(podLabels))
	}

	return apiv1ac.PoolerSpec().
		WithCluster(corev1ac.LocalObjectReference().WithName(clusterName)).
		WithType(apiv1.PoolerTypeRW).
		WithInstances(instances).
		WithPgBouncer(apiv1ac.PgBouncerSpec().
			WithPoolMode(poolMode).
			WithParameters(params)).
		WithServiceTemplate(apiv1ac.ServiceTemplateSpec().
			WithObjectMeta(apiv1ac.Metadata().WithAnnotations(InheritedAnnotations))).
		WithTemplate(template)
}
