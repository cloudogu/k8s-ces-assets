package config

import (
	"fmt"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	namespaceEnvVar = "WATCH_NAMESPACE"
)

var (
	logger = ctrl.Log.WithName("k8s-service-discovery.maintenance.config")
)

func ReadWatchNamespace() (string, error) {
	watchNamespace, found := os.LookupEnv(namespaceEnvVar)
	if !found {
		return "", fmt.Errorf("failed to read namespace to watch from environment variable [%s], please set the variable and try again", namespaceEnvVar)
	}
	logger.Info(fmt.Sprintf("found target namespace: [%s]", watchNamespace))

	return watchNamespace, nil
}
