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

// Helper function to invite and accept a user into a community
func inviteAndAcceptMember(c *utils.TestService, ownerJWT, memberJWT, communityID, memberID string, t *testing.T) {
	// Send invite
	_, code := c.Request(http.MethodPost, ownerJWT, nil, "communities", communityID, "members", memberID)
	qt.Assert(t, code, qt.Equals, 200)

	// Get pending invites
	resp, code := c.Request(http.MethodGet, memberJWT, nil, "communities", "invites")
	qt.Assert(t, code, qt.Equals, 200)

	var invitesResp struct {
		Data []api.CommunityInviteResponse `json:"data"`
	}
	err := json.Unmarshal(resp, &invitesResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(invitesResp.Data) > 0, qt.IsTrue)

	// Find the invite for this community
	var inviteID string
	for _, invite := range invitesResp.Data {
		if invite.CommunityID == communityID {
			inviteID = invite.ID
			break
		}
	}
	qt.Assert(t, inviteID != "", qt.IsTrue, qt.Commentf("Should find invite for community"))

	// Accept the invitation
	_, code = c.Request(http.MethodPut, memberJWT,
		map[string]interface{}{"status": "ACCEPTED"},
		"communities", "invites", inviteID)
	qt.Assert(t, code, qt.Equals, 200)
}

// TestToolCommunityAccessControl tests that tools belonging to communities
// are properly hidden from users who are not members of those communities
func TestToolCommunityAccessControl(t *testing.T) {
	c := utils.NewTestService(t)

	// Create test users
	ownerJWT, ownerID := c.RegisterAndLoginWithID("tool-owner@test.com", "toolowner", "ownerpass")
	memberJWT, memberID := c.RegisterAndLoginWithID("community-member@test.com", "member", "memberpass")
	nonMemberJWT, _ := c.RegisterAndLoginWithID("non-member@test.com", "nonmember", "nonmemberpass")
	otherOwnerJWT, _ := c.RegisterAndLoginWithID("other-owner@test.com", "otherowner", "otherpass")

	t.Run("Single Tool Access Control", func(t *testing.T) {
		// Create a community
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{Name: "Tool Community"},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var communityResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &communityResp)
		qt.Assert(t, err, qt.IsNil)
		communityID := communityResp.Data.ID

		// Invite and accept member
		inviteAndAcceptMember(c, ownerJWT, memberJWT, communityID, memberID, t)

		// Create a public tool (no community)
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Public Tool",
				Description:   "A public tool",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var publicToolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &publicToolResp)
		qt.Assert(t, err, qt.IsNil)
		publicToolID := publicToolResp.Data.ID

		// Create a community tool
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Community Tool",
				Description:   "A community tool",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
				Communities: []string{communityID},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var communityToolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &communityToolResp)
		qt.Assert(t, err, qt.IsNil)
		communityToolID := communityToolResp.Data.ID

		// Test 1: Public tool visible to everyone
		_, code = c.Request(http.MethodGet, nonMemberJWT, nil, "tools", fmt.Sprint(publicToolID))
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Non-member should see public tool"))

		_, code = c.Request(http.MethodGet, memberJWT, nil, "tools", fmt.Sprint(publicToolID))
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Member should see public tool"))

		// Test 2: Community tool NOT visible to non-member (404)
		_, code = c.Request(http.MethodGet, nonMemberJWT, nil, "tools", fmt.Sprint(communityToolID))
		qt.Assert(t, code, qt.Equals, 404, qt.Commentf("Non-member should get 404 for community tool"))

		// Test 3: Community tool visible to member
		_, code = c.Request(http.MethodGet, memberJWT, nil, "tools", fmt.Sprint(communityToolID))
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Member should see community tool"))

		// Test 4: Community tool visible to owner (even if not explicitly member)
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(communityToolID))
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Owner should always see their own tool"))
	})

	t.Run("Tool Owner Access", func(t *testing.T) {
		// Create a tool and assign it to a community the owner is NOT in
		// First create a second community with a different owner
		resp, code := c.Request(http.MethodPost, otherOwnerJWT,
			api.CreateCommunityRequest{Name: "Another Community"},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var communityResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &communityResp)
		qt.Assert(t, err, qt.IsNil)
		anotherCommunityID := communityResp.Data.ID

		// Owner creates a tool
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Owner's Tool",
				Description:   "Tool owned by owner",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		toolID := toolResp.Data.ID

		// Edit tool to add it to a community the owner is NOT a member of
		_, code = c.Request(http.MethodPut, ownerJWT,
			api.Tool{
				Communities: []string{anotherCommunityID},
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Owner should STILL be able to view their own tool
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Owner should always access their own tool"))

		// Non-member of the community should NOT see it
		_, code = c.Request(http.MethodGet, nonMemberJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 404, qt.Commentf("Non-member should not see community tool"))
	})

	t.Run("Tool Search Filtering", func(t *testing.T) {
		// Create a community
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{Name: "Search Test Community"},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var communityResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &communityResp)
		qt.Assert(t, err, qt.IsNil)
		searchCommunityID := communityResp.Data.ID

		// Invite member
		inviteAndAcceptMember(c, ownerJWT, memberJWT, searchCommunityID, memberID, t)

		// Create public and community tools with searchable titles
		_, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "SearchPublic Hammer",
				Description:   "Public hammer",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				IsAvailable:   boolPtr(true),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		_, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "SearchCommunity Drill",
				Description:   "Community drill",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				IsAvailable:   boolPtr(true),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
				Communities: []string{searchCommunityID},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Member searches - should see both tools
		resp, code = c.Request(http.MethodGet, memberJWT, nil, "tools/search?term=Search")
		qt.Assert(t, code, qt.Equals, 200)

		var memberSearchResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &memberSearchResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(memberSearchResp.Data.Tools) >= 2, qt.IsTrue,
			qt.Commentf("Member should see both public and community tools"))

		// Non-member searches - should only see public tool
		resp, code = c.Request(http.MethodGet, nonMemberJWT, nil, "tools/search?term=Search")
		qt.Assert(t, code, qt.Equals, 200)

		var nonMemberSearchResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &nonMemberSearchResp)
		qt.Assert(t, err, qt.IsNil)

		// Count how many search results match our test tools
		communityToolCount := 0
		for _, tool := range nonMemberSearchResp.Data.Tools {
			if tool.Title == "SearchCommunity Drill" {
				communityToolCount++
			}
		}
		qt.Assert(t, communityToolCount, qt.Equals, 0,
			qt.Commentf("Non-member should not see community tool in search"))
	})

	t.Run("User Tools List Filtering", func(t *testing.T) {
		// Create a new community
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{Name: "User Tools Community"},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var communityResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &communityResp)
		qt.Assert(t, err, qt.IsNil)
		userToolsCommunityID := communityResp.Data.ID

		// Member joins community
		inviteAndAcceptMember(c, ownerJWT, memberJWT, userToolsCommunityID, memberID, t)

		// Owner creates one public and one community tool
		_, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Owner Public Tool",
				Description:   "Public",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		_, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Owner Community Tool",
				Description:   "Community only",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
				Communities: []string{userToolsCommunityID},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Member views owner's tools - should see both
		resp, code = c.Request(http.MethodGet, memberJWT, nil, "tools/user", ownerID)
		qt.Assert(t, code, qt.Equals, 200)

		var memberViewResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &memberViewResp)
		qt.Assert(t, err, qt.IsNil)

		ownerPublicCount := 0
		ownerCommunityCount := 0
		for _, tool := range memberViewResp.Data.Tools {
			if tool.Title == "Owner Public Tool" {
				ownerPublicCount++
			}
			if tool.Title == "Owner Community Tool" {
				ownerCommunityCount++
			}
		}
		qt.Assert(t, ownerPublicCount, qt.Equals, 1, qt.Commentf("Member should see public tool"))
		qt.Assert(t, ownerCommunityCount, qt.Equals, 1, qt.Commentf("Member should see community tool"))

		// Non-member views owner's tools - should only see public tool
		resp, code = c.Request(http.MethodGet, nonMemberJWT, nil, "tools/user", ownerID)
		qt.Assert(t, code, qt.Equals, 200)

		var nonMemberViewResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &nonMemberViewResp)
		qt.Assert(t, err, qt.IsNil)

		nonMemberPublicCount := 0
		nonMemberCommunityCount := 0
		for _, tool := range nonMemberViewResp.Data.Tools {
			if tool.Title == "Owner Public Tool" {
				nonMemberPublicCount++
			}
			if tool.Title == "Owner Community Tool" {
				nonMemberCommunityCount++
			}
		}
		qt.Assert(t, nonMemberPublicCount, qt.Equals, 1, qt.Commentf("Non-member should see public tool"))
		qt.Assert(t, nonMemberCommunityCount, qt.Equals, 0, qt.Commentf("Non-member should NOT see community tool"))

		// Owner views own tools - should see ALL tools
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools")
		qt.Assert(t, code, qt.Equals, 200)

		var ownerViewResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &ownerViewResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(ownerViewResp.Data.Tools) > 0, qt.IsTrue,
			qt.Commentf("Owner should see all their own tools"))
	})

	t.Run("Tool Ratings Access Control", func(t *testing.T) {
		// Create community and tool
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{Name: "Ratings Community"},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var communityResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &communityResp)
		qt.Assert(t, err, qt.IsNil)
		ratingsCommunityID := communityResp.Data.ID

		// Add member to community
		inviteAndAcceptMember(c, ownerJWT, memberJWT, ratingsCommunityID, memberID, t)

		// Create community tool
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Ratings Test Tool",
				Description:   "Test ratings",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
				Communities: []string{ratingsCommunityID},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		ratingsToolID := toolResp.Data.ID

		// Non-member tries to view ratings - should get 404
		_, code = c.Request(http.MethodGet, nonMemberJWT, nil, "tools", fmt.Sprint(ratingsToolID), "ratings")
		qt.Assert(t, code, qt.Equals, 404, qt.Commentf("Non-member should not access ratings of community tool"))

		// Member views ratings - should succeed
		_, code = c.Request(http.MethodGet, memberJWT, nil, "tools", fmt.Sprint(ratingsToolID), "ratings")
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Member should access ratings"))

		// Owner views ratings - should succeed
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(ratingsToolID), "ratings")
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Owner should access ratings"))
	})

	t.Run("Tool History Access Control", func(t *testing.T) {
		// Create community and nomadic tool
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{Name: "History Community"},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var communityResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &communityResp)
		qt.Assert(t, err, qt.IsNil)
		historyCommunityID := communityResp.Data.ID

		// Add member
		inviteAndAcceptMember(c, ownerJWT, memberJWT, historyCommunityID, memberID, t)

		// Create nomadic community tool
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Nomadic History Tool",
				Description:   "Test history",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				IsNomadic:     boolPtr(true),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
				Communities: []string{historyCommunityID},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		historyToolID := toolResp.Data.ID

		// Non-member tries to view history - should get 404
		_, code = c.Request(http.MethodGet, nonMemberJWT, nil, "tools", fmt.Sprint(historyToolID), "history")
		qt.Assert(t, code, qt.Equals, 404, qt.Commentf("Non-member should not access history"))

		// Member views history - should succeed
		_, code = c.Request(http.MethodGet, memberJWT, nil, "tools", fmt.Sprint(historyToolID), "history")
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Member should access history"))

		// Owner views history - should succeed
		_, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(historyToolID), "history")
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Owner should access history"))
	})

	t.Run("Multiple Communities Access", func(t *testing.T) {
		// Create two communities
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{Name: "Multi Community A"},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var communityAResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &communityAResp)
		qt.Assert(t, err, qt.IsNil)
		communityAID := communityAResp.Data.ID

		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{Name: "Multi Community B"},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var communityBResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &communityBResp)
		qt.Assert(t, err, qt.IsNil)
		communityBID := communityBResp.Data.ID

		// Member joins only community A
		inviteAndAcceptMember(c, ownerJWT, memberJWT, communityAID, memberID, t)

		// Create a tool in both communities
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Multi Community Tool",
				Description:   "In both communities",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
				Communities: []string{communityAID, communityBID},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		multiToolID := toolResp.Data.ID

		// Member should see it (member of at least one community)
		_, code = c.Request(http.MethodGet, memberJWT, nil, "tools", fmt.Sprint(multiToolID))
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Member of one community should see tool"))

		// Non-member should NOT see it
		_, code = c.Request(http.MethodGet, nonMemberJWT, nil, "tools", fmt.Sprint(multiToolID))
		qt.Assert(t, code, qt.Equals, 404, qt.Commentf("Non-member should not see tool"))
	})

	t.Run("Leave Community Loses Access", func(t *testing.T) {
		// Create community
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{Name: "Leave Test Community"},
			"communities",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var communityResp struct {
			Data api.CommunityResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &communityResp)
		qt.Assert(t, err, qt.IsNil)
		leaveCommunityID := communityResp.Data.ID

		// Member joins
		inviteAndAcceptMember(c, ownerJWT, memberJWT, leaveCommunityID, memberID, t)

		// Create community tool
		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Leave Test Tool",
				Description:   "Test leaving",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location: api.Location{
					Latitude:  41650000,
					Longitude: 2430000,
				},
				Communities: []string{leaveCommunityID},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		leaveToolID := toolResp.Data.ID

		// Member can see tool
		_, code = c.Request(http.MethodGet, memberJWT, nil, "tools", fmt.Sprint(leaveToolID))
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Member should see tool before leaving"))

		// Member leaves community
		_, code = c.Request(http.MethodDelete, memberJWT, nil, "communities", leaveCommunityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)

		// Member should no longer see tool
		_, code = c.Request(http.MethodGet, memberJWT, nil, "tools", fmt.Sprint(leaveToolID))
		qt.Assert(t, code, qt.Equals, 404, qt.Commentf("Member should not see tool after leaving community"))
	})
}
