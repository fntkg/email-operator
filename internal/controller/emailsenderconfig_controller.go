package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	emailv1 "github.com/fntkg/email-operator/api/v1"
	"github.com/go-logr/logr"
)

// EmailSenderConfigReconciler reconciles a EmailSenderConfig object
type EmailSenderConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=email.example.com,resources=emailsenderconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=email.example.com,resources=emailsenderconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=email.example.com,resources=emailsenderconfigs/finalizers,verbs=update

func (r *EmailSenderConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("emailsenderconfig", req.NamespacedName)

	// Fetch the EmailSenderConfig instance
	var emailSenderConfig emailv1.EmailSenderConfig
	if err := r.Get(ctx, req.NamespacedName, &emailSenderConfig); err != nil {
		log.Error(err, "unable to fetch EmailSenderConfig")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Log the creation or update of the EmailSenderConfig
	log.Info("EmailSenderConfig created or updated", "EmailSenderConfig", emailSenderConfig)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EmailSenderConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&emailv1.EmailSenderConfig{}).
		Complete(r)
}
