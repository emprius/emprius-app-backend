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

func TestBookings(t *testing.T) {
	c := utils.NewTestService(t)

	// Create two users: tool owner and renter
	ownerJWT, ownerID := c.RegisterAndLoginWithID("owner@test.com", "owner", "ownerpass")
	renterJWT, renterID := c.RegisterAndLoginWithID("renter@test.com", "renter", "renterpass")

	// Owner creates a tool
	toolID := c.CreateTool(ownerJWT, "Test Tool")

	t.Run("Create Booking", func(t *testing.T) {
		// Try to create booking without auth
		_, code := c.Request(http.MethodPost, "",
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: time.Now().Add(24 * time.Hour).Unix(),
				EndDate:   time.Now().Add(48 * time.Hour).Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 401)

		// Create booking with auth
		resp, code := c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: time.Now().Add(24 * time.Hour).Unix(),
				EndDate:   time.Now().Add(48 * time.Hour).Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var response struct {
			Data api.BookingResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &response)
		qt.Assert(t, err, qt.IsNil)
		bookingID := response.Data.ID

		// Create overlapping booking (should be allowed since first booking is pending)
		_, code = c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: time.Now().Add(36 * time.Hour).Unix(),
				EndDate:   time.Now().Add(60 * time.Hour).Unix(),
				Contact:   "test2@example.com",
				Comments:  "Test booking 2",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Accept first booking
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Try to create another overlapping booking (should fail since there's an accepted booking)
		data, code := c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: time.Now().Add(36 * time.Hour).Unix(),
				EndDate:   time.Now().Add(60 * time.Hour).Unix(),
				Contact:   "test3@example.com",
				Comments:  "Test booking 3",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 400, qt.Commentf("Response: %s", string(data)))

		// Get booking requests (owner)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests", "incoming")
		qt.Assert(t, code, qt.Equals, 200)
		var requestsResp struct {
			Data []api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &requestsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(requestsResp.Data), qt.Equals, 2)

		// Verify one booking is accepted and one is pending
		var hasAccepted, hasPending bool
		for _, booking := range requestsResp.Data {
			if booking.BookingStatus == "ACCEPTED" {
				hasAccepted = true
			} else if booking.BookingStatus == "PENDING" {
				hasPending = true
			}
		}
		qt.Assert(t, hasAccepted, qt.IsTrue)
		qt.Assert(t, hasPending, qt.IsTrue)

		// Get booking petitions (renter)
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing")
		qt.Assert(t, code, qt.Equals, 200)
		var petitionsResp struct {
			Data []api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &petitionsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(petitionsResp.Data), qt.Equals, 2)

		// Verify one booking is accepted and one is pending
		hasAccepted = false
		hasPending = false
		for _, booking := range petitionsResp.Data {
			if booking.BookingStatus == "ACCEPTED" {
				hasAccepted = true
			} else if booking.BookingStatus == "PENDING" {
				hasPending = true
			}
		}
		qt.Assert(t, hasAccepted, qt.IsTrue)
		qt.Assert(t, hasPending, qt.IsTrue)

		// Get specific booking
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)
		var bookingResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, bookingResp.Data.ID, qt.Equals, bookingID)

		// Try to mark as returned by renter (should fail)
		_, code = c.Request(http.MethodPut, renterJWT,
			&api.BookingStatusUpdate{
				Status: "RETURNED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 403)

		// Mark as returned by owner
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "RETURNED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Get pending ratings
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "ratings", "pending")
		qt.Assert(t, code, qt.Equals, 200)
		var ratingsResp struct {
			Data []api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &ratingsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(ratingsResp.Data), qt.Equals, 1)

		// Test rating functionality
		t.Run("Rating Tests", func(t *testing.T) {
			// First test: Basic rating functionality
			// Try to rate without auth
			_, code = c.Request(http.MethodPost, "",
				api.RateRequest{
					Rating:  5,
					Comment: "Try to rate without auth",
				},
				"bookings", bookingID, "ratings",
			)
			qt.Assert(t, code, qt.Equals, 401)

			// Try to rate with invalid rating value
			_, code = c.Request(http.MethodPost, renterJWT,
				api.RateRequest{
					Rating:  6,
					Comment: "Try to rate with invalid rating value",
				},
				"bookings", bookingID, "ratings",
			)
			qt.Assert(t, code, qt.Equals, 400)

			// Try to rate with invalid rating value (too low)
			_, code = c.Request(http.MethodPost, renterJWT,
				api.RateRequest{
					Rating:  0,
					Comment: "Try to rate with invalid rating value (too low)",
				},
				"bookings", bookingID, "ratings",
			)
			qt.Assert(t, code, qt.Equals, 400)

			// Submit valid rating with comment
			_, code = c.Request(http.MethodPost, renterJWT,
				api.RateRequest{
					Rating:  5,
					Comment: "Great experience!",
				},
				"bookings", bookingID, "ratings",
			)
			qt.Assert(t, code, qt.Equals, 200)

			// Try to rate again (should fail)
			_, code = c.Request(http.MethodPost, renterJWT,
				api.RateRequest{
					Rating:  4,
					Comment: "Try to rate again (should fail)",
				},
				"bookings", bookingID, "ratings",
			)
			qt.Assert(t, code, qt.Equals, 403)

			// Get submitted ratings
			resp, code = c.Request(http.MethodGet, renterJWT, nil, "users", renterID, "ratings")
			qt.Assert(t, code, qt.Equals, 200)
			var submittedResp struct {
				Data []*db.UnifiedRating `json:"data"`
			}
			err = json.Unmarshal(resp, &submittedResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(submittedResp.Data), qt.Equals, 1)
			qt.Assert(t, *submittedResp.Data[0].Requester.Rating, qt.Equals, 5)
			qt.Assert(t, *submittedResp.Data[0].Requester.RatingComment, qt.Equals, "Great experience!")

			// Get received ratings (owner)
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "users", ownerID, "ratings")
			qt.Assert(t, code, qt.Equals, 200)
			var receivedResp struct {
				Data []*db.UnifiedRating `json:"data"`
			}
			err = json.Unmarshal(resp, &receivedResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(receivedResp.Data), qt.Equals, 1)
			qt.Assert(t, *receivedResp.Data[0].Requester.Rating, qt.Equals, 5)

			// Second test: Owner rating their own booking
			// Create a new booking where owner will rate it
			resp, code = c.Request(http.MethodPost, ownerJWT,
				api.RateRequest{
					Rating:  4,
					Comment: "Self rating test",
				},
				"bookings", bookingID, "ratings")
			qt.Assert(t, code, qt.Equals, 200)

			// Get submitted ratings
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "users", ownerID, "ratings")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &submittedResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(submittedResp.Data), qt.Equals, 1)
			qt.Assert(t, *submittedResp.Data[0].Owner.Rating, qt.Equals, 4)

			// Test the new GET /bookings/{bookingId}/rate endpoint
			// Get ratings for the booking
			resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", bookingID, "ratings")
			qt.Assert(t, code, qt.Equals, 200)
			var bookingRatingsResp struct {
				Data *db.UnifiedRating `json:"data"`
			}
			err = json.Unmarshal(resp, &bookingRatingsResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, *bookingRatingsResp.Data.Owner.Rating, qt.Equals, 4)
			qt.Assert(t, *bookingRatingsResp.Data.Owner.RatingComment, qt.Equals, "Self rating test")
			qt.Assert(t, *bookingRatingsResp.Data.Requester.Rating, qt.Equals, 5)
			qt.Assert(t, *bookingRatingsResp.Data.Requester.RatingComment, qt.Equals, "Great experience!")

			// Try to get ratings for non-existent booking
			_, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "nonexistentid", "ratings")
			qt.Assert(t, code, qt.Equals, 400) // Invalid ID format

			// Try to get ratings without auth
			_, code = c.Request(http.MethodGet, "", nil, "bookings", bookingID, "ratings")
			qt.Assert(t, code, qt.Equals, 401)
		})

		// todo(kon): recreate this tests after rest api refactoring
		// Test deny petition
		//t.Run("Deny Petition", func(t *testing.T) {
		//	// Create a new booking to deny
		//	resp, code := c.Request(http.MethodPost, renterJWT,
		//		api.CreateBookingRequest{
		//			ToolID:    fmt.Sprint(toolID),
		//			StartDate: time.Now().Add(72 * time.Hour).Unix(),
		//			EndDate:   time.Now().Add(96 * time.Hour).Unix(),
		//			Contact:   "test@example.com",
		//			Comments:  "Test booking to deny",
		//		},
		//		"bookings",
		//	)
		//	qt.Assert(t, code, qt.Equals, 200)
		//
		//	var response struct {
		//		Data api.BookingResponse `json:"data"`
		//	}
		//	err := json.Unmarshal(resp, &response)
		//	qt.Assert(t, err, qt.IsNil)
		//	denyBookingID := response.Data.ID
		//
		//	// Try to deny without auth
		//	_, code = c.Request(http.MethodPost, "",
		//		&api.BookingStatusUpdate{
		//			Status: "REJECTED",
		//		}, "bookings", denyBookingID)
		//	qt.Assert(t, code, qt.Equals, 401)
		//
		//	// Try to deny as renter (should fail)
		//	_, code = c.Request(http.MethodPost, renterJWT,
		//		&api.BookingStatusUpdate{
		//			Status: "REJECTED",
		//		}, "bookings", denyBookingID)
		//	qt.Assert(t, code, qt.Equals, 403)
		//
		//	// Deny as owner
		//	_, code = c.Request(http.MethodPost, ownerJWT,
		//		&api.BookingStatusUpdate{
		//			Status: "REJECTED",
		//		}, "bookings", denyBookingID)
		//	qt.Assert(t, code, qt.Equals, 200)
		//
		//	// Verify booking status is REJECTED
		//	resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", denyBookingID)
		//	qt.Assert(t, code, qt.Equals, 200)
		//	var bookingResp struct {
		//		Data api.BookingResponse `json:"data"`
		//	}
		//	err = json.Unmarshal(resp, &bookingResp)
		//	qt.Assert(t, err, qt.IsNil)
		//	qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "REJECTED")
		//})

		// Test cancel request
		//t.Run("Cancel Request", func(t *testing.T) {
		//	// Create a new booking to cancel
		//	resp, code := c.Request(http.MethodPost, renterJWT,
		//		api.CreateBookingRequest{
		//			ToolID:    fmt.Sprint(toolID),
		//			StartDate: time.Now().Add(120 * time.Hour).Unix(),
		//			EndDate:   time.Now().Add(144 * time.Hour).Unix(),
		//			Contact:   "test@example.com",
		//			Comments:  "Test booking to cancel",
		//		},
		//		"bookings",
		//	)
		//	qt.Assert(t, code, qt.Equals, 200)
		//
		//	var response struct {
		//		Data api.BookingResponse `json:"data"`
		//	}
		//	err := json.Unmarshal(resp, &response)
		//	qt.Assert(t, err, qt.IsNil)
		//	cancelBookingID := response.Data.ID
		//
		//	// Try to cancel without auth
		//	_, code = c.Request(http.MethodPost, "",
		//		&api.BookingStatusUpdate{
		//			Status: "CANCELLED",
		//		}, "bookings", cancelBookingID)
		//	qt.Assert(t, code, qt.Equals, 401)
		//
		//	// Try to cancel as owner (should fail)
		//	_, code = c.Request(http.MethodPost, ownerJWT,
		//		&api.BookingStatusUpdate{
		//			Status: "CANCELLED",
		//		}, "bookings", cancelBookingID)
		//	qt.Assert(t, code, qt.Equals, 403)
		//
		//	// Cancel as renter
		//	_, code = c.Request(http.MethodPost, renterJWT,
		//		&api.BookingStatusUpdate{
		//			Status: "CANCELLED",
		//		}, "bookings", cancelBookingID)
		//	qt.Assert(t, code, qt.Equals, 200)
		//
		//	// Verify booking status is CANCELLED
		//	resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", cancelBookingID)
		//	qt.Assert(t, code, qt.Equals, 200)
		//	var bookingResp struct {
		//		Data api.BookingResponse `json:"data"`
		//	}
		//	err = json.Unmarshal(resp, &bookingResp)
		//	qt.Assert(t, err, qt.IsNil)
		//	qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "CANCELLED")
		//})

		// Test paginated user bookings
		//t.Run("Get User Bookings", func(t *testing.T) {
		//	// Get first page of bookings
		//	resp, code := c.Request(http.MethodGet, renterJWT, nil, "bookings", "user", renterID, "?page=0")
		//	qt.Assert(t, code, qt.Equals, 200)
		//	var pageResp struct {
		//		Data []api.BookingResponse `json:"data"`
		//	}
		//	err := json.Unmarshal(resp, &pageResp)
		//	qt.Assert(t, err, qt.IsNil)
		//	qt.Assert(t, len(pageResp.Data), qt.Equals, 4) // Should show all bookings (accepted, pending, denied, and cancelled)
		//
		//	// Test invalid page number
		//	_, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "user", renterID, "?page=-1")
		//	qt.Assert(t, code, qt.Equals, 400)
		//
		//	// Test with non-existent user ID
		//	_, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "user", "invalid-id")
		//	qt.Assert(t, code, qt.Equals, 400)
		//
		//	// Test without authentication
		//	_, code = c.Request(http.MethodGet, "", nil, "bookings", "user", renterID)
		//	qt.Assert(t, code, qt.Equals, 401)
		//})

		// Test count pending actions
		t.Run("Count Pending Actions", func(t *testing.T) {
			// Create two users: tool owner and renter
			ownerJWT := c.RegisterAndLogin("owner2@test.com", "owner2", "ownerpass")
			renterJWT := c.RegisterAndLogin("renter2@test.com", "renter2", "renterpass")

			// Try to get count without auth
			_, code := c.Request(http.MethodGet, "", nil, "bookings", "requests", "outgoing")
			qt.Assert(t, code, qt.Equals, 401)

			// Create a tool for owner
			toolID := c.CreateTool(ownerJWT, "Test Tool")

			// Get count for owner (should have 0 pending booking)
			resp, code := c.Request(http.MethodGet, ownerJWT, nil, "profile", "pendings")
			qt.Assert(t, code, qt.Equals, 200)
			var countResp struct {
				Data *db.CountPendingActionsResponse `json:"data"`
			}
			err := json.Unmarshal(resp, &countResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, countResp.Data.PendingRequestsCount, qt.Equals, int64(0))
			qt.Assert(t, countResp.Data.PendingRatingsCount, qt.Equals, int64(0))

			// Create a pending booking from renter to owner
			_, code = c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(toolID),
					StartDate: time.Now().Add(168 * time.Hour).Unix(),
					EndDate:   time.Now().Add(192 * time.Hour).Unix(),
					Contact:   "test@example.com",
					Comments:  "Another pending booking",
				},
				"bookings",
			)
			qt.Assert(t, code, qt.Equals, 200)

			// Verify owner now has 1 pending actions
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "profile", "pendings")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &countResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, countResp.Data.PendingRequestsCount, qt.Equals, int64(1))
			qt.Assert(t, countResp.Data.PendingRatingsCount, qt.Equals, int64(0))

			// Get booking requests (owner)
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests", "incoming")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &petitionsResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(petitionsResp.Data), qt.Equals, 1)

			// Accept the booking request
			bookingID := petitionsResp.Data[0].ID
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", bookingID)
			qt.Assert(t, code, qt.Equals, 200)

			// Mark booking as returned
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "RETURNED",
				}, "bookings", bookingID)
			qt.Assert(t, code, qt.Equals, 200)

			// Verify owner now has 0 pending request and 0 pending rating
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "profile", "pendings")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &countResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, countResp.Data.PendingRequestsCount, qt.Equals, int64(0))
			qt.Assert(t, countResp.Data.PendingRatingsCount, qt.Equals, int64(1))

			// Verify renter has 1 pending rating
			resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing")
			qt.Assert(t, code, qt.Equals, 200)
			var petitionsResp struct {
				Data []api.BookingResponse `json:"data"`
			}
			err = json.Unmarshal(resp, &petitionsResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, countResp.Data.PendingRequestsCount, qt.Equals, int64(0))
			qt.Assert(t, countResp.Data.PendingRatingsCount, qt.Equals, int64(1))
		})

		t.Run("Owner Overall Rating", func(t *testing.T) {
			// Scenario A: Only the renter rates the booking.
			{
				// Create a new booking.
				resp, code := c.Request(http.MethodPost, renterJWT,
					api.CreateBookingRequest{
						ToolID:    fmt.Sprint(toolID),
						StartDate: time.Now().Add(24 * time.Hour).Unix(),
						EndDate:   time.Now().Add(48 * time.Hour).Unix(),
						Contact:   "test@example.com",
						Comments:  "Booking for rating test A",
					},
					"bookings",
				)
				qt.Assert(t, code, qt.Equals, 200)

				var bookingResp struct {
					Data api.BookingResponse `json:"data"`
				}
				err := json.Unmarshal(resp, &bookingResp)
				qt.Assert(t, err, qt.IsNil)
				bookingID := bookingResp.Data.ID

				// Owner accepts the booking.
				_, code = c.Request(http.MethodPut, ownerJWT,
					&api.BookingStatusUpdate{
						Status: "ACCEPTED",
					}, "bookings", bookingID)
				qt.Assert(t, code, qt.Equals, 200)

				// Mark booking as returned.
				_, code = c.Request(http.MethodPut, ownerJWT,
					&api.BookingStatusUpdate{
						Status: "RETURNED",
					}, "bookings", bookingID)
				qt.Assert(t, code, qt.Equals, 200)

				// Renter submits a rating of 5.
				_, code = c.Request(http.MethodPost, renterJWT,
					api.RateRequest{
						Rating:  5,
						Comment: "Excellent!",
					},
					"bookings", bookingID, "ratings",
				)
				qt.Assert(t, code, qt.Equals, 200)

				// Retrieve the owner's profile.
				resp, code = c.Request(http.MethodGet, ownerJWT, nil, "profile")
				qt.Assert(t, code, qt.Equals, 200)
				var profileResp struct {
					Data *api.User `json:"data"`
				}
				err = json.Unmarshal(resp, &profileResp)
				qt.Assert(t, err, qt.IsNil)
				// Expect owner's overall rating to be 100 (5 stars → 100%).
				qt.Assert(t, profileResp.Data.Rating, qt.Equals, 100)
			}

			// Scenario B: Both the renter and the owner rate the booking.
			{
				// Create a new booking.
				resp, code := c.Request(http.MethodPost, renterJWT,
					api.CreateBookingRequest{
						ToolID:    fmt.Sprint(toolID),
						StartDate: time.Now().Add(72 * time.Hour).Unix(),
						EndDate:   time.Now().Add(96 * time.Hour).Unix(),
						Contact:   "test@example.com",
						Comments:  "Booking for rating test B",
					},
					"bookings",
				)
				qt.Assert(t, code, qt.Equals, 200)

				var bookingResp struct {
					Data api.BookingResponse `json:"data"`
				}
				err := json.Unmarshal(resp, &bookingResp)
				qt.Assert(t, err, qt.IsNil)
				bookingID := bookingResp.Data.ID

				// Owner accepts the booking.
				_, code = c.Request(http.MethodPut, ownerJWT,
					&api.BookingStatusUpdate{
						Status: "ACCEPTED",
					}, "bookings", bookingID)
				qt.Assert(t, code, qt.Equals, 200)

				// Mark booking as returned.
				_, code = c.Request(http.MethodPut, ownerJWT,
					&api.BookingStatusUpdate{
						Status: "RETURNED",
					}, "bookings", bookingID)
				qt.Assert(t, code, qt.Equals, 200)

				// Renter submits a rating of 5.
				_, code = c.Request(http.MethodPost, renterJWT,
					api.RateRequest{
						Rating:  5,
						Comment: "Excellent!",
					},
					"bookings", bookingID, "ratings",
				)
				qt.Assert(t, code, qt.Equals, 200)

				// Owner submits a rating of 4.
				_, code = c.Request(http.MethodPost, ownerJWT,
					api.RateRequest{
						Rating:  4,
						Comment: "Good, but could improve",
					},
					"bookings", bookingID, "ratings",
				)
				qt.Assert(t, code, qt.Equals, 200)

				// Retrieve the owner's profile.
				resp, code = c.Request(http.MethodGet, ownerJWT, nil, "profile")
				qt.Assert(t, code, qt.Equals, 200)
				var profileResp struct {
					Data *api.User `json:"data"`
				}
				err = json.Unmarshal(resp, &profileResp)
				qt.Assert(t, err, qt.IsNil)
				qt.Assert(t, profileResp.Data.Rating, qt.Equals, 100)
			}
		})

		// Test the unified ratings endpoint
		t.Run("Unified Ratings", func(t *testing.T) {
			// Create a new booking
			resp, code := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(toolID),
					StartDate: time.Now().Add(200 * time.Hour).Unix(),
					EndDate:   time.Now().Add(224 * time.Hour).Unix(),
					Contact:   "test@example.com",
					Comments:  "Test booking for unified ratings",
				},
				"bookings",
			)
			qt.Assert(t, code, qt.Equals, 200)

			var bookingResp struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp, &bookingResp)
			qt.Assert(t, err, qt.IsNil)
			bookingID := bookingResp.Data.ID

			// Owner accepts the booking
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", bookingID)
			qt.Assert(t, code, qt.Equals, 200)

			// Mark booking as returned
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "RETURNED",
				}, "bookings", bookingID)
			qt.Assert(t, code, qt.Equals, 200)

			// Renter submits a rating
			_, code = c.Request(http.MethodPost, renterJWT,
				api.RateRequest{
					Rating:  4,
					Comment: "Good experience with the tool",
				},
				"bookings", bookingID, "ratings",
			)
			qt.Assert(t, code, qt.Equals, 200)

			// Owner submits a rating
			_, code = c.Request(http.MethodPost, ownerJWT,
				api.RateRequest{
					Rating:  5,
					Comment: "Great renter, returned the tool in perfect condition",
				},
				"bookings", bookingID, "ratings",
			)
			qt.Assert(t, code, qt.Equals, 200)

			// Get unified ratings for the renter
			resp, code = c.Request(http.MethodGet, renterJWT, nil, "users", renterID, "ratings")
			qt.Assert(t, code, qt.Equals, 200)

			var unifiedResp struct {
				Data []*db.UnifiedRating `json:"data"`
			}
			err = json.Unmarshal(resp, &unifiedResp)
			qt.Assert(t, err, qt.IsNil)

			// Verify we have at least one unified rating
			qt.Assert(t, len(unifiedResp.Data) > 0, qt.IsTrue)

			// Find the rating for our test booking
			var testBookingRating *db.UnifiedRating
			for _, rating := range unifiedResp.Data {
				if rating.BookingID.Hex() == bookingID {
					testBookingRating = rating
					break
				}
			}

			// Verify the unified rating contains both owner and requester ratings
			qt.Assert(t, testBookingRating, qt.Not(qt.IsNil))
			qt.Assert(t, testBookingRating.Owner, qt.Not(qt.IsNil))
			qt.Assert(t, testBookingRating.Requester, qt.Not(qt.IsNil))

			// Verify owner rating
			qt.Assert(t, testBookingRating.Owner.Rating, qt.Not(qt.IsNil))
			qt.Assert(t, *testBookingRating.Owner.Rating, qt.Equals, 5)
			qt.Assert(t, testBookingRating.Owner.RatingComment, qt.Not(qt.IsNil))
			qt.Assert(t, *testBookingRating.Owner.RatingComment, qt.Equals, "Great renter, returned the tool in perfect condition")

			// Verify requester rating
			qt.Assert(t, testBookingRating.Requester.Rating, qt.Not(qt.IsNil))
			qt.Assert(t, *testBookingRating.Requester.Rating, qt.Equals, 4)
			qt.Assert(t, testBookingRating.Requester.RatingComment, qt.Not(qt.IsNil))
			qt.Assert(t, *testBookingRating.Requester.RatingComment, qt.Equals, "Good experience with the tool")

			// Get unified ratings for the owner
			ownerID := bookingResp.Data.ToUserID
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "users", ownerID, "ratings")
			qt.Assert(t, code, qt.Equals, 200)

			err = json.Unmarshal(resp, &unifiedResp)
			qt.Assert(t, err, qt.IsNil)

			// Verify we have at least one unified rating
			qt.Assert(t, len(unifiedResp.Data) > 0, qt.IsTrue)

			// Find the rating for our test booking
			testBookingRating = nil
			for _, rating := range unifiedResp.Data {
				if rating.BookingID.Hex() == bookingID {
					testBookingRating = rating
					break
				}
			}

			// Verify the unified rating contains both owner and requester ratings
			qt.Assert(t, testBookingRating, qt.Not(qt.IsNil))
			qt.Assert(t, testBookingRating.Owner, qt.Not(qt.IsNil))
			qt.Assert(t, testBookingRating.Requester, qt.Not(qt.IsNil))

			// Verify owner rating
			qt.Assert(t, testBookingRating.Owner.Rating, qt.Not(qt.IsNil))
			qt.Assert(t, *testBookingRating.Owner.Rating, qt.Equals, 5)
			qt.Assert(t, testBookingRating.Owner.RatingComment, qt.Not(qt.IsNil))
			qt.Assert(t, *testBookingRating.Owner.RatingComment, qt.Equals, "Great renter, returned the tool in perfect condition")

			// Verify requester rating
			qt.Assert(t, testBookingRating.Requester.Rating, qt.Not(qt.IsNil))
			qt.Assert(t, *testBookingRating.Requester.Rating, qt.Equals, 4)
			qt.Assert(t, testBookingRating.Requester.RatingComment, qt.Not(qt.IsNil))
			qt.Assert(t, *testBookingRating.Requester.RatingComment, qt.Equals, "Good experience with the tool")
		})
	})

	t.Run("IsRated Attribute", func(t *testing.T) {
		// Create a new booking
		resp, code := c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: time.Now().Add(250 * time.Hour).Unix(),
				EndDate:   time.Now().Add(274 * time.Hour).Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking for isRated attribute",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var bookingResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		bookingID := bookingResp.Data.ID

		// Owner accepts the booking
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Mark booking as returned
		_, code = c.Request(http.MethodPut, ownerJWT,
			&api.BookingStatusUpdate{
				Status: "RETURNED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Check isRated attribute before rating (should be false)
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, bookingResp.Data.IsRated, qt.Equals, false, qt.Commentf("IsRated should be false before rating"))

		// Check isRated attribute in outgoing requests list (should be false)
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing")
		qt.Assert(t, code, qt.Equals, 200)
		var outgoingResp struct {
			Data []api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &outgoingResp)
		qt.Assert(t, err, qt.IsNil)

		var foundBooking bool
		for _, booking := range outgoingResp.Data {
			if booking.ID == bookingID {
				foundBooking = true
				qt.Assert(t, booking.IsRated, qt.Equals, false, qt.Commentf("IsRated should be false in outgoing requests before rating"))
				break
			}
		}
		qt.Assert(t, foundBooking, qt.IsTrue, qt.Commentf("Booking should be found in outgoing requests"))

		// Renter submits a rating
		_, code = c.Request(http.MethodPost, renterJWT,
			api.RateRequest{
				Rating:  4,
				Comment: "Testing isRated attribute",
			},
			"bookings", bookingID, "ratings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Check isRated attribute after rating (should be true)
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, bookingResp.Data.IsRated, qt.Equals, true, qt.Commentf("IsRated should be true after rating"))

		// Check isRated attribute in outgoing requests list (should be true)
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &outgoingResp)
		qt.Assert(t, err, qt.IsNil)

		foundBooking = false
		for _, booking := range outgoingResp.Data {
			if booking.ID == bookingID {
				foundBooking = true
				qt.Assert(t, booking.IsRated, qt.Equals, true, qt.Commentf("IsRated should be true in outgoing requests after rating"))
				break
			}
		}
		qt.Assert(t, foundBooking, qt.IsTrue, qt.Commentf("Booking should be found in outgoing requests"))

		// Check from owner's perspective (should be false since owner hasn't rated yet)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, bookingResp.Data.IsRated, qt.Equals, false, qt.Commentf("IsRated should be false for owner who hasn't rated yet"))

		// Check isRated attribute in incoming requests list (should be false for owner)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests", "incoming")
		qt.Assert(t, code, qt.Equals, 200)
		var incomingResp struct {
			Data []api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &incomingResp)
		qt.Assert(t, err, qt.IsNil)

		foundBooking = false
		for _, booking := range incomingResp.Data {
			if booking.ID == bookingID {
				foundBooking = true
				qt.Assert(t, booking.IsRated, qt.Equals, false, qt.Commentf(
					"IsRated should be false in incoming requests for owner who hasn't rated yet",
				))
				break
			}
		}
		qt.Assert(t, foundBooking, qt.IsTrue, qt.Commentf("Booking should be found in incoming requests"))

		// Owner submits a rating
		_, code = c.Request(http.MethodPost, ownerJWT,
			api.RateRequest{
				Rating:  5,
				Comment: "Testing isRated attribute as owner",
			},
			"bookings", bookingID, "ratings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Check isRated attribute for owner after rating (should be true)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, bookingResp.Data.IsRated, qt.Equals, true, qt.Commentf("IsRated should be true for owner after rating"))

		// Check isRated attribute in incoming requests list (should be true for owner)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests", "incoming")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &incomingResp)
		qt.Assert(t, err, qt.IsNil)

		foundBooking = false
		for _, booking := range incomingResp.Data {
			if booking.ID == bookingID {
				foundBooking = true
				qt.Assert(t, booking.IsRated, qt.Equals, true, qt.Commentf(
					"IsRated should be true in incoming requests for owner after rating",
				))
				break
			}
		}
		qt.Assert(t, foundBooking, qt.IsTrue, qt.Commentf("Booking should be found in incoming requests"))
	})

	t.Run("Date Validation", func(t *testing.T) {
		// Test case 1: Start date before today (should fail)
		yesterday := time.Now().Add(-24 * time.Hour)
		tomorrow := time.Now().Add(24 * time.Hour)

		_, code := c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: yesterday.Unix(),
				EndDate:   tomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking with invalid start date",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 400)

		// Test case 2: End date before start date (should fail)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)
		_, code = c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: dayAfterTomorrow.Unix(),
				EndDate:   tomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking with invalid end date",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 400)

		// Test case 3: Valid dates (should succeed)
		data, code := c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking with valid dates",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var response struct {
			Data api.BookingResponse `json:"data"`
		}
		err := json.Unmarshal(data, &response)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, response.Data.ToolID, qt.Equals, fmt.Sprint(toolID))
	})
}
