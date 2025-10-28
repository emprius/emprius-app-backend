package db

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MessageNotificationQueue represents a pending message digest notification
type MessageNotificationQueue struct {
	ID                       primitive.ObjectID `bson:"_id,omitempty"`
	UserID                   primitive.ObjectID `bson:"userId"`
	ConversationKey          string             `bson:"conversationKey"`
	MessageType              MessageType        `bson:"messageType"`
	FirstUnreadMessageID     primitive.ObjectID `bson:"firstUnreadMessageId"`
	FirstUnreadTime          time.Time          `bson:"firstUnreadTime"`
	UnreadCount              int                `bson:"unreadCount"`
	NotificationScheduledFor time.Time          `bson:"notificationScheduledFor"`
	Processed                bool               `bson:"processed"`
	CreatedAt                time.Time          `bson:"createdAt"`
	UpdatedAt                time.Time          `bson:"updatedAt"`
}

// MessageNotificationQueueService provides methods to interact with the notification queue
type MessageNotificationQueueService struct {
	Collection *mongo.Collection
}

// NewMessageNotificationQueueService creates a new MessageNotificationQueueService
func NewMessageNotificationQueueService(db *Database) *MessageNotificationQueueService {
	return &MessageNotificationQueueService{
		Collection: db.Database.Collection("message_notification_queue"),
	}
}

// EnqueueOrUpdateNotification creates or updates a notification queue entry for a message
func (s *MessageNotificationQueueService) EnqueueOrUpdateNotification(
	ctx context.Context,
	userID primitive.ObjectID,
	conversationKey string,
	messageType MessageType,
	messageID primitive.ObjectID,
	delayMinutes int,
) error {
	now := time.Now()

	// Try to find existing queue entry
	filter := bson.M{
		"userId":          userID,
		"conversationKey": conversationKey,
		"processed":       false,
	}

	var existing MessageNotificationQueue
	err := s.Collection.FindOne(ctx, filter).Decode(&existing)

	if err == mongo.ErrNoDocuments {
		// Create new entry
		entry := MessageNotificationQueue{
			UserID:                   userID,
			ConversationKey:          conversationKey,
			MessageType:              messageType,
			FirstUnreadMessageID:     messageID,
			FirstUnreadTime:          now,
			UnreadCount:              1,
			NotificationScheduledFor: now.Add(time.Duration(delayMinutes) * time.Minute),
			Processed:                false,
			CreatedAt:                now,
			UpdatedAt:                now,
		}

		_, err := s.Collection.InsertOne(ctx, entry)
		return err
	} else if err != nil {
		return err
	}

	// Update existing entry - increment unread count
	update := bson.M{
		"$inc": bson.M{
			"unreadCount": 1,
		},
		"$set": bson.M{
			"updatedAt": now,
		},
	}

	_, err = s.Collection.UpdateOne(ctx, filter, update)
	return err
}

// RemoveNotification removes a notification queue entry when messages are read
func (s *MessageNotificationQueueService) RemoveNotification(
	ctx context.Context,
	userID primitive.ObjectID,
	conversationKey string,
) error {
	filter := bson.M{
		"userId":          userID,
		"conversationKey": conversationKey,
		"processed":       false,
	}

	_, err := s.Collection.DeleteOne(ctx, filter)
	return err
}

// GetPendingNotifications retrieves notifications that are ready to be sent
func (s *MessageNotificationQueueService) GetPendingNotifications(
	ctx context.Context,
	currentTime time.Time,
) ([]*MessageNotificationQueue, error) {
	filter := bson.M{
		"processed":                false,
		"notificationScheduledFor": bson.M{"$lte": currentTime},
	}

	cursor, err := s.Collection.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "notificationScheduledFor", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("error closing cursor")
		}
	}()

	var notifications []*MessageNotificationQueue
	if err := cursor.All(ctx, &notifications); err != nil {
		return nil, err
	}

	return notifications, nil
}

// MarkAsProcessed marks a notification as processed and deletes it
func (s *MessageNotificationQueueService) MarkAsProcessed(
	ctx context.Context,
	notificationID primitive.ObjectID,
) error {
	filter := bson.M{"_id": notificationID}
	_, err := s.Collection.DeleteOne(ctx, filter)
	return err
}

// VerifyMessagesStillUnread verifies if messages in the conversation are still unread
// Returns true if there are unread messages, false otherwise
func (s *MessageNotificationQueueService) VerifyMessagesStillUnread(
	ctx context.Context,
	readStatusCollection *mongo.Collection,
	userID primitive.ObjectID,
	conversationKey string,
	firstUnreadMessageID primitive.ObjectID,
) (bool, int, error) {
	// Check the read status for this conversation
	var readStatus MessageReadStatus
	filter := bson.M{
		"userId":          userID,
		"conversationKey": conversationKey,
	}

	err := readStatusCollection.FindOne(ctx, filter).Decode(&readStatus)
	if err == mongo.ErrNoDocuments {
		// No read status means messages are unread
		return true, 1, nil
	} else if err != nil {
		return false, 0, err
	}

	// If unread count is 0, messages have been read
	if readStatus.UnreadCount == 0 {
		return false, 0, nil
	}

	// Messages are still unread
	return true, int(readStatus.UnreadCount), nil
}
