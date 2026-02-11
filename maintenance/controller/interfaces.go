package controller

import (
	"context"

	"github.com/cloudogu/k8s-registry-lib/repository"
)

type MaintenanceAdapter interface {
	GetStatus(ctx context.Context) (repository.MaintenanceModeDescription, bool, error)
}
