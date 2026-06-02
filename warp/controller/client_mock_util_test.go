package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	v2 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func getClientMock(t *testing.T, entries []v1.WarpMenuEntry) client.WithWatch {
	clientMock := newClientBuilder(t).
		WithRuntimeObjects(
			&v2.Deployment{
				ObjectMeta: controllerruntime.ObjectMeta{Name: testDeploymentName, Namespace: testNamespace},
			},
			getConfigMap(t, config.Configuration{}),
			&v1.WarpMenuEntryList{
				Items: entries,
			},
		).
		WithStatusSubresource(&entries[0]).
		Build()
	return clientMock
}
func getClientMockWithoutConfigMap(t *testing.T, entries []v1.WarpMenuEntry) client.WithWatch {
	clientMock := newClientBuilder(t).
		WithRuntimeObjects(
			&v2.Deployment{
				ObjectMeta: controllerruntime.ObjectMeta{Name: testDeploymentName, Namespace: testNamespace},
			},
			&v1.WarpMenuEntryList{
				Items: entries,
			},
		).
		WithStatusSubresource(&entries[0]).
		Build()
	return clientMock
}

func getClientMockWithListError(t *testing.T, entries []v1.WarpMenuEntry) client.WithWatch {
	clientMock := newClientBuilder(t).
		WithRuntimeObjects(
			&v2.Deployment{
				ObjectMeta: controllerruntime.ObjectMeta{Name: testDeploymentName, Namespace: testNamespace},
			},
			getConfigMap(t, config.Configuration{}),
			&v1.WarpMenuEntryList{
				Items: entries,
			},
		).
		WithInterceptorFuncs(interceptor.Funcs{
			// Wir fangen den List-Aufruf ab und zwingen ihn, einen Fehler zu liefern
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				return fmt.Errorf("Simulating api error")
			},
		}).
		WithStatusSubresource(&entries[0]).
		Build()
	return clientMock
}

func getClientMockWithStatusUpdateError(t *testing.T, entries []v1.WarpMenuEntry) client.WithWatch {
	clientMock := newClientBuilder(t).
		WithRuntimeObjects(
			&v2.Deployment{
				ObjectMeta: controllerruntime.ObjectMeta{Name: testDeploymentName, Namespace: testNamespace},
			},
			getConfigMap(t, config.Configuration{}),
			&v1.WarpMenuEntryList{
				Items: entries,
			},
		).
		WithStatusSubresource(&entries[0]).
		WithInterceptorFuncs(interceptor.Funcs{
			// This single hook intercepts all actions on client.SubResourceClient
			SubResource: func(cl client.WithWatch, subResource string) client.SubResourceClient {
				realSubResourceClient := cl.SubResource(subResource)
				// Verify the code is calling the "status" subresource

				if subResource == "status" {
					return &mockSubResourceClient{
						SubResourceClient: realSubResourceClient,
					}
				}
				// Pass-through for any other subresources (like scale/eviction)
				return realSubResourceClient
			},
		}).
		Build()
	return clientMock
}

func getConfigMap(t *testing.T, warpMenuConfig config.Configuration) *corev1.ConfigMap {
	warpMenuConfigAsString, err := yaml.Marshal(warpMenuConfig)
	require.NoError(t, err)

	configMap := corev1.ConfigMap{}
	data := map[string]string{
		"warp": string(warpMenuConfigAsString),
	}
	configMap.Name = "k8s-ces-warp-config"
	configMap.Namespace = testNamespace
	configMap.Data = data
	return &configMap
}

type mockSubResourceClient struct {
	client.SubResourceClient
}

// Override the Update method to inject your required error branch
func (m *mockSubResourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return fmt.Errorf("mocked SubResourceClient error")
}
