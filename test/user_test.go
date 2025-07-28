package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestUser(t *testing.T) {
	c := utils.NewTestService(t)

	// Create multiple users for pagination testing
	user1JWT := c.RegisterAndLogin("user1@test.com", "user1", "user1pass")
	user2JWT := c.RegisterAndLogin("user2@test.com", "user2", "user2pass")
	c.RegisterAndLogin("user3@test.com", "user3", "user3pass")
	c.RegisterAndLogin("user4@test.com", "user4", "user4pass")
	c.RegisterAndLogin("user5@test.com", "user5", "user5pass")

	t.Run("Paginated Communities List", func(t *testing.T) {
		// Test first page
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "users")
		qt.Assert(t, code, qt.Equals, 200)
		var usersResp struct {
			Data struct {
				Users []*api.User `json:"users"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		// All users since we have less than page size
		qt.Assert(t, len(usersResp.Data.Users), qt.Equals, 5)

		// Test unauthorized access
		_, code = c.Request(http.MethodGet, "", nil, "users")
		qt.Assert(t, code, qt.Equals, 401)
	})

	t.Run("Search Communities by Partial Name", func(t *testing.T) {
		// Test search for "user" - should return all users
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "users?term=user")
		qt.Assert(t, code, qt.Equals, 200)
		var usersResp struct {
			Data struct {
				Users []*api.User `json:"users"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data.Users), qt.Equals, 5) // All users match "user"

		// Test search for "user1" - should return only user1
		resp, code = c.Request(http.MethodGet, user1JWT, nil, "users?term=user1")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data.Users), qt.Equals, 1) // Only user1 matches
		qt.Assert(t, usersResp.Data.Users[0].Name, qt.Equals, "user1")

		// Test case-insensitive search
		resp, code = c.Request(http.MethodGet, user1JWT, nil, "users?term=USER2")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data.Users), qt.Equals, 1) // Only user2 matches
		qt.Assert(t, usersResp.Data.Users[0].Name, qt.Equals, "user2")

		// Test search with no matches
		resp, code = c.Request(http.MethodGet, user1JWT, nil, "users?term=nonexistent")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data.Users), qt.Equals, 0) // No matches

		// Test search with pagination
		resp, code = c.Request(http.MethodGet, user1JWT, nil, "users?term=user&page=0")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data.Users) > 0, qt.IsTrue) // Should have results
	})

	t.Run("User Profile Operations", func(t *testing.T) {
		// Get own profile
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, profileResp.Data.Name, qt.Equals, "user1")
		qt.Assert(t, profileResp.Data.Email, qt.Equals, "user1@test.com")

		// Verify new fields are present
		qt.Assert(t, profileResp.Data.CreatedAt.IsZero(), qt.Equals, false,
			qt.Commentf("CreatedAt should not be zero"))
		qt.Assert(t, profileResp.Data.LastSeen.IsZero(), qt.Equals, false,
			qt.Commentf("LastSeen should not be zero"))
		qt.Assert(t, profileResp.Data.Bio, qt.Equals, "")
		qt.Assert(t, profileResp.Data.RatingCount >= 0, qt.IsTrue,
			qt.Commentf("RatingCount should be >= 0"))

		// Try to get profile without auth
		_, code = c.Request(http.MethodGet, "", nil, "profile")
		qt.Assert(t, code, qt.Equals, 401)

		// Update profile
		_, code = c.Request(http.MethodPost, user1JWT,
			api.UserProfile{
				Name:      "Updated User1",
				Community: "Updated Community",
				Bio:       "This is my bio",
				Location: &api.Location{
					Latitude:  41695384000,
					Longitude: 2492793000,
				},
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Attempt to update profile with a different email
		resp, code = c.Request(http.MethodPost, user1JWT,
			map[string]interface{}{
				"name":  "Updated User1",
				"email": "user2@test.com", // Attempt to use another user's email
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 400) // Should return 400 Bad Request

		// Verify the error message
		var errorResp struct {
			Header struct {
				Success   bool   `json:"success"`
				Message   string `json:"message"`
				ErrorCode int    `json:"errorCode"`
			} `json:"header"`
		}
		err = json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Equals, "email change not allowed")

		// Verify profile update
		resp, code = c.Request(http.MethodGet, user1JWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, profileResp.Data.Name, qt.Equals, "Updated User1")
		qt.Assert(t, profileResp.Data.Community, qt.Equals, "Updated Community")
		qt.Assert(t, profileResp.Data.Bio, qt.Equals, "This is my bio")
		qt.Assert(t, profileResp.Data.LastSeen.IsZero(), qt.Equals, false,
			qt.Commentf("LastSeen should be updated"))

		// Get other user's profile
		var user1ID string
		{
			resp, code := c.Request(http.MethodGet, user1JWT, nil, "profile")
			qt.Assert(t, code, qt.Equals, 200)
			var profileResp struct {
				Data *api.User `json:"data"`
			}
			err := json.Unmarshal(resp, &profileResp)
			qt.Assert(t, err, qt.IsNil)
			user1ID = profileResp.Data.ID
		}

		resp, code = c.Request(http.MethodGet, user2JWT, nil, "users", user1ID)
		qt.Assert(t, code, qt.Equals, 200)
		var otherUserResp struct {
			Data *api.User `json:"data"`
		}
		err = json.Unmarshal(resp, &otherUserResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, otherUserResp.Data.Name, qt.Equals, "Updated User1")

		// Try to get non-existent user
		_, code = c.Request(http.MethodGet, user1JWT, nil, "users", "999999")
		qt.Assert(t, code, qt.Equals, 404)

		// Test password change functionality

		// Attempt to change password without providing actualPassword
		resp, code = c.Request(http.MethodPost, user1JWT,
			api.UserProfile{
				Password: "newpassword",
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 400) // Should return 400 Bad Request

		// Verify the error message
		err = json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Equals, "actual password is required")

		// Attempt to change password with incorrect actualPassword
		resp, code = c.Request(http.MethodPost, user1JWT,
			api.UserProfile{
				Password:       "newpassword",
				ActualPassword: "wrongpassword",
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 403) // Should return 403 Forbidden

		// Verify the error message
		err = json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Equals, "invalid actual password")

		// Successfully change password with correct actualPassword
		_, code = c.Request(http.MethodPost, user1JWT,
			api.UserProfile{
				Password:       "newpassword",
				ActualPassword: "user1pass",
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200) // Should return 200 OK

		// Verify we can login with the new password
		resp, code = c.Request(http.MethodPost, "",
			&api.Login{
				Email:    "user1@test.com",
				Password: "newpassword",
			},
			"login",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Parse the JWT token from the response
		var loginResp struct {
			Data api.LoginResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &loginResp)
		qt.Assert(t, err, qt.IsNil)
		newUser1JWT := loginResp.Data.Token
		qt.Assert(t, newUser1JWT, qt.Not(qt.Equals), "")

		// Get refresh token
		resp, code = c.Request(http.MethodGet, newUser1JWT, nil, "refresh")
		qt.Assert(t, code, qt.Equals, 200)
		var refreshResp struct {
			Data *api.LoginResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &refreshResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, refreshResp.Data.Token, qt.Not(qt.IsNil))
	})
}

func TestUserLocationUpdates(t *testing.T) {
	c := utils.NewTestService(t)

	// Helper function to create a tool with specific location
	createToolWithLocation := func(jwt string, title string, location api.Location) int64 {
		resp, code := c.Request(http.MethodPost, jwt,
			api.Tool{
				Title:         title,
				Description:   "Test tool",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location:      location,
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		return toolResp.Data.ID
	}

	// Helper function to get tool by ID
	getTool := func(jwt string, toolID int64) api.Tool {
		resp, code := c.Request(http.MethodGet, jwt, nil, "tools", fmt.Sprintf("%d", toolID))
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data api.Tool `json:"data"`
		}
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		return toolResp.Data
	}

	// Helper function to create nomadic tool
	createNomadicTool := func(jwt string, title string, location api.Location) int64 {
		isNomadic := true
		resp, code := c.Request(http.MethodPost, jwt,
			api.Tool{
				Title:         title,
				Description:   "Nomadic test tool",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location:      location,
				IsNomadic:     &isNomadic,
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		return toolResp.Data.ID
	}

	barcelonaLocation := api.Location{
		Latitude:  41385063, // Barcelona coordinates in microdegrees
		Longitude: 2173404,
	}
	madridLocation := api.Location{
		Latitude:  40416775, // Madrid coordinates in microdegrees
		Longitude: -3703790,
	}
	parisLocation := api.Location{
		Latitude:  48856614, // Paris coordinates in microdegrees
		Longitude: 2352222,
	}

	t.Run("Location Update Updates Owned Tools", func(t *testing.T) {
		// Create user with Barcelona location
		userJWT := c.RegisterAndLogin(
			"location-test1@test.com", "LocationUser1", "password", &barcelonaLocation)

		// Create tools owned by the user with the same location
		tool1ID := createToolWithLocation(userJWT, "Tool 1", barcelonaLocation)
		tool2ID := createToolWithLocation(userJWT, "Tool 2", barcelonaLocation)

		// Create a tool with different location (should not be updated)
		tool3ID := createToolWithLocation(userJWT, "Tool 3", madridLocation)

		// Update user location to Paris
		_, code := c.Request(http.MethodPost, userJWT,
			api.UserProfile{
				Location: &parisLocation,
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify that tools with matching location were updated
		updatedTool1 := getTool(userJWT, tool1ID)
		qt.Assert(t, updatedTool1.Location.Latitude, qt.Equals, parisLocation.Latitude)
		qt.Assert(t, updatedTool1.Location.Longitude, qt.Equals, parisLocation.Longitude)

		updatedTool2 := getTool(userJWT, tool2ID)
		qt.Assert(t, updatedTool2.Location.Latitude, qt.Equals, parisLocation.Latitude)
		qt.Assert(t, updatedTool2.Location.Longitude, qt.Equals, parisLocation.Longitude)

		// Verify that tool with different location was not updated
		updatedTool3 := getTool(userJWT, tool3ID)
		qt.Assert(t, updatedTool3.Location.Latitude, qt.Equals, madridLocation.Latitude)
		qt.Assert(t, updatedTool3.Location.Longitude, qt.Equals, madridLocation.Longitude)
	})

	t.Run("Location Update Updates Nomadic Tools Held By User", func(t *testing.T) {
		// Create two users with Barcelona location
		ownerJWT, _ := c.RegisterAndLoginWithID(
			"nomadic-owner@test.com", "NomadicOwner", "password", &barcelonaLocation)
		holderJWT, holderID := c.RegisterAndLoginWithID(
			"nomadic-holder@test.com", "NomadicHolder", "password", &barcelonaLocation)

		// Create a nomadic tool owned by owner
		nomadicToolID := createNomadicTool(ownerJWT, "Nomadic Tool", barcelonaLocation)

		// Simulate the tool being picked up by holder
		// (this would normally happen through booking flow)
		// For testing purposes, we'll need to create a booking and mark it as picked up
		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		// Create booking
		resp, code := c.Request(http.MethodPost, holderJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprintf("%d", nomadicToolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking for nomadic tool",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var bookingResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		bookingID := bookingResp.Data.ID

		// Owner accepts the booking
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Owner marks as picked up
		// (this should set actualUserId and update tool location)
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "PICKED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Now update holder's location to Paris
		_, code = c.Request(http.MethodPost, holderJWT,
			api.UserProfile{
				Location: &parisLocation,
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify that the nomadic tool location was updated to holder's new location
		updatedTool := getTool(holderJWT, nomadicToolID)
		qt.Assert(t, updatedTool.Location.Latitude, qt.Equals, parisLocation.Latitude)
		qt.Assert(t, updatedTool.Location.Longitude, qt.Equals, parisLocation.Longitude)
		qt.Assert(t, updatedTool.ActualUserID, qt.Equals, holderID)
	})

	t.Run("Nomadic Tool Owner Location Change Does Not Update Tool", func(t *testing.T) {
		// Create two users with Barcelona location
		ownerJWT, ownerID := c.RegisterAndLoginWithID(
			"nomadic-owner2@test.com", "NomadicOwner2", "password", &barcelonaLocation)
		holderJWT, holderID := c.RegisterAndLoginWithID(
			"nomadic-holder2@test.com", "NomadicHolder2", "password", &barcelonaLocation)

		// Create a nomadic tool owned by owner
		nomadicToolID := createNomadicTool(ownerJWT, "Nomadic Tool 2", barcelonaLocation)

		// Create booking and simulate pickup
		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		resp, code := c.Request(http.MethodPost, holderJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprintf("%d", nomadicToolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking for nomadic tool",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var bookingResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		bookingID := bookingResp.Data.ID

		// Owner accepts and marks as picked up
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "PICKED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Get tool location before owner location change
		toolBeforeUpdate := getTool(holderJWT, nomadicToolID)
		originalLat := toolBeforeUpdate.Location.Latitude
		originalLon := toolBeforeUpdate.Location.Longitude

		// Now update OWNER's location to Paris (not the holder)
		_, code = c.Request(http.MethodPost, ownerJWT,
			api.UserProfile{
				Location: &parisLocation,
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify that the nomadic tool location DID NOT change (should remain at holder's location)
		updatedTool := getTool(holderJWT, nomadicToolID)
		qt.Assert(t, updatedTool.Location.Latitude, qt.Equals, originalLat,
			qt.Commentf("Tool location should not change when owner moves"))
		qt.Assert(t, updatedTool.Location.Longitude, qt.Equals, originalLon,
			qt.Commentf("Tool location should not change when owner moves"))
		qt.Assert(t, updatedTool.ActualUserID, qt.Equals, holderID,
			qt.Commentf("Tool should still be held by holder"))
		qt.Assert(t, updatedTool.UserID, qt.Equals, ownerID,
			qt.Commentf("Tool should still be owned by owner"))
	})

	t.Run("Mixed Scenario - Owned and Nomadic Tools", func(t *testing.T) {
		// Create users with Barcelona location
		user1JWT, user1ID := c.RegisterAndLoginWithID(
			"mixed-user1@test.com", "MixedUser1", "password", &barcelonaLocation)
		user2JWT, _ := c.RegisterAndLoginWithID(
			"mixed-user2@test.com", "MixedUser2", "password", &barcelonaLocation)

		// Create various tools
		// Tool owned by user1 with matching location (should be updated)
		ownedToolID := createToolWithLocation(user1JWT, "Owned Tool", barcelonaLocation)

		// Tool owned by user1 with different location (should not be updated)
		ownedToolDifferentID := createToolWithLocation(user1JWT, "Owned Tool Different", madridLocation)

		// Create nomadic tool owned by user2 but held by user1
		nomadicToolID := createNomadicTool(user2JWT, "Nomadic Tool", barcelonaLocation)

		// Simulate nomadic tool pickup by user1
		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		resp, code := c.Request(http.MethodPost, user1JWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprintf("%d", nomadicToolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var bookingResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		bookingID := bookingResp.Data.ID

		// Accept and mark as picked up
		_, code = c.Request(http.MethodPut, user2JWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		_, code = c.Request(http.MethodPut, user2JWT,
			&api.BookingStatusUpdate{
				Status: "PICKED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Tool owned by different user (should not be updated)
		otherUserToolID := createToolWithLocation(user2JWT, "Other User Tool", barcelonaLocation)

		// Update user1's location to Paris
		_, code = c.Request(http.MethodPost, user1JWT,
			api.UserProfile{
				Location: &parisLocation,
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify owned tool with matching location was updated
		updatedOwnedTool := getTool(user1JWT, ownedToolID)
		qt.Assert(t, updatedOwnedTool.Location.Latitude, qt.Equals, parisLocation.Latitude)
		qt.Assert(t, updatedOwnedTool.Location.Longitude, qt.Equals, parisLocation.Longitude)

		// Verify owned tool with different location was not updated
		updatedOwnedToolDifferent := getTool(user1JWT, ownedToolDifferentID)
		qt.Assert(t, updatedOwnedToolDifferent.Location.Latitude, qt.Equals, madridLocation.Latitude)
		qt.Assert(t, updatedOwnedToolDifferent.Location.Longitude, qt.Equals, madridLocation.Longitude)

		// Verify nomadic tool held by user1 was updated
		updatedNomadicTool := getTool(user1JWT, nomadicToolID)
		qt.Assert(t, updatedNomadicTool.Location.Latitude, qt.Equals, parisLocation.Latitude)
		qt.Assert(t, updatedNomadicTool.Location.Longitude, qt.Equals, parisLocation.Longitude)
		qt.Assert(t, updatedNomadicTool.ActualUserID, qt.Equals, user1ID)

		// Verify tool owned by different user was not updated
		updatedOtherUserTool := getTool(user2JWT, otherUserToolID)
		qt.Assert(t, updatedOtherUserTool.Location.Latitude, qt.Equals, barcelonaLocation.Latitude)
		qt.Assert(t, updatedOtherUserTool.Location.Longitude, qt.Equals, barcelonaLocation.Longitude)
	})

	t.Run("Non-Location Update Does Not Affect Tools", func(t *testing.T) {
		// Create user with Barcelona location
		userJWT := c.RegisterAndLogin(
			"non-location-test@test.com", "NonLocationUser", "password", &barcelonaLocation)

		// Create a tool owned by the user
		toolID := createToolWithLocation(userJWT, "Test Tool", barcelonaLocation)

		// Update user name (not location)
		_, code := c.Request(http.MethodPost, userJWT,
			api.UserProfile{
				Name: "Updated Name",
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify that tool location was not changed
		updatedTool := getTool(userJWT, toolID)
		qt.Assert(t, updatedTool.Location.Latitude, qt.Equals, barcelonaLocation.Latitude)
		qt.Assert(t, updatedTool.Location.Longitude, qt.Equals, barcelonaLocation.Longitude)

		// Verify user name was updated
		resp, code := c.Request(http.MethodGet, userJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, profileResp.Data.Name, qt.Equals, "Updated Name")
	})
}

func TestAdditionalContactsVisibility(t *testing.T) {
	c := utils.NewTestService(t)

	// Create test users with additional contacts
	user1JWT, user1ID := registerUserWithContacts(c, "user1@test.com", "User One", map[string]string{
		"telegram": "@user1_telegram",
		"whatsapp": "+34123456789",
	})

	user2JWT, user2ID := registerUserWithContacts(c, "user2@test.com", "User Two", map[string]string{
		"signal":  "+34987654321",
		"discord": "user2#1234",
	})

	user3JWT, _ := registerUserWithContacts(c, "user3@test.com", "User Three", map[string]string{
		"email": "user3_alt@test.com",
	})

	// Test Case 1: User accessing their own profile should see additional contacts
	t.Run("Own profile includes additional contacts", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, profileResp.Data.AdditionalContacts, qt.Not(qt.IsNil))
		qt.Assert(t, profileResp.Data.AdditionalContacts["telegram"], qt.Equals, "@user1_telegram")
		qt.Assert(t, profileResp.Data.AdditionalContacts["whatsapp"], qt.Equals, "+34123456789")
	})

	// Test Case 2: User accessing another user without accepted bookings should NOT see additional contacts
	t.Run("Other user without bookings does not see additional contacts", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "users", user2ID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		// Should not include additional contacts
		qt.Assert(t, userResp.Data.AdditionalContacts, qt.IsNil)
	})

	// Test Case 3: Create an accepted booking between users and verify additional contacts are visible
	t.Run("Users with accepted bookings see each other's additional contacts", func(t *testing.T) {
		// Create a tool for user2
		toolID := c.CreateTool(user2JWT, "Test Tool")

		// Create a booking from user1 to user2
		bookingID := createBooking(c, user1JWT, toolID)

		// Accept the booking
		acceptBooking(c, user2JWT, bookingID)

		// Now user1 should see user2's additional contacts
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "users", user2ID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, userResp.Data.AdditionalContacts, qt.Not(qt.IsNil))
		qt.Assert(t, userResp.Data.AdditionalContacts["signal"], qt.Equals, "+34987654321")
		qt.Assert(t, userResp.Data.AdditionalContacts["discord"], qt.Equals, "user2#1234")

		// And user2 should see user1's additional contacts
		resp, code = c.Request(http.MethodGet, user2JWT, nil, "users", user1ID)
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, userResp.Data.AdditionalContacts, qt.Not(qt.IsNil))
		qt.Assert(t, userResp.Data.AdditionalContacts["telegram"], qt.Equals, "@user1_telegram")
		qt.Assert(t, userResp.Data.AdditionalContacts["whatsapp"], qt.Equals, "+34123456789")
	})

	// Test Case 4: User3 (no bookings with user1 or user2) should not see additional contacts
	t.Run("Third user without bookings does not see additional contacts", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, user3JWT, nil, "users", user1ID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, userResp.Data.AdditionalContacts, qt.IsNil)

		resp, code = c.Request(http.MethodGet, user3JWT, nil, "users", user2ID)
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, userResp.Data.AdditionalContacts, qt.IsNil)
	})
}

func TestBookingBasedAdditionalContactsVisibility(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users with additional contacts using API
	user1JWT, user1ID := registerUserWithContacts(c, "bookingtest1@test.com", "Booking Test User 1", map[string]string{
		"telegram": "@bookinguser1",
		"whatsapp": "+34111111111",
	})

	user2JWT, user2ID := registerUserWithContacts(c, "bookingtest2@test.com", "Booking Test User 2", map[string]string{
		"signal":  "+34222222222",
		"discord": "bookinguser2#1234",
	})

	user3JWT, user3ID := registerUserWithContacts(c, "bookingtest3@test.com", "Booking Test User 3", map[string]string{
		"email": "bookinguser3_alt@test.com",
	})

	// Test Case 1: No bookings between users - additional contacts should not be visible
	t.Run("No bookings - additional contacts not visible", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "users", user2ID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		// Should not include additional contacts
		qt.Assert(t, userResp.Data.AdditionalContacts, qt.IsNil)
	})

	// Test Case 2: Same user accessing themselves should see additional contacts via users endpoint
	t.Run("Same user via users endpoint - additional contacts visible", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "users", user1ID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		// Should include additional contacts when accessing own profile via users endpoint
		qt.Assert(t, userResp.Data.AdditionalContacts, qt.Not(qt.IsNil))
		qt.Assert(t, userResp.Data.AdditionalContacts["telegram"], qt.Equals, "@bookinguser1")
		qt.Assert(t, userResp.Data.AdditionalContacts["whatsapp"], qt.Equals, "+34111111111")
	})

	// Test Case 3: Create a pending booking - additional contacts should still not be visible
	t.Run("Pending booking - additional contacts not visible", func(t *testing.T) {
		// Create a tool for user2
		toolID := c.CreateTool(user2JWT, "Booking Test Tool")

		// Create a booking from user1 to user2
		bookingID := createBooking(c, user1JWT, toolID)

		// Verify booking was created but additional contacts still not visible
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "users", user2ID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		// Should still not include additional contacts for pending booking
		qt.Assert(t, userResp.Data.AdditionalContacts, qt.IsNil)

		// Accept the booking
		acceptBooking(c, user2JWT, bookingID)

		// Now user1 should see user2's additional contacts
		resp, code = c.Request(http.MethodGet, user1JWT, nil, "users", user2ID)
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, userResp.Data.AdditionalContacts, qt.Not(qt.IsNil))
		qt.Assert(t, userResp.Data.AdditionalContacts["signal"], qt.Equals, "+34222222222")
		qt.Assert(t, userResp.Data.AdditionalContacts["discord"], qt.Equals, "bookinguser2#1234")

		// And user2 should see user1's additional contacts
		resp, code = c.Request(http.MethodGet, user2JWT, nil, "users", user1ID)
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, userResp.Data.AdditionalContacts, qt.Not(qt.IsNil))
		qt.Assert(t, userResp.Data.AdditionalContacts["telegram"], qt.Equals, "@bookinguser1")
		qt.Assert(t, userResp.Data.AdditionalContacts["whatsapp"], qt.Equals, "+34111111111")
	})

	// Test Case 4: User without bookings should not see additional contacts
	t.Run("User without bookings does not see additional contacts", func(t *testing.T) {
		// User3 should not see additional contacts from user1 or user2 (no bookings between them)
		resp, code := c.Request(http.MethodGet, user3JWT, nil, "users", user1ID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, userResp.Data.AdditionalContacts, qt.IsNil)

		resp, code = c.Request(http.MethodGet, user3JWT, nil, "users", user2ID)
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, userResp.Data.AdditionalContacts, qt.IsNil)
	})

	// Test Case 4: Bidirectional visibility after accepted booking
	t.Run("Bidirectional visibility after accepted booking", func(t *testing.T) {
		// Create another tool for user3
		toolID := c.CreateTool(user3JWT, "Another Test Tool")

		// Create a booking from user1 to user3
		bookingID := createBooking(c, user1JWT, toolID)

		// Accept the booking
		acceptBooking(c, user3JWT, bookingID)

		// Now user1 should see user3's additional contacts
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "users", user3ID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, userResp.Data.AdditionalContacts, qt.Not(qt.IsNil))
		qt.Assert(t, userResp.Data.AdditionalContacts["email"], qt.Equals, "bookinguser3_alt@test.com")

		// And user3 should see user1's additional contacts
		resp, code = c.Request(http.MethodGet, user3JWT, nil, "users", user1ID)
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, userResp.Data.AdditionalContacts, qt.Not(qt.IsNil))
		qt.Assert(t, userResp.Data.AdditionalContacts["telegram"], qt.Equals, "@bookinguser1")
		qt.Assert(t, userResp.Data.AdditionalContacts["whatsapp"], qt.Equals, "+34111111111")
	})
}

func TestAdditionalContactsValidation(t *testing.T) {
	// Test Case 1: Valid additional contacts
	t.Run("Valid additional contacts", func(t *testing.T) {
		contacts := api.AdditionalContacts{
			"telegram": "@validuser",
			"whatsapp": "+34123456789",
		}
		err := contacts.Validate()
		qt.Assert(t, err, qt.IsNil)
	})

	// Test Case 2: Empty additional contacts (should fail)
	t.Run("Empty additional contacts fails validation", func(t *testing.T) {
		contacts := api.AdditionalContacts{}
		err := contacts.Validate()
		qt.Assert(t, err, qt.Not(qt.IsNil))
		qt.Assert(t, err.Error(), qt.Contains, "at least one additional contact is required")
	})

	// Test Case 3: Empty key (should fail)
	t.Run("Empty key fails validation", func(t *testing.T) {
		contacts := api.AdditionalContacts{
			"": "somevalue",
		}
		err := contacts.Validate()
		qt.Assert(t, err, qt.Not(qt.IsNil))
		qt.Assert(t, err.Error(), qt.Contains, "contact key '' is invalid")
	})

	// Test Case 4: Empty value (should fail)
	t.Run("Empty value fails validation", func(t *testing.T) {
		contacts := api.AdditionalContacts{
			"telegram": "",
		}
		err := contacts.Validate()
		qt.Assert(t, err, qt.Not(qt.IsNil))
		qt.Assert(t, err.Error(), qt.Contains, "value for key 'telegram' is invalid")
	})

	// Test Case 5: Key too long (should fail)
	t.Run("Key too long fails validation", func(t *testing.T) {
		longKey := string(make([]byte, 51)) // 51 characters
		for i := range longKey {
			longKey = longKey[:i] + "a" + longKey[i+1:]
		}
		contacts := api.AdditionalContacts{
			longKey: "value",
		}
		err := contacts.Validate()
		qt.Assert(t, err, qt.Not(qt.IsNil))
		qt.Assert(t, err.Error(), qt.Contains, "contact key")
		qt.Assert(t, err.Error(), qt.Contains, "is invalid")
	})

	// Test Case 6: Value too long (should fail)
	t.Run("Value too long fails validation", func(t *testing.T) {
		longValue := string(make([]byte, 51)) // 51 characters
		for i := range longValue {
			longValue = longValue[:i] + "a" + longValue[i+1:]
		}
		contacts := api.AdditionalContacts{
			"telegram": longValue,
		}
		err := contacts.Validate()
		qt.Assert(t, err, qt.Not(qt.IsNil))
		qt.Assert(t, err.Error(), qt.Contains, "value for key 'telegram' is invalid")
	})
}

// Helper functions

func registerUserWithContacts(c *utils.TestService, email, name string, contacts map[string]string) (string, string) {
	// Register user with additional contacts
	_, code := c.Request(http.MethodPost, "",
		map[string]interface{}{
			"email":              email,
			"invitationToken":    utils.RegisterToken,
			"name":               name,
			"community":          "testCommunity",
			"password":           "testpassword123",
			"location":           map[string]int64{"latitude": 41385064, "longitude": 2173403},
			"additionalContacts": contacts,
		},
		"register",
	)
	qt.Assert(c.GetT(), code, qt.Equals, 200)

	// Login to get JWT
	resp, code := c.Request(http.MethodPost, "",
		&api.Login{
			Email:    email,
			Password: "testpassword123",
		},
		"login",
	)
	qt.Assert(c.GetT(), code, qt.Equals, 200)

	var loginResponse struct {
		Data api.LoginResponse `json:"data"`
	}
	err := json.Unmarshal(resp, &loginResponse)
	qt.Assert(c.GetT(), err, qt.IsNil)
	jwt := loginResponse.Data.Token

	// Get profile to get user ID
	resp, code = c.Request(http.MethodGet, jwt, nil, "profile")
	qt.Assert(c.GetT(), code, qt.Equals, 200)

	var profileResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	err = json.Unmarshal(resp, &profileResponse)
	qt.Assert(c.GetT(), err, qt.IsNil)

	return jwt, profileResponse.Data.ID
}

func createBooking(c *utils.TestService, jwt string, toolID int64) string {
	bookingData := map[string]interface{}{
		"toolId":    fmt.Sprintf("%d", toolID),
		"startDate": time.Now().Add(24 * time.Hour).Unix(),
		"endDate":   time.Now().Add(48 * time.Hour).Unix(),
		"contact":   "test@test.com",
		"comments":  "Test booking",
	}

	resp, code := c.Request(http.MethodPost, jwt, bookingData, "bookings")
	qt.Assert(c.GetT(), code, qt.Equals, 200)

	var bookingResp struct {
		Data api.BookingResponse `json:"data"`
	}
	err := json.Unmarshal(resp, &bookingResp)
	qt.Assert(c.GetT(), err, qt.IsNil)

	return bookingResp.Data.ID
}

func acceptBooking(c *utils.TestService, jwt string, bookingID string) {
	statusData := map[string]interface{}{
		"status": "ACCEPTED",
	}

	_, code := c.Request(http.MethodPut, jwt, statusData, "bookings", bookingID)
	qt.Assert(c.GetT(), code, qt.Equals, 200)
}
