package test

import (
	"encoding/json"
	"net/http"
	"testing"

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

	t.Run("Paginated Users List", func(t *testing.T) {
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
		qt.Assert(t, len(usersResp.Data.Users), qt.Equals, 5) // All users since we have less than page size

		// Test invalid page number
		_, code = c.Request(http.MethodGet, user1JWT, nil, "users?page=-1")
		qt.Assert(t, code, qt.Equals, 400)

		// Test unauthorized access
		_, code = c.Request(http.MethodGet, "", nil, "users")
		qt.Assert(t, code, qt.Equals, 401)
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

		// Try to get profile without auth
		_, code = c.Request(http.MethodGet, "", nil, "profile")
		qt.Assert(t, code, qt.Equals, 401)

		// Update profile
		_, code = c.Request(http.MethodPost, user1JWT,
			api.UserProfile{
				Name:      "Updated User1",
				Community: "Updated Community",
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
