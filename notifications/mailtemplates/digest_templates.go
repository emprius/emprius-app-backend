package mailtemplates

import "github.com/emprius/emprius-app-backend/notifications"

// PrivateMessageDigestMailNotification notification used to notify a user about unread private messages
var PrivateMessageDigestMailNotification = MailTemplate{
	File: "private_message_digest",
	Placeholder: notifications.Notification{
		Subject: "You have unread messages from {{.SenderName}}",
		PlainBody: `{{.SenderName}} sent you {{.UnreadCount}} message(s).

View your messages on {{.ButtonUrl}}.`,
	},
	WebAppURI: "/messages",
	Subjects: map[string]string{
		"en": "You have unread messages from {{.SenderName}}",
		"es": "Tienes mensajes sin leer de {{.SenderName}}",
		"ca": "Tens missatges sense llegir de {{.SenderName}}",
	},
	PlainBodies: map[string]string{
		"en": `{{.SenderName}} sent you {{.UnreadCount}} message(s).

View your messages on {{.ButtonUrl}}.

Manage notification preferences: {{.NotificationsUrl}}`,
		"es": `{{.SenderName}} te envió {{.UnreadCount}} mensaje(s).

Ve tus mensajes en {{.ButtonUrl}}.

Gestionar preferencias de notificaciones: {{.NotificationsUrl}}`,
		"ca": `{{.SenderName}} t'ha enviat {{.UnreadCount}} missatge(s).

Consulta els teus missatges a {{.ButtonUrl}}.

Gestionar preferències de notificacions: {{.NotificationsUrl}}`,
	},
}

// CommunityMessageDigestMailNotification notification used to notify a user about unread community messages
var CommunityMessageDigestMailNotification = MailTemplate{
	File: "community_message_digest",
	Placeholder: notifications.Notification{
		Subject: "You have {{.UnreadCount}} unread message(s) in {{.CommunityName}}",
		PlainBody: `There are {{.UnreadCount}} unread message(s) in the {{.CommunityName}} community.

View your messages on {{.ButtonUrl}}.`,
	},
	WebAppURI: "/messages",
	Subjects: map[string]string{
		"en": "You have {{.UnreadCount}} unread message(s) in {{.CommunityName}}",
		"es": "Tienes {{.UnreadCount}} mensaje(s) sin leer en {{.CommunityName}}",
		"ca": "Tens {{.UnreadCount}} missatge(s) sense llegir a {{.CommunityName}}",
	},
	PlainBodies: map[string]string{
		"en": `There are {{.UnreadCount}} unread message(s) in the {{.CommunityName}} community.

View your messages on {{.ButtonUrl}}.

Manage notification preferences: {{.NotificationsUrl}}`,
		"es": `Hay {{.UnreadCount}} mensaje(s) sin leer en la comunidad {{.CommunityName}}.

Ve tus mensajes en {{.ButtonUrl}}.

Gestionar preferencias de notificaciones: {{.NotificationsUrl}}`,
		"ca": `Hi ha {{.UnreadCount}} missatge(s) sense llegir a la comunitat {{.CommunityName}}.

Consulta els teus missatges a {{.ButtonUrl}}.

Gestionar preferències de notificacions: {{.NotificationsUrl}}`,
	},
}

// GeneralMessageDigestMailNotification notification used to notify a user about unread general forum messages
var GeneralMessageDigestMailNotification = MailTemplate{
	File: "general_message_digest",
	Placeholder: notifications.Notification{
		Subject: "You have {{.UnreadCount}} unread message(s) in the General Forum",
		PlainBody: `There are {{.UnreadCount}} unread message(s) in the General Forum.

View your messages on {{.ButtonUrl}}.`,
	},
	WebAppURI: "/messages",
	Subjects: map[string]string{
		"en": "You have {{.UnreadCount}} unread message(s) in the General Forum",
		"es": "Tienes {{.UnreadCount}} mensaje(s) sin leer en el Foro General",
		"ca": "Tens {{.UnreadCount}} missatge(s) sense llegir al Fòrum General",
	},
	PlainBodies: map[string]string{
		"en": `There are {{.UnreadCount}} unread message(s) in the General Forum.

View your messages on {{.ButtonUrl}}.

Manage notification preferences: {{.NotificationsUrl}}`,
		"es": `Hay {{.UnreadCount}} mensaje(s) sin leer en el Foro General.

Ve tus mensajes en {{.ButtonUrl}}.

Gestionar preferencias de notificaciones: {{.NotificationsUrl}}`,
		"ca": `Hi ha {{.UnreadCount}} missatge(s) sense llegir al Fòrum General.

Consulta els teus missatges a {{.ButtonUrl}}.

Gestionar preferències de notificacions: {{.NotificationsUrl}}`,
	},
}

// DailyMessageDigestMailNotification notification used to send a daily digest with all unread message counts
var DailyMessageDigestMailNotification = MailTemplate{
	File: "daily_message_digest",
	Placeholder: notifications.Notification{
		Subject: "Your Daily Message Digest - {{.TotalUnread}} unread message(s)",
		PlainBody: `You have {{.TotalUnread}} unread message(s).

View your messages on {{.ButtonUrl}}.`,
	},
	WebAppURI: "/messages",
	Subjects: map[string]string{
		"en": "Your Daily Message Digest - {{.TotalUnread}} unread message(s)",
		"es": "Tu Resumen Diario de Mensajes - {{.TotalUnread}} mensaje(s) sin leer",
		"ca": "El Teu Resum Diari de Missatges - {{.TotalUnread}} missatge(s) sense llegir",
	},
	PlainBodies: map[string]string{
		"en": `You have {{.TotalUnread}} unread message(s).

View your messages on {{.ButtonUrl}}.

Manage notification preferences: {{.NotificationsUrl}}`,
		"es": `Tienes {{.TotalUnread}} mensaje(s) sin leer.

Ve tus mensajes en {{.ButtonUrl}}.

Gestionar preferencias de notificaciones: {{.NotificationsUrl}}`,
		"ca": `Tens {{.TotalUnread}} missatge(s) sense llegir.

Consulta els teus missatges a {{.ButtonUrl}}.

Gestionar preferències de notificacions: {{.NotificationsUrl}}`,
	},
}
