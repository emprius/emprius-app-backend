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

func TestCommunities(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users for testing
	ownerJWT, ownerID := c.RegisterAndLoginWithID("community-owner@test.com", "owner", "ownerpass")
	memberJWT, memberID := c.RegisterAndLoginWithID("community-member@test.com", "member", "memberpass")
	nonMemberJWT, nonMemberID := c.RegisterAndLoginWithID("community-nonmember@test.com", "nonmember", "nonmemberpass")

	t.Run("Community Creation and Management", func(t *testing.T) {
		// Test creating a community without auth
		_, code := c.Request(http.MethodPost, "",
			api.CreateCommunityRequest{
				Name: "Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 401)

		// Test creating a community with auth
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var createResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		communityID := createResp.Data.ID

		// Test getting community details
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID)
		qt.Assert(t, code, qt.Equals, 200)

		var getResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &getResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, getResp.Data.Name, qt.Equals, "Test Community")
		qt.Assert(t, getResp.Data.OwnerID, qt.Equals, ownerID)

		// Test updating community without auth
		_, code = c.Request(http.MethodPut, "",
			api.UpdateCommunityRequest{
				Name: "Updated Community",
			},
			"communities", communityID,
		)
		qt.Assert(t, code, qt.Equals, 401)

		// Test updating community as non-owner
		_, code = c.Request(http.MethodPut, memberJWT,
			api.UpdateCommunityRequest{
				Name: "Updated Community",
			},
			"communities", communityID,
		)
		qt.Assert(t, code, qt.Equals, 401)

		// Test updating community as owner
		_, code = c.Request(http.MethodPut, ownerJWT,
			api.UpdateCommunityRequest{
				Name: "Updated Community",
			},
			"communities", communityID,
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the update
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &getResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, getResp.Data.Name, qt.Equals, "Updated Community")

		// Test deleting community without auth
		_, code = c.Request(http.MethodDelete, "", nil, "communities", communityID)
		qt.Assert(t, code, qt.Equals, 401)

		// Test deleting community as non-owner
		_, code = c.Request(http.MethodDelete, memberJWT, nil, "communities", communityID)
		qt.Assert(t, code, qt.Equals, 401)

		// Create a second community for deletion test
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Community to Delete",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		communityToDeleteID := createResp.Data.ID

		// Test deleting community as owner
		_, code = c.Request(http.MethodDelete, ownerJWT, nil, "communities", communityToDeleteID)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the deletion
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityToDeleteID)
		qt.Assert(t, code, qt.Equals, 404)
	})

	t.Run("Community Users", func(t *testing.T) {
		// Create a community for testing
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Users Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var createResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		communityID := createResp.Data.ID

		// Test getting community users without auth
		_, code = c.Request(http.MethodGet, "", nil, "communities", communityID, "members")
		qt.Assert(t, code, qt.Equals, 401)

		// Test getting community users with auth
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "members")
		qt.Assert(t, code, qt.Equals, 200)

		var usersResp struct {
			Data []api.CommunityUserResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data), qt.Equals, 1) // Only the owner should be in the community initially

		// Verify the owner is in the community with the correct role
		data := usersResp.Data[0]
		qt.Assert(t, data.ID, qt.Equals, ownerID)
		qt.Assert(t, string(usersResp.Data[0].Role), qt.Equals, "owner")

		// Test pagination by creating a community with multiple users
		// First invite and accept a user to the community
		_, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", communityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)

		// Get pending invites for the member
		resp, code = c.Request(http.MethodGet, memberJWT, nil, "communities", "invites")
		qt.Assert(t, code, qt.Equals, 200)

		var invitesResp struct {
			Data []api.CommunityInviteResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &invitesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(invitesResp.Data), qt.Equals, 1)
		inviteID := invitesResp.Data[0].ID

		// Accept the invitation
		_, code = c.Request(http.MethodPut, memberJWT,
			map[string]interface{}{
				"status": "ACCEPTED",
			},
			"communities", "invites", inviteID)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the member is now in the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "members")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data), qt.Equals, 2) // Owner and member

		// Test pagination with page parameter
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "members?page=0")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data), qt.Equals, 2) // Should return all users since we have less than page size

		// Test invalid page number
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "members?page=-1")
		qt.Assert(t, code, qt.Equals, 400)
	})

	t.Run("Community Invitations", func(t *testing.T) {
		// Create a community for testing
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Invitations Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var createResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		communityID := createResp.Data.ID

		// Test inviting a user without auth
		_, code = c.Request(http.MethodPost, "", nil, "communities", communityID, "members", nonMemberID)
		qt.Assert(t, code, qt.Equals, 401)

		// Test inviting a user with auth
		resp, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", communityID, "members", nonMemberID)
		qt.Assert(t, code, qt.Equals, 200)

		var inviteResp struct {
			Data api.CommunityInviteResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &inviteResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, inviteResp.Data.CommunityID, qt.Equals, communityID)
		qt.Assert(t, inviteResp.Data.UserID, qt.Equals, nonMemberID)
		qt.Assert(t, inviteResp.Data.Status, qt.Equals, "pending")
		inviteID := inviteResp.Data.ID

		// Test getting pending invites without auth
		_, code = c.Request(http.MethodGet, "", nil, "communities", "invites")
		qt.Assert(t, code, qt.Equals, 401)

		// Test getting pending invites with auth
		resp, code = c.Request(http.MethodGet, nonMemberJWT, nil, "communities", "invites")
		qt.Assert(t, code, qt.Equals, 200)

		var invitesResp struct {
			Data []api.CommunityInviteResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &invitesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(invitesResp.Data), qt.Equals, 1)
		qt.Assert(t, invitesResp.Data[0].ID, qt.Equals, inviteID)

		// Test accepting an invite without auth
		_, code = c.Request(http.MethodPut, "",
			map[string]interface{}{
				"status": "ACCEPTED",
			},
			"communities", "invites", inviteID)
		qt.Assert(t, code, qt.Equals, 401)

		// Test accepting an invite with wrong user
		_, code = c.Request(http.MethodPut, memberJWT,
			map[string]interface{}{
				"status": "ACCEPTED",
			},
			"communities", "invites", inviteID)
		qt.Assert(t, code, qt.Equals, 500) // Internal server error when invite not found for this user

		// Test accepting an invite with correct user
		_, code = c.Request(http.MethodPut, nonMemberJWT,
			map[string]interface{}{
				"status": "ACCEPTED",
			},
			"communities", "invites", inviteID)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the user is now in the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "members")
		qt.Assert(t, code, qt.Equals, 200)
		var usersResp struct {
			Data []api.CommunityUserResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)

		// Find the non-member in the users list
		var found bool
		for _, user := range usersResp.Data {
			if user.ID == nonMemberID {
				found = true
				qt.Assert(t, string(user.Role), qt.Equals, "user")
				break
			}
		}
		qt.Assert(t, found, qt.IsTrue)

		// Create another community for testing invite rejection
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Rejection Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		rejectionCommunityID := createResp.Data.ID

		// Invite the member to the new community
		resp, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", rejectionCommunityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &inviteResp)
		qt.Assert(t, err, qt.IsNil)
		rejectInviteID := inviteResp.Data.ID

		// Test rejecting an invite without auth
		_, code = c.Request(http.MethodPut, "",
			map[string]interface{}{
				"status": "REJECTED",
			},
			"communities", "invites", rejectInviteID)
		qt.Assert(t, code, qt.Equals, 401)

		// Test rejecting an invite with wrong user
		_, code = c.Request(http.MethodPut, nonMemberJWT,
			map[string]interface{}{
				"status": "REJECTED",
			},
			"communities", "invites", rejectInviteID)
		qt.Assert(t, code, qt.Equals, 500) // Internal server error when invite not found for this user

		// Test rejecting an invite with correct user
		_, code = c.Request(http.MethodPut, memberJWT,
			map[string]interface{}{
				"status": "REJECTED",
			},
			"communities", "invites", rejectInviteID)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the user is not in the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", rejectionCommunityID, "members")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)

		// Ensure the member is not in the community
		found = false
		for _, user := range usersResp.Data {
			if user.ID == memberID {
				found = true
				break
			}
		}
		qt.Assert(t, found, qt.IsFalse)

		// Test counting pending invites
		// Create a new community and invite the member
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Count Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		countCommunityID := createResp.Data.ID

		// Invite the member to the new community
		_, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", countCommunityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)

		// Get pending counts
		resp, code = c.Request(http.MethodGet, memberJWT, nil, "profile", "pendings")
		qt.Assert(t, code, qt.Equals, 200)
		var pendingsResp struct {
			Data api.PendingActionsResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &pendingsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, pendingsResp.Data.PendingInvitesCount, qt.Equals, int64(1))

		// Test the new updateInviteStatus endpoint
		// Create a new community for testing
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Update Invite Status Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		updateInviteCommunityID := createResp.Data.ID

		// Invite the member to the new community
		resp, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", updateInviteCommunityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &inviteResp)
		qt.Assert(t, err, qt.IsNil)
		updateInviteID := inviteResp.Data.ID

		// Test updating invite status without auth
		_, code = c.Request(http.MethodPut, "",
			map[string]interface{}{
				"status": "ACCEPTED",
			},
			"communities", "invites", updateInviteID)
		qt.Assert(t, code, qt.Equals, 401)

		// Test updating invite status with wrong user
		_, code = c.Request(http.MethodPut, nonMemberJWT,
			map[string]interface{}{
				"status": "ACCEPTED",
			},
			"communities", "invites", updateInviteID)
		qt.Assert(t, code, qt.Equals, 500) // Internal server error when invite not found for this user

		// Test accepting invite with new endpoint
		_, code = c.Request(http.MethodPut, memberJWT,
			map[string]interface{}{
				"status": "ACCEPTED",
			},
			"communities", "invites", updateInviteID)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the member is now in the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", updateInviteCommunityID, "members")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)

		// Find the member in the users list
		found = false
		for _, user := range usersResp.Data {
			if user.ID == memberID {
				found = true
				qt.Assert(t, string(user.Role), qt.Equals, "user")
				break
			}
		}
		qt.Assert(t, found, qt.IsTrue)

		// Create another community for testing invite rejection with new endpoint
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "New Rejection Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		newRejectionCommunityID := createResp.Data.ID

		// Invite the member to the new community
		resp, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", newRejectionCommunityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &inviteResp)
		qt.Assert(t, err, qt.IsNil)
		newRejectInviteID := inviteResp.Data.ID

		// Test rejecting invite with new endpoint
		_, code = c.Request(http.MethodPut, memberJWT,
			map[string]interface{}{
				"status": "REJECTED",
			},
			"communities", "invites", newRejectInviteID)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the member is not in the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", newRejectionCommunityID, "members")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)

		// Ensure the member is not in the community
		found = false
		for _, user := range usersResp.Data {
			if user.ID == memberID {
				found = true
				break
			}
		}
		qt.Assert(t, found, qt.IsFalse)
	})

	t.Run("Community Membership", func(t *testing.T) {
		// Create a community for testing
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Membership Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var createResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		communityID := createResp.Data.ID

		// Invite and accept a user to the community
		_, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", communityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)

		// Get pending invites for the member
		resp, code = c.Request(http.MethodGet, memberJWT, nil, "communities", "invites")
		qt.Assert(t, code, qt.Equals, 200)

		var invitesResp struct {
			Data []api.CommunityInviteResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &invitesResp)
		qt.Assert(t, err, qt.IsNil)

		// Find the invite for this community
		var inviteID string
		for _, invite := range invitesResp.Data {
			if invite.CommunityID == communityID {
				inviteID = invite.ID
				break
			}
		}
		qt.Assert(t, inviteID, qt.Not(qt.Equals), "")

		// Accept the invitation
		_, code = c.Request(http.MethodPut, memberJWT,
			map[string]interface{}{
				"status": "ACCEPTED",
			},
			"communities", "invites", inviteID)
		qt.Assert(t, code, qt.Equals, 200)

		// Test leaving a community without auth
		_, code = c.Request(http.MethodDelete, "", nil, "communities", communityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 401)

		// Test owner trying to leave the community (should fail)
		_, code = c.Request(http.MethodDelete, ownerJWT, nil, "communities", communityID, "members", ownerID)
		qt.Assert(t, code, qt.Equals, 500) // Internal server error with message "community owner cannot leave the community"

		// Test member leaving the community
		_, code = c.Request(http.MethodDelete, memberJWT, nil, "communities", communityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the member is no longer in the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "members")
		qt.Assert(t, code, qt.Equals, 200)
		var usersResp struct {
			Data []api.CommunityUserResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)

		// Ensure the member is not in the community
		found := false
		for _, user := range usersResp.Data {
			if user.ID == memberID {
				found = true
				break
			}
		}
		qt.Assert(t, found, qt.IsFalse)

		// Verify user roles in communities
		// Get the owner's profile
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		var profileResp struct {
			Data api.User `json:"data"`
		}
		err = json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)

		// Find the community in the owner's communities
		found = false
		for _, comm := range profileResp.Data.Communities {
			if comm.ID == communityID {
				found = true
				qt.Assert(t, string(comm.Role), qt.Equals, "owner")
				break
			}
		}
		qt.Assert(t, found, qt.IsTrue)
	})

	t.Run("Modified Endpoints", func(t *testing.T) {
		// Create a community for testing
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Modified Endpoints Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var createResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		communityID := createResp.Data.ID

		// Test GET profile/pendings with pending invites count
		// Invite the member to the community
		_, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", communityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)

		// Get pending counts for the member
		resp, code = c.Request(http.MethodGet, memberJWT, nil, "profile", "pendings")
		qt.Assert(t, code, qt.Equals, 200)
		var pendingsResp struct {
			Data api.PendingActionsResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &pendingsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, pendingsResp.Data.PendingInvitesCount > 0, qt.IsTrue)

		// Test GET users/ with username search filter
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "users?username=member")
		qt.Assert(t, code, qt.Equals, 200)
		var usersResp struct {
			Data struct {
				Users []api.User `json:"users"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data.Users) > 0, qt.IsTrue)

		// Verify at least one user with "member" in the name is found
		found := false
		for _, user := range usersResp.Data.Users {
			if user.ID == memberID {
				found = true
				break
			}
		}
		qt.Assert(t, found, qt.IsTrue)

		// Test tool response with communities array
		// Create a tool
		toolID := c.CreateTool(ownerJWT, "Community Tool")

		// Add the tool to the community
		_, code = c.Request(http.MethodPut, ownerJWT,
			map[string]interface{}{
				"communities": []string{communityID},
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get the tool and verify it has the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200)
		var toolResp struct {
			Data api.Tool `json:"data"`
		}
		err = json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(toolResp.Data.Communities), qt.Equals, 1)
		qt.Assert(t, toolResp.Data.Communities[0], qt.Equals, communityID)

		// Test PUT/POST tools with communities array
		// Create a new tool with community
		resp, code = c.Request(http.MethodPost, ownerJWT,
			map[string]interface{}{
				"title":          "Another Community Tool",
				"description":    "Test tool with community",
				"mayBeFree":      true,
				"askWithFee":     false,
				"cost":           10,
				"toolCategory":   1,
				"estimatedValue": 20,
				"height":         30,
				"weight":         40,
				"communities":    []string{communityID},
				"location": map[string]interface{}{
					"latitude":  41695384,
					"longitude": 2492793,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)
		var newToolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &newToolResp)
		qt.Assert(t, err, qt.IsNil)
		newToolID := newToolResp.Data.ID

		// Get the new tool and verify it has the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(newToolID))
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(toolResp.Data.Communities), qt.Equals, 1)
		qt.Assert(t, toolResp.Data.Communities[0], qt.Equals, communityID)

		// Test user/profile response with communities attribute
		// Get the owner's profile
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		var profileResp struct {
			Data api.User `json:"data"`
		}
		err = json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify the owner has communities in their profile
		qt.Assert(t, len(profileResp.Data.Communities) > 0, qt.IsTrue)

		// Find the test community in the owner's communities
		found = false
		for _, comm := range profileResp.Data.Communities {
			if comm.ID == communityID {
				found = true
				qt.Assert(t, comm.Role, qt.Equals, "owner")
				break
			}
		}
		qt.Assert(t, found, qt.IsTrue)
	})
}
