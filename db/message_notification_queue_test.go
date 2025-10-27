package db

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMessageNotificationQueueService(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Start MongoDB container
	container, err := StartMongoContainer(ctx)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to start MongoDB container"))
	defer func() { _ = container.Terminate(ctx) }()

	// Get MongoDB connection string
	mongoURI, err := container.Endpoint(ctx, "mongodb")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to get MongoDB connection string"))

	// Create a MongoDB client
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create MongoDB client"))
	defer func() { _ = client.Disconnect(ctx) }()

	// Use a random database name for isolation
	dbName := RandomDatabaseName()
	database := client.Database(dbName)

	// Initialize Database with all services
	db := &Database{
		Client:   client,
		Database: database,
	}
	db.UserService = NewUserService(db)
	db.CommunityService = NewCommunityService(db)
	db.MessageService = NewMessageService(db)
	db.MessageNotificationQueueService = NewMessageNotificationQueueService(db)

	// Wire up message service with notification queue service
	db.MessageService.MessageNotificationQueueService = db.MessageNotificationQueueService

	c.Run("EnqueueOrUpdateNotification - Creates New Notification", func(c *qt.C) {
		// Create test users
		user1ID, err := CreateTestUser(ctx, db.UserService, "enqueue1@test.com", "Enqueue User 1")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

		user2ID, err := CreateTestUser(ctx, db.UserService, "enqueue2@test.com", "Enqueue User 2")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

		// Create conversation key
		conversationKey := GenerateConversationKeyFromData(MessageTypePrivate, user1ID, user2ID, nil)

		// Enqueue notification
		err = db.MessageNotificationQueueService.EnqueueOrUpdateNotification(
			ctx,
			user1ID,
			conversationKey,
			MessageTypePrivate,
			primitive.NewObjectID(),
			60,
		)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to enqueue notification"))

		// Verify notification was created
		notifications, err := db.MessageNotificationQueueService.GetPendingNotifications(ctx, time.Now().Add(2*time.Hour))
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending notifications"))

		found := false
		for _, notif := range notifications {
			if notif.UserID == user1ID && notif.ConversationKey == conversationKey {
				found = true
				c.Assert(notif.MessageType, qt.Equals, MessageTypePrivate)
				c.Assert(notif.UnreadCount, qt.Equals, 1)
				c.Assert(notif.Processed, qt.Equals, false)
				break
			}
		}
		c.Assert(found, qt.IsTrue, qt.Commentf("Notification should be created"))
	})

	c.Run("EnqueueOrUpdateNotification - Updates Existing Notification", func(c *qt.C) {
		// Create test users
		user1ID, err := CreateTestUser(ctx, db.UserService, "update1@test.com", "Update User 1")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

		user2ID, err := CreateTestUser(ctx, db.UserService, "update2@test.com", "Update User 2")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

		// Create conversation key
		conversationKey := GenerateConversationKeyFromData(MessageTypePrivate, user1ID, user2ID, nil)

		// Enqueue first notification
		err = db.MessageNotificationQueueService.EnqueueOrUpdateNotification(
			ctx,
			user1ID,
			conversationKey,
			MessageTypePrivate,
			primitive.NewObjectID(),
			60,
		)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to enqueue first notification"))

		time.Sleep(100 * time.Millisecond)

		// Enqueue second notification (should update count)
		err = db.MessageNotificationQueueService.EnqueueOrUpdateNotification(
			ctx,
			user1ID,
			conversationKey,
			MessageTypePrivate,
			primitive.NewObjectID(),
			60,
		)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to enqueue second notification"))

		// Verify notification count was updated
		notifications, err := db.MessageNotificationQueueService.GetPendingNotifications(ctx, time.Now().Add(2*time.Hour))
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending notifications"))

		found := false
		for _, notif := range notifications {
			if notif.UserID == user1ID && notif.ConversationKey == conversationKey {
				found = true
				c.Assert(notif.UnreadCount, qt.Equals, 2, qt.Commentf("Unread count should be updated to 2"))
				break
			}
		}
		c.Assert(found, qt.IsTrue, qt.Commentf("Notification should exist"))
	})

	c.Run("RemoveNotification - Removes Notification When Read", func(c *qt.C) {
		// Create test users
		user1ID, err := CreateTestUser(ctx, db.UserService, "remove1@test.com", "Remove User 1")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

		user2ID, err := CreateTestUser(ctx, db.UserService, "remove2@test.com", "Remove User 2")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

		// Create conversation key
		conversationKey := GenerateConversationKeyFromData(MessageTypePrivate, user1ID, user2ID, nil)

		// Enqueue notification
		err = db.MessageNotificationQueueService.EnqueueOrUpdateNotification(
			ctx,
			user1ID,
			conversationKey,
			MessageTypePrivate,
			primitive.NewObjectID(),
			60,
		)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to enqueue notification"))

		// Verify notification exists
		notifications, err := db.MessageNotificationQueueService.GetPendingNotifications(ctx, time.Now().Add(2*time.Hour))
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending notifications"))

		found := false
		for _, notif := range notifications {
			if notif.UserID == user1ID && notif.ConversationKey == conversationKey {
				found = true
				break
			}
		}
		c.Assert(found, qt.IsTrue, qt.Commentf("Notification should exist before removal"))

		// Remove notification
		err = db.MessageNotificationQueueService.RemoveNotification(ctx, user1ID, conversationKey)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to remove notification"))

		// Verify notification was removed
		notifications, err = db.MessageNotificationQueueService.GetPendingNotifications(ctx, time.Now().Add(2*time.Hour))
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending notifications"))

		found = false
		for _, notif := range notifications {
			if notif.UserID == user1ID && notif.ConversationKey == conversationKey {
				found = true
				break
			}
		}
		c.Assert(found, qt.IsFalse, qt.Commentf("Notification should be removed"))
	})

	c.Run("GetPendingNotifications - Returns Only Due Notifications", func(c *qt.C) {
		// Create test users
		user1ID, err := CreateTestUser(ctx, db.UserService, "pending1@test.com", "Pending User 1")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

		user2ID, err := CreateTestUser(ctx, db.UserService, "pending2@test.com", "Pending User 2")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

		// Create conversation keys
		conversationKey1 := GenerateConversationKeyFromData(MessageTypePrivate, user1ID, user2ID, nil)

		// Enqueue notification with short delay (should be pending)
		err = db.MessageNotificationQueueService.EnqueueOrUpdateNotification(
			ctx,
			user1ID,
			conversationKey1,
			MessageTypePrivate,
			primitive.NewObjectID(),
			0, // No delay - should be immediately available
		)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to enqueue notification"))

		// Get pending notifications
		notifications, err := db.MessageNotificationQueueService.GetPendingNotifications(ctx, time.Now())
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending notifications"))

		// Should find the notification
		found := false
		for _, notif := range notifications {
			if notif.UserID == user1ID && notif.ConversationKey == conversationKey1 {
				found = true
				break
			}
		}
		c.Assert(found, qt.IsTrue, qt.Commentf("Notification should be pending"))
	})

	c.Run("Community Message Notification", func(c *qt.C) {
		// Create test users
		user1ID, err := CreateTestUser(ctx, db.UserService, "community1@test.com", "Community User 1")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

		user2ID, err := CreateTestUser(ctx, db.UserService, "community2@test.com", "Community User 2")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

		// Create a community
		community, err := db.CommunityService.CreateCommunity(ctx, "Test Community", nil, user1ID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create community"))

		// Add user2 to community
		err = db.UserService.AddUserToCommunity(ctx, user2ID, community.ID, CommunityRoleUser)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to add user2 to community"))

		// Create conversation key
		conversationKey := GenerateConversationKeyFromData(MessageTypeCommunity, user1ID, primitive.NilObjectID, &community.ID)

		// Enqueue notification for community message
		err = db.MessageNotificationQueueService.EnqueueOrUpdateNotification(
			ctx,
			user2ID,
			conversationKey,
			MessageTypeCommunity,
			primitive.NewObjectID(),
			60,
		)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to enqueue community notification"))

		// Verify notification was created
		notifications, err := db.MessageNotificationQueueService.GetPendingNotifications(ctx, time.Now().Add(2*time.Hour))
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending notifications"))

		found := false
		for _, notif := range notifications {
			if notif.UserID == user2ID && notif.ConversationKey == conversationKey {
				found = true
				c.Assert(notif.MessageType, qt.Equals, MessageTypeCommunity)
				break
			}
		}
		c.Assert(found, qt.IsTrue, qt.Commentf("Community notification should be enqueued"))
	})

	c.Run("General Message Notification", func(c *qt.C) {
		// Create test users
		_, err := CreateTestUser(ctx, db.UserService, "general1@test.com", "General User 1")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

		user2ID, err := CreateTestUser(ctx, db.UserService, "general2@test.com", "General User 2")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

		// Create conversation key for general messages
		conversationKey := "general"

		// Enqueue notification for general message to user2
		err = db.MessageNotificationQueueService.EnqueueOrUpdateNotification(
			ctx,
			user2ID,
			conversationKey,
			MessageTypeGeneral,
			primitive.NewObjectID(),
			60,
		)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to enqueue general notification"))

		// Verify notification was created
		notifications, err := db.MessageNotificationQueueService.GetPendingNotifications(ctx, time.Now().Add(2*time.Hour))
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending notifications"))

		found := false
		for _, notif := range notifications {
			if notif.UserID == user2ID && notif.ConversationKey == conversationKey {
				found = true
				c.Assert(notif.MessageType, qt.Equals, MessageTypeGeneral)
				break
			}
		}
		c.Assert(found, qt.IsTrue, qt.Commentf("General notification should be enqueued"))
	})
}
