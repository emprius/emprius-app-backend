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
		qt.Assert(t, getResp.Data.MembersCount, qt.Equals, int64(1)) // Only the owner should be in the community initially

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
		qt.Assert(t, code, qt.Equals, 403)

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
		qt.Assert(t, code, qt.Equals, 403)

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

	t.Run("Community Communities", func(t *testing.T) {
		// Create a community for testing
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Communities Test Community",
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
			Data api.PaginatedCommunityUserResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data.Users), qt.Equals, 1) // Only the owner should be in the community initially

		// Verify the owner is in the community with the correct role
		data := usersResp.Data.Users[0]
		qt.Assert(t, data.ID, qt.Equals, ownerID)
		qt.Assert(t, string(usersResp.Data.Users[0].Role), qt.Equals, "owner")

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
		qt.Assert(t, len(usersResp.Data.Users), qt.Equals, 2) // Owner and member

		// Test pagination with page parameter
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "members?page=0")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(usersResp.Data.Users), qt.Equals, 2) // Should return all users since we have less than page size
	})

	t.Run("Community Tools", func(t *testing.T) {
		// Create a community for testing
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Tools Test Community",
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

		// Verify the initial tool count is 0
		qt.Assert(t, createResp.Data.ToolsCount, qt.Equals, int64(0))

		// Create a tool
		toolID := c.CreateTool(ownerJWT, "Community Tool")

		// Test getting community tools without auth
		_, code = c.Request(http.MethodGet, "", nil, "communities", communityID, "tools")
		qt.Assert(t, code, qt.Equals, 401)

		// Test getting community tools with auth (should be empty initially)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "tools")
		qt.Assert(t, code, qt.Equals, 200)

		var toolsResp struct {
			Data struct {
				Tools []*api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(toolsResp.Data.Tools), qt.Equals, 0) // No tools in the community initially

		// Add the tool to the community
		_, code = c.Request(http.MethodPut, ownerJWT,
			map[string]interface{}{
				"communities": []string{communityID},
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the tool is now in the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "tools")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(toolsResp.Data.Tools), qt.Equals, 1) // One tool in the community now
		qt.Assert(t, toolsResp.Data.Tools[0].ID, qt.Equals, toolID)

		// Verify the tool count is updated in the community response
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID)
		qt.Assert(t, code, qt.Equals, 200)
		var communityResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &communityResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, communityResp.Data.ToolsCount, qt.Equals, int64(1)) // Tool count should be 1

		// Create a second tool
		tool2ID := c.CreateTool(ownerJWT, "Another Community Tool")

		// Add the second tool to the community
		_, code = c.Request(http.MethodPut, ownerJWT,
			map[string]interface{}{
				"communities": []string{communityID},
			},
			"tools", fmt.Sprint(tool2ID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify both tools are in the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "tools")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(toolsResp.Data.Tools), qt.Equals, 2) // Two tools in the community now

		// Verify the tool count is updated to 2 in the community response
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &communityResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, communityResp.Data.ToolsCount, qt.Equals, int64(2)) // Tool count should be 2

		// Test with non-existent community ID
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", "507f1f77bcf86cd799439011", "tools")
		qt.Assert(t, code, qt.Equals, 403)

		// Test with invalid community ID
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", "invalid-id", "tools")
		qt.Assert(t, code, qt.Equals, 400)

		// Remove a tool from the community
		_, code = c.Request(http.MethodPut, ownerJWT,
			map[string]interface{}{
				"communities": []string{},
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify only one tool remains in the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "tools")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(toolsResp.Data.Tools), qt.Equals, 1) // One tool in the community now
		qt.Assert(t, toolsResp.Data.Tools[0].ID, qt.Equals, tool2ID)
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

		// Test user trying to invite themselves (should fail)
		_, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", communityID, "members", ownerID)
		qt.Assert(t, code, qt.Equals, 400) // Bad request - users cannot invite themselves

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
		qt.Assert(t, inviteResp.Data.Status, qt.Equals, "PENDING")
		// Verify community information is included
		qt.Assert(t, inviteResp.Data.Community.Name, qt.Not(qt.Equals), "")
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
		// Verify community information is included in the invite response
		qt.Assert(t, invitesResp.Data[0].Community.Name, qt.Not(qt.Equals), "")

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
			Data api.PaginatedCommunityUserResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)

		// Find the non-member in the users list
		var found bool
		for _, user := range usersResp.Data.Users {
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
		for _, user := range usersResp.Data.Users {
			if user.ID == memberID {
				found = true
				break
			}
		}
		qt.Assert(t, found, qt.IsFalse)

		// Create another community for testing invite cancellation
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Cancellation Test Community",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		cancelCommunityID := createResp.Data.ID

		// Invite the member to the new community
		resp, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", cancelCommunityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &inviteResp)
		qt.Assert(t, err, qt.IsNil)
		cancelInviteID := inviteResp.Data.ID

		// Test canceling an invite without auth
		_, code = c.Request(http.MethodPut, "",
			map[string]interface{}{
				"status": "CANCELED",
			},
			"communities", "invites", cancelInviteID)
		qt.Assert(t, code, qt.Equals, 401)

		// Test canceling an invite with wrong user (non-inviter)
		_, code = c.Request(http.MethodPut, memberJWT,
			map[string]interface{}{
				"status": "CANCELED",
			},
			"communities", "invites", cancelInviteID)
		qt.Assert(t, code, qt.Equals, 500) // Internal server error when invite not found for this user as inviter

		// Test canceling an invite with correct user (inviter)
		_, code = c.Request(http.MethodPut, ownerJWT,
			map[string]interface{}{
				"status": "CANCELED",
			},
			"communities", "invites", cancelInviteID)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the invite is canceled by checking the member is not in the community
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", cancelCommunityID, "members")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)

		// Ensure the member is not in the community
		found = false
		for _, user := range usersResp.Data.Users {
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
			Data api.PaginatedCommunityUserResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)

		// Ensure the member is not in the community
		found := false
		for _, user := range usersResp.Data.Users {
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

	t.Run("User Communities", func(t *testing.T) {
		// Create communities for testing
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "User Communities Test 1",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var createResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		community1ID := createResp.Data.ID

		// Create a second community
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "User Communities Test 2",
			},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		community2ID := createResp.Data.ID

		// Invite member to first community
		_, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", community1ID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)

		// Get pending invites for the member
		resp, code = c.Request(http.MethodGet, memberJWT, nil, "communities", "invites")
		qt.Assert(t, code, qt.Equals, 200)

		var invitesResp struct {
			Data []api.CommunityInviteResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &invitesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(invitesResp.Data) > 0, qt.IsTrue)

		// Find the invite for the first community
		var inviteID string
		for _, invite := range invitesResp.Data {
			if invite.CommunityID == community1ID {
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

		// Test getting user communities without auth
		_, code = c.Request(http.MethodGet, "", nil, "users", ownerID, "communities")
		qt.Assert(t, code, qt.Equals, 401)

		// Test getting owner's communities
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "users", ownerID, "communities")
		qt.Assert(t, code, qt.Equals, 200)

		var communitiesResp struct {
			Data api.PaginatedCommunityResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &communitiesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(communitiesResp.Data.Communities) >= 2, qt.IsTrue) // Owner should be in at least the two test communities

		// Verify the communities include the test communities
		foundCommunity1 := false
		foundCommunity2 := false
		for _, community := range communitiesResp.Data.Communities {
			if community.ID == community1ID {
				foundCommunity1 = true
				// Verify the member count is at least 2 (owner and member)
				qt.Assert(t, community.MembersCount >= 2, qt.IsTrue)
			}
			if community.ID == community2ID {
				foundCommunity2 = true
				// Verify the member count is at least 1 (just the owner)
				qt.Assert(t, community.MembersCount >= 1, qt.IsTrue)
			}
		}
		qt.Assert(t, foundCommunity1, qt.IsTrue)
		qt.Assert(t, foundCommunity2, qt.IsTrue)

		// Test getting member's communities
		resp, code = c.Request(http.MethodGet, memberJWT, nil, "users", memberID, "communities")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &communitiesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(communitiesResp.Data.Communities) >= 1, qt.IsTrue) // Member should be in at least the first test community

		// Verify the member is in the first community but not the second
		foundCommunity1 = false
		foundCommunity2 = false
		for _, community := range communitiesResp.Data.Communities {
			if community.ID == community1ID {
				foundCommunity1 = true
			}
			if community.ID == community2ID {
				foundCommunity2 = true
			}
		}
		qt.Assert(t, foundCommunity1, qt.IsTrue)
		qt.Assert(t, foundCommunity2, qt.IsFalse)

		// Test pagination
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "users", ownerID, "communities?page=0")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &communitiesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(communitiesResp.Data.Communities) > 0, qt.IsTrue)

		// Test non-existent user
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "users", "507f1f77bcf86cd799439011", "communities")
		qt.Assert(t, code, qt.Equals, 404)

		// Test invalid user ID
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "users", "invalid-id", "communities")
		qt.Assert(t, code, qt.Equals, 400)
	})

	t.Run("Community Membership Access Control", func(t *testing.T) {
		// Create a community for testing
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Access Control Test Community",
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

		// Invite and accept member to the community
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

		// Create a tool and add it to the community
		toolID := c.CreateTool(ownerJWT, "Access Control Test Tool")
		_, code = c.Request(http.MethodPut, ownerJWT,
			map[string]interface{}{
				"communities": []string{communityID},
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Test 7: Non-member cannot view community tools
		_, code = c.Request(http.MethodGet, nonMemberJWT, nil, "communities", communityID, "tools")
		qt.Assert(t, code, qt.Equals, 403, qt.Commentf("Non-member should not be able to view community tools"))

		// Test 8: Member CAN view community tools
		resp, code = c.Request(http.MethodGet, memberJWT, nil, "communities", communityID, "tools")
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Member should be able to view community tools"))

		var toolsResp struct {
			Data struct {
				Tools []*api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(toolsResp.Data.Tools), qt.Equals, 1)
		qt.Assert(t, toolsResp.Data.Tools[0].ID, qt.Equals, toolID)

		// Test 9: Owner CAN view community tools
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "communities", communityID, "tools")
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Owner should be able to view community tools"))

		// Test 10: Verify that after member leaves, they cannot access community anymore
		_, code = c.Request(http.MethodDelete, memberJWT, nil, "communities", communityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)

		// Member should no longer be able to view community tools
		_, code = c.Request(http.MethodGet, memberJWT, nil, "communities", communityID, "tools")
		qt.Assert(t, code, qt.Equals, 403, qt.Commentf("Former member should not be able to view community tools after leaving"))
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
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "users?term=member")
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
				"title":         "Another Community Tool",
				"description":   "Test tool with community",
				"mayBeFree":     true,
				"askWithFee":    false,
				"cost":          10,
				"toolCategory":  1,
				"toolValuation": 20,
				"height":        30,
				"weight":        40,
				"communities":   []string{communityID},
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
