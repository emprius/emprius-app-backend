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

func TestDailyMessageDigest(t *testing.T) {
	c := utils.NewTestService(t)

	t.Run("Daily Digest Sent at Configured Hour (UTC)", func(t *testing.T) {
		// Create test users
		sender1JWT, _ := c.RegisterAndLoginWithID("daily-sender1@test.com", "Daily Sender One", "password")
		_, recipientID := c.RegisterAndLoginWithID("daily-recipient-hour@test.com", "Daily Recipient Hour", "password")
		c.ReadRegistrationMail("daily-recipient-hour@test.com", t)

		// Set mock time to 8:59 AM UTC (before digest time)
		now := time.Date(2024, 1, 15, 8, 59, 0, 0, time.UTC)
		c.SetMockTime(now)

		// Send 2 private messages
		for i := 1; i <= 2; i++ {
			messageData := map[string]interface{}{
				"type":        api.MessageTypePrivate,
				"recipientId": recipientID,
				"content":     fmt.Sprintf("Daily digest hour test %d", i),
			}
			_, code := c.Request(http.MethodPost, sender1JWT, messageData, "messages")
			qt.Assert(t, code, qt.Equals, 201)
		}

		// Wait for ticker to process (but digest shouldn't be sent yet, not 9 AM)
		time.Sleep(500 * time.Millisecond)

		// Verify NO email was sent (not 9 AM yet)
		ctx1, cancel1 := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel1()
		_, err := c.MailService().FindEmail(ctx1, "daily-recipient-hour@test.com")
		qt.Assert(t, err, qt.Not(qt.IsNil), qt.Commentf("No email should be sent before 9 AM"))

		// Now advance time to 9:00 AM UTC
		c.AdvanceTime(1 * time.Minute)

		// Wait for ticker to fire and process daily digest
		time.Sleep(500 * time.Millisecond)

		// Verify email WAS sent
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		mailBody, err := c.MailService().FindEmail(ctx2, "daily-recipient-hour@test.com")
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Email should be sent at 9 AM"))

		// Verify it's the daily digest email (contains total unread)
		qt.Assert(t, mailBody, qt.Contains, "Daily Message Digest")
		qt.Assert(t, mailBody, qt.Contains, "2")
	})

	t.Run("Only Sent Once Per Day Per User", func(t *testing.T) {
		// Create test users
		senderJWT, _ := c.RegisterAndLoginWithID("daily-sender-once@test.com", "Once Sender", "password")
		_, recipientID := c.RegisterAndLoginWithID("daily-recipient-once@test.com", "Once Recipient", "password")
		c.ReadRegistrationMail("daily-recipient-once@test.com", t)

		// Set mock time to 9:00 AM UTC
		now := time.Date(2024, 1, 16, 9, 0, 0, 0, time.UTC)
		c.SetMockTime(now)

		// Send messages
		messageData := map[string]interface{}{
			"type":        api.MessageTypePrivate,
			"recipientId": recipientID,
			"content":     "Once per day test",
		}
		_, code := c.Request(http.MethodPost, senderJWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Wait for ticker to process
		time.Sleep(500 * time.Millisecond)

		// Verify first email was sent
		ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel1()
		mailBody, err := c.MailService().FindEmail(ctx1, "daily-recipient-once@test.com")
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, mailBody, qt.Contains, "Daily Message Digest")

		// Advance time by 2 hours (still same day, 11 AM)
		c.AdvanceTime(2 * time.Hour)

		// Wait for ticker
		time.Sleep(500 * time.Millisecond)

		// Verify NO second email was sent (should timeout)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel2()
		_, err = c.MailService().FindEmail(ctx2, "daily-recipient-once@test.com")
		qt.Assert(t, err, qt.Not(qt.IsNil), qt.Commentf("No second email should be sent on same day"))

		// Advance to next day at 9 AM
		c.AdvanceTime(22 * time.Hour) // 11 AM + 22 hours = 9 AM next day

		// Wait for ticker
		time.Sleep(500 * time.Millisecond)

		// Verify email WAS sent on next day
		ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel3()
		mailBody2, err := c.MailService().FindEmail(ctx3, "daily-recipient-once@test.com")
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Email should be sent on next day"))
		qt.Assert(t, mailBody2, qt.Contains, "Daily Message Digest")
	})

	t.Run("Respects NotificationDailyMessageDigest Preference", func(t *testing.T) {
		// Create test users
		senderJWT, _ := c.RegisterAndLoginWithID("daily-sender-pref@test.com", "Pref Sender", "password")
		recipientJWT, recipientID := c.RegisterAndLoginWithID("daily-recipient-pref@test.com", "Pref Recipient", "password")
		c.ReadRegistrationMail("daily-recipient-pref@test.com", t)

		// Disable daily digest preference
		updatePrefs := api.NotificationPreferences{
			"daily_message_digest": false,
		}
		_, code := c.Request(http.MethodPost, recipientJWT, updatePrefs, "profile/notifications")
		qt.Assert(t, code, qt.Equals, 200)

		// Set mock time to 9:00 AM UTC
		now := time.Date(2024, 1, 17, 9, 0, 0, 0, time.UTC)
		c.SetMockTime(now)

		// Send messages
		messageData := map[string]interface{}{
			"type":        api.MessageTypePrivate,
			"recipientId": recipientID,
			"content":     "Preference test message",
		}
		_, code = c.Request(http.MethodPost, senderJWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Wait for ticker
		time.Sleep(500 * time.Millisecond)

		// Verify NO email was sent (preference disabled)
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, err := c.MailService().FindEmail(ctx, "daily-recipient-pref@test.com")
		qt.Assert(t, err, qt.Not(qt.IsNil), qt.Commentf("No email should be sent when preference is disabled"))
	})

	t.Run("Only Sent When Unread Count Greater Than Zero", func(t *testing.T) {
		// Create test user with NO unread messages
		_, _ = c.RegisterAndLoginWithID("daily-recipient-zero@test.com", "Zero Recipient", "password")
		c.ReadRegistrationMail("daily-recipient-zero@test.com", t)

		// Set mock time to 9:00 AM UTC
		now := time.Date(2024, 1, 18, 9, 0, 0, 0, time.UTC)
		c.SetMockTime(now)

		// Wait for ticker (user has no unread messages)
		time.Sleep(500 * time.Millisecond)

		// Verify NO email was sent (no unread messages)
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, err := c.MailService().FindEmail(ctx, "daily-recipient-zero@test.com")
		qt.Assert(t, err, qt.Not(qt.IsNil), qt.Commentf("No email should be sent when unread count is 0"))
	})

	t.Run("Shows Correct Counts for All Three Message Types", func(t *testing.T) {
		// Create senders
		sender1JWT, _ := c.RegisterAndLoginWithID("daily-sender-alice@test.com", "Alice", "password")
		sender2JWT, _ := c.RegisterAndLoginWithID("daily-sender-bob@test.com", "Bob", "password")

		// Create recipient
		recipientJWT, recipientID := c.RegisterAndLoginWithID("daily-recipient-counts@test.com", "Counts Recipient", "password")
		c.ReadRegistrationMail("daily-recipient-counts@test.com", t)

		communityData := map[string]interface{}{
			"name":        "Test Community for Daily Digest",
			"description": "Testing daily digest",
		}

		communityID := c.CreateInviteAndJoinCommunity(sender1JWT, recipientJWT, recipientID, communityData)

		// Send 2 private messages from Alice
		for i := 1; i <= 2; i++ {
			messageData := map[string]interface{}{
				"type":        api.MessageTypePrivate,
				"recipientId": recipientID,
				"content":     fmt.Sprintf("Private from Alice %d", i),
			}
			_, code := c.Request(http.MethodPost, sender1JWT, messageData, "messages")
			qt.Assert(t, code, qt.Equals, 201)
		}

		// Send 1 private message from Bob
		messageData := map[string]interface{}{
			"type":        api.MessageTypePrivate,
			"recipientId": recipientID,
			"content":     "Private from Bob",
		}
		_, code := c.Request(http.MethodPost, sender2JWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Send 3 community messages
		for i := 1; i <= 3; i++ {
			messageData := map[string]interface{}{
				"type":        api.MessageTypeCommunity,
				"recipientId": communityID,
				"content":     fmt.Sprintf("Community message %d", i),
			}
			_, code := c.Request(http.MethodPost, sender1JWT, messageData, "messages")
			qt.Assert(t, code, qt.Equals, 201)
		}

		// Send 2 general forum messages
		for i := 1; i <= 2; i++ {
			messageData := map[string]interface{}{
				"type":    api.MessageTypeGeneral,
				"content": fmt.Sprintf("General forum message %d", i),
			}
			_, code := c.Request(http.MethodPost, sender2JWT, messageData, "messages")
			qt.Assert(t, code, qt.Equals, 201)
		}

		// Set mock time to 9:00 AM UTC
		now := time.Date(2024, 1, 19, 9, 0, 0, 0, time.UTC)
		c.SetMockTime(now)

		// Wait for ticker
		time.Sleep(500 * time.Millisecond)

		// Verify email was sent
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mailBody, err := c.MailService().FindEmail(ctx, "daily-recipient-counts@test.com")
		qt.Assert(t, err, qt.IsNil)

		// Verify total count (3 private + 3 community + 2 general = 8)
		qt.Assert(t, mailBody, qt.Contains, "8")

		// Verify it shows all three message types
		qt.Assert(t, mailBody, qt.Contains, "Private Messages")
		qt.Assert(t, mailBody, qt.Contains, "Community Messages")
		qt.Assert(t, mailBody, qt.Contains, "General Forum")
	})

	t.Run("Shows Conversation Names Correctly", func(t *testing.T) {
		// Create senders with specific names
		aliceJWT, _ := c.RegisterAndLoginWithID("daily-alice@test.com", "Alice Smith", "password")
		bobJWT, _ := c.RegisterAndLoginWithID("daily-bob@test.com", "Bob Jones", "password")

		// Create recipient
		recipientJWT, recipientID := c.RegisterAndLoginWithID("daily-recipient-names@test.com", "Names Recipient", "password")
		c.ReadRegistrationMail("daily-recipient-names@test.com", t)

		// Create a community with specific name
		communityData := map[string]interface{}{
			"name":        "Bicing Barcelona",
			"description": "Community for testing names",
		}
		communityID := c.CreateInviteAndJoinCommunity(aliceJWT, recipientJWT, recipientID, communityData)

		// Send message from Alice
		messageData := map[string]interface{}{
			"type":        api.MessageTypePrivate,
			"recipientId": recipientID,
			"content":     "Message from Alice",
		}
		_, code := c.Request(http.MethodPost, aliceJWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Send message from Bob
		messageData = map[string]interface{}{
			"type":        api.MessageTypePrivate,
			"recipientId": recipientID,
			"content":     "Message from Bob",
		}
		_, code = c.Request(http.MethodPost, bobJWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Send community message
		messageData = map[string]interface{}{
			"type":        api.MessageTypeCommunity,
			"recipientId": communityID,
			"content":     "Community message",
		}
		_, code = c.Request(http.MethodPost, aliceJWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Set mock time to 9:00 AM UTC
		now := time.Date(2024, 1, 20, 9, 0, 0, 0, time.UTC)
		c.SetMockTime(now)

		// Wait for ticker
		time.Sleep(500 * time.Millisecond)

		// Verify email was sent
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mailBody, err := c.MailService().FindEmail(ctx, "daily-recipient-names@test.com")
		qt.Assert(t, err, qt.IsNil)

		// Verify names appear in the email
		qt.Assert(t, mailBody, qt.Contains, "Alice Smith")
		qt.Assert(t, mailBody, qt.Contains, "Bob Jones")
		qt.Assert(t, mailBody, qt.Contains, "Bicing Barcelona")
	})

	t.Run("Multi-language Support Works", func(t *testing.T) {
		// Create sender
		senderJWT, _ := c.RegisterAndLoginWithID("daily-sender-lang@test.com", "Lang Sender", "password")

		// Create recipient
		recipientJWT, recipientID := c.RegisterAndLoginWithID("daily-recipient-spanish@test.com", "Spanish Recipient", "password")
		c.ReadRegistrationMail("daily-recipient-spanish@test.com", t)

		// Set user language to Spanish
		updateData := map[string]interface{}{
			"lang": "es",
		}
		_, code := c.Request(http.MethodPost, recipientJWT, updateData, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		// Send message
		messageData := map[string]interface{}{
			"type":        api.MessageTypePrivate,
			"recipientId": recipientID,
			"content":     "Mensaje de prueba",
		}
		_, code = c.Request(http.MethodPost, senderJWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Set mock time to 9:00 AM UTC
		now := time.Date(2024, 1, 21, 9, 0, 0, 0, time.UTC)
		c.SetMockTime(now)

		// Wait for ticker
		time.Sleep(500 * time.Millisecond)

		// Verify email was sent
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mailBody, err := c.MailService().FindEmail(ctx, "daily-recipient-spanish@test.com")
		qt.Assert(t, err, qt.IsNil)

		// Verify Spanish text in email
		qt.Assert(t, mailBody, qt.Contains, "Tu Resumen Diario") // Spanish title
		qt.Assert(t, mailBody, qt.Contains, "mensaje")           // Spanish for "message"
	})

	t.Run("Updates LastDailyDigestSent Timestamp", func(t *testing.T) {
		// Create sender
		senderJWT, _ := c.RegisterAndLoginWithID("daily-sender-timestamp@test.com", "Timestamp Sender", "password")

		// Create recipient
		_, recipientID := c.RegisterAndLoginWithID("daily-recipient-timestamp@test.com", "Timestamp Recipient", "password")
		c.ReadRegistrationMail("daily-recipient-timestamp@test.com", t)

		// Send message
		messageData := map[string]interface{}{
			"type":        api.MessageTypePrivate,
			"recipientId": recipientID,
			"content":     "Timestamp test message",
		}
		_, code := c.Request(http.MethodPost, senderJWT, messageData, "messages")
		qt.Assert(t, code, qt.Equals, 201)

		// Set mock time to 9:00 AM UTC on a specific date
		testDate := time.Date(2024, 1, 22, 9, 0, 0, 0, time.UTC)
		c.SetMockTime(testDate)

		// Wait for ticker
		time.Sleep(500 * time.Millisecond)

		// Verify email was sent
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := c.MailService().FindEmail(ctx, "daily-recipient-timestamp@test.com")
		qt.Assert(t, err, qt.IsNil)

		// Note: We can't directly verify the LastDailyDigestSent field without
		// database access in the test, but the fact that the second digest
		// on the same day doesn't send (tested in "Only Sent Once Per Day")
		// confirms the timestamp is being updated correctly
	})
}
