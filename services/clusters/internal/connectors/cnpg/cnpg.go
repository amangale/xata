package cnpg

import (
	"context"
	"fmt"

	"xata/services/clusters/internal/connectors/cnpg/resources"
	"xata/services/clusters/internal/kubernetes"

	"k8s.io/apimachinery/pkg/types"

	barmanPluginApi "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
	apiv1 "github.com/xataio/xata-cnpg/api/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate go run github.com/vektra/mockery/v3 --with-expecter --name Connector

// Connector is an interface for interacting with the cnpg operator
type Connector interface {
	GetObjectStore(ctx context.Context, id, namespace string) (*barmanPluginApi.ObjectStore, error)
	RegisterCluster(ctx context.Context, clusterID, clustersNamespace, xataNamespace string) error
	DeregisterCluster(ctx context.Context, clusterID, clustersNamespace, xataNamespace string) error
	GetClusterCredentials(ctx context.Context, id, namespace, username string) (*Credentials, error)
}

type Credentials struct {
	SecretVersion string
	Username      string
	Password      string
}

type DefaultConnector struct {
	KubernetesClient client.Client
}

// NewConnector creates a client for communicating with the k8s cnpg operator
func NewConnector(kubeConfig string) (*DefaultConnector, error) {
	dcp := &kubernetes.DefaultConfigProvider{
		KubeConfigPath: kubeConfig,
		MasterURL:      "",
	}
	cfg, err := kubernetes.GetRestConfig(dcp)
	if err != nil {
		return nil, fmt.Errorf("get restconfig %w", err)
	}

	// make the client aware of the cnpg scheme
	scheme := runtime.NewScheme()
	if err = clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add to scheme clientgo %w", err)
	}

	if err = apiv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add to scheme cnpg %w", err)
	}

	// make the client aware of the barman cloud plugin scheme
	if err = barmanPluginApi.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add to scheme barman plugin %w", err)
	}

	clientK8s, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("create clientset %w", err)
	}

	return &DefaultConnector{KubernetesClient: clientK8s}, nil
}

func (c *DefaultConnector) RegisterCluster(ctx context.Context, clusterID, clustersNamespace, xataNamespace string) error {
	svcs := resources.GlobalCNPGServices(clusterID, clustersNamespace)

	for _, svc := range svcs {
		if err := c.KubernetesClient.Create(ctx, &svc); err != nil {
			return err
		}
	}

	poolerSvc := resources.GlobalPoolerService(clusterID, clustersNamespace)
	if err := c.KubernetesClient.Create(ctx, &poolerSvc); err != nil {
		return err
	}

	svc := resources.GlobalClustersService(clusterID, xataNamespace)
	return c.KubernetesClient.Create(ctx, &svc)
}

func (c *DefaultConnector) DeregisterCluster(ctx context.Context, clusterID, clustersNamespace, xataNamespace string) error {
	svcs := resources.GlobalCNPGServices(clusterID, clustersNamespace)

	// Remove the global CNPG services for the branch.
	for _, svc := range svcs {
		err := c.KubernetesClient.Get(ctx, types.NamespacedName{
			Namespace: svc.Namespace,
			Name:      svc.Name,
		}, &svc)
		if err != nil {
			continue
		}

		if err := c.KubernetesClient.Delete(ctx, &svc); err != nil {
			return err
		}
	}

	// Remove the global pooler service for the branch.
	poolerSvc := resources.GlobalPoolerService(clusterID, clustersNamespace)
	if err := c.KubernetesClient.Get(ctx, types.NamespacedName{
		Namespace: poolerSvc.Namespace,
		Name:      poolerSvc.Name,
	}, &poolerSvc); err == nil {
		if err := c.KubernetesClient.Delete(ctx, &poolerSvc); err != nil {
			return err
		}
	}

	// Deregister the clusters service for the branch
	svc := &v1.Service{}
	err := c.KubernetesClient.Get(ctx, types.NamespacedName{
		Namespace: xataNamespace,
		Name:      resources.ClustersServicePrefix + clusterID,
	}, svc)
	if err != nil {
		// If the service doesn't exist, consider it already deregistered
		return nil
	}

	return c.KubernetesClient.Delete(ctx, svc)
}

func (c *DefaultConnector) GetObjectStore(ctx context.Context, id, namespace string) (*barmanPluginApi.ObjectStore, error) {
	objectStore := &barmanPluginApi.ObjectStore{}
	err := c.KubernetesClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      id,
	}, objectStore)
	if err != nil {
		return nil, err
	}
	return objectStore, nil
}

func (c *DefaultConnector) GetClusterCredentials(ctx context.Context, id, namespace, username string) (*Credentials, error) {
	// Read the secret with Kubernetes client
	var secret v1.Secret
	err := c.KubernetesClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      id + "-" + username,
	}, &secret)
	if err != nil {
		return nil, err
	}

	return &Credentials{
		SecretVersion: secret.ResourceVersion,
		Username:      string(secret.Data[v1.BasicAuthUsernameKey]),
		Password:      string(secret.Data[v1.BasicAuthPasswordKey]),
	}, nil
}
