package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestInviteCodes(t *testing.T) {
	c := utils.NewTestService(t)

	// Register a user with the master token
	userJWT, _ := c.RegisterAndLoginWithID("user@test.com", "user", "userpass")

	t.Run("Request Invite Codes", func(t *testing.T) {
		// Verify invite codes are already present in profile after registration
		resp, code := c.Request(http.MethodGet, userJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(profileResp.Data.InviteCodes) > 0, qt.IsTrue)

		// Try to request codes while still having unused ones (should fail with 409 Conflict)
		resp, code = c.Request(http.MethodPost, userJWT, nil, "profile", "invites")
		qt.Assert(t, code, qt.Equals, 409, qt.Commentf("Expected 409 Conflict when requesting codes while having unused ones"))

		// Verify error message
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err = json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Equals, "you still have unused invite codes")
	})

	t.Run("Register with Invite Code", func(t *testing.T) {
		// Get an invite code from the first user
		resp, code := c.Request(http.MethodGet, userJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(profileResp.Data.InviteCodes) > 0, qt.IsTrue)

		inviteCode := profileResp.Data.InviteCodes[0].Code

		// Register a new user with the invite code
		_, code = c.Request(http.MethodPost, "",
			&api.Register{
				UserEmail:         "invited@test.com",
				RegisterAuthToken: inviteCode,
				UserProfile: api.UserProfile{
					Name:     "Invited User",
					Password: "invitedpass",
				},
			},
			"register",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the invite code is no longer in the profile (since we only return unused codes)
		resp, code = c.Request(http.MethodGet, userJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)

		// Check that the used invite code is no longer in the list
		var foundUsedCode bool
		for _, code := range profileResp.Data.InviteCodes {
			if code.Code == inviteCode {
				foundUsedCode = true
				break
			}
		}
		qt.Assert(t, foundUsedCode, qt.Equals, false, qt.Commentf("Used invite code should not be returned in profile"))

		// Try to register another user with the same invite code (should fail)
		resp, code = c.Request(http.MethodPost, "",
			&api.Register{
				UserEmail:         "another@test.com",
				RegisterAuthToken: inviteCode,
				UserProfile: api.UserProfile{
					Name:     "Another User",
					Password: "anotherpass",
				},
			},
			"register",
		)
		qt.Assert(t, code, qt.Equals, 400) // Bad Request

		// Verify error message
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err = json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "invite code already used")
	})

	t.Run("Invalid Invite Code", func(t *testing.T) {
		// Try to register with an invalid invite code
		resp, code := c.Request(http.MethodPost, "",
			&api.Register{
				UserEmail:         "invalid@test.com",
				RegisterAuthToken: "invalid-code",
				UserProfile: api.UserProfile{
					Name:     "Invalid User",
					Password: "invalidpass",
				},
			},
			"register",
		)
		qt.Assert(t, code, qt.Equals, 400) // Bad Request

		// Verify error message
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err := json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "invalid invite code")
	})
}
