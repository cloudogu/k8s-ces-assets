package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	v2 "k8s.io/api/apps/v1"
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
