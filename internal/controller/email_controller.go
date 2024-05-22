package controller

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/mailgun/mailgun-go/v4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	emailv1 "github.com/fntkg/email-operator/api/v1"
	"github.com/go-logr/logr"
	"github.com/mailersend/mailersend-go"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// EmailReconciler reconciles a Email object
type EmailReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Config *rest.Config
}

//+kubebuilder:rbac:groups=email.example.com,resources=emails,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=email.example.com,resources=emails/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=email.example.com,resources=emails/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *EmailReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch the Email instance
	var email emailv1.Email
	if err := r.Get(ctx, req.NamespacedName, &email); err != nil {
		log.Error(err, "unable to fetch Email")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if the email is already sent
	if email.Status.DeliveryStatus == "Sent" {
		return ctrl.Result{}, nil
	}

	// Fetch the EmailSenderConfig referenced by the Email
	var emailSenderConfig emailv1.EmailSenderConfig
	if err := r.Get(ctx, client.ObjectKey{Name: email.Spec.SenderConfigRef, Namespace: req.Namespace}, &emailSenderConfig); err != nil {
		log.Error(err, "unable to fetch EmailSenderConfig")
		return ctrl.Result{}, err
	}

	// Fetch the Secret referenced by the EmailSenderConfig
	secret := &corev1.Secret{}
	secretName := emailSenderConfig.Spec.APITokenSecretRef
	err := r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: req.Namespace}, secret)
	if err != nil {
		log.Error(err, "unable to fetch Secret", "secret", secretName)
		return ctrl.Result{}, err
	}

	// Get the API token from the Secret
	apiToken, ok := secret.Data["apiToken"]
	if !ok {
		log.Error(fmt.Errorf("apiToken not found in secret"), "secret", secretName)
		return ctrl.Result{}, err
	}

	// Send the email using the provider specified in the EmailSenderConfig
	switch emailSenderConfig.Spec.Provider {
	case "mailersend":
		return r.sendWithMailersend(ctx, emailSenderConfig, email, apiToken)
	case "mailgun":
		return r.sendWithMailgun(ctx, emailSenderConfig, email, apiToken)
	default:
		err := fmt.Errorf("provider not supported")
		log.Error(err, "provider", emailSenderConfig.Spec.Provider)
		return ctrl.Result{}, err
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *EmailReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&emailv1.Email{}).
		Complete(r)
}

func (r *EmailReconciler) sendWithMailersend(ctx context.Context, config emailv1.EmailSenderConfig, email emailv1.Email, apiToken []byte) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	token := "mlsn." + string(apiToken)
	ms := mailersend.NewMailersend(token)

	// Define the email content and recipient details
	message := ms.Email.NewMessage()
	message.SetFrom(mailersend.From{Email: config.Spec.SenderEmail})
	message.SetRecipients([]mailersend.Recipient{{Email: email.Spec.RecipientEmail}})
	message.SetSubject(email.Spec.Subject)
	message.SetText(email.Spec.Body)

	// Send the message
	res, err := ms.Email.Send(ctx, message)
	if err != nil {
		log.Error(err, "unable to send email")
		return r.updateEmailStatus(ctx, &email, "Failed", err.Error())
	}

	// Update the Email status
	return r.updateEmailStatus(ctx, &email, "Sent", res.Header.Get("X-Message-Id"))
}

func (r *EmailReconciler) sendWithMailgun(ctx context.Context, config emailv1.EmailSenderConfig, email emailv1.Email, apiToken []byte) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Extract the domain from the sender email
	re := regexp.MustCompile(`@(.+)$`)
	domain := re.FindStringSubmatch(config.Spec.SenderEmail)[1]

	// Create a new Mailgun client
	mg := mailgun.NewMailgun(domain, string(apiToken))
	message := mg.NewMessage(config.Spec.SenderEmail, email.Spec.Subject, email.Spec.Body, email.Spec.RecipientEmail)

	// Set a timeout of 10 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send the message
	_, id, err := mg.Send(ctx, message)
	if err != nil {
		log.Error(err, "unable to send email")
		return r.updateEmailStatus(ctx, &email, "Failed", err.Error())
	}

	// Update the Email status
	return r.updateEmailStatus(ctx, &email, "Sent", id)
}

func (r *EmailReconciler) updateEmailStatus(ctx context.Context, email *emailv1.Email, status, messageID string) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Update the Email status
	email.Status.DeliveryStatus = status
	email.Status.MessageID = messageID
	if status == "Failed" {
		email.Status.Error = messageID
	}
	if err := r.Status().Update(ctx, email); err != nil {
		log.Error(err, "unable to update Email status")
		return ctrl.Result{}, err
	}
	log.Info("Email status updated successfully", "Email", email)
	return ctrl.Result{}, nil
}
