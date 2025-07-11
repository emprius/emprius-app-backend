package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/db"

	"github.com/emprius/emprius-app-backend/api"
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

// Helper function to get user profile
func getUserProfile(c *utils.TestService, jwt, userID string) (*api.User, error) {
	resp, code := c.Request(http.MethodGet, jwt, nil, "users", userID)
	if code != 200 {
		return nil, fmt.Errorf("failed to get user profile, status code: %d", code)
	}

	var userResp struct {
		Data *api.User `json:"data"`
	}
	err := json.Unmarshal(resp, &userResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user profile: %w", err)
	}

	return userResp.Data, nil
}

// Helper function to create and complete a booking workflow
func createAndCompleteBooking(c *utils.TestService, renterJWT, ownerJWT string, toolID int64) (string, error) {
	// Create booking
	tomorrow := time.Now().Add(24 * time.Hour)
	dayAfterTomorrow := time.Now().Add(48 * time.Hour)

	resp, code := c.Request(http.MethodPost, renterJWT,
		api.CreateBookingRequest{
			ToolID:    fmt.Sprint(toolID),
			StartDate: tomorrow.Unix(),
			EndDate:   dayAfterTomorrow.Unix(),
			Contact:   "test@example.com",
			Comments:  "Test booking for rating",
		},
		"bookings",
	)
	if code != 200 {
		return "", fmt.Errorf("failed to create booking, status code: %d", code)
	}

	var bookingResp struct {
		Data api.BookingResponse `json:"data"`
	}
	err := json.Unmarshal(resp, &bookingResp)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal booking response: %w", err)
	}
	bookingID := bookingResp.Data.ID

	// Owner accepts the booking
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "ACCEPTED",
		}, "bookings", bookingID)
	if code != 200 {
		return "", fmt.Errorf("failed to accept booking, status code: %d", code)
	}

	// Owner marks as returned
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "RETURNED",
		}, "bookings", bookingID)
	if code != 200 {
		return "", fmt.Errorf("failed to mark booking as returned, status code: %d", code)
	}

	return bookingID, nil
}

// Helper function to submit a rating
func submitRating(c *utils.TestService, userJWT, bookingID string, rating int, comment string) error {
	_, code := c.Request(http.MethodPost, userJWT,
		api.RateRequest{
			Rating:  rating,
			Comment: comment,
		},
		"bookings", bookingID, "ratings",
	)
	if code != 200 {
		return fmt.Errorf("failed to submit rating, status code: %d", code)
	}
	return nil
}

func TestSingleRatingUpdatesProfile(t *testing.T) {
	c := utils.NewTestService(t)

	// Create owner and renter
	ownerJWT, ownerID := c.RegisterAndLoginWithID("single-owner@test.com", "owner", "ownerpass")
	renterJWT, _ := c.RegisterAndLoginWithID("single-renter@test.com", "renter", "renterpass")

	// Get baseline owner profile
	baselineProfile, err := getUserProfile(c, ownerJWT, ownerID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, baselineProfile.Rating, qt.Equals, 50, qt.Commentf("Initial rating should be 50"))
	qt.Assert(t, baselineProfile.RatingCount, qt.Equals, 0, qt.Commentf("Initial rating count should be 0"))

	// Create tool and complete booking workflow
	toolID := c.CreateTool(ownerJWT, "Single Rating Test Tool")
	bookingID, err := createAndCompleteBooking(c, renterJWT, ownerJWT, toolID)
	qt.Assert(t, err, qt.IsNil)

	// Submit a 5-star rating from renter to owner
	err = submitRating(c, renterJWT, bookingID, 5, "Excellent service!")
	qt.Assert(t, err, qt.IsNil)

	// Verify owner profile is updated
	updatedProfile, err := getUserProfile(c, ownerJWT, ownerID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, updatedProfile.Rating, qt.Equals, 100, qt.Commentf("Rating should be 100 after 5-star rating"))
	qt.Assert(t, updatedProfile.RatingCount, qt.Equals, 1, qt.Commentf("Rating count should be 1 after first rating"))

	// Also verify via profile endpoint
	resp, code := c.Request(http.MethodGet, ownerJWT, nil, "profile")
	qt.Assert(t, code, qt.Equals, 200)
	var profileResp struct {
		Data *api.User `json:"data"`
	}
	err = json.Unmarshal(resp, &profileResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, profileResp.Data.Rating, qt.Equals, 100, qt.Commentf("Profile endpoint should show rating 100"))
	qt.Assert(t, profileResp.Data.RatingCount, qt.Equals, 1, qt.Commentf("Profile endpoint should show rating count 1"))
}

func TestMultipleRatingsUpdateProfile(t *testing.T) {
	c := utils.NewTestService(t)

	// Create owner and multiple renters
	ownerJWT, ownerID := c.RegisterAndLoginWithID("multi-owner@test.com", "owner", "ownerpass")
	renter1JWT, renter1ID := c.RegisterAndLoginWithID("multi-renter1@test.com", "renter1", "renterpass")
	renter2JWT, renter2ID := c.RegisterAndLoginWithID("multi-renter2@test.com", "renter2", "renterpass")
	renter3JWT, renter3ID := c.RegisterAndLoginWithID("multi-renter3@test.com", "renter3", "renterpass")

	// Get baseline owner profile
	baselineProfile, err := getUserProfile(c, ownerJWT, ownerID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, baselineProfile.Rating, qt.Equals, 50, qt.Commentf("Initial rating should be 50"))
	qt.Assert(t, baselineProfile.RatingCount, qt.Equals, 0, qt.Commentf("Initial rating count should be 0"))

	// Get baseline renter profiles
	baselineRenter1, err := getUserProfile(c, renter1JWT, renter1ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, baselineRenter1.Rating, qt.Equals, 50, qt.Commentf("Initial renter1 rating should be 50"))
	qt.Assert(t, baselineRenter1.RatingCount, qt.Equals, 0, qt.Commentf("Initial renter1 rating count should be 0"))

	baselineRenter2, err := getUserProfile(c, renter2JWT, renter2ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, baselineRenter2.Rating, qt.Equals, 50, qt.Commentf("Initial renter2 rating should be 50"))
	qt.Assert(t, baselineRenter2.RatingCount, qt.Equals, 0, qt.Commentf("Initial renter2 rating count should be 0"))

	baselineRenter3, err := getUserProfile(c, renter3JWT, renter3ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, baselineRenter3.Rating, qt.Equals, 50, qt.Commentf("Initial renter3 rating should be 50"))
	qt.Assert(t, baselineRenter3.RatingCount, qt.Equals, 0, qt.Commentf("Initial renter3 rating count should be 0"))

	// Create tool
	toolID := c.CreateTool(ownerJWT, "Multiple Ratings Test Tool")

	// Complete first booking and submit 5-star rating
	bookingID1, err := createAndCompleteBooking(c, renter1JWT, ownerJWT, toolID)
	qt.Assert(t, err, qt.IsNil)
	err = submitRating(c, renter1JWT, bookingID1, 5, "Excellent!")
	qt.Assert(t, err, qt.IsNil)

	// Verify after first rating
	profile1, err := getUserProfile(c, ownerJWT, ownerID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, profile1.Rating, qt.Equals, 100, qt.Commentf("Rating should be 100 after first 5-star rating"))
	qt.Assert(t, profile1.RatingCount, qt.Equals, 1, qt.Commentf("Rating count should be 1"))

	// Complete second booking and submit 4-star rating
	bookingID2, err := createAndCompleteBooking(c, renter2JWT, ownerJWT, toolID)
	qt.Assert(t, err, qt.IsNil)
	err = submitRating(c, renter2JWT, bookingID2, 4, "Very good!")
	qt.Assert(t, err, qt.IsNil)

	// Verify after second rating (average of 5 and 4 = 4.5 = 90)
	profile2, err := getUserProfile(c, ownerJWT, ownerID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, profile2.Rating, qt.Equals, 90, qt.Commentf("Rating should be 90 after averaging 5 and 4 stars"))
	qt.Assert(t, profile2.RatingCount, qt.Equals, 2, qt.Commentf("Rating count should be 2"))

	// Complete third booking and submit 3-star rating
	bookingID3, err := createAndCompleteBooking(c, renter3JWT, ownerJWT, toolID)
	qt.Assert(t, err, qt.IsNil)
	err = submitRating(c, renter3JWT, bookingID3, 3, "Good")
	qt.Assert(t, err, qt.IsNil)

	// Verify after third rating (average of 5, 4, 3 = 4.0 = 80)
	profile3, err := getUserProfile(c, ownerJWT, ownerID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, profile3.Rating, qt.Equals, 80, qt.Commentf("Rating should be 80 after averaging 5, 4, and 3 stars"))
	qt.Assert(t, profile3.RatingCount, qt.Equals, 3, qt.Commentf("Rating count should be 3"))

	// Now test owner rating renters - submit ratings from owner to each renter
	// Owner rates renter1 with 4 stars
	err = submitRating(c, ownerJWT, bookingID1, 4, "Good renter!")
	qt.Assert(t, err, qt.IsNil)

	// Verify renter1 profile is updated (4 stars = 80%)
	renter1Profile, err := getUserProfile(c, renter1JWT, renter1ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, renter1Profile.Rating, qt.Equals, 80, qt.Commentf("Renter1 rating should be 80 after 4-star rating"))
	qt.Assert(t, renter1Profile.RatingCount, qt.Equals, 1, qt.Commentf("Renter1 rating count should be 1"))

	// Owner rates renter2 with 5 stars
	err = submitRating(c, ownerJWT, bookingID2, 5, "Excellent renter!")
	qt.Assert(t, err, qt.IsNil)

	// Verify renter2 profile is updated (5 stars = 100%)
	renter2Profile, err := getUserProfile(c, renter2JWT, renter2ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, renter2Profile.Rating, qt.Equals, 100, qt.Commentf("Renter2 rating should be 100 after 5-star rating"))
	qt.Assert(t, renter2Profile.RatingCount, qt.Equals, 1, qt.Commentf("Renter2 rating count should be 1"))

	// Owner rates renter3 with 3 stars
	err = submitRating(c, ownerJWT, bookingID3, 3, "Average renter")
	qt.Assert(t, err, qt.IsNil)

	// Verify renter3 profile is updated (3 stars = 60%)
	renter3Profile, err := getUserProfile(c, renter3JWT, renter3ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, renter3Profile.Rating, qt.Equals, 60, qt.Commentf("Renter3 rating should be 60 after 3-star rating"))
	qt.Assert(t, renter3Profile.RatingCount, qt.Equals, 1, qt.Commentf("Renter3 rating count should be 1"))

	// Verify owner profile remains unchanged (owner ratings don't affect owner's own profile)
	finalOwnerProfile, err := getUserProfile(c, ownerJWT, ownerID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, finalOwnerProfile.Rating, qt.Equals, 80, qt.Commentf("Owner rating should remain 80"))
	qt.Assert(t, finalOwnerProfile.RatingCount, qt.Equals, 3, qt.Commentf("Owner rating count should remain 3"))
}

func TestRatingCalculationAccuracy(t *testing.T) {
	c := utils.NewTestService(t)

	t.Run("Single 5-star rating", func(t *testing.T) {
		ownerJWT, ownerID := c.RegisterAndLoginWithID("calc-owner1@test.com", "owner1", "ownerpass")
		renterJWT, _ := c.RegisterAndLoginWithID("calc-renter1@test.com", "renter1", "renterpass")

		toolID := c.CreateTool(ownerJWT, "Calculation Test Tool 1")
		bookingID, err := createAndCompleteBooking(c, renterJWT, ownerJWT, toolID)
		qt.Assert(t, err, qt.IsNil)

		err = submitRating(c, renterJWT, bookingID, 5, "Perfect!")
		qt.Assert(t, err, qt.IsNil)

		profile, err := getUserProfile(c, ownerJWT, ownerID)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, profile.Rating, qt.Equals, 100, qt.Commentf("5 stars should equal 100"))
		qt.Assert(t, profile.RatingCount, qt.Equals, 1)
	})

	t.Run("Single 4-star rating", func(t *testing.T) {
		ownerJWT, ownerID := c.RegisterAndLoginWithID("calc-owner2@test.com", "owner2", "ownerpass")
		renterJWT, _ := c.RegisterAndLoginWithID("calc-renter2@test.com", "renter2", "renterpass")

		toolID := c.CreateTool(ownerJWT, "Calculation Test Tool 2")
		bookingID, err := createAndCompleteBooking(c, renterJWT, ownerJWT, toolID)
		qt.Assert(t, err, qt.IsNil)

		err = submitRating(c, renterJWT, bookingID, 4, "Very good!")
		qt.Assert(t, err, qt.IsNil)

		profile, err := getUserProfile(c, ownerJWT, ownerID)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, profile.Rating, qt.Equals, 80, qt.Commentf("4 stars should equal 80"))
		qt.Assert(t, profile.RatingCount, qt.Equals, 1)
	})

	t.Run("Single 3-star rating", func(t *testing.T) {
		ownerJWT, ownerID := c.RegisterAndLoginWithID("calc-owner3@test.com", "owner3", "ownerpass")
		renterJWT, _ := c.RegisterAndLoginWithID("calc-renter3@test.com", "renter3", "renterpass")

		toolID := c.CreateTool(ownerJWT, "Calculation Test Tool 3")
		bookingID, err := createAndCompleteBooking(c, renterJWT, ownerJWT, toolID)
		qt.Assert(t, err, qt.IsNil)

		err = submitRating(c, renterJWT, bookingID, 3, "Good")
		qt.Assert(t, err, qt.IsNil)

		profile, err := getUserProfile(c, ownerJWT, ownerID)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, profile.Rating, qt.Equals, 60, qt.Commentf("3 stars should equal 60"))
		qt.Assert(t, profile.RatingCount, qt.Equals, 1)
	})

	t.Run("Multiple same ratings", func(t *testing.T) {
		ownerJWT, ownerID := c.RegisterAndLoginWithID("calc-owner4@test.com", "owner4", "ownerpass")
		renter1JWT, _ := c.RegisterAndLoginWithID("calc-renter4a@test.com", "renter4a", "renterpass")
		renter2JWT, _ := c.RegisterAndLoginWithID("calc-renter4b@test.com", "renter4b", "renterpass")

		toolID := c.CreateTool(ownerJWT, "Calculation Test Tool 4")

		// Submit two 4-star ratings
		bookingID1, err := createAndCompleteBooking(c, renter1JWT, ownerJWT, toolID)
		qt.Assert(t, err, qt.IsNil)
		err = submitRating(c, renter1JWT, bookingID1, 4, "Very good!")
		qt.Assert(t, err, qt.IsNil)

		bookingID2, err := createAndCompleteBooking(c, renter2JWT, ownerJWT, toolID)
		qt.Assert(t, err, qt.IsNil)
		err = submitRating(c, renter2JWT, bookingID2, 4, "Very good!")
		qt.Assert(t, err, qt.IsNil)

		profile, err := getUserProfile(c, ownerJWT, ownerID)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, profile.Rating, qt.Equals, 80, qt.Commentf("Two 4-star ratings should average to 80"))
		qt.Assert(t, profile.RatingCount, qt.Equals, 2)
	})

	t.Run("Mixed ratings calculation", func(t *testing.T) {
		ownerJWT, ownerID := c.RegisterAndLoginWithID("calc-owner5@test.com", "owner5", "ownerpass")
		renter1JWT, _ := c.RegisterAndLoginWithID("calc-renter5a@test.com", "renter5a", "renterpass")
		renter2JWT, _ := c.RegisterAndLoginWithID("calc-renter5b@test.com", "renter5b", "renterpass")
		renter3JWT, _ := c.RegisterAndLoginWithID("calc-renter5c@test.com", "renter5c", "renterpass")
		renter4JWT, _ := c.RegisterAndLoginWithID("calc-renter5d@test.com", "renter5d", "renterpass")

		toolID := c.CreateTool(ownerJWT, "Calculation Test Tool 5")

		// Submit ratings: 5, 4, 3, 2 stars (average = 3.5 = 70)
		ratings := []struct {
			jwt     string
			rating  int
			comment string
		}{
			{renter1JWT, 5, "Excellent!"},
			{renter2JWT, 4, "Very good!"},
			{renter3JWT, 3, "Good"},
			{renter4JWT, 2, "Fair"},
		}

		for _, r := range ratings {
			bookingID, err := createAndCompleteBooking(c, r.jwt, ownerJWT, toolID)
			qt.Assert(t, err, qt.IsNil)
			err = submitRating(c, r.jwt, bookingID, r.rating, r.comment)
			qt.Assert(t, err, qt.IsNil)
		}

		profile, err := getUserProfile(c, ownerJWT, ownerID)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, profile.Rating, qt.Equals, 70, qt.Commentf("Average of 5,4,3,2 stars should be 70"))
		qt.Assert(t, profile.RatingCount, qt.Equals, 4)
	})
}
