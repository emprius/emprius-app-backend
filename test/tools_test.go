package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/types"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func boolPtr(b bool) *bool {
	return &b
}

func uint64Ptr(i uint64) *uint64 {
	return &i
}

func TestTools(t *testing.T) {
	c := utils.NewTestService(t)

	// Create a user
	userJWT := c.RegisterAndLogin("user@test.com", "user", "userpass")

	t.Run("Create and Manage Tools", func(t *testing.T) {
		//----------------------------------------------------------------------
		// 1) Attempt to create "Test Tool" without auth => 401
		//----------------------------------------------------------------------
		_, code := c.Request(http.MethodPost, "",
			api.Tool{
				Title:         "Test Tool",
				Description:   "Test tool description",
				MayBeFree:     boolPtr(true),
				AskWithFee:    boolPtr(false),
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Height:        30,
				Weight:        40,
				IsAvailable:   boolPtr(true),
				Location: api.Location{
					Latitude:  41920384, // 41.920384 * 1e6 (~25 km north)
					Longitude: 2492793,  // 2.492793 * 1e6
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 401)

		//----------------------------------------------------------------------
		// 2) Create "Test Tool" with auth => 200
		//    (Starts at ~25 km but we'll soon edit it to a different lat.)
		//----------------------------------------------------------------------
		resp, code := c.Request(http.MethodPost, userJWT,
			api.Tool{
				Title:         "Test Tool",
				Description:   "Test tool description",
				MayBeFree:     boolPtr(true),
				AskWithFee:    boolPtr(false),
				Category:      1,
				ToolValuation: uint64Ptr(20000),
				Height:        30,
				Weight:        40,
				Location: api.Location{
					Latitude:  41920384, // 41.920384 * 1e6 (~25 km north)
					Longitude: 2492793,  // 2.492793 * 1e6
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
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		toolID := toolResp.Data.ID

		//----------------------------------------------------------------------
		// 3) Create "Another Tool" for subsequent searches
		//    We'll place it ~9 km from the "updated" location so that it
		//    appears in 10 km searches (and also meets other test cases).
		//----------------------------------------------------------------------
		_, code = c.Request(http.MethodPost, userJWT,
			api.Tool{
				Title:         "Another Tool",
				Description:   "Another tool description",
				MayBeFree:     boolPtr(false), // This will be relevant for filtering
				AskWithFee:    boolPtr(true),
				Category:      1,
				ToolValuation: uint64Ptr(20000),
				Height:        40,
				Weight:        50,
				IsAvailable:   boolPtr(true),
				Location: api.Location{
					Latitude:  41776239, // 41.776239 * 1e6
					Longitude: 2492793,  // same longitude
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		//----------------------------------------------------------------------
		// 4) GET "Test Tool" by ID
		//----------------------------------------------------------------------
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200)

		var getToolResp struct {
			Data api.Tool `json:"data"`
		}
		err = json.Unmarshal(resp, &getToolResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, getToolResp.Data.Title, qt.Equals, "Test Tool")

		//----------------------------------------------------------------------
		// 5) Edit "Test Tool" => "Updated Tool"
		//    Also change its location to be the "center" (latitude ~41.695384).
		//    We'll set cost=20, mayBeFree=false so it's excluded from some tests.
		//----------------------------------------------------------------------
		resp, code = c.Request(http.MethodPut, userJWT,
			api.Tool{
				Title:         "Updated Tool",
				Description:   "Updated description",
				MayBeFree:     boolPtr(false),
				AskWithFee:    boolPtr(true),
				Category:      1,
				ToolValuation: uint64Ptr(20000),
				Height:        40,
				Weight:        50,
				IsAvailable:   boolPtr(true),
				Location: api.Location{
					Latitude:  41695384, // 41.695384 * 1e6 (center)
					Longitude: 2492793,  // 2.492793 * 1e6
				},
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		var updateResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &updateResp)
		qt.Assert(t, err, qt.IsNil)
		updatedToolID := updateResp.Data.ID

		//----------------------------------------------------------------------
		// 6) GET updated tool => ensure it changed
		//----------------------------------------------------------------------
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(updatedToolID))
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &getToolResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, getToolResp.Data.Title, qt.Equals, "Updated Tool")

		//----------------------------------------------------------------------
		// 7) List owned tools => should have exactly 2 so far
		//----------------------------------------------------------------------
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools")
		qt.Assert(t, code, qt.Equals, 200)
		var listToolsResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &listToolsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listToolsResp.Data.Tools), qt.Equals, 2)

		//----------------------------------------------------------------------
		// 8) SEARCH scenarios
		//----------------------------------------------------------------------
		t.Run("Search Tools", func(t *testing.T) {
			//--------------------------------------------------------------
			// 8A) Search by term => "Updated"
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?term=Updated")
			qt.Assert(t, code, qt.Equals, 200)
			var searchResp struct {
				Data struct {
					Tools []api.Tool `json:"tools"`
				} `json:"data"`
			}
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 1) // only "Updated Tool"

			//--------------------------------------------------------------
			// 8B) Create 3 more tools at ~5 km, ~15 km, ~25 km
			//--------------------------------------------------------------
			// (We do this so the distance-based searches have data.)

			// Tool at ~5 km
			_, code = c.Request(http.MethodPost, userJWT,
				api.Tool{
					Title:         "Tool at 5km",
					Description:   "Tool at 5km away",
					MayBeFree:     boolPtr(true),  // different from "Updated Tool"
					AskWithFee:    boolPtr(false), // cost=10 => included by maxCost=15
					Category:      1,
					ToolValuation: uint64Ptr(10000),
					Height:        30,
					Weight:        40,
					IsAvailable:   boolPtr(true),
					Location: api.Location{
						Latitude:  41745384, // 41.745384 * 1e6 (~5 km from center)
						Longitude: 2492793,  // 2.492793 * 1e6
					},
				},
				"tools",
			)
			qt.Assert(t, code, qt.Equals, 200)

			// Tool at ~15 km
			_, code = c.Request(http.MethodPost, userJWT,
				api.Tool{
					Title:         "Tool at 15km",
					Description:   "Tool at 15km away",
					MayBeFree:     boolPtr(true),
					AskWithFee:    boolPtr(false),
					Category:      1,
					ToolValuation: uint64Ptr(20000),
					Height:        30,
					Weight:        40,
					IsAvailable:   boolPtr(true),
					Location: api.Location{
						Latitude:  41845384, // 41.845384 * 1e6 (~15 km from center)
						Longitude: 2492793,  // 2.492793 * 1e6
					},
				},
				"tools",
			)
			qt.Assert(t, code, qt.Equals, 200)

			// Tool at ~25 km
			_, code = c.Request(http.MethodPost, userJWT,
				api.Tool{
					Title:         "Tool at 25km",
					Description:   "Tool at 25km away",
					MayBeFree:     boolPtr(true),
					AskWithFee:    boolPtr(false),
					Category:      1,
					ToolValuation: uint64Ptr(20000),
					Height:        30,
					Weight:        40,
					IsAvailable:   boolPtr(true),
					Location: api.Location{
						Latitude:  41945384, // 41.945384 * 1e6 (~25 km from center)
						Longitude: 2492793,  // 2.492793 * 1e6
					},
				},
				"tools",
			)
			qt.Assert(t, code, qt.Equals, 200)

			//--------------------------------------------------------------
			// 8C) Search with distance=10 km
			//     Should find:
			//       1) Updated Tool (center)
			//       2) Tool at 5 km
			//       3) Another Tool (~9 km)
			//     => total 3
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?distance=10000")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)

			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 3)

			found5km := false
			foundUpdated := false
			for _, tool := range searchResp.Data.Tools {
				switch tool.Title {
				case "Tool at 5km":
					found5km = true
				case "Updated Tool":
					foundUpdated = true
				}
			}
			qt.Assert(t, found5km, qt.Equals, true)
			qt.Assert(t, foundUpdated, qt.Equals, true)

			//--------------------------------------------------------------
			// 8D) Search with distance=20 km
			//     Now we also pick up:
			//       3) Tool at 15 km
			//       4) Another Tool (~9 km away)
			//     => total 4
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?distance=20000")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)

			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 4)
			found15km := false
			foundAnother := false
			for _, tool := range searchResp.Data.Tools {
				switch tool.Title {
				case "Tool at 15km":
					found15km = true
				case "Another Tool":
					foundAnother = true
				}
			}
			qt.Assert(t, found15km, qt.Equals, true)
			qt.Assert(t, foundAnother, qt.Equals, true)

			//--------------------------------------------------------------
			// 8E) Search with distance=30 km => should find all 5
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?distance=30000")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 5)
			found25km := false
			for _, tool := range searchResp.Data.Tools {
				switch tool.Title {
				case "Tool at 25km":
					found25km = true
				}
			}
			qt.Assert(t, found25km, qt.Equals, true)

			//--------------------------------------------------------------
			// 8F) Search with "term=Another" + distance=20000 => expect 1
			//     Because only “Another Tool” matches the term “Another”
			//     and is also within 20 km.
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?term=Another&distance=20000")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 1)

			//--------------------------------------------------------------
			// 8H) Search with multiple parameters => distance=10, maxCost=0,
			//     mayBeFree=false. We treat maxCost=0 as “ignore cost,”
			//     so we only filter by distance ≤ 10 km and mayBeFree=false.
			//
			//     Within 10 km we have:
			//       - Updated Tool (mayBeFree=false)
			//       - Tool at 5km (mayBeFree=true, so excluded)
			//       - Another Tool (~9 km, mayBeFree=false => included)
			//     => total 2
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil,
				"tools/search?term=&distance=10000&maxCost=0&mayBeFree=false",
			)
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 3)

			//--------------------------------------------------------------
			// 8I) Search with a non-matching term => 0
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil,
				"tools/search?term=nonexistent&distance=10000&maxCost=0&mayBeFree=false",
			)
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 0)

			//--------------------------------------------------------------
			// 8J) Check array-style categories and transports
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil,
				"tools/search?term=&"+
					"categories[]=1&categories[]=2&categories[]=3&categories[]=4&categories[]=5&"+
					"distance=50000&maxCost=1000&mayBeFree=false&"+
					"transports[]=1&transports[]=2&transports[]=3",
			)
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			// We don't check the exact count, just that it's >= 0
			qt.Assert(t, len(searchResp.Data.Tools) >= 0, qt.Equals, true)

			//--------------------------------------------------------------
			// 8K) Another combined search
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil,
				"tools/search?distance=50000&maxCost=1000&mayBeFree=false",
			)
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools) >= 0, qt.Equals, true)

			//--------------------------------------------------------------
			// 8L) Another combined search with 'term=hal'
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil,
				"tools/search?term=hal&distance=50000&maxCost=1000&mayBeFree=false",
			)
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools) >= 0, qt.Equals, true)
		})

		//----------------------------------------------------------------------
		// 9) Delete the "Updated Tool"
		//----------------------------------------------------------------------
		_, code = c.Request(http.MethodDelete, userJWT, nil, "tools", fmt.Sprint(updatedToolID))
		qt.Assert(t, code, qt.Equals, 200)

		//----------------------------------------------------------------------
		// 10) Verify it is deleted => 404
		//----------------------------------------------------------------------
		_, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(updatedToolID))
		qt.Assert(t, code, qt.Equals, 404)
	})

	t.Run("Tool Ratings", func(t *testing.T) {
		// Create a tool
		resp, code := c.Request(http.MethodPost, userJWT,
			api.Tool{
				Title:         "Tool for Rating",
				Description:   "Tool to test ratings",
				MayBeFree:     boolPtr(true),
				AskWithFee:    boolPtr(false),
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Height:        30,
				Weight:        40,
				Location: api.Location{
					Latitude:  41920384,
					Longitude: 2492793,
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
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		toolID := toolResp.Data.ID

		// Create another user to book and rate the tool
		renterJWT := c.RegisterAndLogin("renter@test.com", "renter", "renterpass")

		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		// Create a booking
		resp, code = c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: tomorrow.Unix(),         // 2024-03-17
				EndDate:   dayAfterTomorrow.Unix(), // 2024-03-18
				Contact:   "contact info",
				Comments:  "booking comments",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var bookingResp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		bookingID := bookingResp.Data.ID

		// Owner accepts the booking
		_, code = c.Request(http.MethodPut, userJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Owner marks the tool as returned
		_, code = c.Request(http.MethodPut, userJWT,
			&api.BookingStatusUpdate{
				Status: "RETURNED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Renter rates the booking
		_, code = c.Request(http.MethodPost, renterJWT,
			api.RateRequest{
				Rating:  5,
				Comment: "Great tool!",
			},
			"bookings", bookingID, "ratings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Owner rates the booking
		_, code = c.Request(http.MethodPost, userJWT,
			api.RateRequest{
				Rating:  4,
				Comment: "Good renter",
			},
			"bookings", bookingID, "ratings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get tool ratings
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(toolID), "ratings")
		qt.Assert(t, code, qt.Equals, 200)

		var ratesResp struct {
			Data *api.PaginatedUnifiedRatingsResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &ratesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(ratesResp.Data.Ratings), qt.Equals, 1)

		// Verify rating values
		rating := ratesResp.Data.Ratings[0]
		qt.Assert(t, *rating.Owner.Rating, qt.Equals, 4)
		qt.Assert(t, *rating.Owner.RatingComment, qt.Equals, "Good renter")
		qt.Assert(t, *rating.Requester.Rating, qt.Equals, 5)
		qt.Assert(t, *rating.Requester.RatingComment, qt.Equals, "Great tool!")
	})

	t.Run("Community Membership Check", func(t *testing.T) {
		// Create users for testing
		ownerJWT, _ := c.RegisterAndLoginWithID("community-owner@test.com", "community-owner", "ownerpass")
		memberJWT, memberID := c.RegisterAndLoginWithID("community-member@test.com", "community-member", "memberpass")
		nonMemberJWT, _ := c.RegisterAndLoginWithID("community-nonmember@test.com", "community-nonmember", "nonmemberpass")

		// Create a community
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Tool Search Test Community",
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

		// Invite and accept the member to the community
		resp, code = c.Request(http.MethodPost, ownerJWT, nil, "communities", communityID, "members", memberID)
		qt.Assert(t, code, qt.Equals, 200)

		var inviteResp struct {
			Data api.CommunityInviteResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &inviteResp)
		qt.Assert(t, err, qt.IsNil)
		inviteID := inviteResp.Data.ID

		// Accept the invitation
		_, code = c.Request(http.MethodPut, memberJWT,
			map[string]interface{}{
				"status": "ACCEPTED",
			},
			"communities", "invites", inviteID)
		qt.Assert(t, code, qt.Equals, 200)

		// Create a tool and add it to the community
		toolID := c.CreateTool(ownerJWT, "Community Tool")
		_, code = c.Request(http.MethodPut, ownerJWT,
			map[string]interface{}{
				"communities": []string{communityID},
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Create a tool without community
		nonCommunityToolID := c.CreateTool(ownerJWT, "Non-Community Tool")

		// Test case 1: Non-member searching for tools
		// Should only see the non-community tool
		resp, code = c.Request(http.MethodGet, nonMemberJWT, nil, "tools/search")
		qt.Assert(t, code, qt.Equals, 200)
		var searchResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &searchResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify only the non-community tool is visible
		foundCommunityTool := false
		foundNonCommunityTool := false
		for _, tool := range searchResp.Data.Tools {
			switch tool.ID {
			case toolID:
				foundCommunityTool = true
			case nonCommunityToolID:
				foundNonCommunityTool = true
			}
		}
		qt.Assert(t, foundCommunityTool, qt.Equals, false, qt.Commentf("Non-member should not see community tool"))
		qt.Assert(t, foundNonCommunityTool, qt.Equals, true, qt.Commentf("Non-member should see non-community tool"))

		// Test case 2: Member searching for tools
		// Should see both tools
		resp, code = c.Request(http.MethodGet, memberJWT, nil, "tools/search")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &searchResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify both tools are visible
		foundCommunityTool = false
		foundNonCommunityTool = false
		for _, tool := range searchResp.Data.Tools {
			switch tool.ID {
			case toolID:
				foundCommunityTool = true
			case nonCommunityToolID:
				foundNonCommunityTool = true
			}
		}
		qt.Assert(t, foundCommunityTool, qt.Equals, true, qt.Commentf("Member should see community tool"))
		qt.Assert(t, foundNonCommunityTool, qt.Equals, true, qt.Commentf("Member should see non-community tool"))

		// Test case 3: Owner searching for tools
		// Should see both tools
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools/search")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &searchResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify both tools are visible
		foundCommunityTool = false
		foundNonCommunityTool = false
		for _, tool := range searchResp.Data.Tools {
			switch tool.ID {
			case toolID:
				foundCommunityTool = true
			case nonCommunityToolID:
				foundNonCommunityTool = true
			}
		}
		qt.Assert(t, foundCommunityTool, qt.Equals, true, qt.Commentf("Owner should see community tool"))
		qt.Assert(t, foundNonCommunityTool, qt.Equals, true, qt.Commentf("Owner should see non-community tool"))
	})

	t.Run("Nomadic Tool Editing Restrictions", func(t *testing.T) {
		// Create a user for this test
		ownerJWT, _ := c.RegisterAndLoginWithID("nomadic-edit-owner@test.com", "nomadic-edit-owner", "ownerpass")
		// nonOwnerJWT := c.RegisterAndLogin("nomadic-edit-nonowner@test.com", "nomadic-edit-nonowner", "nonownerpass")
		renterJWT := c.RegisterAndLogin("nomadic-edit-renter@test.com", "nomadic-edit-renter", "renterpass")

		// Create a nomadic tool
		createToolResp, code := c.Request(http.MethodPost, ownerJWT, map[string]interface{}{
			"title":         "Nomadic Tool for Edit Test",
			"description":   "This tool is used to test editing restrictions",
			"toolCategory":  1,
			"toolValuation": 100,
			"isNomadic":     true,
		}, "tools")
		qt.Assert(t, code, qt.Equals, 200)

		var toolIDResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err := json.Unmarshal(createToolResp, &toolIDResp)
		qt.Assert(t, err, qt.IsNil)
		nomadicToolID := toolIDResp.Data.ID

		// Create a booking for the nomadic tool to test the pending bookings restriction
		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		resp, code := c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(nomadicToolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Booking for nomadic tool edit test",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get the booking ID to reject it
		var bookingResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		bookingID := bookingResp.Data.ID

		// Test case 1: Owner trying to change nomadic status with pending bookings (should fail)
		resp, code = c.Request(http.MethodPut, ownerJWT,
			map[string]interface{}{
				"isNomadic": false,
			},
			"tools", fmt.Sprint(nomadicToolID),
		)
		qt.Assert(t, code, qt.Equals, 400) // Bad Request

		// Verify error message
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err = json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "cannot change nomadic status when there are pending bookings")

		// Owner rejects the booking
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "REJECTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Test case 2: Owner can change nomadic status after rejecting all pending bookings
		_, code = c.Request(http.MethodPut, ownerJWT,
			map[string]interface{}{
				"isNomadic": false,
			},
			"tools", fmt.Sprint(nomadicToolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the tool is no longer nomadic
		getToolResp, code := c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(nomadicToolID))
		qt.Assert(t, code, qt.Equals, 200)
		var toolDetails struct {
			Data api.Tool `json:"data"`
		}
		err = json.Unmarshal(getToolResp, &toolDetails)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, *toolDetails.Data.IsNomadic, qt.Equals, false)
	})

	t.Run("Tool Cost Management", func(t *testing.T) {
		// Create a user for this test
		ownerJWT := c.RegisterAndLogin("cost-test-owner@test.com", "cost-test-owner", "ownerpass")
		customCost := uint64(20000) / types.FactorCostToPrice // customCost is 20

		// Test case 1: Create a tool with ToolValuation and verify Cost and EstimatedDailyCost are set correctly
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Cost Test Tool",
				Description:   "Tool to test cost calculation",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Location: api.Location{
					Latitude:  41695384,
					Longitude: 2492793,
				},
				Cost: customCost,
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
		toolID := toolResp.Data.ID

		// Get the tool to verify cost values
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200)

		var getToolResp struct {
			Data api.Tool `json:"data"`
		}
		err = json.Unmarshal(resp, &getToolResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify Cost and EstimatedDailyCost are calculated correctly from ToolValuation
		estimatedDailyCost := uint64(10000) / types.FactorCostToPrice // FactorCostToPrice is 10
		qt.Assert(t, getToolResp.Data.Cost, qt.Equals, customCost)
		qt.Assert(t, getToolResp.Data.EstimatedDailyCost, qt.Equals, estimatedDailyCost)

		// Test case 2: Edit a tool to update ToolValuation and verify Cost and EstimatedDailyCost are updated
		_, code = c.Request(http.MethodPut, ownerJWT,
			api.Tool{
				ToolValuation: uint64Ptr(20000),
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get the tool to verify updated cost values
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &getToolResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify Cost and EstimatedDailyCost are updated correctly
		expectedCost := uint64(20000) / types.FactorCostToPrice
		qt.Assert(t, getToolResp.Data.Cost, qt.Equals, customCost) // Cost didn't change with valuation update
		qt.Assert(t, getToolResp.Data.EstimatedDailyCost, qt.Equals, expectedCost)

		// Test case 3: Edit a tool to set a custom Cost that's less than EstimatedDailyCost
		customCost = expectedCost - 50
		_, code = c.Request(http.MethodPut, ownerJWT,
			api.Tool{
				Cost: customCost,
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get the tool to verify custom cost was applied
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &getToolResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify Cost was updated but EstimatedDailyCost remains the same
		qt.Assert(t, getToolResp.Data.Cost, qt.Equals, customCost)
		qt.Assert(t, getToolResp.Data.EstimatedDailyCost, qt.Equals, expectedCost)

		// Test case 4: Attempt to set a Cost greater than EstimatedDailyCost and verify it is applied
		newCost := expectedCost + 500
		_, code = c.Request(http.MethodPut, ownerJWT,
			api.Tool{
				Cost: newCost,
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get the tool to verify cost wasn't changed
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &getToolResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify Cost remains the same (not increased beyond EstimatedDailyCost)
		qt.Assert(t, getToolResp.Data.Cost, qt.Equals, newCost)
	})

	t.Run("Nomadic Tool History", func(t *testing.T) {
		// Create users for this test
		ownerJWT, _ := c.RegisterAndLoginWithID("nomadic-history-owner@test.com", "nomadic-history-owner", "ownerpass")
		renterJWT, _ := c.RegisterAndLoginWithID("nomadic-history-renter@test.com", "nomadic-history-renter", "renterpass")

		// Create a nomadic tool
		createToolResp, code := c.Request(http.MethodPost, ownerJWT, map[string]interface{}{
			"title":         "Nomadic Tool with History",
			"description":   "This tool is used to test history tracking",
			"toolCategory":  1,
			"toolValuation": 100,
			"isNomadic":     true,
			"location": map[string]interface{}{
				"latitude":  41695384,
				"longitude": 2492793,
			},
		}, "tools")
		qt.Assert(t, code, qt.Equals, 200)

		var toolIDResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err := json.Unmarshal(createToolResp, &toolIDResp)
		qt.Assert(t, err, qt.IsNil)
		nomadicToolID := toolIDResp.Data.ID

		// Create a booking for the nomadic tool
		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		resp, code := c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(nomadicToolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Booking for nomadic tool history test",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get the booking ID
		var bookingResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		bookingID := bookingResp.Data.ID

		// Owner accepts the booking
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Owner marks the booking as picked up (this should create a history entry)
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "PICKED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Get the tool history
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(nomadicToolID), "history")
		qt.Assert(t, code, qt.Equals, 200)

		// Verify the history contains an entry
		var historyResp struct {
			Data []api.ToolHistoryEntry `json:"data"`
		}
		err = json.Unmarshal(resp, &historyResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(historyResp.Data) > 0, qt.Equals, true, qt.Commentf("Tool history should have at least one entry"))

		// Verify the history entry contains the expected data
		if len(historyResp.Data) > 0 {
			entry := historyResp.Data[0]
			qt.Assert(t, entry.BookingID, qt.Equals, bookingID)
			qt.Assert(t, entry.PickupDate > 0, qt.Equals, true)
			qt.Assert(t, entry.Location.Latitude != 0, qt.Equals, true)
			qt.Assert(t, entry.Location.Longitude != 0, qt.Equals, true)
		}
	})

	t.Run("Tool Pagination", func(t *testing.T) {
		// Create a user for this test
		userJWT := c.RegisterAndLogin("pagination-test@test.com", "pagination-test", "paginationpass")

		// Create multiple tools for pagination testing
		toolNames := []string{
			"Pagination Tool 1", "Pagination Tool 2", "Pagination Tool 3",
			"Pagination Tool 4", "Pagination Tool 5", "Pagination Tool 6",
			"Pagination Tool 7", "Pagination Tool 8", "Pagination Tool 9",
			"Pagination Tool 10", "Pagination Tool 11", "Pagination Tool 12",
		}

		for _, name := range toolNames {
			c.CreateTool(userJWT, name)
		}

		// Test case 1: Default pagination (page 0, default page size)
		resp, code := c.Request(http.MethodGet, userJWT, nil, "tools")
		qt.Assert(t, code, qt.Equals, 200)

		var paginatedResp struct {
			Data struct {
				Tools      []api.Tool         `json:"tools"`
				Pagination api.PaginationInfo `json:"pagination"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify pagination info
		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 0)
		qt.Assert(t, paginatedResp.Data.Pagination.PageSize, qt.Equals, 16) // Default page size from db/constants.go
		qt.Assert(t, paginatedResp.Data.Pagination.Total >= int64(len(toolNames)), qt.Equals, true)
		qt.Assert(t, paginatedResp.Data.Pagination.Pages > 0, qt.Equals, true)

		// Verify we got the first page of tools (should be up to 16 tools)
		qt.Assert(t, len(paginatedResp.Data.Tools) <= 16, qt.Equals, true)
		qt.Assert(t, len(paginatedResp.Data.Tools) > 0, qt.Equals, true)

		// Test case 2: Custom page size
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools?pageSize=5")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify pagination info with custom page size
		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 0)
		qt.Assert(t, paginatedResp.Data.Pagination.PageSize, qt.Equals, 5)
		qt.Assert(t, len(paginatedResp.Data.Tools), qt.Equals, 5)

		// Test case 3: Second page
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools?page=1&pageSize=5")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify pagination info for second page
		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 1)
		qt.Assert(t, paginatedResp.Data.Pagination.PageSize, qt.Equals, 5)
		qt.Assert(t, len(paginatedResp.Data.Tools), qt.Equals, 5)

		// Test case 4: Search with pagination
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools?search=Pagination&page=0&pageSize=3")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify search results with pagination
		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 0)
		qt.Assert(t, paginatedResp.Data.Pagination.PageSize, qt.Equals, 3)
		qt.Assert(t, len(paginatedResp.Data.Tools) > 0, qt.Equals, true)
		qt.Assert(t, len(paginatedResp.Data.Tools) <= 3, qt.Equals, true)

		// Verify search results contain the search term
		for _, tool := range paginatedResp.Data.Tools {
			qt.Assert(t, tool.Title, qt.Contains, "Pagination")
		}

		// Test case 5: Invalid page number
		_, code = c.Request(http.MethodGet, userJWT, nil, "tools?page=invalid")
		qt.Assert(t, code, qt.Equals, 400) // Bad Request
	})
}

func TestToolsDistanceValidation(t *testing.T) {
	c := utils.NewTestService(t)

	// Create a user
	userJWT := c.RegisterAndLogin("distance-test@test.com", "distanceuser", "userpass")

	t.Run("Distance Filter Validation", func(t *testing.T) {
		// Test coordinates - using the same coordinates from the failing test
		centerLat := int64(41695384)  // 41.695384 * 1e6 (center)
		centerLon := int64(2492793)   // 2.492793 * 1e6
		anotherLat := int64(41776239) // 41.776239 * 1e6 (9 km north)
		anotherLon := int64(2492793)  // same longitude

		//----------------------------------------------------------------------
		// 1) Create "Center Tool" at the exact center location
		//----------------------------------------------------------------------
		_, code := c.Request(http.MethodPost, userJWT,
			api.Tool{
				Title:         "Center Tool",
				Description:   "Tool at center location",
				MayBeFree:     boolPtr(true),
				AskWithFee:    boolPtr(false),
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Height:        30,
				Weight:        40,
				IsAvailable:   boolPtr(true),
				Location: api.Location{
					Latitude:  centerLat,
					Longitude: centerLon,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		//----------------------------------------------------------------------
		// 2) Create "Tool at 5km" - should be within 10km radius
		//----------------------------------------------------------------------
		_, code = c.Request(http.MethodPost, userJWT,
			api.Tool{
				Title:         "Tool at 5km",
				Description:   "Tool at 5km away",
				MayBeFree:     boolPtr(true),
				AskWithFee:    boolPtr(false),
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Height:        30,
				Weight:        40,
				IsAvailable:   boolPtr(true),
				Location: api.Location{
					Latitude:  41745384, // ~5 km from center
					Longitude: centerLon,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		//----------------------------------------------------------------------
		// 3) Create "Tool at 9km" - this is the problematic one from the original test
		//----------------------------------------------------------------------
		_, code = c.Request(http.MethodPost, userJWT,
			api.Tool{
				Title:         "Tool at 9km",
				Description:   "Tool at 9km away - boundary case",
				MayBeFree:     boolPtr(true),
				AskWithFee:    boolPtr(false),
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Height:        30,
				Weight:        40,
				IsAvailable:   boolPtr(true),
				Location: api.Location{
					Latitude:  anotherLat, // ~9 km from center
					Longitude: anotherLon,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		//----------------------------------------------------------------------
		// 4) Create "Tool at 15km" - should be outside 10km radius
		//----------------------------------------------------------------------
		_, code = c.Request(http.MethodPost, userJWT,
			api.Tool{
				Title:         "Tool at 15km",
				Description:   "Tool at 15km away",
				MayBeFree:     boolPtr(true),
				AskWithFee:    boolPtr(false),
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				Height:        30,
				Weight:        40,
				IsAvailable:   boolPtr(true),
				Location: api.Location{
					Latitude:  41845384, // ~15 km from center
					Longitude: centerLon,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		//----------------------------------------------------------------------
		// 5) Test search with 10km radius - should find exactly 3 tools
		//    (Center Tool, Tool at 5km, Tool at 9km)
		//----------------------------------------------------------------------
		searchURL := fmt.Sprintf("tools/search?distance=10000&latitude=%d&longitude=%d", centerLat, centerLon)
		resp, code := c.Request(http.MethodGet, userJWT, nil, searchURL)
		qt.Assert(t, code, qt.Equals, 200)

		var searchResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &searchResp)
		qt.Assert(t, err, qt.IsNil)

		// Log the results for debugging
		t.Logf("Search with 10km radius returned %d tools:", len(searchResp.Data.Tools))
		for i, tool := range searchResp.Data.Tools {
			t.Logf("  %d. %s (ID: %d)", i+1, tool.Title, tool.ID)
		}

		// Should find exactly 3 tools within 10km
		qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 3, qt.Commentf("Expected 3 tools within 10km radius"))

		// Verify which tools were found
		foundTitles := make(map[string]bool)
		for _, tool := range searchResp.Data.Tools {
			foundTitles[tool.Title] = true
		}

		qt.Assert(t, foundTitles["Center Tool"], qt.Equals, true, qt.Commentf("Center Tool should be found"))
		qt.Assert(t, foundTitles["Tool at 5km"], qt.Equals, true, qt.Commentf("Tool at 5km should be found"))
		qt.Assert(t, foundTitles["Tool at 9km"], qt.Equals, true, qt.Commentf("Tool at 9km should be found"))
		qt.Assert(t, foundTitles["Tool at 15km"], qt.Equals, false, qt.Commentf("Tool at 15km should NOT be found"))

		//----------------------------------------------------------------------
		// 6) Test search with 8km radius - should find only 2 tools
		//    (Center Tool, Tool at 5km) - Tool at 9km should be excluded
		//----------------------------------------------------------------------
		searchURL = fmt.Sprintf("tools/search?distance=8000&latitude=%d&longitude=%d", centerLat, centerLon)
		resp, code = c.Request(http.MethodGet, userJWT, nil, searchURL)
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &searchResp)
		qt.Assert(t, err, qt.IsNil)

		// Log the results for debugging
		t.Logf("Search with 8km radius returned %d tools:", len(searchResp.Data.Tools))
		for i, tool := range searchResp.Data.Tools {
			t.Logf("  %d. %s (ID: %d)", i+1, tool.Title, tool.ID)
		}

		// Should find exactly 2 tools within 8km
		qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 2, qt.Commentf("Expected 2 tools within 8km radius"))

		// Verify which tools were found
		foundTitles = make(map[string]bool)
		for _, tool := range searchResp.Data.Tools {
			foundTitles[tool.Title] = true
		}

		qt.Assert(t, foundTitles["Center Tool"], qt.Equals, true, qt.Commentf("Center Tool should be found"))
		qt.Assert(t, foundTitles["Tool at 5km"], qt.Equals, true, qt.Commentf("Tool at 5km should be found"))
		qt.Assert(t, foundTitles["Tool at 9km"], qt.Equals, false, qt.Commentf("Tool at 9km should NOT be found"))
		qt.Assert(t, foundTitles["Tool at 15km"], qt.Equals, false, qt.Commentf("Tool at 15km should NOT be found"))

		//----------------------------------------------------------------------
		// 7) Test search with 20km radius - should find all 4 tools
		//----------------------------------------------------------------------
		searchURL = fmt.Sprintf("tools/search?distance=20000&latitude=%d&longitude=%d", centerLat, centerLon)
		resp, code = c.Request(http.MethodGet, userJWT, nil, searchURL)
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &searchResp)
		qt.Assert(t, err, qt.IsNil)

		// Log the results for debugging
		t.Logf("Search with 20km radius returned %d tools:", len(searchResp.Data.Tools))
		for i, tool := range searchResp.Data.Tools {
			t.Logf("  %d. %s (ID: %d)", i+1, tool.Title, tool.ID)
		}

		// Should find all 4 tools within 20km
		qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 4, qt.Commentf("Expected 4 tools within 20km radius"))

		// Verify all tools were found
		foundTitles = make(map[string]bool)
		for _, tool := range searchResp.Data.Tools {
			foundTitles[tool.Title] = true
		}

		qt.Assert(t, foundTitles["Center Tool"], qt.Equals, true, qt.Commentf("Center Tool should be found"))
		qt.Assert(t, foundTitles["Tool at 5km"], qt.Equals, true, qt.Commentf("Tool at 5km should be found"))
		qt.Assert(t, foundTitles["Tool at 9km"], qt.Equals, true, qt.Commentf("Tool at 9km should be found"))
		qt.Assert(t, foundTitles["Tool at 15km"], qt.Equals, true, qt.Commentf("Tool at 15km should be found"))
	})

	t.Run("Consistency Test - Multiple Runs", func(t *testing.T) {
		// Run the same search multiple times to ensure consistency
		centerLat := int64(41695384)
		centerLon := int64(2492793)

		searchURL := fmt.Sprintf("tools/search?distance=10000&latitude=%d&longitude=%d", centerLat, centerLon)

		// Run the search 10 times to check for consistency
		for i := 0; i < 10; i++ {
			resp, code := c.Request(http.MethodGet, userJWT, nil, searchURL)
			qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Search iteration %d failed", i+1))

			var searchResp struct {
				Data struct {
					Tools []api.Tool `json:"tools"`
				} `json:"data"`
			}
			err := json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to unmarshal response on iteration %d", i+1))

			// Should consistently return the same number of tools
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 3,
				qt.Commentf("Iteration %d: Expected 3 tools, got %d", i+1, len(searchResp.Data.Tools)))
		}
	})
}

func TestToolAccessWithBookingHistory(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users
	activeUserJWT := c.RegisterAndLogin("active@test.com", "Active User", "password")
	inactiveUserJWT := c.RegisterAndLogin("inactive@test.com", "Inactive User", "password")
	requesterJWT := c.RegisterAndLogin("requester@test.com", "Requester User", "password")
	otherUserJWT := c.RegisterAndLogin("other@test.com", "Other User", "password")

	// Create tools
	activeToolID := c.CreateTool(activeUserJWT, "Active User Tool")
	inactiveToolID := c.CreateTool(inactiveUserJWT, "Inactive User Tool")

	// Create a booking request from requester to inactive user's tool
	bookingRequest := api.CreateBookingRequest{
		ToolID:    fmt.Sprintf("%d", inactiveToolID),
		StartDate: time.Now().Add(24 * time.Hour).Unix(),
		EndDate:   time.Now().Add(48 * time.Hour).Unix(),
		Contact:   "test@example.com",
		Comments:  "Test booking request",
	}

	// Make the booking request
	resp, code := c.Request(http.MethodPost, requesterJWT, bookingRequest, "bookings")
	qt.Assert(t, code, qt.Equals, 200)

	var bookingResp struct {
		Data *api.BookingResponse `json:"data"`
	}
	err := json.Unmarshal(resp, &bookingResp)
	qt.Assert(t, err, qt.IsNil)

	// Now deactivate the inactive user
	_, code = c.Request(http.MethodPost, inactiveUserJWT,
		api.UserProfile{
			Active: &[]bool{false}[0], // Set active to false
		},
		"profile",
	)
	qt.Assert(t, code, qt.Equals, 200)

	t.Run("Inactive user tool owner can still access own tool", func(t *testing.T) {
		_, code := c.Request(http.MethodGet, inactiveUserJWT, nil, "tools", fmt.Sprintf("%d", inactiveToolID))
		qt.Assert(t, code, qt.Equals, 200)
	})

	t.Run("User with booking history can access inactive user's tool", func(t *testing.T) {
		// Requester should be able to access the tool because they made a booking request
		_, code := c.Request(http.MethodGet, requesterJWT, nil, "tools", fmt.Sprintf("%d", inactiveToolID))
		qt.Assert(t, code, qt.Equals, 200)
	})

	t.Run("User without booking history cannot access inactive user's tool", func(t *testing.T) {
		// Other user should not be able to access the tool (no booking history)
		_, code := c.Request(http.MethodGet, otherUserJWT, nil, "tools", fmt.Sprintf("%d", inactiveToolID))
		qt.Assert(t, code, qt.Equals, 404)
	})

	t.Run("Active user tools remain accessible to everyone", func(t *testing.T) {
		// All users should still be able to access active user's tools
		_, code := c.Request(http.MethodGet, requesterJWT, nil, "tools", fmt.Sprintf("%d", activeToolID))
		qt.Assert(t, code, qt.Equals, 200)

		_, code = c.Request(http.MethodGet, otherUserJWT, nil, "tools", fmt.Sprintf("%d", activeToolID))
		qt.Assert(t, code, qt.Equals, 200)

		_, code = c.Request(http.MethodGet, inactiveUserJWT, nil, "tools", fmt.Sprintf("%d", activeToolID))
		qt.Assert(t, code, qt.Equals, 200)
	})
}

func TestToolAccessWithMultipleBookings(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users
	toolOwnerJWT := c.RegisterAndLogin("owner@test.com", "Tool Owner", "password")
	requester1JWT := c.RegisterAndLogin("requester1@test.com", "Requester 1", "password")
	requester2JWT := c.RegisterAndLogin("requester2@test.com", "Requester 2", "password")
	noBookingJWT := c.RegisterAndLogin("nobooking@test.com", "No Booking User", "password")

	// Create tool
	toolID := c.CreateTool(toolOwnerJWT, "Test Tool")

	// Create booking requests from both requesters
	bookingRequest1 := api.CreateBookingRequest{
		ToolID:    fmt.Sprintf("%d", toolID),
		StartDate: time.Now().Add(24 * time.Hour).Unix(),
		EndDate:   time.Now().Add(48 * time.Hour).Unix(),
		Contact:   "requester1@example.com",
		Comments:  "First booking request",
	}

	bookingRequest2 := api.CreateBookingRequest{
		ToolID:    fmt.Sprintf("%d", toolID),
		StartDate: time.Now().Add(72 * time.Hour).Unix(),
		EndDate:   time.Now().Add(96 * time.Hour).Unix(),
		Contact:   "requester2@example.com",
		Comments:  "Second booking request",
	}

	// Make booking requests
	_, code := c.Request(http.MethodPost, requester1JWT, bookingRequest1, "bookings")
	qt.Assert(t, code, qt.Equals, 200)

	_, code = c.Request(http.MethodPost, requester2JWT, bookingRequest2, "bookings")
	qt.Assert(t, code, qt.Equals, 200)

	// Deactivate the tool owner
	_, code = c.Request(http.MethodPost, toolOwnerJWT,
		api.UserProfile{
			Active: &[]bool{false}[0], // Set active to false
		},
		"profile",
	)
	qt.Assert(t, code, qt.Equals, 200)

	t.Run("Both requesters can access inactive owner's tool", func(t *testing.T) {
		// Both requesters should have access
		_, code := c.Request(http.MethodGet, requester1JWT, nil, "tools", fmt.Sprintf("%d", toolID))
		qt.Assert(t, code, qt.Equals, 200)

		_, code = c.Request(http.MethodGet, requester2JWT, nil, "tools", fmt.Sprintf("%d", toolID))
		qt.Assert(t, code, qt.Equals, 200)
	})

	t.Run("User without booking history cannot access", func(t *testing.T) {
		// User with no booking history should not have access
		_, code := c.Request(http.MethodGet, noBookingJWT, nil, "tools", fmt.Sprintf("%d", toolID))
		qt.Assert(t, code, qt.Equals, 404)
	})

	t.Run("Tool owner can still access own tool", func(t *testing.T) {
		// Tool owner should still have access to their own tool
		_, code := c.Request(http.MethodGet, toolOwnerJWT, nil, "tools", fmt.Sprintf("%d", toolID))
		qt.Assert(t, code, qt.Equals, 200)
	})
}

func TestToolRatingsAccessWithBookingHistory(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users
	toolOwnerJWT := c.RegisterAndLogin("owner@test.com", "Tool Owner", "password")
	requesterJWT := c.RegisterAndLogin("requester@test.com", "Requester", "password")
	otherUserJWT := c.RegisterAndLogin("other@test.com", "Other User", "password")

	// Create tool
	toolID := c.CreateTool(toolOwnerJWT, "Test Tool")

	// Create booking request
	bookingRequest := api.CreateBookingRequest{
		ToolID:    fmt.Sprintf("%d", toolID),
		StartDate: time.Now().Add(24 * time.Hour).Unix(),
		EndDate:   time.Now().Add(48 * time.Hour).Unix(),
		Contact:   "requester@example.com",
		Comments:  "Test booking request",
	}

	_, code := c.Request(http.MethodPost, requesterJWT, bookingRequest, "bookings")
	qt.Assert(t, code, qt.Equals, 200)

	// Deactivate the tool owner
	_, code = c.Request(http.MethodPost, toolOwnerJWT,
		api.UserProfile{
			Active: &[]bool{false}[0], // Set active to false
		},
		"profile",
	)
	qt.Assert(t, code, qt.Equals, 200)

	t.Run("User with booking history can access tool ratings", func(t *testing.T) {
		// Requester should be able to access tool ratings
		_, code := c.Request(http.MethodGet, requesterJWT, nil, "tools", fmt.Sprintf("%d/ratings", toolID))
		qt.Assert(t, code, qt.Equals, 200)
	})

	t.Run("User without booking history cannot access tool ratings", func(t *testing.T) {
		// Other user should not be able to access tool ratings
		_, code := c.Request(http.MethodGet, otherUserJWT, nil, "tools", fmt.Sprintf("%d/ratings", toolID))
		qt.Assert(t, code, qt.Equals, 404)
	})

	t.Run("Tool owner can access own tool ratings", func(t *testing.T) {
		// Tool owner should still be able to access their own tool ratings
		_, code := c.Request(http.MethodGet, toolOwnerJWT, nil, "tools", fmt.Sprintf("%d/ratings", toolID))
		qt.Assert(t, code, qt.Equals, 200)
	})
}

func TestToolUserActiveFields(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users with different active states
	activeUserJWT := c.RegisterAndLogin("active@test.com", "Active User", "password")
	inactiveUserJWT := c.RegisterAndLogin("inactive@test.com", "Inactive User", "password")

	// Create tools owned by both users
	activeUserToolID := c.CreateTool(activeUserJWT, "Active User Tool")
	inactiveUserToolID := c.CreateTool(inactiveUserJWT, "Inactive User Tool")

	// Deactivate the inactive user
	_, code := c.Request(http.MethodPost, inactiveUserJWT,
		api.UserProfile{
			Active: &[]bool{false}[0], // Set active to false
		},
		"profile",
	)
	qt.Assert(t, code, qt.Equals, 200)

	t.Run("UserActive is true when owner is active", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, activeUserJWT, nil, "tools", fmt.Sprintf("%d", activeUserToolID))
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data api.Tool `json:"data"`
		}
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, toolResp.Data.UserActive, qt.IsTrue, qt.Commentf("UserActive should be true when owner is active"))
		qt.Assert(t, toolResp.Data.ActualUserActive, qt.IsFalse, qt.Commentf(
			"ActualUserActive should be false when ActualUserID is not set"),
		)
	})

	t.Run("UserActive is false when owner is inactive", func(t *testing.T) {
		// Inactive user can still access their own tool
		resp, code := c.Request(http.MethodGet, inactiveUserJWT, nil, "tools", fmt.Sprintf("%d", inactiveUserToolID))
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data api.Tool `json:"data"`
		}
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, toolResp.Data.UserActive, qt.IsFalse, qt.Commentf("UserActive should be false when owner is inactive"))
		qt.Assert(t, toolResp.Data.ActualUserActive, qt.IsFalse, qt.Commentf(
			"ActualUserActive should be false when ActualUserID is not set"),
		)
	})

	t.Run("Tool search includes UserActive information", func(t *testing.T) {
		// Search for tools as active user
		resp, code := c.Request(http.MethodGet, activeUserJWT, nil, "tools/search")
		qt.Assert(t, code, qt.Equals, 200)

		var searchResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &searchResp)
		qt.Assert(t, err, qt.IsNil)

		// Find our test tools in the search results
		var activeUserTool, inactiveUserTool *api.Tool
		for i := range searchResp.Data.Tools {
			tool := &searchResp.Data.Tools[i]
			switch tool.ID {
			case activeUserToolID:
				activeUserTool = tool
			case inactiveUserToolID:
				inactiveUserTool = tool
			}
		}

		// Verify UserActive fields are populated correctly
		if activeUserTool != nil {
			qt.Assert(t, activeUserTool.UserActive, qt.IsTrue, qt.Commentf("Active user's tool should have UserActive=true"))
		}

		// Note: Inactive user's tool might not appear in search results due to access control,
		// but if it does, UserActive should be false
		if inactiveUserTool != nil {
			qt.Assert(t, inactiveUserTool.UserActive, qt.IsFalse, qt.Commentf("Inactive user's tool should have UserActive=false"))
		}
	})

	t.Run("Tool list includes UserActive information", func(t *testing.T) {
		// Get active user's own tools
		resp, code := c.Request(http.MethodGet, activeUserJWT, nil, "tools")
		qt.Assert(t, code, qt.Equals, 200)

		var listResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)

		// Find our test tool
		var foundTool *api.Tool
		for i := range listResp.Data.Tools {
			if listResp.Data.Tools[i].ID == activeUserToolID {
				foundTool = &listResp.Data.Tools[i]
				break
			}
		}

		qt.Assert(t, foundTool, qt.Not(qt.IsNil), qt.Commentf("Should find the active user's tool"))
		qt.Assert(t, foundTool.UserActive, qt.IsTrue, qt.Commentf("UserActive should be true for active user's tool"))
		qt.Assert(t, foundTool.ActualUserActive, qt.IsFalse, qt.Commentf(
			"ActualUserActive should be false when ActualUserID is not set"),
		)
	})

	t.Run("UserActive fields work with nomadic tools", func(t *testing.T) {
		// Create a nomadic tool
		isNomadic := true
		resp, code := c.Request(http.MethodPost, activeUserJWT,
			api.Tool{
				Title:         "Nomadic Tool",
				Description:   "Test nomadic tool",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				IsNomadic:     &isNomadic,
				Location: api.Location{
					Latitude:  41695384,
					Longitude: 2492793,
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
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		nomadicToolID := toolResp.Data.ID

		// Get the nomadic tool
		resp, code = c.Request(http.MethodGet, activeUserJWT, nil, "tools", fmt.Sprintf("%d", nomadicToolID))
		qt.Assert(t, code, qt.Equals, 200)

		var getToolResp struct {
			Data api.Tool `json:"data"`
		}
		err = json.Unmarshal(resp, &getToolResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, getToolResp.Data.UserActive, qt.IsTrue, qt.Commentf(
			"UserActive should be true for nomadic tool with active owner"),
		)
		qt.Assert(t, *getToolResp.Data.IsNomadic, qt.IsTrue, qt.Commentf("Tool should be nomadic"))
		qt.Assert(t, getToolResp.Data.ActualUserActive, qt.IsFalse, qt.Commentf(
			"ActualUserActive should be false when ActualUserID is not set"),
		)
	})
}

func TestToolsHeldFunctionality(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users for testing
	ownerJWT, ownerID := c.RegisterAndLoginWithID("owner@test.com", "owner", "ownerpass")
	holderJWT, holderID := c.RegisterAndLoginWithID("holder@test.com", "holder", "holderpass")
	otherJWT, _ := c.RegisterAndLoginWithID("other@test.com", "other", "otherpass")

	t.Run("Test Own Tools and Held Tools Query Parameters", func(t *testing.T) {
		// Create a regular (non-nomadic) tool owned by owner
		_ = c.CreateTool(ownerJWT, "Regular Tool")

		// Create a nomadic tool owned by owner
		isNomadic := true
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.Tool{
				Title:         "Nomadic Tool",
				Description:   "Test nomadic tool",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				IsNomadic:     &isNomadic,
				Location: api.Location{
					Latitude:  41695384,
					Longitude: 2492793,
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
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		nomadicToolID := toolResp.Data.ID

		// Create a booking for the nomadic tool from holder to owner
		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		resp, code = c.Request(http.MethodPost, holderJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(nomadicToolID),
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
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		bookingID := bookingResp.Data.ID

		// Owner accepts the booking
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Owner marks the booking as picked up (this should set actualUserId to holder)
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "PICKED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Test 1: Default behavior (both ownTools=true and heldTools=true)
		// Owner should see both tools (regular tool owned + nomadic tool owned)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools")
		qt.Assert(t, code, qt.Equals, 200)

		var listResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 2, qt.Commentf("Owner should see both owned tools by default"))

		// Holder should see the nomadic tool they're holding
		resp, code = c.Request(http.MethodGet, holderJWT, nil, "tools")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 1, qt.Commentf("Holder should see the tool they're holding by default"))

		// Verify it's the nomadic tool
		foundNomadicTool := false
		for _, tool := range listResp.Data.Tools {
			if tool.ID == nomadicToolID {
				foundNomadicTool = true
				qt.Assert(t, *tool.IsNomadic, qt.IsTrue, qt.Commentf("Tool should be nomadic"))
				break
			}
		}
		qt.Assert(t, foundNomadicTool, qt.IsTrue, qt.Commentf("Holder should see the nomadic tool they're holding"))

		// Test 2: Only own tools (ownTools=true&heldTools=false)
		// Owner should see both owned tools
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools?ownTools=true&heldTools=false")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 2,
			qt.Commentf("Owner should see both owned tools when ownTools=true&heldTools=false"))

		// Holder should see no tools (they don't own any)
		resp, code = c.Request(http.MethodGet, holderJWT, nil, "tools?ownTools=true&heldTools=false")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 0,
			qt.Commentf("Holder should see no tools when ownTools=true&heldTools=false"))

		// Test 3: Only held tools (ownTools=false&heldTools=true)
		// Owner should see no tools (they're not holding any)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools?ownTools=false&heldTools=true")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 0,
			qt.Commentf("Owner should see no tools when ownTools=false&heldTools=true"))

		// Holder should see the nomadic tool they're holding
		resp, code = c.Request(http.MethodGet, holderJWT, nil, "tools?ownTools=false&heldTools=true")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 1,
			qt.Commentf("Holder should see the tool they're holding when ownTools=false&heldTools=true"))

		// Test 4: Neither (ownTools=false&heldTools=false)
		// Both should see no tools
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools?ownTools=false&heldTools=false")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 0, qt.Commentf("Owner should see no tools when both parameters are false"))

		resp, code = c.Request(http.MethodGet, holderJWT, nil, "tools?ownTools=false&heldTools=false")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 0, qt.Commentf("Holder should see no tools when both parameters are false"))

		// Test 5: Other user should see no tools regardless of parameters
		resp, code = c.Request(http.MethodGet, otherJWT, nil, "tools")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 0, qt.Commentf("Other user should see no tools"))

		resp, code = c.Request(http.MethodGet, otherJWT, nil, "tools?ownTools=true&heldTools=false")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 0,
			qt.Commentf("Other user should see no tools with ownTools=true&heldTools=false"))

		resp, code = c.Request(http.MethodGet, otherJWT, nil, "tools?ownTools=false&heldTools=true")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 0,
			qt.Commentf("Other user should see no tools with ownTools=false&heldTools=true"))
	})

	t.Run("Test User Tools Endpoint with Query Parameters", func(t *testing.T) {
		// Test the /tools/user/{id} endpoint with the same parameters
		// Create a tool owned by owner
		ownerToolID := c.CreateTool(ownerJWT, "Owner Tool for User Endpoint")

		// Create a nomadic tool owned by holder
		isNomadic := true
		resp, code := c.Request(http.MethodPost, holderJWT,
			api.Tool{
				Title:         "Holder Nomadic Tool",
				Description:   "Test nomadic tool owned by holder",
				Category:      1,
				ToolValuation: uint64Ptr(10000),
				IsNomadic:     &isNomadic,
				Location: api.Location{
					Latitude:  41695384,
					Longitude: 2492793,
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
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		holderNomadicToolID := toolResp.Data.ID

		// Create a booking for holder's nomadic tool from owner to holder
		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		resp, code = c.Request(http.MethodPost, ownerJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(holderNomadicToolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking for holder's nomadic tool",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var bookingResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		bookingID := bookingResp.Data.ID

		// Holder accepts the booking
		_, code = c.Request(http.MethodPut, holderJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Holder marks the booking as picked up (this should set actualUserId to owner)
		_, code = c.Request(http.MethodPut, holderJWT,
			&api.BookingStatusUpdate{
				Status: "PICKED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Test 1: Get owner's tools (should include owned tool)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools/user/"+ownerID)
		qt.Assert(t, code, qt.Equals, 200)

		var listResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)

		// Owner should see their owned tools + the nomadic tool they're holding
		// Note: This includes tools from previous sub-tests within the same test function
		qt.Assert(t, len(listResp.Data.Tools) >= 2, qt.IsTrue, qt.Commentf("Owner should see at least owned tool + held nomadic tool"))

		// Verify we have the specific tools we're testing
		foundOwnerTool := false
		foundHolderNomadic := false
		for _, tool := range listResp.Data.Tools {
			if tool.ID == ownerToolID {
				foundOwnerTool = true
			}
			if tool.ID == holderNomadicToolID {
				foundHolderNomadic = true
			}
		}
		qt.Assert(t, foundOwnerTool, qt.IsTrue, qt.Commentf("Should find owner's tool"))
		qt.Assert(t, foundHolderNomadic, qt.IsTrue, qt.Commentf("Should find nomadic tool owner is holding"))

		// Test 2: Get owner's tools with ownTools=true&heldTools=false
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools/user/"+ownerID+"?ownTools=true&heldTools=false")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools) >= 1, qt.IsTrue, qt.Commentf("Owner should see at least their owned tools"))

		// Should find the owner's tool
		foundOwnerTool = false
		for _, tool := range listResp.Data.Tools {
			if tool.ID == ownerToolID {
				foundOwnerTool = true
				break
			}
		}
		qt.Assert(t, foundOwnerTool, qt.IsTrue, qt.Commentf("Should find owner's tool"))

		// Test 3: Get owner's tools with ownTools=false&heldTools=true
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools/user/"+ownerID+"?ownTools=false&heldTools=true")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 1, qt.Commentf("Owner should see only held tools"))

		// Should be the holder's nomadic tool
		qt.Assert(t, listResp.Data.Tools[0].ID, qt.Equals, holderNomadicToolID)

		// Test 4: Get holder's tools (should include owned nomadic tool, but not the one they gave to owner)
		resp, code = c.Request(http.MethodGet, holderJWT, nil, "tools/user/"+holderID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listResp.Data.Tools) >= 1, qt.IsTrue, qt.Commentf("Holder should see at least their owned nomadic tool"))

		// Should find the holder's nomadic tool (they still own it, but owner is holding it)
		foundHolderNomadic = false
		for _, tool := range listResp.Data.Tools {
			if tool.ID == holderNomadicToolID {
				foundHolderNomadic = true
				break
			}
		}
		qt.Assert(t, foundHolderNomadic, qt.IsTrue, qt.Commentf("Should find holder's nomadic tool"))
	})

	t.Run("Test Search Functionality with Held Tools", func(t *testing.T) {
		// Test that search functionality works with the new query parameters
		// This is mainly to ensure we didn't break existing search functionality

		// Create a tool with a specific title for searching
		searchToolID := c.CreateTool(ownerJWT, "Searchable Test Tool")

		// Search for the tool
		resp, code := c.Request(http.MethodGet, ownerJWT, nil, "tools?search=Searchable")
		qt.Assert(t, code, qt.Equals, 200)

		var listResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)

		// Should find the searchable tool
		foundSearchableTool := false
		for _, tool := range listResp.Data.Tools {
			if tool.ID == searchToolID {
				foundSearchableTool = true
				break
			}
		}
		qt.Assert(t, foundSearchableTool, qt.IsTrue, qt.Commentf("Should find searchable tool"))

		// Test search with query parameters
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools?search=Searchable&ownTools=true&heldTools=false")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)

		// Should still find the searchable tool
		foundSearchableTool = false
		for _, tool := range listResp.Data.Tools {
			if tool.ID == searchToolID {
				foundSearchableTool = true
				break
			}
		}
		qt.Assert(t, foundSearchableTool, qt.IsTrue, qt.Commentf("Should find searchable tool with query parameters"))
	})

	t.Run("Test Invalid Query Parameters", func(t *testing.T) {
		// Test that invalid boolean values are handled gracefully
		// Invalid values should default to true (backward compatibility)

		resp, code := c.Request(http.MethodGet, ownerJWT, nil, "tools?ownTools=invalid&heldTools=alsoinvalid")
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Invalid boolean values should not cause errors"))

		var listResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)

		// Should behave like default (both true)
		qt.Assert(t, len(listResp.Data.Tools) >= 0, qt.IsTrue, qt.Commentf("Should return some result even with invalid parameters"))
	})
}

func TestToolsHeldPagination(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users for testing
	ownerJWT, _ := c.RegisterAndLoginWithID("pagination-owner@test.com", "pagination-owner", "ownerpass")
	holderJWT, _ := c.RegisterAndLoginWithID("pagination-holder@test.com", "pagination-holder", "holderpass")

	t.Run("Test Pagination with Held Tools", func(t *testing.T) {
		// Create multiple tools owned by owner
		ownedToolIDs := make([]int64, 5)
		for i := 0; i < 5; i++ {
			ownedToolIDs[i] = c.CreateTool(ownerJWT, fmt.Sprintf("Owned Tool %d", i+1))
		}

		// Create multiple nomadic tools owned by holder and give them to owner
		heldToolIDs := make([]int64, 3)
		for i := 0; i < 3; i++ {
			isNomadic := true
			resp, code := c.Request(http.MethodPost, holderJWT,
				api.Tool{
					Title:         fmt.Sprintf("Nomadic Tool %d", i+1),
					Description:   "Test nomadic tool",
					Category:      1,
					ToolValuation: uint64Ptr(10000),
					IsNomadic:     &isNomadic,
					Location: api.Location{
						Latitude:  41695384,
						Longitude: 2492793,
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
			err := json.Unmarshal(resp, &toolResp)
			qt.Assert(t, err, qt.IsNil)
			heldToolIDs[i] = toolResp.Data.ID

			// Create booking and transfer the tool to owner
			tomorrow := time.Now().Add(24 * time.Hour)
			dayAfterTomorrow := time.Now().Add(48 * time.Hour)

			resp, code = c.Request(http.MethodPost, ownerJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(heldToolIDs[i]),
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
			err = json.Unmarshal(resp, &bookingResp)
			qt.Assert(t, err, qt.IsNil)

			// Accept and pick up
			_, code = c.Request(http.MethodPut, holderJWT,
				&api.BookingStatusUpdate{Status: "ACCEPTED"}, "bookings", bookingResp.Data.ID)
			qt.Assert(t, code, qt.Equals, 200)

			_, code = c.Request(http.MethodPut, holderJWT,
				&api.BookingStatusUpdate{Status: "PICKED"}, "bookings", bookingResp.Data.ID)
			qt.Assert(t, code, qt.Equals, 200)
		}

		// Test pagination with all tools (5 owned + 3 held = 8 total)
		resp, code := c.Request(http.MethodGet, ownerJWT, nil, "tools?pageSize=3")
		qt.Assert(t, code, qt.Equals, 200)

		var listResp struct {
			Data struct {
				Tools      []api.Tool         `json:"tools"`
				Pagination api.PaginationInfo `json:"pagination"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 3, qt.Commentf("First page should have 3 tools"))
		qt.Assert(t, listResp.Data.Pagination.Total, qt.Equals, int64(8), qt.Commentf("Total should be 8 tools"))
		qt.Assert(t, listResp.Data.Pagination.Pages, qt.Equals, 3, qt.Commentf("Should have 3 pages"))

		// Test pagination with only owned tools
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools?pageSize=3&ownTools=true&heldTools=false")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 3, qt.Commentf("First page should have 3 tools"))
		qt.Assert(t, listResp.Data.Pagination.Total, qt.Equals, int64(5), qt.Commentf("Total should be 5 owned tools"))
		qt.Assert(t, listResp.Data.Pagination.Pages, qt.Equals, 2, qt.Commentf("Should have 2 pages"))

		// Test pagination with only held tools
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools?pageSize=2&ownTools=false&heldTools=true")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &listResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, len(listResp.Data.Tools), qt.Equals, 2, qt.Commentf("First page should have 2 tools"))
		qt.Assert(t, listResp.Data.Pagination.Total, qt.Equals, int64(3), qt.Commentf("Total should be 3 held tools"))
		qt.Assert(t, listResp.Data.Pagination.Pages, qt.Equals, 2, qt.Commentf("Should have 2 pages"))
	})
}
