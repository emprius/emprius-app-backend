package api

import (
	"context"
	"fmt"
	"time"

	"github.com/emprius/emprius-app-backend/notifications"
	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"
)

// sendMail method sends a notification to the email provided. It requires the
// email template and the data to fill it. It executes the mail template with
// the data to get the notification and sends it with the recipient email
// address provided. It returns an error if the mail service is available and
// the notification could not be sent or the email address is invalid. If the
// mail service is not available, it does nothing.
// The lang parameter specifies the language code for the email template.
func (a *API) sendMail(ctx context.Context, to string, mail mailtemplates.MailTemplate, data interface{}, lang string) error {
	if a.mail != nil {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		// check if the email address is valid
		if !notifications.ValidEmail(to) {
			return fmt.Errorf("invalid email address")
		}
		// execute the mail template to get the notification
		notification, err := mail.ExecTemplate(data, lang)
		if err != nil {
			return err
		}
		// set the recipient email address
		notification.ToAddress = to
		// send the mail notification
		if err := a.mail.SendNotification(ctx, notification); err != nil {
			return err
		}
	}
	return nil
}
