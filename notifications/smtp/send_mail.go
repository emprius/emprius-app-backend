package smtp

import (
	"context"
	"fmt"
	"time"

	"github.com/emprius/emprius-app-backend/notifications"
	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"
)

// SendMail is a helper function that executes an email template and sends the notification.
// It validates the email address, executes the template with the provided data,
// and sends the notification via the provided mail service.
// If mailService is nil, it returns nil (graceful degradation).
// The lang parameter specifies the language code for the email template.
func SendMail(
	ctx context.Context,
	mailService notifications.NotificationService,
	to string,
	template mailtemplates.MailTemplate,
	data interface{},
	lang string,
) error {
	if mailService != nil {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		// check if the email address is valid
		if !notifications.ValidEmail(to) {
			return fmt.Errorf("invalid email address")
		}
		// execute the mail template to get the notification
		notification, err := template.ExecTemplate(data, lang)
		if err != nil {
			return err
		}
		// set the recipient email address
		notification.ToAddress = to
		// send the mail notification
		if err := mailService.SendNotification(ctx, notification); err != nil {
			return err
		}
	}
	return nil
}
