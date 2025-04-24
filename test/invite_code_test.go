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
		// Request invite codes
		resp, code := c.Request(http.MethodPost, userJWT, nil, "profile", "invites")
		qt.Assert(t, code, qt.Equals, 200)

		var inviteResp struct {
			Data []*api.InviteCode `json:"data"`
		}
		err := json.Unmarshal(resp, &inviteResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(inviteResp.Data) > 0, qt.IsTrue)

		// Verify invite codes are returned in profile
		resp, code = c.Request(http.MethodGet, userJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err = json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(profileResp.Data.InviteCodes) > 0, qt.IsTrue)

		// Try to request more codes while still having unused ones
		resp, code = c.Request(http.MethodPost, userJWT, nil, "profile", "invites")
		qt.Assert(t, code, qt.Equals, 409) // Conflict

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

		// Verify the invite code is now marked as used
		resp, code = c.Request(http.MethodGet, userJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)

		var foundUsedCode bool
		for _, code := range profileResp.Data.InviteCodes {
			if code.Code == inviteCode {
				qt.Assert(t, code.UsedByID != nil, qt.IsTrue)
				qt.Assert(t, code.UsedOn != nil, qt.IsTrue)
				foundUsedCode = true
				break
			}
		}
		qt.Assert(t, foundUsedCode, qt.IsTrue)

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
