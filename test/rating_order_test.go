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

func TestRatingOrder(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users: tool owner and renter
	ownerJWT := c.RegisterAndLogin("rating-owner@test.com", "owner", "ownerpass")
	renterJWT, renterID := c.RegisterAndLoginWithID("rating-renter@test.com", "renter", "renterpass")

	// Owner creates a tool
	toolID := c.CreateTool(ownerJWT, "Rating Test Tool")

	// Create multiple bookings with different timestamps
	var bookingIDs []string

	// Create first booking (oldest)
	req1 := api.CreateBookingRequest{
		ToolID:    fmt.Sprint(toolID),
		StartDate: time.Now().Add(24 * time.Hour).Unix(),
		EndDate:   time.Now().Add(48 * time.Hour).Unix(),
		Contact:   "test@example.com",
		Comments:  "First booking",
	}
	resp, code := c.Request(http.MethodPost, renterJWT, req1, "bookings")
	qt.Assert(t, code, qt.Equals, 200)
	var createResp struct {
		Data api.BookingResponse `json:"data"`
	}
	err := json.Unmarshal(resp, &createResp)
	qt.Assert(t, err, qt.IsNil)
	bookingIDs = append(bookingIDs, createResp.Data.ID)

	// Owner accepts the booking
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "ACCEPTED",
		},
		"bookings", bookingIDs[0])
	qt.Assert(t, code, qt.Equals, 200)

	// Mark as returned
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "RETURNED",
		}, "bookings", bookingIDs[0])
	qt.Assert(t, code, qt.Equals, 200)

	// Submit rating
	_, code = c.Request(http.MethodPost, renterJWT,
		api.RateRequest{
			Rating:  5,
			Comment: "First rating",
		},
		"bookings", bookingIDs[0], "ratings",
	)
	qt.Assert(t, code, qt.Equals, 200)

	// Wait a bit to ensure different timestamps
	time.Sleep(1 * time.Second)

	// Create second booking (newer)
	req2 := api.CreateBookingRequest{
		ToolID:    fmt.Sprint(toolID),
		StartDate: time.Now().Add(72 * time.Hour).Unix(),
		EndDate:   time.Now().Add(96 * time.Hour).Unix(),
		Contact:   "test@example.com",
		Comments:  "Second booking",
	}
	resp, code = c.Request(http.MethodPost, renterJWT, req2, "bookings")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &createResp)
	qt.Assert(t, err, qt.IsNil)
	bookingIDs = append(bookingIDs, createResp.Data.ID)

	// Owner accepts the booking
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "ACCEPTED",
		}, "bookings", bookingIDs[1])
	qt.Assert(t, code, qt.Equals, 200)

	// Mark as returned
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "RETURNED",
		}, "bookings", bookingIDs[1])
	qt.Assert(t, code, qt.Equals, 200)

	// Submit rating
	_, code = c.Request(http.MethodPost, renterJWT,
		api.RateRequest{
			Rating:  4,
			Comment: "Second rating",
		},
		"bookings", bookingIDs[1], "ratings",
	)
	qt.Assert(t, code, qt.Equals, 200)

	// Wait a bit to ensure different timestamps
	time.Sleep(1 * time.Second)

	// Create third booking (newest)
	req3 := api.CreateBookingRequest{
		ToolID:    fmt.Sprint(toolID),
		StartDate: time.Now().Add(120 * time.Hour).Unix(),
		EndDate:   time.Now().Add(144 * time.Hour).Unix(),
		Contact:   "test@example.com",
		Comments:  "Third booking",
	}
	resp, code = c.Request(http.MethodPost, renterJWT, req3, "bookings")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &createResp)
	qt.Assert(t, err, qt.IsNil)
	bookingIDs = append(bookingIDs, createResp.Data.ID)

	// Owner accepts the booking
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "ACCEPTED",
		}, "bookings", bookingIDs[2])
	qt.Assert(t, code, qt.Equals, 200)

	// Mark as returned
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "RETURNED",
		}, "bookings", bookingIDs[2])
	qt.Assert(t, code, qt.Equals, 200)

	// Submit rating
	_, code = c.Request(http.MethodPost, renterJWT,
		api.RateRequest{
			Rating:  3,
			Comment: "Third rating",
		},
		"bookings", bookingIDs[2], "ratings",
	)
	qt.Assert(t, code, qt.Equals, 200)

	// Test 1: GET /tools/{id}/rates - should return ratings ordered by newest first
	t.Run("Tool Ratings Order", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(toolID), "ratings")
		qt.Assert(t, code, qt.Equals, 200)

		var ratesResp struct {
			Data *api.PaginatedUnifiedRatingsResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &ratesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(ratesResp.Data.Ratings), qt.Equals, 3)

		// Verify order: newest first
		// The third booking should be first (newest)
		qt.Assert(t, *ratesResp.Data.Ratings[0].Requester.Rating, qt.Equals, 3)
		qt.Assert(t, *ratesResp.Data.Ratings[0].Requester.RatingComment, qt.Equals, "Third rating")

		// The second booking should be second
		qt.Assert(t, *ratesResp.Data.Ratings[1].Requester.Rating, qt.Equals, 4)
		qt.Assert(t, *ratesResp.Data.Ratings[1].Requester.RatingComment, qt.Equals, "Second rating")

		// The first booking should be last (oldest)
		qt.Assert(t, *ratesResp.Data.Ratings[2].Requester.Rating, qt.Equals, 5)
		qt.Assert(t, *ratesResp.Data.Ratings[2].Requester.RatingComment, qt.Equals, "First rating")
	})

	// Test 2: GET /users/{id}/rates - should return ratings ordered by newest first
	t.Run("User Ratings Order", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, ownerJWT, nil, "users", renterID, "ratings")
		qt.Assert(t, code, qt.Equals, 200)

		var ratesResp struct {
			Data *api.PaginatedUnifiedRatingsResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &ratesResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(ratesResp.Data.Ratings), qt.Equals, 3)

		// Verify order: newest first
		// The third booking should be first (newest)
		qt.Assert(t, *ratesResp.Data.Ratings[0].Requester.Rating, qt.Equals, 3)
		qt.Assert(t, *ratesResp.Data.Ratings[0].Requester.RatingComment, qt.Equals, "Third rating")

		// The second booking should be second
		qt.Assert(t, *ratesResp.Data.Ratings[1].Requester.Rating, qt.Equals, 4)
		qt.Assert(t, *ratesResp.Data.Ratings[1].Requester.RatingComment, qt.Equals, "Second rating")

		// The first booking should be last (oldest)
		qt.Assert(t, *ratesResp.Data.Ratings[2].Requester.Rating, qt.Equals, 5)
		qt.Assert(t, *ratesResp.Data.Ratings[2].Requester.RatingComment, qt.Equals, "First rating")
	})

	// Test 3: GET /bookings/{bookingId}/rate - should return unified ratings
	t.Run("Booking Ratings Order", func(t *testing.T) {
		// For this test, we need to add owner ratings to the bookings
		for i, bookingID := range bookingIDs {
			_, code := c.Request(http.MethodPost, ownerJWT,
				api.RateRequest{
					Rating:  5 - i, // 5, 4, 3
					Comment: fmt.Sprintf("Owner rating %d", i+1),
				},
				"bookings", bookingID, "ratings",
			)
			qt.Assert(t, code, qt.Equals, 200)
		}

		// Check the third booking (newest)
		resp, code := c.Request(http.MethodGet, ownerJWT, nil, "bookings", bookingIDs[2], "ratings")
		qt.Assert(t, code, qt.Equals, 200)

		var ratingResp struct {
			Data *db.UnifiedRating `json:"data"`
		}
		err := json.Unmarshal(resp, &ratingResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify both owner and requester ratings exist
		qt.Assert(t, ratingResp.Data.Owner != nil, qt.IsTrue)
		qt.Assert(t, ratingResp.Data.Requester != nil, qt.IsTrue)
		qt.Assert(t, ratingResp.Data.Owner.Rating != nil, qt.IsTrue)
		qt.Assert(t, ratingResp.Data.Requester.Rating != nil, qt.IsTrue)

		// Verify the ratings are correct
		qt.Assert(t, *ratingResp.Data.Owner.Rating, qt.Equals, 3)
		qt.Assert(t, *ratingResp.Data.Owner.RatingComment, qt.Equals, "Owner rating 3")
		qt.Assert(t, *ratingResp.Data.Requester.Rating, qt.Equals, 3)
		qt.Assert(t, *ratingResp.Data.Requester.RatingComment, qt.Equals, "Third rating")
	})

	// Test 4: GET /bookings/rates - should return pending ratings ordered by newest first
	t.Run("Pending Ratings Order", func(t *testing.T) {
		// Create new bookings that will be pending for rating
		var pendingBookingIDs []string

		// Create first pending booking (oldest)
		req1 := api.CreateBookingRequest{
			ToolID:    fmt.Sprint(toolID),
			StartDate: time.Now().Add(168 * time.Hour).Unix(),
			EndDate:   time.Now().Add(192 * time.Hour).Unix(),
			Contact:   "test@example.com",
			Comments:  "First pending booking",
		}
		resp, code := c.Request(http.MethodPost, renterJWT, req1, "bookings")
		qt.Assert(t, code, qt.Equals, 200)
		var createResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		pendingBookingIDs = append(pendingBookingIDs, createResp.Data.ID)

		// Owner accepts the booking
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", pendingBookingIDs[0])
		qt.Assert(t, code, qt.Equals, 200)

		// Mark as returned
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "RETURNED",
			}, "bookings", pendingBookingIDs[0])
		qt.Assert(t, code, qt.Equals, 200)

		// Wait a bit to ensure different timestamps
		time.Sleep(1 * time.Second)

		// Create second pending booking (newer)
		req2 := api.CreateBookingRequest{
			ToolID:    fmt.Sprint(toolID),
			StartDate: time.Now().Add(216 * time.Hour).Unix(),
			EndDate:   time.Now().Add(240 * time.Hour).Unix(),
			Contact:   "test@example.com",
			Comments:  "Second pending booking",
		}
		resp, code = c.Request(http.MethodPost, renterJWT, req2, "bookings")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &createResp)
		qt.Assert(t, err, qt.IsNil)
		pendingBookingIDs = append(pendingBookingIDs, createResp.Data.ID)

		// Owner accepts the booking
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", pendingBookingIDs[1])
		qt.Assert(t, code, qt.Equals, 200)

		// Mark as returned
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "RETURNED",
			}, "bookings", pendingBookingIDs[1])
		qt.Assert(t, code, qt.Equals, 200)

		// Get pending ratings
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "ratings", "pending")
		qt.Assert(t, code, qt.Equals, 200)

		var pendingResp struct {
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &pendingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(pendingResp.Data.Bookings), qt.Equals, 2)

		// Verify order: newest first
		// The second pending booking should be first (newest)
		qt.Assert(t, pendingResp.Data.Bookings[0].Comments, qt.Equals, "Second pending booking")

		// The first pending booking should be second (oldest)
		qt.Assert(t, pendingResp.Data.Bookings[1].Comments, qt.Equals, "First pending booking")
	})
}
