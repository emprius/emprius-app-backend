package digest_scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/notifications"
	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"
	"github.com/emprius/emprius-app-backend/types"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DigestScheduler manages the background task for sending message digest notifications
type DigestScheduler struct {
	database       *db.Database
	mailService    notifications.NotificationService
	timeProvider   TimeProvider
	stopChan       chan struct{}
	tickerInterval time.Duration
	mu             sync.RWMutex // protects timeProvider and tickerInterval
}

// NewDigestScheduler creates a new digest scheduler
func NewDigestScheduler(
	database *db.Database,
	mailService notifications.NotificationService,
) *DigestScheduler {
	return &DigestScheduler{
		database:       database,
		mailService:    mailService,
		timeProvider:   RealTimeProvider{},
		stopChan:       make(chan struct{}),
		tickerInterval: 1 * time.Minute, // Default production interval
	}
}

// SetTickerInterval sets a custom ticker interval (useful for testing)
func (ds *DigestScheduler) SetTickerInterval(interval time.Duration) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.tickerInterval = interval
}

// SetTimeProvider sets a custom time provider (useful for testing)
func (ds *DigestScheduler) SetTimeProvider(tp TimeProvider) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.timeProvider = tp
}

// Start begins the digest scheduler background task
func (ds *DigestScheduler) Start() {
	log.Info().Msg("starting message digest scheduler")
	go ds.run()
}

// Stop gracefully stops the digest scheduler
func (ds *DigestScheduler) Stop() {
	log.Info().Msg("stopping message digest scheduler")
	close(ds.stopChan)
}

// run is the main loop that processes pending notifications
func (ds *DigestScheduler) run() {
	ds.mu.RLock()
	interval := ds.tickerInterval
	ds.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := ds.processPendingNotifications(); err != nil {
				log.Error().Err(err).Msg("error processing pending notifications")
			}
		case <-ds.stopChan:
			log.Info().Msg("digest scheduler stopped")
			return
		}
	}
}

// ProcessPendingNotificationsNow processes pending notifications immediately (for testing)
func (ds *DigestScheduler) ProcessPendingNotificationsNow() error {
	return ds.processPendingNotifications()
}

// processPendingNotifications checks for and processes all pending notifications
func (ds *DigestScheduler) processPendingNotifications() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ds.mu.RLock()
	currentTime := ds.timeProvider.Now()
	ds.mu.RUnlock()

	// Get all pending notifications
	notifications, err := ds.database.MessageNotificationQueueService.GetPendingNotifications(ctx, currentTime)
	if err != nil {
		return fmt.Errorf("failed to get pending notifications: %w", err)
	}

	if len(notifications) == 0 {
		return nil
	}

	log.Info().Int("count", len(notifications)).Msg("processing pending message digest notifications")

	for _, notification := range notifications {
		if err := ds.processNotification(ctx, notification); err != nil {
			log.Error().
				Err(err).
				Str("notificationId", notification.ID.Hex()).
				Str("userId", notification.UserID.Hex()).
				Str("conversationKey", notification.ConversationKey).
				Msg("failed to process notification")
			// Continue processing other notifications even if one fails
		}
	}

	return nil
}

// processNotification processes a single notification
func (ds *DigestScheduler) processNotification(ctx context.Context, notification *db.MessageNotificationQueue) error {
	// Verify messages are still unread
	stillUnread, actualCount, err := ds.database.MessageNotificationQueueService.VerifyMessagesStillUnread(
		ctx,
		ds.database.MessageService.ReadStatusCollection,
		notification.UserID,
		notification.ConversationKey,
		notification.FirstUnreadMessageID,
	)
	if err != nil {
		return fmt.Errorf("failed to verify unread status: %w", err)
	}

	if !stillUnread {
		// Messages have been read, remove from queue
		log.Debug().
			Str("notificationId", notification.ID.Hex()).
			Str("userId", notification.UserID.Hex()).
			Msg("messages already read, removing notification from queue")
		return ds.database.MessageNotificationQueueService.MarkAsProcessed(ctx, notification.ID)
	}

	// Get user
	user, err := ds.database.UserService.GetUserByID(ctx, notification.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Check notification preferences
	notificationType := ds.getNotificationTypeForMessageType(notification.MessageType)
	if !user.NotificationPreferences[string(notificationType)] {
		log.Debug().
			Str("userId", user.ID.Hex()).
			Str("type", string(notificationType)).
			Msg("user has disabled this notification type, removing from queue")
		return ds.database.MessageNotificationQueueService.MarkAsProcessed(ctx, notification.ID)
	}

	// Send the appropriate email
	if err := ds.sendDigestEmail(ctx, user, notification, actualCount); err != nil {
		return fmt.Errorf("failed to send digest email: %w", err)
	}

	// Mark as processed (delete from queue)
	if err := ds.database.MessageNotificationQueueService.MarkAsProcessed(ctx, notification.ID); err != nil {
		return fmt.Errorf("failed to mark notification as processed: %w", err)
	}

	log.Info().
		Str("userId", user.ID.Hex()).
		Str("email", user.Email).
		Str("messageType", string(notification.MessageType)).
		Int("unreadCount", actualCount).
		Msg("sent message digest notification")

	return nil
}

// sendDigestEmail sends the appropriate digest email based on message type
func (ds *DigestScheduler) sendDigestEmail(
	ctx context.Context,
	user *db.User,
	notification *db.MessageNotificationQueue,
	unreadCount int,
) error {
	switch notification.MessageType {
	case db.MessageTypePrivate:
		return ds.sendPrivateMessageDigest(ctx, user, notification, unreadCount)
	case db.MessageTypeCommunity:
		return ds.sendCommunityMessageDigest(ctx, user, notification, unreadCount)
	case db.MessageTypeGeneral:
		return ds.sendGeneralMessageDigest(ctx, user, notification, unreadCount)
	default:
		return fmt.Errorf("unknown message type: %s", notification.MessageType)
	}
}

// sendPrivateMessageDigest sends a digest email for private messages
func (ds *DigestScheduler) sendPrivateMessageDigest(
	ctx context.Context,
	user *db.User,
	notification *db.MessageNotificationQueue,
	unreadCount int,
) error {
	// Extract other user ID from conversation key (format: "private:id1:id2")
	parts := strings.Split(notification.ConversationKey, ":")
	if len(parts) != 3 {
		return fmt.Errorf("invalid private conversation key format: %s", notification.ConversationKey)
	}

	// Determine which ID is the other user
	var otherUserIDStr string
	if parts[1] == user.ID.Hex() {
		otherUserIDStr = parts[2]
	} else {
		otherUserIDStr = parts[1]
	}

	otherUserID, err := primitive.ObjectIDFromHex(otherUserIDStr)
	if err != nil {
		return fmt.Errorf("invalid user ID in conversation key: %w", err)
	}

	// Get the other user's name
	otherUser, err := ds.database.UserService.GetUserByID(ctx, otherUserID)
	if err != nil {
		return fmt.Errorf("failed to get sender user: %w", err)
	}

	// Prepare email data
	emailData := struct {
		AppName     string
		LogoURL     string
		SenderName  string
		UnreadCount int
		ButtonUrl   string
	}{
		AppName:     mailtemplates.AppName,
		LogoURL:     mailtemplates.LogoURL,
		SenderName:  otherUser.Name,
		UnreadCount: unreadCount,
		ButtonUrl:   fmt.Sprintf("%s/messages?type=private&userId=%s", mailtemplates.AppUrl, otherUserIDStr),
	}

	// Execute template
	mailNotification, err := mailtemplates.PrivateMessageDigestMailNotification.ExecTemplate(emailData, user.LanguageCode)
	if err != nil {
		return fmt.Errorf("failed to execute email template: %w", err)
	}

	// Set recipient
	mailNotification.ToAddress = user.Email

	// Send email
	return ds.mailService.SendNotification(ctx, mailNotification)
}

// sendCommunityMessageDigest sends a digest email for community messages
func (ds *DigestScheduler) sendCommunityMessageDigest(
	ctx context.Context,
	user *db.User,
	notification *db.MessageNotificationQueue,
	unreadCount int,
) error {
	// Extract community ID from conversation key (format: "community:id")
	parts := strings.Split(notification.ConversationKey, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid community conversation key format: %s", notification.ConversationKey)
	}

	communityIDStr := parts[1]
	communityID, err := primitive.ObjectIDFromHex(communityIDStr)
	if err != nil {
		return fmt.Errorf("invalid community ID in conversation key: %w", err)
	}

	// Get community name
	community, err := ds.database.CommunityService.GetCommunity(ctx, communityID)
	if err != nil {
		return fmt.Errorf("failed to get community: %w", err)
	}

	// Prepare email data
	emailData := struct {
		AppName       string
		LogoURL       string
		CommunityName string
		UnreadCount   int
		ButtonUrl     string
	}{
		AppName:       mailtemplates.AppName,
		LogoURL:       mailtemplates.LogoURL,
		CommunityName: community.Name,
		UnreadCount:   unreadCount,
		ButtonUrl:     fmt.Sprintf("%s/messages?type=community&communityId=%s", mailtemplates.AppUrl, communityIDStr),
	}

	// Execute template
	mailNotification, err := mailtemplates.CommunityMessageDigestMailNotification.ExecTemplate(emailData, user.LanguageCode)
	if err != nil {
		return fmt.Errorf("failed to execute email template: %w", err)
	}

	// Set recipient
	mailNotification.ToAddress = user.Email

	// Send email
	return ds.mailService.SendNotification(ctx, mailNotification)
}

// sendGeneralMessageDigest sends a digest email for general forum messages
func (ds *DigestScheduler) sendGeneralMessageDigest(
	ctx context.Context,
	user *db.User,
	notification *db.MessageNotificationQueue,
	unreadCount int,
) error {
	// Prepare email data
	emailData := struct {
		AppName     string
		LogoURL     string
		UnreadCount int
		ButtonUrl   string
	}{
		AppName:     mailtemplates.AppName,
		LogoURL:     mailtemplates.LogoURL,
		UnreadCount: unreadCount,
		ButtonUrl:   fmt.Sprintf("%s/messages?type=general", mailtemplates.AppUrl),
	}

	// Execute template
	mailNotification, err := mailtemplates.GeneralMessageDigestMailNotification.ExecTemplate(emailData, user.LanguageCode)
	if err != nil {
		return fmt.Errorf("failed to execute email template: %w", err)
	}

	// Set recipient
	mailNotification.ToAddress = user.Email

	// Send email
	return ds.mailService.SendNotification(ctx, mailNotification)
}

// getNotificationTypeForMessageType maps message types to notification types
func (ds *DigestScheduler) getNotificationTypeForMessageType(messageType db.MessageType) types.NotificationType {
	switch messageType {
	case db.MessageTypePrivate:
		return types.NotificationPrivateMessages
	case db.MessageTypeCommunity:
		return types.NotificationCommunityMessages
	case db.MessageTypeGeneral:
		return types.NotificationGeneralForumMessages
	default:
		return types.NotificationPrivateMessages
	}
}
