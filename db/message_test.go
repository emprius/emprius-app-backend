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

// TestConversationUnreadCount tests that unread counts are properly tracked per conversation
func TestConversationUnreadCount(t *testing.T) {
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

	// Create test users
	user1ID, err := CreateTestUser(ctx, db.UserService, "unreadconv1@test.com", "Unread Conv User One")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

	user2ID, err := CreateTestUser(ctx, db.UserService, "unreadconv2@test.com", "Unread Conv User Two")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

	// Send messages from user1 to user2
	for i := 0; i < 3; i++ {
		message := &Message{
			Type:        MessageTypePrivate,
			SenderID:    user1ID,
			RecipientID: &user2ID,
			Content:     "Test message",
			CreatedAt:   time.Now(),
		}

		_, err = db.MessageService.SendMessage(ctx, message)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to send message"))
	}

	// Get conversations for user2 - should have 3 unread messages
	conversations, _, err := db.MessageService.GetConversations(ctx, user2ID, MessageTypeAll, 0, 10)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to get conversations"))
	c.Assert(len(conversations), qt.Equals, 1, qt.Commentf("Expected 1 conversation"))

	// Verify unread count before reading
	conversationKey := (&Message{
		Type:        MessageTypePrivate,
		SenderID:    user1ID,
		RecipientID: &user2ID,
	}).GenerateConversationKey()

	readStatus := &MessageReadStatus{}
	err = db.MessageService.ReadStatusCollection.FindOne(
		ctx,
		map[string]interface{}{
			"userId":          user2ID,
			"conversationKey": conversationKey,
		},
	).Decode(readStatus)

	c.Assert(err, qt.IsNil, qt.Commentf("Failed to get read status"))
	c.Assert(readStatus.UnreadCount, qt.Equals, int64(3), qt.Commentf("Expected unread count to be 3"))

	// Mark conversation as read
	err = db.MessageService.MarkConversationAsRead(ctx, user2ID, conversationKey)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to mark conversation as read"))

	// Verify unread count is now 0
	err = db.MessageService.ReadStatusCollection.FindOne(
		ctx,
		map[string]interface{}{
			"userId":          user2ID,
			"conversationKey": conversationKey,
		},
	).Decode(readStatus)

	c.Assert(err, qt.IsNil, qt.Commentf("Failed to get read status after marking as read"))
	c.Assert(readStatus.UnreadCount, qt.Equals, int64(0), qt.Commentf("Expected unread count to be 0 after marking as read"))

	// Send another message - should increment unread count to 1
	message := &Message{
		Type:        MessageTypePrivate,
		SenderID:    user1ID,
		RecipientID: &user2ID,
		Content:     "New message after read",
		CreatedAt:   time.Now(),
	}

	_, err = db.MessageService.SendMessage(ctx, message)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to send message after read"))

	// Verify unread count is now 1
	err = db.MessageService.ReadStatusCollection.FindOne(
		ctx,
		map[string]interface{}{
			"userId":          user2ID,
			"conversationKey": conversationKey,
		},
	).Decode(readStatus)

	c.Assert(err, qt.IsNil, qt.Commentf("Failed to get read status after new message"))
	c.Assert(readStatus.UnreadCount, qt.Equals, int64(1), qt.Commentf("Expected unread count to be 1 after new message"))
}

// TestCommunityMessageReadPermissions tests that only community members can read community messages
func TestCommunityMessageReadPermissions(t *testing.T) {
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
	db.CommunityService = NewCommunityService(db)
	db.MessageService = NewMessageService(db)

	// Create test users
	memberID, err := CreateTestUser(ctx, db.UserService, "member@test.com", "Community Member")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create member user"))

	nonMemberID, err := CreateTestUser(ctx, db.UserService, "nonmember@test.com", "Non Member")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create non-member user"))

	// Create a test community
	community := &Community{
		Name:      "Test Community",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		OwnerID:   memberID,
	}

	result, err := db.CommunityService.Collection.InsertOne(ctx, community)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create community"))
	communityID := result.InsertedID.(primitive.ObjectID)

	// Add member to community
	err = db.UserService.AddUserToCommunity(ctx, memberID, communityID, CommunityRoleUser)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to add user to community"))

	// Send a community message from member
	message := &Message{
		Type:        MessageTypeCommunity,
		SenderID:    memberID,
		CommunityID: &communityID,
		Content:     "Hello community!",
		CreatedAt:   time.Now(),
	}

	sentMessage, err := db.MessageService.SendMessage(ctx, message)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to send community message"))
	messageID := sentMessage.ID

	// Test 1: Community member CAN mark message as read
	err = db.MessageService.MarkAsRead(ctx, memberID, messageID)
	c.Assert(err, qt.IsNil, qt.Commentf("Community member should be able to mark message as read"))

	// Test 2: Non-member CANNOT mark message as read
	err = db.MessageService.MarkAsRead(ctx, nonMemberID, messageID)
	c.Assert(err, qt.Not(qt.IsNil), qt.Commentf("Non-member should NOT be able to mark message as read"))
	c.Assert(err.Error(), qt.Contains, "does not have permission", qt.Commentf("Error should indicate permission denied"))

	// Test 3: Test canUserReadMessage directly for member
	canRead := db.MessageService.canUserReadMessage(ctx, memberID, sentMessage)
	c.Assert(canRead, qt.IsTrue, qt.Commentf("Member should be able to read community message"))

	// Test 4: Test canUserReadMessage directly for non-member
	canRead = db.MessageService.canUserReadMessage(ctx, nonMemberID, sentMessage)
	c.Assert(canRead, qt.IsFalse, qt.Commentf("Non-member should NOT be able to read community message"))

	// Test 5: Test MarkConversationAsRead for member
	conversationKey := sentMessage.GenerateConversationKey()
	err = db.MessageService.MarkConversationAsRead(ctx, memberID, conversationKey)
	c.Assert(err, qt.IsNil, qt.Commentf("Community member should be able to mark conversation as read"))

	// Test 6: Test MarkConversationAsRead for non-member
	err = db.MessageService.MarkConversationAsRead(ctx, nonMemberID, conversationKey)
	c.Assert(err, qt.Not(qt.IsNil), qt.Commentf("Non-member should NOT be able to mark conversation as read"))
	c.Assert(err.Error(), qt.Contains, "does not have permission", qt.Commentf("Error should indicate permission denied"))
}

// TestCanUserReadMessage tests the canUserReadMessage function for all message types
func TestCanUserReadMessage(t *testing.T) {
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
	db.CommunityService = NewCommunityService(db)
	db.MessageService = NewMessageService(db)

	// Create test users
	user1ID, err := CreateTestUser(ctx, db.UserService, "testread1@test.com", "Test User One")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user1"))

	user2ID, err := CreateTestUser(ctx, db.UserService, "testread2@test.com", "Test User Two")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user2"))

	user3ID, err := CreateTestUser(ctx, db.UserService, "testread3@test.com", "Test User Three")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user3"))

	// Create a test community
	community := &Community{
		Name:      "Read Test Community",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		OwnerID:   user1ID,
	}
	result, err := db.CommunityService.Collection.InsertOne(ctx, community)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create community"))
	communityID := result.InsertedID.(primitive.ObjectID)

	// Add user1 to community
	err = db.UserService.AddUserToCommunity(ctx, user1ID, communityID, CommunityRoleUser)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to add user1 to community"))

	c.Run("Private Message", func(c *qt.C) {
		// Test private message between user1 and user2
		privateMsg := &Message{
			Type:        MessageTypePrivate,
			SenderID:    user1ID,
			RecipientID: &user2ID,
			Content:     "Private message",
		}

		// Sender can read
		canRead := db.MessageService.canUserReadMessage(ctx, user1ID, privateMsg)
		c.Assert(canRead, qt.IsTrue, qt.Commentf("Sender should be able to read private message"))

		// Recipient can read
		canRead = db.MessageService.canUserReadMessage(ctx, user2ID, privateMsg)
		c.Assert(canRead, qt.IsTrue, qt.Commentf("Recipient should be able to read private message"))

		// Other user cannot read
		canRead = db.MessageService.canUserReadMessage(ctx, user3ID, privateMsg)
		c.Assert(canRead, qt.IsFalse, qt.Commentf("Other user should NOT be able to read private message"))
	})

	c.Run("Community Message", func(c *qt.C) {
		// Test community message
		communityMsg := &Message{
			Type:        MessageTypeCommunity,
			SenderID:    user1ID,
			CommunityID: &communityID,
			Content:     "Community message",
		}

		// Community member can read
		canRead := db.MessageService.canUserReadMessage(ctx, user1ID, communityMsg)
		c.Assert(canRead, qt.IsTrue, qt.Commentf("Community member should be able to read community message"))

		// Non-member cannot read
		canRead = db.MessageService.canUserReadMessage(ctx, user2ID, communityMsg)
		c.Assert(canRead, qt.IsFalse, qt.Commentf("Non-member should NOT be able to read community message"))

		canRead = db.MessageService.canUserReadMessage(ctx, user3ID, communityMsg)
		c.Assert(canRead, qt.IsFalse, qt.Commentf("Non-member should NOT be able to read community message"))
	})

	c.Run("General Message", func(c *qt.C) {
		// Test general message
		generalMsg := &Message{
			Type:     MessageTypeGeneral,
			SenderID: user1ID,
			Content:  "General message",
		}

		// All users can read general messages
		canRead := db.MessageService.canUserReadMessage(ctx, user1ID, generalMsg)
		c.Assert(canRead, qt.IsTrue, qt.Commentf("User1 should be able to read general message"))

		canRead = db.MessageService.canUserReadMessage(ctx, user2ID, generalMsg)
		c.Assert(canRead, qt.IsTrue, qt.Commentf("User2 should be able to read general message"))

		canRead = db.MessageService.canUserReadMessage(ctx, user3ID, generalMsg)
		c.Assert(canRead, qt.IsTrue, qt.Commentf("User3 should be able to read general message"))
	})

	c.Run("Community Message with nil CommunityID", func(c *qt.C) {
		// Test edge case: community message with nil community ID
		invalidMsg := &Message{
			Type:     MessageTypeCommunity,
			SenderID: user1ID,
			Content:  "Invalid community message",
			// CommunityID is nil
		}

		// Should return false for any user
		canRead := db.MessageService.canUserReadMessage(ctx, user1ID, invalidMsg)
		c.Assert(canRead, qt.IsFalse, qt.Commentf("Should return false for community message with nil CommunityID"))
	})
}
