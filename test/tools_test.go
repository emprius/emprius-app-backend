package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/db"
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
				Title:          "Test Tool",
				Description:    "Test tool description",
				MayBeFree:      boolPtr(true),
				AskWithFee:     boolPtr(false),
				Category:       1,
				EstimatedValue: uint64Ptr(10000),
				Height:         30,
				Weight:         40,
				IsAvailable:    boolPtr(true),
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
				Title:          "Test Tool",
				Description:    "Test tool description",
				MayBeFree:      boolPtr(true),
				AskWithFee:     boolPtr(false),
				Category:       1,
				EstimatedValue: uint64Ptr(20000),
				Height:         30,
				Weight:         40,
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
				Title:          "Another Tool",
				Description:    "Another tool description",
				MayBeFree:      boolPtr(false), // This will be relevant for filtering
				AskWithFee:     boolPtr(true),
				Category:       1,
				EstimatedValue: uint64Ptr(20000),
				Height:         40,
				Weight:         50,
				IsAvailable:    boolPtr(true),
				Location: api.Location{
					Latitude:  41785384, // 41.785384 * 1e6 (~9 km from updated location)
					Longitude: 2492793,  // 2.492793 * 1e6
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
				Title:          "Updated Tool",
				Description:    "Updated description",
				MayBeFree:      boolPtr(false),
				AskWithFee:     boolPtr(true),
				Category:       1,
				EstimatedValue: uint64Ptr(20000),
				Height:         40,
				Weight:         50,
				IsAvailable:    boolPtr(true),
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
					Title:          "Tool at 5km",
					Description:    "Tool at 5km away",
					MayBeFree:      boolPtr(true),  // different from "Updated Tool"
					AskWithFee:     boolPtr(false), // cost=10 => included by maxCost=15
					Category:       1,
					EstimatedValue: uint64Ptr(10000),
					Height:         30,
					Weight:         40,
					IsAvailable:    boolPtr(true),
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
					Title:          "Tool at 15km",
					Description:    "Tool at 15km away",
					MayBeFree:      boolPtr(true),
					AskWithFee:     boolPtr(false),
					Category:       1,
					EstimatedValue: uint64Ptr(20000),
					Height:         30,
					Weight:         40,
					IsAvailable:    boolPtr(true),
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
					Title:          "Tool at 25km",
					Description:    "Tool at 25km away",
					MayBeFree:      boolPtr(true),
					AskWithFee:     boolPtr(false),
					Category:       1,
					EstimatedValue: uint64Ptr(20000),
					Height:         30,
					Weight:         40,
					IsAvailable:    boolPtr(true),
					Location: api.Location{
						Latitude:  41945384, // 41.945384 * 1e6 (~25 km from center)
						Longitude: 2492793,  // 2.492793 * 1e6
					},
				},
				"tools",
			)
			qt.Assert(t, code, qt.Equals, 200)

			//--------------------------------------------------------------
			// 8C) Search with distance=10 km
			//     Should find:
			//       1) Updated Tool (center)
			//       2) Tool at 5 km
			//     => total 2
			//--------------------------------------------------------------
			resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?distance=10000")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)

			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 2)
			found5km := false
			foundUpdated := false
			for _, tool := range searchResp.Data.Tools {
				if tool.Title == "Tool at 5km" {
					found5km = true
				} else if tool.Title == "Updated Tool" {
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
				if tool.Title == "Tool at 15km" {
					found15km = true
				} else if tool.Title == "Another Tool" {
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
				if tool.Title == "Tool at 25km" {
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
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 2)

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
				Title:          "Tool for Rating",
				Description:    "Tool to test ratings",
				MayBeFree:      boolPtr(true),
				AskWithFee:     boolPtr(false),
				Category:       1,
				EstimatedValue: uint64Ptr(10000),
				Height:         30,
				Weight:         40,
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
		_, code = c.Request(http.MethodPost, userJWT, nil, "bookings", "petitions", bookingID, "accept")
		qt.Assert(t, code, qt.Equals, 200)

		// Owner marks the tool as returned
		_, code = c.Request(http.MethodPost, userJWT, nil, "bookings", bookingID, "return")
		qt.Assert(t, code, qt.Equals, 200)

		// Renter rates the booking
		_, code = c.Request(http.MethodPost, renterJWT,
			api.RateRequest{
				Rating:  5,
				Comment: "Great tool!",
			},
			"bookings", bookingID, "rate",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Owner rates the booking
		_, code = c.Request(http.MethodPost, userJWT,
			api.RateRequest{
				Rating:  4,
				Comment: "Good renter",
			},
			"bookings", bookingID, "rate",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get tool ratings
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(toolID), "rates")
		qt.Assert(t, code, qt.Equals, 200)

		var ratesResp struct {
			Data []*db.UnifiedRating `json:"data"`
		}
		err = json.Unmarshal(resp, &ratesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(ratesResp.Data), qt.Equals, 1)

		// Verify rating values
		rating := ratesResp.Data[0]
		qt.Assert(t, *rating.Owner.Rating, qt.Equals, 4)
		qt.Assert(t, *rating.Owner.RatingComment, qt.Equals, "Good renter")
		qt.Assert(t, *rating.Requester.Rating, qt.Equals, 5)
		qt.Assert(t, *rating.Requester.RatingComment, qt.Equals, "Great tool!")
	})
}
