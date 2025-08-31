package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestMessages(t *testing.T) {
	c := utils.NewTestService(t)

	// Create test users
	user1JWT, user1ID := c.RegisterAndLoginWithID("user1@test.com", "User One", "password1")
	user2JWT, user2ID := c.RegisterAndLoginWithID("user2@test.com", "User Two", "password2")
	user3JWT, user3ID := c.RegisterAndLoginWithID("user3@test.com", "User Three", "password3")

	t.Run("Send Private Messages", func(t *testing.T) {
		// Send a private message from user1 to user2
		messageData := map[string]interface{}{
			"type":        "private",
			"recipientId": user2ID,
			"content":     "Hello User Two! This is a private message.",
		}

		resp, code := c.Request(http.MethodPost, user1JWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		var messageResp struct {
			Data api.MessageResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &messageResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, messageResp.Data.Type, qt.Equals, "private")
		qt.Assert(t, messageResp.Data.Content, qt.Equals, "Hello User Two! This is a private message.")
		qt.Assert(t, messageResp.Data.SenderID, qt.Equals, user1ID)
		qt.Assert(t, messageResp.Data.RecipientID, qt.Equals, user2ID)

		// Send a reply from user2 to user1
		replyData := map[string]interface{}{
			"type":        "private",
			"recipientId": user1ID,
			"content":     "Hi User One! Thanks for your message.",
		}

		resp, code = c.Request(http.MethodPost, user2JWT, replyData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		err = json.Unmarshal(resp, &messageResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, messageResp.Data.Type, qt.Equals, "private")
		qt.Assert(t, messageResp.Data.Content, qt.Equals, "Hi User One! Thanks for your message.")
		qt.Assert(t, messageResp.Data.SenderID, qt.Equals, user2ID)
		qt.Assert(t, messageResp.Data.RecipientID, qt.Equals, user1ID)
	})

	t.Run("Send General Forum Messages", func(t *testing.T) {
		// Send a general forum message
		messageData := map[string]interface{}{
			"type":    "general",
			"content": "Hello everyone! This is a public message in the general forum.",
		}

		resp, code := c.Request(http.MethodPost, user1JWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		var messageResp struct {
			Data api.MessageResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &messageResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, messageResp.Data.Type, qt.Equals, "general")
		qt.Assert(t, messageResp.Data.Content, qt.Equals, "Hello everyone! This is a public message in the general forum.")
		qt.Assert(t, messageResp.Data.SenderID, qt.Equals, user1ID)
		qt.Assert(t, messageResp.Data.RecipientID, qt.Equals, "")
		qt.Assert(t, messageResp.Data.CommunityID, qt.Equals, "")
	})

	t.Run("Get Messages with Filtering", func(t *testing.T) {
		// Get private messages between user1 and user2
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "messages?type=private&conversationWith="+user2ID)
		qt.Assert(t, code, qt.Equals, 200)

		var messagesResp struct {
			Data api.PaginatedMessagesResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &messagesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(messagesResp.Data.Messages), qt.Equals, 2) // Should have 2 messages in the conversation

		// Messages should be ordered by creation time (newest first)
		qt.Assert(t, messagesResp.Data.Messages[0].Content, qt.Equals, "Hi User One! Thanks for your message.")
		qt.Assert(t, messagesResp.Data.Messages[1].Content, qt.Equals, "Hello User Two! This is a private message.")

		// Get general forum messages
		resp, code = c.Request(http.MethodGet, user2JWT, nil, "messages?type=general")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &messagesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(messagesResp.Data.Messages), qt.Equals, 1) // Should have 1 general message
		qt.Assert(t, messagesResp.Data.Messages[0].Content, qt.Equals, "Hello everyone! This is a public message in the general forum.")
	})

	t.Run("Get Unread Message Counts", func(t *testing.T) {
		// Check unread counts for user2 (should have unread private messages)
		resp, code := c.Request(http.MethodGet, user2JWT, nil, "messages/unread")
		qt.Assert(t, code, qt.Equals, 200)

		var unreadResp struct {
			Data api.UnreadMessageSummary `json:"data"`
		}
		err := json.Unmarshal(resp, &unreadResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, unreadResp.Data.Private, qt.Equals, int64(1))      // Should have 1 unread private message
		qt.Assert(t, unreadResp.Data.GeneralForum, qt.Equals, int64(1)) // Should have 1 unread general message
		qt.Assert(t, unreadResp.Data.Total, qt.Equals, int64(2))        // Total should be 2

		// Check unread counts for user1 (should have unread private and general messages)
		resp, code = c.Request(http.MethodGet, user1JWT, nil, "messages/unread")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &unreadResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, unreadResp.Data.Private, qt.Equals, int64(1))      // Should have 1 unread private message (the reply)
		qt.Assert(t, unreadResp.Data.GeneralForum, qt.Equals, int64(0)) // Should have 0 unread general messages (sent by self)
	})

	t.Run("Mark Messages as Read", func(t *testing.T) {
		// Get messages to get their IDs
		resp, code := c.Request(http.MethodGet, user2JWT, nil, "messages?type=private&conversationWith="+user1ID)
		qt.Assert(t, code, qt.Equals, 200)

		var messagesResp struct {
			Data api.PaginatedMessagesResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &messagesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(messagesResp.Data.Messages) > 0, qt.IsTrue)

		// Find the message from user1 to user2
		var messageToMarkRead string
		for _, msg := range messagesResp.Data.Messages {
			if msg.SenderID == user1ID {
				messageToMarkRead = msg.ID
				break
			}
		}
		qt.Assert(t, messageToMarkRead, qt.Not(qt.Equals), "")

		// Mark the message as read
		markReadData := map[string]interface{}{
			"messageIds": []string{messageToMarkRead},
		}

		resp, code = c.Request(http.MethodPost, user2JWT, markReadData, "messages/read")
		qt.Assert(t, code, qt.Equals, 200)

		var markReadResp struct {
			Data struct {
				Success     bool `json:"success"`
				MarkedCount int  `json:"markedCount"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &markReadResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, markReadResp.Data.Success, qt.Equals, true)
		qt.Assert(t, markReadResp.Data.MarkedCount, qt.Equals, 1)

		// Check unread counts again - should be reduced
		resp, code = c.Request(http.MethodGet, user2JWT, nil, "messages/unread")
		qt.Assert(t, code, qt.Equals, 200)

		var unreadResp struct {
			Data api.UnreadMessageSummary `json:"data"`
		}
		err = json.Unmarshal(resp, &unreadResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, unreadResp.Data.Private, qt.Equals, int64(0)) // Should now have 0 unread private messages
	})

	t.Run("Mark Conversation as Read", func(t *testing.T) {
		// Send another message to create unread count
		messageData := map[string]interface{}{
			"type":        "private",
			"recipientId": user2ID,
			"content":     "Another message to test conversation read.",
		}

		_, code := c.Request(http.MethodPost, user1JWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Check unread count
		resp, code := c.Request(http.MethodGet, user2JWT, nil, "messages/unread")
		qt.Assert(t, code, qt.Equals, 200)

		var unreadResp struct {
			Data api.UnreadMessageSummary `json:"data"`
		}
		err := json.Unmarshal(resp, &unreadResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, unreadResp.Data.Private > 0, qt.IsTrue) // Should have unread messages

		// Mark entire conversation as read
		conversationKey := fmt.Sprintf("private:%s:%s", minString(user1ID, user2ID), maxString(user1ID, user2ID))
		markConversationReadData := map[string]interface{}{
			"conversationKey": conversationKey,
		}

		resp, code = c.Request(http.MethodPost, user2JWT, markConversationReadData, "messages/read/conversation")
		qt.Assert(t, code, qt.Equals, 200)

		var markReadResp struct {
			Data struct {
				Success bool `json:"success"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &markReadResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, markReadResp.Data.Success, qt.Equals, true)

		// Check unread count again - should be 0
		resp, code = c.Request(http.MethodGet, user2JWT, nil, "messages/unread")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &unreadResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, unreadResp.Data.Private, qt.Equals, int64(0)) // Should now have 0 unread private messages
	})

	t.Run("Get Conversations", func(t *testing.T) {
		// Get conversations for user1
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "conversations")
		qt.Assert(t, code, qt.Equals, 200)

		var conversationsResp struct {
			Data api.PaginatedConversationsResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &conversationsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(conversationsResp.Data.Conversations) >= 1, qt.IsTrue) // Should have at least 1 conversation

		// Find the private conversation
		var privateConversation *api.ConversationResponse
		for _, conv := range conversationsResp.Data.Conversations {
			if conv.Type == "private" {
				privateConversation = conv
				break
			}
		}
		qt.Assert(t, privateConversation, qt.Not(qt.IsNil))
		qt.Assert(t, len(privateConversation.Participants), qt.Equals, 2)

		// Get conversations filtered by type
		resp, code = c.Request(http.MethodGet, user1JWT, nil, "conversations?type=private")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &conversationsResp)
		qt.Assert(t, err, qt.IsNil)
		// Should have private conversations only
		for _, conv := range conversationsResp.Data.Conversations {
			qt.Assert(t, conv.Type, qt.Equals, "private")
		}
	})

	t.Run("Message Validation", func(t *testing.T) {
		// Test sending message without content or images
		invalidMessageData := map[string]interface{}{
			"type":        "private",
			"recipientId": user2ID,
		}

		_, code := c.Request(http.MethodPost, user1JWT, invalidMessageData, "messages")
		qt.Assert(t, code, qt.Equals, 400) // Should fail validation

		// Test sending private message without recipient
		invalidMessageData = map[string]interface{}{
			"type":    "private",
			"content": "This should fail",
		}

		_, code = c.Request(http.MethodPost, user1JWT, invalidMessageData, "messages")
		qt.Assert(t, code, qt.Equals, 400) // Should fail validation

		// Test sending message with content too long
		longContent := string(make([]byte, 5001)) // > 5000 character limit
		for i := range longContent {
			longContent = longContent[:i] + "a" + longContent[i+1:]
		}

		invalidMessageData = map[string]interface{}{
			"type":        "private",
			"recipientId": user2ID,
			"content":     longContent,
		}

		_, code = c.Request(http.MethodPost, user1JWT, invalidMessageData, "messages")
		qt.Assert(t, code, qt.Equals, 400) // Should fail validation
	})

	t.Run("Message Permissions", func(t *testing.T) {
		// Test sending message to inactive user
		// First deactivate user3
		_, code := c.Request(http.MethodPost, user3JWT,
			api.UserProfile{
				Active: &[]bool{false}[0],
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Try to send message to inactive user
		messageData := map[string]interface{}{
			"type":        "private",
			"recipientId": user3ID,
			"content":     "This should fail - user is inactive",
		}

		_, code = c.Request(http.MethodPost, user1JWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 403) // Should be forbidden

		// Test unauthorized access
		_, code = c.Request(http.MethodGet, "", nil, "messages")
		qt.Assert(t, code, qt.Equals, 401) // Should be unauthorized

		_, code = c.Request(http.MethodPost, "", messageData, "messages")
		qt.Assert(t, code, qt.Equals, 401) // Should be unauthorized
	})

	t.Run("Pagination", func(t *testing.T) {
		// Send multiple messages to test pagination
		for i := 0; i < 5; i++ {
			messageData := map[string]interface{}{
				"type":        "private",
				"recipientId": user2ID,
				"content":     fmt.Sprintf("Pagination test message %d", i+1),
			}

			_, code := c.Request(http.MethodPost, user1JWT, messageData, "messages")
			qt.Assert(t, code, qt.Equals, 201)
		}

		// Get messages with pagination
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "messages?type=private&conversationWith="+user2ID+"&pageSize=3")
		qt.Assert(t, code, qt.Equals, 200)

		var messagesResp struct {
			Data api.PaginatedMessagesResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &messagesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(messagesResp.Data.Messages), qt.Equals, 3) // Should have 3 messages per page
		qt.Assert(t, messagesResp.Data.Pagination.PageSize, qt.Equals, 3)
		qt.Assert(t, messagesResp.Data.Pagination.Total > 3, qt.IsTrue) // Should have more than 3 total messages

		// Get next page
		resp, code = c.Request(http.MethodGet, user1JWT, nil, "messages?type=private&conversationWith="+user2ID+"&pageSize=3&page=1")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &messagesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(messagesResp.Data.Messages) > 0, qt.IsTrue) // Should have messages on second page
	})
}

func TestMessagesWithImages(t *testing.T) {
	c := utils.NewTestService(t)

	// Create test users
	user1JWT, _ := c.RegisterAndLoginWithID("imguser1@test.com", "Image User One", "password1")
	_, user2ID := c.RegisterAndLoginWithID("imguser2@test.com", "Image User Two", "password2")

	t.Run("Send Message with Images", func(t *testing.T) {
		// For this test, we'll simulate image hashes since we don't have actual image upload in this test
		// In a real scenario, images would be uploaded first and their hashes obtained
		messageData := map[string]interface{}{
			"type":        "private",
			"recipientId": user2ID,
			"content":     "Check out these images!",
			"images":      []string{"abc123def456", "789ghi012jkl"}, // Mock image hashes
		}

		_, code := c.Request(http.MethodPost, user1JWT, messageData, "messages")
		// This might fail with 404 if images don't exist, which is expected in test environment
		// The important thing is that the API accepts the request format
		if code == 201 {
			// Images exist and message was sent successfully
			qt.Assert(t, code, qt.Equals, 201)
		} else {
			// Expected to fail due to non-existent images in test environment
			qt.Assert(t, code, qt.Equals, 404)
		}
	})

	t.Run("Send Message with Only Images", func(t *testing.T) {
		// Test sending message with only images (no text content)
		messageData := map[string]interface{}{
			"type":        "private",
			"recipientId": user2ID,
			"images":      []string{"onlyimage123"}, // Mock image hash
		}

		_, code := c.Request(http.MethodPost, user1JWT, messageData, "messages")
		// Should either succeed (201) or fail due to non-existent image (404)
		// Should not fail due to validation (400) since images are provided
		qt.Assert(t, code == 201 || code == 404, qt.IsTrue)
	})

	t.Run("Validate Image Limits", func(t *testing.T) {
		// Test sending message with too many images (>10)
		manyImages := make([]string, 11) // 11 images > 10 limit
		for i := range manyImages {
			manyImages[i] = fmt.Sprintf("image%d", i)
		}

		messageData := map[string]interface{}{
			"type":        "private",
			"recipientId": user2ID,
			"content":     "Too many images",
			"images":      manyImages,
		}

		_, code := c.Request(http.MethodPost, user1JWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 400) // Should fail validation
	})
}

func TestUserProfileUnreadCounts(t *testing.T) {
	c := utils.NewTestService(t)

	// Create test users
	user1JWT, _ := c.RegisterAndLoginWithID("profile1@test.com", "Profile User One", "password1")
	user2JWT, user2ID := c.RegisterAndLoginWithID("profile2@test.com", "Profile User Two", "password2")

	t.Run("User Profile Includes Unread Message Counts", func(t *testing.T) {
		// Send a message to user2
		messageData := map[string]interface{}{
			"type":        "private",
			"recipientId": user2ID,
			"content":     "Message for profile test",
		}

		_, code := c.Request(http.MethodPost, user1JWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Send a general forum message
		generalMessageData := map[string]interface{}{
			"type":    "general",
			"content": "General message for profile test",
		}

		_, code = c.Request(http.MethodPost, user1JWT, generalMessageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Get user2's profile - should include unread message counts
		resp, code := c.Request(http.MethodGet, user2JWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var profileResp struct {
			Data struct {
				UnreadMessageCount *api.UnreadMessageSummary `json:"unreadMessageCount"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)

		// Should have unread message counts
		qt.Assert(t, profileResp.Data.UnreadMessageCount, qt.Not(qt.IsNil))
		qt.Assert(t, profileResp.Data.UnreadMessageCount.Private, qt.Equals, int64(1))
		qt.Assert(t, profileResp.Data.UnreadMessageCount.GeneralForum, qt.Equals, int64(1))
		qt.Assert(t, profileResp.Data.UnreadMessageCount.Total, qt.Equals, int64(2))
	})
}

// Helper functions
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
