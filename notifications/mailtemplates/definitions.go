package mailtemplates

import "github.com/emprius/emprius-app-backend/notifications"

// WelcomeMailNotification is the notification to be sent when a user creates
// an account and needs to verify it.
var WelcomeMailNotification = MailTemplate{
	File: "welcome",
	Placeholder: notifications.Notification{
		Subject: "Welcome to ComunPop",
		PlainBody: `You successfully registered to {{.AppName}}

Start using the app on {{.AppUrl}}`,
	},
	WebAppURI: "/profile",
}
