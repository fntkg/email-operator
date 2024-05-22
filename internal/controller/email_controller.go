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
	if email.Status.DeliveryStatus != "Sent" {
		// Fetch the EmailSenderConfig referenced by the Email
		var emailSenderConfig emailv1.EmailSenderConfig
		if err := r.Get(ctx, client.ObjectKey{Name: email.Spec.SenderConfigRef, Namespace: req.Namespace}, &emailSenderConfig); err != nil {
			log.Error(err, "unable to fetch EmailSenderConfig")
			return ctrl.Result{}, err
		}

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

		provider := emailSenderConfig.Spec.Provider
		switch provider {
		case "mailersend":
			token := "mlsn." + string(apiToken[:])
			ms := mailersend.NewMailersend(string(token))

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

			// Update the Email status
			email.Status.DeliveryStatus = "Sent"
			email.Status.MessageID = res.Header.Get("X-Message-Id")
			if err := r.Status().Update(ctx, &email); err != nil {
				log.Error(err, "unable to update Email status")
				return ctrl.Result{}, err
			}
		case "mailgun":
			// 4d1c68aa48bf068f5cbf704a62de0633-a2dd40a3-194f2a01
			senderEmail := emailSenderConfig.Spec.SenderEmail
			re := regexp.MustCompile(`@(.+)$`)
			domain := re.FindStringSubmatch(senderEmail)

			// Create an instance of the Mailgun Client
			token := string(apiToken[:])
			mg := mailgun.NewMailgun(domain[1], token)

			sender := emailSenderConfig.Spec.SenderEmail
			subject := email.Spec.Subject
			body := email.Spec.Body
			recipient := email.Spec.RecipientEmail

			// The message object allows you to add attachments and Bcc recipients
			message := mg.NewMessage(sender, subject, body, recipient)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			// Send the message with a 10 second timeout
			_, id, err := mg.Send(ctx, message)
			if err != nil {
				log.Error(err, "unable to send email")
				return ctrl.Result{}, err
			}

			// Update the Email status
			email.Status.DeliveryStatus = "Sent"
			email.Status.MessageID = id
			if err := r.Status().Update(ctx, &email); err != nil {
				log.Error(err, "unable to update Email status")
				return ctrl.Result{}, err
			}
		default:
			log.Error(fmt.Errorf("provider not supported"), "provider", provider)
		}

		log.Info("Email sent successfully", "Email", email)

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EmailReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&emailv1.Email{}).
		Complete(r)
}
