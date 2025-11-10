package types

// NotificationType represents the different types of notifications available in the system
type NotificationType string

// Notification type constants
const (
	NotificationNewIncomingRequest       NotificationType = "incoming_requests"
	NotificationBookingAccepted          NotificationType = "booking_accepted"
	NotificationNomadicToolHolderChanged NotificationType = "tool_holder_changed"

	// Message notifications
	NotificationPrivateMessages      NotificationType = "private_messages"
	NotificationCommunityMessages    NotificationType = "community_messages"
	NotificationGeneralForumMessages NotificationType = "general_forum_messages"
	NotificationDailyMessageDigest   NotificationType = "daily_message_digest"
)

// GetAllNotificationTypes returns all available notification types
func GetAllNotificationTypes() []NotificationType {
	return []NotificationType{
		NotificationNewIncomingRequest,
		NotificationBookingAccepted,
		NotificationNomadicToolHolderChanged,
		NotificationPrivateMessages,
		NotificationCommunityMessages,
		NotificationGeneralForumMessages,
		NotificationDailyMessageDigest,
	}
}

// IsValidNotificationType checks if a string represents a valid notification type
func IsValidNotificationType(notificationType string) bool {
	for _, nt := range GetAllNotificationTypes() {
		if string(nt) == notificationType {
			return true
		}
	}
	return false
}

// String returns the string representation of the notification type
func (nt NotificationType) String() string {
	return string(nt)
}

// GetDefaultNotificationPreferences returns the default notification preferences for new users
func GetDefaultNotificationPreferences() map[string]bool {
	preferences := make(map[string]bool)
	for _, notificationType := range GetAllNotificationTypes() {
		switch notificationType {
		case NotificationPrivateMessages,
			NotificationCommunityMessages,
			NotificationGeneralForumMessages,
			NotificationDailyMessageDigest:
			preferences[notificationType.String()] = false // Disable message notifications by default
		default:
			preferences[notificationType.String()] = true // Rest of notifications enabled by default
		}

	}
	return preferences
}
