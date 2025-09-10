package controller

import (
	"context"
	"fmt"
	"html/template"
	"k8s.io/apimachinery/pkg/util/yaml"
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
		WithEventFilter(MaintenanceChangedPredicate()).
		Complete(r)
}

func globalConfigPredicate() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == "global-config"
	})
}

type Config struct {
	Maintenance string `yaml:"maintenance"`
}

func MaintenanceChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCM, ok1 := e.ObjectOld.(*corev1.ConfigMap)
			newCM, ok2 := e.ObjectNew.(*corev1.ConfigMap)
			if !ok1 || !ok2 {
				return false
			}

			oldCfg := &Config{}
			newCfg := &Config{}

			if dataOld, ok := oldCM.Data["config.yaml"]; ok {
				_ = yaml.Unmarshal([]byte(dataOld), oldCfg)
			}
			if dataNew, ok := newCM.Data["config.yaml"]; ok {
				_ = yaml.Unmarshal([]byte(dataNew), newCfg)
			}

			return oldCfg.Maintenance != newCfg.Maintenance
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
