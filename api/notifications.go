package api

import (
	"context"
	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"
	"github.com/emprius/emprius-app-backend/notifications/smtp"
)

// sendMail method sends a notification to the email provided. It requires the
// email template and the data to fill it. It executes the mail template with
// the data to get the notification and sends it with the recipient email
// address provided. It returns an error if the mail service is available and
// the notification could not be sent or the email address is invalid. If the
// mail service is not available, it does nothing.
// The lang parameter specifies the language code for the email template.
func (a *API) sendMail(ctx context.Context, to string, mail mailtemplates.MailTemplate, data interface{}, lang string) error {
	return smtp.SendMail(ctx, a.mail, to, mail, data, lang)
}
