package types

// NotificationType represents the different types of notifications available in the system
type NotificationType string

// Notification type constants
const (
	NotificationNewIncomingRequest NotificationType = "incoming_requests"
	NotificationBookingAccepted    NotificationType = "booking_accepted"
)

// GetAllNotificationTypes returns all available notification types
func GetAllNotificationTypes() []NotificationType {
	return []NotificationType{
		NotificationNewIncomingRequest,
		NotificationBookingAccepted,
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
		preferences[notificationType.String()] = true // All notifications enabled by default
	}
	return preferences
}
