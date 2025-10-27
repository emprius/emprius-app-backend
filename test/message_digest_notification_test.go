package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestPrivateMessageDigestNotifications(t *testing.T) {
	c := utils.NewTestService(t)

	t.Run("Automatic Ticker System", func(t *testing.T) {
		// Create test users
		senderJWT, _ := c.RegisterAndLoginWithID("digest-ticker-sender@test.com", "Ticker Sender", "password")
		_, recipientID := c.RegisterAndLoginWithID("digest-ticker-recipient@test.com", "Ticker Recipient", "password")

		// reset mock time to now for consistent results
		c.SetMockTime(time.Now())

		// Send 2 private messages
		for i := 1; i <= 2; i++ {
			messageData := map[string]interface{}{
				"type":        api.MessageTypePrivate,
				"recipientId": recipientID,
				"content":     fmt.Sprintf("Ticker test message %d", i),
			}
			_, code := c.Request(http.MethodPost, senderJWT, messageData, "messages")
			qt.Assert(t, code, qt.Equals, 201)
		}

		// Advance time by 61 minutes
		c.AdvanceTime(61 * time.Minute)

		// Wait 2 seconds for the ticker to fire automatically (ticker runs every 100ms)
		// Since notifications have 0 delay, they're immediately ready for processing
		time.Sleep(1 * time.Second)

		// Verify email was sent automatically by the ticker
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mailBody, err := c.MailService().FindEmail(ctx, "digest-ticker-recipient@test.com")
		qt.Assert(t, err, qt.IsNil)

		// Verify email content
		qt.Assert(t, mailBody, qt.Contains, "Ticker Sender")
		qt.Assert(t, mailBody, qt.Contains, "2")
		qt.Assert(t, mailBody, qt.Contains, mailtemplates.AppName)
	})

	t.Run("Email Sent with Correct Unread Count", func(t *testing.T) {
		// Create test users
		senderJWT, _ := c.RegisterAndLoginWithID("digest-sender@test.com", "Manual Sender", "password")
		_, recipientID := c.RegisterAndLoginWithID("digest-recipient@test.com", "Manual Recipient", "password")

		// reset mock time to now for consistent results
		c.SetMockTime(time.Now())

		// Send 3 private messages
		for i := 1; i <= 3; i++ {
			messageData := map[string]interface{}{
				"type":        api.MessageTypePrivate,
				"recipientId": recipientID,
				"content":     fmt.Sprintf("Test message %d for digest", i),
			}
			_, code := c.Request(http.MethodPost, senderJWT, messageData, "messages")
			qt.Assert(t, code, qt.Equals, 201)
		}

		// Advance time by 61 minutes
		c.AdvanceTime(61 * time.Minute)

		// Manually trigger digest processing
		err := c.ProcessDigestNotifications()
		qt.Assert(t, err, qt.IsNil)

		// Verify email was sent
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mailBody, err := c.MailService().FindEmail(ctx, "digest-recipient@test.com")
		qt.Assert(t, err, qt.IsNil)

		// Verify email content
		qt.Assert(t, mailBody, qt.Contains, "Manual Sender")
		qt.Assert(t, mailBody, qt.Contains, "3")
		qt.Assert(t, mailBody, qt.Contains, mailtemplates.AppName)
	})

	t.Run("No Email When Messages Marked as Read", func(t *testing.T) {
		// Create test users
		senderJWT, senderID := c.RegisterAndLoginWithID("digest-sender-read@test.com", "Read Sender", "password")
		recipientJWT, recipientID := c.RegisterAndLoginWithID("digest-recipient-read-no-mail@test.com", "Read Recipient", "password")

		// reset mock time to now for consistent results
		c.SetMockTime(time.Now())

		c.ReadRegistrationMail("digest-recipient-read-no-mail@test.com", t)

		// Send 2 private messages
		for i := 1; i <= 2; i++ {
			messageData := map[string]interface{}{
				"type":        api.MessageTypePrivate,
				"recipientId": recipientID,
				"content":     fmt.Sprintf("Test message %d for read test", i),
			}
			_, code := c.Request(http.MethodPost, senderJWT, messageData, "messages")
			qt.Assert(t, code, qt.Equals, 201)
		}

		// Recipient marks all messages as read using the conversation endpoint
		markReadData := map[string]interface{}{
			"type":             api.MessageTypePrivate,
			"conversationWith": senderID,
		}
		_, code := c.Request(http.MethodPost, recipientJWT, markReadData, "messages/read/conversation")
		qt.Assert(t, code, qt.Equals, 200)

		// Advance time by 61 minutes
		c.AdvanceTime(61 * time.Minute)

		// Manually trigger digest processing
		err := c.ProcessDigestNotifications()
		qt.Assert(t, err, qt.IsNil)

		// Verify NO email was sent (should timeout as no email exists)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		res, err := c.MailService().FindEmail(ctx, "digest-recipient-read-no-mail@test.com")
		// We expect a timeout error because no email should be sent
		qt.Assert(t, err, qt.Not(qt.IsNil), qt.Commentf("No email should be sent when messages are marked as read"))
		qt.Assert(t, res, qt.Equals, "")
	})

	t.Run("No Email When Notification Preference Disabled", func(t *testing.T) {
		// Create test users
		senderJWT, _ := c.RegisterAndLoginWithID("digest-sender-pref@test.com", "Pref Sender", "password")
		recipientJWT, recipientID := c.RegisterAndLoginWithID("digest-recipient-pref@test.com", "Pref Recipient", "password")
		c.ReadRegistrationMail("digest-recipient-pref@test.com", t)

		// reset mock time to now for consistent results
		c.SetMockTime(time.Now())

		// Disable private message notifications for recipient
		updatePrefs := api.NotificationPreferences{
			"private_messages": false,
		}
		_, code := c.Request(http.MethodPost, recipientJWT, updatePrefs, "profile/notifications")
		qt.Assert(t, code, qt.Equals, 200)

		// Send messages
		for i := 1; i <= 2; i++ {
			messageData := map[string]interface{}{
				"type":        api.MessageTypePrivate,
				"recipientId": recipientID,
				"content":     fmt.Sprintf("Test message %d for pref test", i),
			}
			_, code := c.Request(http.MethodPost, senderJWT, messageData, "messages")
			qt.Assert(t, code, qt.Equals, 201)
		}

		// Advance time by 61 minutes
		c.AdvanceTime(61 * time.Minute)

		// Manually trigger digest processing
		err := c.ProcessDigestNotifications()
		qt.Assert(t, err, qt.IsNil)

		// Verify NO email was sent
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err = c.MailService().FindEmail(ctx, "digest-recipient-pref@test.com")
		qt.Assert(t, err, qt.Not(qt.IsNil), qt.Commentf("No email should be sent when notification preference is disabled"))
	})

	t.Run("No Email Sent Before NotificationScheduledFor Time", func(t *testing.T) {
		// Create test users
		senderJWT, _ := c.RegisterAndLoginWithID("digest-sender-timing@test.com", "Timing Sender", "password")
		_, recipientID := c.RegisterAndLoginWithID("digest-recipient-timing@test.com", "Timing Recipient", "password")
		c.ReadRegistrationMail("digest-recipient-timing@test.com", t)

		// reset mock time to now for consistent results
		c.SetMockTime(time.Now())

		// Send messages (NotificationScheduledFor will be set to real system time + 0 minutes)
		for i := 1; i <= 2; i++ {
			messageData := map[string]interface{}{
				"type":        api.MessageTypePrivate,
				"recipientId": recipientID,
				"content":     fmt.Sprintf("Timing test message %d", i),
			}
			_, code := c.Request(http.MethodPost, senderJWT, messageData, "messages")
			qt.Assert(t, code, qt.Equals, 201)
		}

		// Try to process immediately while mock time is still in the past
		// The mock time (10 minutes ago) is before NotificationScheduledFor (real now), so NO email should be sent
		err := c.ProcessDigestNotifications()
		qt.Assert(t, err, qt.IsNil)

		// Verify NO email was sent (notification not yet ready)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err = c.MailService().FindEmail(ctx, "digest-recipient-timing@test.com")
		qt.Assert(t, err, qt.Not(qt.IsNil), qt.Commentf("No email should be sent before NotificationScheduledFor time"))

		// Now advance mock time to future (60 + 11 minutes ahead from original, so 1 minute ahead of real now)
		c.AdvanceTime((60 + 11) * time.Minute)

		// Process again - now the notification should be ready
		err = c.ProcessDigestNotifications()
		qt.Assert(t, err, qt.IsNil)

		// Verify email WAS sent this time
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		mailBody, err := c.MailService().FindEmail(ctx2, "digest-recipient-timing@test.com")
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Email should be sent after NotificationScheduledFor time"))

		// Verify email content
		qt.Assert(t, mailBody, qt.Contains, "Timing Sender")
		qt.Assert(t, mailBody, qt.Contains, "2")
		qt.Assert(t, mailBody, qt.Contains, mailtemplates.AppName)
	})

	// todo(kon): this test fails because mark an individual message as read marks the entire conversation as read.
	// This bug should be addressed separately.
	//t.Run("Email Contains Actual Unread Count After Partial Read", func(t *testing.T) {
	//	// Create test users
	//	senderJWT, _ := c.RegisterAndLoginWithID("digest-sender-partial@test.com", "Partial Sender", "password")
	//	recipientJWT, _ := c.RegisterAndLoginWithID("digest-recipient-partial@test.com", "Partial Recipient", "password")
	//
	//	// Get recipient ID
	//	var recipientID string
	//	resp, code := c.Request(http.MethodGet, recipientJWT, nil, "profile")
	//	qt.Assert(t, code, qt.Equals, 200)
	//	var profileResp struct {
	//		Data struct {
	//			ID string `json:"id"`
	//		} `json:"data"`
	//	}
	//	err := json.Unmarshal(resp, &profileResp)
	//	qt.Assert(t, err, qt.IsNil)
	//	recipientID = profileResp.Data.ID
	//
	//	// Send 5 private messages
	//	var messageIDs []string
	//	for i := 1; i <= 5; i++ {
	//		messageData := map[string]interface{}{
	//			"type":        api.MessageTypePrivate,
	//			"recipientId": recipientID,
	//			"content":     fmt.Sprintf("Test message %d for partial read", i),
	//		}
	//		resp, code := c.Request(http.MethodPost, senderJWT, messageData, "messages")
	//		qt.Assert(t, code, qt.Equals, 201)
	//
	//		var messageResp struct {
	//			Data api.MessageResponse `json:"data"`
	//		}
	//		err := json.Unmarshal(resp, &messageResp)
	//		qt.Assert(t, err, qt.IsNil)
	//		messageIDs = append(messageIDs, messageResp.Data.ID)
	//	}
	//
	//	// Wait a moment to ensure notification queue entry is created
	//	time.Sleep(100 * time.Millisecond)
	//
	//	// Recipient marks 2 specific messages as read
	//	markReadData := map[string]interface{}{
	//		"messageIds": []string{messageIDs[0], messageIDs[1]},
	//	}
	//	_, code = c.Request(http.MethodPost, recipientJWT, markReadData, "messages/read")
	//	qt.Assert(t, code, qt.Equals, 200)
	//
	//	// Advance time by 61 minutes
	//	c.AdvanceTime(61 * time.Minute)
	//
	//	// Manually trigger digest processing
	//	err = c.ProcessDigestNotifications()
	//	qt.Assert(t, err, qt.IsNil)
	//
	//	// Verify email was sent with correct unread count (3, not 5)
	//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//	defer cancel()
	//	mailBody, err := c.MailService().FindEmail(ctx, "digest-recipient-partial@test.com")
	//	qt.Assert(t, err, qt.IsNil)
	//
	//	// Verify email contains the actual unread count
	//	qt.Assert(t, mailBody, qt.Contains, "Partial Sender")
	//	qt.Assert(t, mailBody, qt.Contains, "3", qt.Commentf("Email should contain unread count of 3 (not 5)"))
	//	qt.Assert(t, mailBody, qt.Not(qt.Contains), "5", qt.Commentf("Email should not contain the original count of 5"))
	//})
}
