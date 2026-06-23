package cnpg

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"xata/services/clusters/internal/connectors/cnpg/resources"

	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func Test_RegisterCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)

	testCNPGServices := func(clusterName, namespace string) []*v1.Service {
		suffixes := []string{"-rw", "-r", "-ro"}
		services := make([]*v1.Service, 0, len(suffixes))
		for _, suffix := range suffixes {
			services = append(services, &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "branch-" + clusterName + suffix,
					Namespace:       namespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						"service.cilium.io/global": "true",
					},
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeClusterIP,
					Ports: []v1.ServicePort{
						{
							Name:       "postgres",
							Port:       5432,
							TargetPort: intstr.FromInt(5432),
							Protocol:   v1.ProtocolTCP,
						},
					},
				},
			})
		}
		return services
	}

	testClustersService := func(clusterName, namespace string) *v1.Service {
		return &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:            resources.ClustersServicePrefix + clusterName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Annotations: map[string]string{
					"service.cilium.io/global": "true",
				},
			},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeClusterIP,
				Ports: []v1.ServicePort{
					{
						Name: "grpc",
						Port: 5002,

						TargetPort: intstr.FromInt(5002),
						Protocol:   v1.ProtocolTCP,
					},
				},
			},
		}
	}

	errTest := errors.New("some random error")

	tests := []struct {
		name          string
		clusterName   string
		namespace     string
		xataNamespace string
		fakeClient    client.Client

		wantCNPGServices    []*v1.Service
		wantClustersService *v1.Service
		wantError           error
		errorMessage        string
	}{
		{
			name:                "RegisterCluster works",
			clusterName:         "test-cluster",
			namespace:           "xata-clusters",
			xataNamespace:       "xata",
			fakeClient:          fake.NewClientBuilder().WithScheme(scheme).Build(),
			wantCNPGServices:    testCNPGServices("test-cluster", "xata-clusters"),
			wantClustersService: testClustersService("test-cluster", "xata"),
			wantError:           nil,
		},
		{
			name:                "RegisterCluster works with a different clusters namespace",
			clusterName:         "another-cluster",
			namespace:           "another-namespace",
			xataNamespace:       "xata",
			fakeClient:          fake.NewClientBuilder().WithScheme(scheme).Build(),
			wantCNPGServices:    testCNPGServices("another-cluster", "another-namespace"),
			wantClustersService: testClustersService("another-cluster", "xata"),
			wantError:           nil,
		},
		{
			name:                "RegisterCluster works with a different xata namespace",
			clusterName:         "another-cluster",
			namespace:           "xata-clusters",
			xataNamespace:       "another-namespace",
			fakeClient:          fake.NewClientBuilder().WithScheme(scheme).Build(),
			wantCNPGServices:    testCNPGServices("another-cluster", "xata-clusters"),
			wantClustersService: testClustersService("another-cluster", "another-namespace"),
			wantError:           nil,
		},
		{
			name:          "RegisterCluster fails for existing service",
			clusterName:   "test-cluster",
			namespace:     "xata-clusters",
			xataNamespace: "xata",
			fakeClient: fake.NewClientBuilder().WithScheme(scheme).WithObjects(&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "branch-test-cluster-rw",
					Namespace: "xata-clusters",
				},
			}).Build(),
			wantError: &k8serrors.StatusError{
				ErrStatus: metav1.Status{
					Status:  "Failure",
					Code:    http.StatusConflict,
					Reason:  metav1.StatusReasonAlreadyExists,
					Message: "services \"branch-test-cluster-rw\" already exists",
					Details: &metav1.StatusDetails{
						Name:  "branch-test-cluster-rw",
						Group: "",
						Kind:  "services",
					},
				},
			},
		},
		{
			name:          "RegisterCluster fails for creation error",
			clusterName:   "test-cluster",
			namespace:     "xata-clusters",
			xataNamespace: "xata",
			fakeClient: fake.NewClientBuilder().WithInterceptorFuncs(
				interceptor.Funcs{
					Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
						return errTest
					},
				},
			).Build(),
			wantError:    errTest,
			errorMessage: "some random error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := &DefaultConnector{
				KubernetesClient: tt.fakeClient,
			}

			err := connector.RegisterCluster(context.Background(), tt.clusterName, tt.namespace, tt.xataNamespace)
			if tt.wantError != nil {
				if !errors.Is(err, tt.wantError) {
					require.Equal(t, tt.wantError, err)
				}
				return
			}

			require.NoError(t, err)

			// Verify all CNPG services were created with correct configuration
			suffixes := []string{"-rw", "-r", "-ro"}
			for i, suffix := range suffixes {
				createdService := &v1.Service{}
				err = tt.fakeClient.Get(context.Background(), types.NamespacedName{
					Namespace: tt.namespace,
					Name:      "branch-" + tt.clusterName + suffix,
				}, createdService)
				require.NoError(t, err)
				require.Equal(t, tt.wantCNPGServices[i], createdService, "CNPG service branch-%s should match expected configuration", suffix)
			}

			// Verify the clusters service was created with correct configuration
			createdClustersService := &v1.Service{}
			err = tt.fakeClient.Get(context.Background(), types.NamespacedName{
				Namespace: tt.xataNamespace,
				Name:      resources.ClustersServicePrefix + tt.clusterName,
			}, createdClustersService)
			require.NoError(t, err)
			require.Equal(t, tt.wantClustersService, createdClustersService)
		})
	}
}

func Test_DeregisterCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)

	testCNPGServices := func(clusterName, namespace string) []*v1.Service {
		suffixes := []string{"-rw", "-r", "-ro"}
		services := make([]*v1.Service, 0, len(suffixes))
		for _, suffix := range suffixes {
			services = append(services, &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "branch-" + clusterName + suffix,
					Namespace: namespace,
					Annotations: map[string]string{
						"service.cilium.io/global": "true",
					},
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeClusterIP,
					Ports: []v1.ServicePort{
						{
							Name:       "postgres",
							Port:       5432,
							TargetPort: intstr.FromInt(5432),
							Protocol:   v1.ProtocolTCP,
						},
					},
				},
			})
		}
		return services
	}

	testClustersService := func(clusterName, namespace string) *v1.Service {
		return &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resources.ClustersServicePrefix + clusterName,
				Namespace: namespace,
				Annotations: map[string]string{
					"service.cilium.io/global": "true",
				},
			},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeClusterIP,
				Ports: []v1.ServicePort{
					{
						Name:       "grpc",
						Port:       5002,
						TargetPort: intstr.FromInt(5002),
						Protocol:   v1.ProtocolTCP,
					},
				},
			},
		}
	}

	errTest := errors.New("some random error")

	tests := []struct {
		name              string
		clusterName       string
		clustersNamespace string
		xataNamespace     string
		fakeClient        client.Client

		existingCNPGServices    []*v1.Service
		existingClustersService *v1.Service
		wantError               error
		errorMessage            string
	}{
		{
			name:                    "DeregisterCluster works",
			clusterName:             "test-cluster",
			clustersNamespace:       "xata-clusters",
			xataNamespace:           "xata",
			fakeClient:              fake.NewClientBuilder().WithScheme(scheme).Build(),
			existingCNPGServices:    testCNPGServices("test-cluster", "xata-clusters"),
			existingClustersService: testClustersService("test-cluster", "xata"),
			wantError:               nil,
		},
		{
			name:                    "DeregisterCluster works with different namespace",
			clusterName:             "another-cluster",
			clustersNamespace:       "another-namespace",
			xataNamespace:           "xata2",
			fakeClient:              fake.NewClientBuilder().WithScheme(scheme).Build(),
			existingCNPGServices:    testCNPGServices("another-cluster", "another-namespace"),
			existingClustersService: testClustersService("test-cluster", "xata2"),
			wantError:               nil,
		},
		{
			name:                    "DeregisterCluster succeeds when service doesn't exist",
			clusterName:             "test-cluster",
			clustersNamespace:       "xata-clusters",
			xataNamespace:           "xata",
			fakeClient:              fake.NewClientBuilder().WithScheme(scheme).Build(),
			existingCNPGServices:    nil, // No existing CNPG services
			existingClustersService: nil, // No existing clusters service
			wantError:               nil,
		},
		{
			name:                 "DeregisterCluster fails for deletion error",
			clusterName:          "test-cluster",
			clustersNamespace:    "xata-clusters",
			xataNamespace:        "xata",
			existingCNPGServices: testCNPGServices("test-cluster", "xata-clusters"),
			fakeClient: fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(
				interceptor.Funcs{
					Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						return errTest
					},
				},
			).Build(),
			wantError:    errTest,
			errorMessage: "some random error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the existing CNPG services if needed
			if tt.existingCNPGServices != nil {
				for _, svc := range tt.existingCNPGServices {
					err := tt.fakeClient.Create(context.Background(), svc)
					require.NoError(t, err)
				}
			}

			// Create the existing clusters service if needed
			if tt.existingClustersService != nil {
				err := tt.fakeClient.Create(context.Background(), tt.existingClustersService)
				require.NoError(t, err)
			}

			connector := &DefaultConnector{
				KubernetesClient: tt.fakeClient,
			}

			err := connector.DeregisterCluster(context.Background(), tt.clusterName, tt.clustersNamespace, tt.xataNamespace)
			if tt.wantError != nil {
				if !errors.Is(err, tt.wantError) {
					require.Equal(t, tt.wantError, err)
				}
				return
			}

			require.NoError(t, err)

			// Verify all CNPG services were deleted
			suffixes := []string{"-rw", "-r", "-ro"}
			for _, suffix := range suffixes {
				deletedService := &v1.Service{}
				err = tt.fakeClient.Get(context.Background(), types.NamespacedName{
					Namespace: tt.clustersNamespace,
					Name:      "branch-" + tt.clusterName + suffix,
				}, deletedService)
				require.Error(t, err)
				require.True(t, k8serrors.IsNotFound(err), "CNPG service branch-%s%s should be deleted", tt.clusterName, suffix)
			}

			// Verify the clusters service was deleted
			deletedService := &v1.Service{}
			err = tt.fakeClient.Get(context.Background(), types.NamespacedName{
				Namespace: tt.xataNamespace,
				Name:      resources.ClustersServicePrefix + tt.clusterName,
			}, deletedService)
			require.Error(t, err)
			require.True(t, k8serrors.IsNotFound(err), "clusters service should be deleted")
		})
	}
}

func Test_GetClusterCredentials(t *testing.T) {
	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)
	v1.AddToScheme(scheme)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "xata-clusters",
			Name:            "fakeID-superuser",
			ResourceVersion: "1",
		},
		Data: map[string][]byte{
			v1.BasicAuthUsernameKey: []byte("foo"),
			v1.BasicAuthPasswordKey: []byte("bar"),
		},
	}

	tests := []struct {
		name         string
		clusterID    string
		username     string
		fakeClient   client.Client
		wantError    bool
		wantCreds    *Credentials
		errorMessage string
	}{
		{
			name:       "GetClusterCredentials works",
			clusterID:  "fakeID",
			username:   "superuser",
			fakeClient: fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build(),
			wantError:  false,
			wantCreds: &Credentials{
				SecretVersion: "1",
				Username:      "foo",
				Password:      "bar",
			},
		},
		{
			name:      "GetClusterCredentials works with different username",
			clusterID: "fakeID",
			username:  "anotheruser",
			fakeClient: fake.NewClientBuilder().WithScheme(scheme).WithObjects(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       "xata-clusters",
					Name:            "fakeID-anotheruser",
					ResourceVersion: "1",
				},
				Data: map[string][]byte{
					v1.BasicAuthUsernameKey: []byte("anotherusername"),
					v1.BasicAuthPasswordKey: []byte("anotheruserspassword"),
				},
			}).Build(),
			wantError: false,
			wantCreds: &Credentials{
				SecretVersion: "1",
				Username:      "anotherusername",
				Password:      "anotheruserspassword",
			},
		},
		{
			name:         "GetClusterCredentials fails for non-existing cluster",
			clusterID:    "fakeID",
			username:     "superuser",
			fakeClient:   fake.NewClientBuilder().WithScheme(scheme).Build(),
			wantError:    true,
			errorMessage: "secrets \"fakeID-superuser\" not found",
		},
		{
			name:      "GetClusterCredentials fails for other error",
			clusterID: "fakeID",
			username:  "superuser",
			fakeClient: fake.NewClientBuilder().WithInterceptorFuncs(
				interceptor.Funcs{
					Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						return errors.New("some random error")
					},
				},
			).Build(),
			wantError:    true,
			errorMessage: "some random error",
		},
		{
			name:         "GetClusterCredentials fails for non-existing username in secret",
			clusterID:    "fakeID",
			username:     "anotheruser",
			fakeClient:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build(),
			wantError:    true,
			errorMessage: "secrets \"fakeID-anotheruser\" not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := &DefaultConnector{
				KubernetesClient: tt.fakeClient,
			}

			creds, err := connector.GetClusterCredentials(context.Background(), tt.clusterID, "xata-clusters", tt.username)
			if tt.wantError {
				assert.Error(t, err)
				assert.Equal(t, tt.errorMessage, err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCreds, creds)
			}
		})
	}
}
