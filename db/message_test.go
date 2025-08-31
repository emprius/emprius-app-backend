package db

import (
	"context"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/types"
	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMessage_Validate(t *testing.T) {
	tests := []struct {
		name    string
		message *Message
		wantErr bool
	}{
		{
			name: "valid private message with content",
			message: &Message{
				Type:        MessageTypePrivate,
				SenderID:    primitive.NewObjectID(),
				RecipientID: func() *primitive.ObjectID { id := primitive.NewObjectID(); return &id }(),
				Content:     "Hello, world!",
			},
			wantErr: false,
		},
		{
			name: "valid private message with images",
			message: &Message{
				Type:        MessageTypePrivate,
				SenderID:    primitive.NewObjectID(),
				RecipientID: func() *primitive.ObjectID { id := primitive.NewObjectID(); return &id }(),
				Images:      []types.HexBytes{[]byte("test-hash")},
			},
			wantErr: false,
		},
		{
			name: "invalid message without content or images",
			message: &Message{
				Type:        MessageTypePrivate,
				SenderID:    primitive.NewObjectID(),
				RecipientID: func() *primitive.ObjectID { id := primitive.NewObjectID(); return &id }(),
			},
			wantErr: true,
		},
		{
			name: "invalid private message without recipient",
			message: &Message{
				Type:     MessageTypePrivate,
				SenderID: primitive.NewObjectID(),
				Content:  "Hello, world!",
			},
			wantErr: true,
		},
		{
			name: "valid community message",
			message: &Message{
				Type:        MessageTypeCommunity,
				SenderID:    primitive.NewObjectID(),
				CommunityID: func() *primitive.ObjectID { id := primitive.NewObjectID(); return &id }(),
				Content:     "Hello, community!",
			},
			wantErr: false,
		},
		{
			name: "invalid community message without community ID",
			message: &Message{
				Type:     MessageTypeCommunity,
				SenderID: primitive.NewObjectID(),
				Content:  "Hello, community!",
			},
			wantErr: true,
		},
		{
			name: "valid general message",
			message: &Message{
				Type:     MessageTypeGeneral,
				SenderID: primitive.NewObjectID(),
				Content:  "Hello, everyone!",
			},
			wantErr: false,
		},
		{
			name: "invalid message with too many images",
			message: &Message{
				Type:        MessageTypePrivate,
				SenderID:    primitive.NewObjectID(),
				RecipientID: func() *primitive.ObjectID { id := primitive.NewObjectID(); return &id }(),
				Images: []types.HexBytes{
					[]byte("hash1"), []byte("hash2"), []byte("hash3"), []byte("hash4"),
					[]byte("hash5"), []byte("hash6"), []byte("hash7"), []byte("hash8"),
					[]byte("hash9"), []byte("hash10"), []byte("hash11"), // 11 images > 10 limit
				},
			},
			wantErr: true,
		},
		{
			name: "invalid message with content too long",
			message: &Message{
				Type:        MessageTypePrivate,
				SenderID:    primitive.NewObjectID(),
				RecipientID: func() *primitive.ObjectID { id := primitive.NewObjectID(); return &id }(),
				Content:     string(make([]byte, 5001)), // > 5000 character limit
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Message.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMessage_GenerateConversationKey(t *testing.T) {
	userID1 := primitive.NewObjectID()
	userID2 := primitive.NewObjectID()
	communityID := primitive.NewObjectID()

	tests := []struct {
		name    string
		message *Message
		want    string
	}{
		{
			name: "private message conversation key",
			message: &Message{
				Type:        MessageTypePrivate,
				SenderID:    userID1,
				RecipientID: &userID2,
			},
			want: "private:" + minString(userID1.Hex(), userID2.Hex()) + ":" + maxString(userID1.Hex(), userID2.Hex()),
		},
		{
			name: "private message conversation key (reversed order)",
			message: &Message{
				Type:        MessageTypePrivate,
				SenderID:    userID2,
				RecipientID: &userID1,
			},
			want: "private:" + minString(userID1.Hex(), userID2.Hex()) + ":" + maxString(userID1.Hex(), userID2.Hex()),
		},
		{
			name: "community message conversation key",
			message: &Message{
				Type:        MessageTypeCommunity,
				SenderID:    userID1,
				CommunityID: &communityID,
			},
			want: "community:" + communityID.Hex(),
		},
		{
			name: "general message conversation key",
			message: &Message{
				Type:     MessageTypeGeneral,
				SenderID: userID1,
			},
			want: "general",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.message.GenerateConversationKey()
			if got != tt.want {
				t.Errorf("Message.GenerateConversationKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessageService(t *testing.T) {
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

	// Initialize all required services
	db.UserService = NewUserService(db)
	db.MessageService = NewMessageService(db)

	c.Run("SendMessage", func(c *qt.C) {
		// Create test users
		user1ID, err := CreateTestUser(ctx, db.UserService, "user1@test.com", "User One")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

		user2ID, err := CreateTestUser(ctx, db.UserService, "user2@test.com", "User Two")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

		// Test sending a valid private message
		message := &Message{
			Type:        MessageTypePrivate,
			SenderID:    user1ID,
			RecipientID: &user2ID,
			Content:     "Hello, User Two!",
			CreatedAt:   time.Now(),
		}

		result, err := db.MessageService.SendMessage(ctx, message)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to send message"))
		c.Assert(result.ID, qt.Not(qt.Equals), primitive.NilObjectID, qt.Commentf("Message ID should not be nil"))

		// Test sending a general message
		generalMessage := &Message{
			Type:      MessageTypeGeneral,
			SenderID:  user1ID,
			Content:   "Hello, everyone!",
			CreatedAt: time.Now(),
		}

		result, err = db.MessageService.SendMessage(ctx, generalMessage)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to send general message"))
		c.Assert(result.ID, qt.Not(qt.Equals), primitive.NilObjectID, qt.Commentf("General message ID should not be nil"))
	})

	c.Run("GetMessages", func(c *qt.C) {
		// Create test users
		user1ID, err := CreateTestUser(ctx, db.UserService, "getmsg1@test.com", "GetMsg User One")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

		user2ID, err := CreateTestUser(ctx, db.UserService, "getmsg2@test.com", "GetMsg User Two")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

		// Send a few messages
		messages := []*Message{
			{
				Type:        MessageTypePrivate,
				SenderID:    user1ID,
				RecipientID: &user2ID,
				Content:     "First message",
				CreatedAt:   time.Now().Add(-2 * time.Hour),
			},
			{
				Type:        MessageTypePrivate,
				SenderID:    user2ID,
				RecipientID: &user1ID,
				Content:     "Second message",
				CreatedAt:   time.Now().Add(-1 * time.Hour),
			},
			{
				Type:        MessageTypePrivate,
				SenderID:    user1ID,
				RecipientID: &user2ID,
				Content:     "Third message",
				CreatedAt:   time.Now(),
			},
		}

		for _, msg := range messages {
			_, err := db.MessageService.SendMessage(ctx, msg)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to send test message"))
		}

		// Get messages for the conversation
		filter := MessageFilter{
			Type:             MessageTypePrivate,
			UserID:           user1ID,
			ConversationWith: &user2ID,
		}
		retrievedMessages, total, err := db.MessageService.GetMessages(ctx, filter, 0, 10)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get messages"))
		c.Assert(len(retrievedMessages), qt.Equals, 3, qt.Commentf("Expected 3 messages"))
		c.Assert(total, qt.Equals, int64(3), qt.Commentf("Expected total count of 3"))

		// Messages should be ordered by creation time (newest first)
		c.Assert(retrievedMessages[0].Content, qt.Equals, "Third message", qt.Commentf("First message should be the newest"))
		c.Assert(retrievedMessages[2].Content, qt.Equals, "First message", qt.Commentf("Last message should be the oldest"))
	})

	c.Run("MarkAsRead", func(c *qt.C) {
		// Create test users
		user1ID, err := CreateTestUser(ctx, db.UserService, "read1@test.com", "Read User One")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

		user2ID, err := CreateTestUser(ctx, db.UserService, "read2@test.com", "Read User Two")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

		// Send a message
		message := &Message{
			Type:        MessageTypePrivate,
			SenderID:    user1ID,
			RecipientID: &user2ID,
			Content:     "Test read message",
			CreatedAt:   time.Now(),
		}

		result, err := db.MessageService.SendMessage(ctx, message)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to send message"))
		messageID := result.ID

		// Mark message as read by user2
		err = db.MessageService.MarkAsRead(ctx, user2ID, messageID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to mark message as read"))

		// Verify the message is marked as read by checking unread counts
		unreadCounts, err := db.MessageService.GetUnreadCounts(ctx, user2ID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get unread counts after marking as read"))
		c.Assert(unreadCounts.Private, qt.Equals, int64(0), qt.Commentf("Should have 0 unread private messages after marking as read"))
	})

	c.Run("GetUnreadCounts", func(c *qt.C) {
		// Create test users
		user1ID, err := CreateTestUser(ctx, db.UserService, "unread1@test.com", "Unread User One")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

		user2ID, err := CreateTestUser(ctx, db.UserService, "unread2@test.com", "Unread User Two")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

		// Send messages to user2
		messages := []*Message{
			{
				Type:        MessageTypePrivate,
				SenderID:    user1ID,
				RecipientID: &user2ID,
				Content:     "Unread message 1",
				CreatedAt:   time.Now(),
			},
			{
				Type:        MessageTypePrivate,
				SenderID:    user1ID,
				RecipientID: &user2ID,
				Content:     "Unread message 2",
				CreatedAt:   time.Now(),
			},
			{
				Type:      MessageTypeGeneral,
				SenderID:  user1ID,
				Content:   "General unread message",
				CreatedAt: time.Now(),
			},
		}

		for _, msg := range messages {
			_, err := db.MessageService.SendMessage(ctx, msg)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to send test message"))
		}

		// Get unread counts for user2
		unreadCounts, err := db.MessageService.GetUnreadCounts(ctx, user2ID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get unread counts"))
		c.Assert(unreadCounts.Private, qt.Equals, int64(2), qt.Commentf("Expected 2 unread private messages"))
		c.Assert(unreadCounts.GeneralForum, qt.Equals, int64(1), qt.Commentf("Expected 1 unread general message"))
	})
}

// Helper functions for testing
func minString(a, b string) string {
	if a < b {
		return a
	}
	return b
}

func maxString(a, b string) string {
	if a > b {
		return a
	}
	return b
}
