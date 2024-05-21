package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	emailv1 "github.com/fntkg/email-operator/api/v1"
	"github.com/go-logr/logr"

	"github.com/mailersend/mailersend-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (r *EmailReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("email", req.NamespacedName)

	// Fetch the Email instance
	var email emailv1.Email
	if err := r.Get(ctx, req.NamespacedName, &email); err != nil {
		log.Error(err, "unable to fetch Email")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Fetch the EmailSenderConfig referenced by the Email
	var emailSenderConfig emailv1.EmailSenderConfig
	if err := r.Get(ctx, client.ObjectKey{Name: email.Spec.SenderConfigRef, Namespace: req.Namespace}, &emailSenderConfig); err != nil {
		log.Error(err, "unable to fetch EmailSenderConfig")
		return ctrl.Result{}, err
	}

	// Fetch the API token from the secret

	k8sClient, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		log.Error(err, "unable to create kubernetes client")
		return ctrl.Result{}, err
	}

	secret, err := k8sClient.CoreV1().Secrets(req.Namespace).Get(ctx, emailSenderConfig.Spec.APITokenSecretRef, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "unable to fetch secret")
		return ctrl.Result{}, err
	}

	// Create a new MailerSend client
	apiToken := string(secret.Data["apiToken"])
	ms := mailersend.NewMailersend(apiToken)
	//mailer := mailersend.NewMailersend(apiToken)

	// Define the email content and recipient details
	subject := email.Spec.Subject
	text := email.Spec.Body

	from := mailersend.From{
		Email: emailSenderConfig.Spec.SenderEmail,
	}

	recipients := []mailersend.Recipient{
		{
			Email: email.Spec.RecipientEmail,
		},
	}

	// Create the new message
	message := ms.Email.NewMessage()

	// Setup the new message
	message.SetFrom(from)
	message.SetRecipients(recipients)
	message.SetSubject(subject)
	message.SetText(text)

	// Send the message
	res, err := ms.Email.Send(ctx, message)
	if err != nil {
		log.Error(err, "unable to send email")
		email.Status.DeliveryStatus = "Failed"
		email.Status.Error = err.Error()
		if err := r.Status().Update(ctx, &email); err != nil {
			log.Error(err, "unable to update Email status")
		}
		return ctrl.Result{}, err
	}

	/*
		// Send the email using MailerSend API
		msg := mailersend.Email{
			From:    mailersend.Recipient{Email: emailSenderConfig.Spec.SenderEmail},
			To:      []mailersend.Recipient{{Email: email.Spec.RecipientEmail}},
			Subject: email.Spec.Subject,
			Text:    email.Spec.Body,
		}

		response, err := mailer.Send(ctx, msg)
		if err != nil {
			log.Error(err, "unable to send email")
			email.Status.DeliveryStatus = "Failed"
			email.Status.Error = err.Error()
			if err := r.Status().Update(ctx, &email); err != nil {
				log.Error(err, "unable to update Email status")
			}
			return ctrl.Result{}, err
		}
	*/

	// Update the Email status
	email.Status.DeliveryStatus = "Sent"
	email.Status.MessageID = res.Header.Get("X-Message-Id")
	if err := r.Status().Update(ctx, &email); err != nil {
		log.Error(err, "unable to update Email status")
		return ctrl.Result{}, err
	}

	log.Info("Email sent successfully", "Email", email)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EmailReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&emailv1.Email{}).
		Complete(r)
}
