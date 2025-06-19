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

// NewIncomingRequestMailNotification notification used to notify a user when a new
// incoming request is received.
var NewIncomingRequestMailNotification = MailTemplate{
	File: "new_incoming_request",
	Placeholder: notifications.Notification{
		Subject: "New Tool request received",
		PlainBody: `{{.UserName}} is interested in borrowing your tool {{.ToolName}} from 
{{.FromDate}} to {{.ToDate}}.

Please check the request details and respond accordingly on {{.ButtonUrl}}.
`,
	},
	WebAppURI: "/bookings/requests",
}

// BookingAcceptedMailNotification notification used to notify a user when a booking
// request is accepted.
var BookingAcceptedMailNotification = MailTemplate{
	File: "booking_accepted",
	Placeholder: notifications.Notification{
		Subject: "Your tool request has been accepted",
		PlainBody: `Your request for tool {{.ToolName}} from 
{{.FromDate}} to {{.ToDate}} has been accepted by {{.UserName}}.

Check your booking on {{.ButtonUrl}}.
`,
	},
	WebAppURI: "/bookings/requests",
}
