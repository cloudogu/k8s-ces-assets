package maintenance

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"os"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"encoding/json"
)

const (
	maintenanceKey = "maintenance"
	path503        = "/var/www/html/errors/503.html"
)

type TmplConfig map[string]string

type MaintenanceMode struct {
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
}

func (c TmplConfig) GetOrDefault(key, def string) string {
	if v, ok := c[key]; ok && v != "" {
		return v
	}
	return def
}

type PageData struct {
	Config TmplConfig
}

// MaintenanceReconciler is responsible for reconciling the global configmap and to create a corresponding error page
// for the maintenance mode
type MaintenanceReconciler struct {
	Client             client.Client
	GlobalConfigGetter GlobalConfigRepository
}

// Reconcile reconciles the global configmap and triggers the error page generation
func (r *MaintenanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Reconciling global config for redirect")

	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, req.NamespacedName, cm)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get global config map: %w", err)
	}

	globalCfg, err := r.GlobalConfigGetter.Get(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get global config: %w", err)
	}

	maintenanceModeJson, ok := globalCfg.Get(maintenanceKey)
	if !ok {
		return ctrl.Result{}, fmt.Errorf("maintenance not found in global config")
	}
	logger.Info(fmt.Sprintf("Mode: %s", maintenanceModeJson))

	maintenanceMode := &MaintenanceMode{}
	err = json.Unmarshal([]byte(maintenanceModeJson), maintenanceMode)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse maintenancemode: %w", err)
	}

	tmpl := template.Must(template.ParseFiles(fmt.Sprintf("%s.tpl", path503)))

	data := PageData{
		Config: TmplConfig{
			"maintenance/title": maintenanceMode.Title,
			"maintenance/text":  maintenanceMode.Text,
		},
	}
	if err = renderToFile(tmpl, path503, data); err != nil {
		log.Fatal(err)
	}
	return ctrl.Result{}, nil
}

func renderToFile(t *template.Template, outPath string, data any) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, data)
}

// SetupWithManager sets up the global configmap controller with the Manager.
// The controller watches for changes to the global configmap.
func (r *MaintenanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}, builder.WithPredicates(globalConfigPredicate())).
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(maintenancePredicate())).
		Complete(r)
}

func globalConfigPredicate() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == "global-config"
	})
}

func maintenancePredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			return false
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			return e.Object.GetName() == maintenanceKey
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			return e.ObjectNew.GetName() == maintenanceKey
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
