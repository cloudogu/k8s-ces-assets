package controller

import (
	"context"
	"fmt"
	"html/template"
	"os"

	"github.com/cloudogu/k8s-registry-lib/repository"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	path503 = "/var/www/html/errors/503.html"
)

type TmplConfig map[string]string

func (c TmplConfig) GetOrDefault(key, def string) string {
	if v, ok := c[key]; ok && v != "" {
		return v
	}
	return def
}

type PageData struct {
	Config TmplConfig
}

// MaintenanceReconciler is responsible for reconciling the maintenance configmap and to create a corresponding error page
// for the maintenance mode
type MaintenanceReconciler struct {
	Adapter MaintenanceAdapter
}

// Reconcile reconciles the maintenance configmap and triggers the error page generation
func (r *MaintenanceReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Reconciling maintenance config for redirect")

	description, active, err := r.Adapter.GetStatus(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get maintenance status: %w", err)
	}

	if !active {
		logger.Info("Maintenance mode is inactive. Skipping maintenance page generation.")
		return ctrl.Result{}, nil
	}

	tmpl := template.Must(template.ParseFiles(fmt.Sprintf("%s.tpl", path503)))

	data := PageData{
		Config: TmplConfig{
			"maintenance/title": description.Title,
			"maintenance/text":  description.Text,
		},
	}
	if err = renderToFile(tmpl, path503, data); err != nil {
		logger.Error(err, "Failed to render maintenance page")
		return ctrl.Result{}, fmt.Errorf("failed to render maintenance page: %w", err)
	}

	return ctrl.Result{}, nil
}

func renderToFile(t *template.Template, outPath string, data any) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	return t.Execute(f, data)
}

// SetupWithManager sets up the maintenance configmap controller with the Manager.
// The controller watches for changes to the maintenance configmap.
func (r *MaintenanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}, builder.WithPredicates(maintenancePredicate())).
		Complete(r)
}

func maintenancePredicate() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == repository.MaintenanceConfigMapName
	})
}
