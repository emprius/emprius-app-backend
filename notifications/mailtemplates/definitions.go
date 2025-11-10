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
	Subjects: map[string]string{
		"en": "Welcome to ComunPop",
		"es": "Bienvenido a ComunPop",
		"ca": "Benvingut a ComunPop",
	},
	PlainBodies: map[string]string{
		"en": `You successfully registered to {{.AppName}}

Start using the app on {{.AppUrl}}`,
		"es": `Te has registrado exitosamente en {{.AppName}}

Comienza a usar la aplicación en {{.AppUrl}}`,
		"ca": `T'has registrat amb èxit a {{.AppName}}

Comença a utilitzar l'aplicació a {{.AppUrl}}`,
	},
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
	Subjects: map[string]string{
		"en": "New Tool request received",
		"es": "Nueva solicitud de herramienta recibida",
		"ca": "Nova sol·licitud d'eina rebuda",
	},
	PlainBodies: map[string]string{
		"en": `{{.UserName}} is interested in borrowing your tool {{.ToolName}} from
{{.FromDate}} to {{.ToDate}}.

Please check the request details and respond accordingly on {{.ButtonUrl}}.

Manage notification preferences: {{.NotificationsUrl}}`,
		"es": `{{.UserName}} está interesado en pedir prestada tu herramienta {{.ToolName}} desde
{{.FromDate}} hasta {{.ToDate}}.

Por favor revisa los detalles de la solicitud y responde en {{.ButtonUrl}}.

Gestionar preferencias de notificaciones: {{.NotificationsUrl}}`,
		"ca": `{{.UserName}} està interessat a demanar prestada la teva eina {{.ToolName}} des de
{{.FromDate}} fins {{.ToDate}}.

Si us plau revisa els detalls de la sol·licitud i respon a {{.ButtonUrl}}.

Gestionar preferències de notificacions: {{.NotificationsUrl}}`,
	},
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
	Subjects: map[string]string{
		"en": "Your tool request has been accepted",
		"es": "Tu solicitud de herramienta ha sido aceptada",
		"ca": "La teva sol·licitud d'eina ha estat acceptada",
	},
	PlainBodies: map[string]string{
		"en": `Your request for tool {{.ToolName}} from
{{.FromDate}} to {{.ToDate}} has been accepted by {{.UserName}}.

Check your booking on {{.ButtonUrl}}.

Manage notification preferences: {{.NotificationsUrl}}`,
		"es": `Tu solicitud para la herramienta {{.ToolName}} desde
{{.FromDate}} hasta {{.ToDate}} ha sido aceptada por {{.UserName}}.

Revisa tu reserva en {{.ButtonUrl}}.

Gestionar preferencias de notificaciones: {{.NotificationsUrl}}`,
		"ca": `La teva sol·licitud per l'eina {{.ToolName}} des de
{{.FromDate}} fins {{.ToDate}} ha estat acceptada per {{.UserName}}.

Revisa la teva reserva a {{.ButtonUrl}}.

Gestionar preferències de notificacions: {{.NotificationsUrl}}`,
	},
}

var NomadicToolHolderIsChangedMailNotification = MailTemplate{
	File: "nomadic_holder_is_changed",
	Placeholder: notifications.Notification{
		Subject: "Nomadic tool holder has been changed",
		PlainBody: `The holder for nomadic tool {{.ToolName}} has been changed to {{.UserName}}.
Check new pick up location on {{.ButtonUrl}}.`,
	},
	WebAppURI: "/bookings",
	Subjects: map[string]string{
		"en": "Nomadic tool holder has been changed",
		"es": "El portador de la herramienta nómada ha cambiado",
		"ca": "El portador de l'eina nòmada ha canviat",
	},
	PlainBodies: map[string]string{
		"en": `The holder for nomadic tool {{.ToolName}} has been changed to {{.UserName}}.
Check new pick up location on {{.ButtonUrl}}.

Manage notification preferences: {{.NotificationsUrl}}`,
		"es": `El portador de la herramienta nómada {{.ToolName}} ha cambiado a {{.UserName}}.
Consulta el nuevo lugar de recogida en {{.ButtonUrl}}.

Gestionar preferencias de notificaciones: {{.NotificationsUrl}}`,
		"ca": `El portador de l'eina nòmada {{.ToolName}} ha canviat a {{.UserName}}.
Consulta el nou lloc de recollida a {{.ButtonUrl}}.

Gestionar preferències de notificacions: {{.NotificationsUrl}}`,
	},
}
