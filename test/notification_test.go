package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestNotificationPreferences(t *testing.T) {
	c := utils.NewTestService(t)

	// Create a test user
	userJWT := c.RegisterAndLogin("testuser@test.com", "testuser", "testpass")

	t.Run("Get Default Notification Preferences", func(t *testing.T) {
		// Get notification preferences for a new user
		resp, code := c.Request(http.MethodGet, userJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var userProfile struct {
			Data api.UserProfile `json:"data"`
		}
		err := json.Unmarshal(resp, &userProfile)
		qt.Assert(t, err, qt.IsNil)

		// Verify all default notification types are present and enabled
		expectedDefaults := db.GetDefaultNotificationPreferences()
		qt.Assert(t, len(userProfile.Data.NotificationPreferences), qt.Equals, len(expectedDefaults))

		for key, expectedValue := range expectedDefaults {
			actualValue, exists := userProfile.Data.NotificationPreferences[key]
			qt.Assert(t, exists, qt.IsTrue, qt.Commentf("Notification type %s should exist", key))
			qt.Assert(
				t,
				actualValue,
				qt.Equals,
				expectedValue,
				qt.Commentf("Notification type %s should have default value %v", key, expectedValue),
			)
		}
	})

	t.Run("Update Notification Preferences", func(t *testing.T) {
		// Update some notification preferences
		updatePrefs := api.NotificationPreferences{
			"incoming_requests": false,
			"booking_accepted":  true,
		}

		resp, code := c.Request(http.MethodPost, userJWT, updatePrefs, "profile", "notifications")
		qt.Assert(t, code, qt.Equals, 200)

		var updateResp struct {
			Data api.NotificationPreferences `json:"data"`
		}
		err := json.Unmarshal(resp, &updateResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify the updated preferences
		qt.Assert(t, updateResp.Data["incoming_requests"], qt.Equals, false)
		qt.Assert(t, updateResp.Data["booking_accepted"], qt.Equals, true)
	})

	t.Run("Get Updated Notification Preferences", func(t *testing.T) {
		// Get notification preferences again to verify persistence
		resp, code := c.Request(http.MethodGet, userJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var userProfile struct {
			Data api.UserProfile `json:"data"`
		}
		err := json.Unmarshal(resp, &userProfile)
		qt.Assert(t, err, qt.IsNil)

		// Verify the previously updated preferences are still correct
		qt.Assert(t, userProfile.Data.NotificationPreferences["incoming_requests"], qt.Equals, false)
		qt.Assert(t, userProfile.Data.NotificationPreferences["booking_accepted"], qt.Equals, true)
	})

	t.Run("Update with Invalid Notification Type", func(t *testing.T) {
		// Try to update with an invalid notification type
		invalidPrefs := api.NotificationPreferences{
			"invalid_notification_type": true,
			"incoming_requests":         false,
		}

		resp, code := c.Request(http.MethodPost, userJWT, invalidPrefs, "profile", "notifications")
		qt.Assert(t, code, qt.Equals, 400)

		var errorResp struct {
			Header struct {
				Success   bool   `json:"success"`
				Message   string `json:"message"`
				ErrorCode int    `json:"errorCode"`
			} `json:"header"`
		}
		err := json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "unknown notification type")
	})

	t.Run("Partial Update Notification Preferences", func(t *testing.T) {
		// Update only some notification preferences
		partialPrefs := api.NotificationPreferences{
			"incoming_requests": false,
		}

		resp, code := c.Request(http.MethodPost, userJWT, partialPrefs, "profile", "notifications")
		qt.Assert(t, code, qt.Equals, 200)

		var updateResp struct {
			Data api.NotificationPreferences `json:"data"`
		}
		err := json.Unmarshal(resp, &updateResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify that previously set preferences are maintained
		qt.Assert(t, updateResp.Data["incoming_requests"], qt.Equals, false)

		// Verify that other preferences remain at their default values
		qt.Assert(t, updateResp.Data["booking_accepted"], qt.Equals, true)
	})

	t.Run("Unauthorized Access", func(t *testing.T) {
		// Try to access notification preferences without authentication
		_, code := c.Request(http.MethodGet, "", nil, "profile")
		qt.Assert(t, code, qt.Equals, 401)

		// Try to update notification preferences without authentication
		prefs := api.NotificationPreferences{
			"incoming_requests": false,
		}
		_, code = c.Request(http.MethodPost, "", prefs, "profile", "notifications")
		qt.Assert(t, code, qt.Equals, 401)
	})

	t.Run("User Profile Includes Notification Preferences", func(t *testing.T) {
		// Get user profile and verify it includes notification preferences
		resp, code := c.Request(http.MethodGet, userJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify notification preferences are included in the profile
		qt.Assert(t, profileResp.Data.NotificationPreferences, qt.Not(qt.IsNil))
		qt.Assert(t, len(profileResp.Data.NotificationPreferences), qt.Not(qt.Equals), 0)

		// Verify some of the previously set preferences
		qt.Assert(t, profileResp.Data.NotificationPreferences["incoming_requests"], qt.Equals, false)
	})

	t.Run("New User Has Default Notification Preferences", func(t *testing.T) {
		// Register a new user and verify they get default notification preferences
		newUserJWT := c.RegisterAndLogin("newuser@test.com", "newuser", "newuser@test.com")

		// Get the new user's profile
		resp, code := c.Request(http.MethodGet, newUserJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify the new user has default notification preferences
		expectedDefaults := db.GetDefaultNotificationPreferences()
		qt.Assert(t, len(profileResp.Data.NotificationPreferences), qt.Equals, len(expectedDefaults))

		for key, expectedValue := range expectedDefaults {
			actualValue, exists := profileResp.Data.NotificationPreferences[key]
			qt.Assert(t, exists, qt.IsTrue, qt.Commentf("New user should have notification type %s", key))
			qt.Assert(
				t,
				actualValue,
				qt.Equals,
				expectedValue,
				qt.Commentf("New user should have default value %v for %s", expectedValue, key),
			)
		}
	})

	t.Run("Empty Update Request", func(t *testing.T) {
		// Send an empty update request
		emptyPrefs := api.NotificationPreferences{}

		resp, code := c.Request(http.MethodPost, userJWT, emptyPrefs, "profile", "notifications")
		qt.Assert(t, code, qt.Equals, 200)

		var updateResp struct {
			Data api.NotificationPreferences `json:"data"`
		}
		err := json.Unmarshal(resp, &updateResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify that existing preferences are maintained
		qt.Assert(t, updateResp.Data["incoming_requests"], qt.Equals, false)
	})

	t.Run("Public User Endpoints Do Not Include Notification Preferences", func(t *testing.T) {
		// Create another user to test public access
		otherUserJWT, otherUserID := c.RegisterAndLoginWithID("otheruser@test.com", "otheruser", "otherpass")

		// Test GET /users/{id} - should NOT include notification preferences
		resp, code := c.Request(http.MethodGet, userJWT, nil, "users", otherUserID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify notification preferences are NOT included in public user view
		qt.Assert(t, userResp.Data.NotificationPreferences, qt.IsNil)

		// Test GET /users (list users) - should NOT include notification preferences
		resp, code = c.Request(http.MethodGet, userJWT, nil, "users")
		qt.Assert(t, code, qt.Equals, 200)

		var usersResp struct {
			Data struct {
				Users []*api.User `json:"users"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify notification preferences are NOT included in any user in the list
		for _, user := range usersResp.Data.Users {
			qt.Assert(
				t,
				user.NotificationPreferences,
				qt.IsNil,
				qt.Commentf("User %s should not have notification preferences in public list", user.ID),
			)
		}

		// But verify that the user's own profile DOES include notification preferences
		resp, code = c.Request(http.MethodGet, otherUserJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var ownProfileResp struct {
			Data *api.User `json:"data"`
		}
		err = json.Unmarshal(resp, &ownProfileResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify notification preferences ARE included in own profile
		qt.Assert(t, ownProfileResp.Data.NotificationPreferences, qt.Not(qt.IsNil))
		qt.Assert(t, len(ownProfileResp.Data.NotificationPreferences), qt.Not(qt.Equals), 0)
	})
}

func TestNotificationPreferencesValidation(t *testing.T) {
	c := utils.NewTestService(t)
	userJWT := c.RegisterAndLogin("validationuser@test.com", "validationuser", "validationpass")

	t.Run("Invalid JSON Request", func(t *testing.T) {
		// Test with malformed notification preferences (non-boolean values)
		invalidPrefs := map[string]interface{}{
			"incoming_requests": "not_a_boolean",
		}

		_, code := c.Request(http.MethodPost, userJWT, invalidPrefs, "profile", "notifications")
		qt.Assert(t, code, qt.Equals, 400)
	})

	t.Run("All Valid Notification Types", func(t *testing.T) {
		// Test updating all valid notification types
		allValidPrefs := db.GetDefaultNotificationPreferences()

		// Flip all values to opposite of defaults
		updatePrefs := make(api.NotificationPreferences)
		for key, value := range allValidPrefs {
			updatePrefs[key] = !value
		}

		resp, code := c.Request(http.MethodPost, userJWT, updatePrefs, "profile", "notifications")
		qt.Assert(t, code, qt.Equals, 200)

		var updateResp struct {
			Data api.NotificationPreferences `json:"data"`
		}
		err := json.Unmarshal(resp, &updateResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify all preferences were updated correctly
		for key, expectedValue := range updatePrefs {
			actualValue, exists := updateResp.Data[key]
			qt.Assert(t, exists, qt.IsTrue, qt.Commentf("Notification type %s should exist", key))
			qt.Assert(t, actualValue, qt.Equals, expectedValue, qt.Commentf("Notification type %s should be %v", key, expectedValue))
		}
	})
}

func TestNewIncomingRequestNotification(t *testing.T) {
	c := utils.NewTestService(t)

	t.Run("Notification Enabled - Email Should Be Sent", func(t *testing.T) {
		// Create tool owner and renter
		ownerJWT := c.RegisterAndLogin("owner-notifications@test.com", "owner", "ownerpass")
		renterJWT := c.RegisterAndLogin("renter-notifications@test.com", "renter", "renterpass")

		// Verify owner has default notification preferences (should be enabled by default)
		resp, code := c.Request(http.MethodGet, ownerJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var userProfile struct {
			Data api.UserProfile `json:"data"`
		}
		err := json.Unmarshal(resp, &userProfile)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(
			t,
			userProfile.Data.NotificationPreferences["incoming_requests"],
			qt.Equals,
			true,
			qt.Commentf("incoming_requests should be enabled by default"),
		)

		// Owner creates a tool
		toolID := c.CreateTool(ownerJWT, "Test Tool for Notifications")

		// Renter creates a booking request
		_, code = c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: time.Now().Add(24 * time.Hour).Unix(),
				EndDate:   time.Now().Add(48 * time.Hour).Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking for notification",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Check that email notification was sent to tool owner
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mailBody, err := c.MailService().FindEmail(ctx, "owner-notifications@test.com")
		qt.Assert(t, err, qt.IsNil)

		// Verify mail contains expected information
		qt.Assert(t, mailBody, qt.Contains, "test@example.com")              // Contact
		qt.Assert(t, mailBody, qt.Contains, "Test booking for notification") // Comments
		qt.Assert(t, mailBody, qt.Contains, "renter")                        // From UserName
		qt.Assert(t, mailBody, qt.Contains, "Test Tool for Notifications")   // Tool title
		qt.Assert(t, mailBody, qt.Contains, mailtemplates.AppName)           // App name
	})

	t.Run("Notification Disabled - No Email Should Be Sent", func(t *testing.T) {
		// Create tool owner and renter
		ownerJWT := c.RegisterAndLogin("owner-no-notifications@test.com", "owner2", "ownerpass")
		renterJWT := c.RegisterAndLogin("renter-no-notifications@test.com", "renter2", "renterpass")

		c.ReadRegistrationMail("owner-no-notifications@test.com", t)

		// Disable booking request notifications for the owner
		updatePrefs := api.NotificationPreferences{
			"incoming_requests": false,
		}
		_, code := c.Request(http.MethodPost, ownerJWT, updatePrefs, "profile", "notifications")
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the preference was updated
		resp, code := c.Request(http.MethodGet, ownerJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var userProfile struct {
			Data api.UserProfile `json:"data"`
		}
		err := json.Unmarshal(resp, &userProfile)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(
			t,
			userProfile.Data.NotificationPreferences["incoming_requests"],
			qt.Equals,
			false,
			qt.Commentf("incoming_requests should be disabled"),
		)

		// Owner creates a tool
		toolID := c.CreateTool(ownerJWT, "Test Tool No Notifications")

		// Renter creates a booking request
		_, code = c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: time.Now().Add(24 * time.Hour).Unix(),
				EndDate:   time.Now().Add(48 * time.Hour).Unix(),
				Contact:   "test2@example.com",
				Comments:  "Test booking no notification",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify no email was sent to tool owner
		// We need to wait a bit to ensure that if an email was going to be sent, it would have been sent by now
		time.Sleep(1 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err = c.MailService().FindEmail(ctx, "owner-no-notifications@test.com")
		qt.Assert(t, err, qt.Not(qt.IsNil), qt.Commentf("No email should be sent when notifications are disabled"))
	})

	t.Run("Email Content Verification", func(t *testing.T) {
		// Create tool owner and renter
		ownerJWT := c.RegisterAndLogin("owner-content@test.com", "contentowner", "ownerpass")
		renterJWT := c.RegisterAndLogin("renter-content@test.com", "contentrenter", "renterpass")

		// Owner creates a tool
		toolID := c.CreateTool(ownerJWT, "Content Verification Tool")

		// Renter creates a booking request with specific content
		_, code := c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: time.Now().Add(24 * time.Hour).Unix(),
				EndDate:   time.Now().Add(48 * time.Hour).Unix(),
				Contact:   "specific-contact@example.com",
				Comments:  "Specific test comment for verification",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Check email content
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mailBody, err := c.MailService().FindEmail(ctx, "owner-content@test.com")
		qt.Assert(t, err, qt.IsNil)

		// Verify all required mail content is present
		qt.Assert(t, mailBody, qt.Contains, "specific-contact@example.com")           // Way of contact
		qt.Assert(t, mailBody, qt.Contains, "Specific test comment for verification") // Comments
		qt.Assert(t, mailBody, qt.Contains, "contentrenter")                          // From UserName
		qt.Assert(t, mailBody, qt.Contains, "Content Verification Tool")              // Tool title
		qt.Assert(t, mailBody, qt.Contains, mailtemplates.AppName)                    // App name
		qt.Assert(t, mailBody, qt.Contains, mailtemplates.IncomingUrl)                // Button URL
	})
}
