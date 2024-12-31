package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestUser(t *testing.T) {
	c := utils.NewTestService(t)

	// Create two users
	user1JWT := c.RegisterAndLogin("user1@test.com", "user1", "user1pass")
	user2JWT := c.RegisterAndLogin("user2@test.com", "user2", "user2pass")

	t.Run("User Profile Operations", func(t *testing.T) {
		// Get own profile
		resp, code := c.Request(http.MethodGet, user1JWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		var profileResp struct {
			Data db.User `json:"data"`
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
			map[string]interface{}{
				"name":      "Updated User1",
				"community": "Updated Community",
				"location": map[string]int64{
					"latitude":  41695384000,
					"longitude": 2492793000,
				},
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

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
				Data db.User `json:"data"`
			}
			err := json.Unmarshal(resp, &profileResp)
			qt.Assert(t, err, qt.IsNil)
			user1ID = profileResp.Data.ID.Hex()
		}

		resp, code = c.Request(http.MethodGet, user2JWT, nil, "users", user1ID)
		qt.Assert(t, code, qt.Equals, 200)
		var otherUserResp struct {
			Data db.User `json:"data"`
		}
		err = json.Unmarshal(resp, &otherUserResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, otherUserResp.Data.Name, qt.Equals, "Updated User1")

		// Try to get non-existent user
		_, code = c.Request(http.MethodGet, user1JWT, nil, "users", "999999")
		qt.Assert(t, code, qt.Equals, 404)

		// Get refresh token
		resp, code = c.Request(http.MethodGet, user1JWT, nil, "refresh")
		qt.Assert(t, code, qt.Equals, 200)
		var refreshResp struct {
			Data api.LoginResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &refreshResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, refreshResp.Data.Token, qt.Not(qt.IsNil))
	})
}
