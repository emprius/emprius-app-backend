package test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"

	"github.com/emprius/emprius-app-backend/types"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

// TestImageBase64 is a small 1x1 pixel PNG image used for testing
const testImageBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

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

		// Check mail notification is sent to tool owner with all required information
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mailBody, err := c.MailService().FindEmail(ctx, "owner@test.com")
		qt.Assert(t, err, qt.IsNil)

		// Verify mail contains all required information:
		// - Way of contact
		qt.Assert(t, mailBody, qt.Contains, "test@example.com")
		// - Comments
		qt.Assert(t, mailBody, qt.Contains, "Test booking")
		// - From UserName
		qt.Assert(t, mailBody, qt.Contains, "renter")
		// - Tool title
		qt.Assert(t, mailBody, qt.Contains, "Test Tool")
		// - App name (general verification)
		qt.Assert(t, mailBody, qt.Contains, mailtemplates.AppName)

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

		// Check mail notification is sent to tool owner with all required information
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mailBody, err = c.MailService().FindEmail(ctx, "renter@test.com")
		qt.Assert(t, err, qt.IsNil)

		// - To UserName
		qt.Assert(t, mailBody, qt.Contains, "owner")
		// - Tool title
		qt.Assert(t, mailBody, qt.Contains, "Test Tool")
		// - App name (general verification)
		qt.Assert(t, mailBody, qt.Contains, mailtemplates.AppName)

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
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &requestsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(requestsResp.Data.Bookings), qt.Equals, 2)

		// Verify one booking is accepted and one is pending
		var hasAccepted, hasPending bool
		for _, booking := range requestsResp.Data.Bookings {
			switch booking.BookingStatus {
			case api.BookingStatusAccepted:
				hasAccepted = true
			case api.BookingStatusPending:
				hasPending = true
			}
		}
		qt.Assert(t, hasAccepted, qt.IsTrue)
		qt.Assert(t, hasPending, qt.IsTrue)

		// Get booking petitions (renter)
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing")
		qt.Assert(t, code, qt.Equals, 200)
		var petitionsResp struct {
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &petitionsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(petitionsResp.Data.Bookings), qt.Equals, 2)

		// Verify one booking is accepted and one is pending
		hasAccepted = false
		hasPending = false
		for _, booking := range petitionsResp.Data.Bookings {
			switch booking.BookingStatus {
			case "ACCEPTED":
				hasAccepted = true
			case "PENDING":
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
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &ratingsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(ratingsResp.Data.Bookings), qt.Equals, 1)

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
				Data *api.PaginatedUnifiedRatingsResponse `json:"data"`
			}
			err = json.Unmarshal(resp, &submittedResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(submittedResp.Data.Ratings), qt.Equals, 1)
			qt.Assert(t, *submittedResp.Data.Ratings[0].Requester.Rating, qt.Equals, 5)
			qt.Assert(t, *submittedResp.Data.Ratings[0].Requester.RatingComment, qt.Equals, "Great experience!")

			// Get received ratings (owner)
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "users", ownerID, "ratings")
			qt.Assert(t, code, qt.Equals, 200)
			var receivedResp struct {
				Data *api.PaginatedUnifiedRatingsResponse `json:"data"`
			}
			err = json.Unmarshal(resp, &receivedResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(receivedResp.Data.Ratings), qt.Equals, 1)
			qt.Assert(t, *receivedResp.Data.Ratings[0].Requester.Rating, qt.Equals, 5)

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
			qt.Assert(t, len(submittedResp.Data.Ratings), qt.Equals, 1)
			qt.Assert(t, *submittedResp.Data.Ratings[0].Owner.Rating, qt.Equals, 4)

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

		// Test deny petition
		t.Run("Deny Petition", func(t *testing.T) {
			// Create a new booking to deny
			resp, code := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(toolID),
					StartDate: time.Now().Add(72 * time.Hour).Unix(),
					EndDate:   time.Now().Add(96 * time.Hour).Unix(),
					Contact:   "test@example.com",
					Comments:  "Test booking to deny",
				},
				"bookings",
			)
			qt.Assert(t, code, qt.Equals, 200)

			var response struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp, &response)
			qt.Assert(t, err, qt.IsNil)
			denyBookingID := response.Data.ID

			// Try to deny without auth
			_, code = c.Request(http.MethodPut, "",
				&api.BookingStatusUpdate{
					Status: "REJECTED",
				}, "bookings", denyBookingID)
			qt.Assert(t, code, qt.Equals, 401)

			// Try to deny as renter (should fail)
			_, code = c.Request(http.MethodPut, renterJWT,
				&api.BookingStatusUpdate{
					Status: "REJECTED",
				}, "bookings", denyBookingID)
			qt.Assert(t, code, qt.Equals, 403)

			// Deny as owner
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "REJECTED",
				}, "bookings", denyBookingID)
			qt.Assert(t, code, qt.Equals, 200)

			// Verify booking status is REJECTED
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", denyBookingID)
			qt.Assert(t, code, qt.Equals, 200)
			var bookingResp struct {
				Data api.BookingResponse `json:"data"`
			}
			err = json.Unmarshal(resp, &bookingResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "REJECTED")
		})

		// Test cancel request
		t.Run("Cancel Request", func(t *testing.T) {
			// Create a new booking to cancel
			resp, code := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(toolID),
					StartDate: time.Now().Add(120 * time.Hour).Unix(),
					EndDate:   time.Now().Add(144 * time.Hour).Unix(),
					Contact:   "test@example.com",
					Comments:  "Test booking to cancel",
				},
				"bookings",
			)
			qt.Assert(t, code, qt.Equals, 200)

			var response struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp, &response)
			qt.Assert(t, err, qt.IsNil)
			cancelBookingID := response.Data.ID

			// Try to cancel without auth
			_, code = c.Request(http.MethodPut, "",
				&api.BookingStatusUpdate{
					Status: "CANCELLED",
				}, "bookings", cancelBookingID)
			qt.Assert(t, code, qt.Equals, 401)

			// Try to cancel as owner (should fail)
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "CANCELLED",
				}, "bookings", cancelBookingID)
			qt.Assert(t, code, qt.Equals, 403)

			// Cancel as renter
			_, code = c.Request(http.MethodPut, renterJWT,
				&api.BookingStatusUpdate{
					Status: "CANCELLED",
				}, "bookings", cancelBookingID)
			qt.Assert(t, code, qt.Equals, 200)

			// Verify booking status is CANCELLED
			resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", cancelBookingID)
			qt.Assert(t, code, qt.Equals, 200)
			var bookingResp struct {
				Data api.BookingResponse `json:"data"`
			}
			err = json.Unmarshal(resp, &bookingResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "CANCELLED")
		})

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
			qt.Assert(t, len(petitionsResp.Data.Bookings), qt.Equals, 1)

			// Accept the booking request
			bookingID := petitionsResp.Data.Bookings[0].ID
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
				Data api.PaginatedBookingsResponse `json:"data"`
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
				Data *api.PaginatedUnifiedRatingsResponse `json:"data"`
			}
			err = json.Unmarshal(resp, &unifiedResp)
			qt.Assert(t, err, qt.IsNil)

			// Verify we have at least one unified rating
			qt.Assert(t, len(unifiedResp.Data.Ratings) > 0, qt.IsTrue)

			// Find the rating for our test booking
			var testBookingRating *db.UnifiedRating
			for _, rating := range unifiedResp.Data.Ratings {
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
			qt.Assert(t, len(unifiedResp.Data.Ratings) > 0, qt.IsTrue)

			// Find the rating for our test booking
			testBookingRating = nil
			for _, rating := range unifiedResp.Data.Ratings {
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

		// Test nomadic tool feature
		t.Run("IsNomadic Tool", func(t *testing.T) {
			// Create a new user for this test
			ownerJWT := c.RegisterAndLogin("nomadic-owner@test.com", "nomadic-owner", "ownerpass")
			renterJWT, renterID := c.RegisterAndLoginWithID("nomadic-renter@test.com", "nomadic-renter", "renterpass")

			// Create a nomadic tool
			createToolResp, code := c.Request(http.MethodPost, ownerJWT, map[string]interface{}{
				"title":         "IsNomadic Tool",
				"description":   "This tool changes location when rented",
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

			// Get the tool to verify it's nomadic
			getToolResp, code := c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(nomadicToolID))
			qt.Assert(t, code, qt.Equals, 200)
			var toolDetails struct {
				Data api.Tool `json:"data"`
			}
			err = json.Unmarshal(getToolResp, &toolDetails)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, *toolDetails.Data.IsNomadic, qt.IsTrue)

			// Create a booking for the nomadic tool
			resp, code = c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(nomadicToolID),
					StartDate: time.Now().Add(24 * time.Hour).Unix(),
					EndDate:   time.Now().Add(48 * time.Hour).Unix(),
					Contact:   "test@example.com",
					Comments:  "Booking for nomadic tool test",
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

			// Try to mark as picked without auth
			_, code = c.Request(http.MethodPut, "",
				&api.BookingStatusUpdate{
					Status: "PICKED",
				}, "bookings", bookingID)
			qt.Assert(t, code, qt.Equals, 401)

			// Try to mark as picked by renter (should fail)
			_, code = c.Request(http.MethodPut, renterJWT,
				&api.BookingStatusUpdate{
					Status: "PICKED",
				}, "bookings", bookingID)
			qt.Assert(t, code, qt.Equals, 403)

			// Mark as picked by owner
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "PICKED",
				}, "bookings", bookingID)
			qt.Assert(t, code, qt.Equals, 200)

			// Get the booking to verify it's marked as PICKED
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", bookingID)
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &bookingResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "PICKED")

			// Get the tool to verify actualUserId is set to the renter
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "tools", fmt.Sprint(nomadicToolID))
			qt.Assert(t, code, qt.Equals, 200)
			var toolDetailsAfterPick struct {
				Data api.Tool `json:"data"`
			}
			err = json.Unmarshal(resp, &toolDetailsAfterPick)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, toolDetailsAfterPick.Data.ActualUserID, qt.Equals, renterID)

			// Create a non-nomadic tool for comparison
			regularToolResp, code := c.Request(http.MethodPost, ownerJWT, map[string]interface{}{
				"title":         "Regular Tool",
				"description":   "This is a regular non-nomadic tool",
				"toolCategory":  1,
				"toolValuation": 100,
				"isNomadic":     false,
			}, "tools")
			qt.Assert(t, code, qt.Equals, 200)

			var regularToolIDResp struct {
				Data struct {
					ID int64 `json:"id"`
				} `json:"data"`
			}
			err = json.Unmarshal(regularToolResp, &regularToolIDResp)
			qt.Assert(t, err, qt.IsNil)
			regularToolID := regularToolIDResp.Data.ID

			// Create a booking for the regular tool
			resp, code = c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(regularToolID),
					StartDate: time.Now().Add(72 * time.Hour).Unix(),
					EndDate:   time.Now().Add(96 * time.Hour).Unix(),
					Contact:   "test@example.com",
					Comments:  "Booking for regular tool test",
				},
				"bookings",
			)
			qt.Assert(t, code, qt.Equals, 200)

			err = json.Unmarshal(resp, &bookingResp)
			qt.Assert(t, err, qt.IsNil)
			regularBookingID := bookingResp.Data.ID

			// Owner accepts the booking
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", regularBookingID)
			qt.Assert(t, code, qt.Equals, 200)

			// Try to mark regular tool as picked (should fail because it's not nomadic)
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "PICKED",
				}, "bookings", regularBookingID)
			qt.Assert(t, code, qt.Equals, 422) // Unprocessable Entity - tool is not nomadic
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
		qt.Assert(t, *bookingResp.Data.IsRated, qt.Equals, false, qt.Commentf("IsRated should be false before rating"))

		// Check isRated attribute in outgoing requests list (should be false)
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing")
		qt.Assert(t, code, qt.Equals, 200)
		var outgoingResp struct {
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &outgoingResp)
		qt.Assert(t, err, qt.IsNil)

		var foundBooking bool
		for _, booking := range outgoingResp.Data.Bookings {
			if booking.ID == bookingID {
				foundBooking = true
				qt.Assert(t, *booking.IsRated, qt.Equals, false, qt.Commentf("IsRated should be false in outgoing requests before rating"))
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
		qt.Assert(t, *bookingResp.Data.IsRated, qt.Equals, true, qt.Commentf("IsRated should be true after rating"))

		// Check isRated attribute in outgoing requests list (should be true)
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &outgoingResp)
		qt.Assert(t, err, qt.IsNil)

		foundBooking = false
		for _, booking := range outgoingResp.Data.Bookings {
			if booking.ID == bookingID {
				foundBooking = true
				qt.Assert(t, *booking.IsRated, qt.Equals, true, qt.Commentf("IsRated should be true in outgoing requests after rating"))
				break
			}
		}
		qt.Assert(t, foundBooking, qt.IsTrue, qt.Commentf("Booking should be found in outgoing requests"))

		// Check from owner's perspective (should be false since owner hasn't rated yet)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, *bookingResp.Data.IsRated, qt.Equals, false, qt.Commentf("IsRated should be false for owner who hasn't rated yet"))

		// Check isRated attribute in incoming requests list (should be false for owner)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests", "incoming")
		qt.Assert(t, code, qt.Equals, 200)
		var incomingResp struct {
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &incomingResp)
		qt.Assert(t, err, qt.IsNil)

		foundBooking = false
		for _, booking := range incomingResp.Data.Bookings {
			if booking.ID == bookingID {
				foundBooking = true
				qt.Assert(t, *booking.IsRated, qt.Equals, false, qt.Commentf(
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
		qt.Assert(t, *bookingResp.Data.IsRated, qt.Equals, true, qt.Commentf("IsRated should be true for owner after rating"))

		// Check isRated attribute in incoming requests list (should be true for owner)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests", "incoming")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &incomingResp)
		qt.Assert(t, err, qt.IsNil)

		foundBooking = false
		for _, booking := range incomingResp.Data.Bookings {
			if booking.ID == bookingID {
				foundBooking = true
				qt.Assert(t, *booking.IsRated, qt.Equals, true, qt.Commentf(
					"IsRated should be true in incoming requests for owner after rating",
				))
				break
			}
		}
		qt.Assert(t, foundBooking, qt.IsTrue, qt.Commentf("Booking should be found in incoming requests"))
	})

	t.Run("MaxDistance Validation", func(t *testing.T) {
		// Create a new tool owner with a specific location
		ownerWithLocationJWT := c.RegisterAndLogin("owner_location@test.com", "owner_location", "ownerpass")

		// Create a tool
		toolWithMaxDistanceID := c.CreateTool(ownerWithLocationJWT, "Distance Limited Tool")

		// Update the tool to set MaxDistance
		c.Request(http.MethodPut, ownerWithLocationJWT,
			map[string]interface{}{
				"maxDistance": 50, // 50 km max distance
			},
			"tools", fmt.Sprint(toolWithMaxDistanceID),
		)

		// Create a renter with a location within the max distance (Toledo - about 70 km from Madrid)
		nearRenterJWT := c.RegisterAndLogin("near_renter@test.com", "near_renter", "renterpass")
		c.Request(http.MethodPost, nearRenterJWT,
			api.UserProfile{
				Location: &api.Location{
					Latitude:  41385064, // Barcelona latitude in microdegrees
					Longitude: 2173404,  // Barcelona longitude in microdegrees
				},
			},
			"profile",
		)

		// Create a renter with a location beyond the max distance (Barcelona - about 600 km from Madrid)
		farRenterJWT := c.RegisterAndLogin("far_renter@test.com", "far_renter", "renterpass")
		c.Request(http.MethodPost, farRenterJWT,
			api.UserProfile{
				Location: &api.Location{
					Latitude:  39868164, // Toledo latitude in microdegrees
					Longitude: -4027348, // Toledo longitude in microdegrees
				},
			},
			"profile",
		)

		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		// Test case 1: Renter too far away (should fail)
		data, code := c.Request(http.MethodPost, farRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolWithMaxDistanceID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking from too far away",
			},
			"bookings",
		)

		qt.Assert(t, code, qt.Equals, 422) // Unprocessable Entity

		// Verify error message
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err := json.Unmarshal(data, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "tool location is too far away")

		// Test case 2: Renter within max distance (should succeed)
		data, code = c.Request(http.MethodPost, nearRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolWithMaxDistanceID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking within max distance",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var response struct {
			Data api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(data, &response)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, response.Data.ToolID, qt.Equals, fmt.Sprint(toolWithMaxDistanceID))
	})

	t.Run("Rating with Images", func(t *testing.T) {
		// Create a new booking for testing ratings with images
		resp, code := c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: time.Now().Add(300 * time.Hour).Unix(),
				EndDate:   time.Now().Add(324 * time.Hour).Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking for rating with images",
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

		// Upload test images
		// First test image
		imageData1 := testImageBase64
		decodedImage1, err := base64.StdEncoding.DecodeString(imageData1)
		qt.Assert(t, err, qt.IsNil)
		resp, code = c.Request(http.MethodPost, renterJWT, &db.Image{
			Content: decodedImage1,
			Name:    "test1.jpg",
		}, "images")
		qt.Assert(t, code, qt.Equals, 200)

		var imageResp1 struct {
			Data struct {
				Hash string `json:"hash"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &imageResp1)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, imageResp1.Data.Hash, qt.Not(qt.Equals), "")

		// Second test image
		imageData2 := testImageBase64
		decodedImage2, err := base64.StdEncoding.DecodeString(imageData2)
		qt.Assert(t, err, qt.IsNil)
		resp, code = c.Request(http.MethodPost, renterJWT, &db.Image{
			Content: decodedImage2,
			Name:    "test2.jpg",
		}, "images")
		qt.Assert(t, code, qt.Equals, 200)

		var imageResp2 struct {
			Data struct {
				Hash string `json:"hash"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &imageResp2)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, imageResp2.Data.Hash, qt.Not(qt.Equals), "")

		// Submit rating with images
		_, code = c.Request(http.MethodPost, renterJWT,
			api.RateRequest{
				Rating:  5,
				Comment: "Great experience with images!",
				Images:  []types.HexBytes{types.HexStringToHexBytes(imageResp1.Data.Hash), types.HexStringToHexBytes(imageResp2.Data.Hash)},
			},
			"bookings", bookingID, "ratings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get the rating and verify images are included
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", bookingID, "ratings")
		qt.Assert(t, code, qt.Equals, 200)

		var ratingResp struct {
			Data *db.UnifiedRating `json:"data"`
		}
		err = json.Unmarshal(resp, &ratingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, ratingResp.Data.Requester, qt.Not(qt.IsNil))
		qt.Assert(t, ratingResp.Data.Requester.Images, qt.Not(qt.IsNil))
		qt.Assert(t, len(ratingResp.Data.Requester.Images), qt.Equals, 2)
		qt.Assert(t, ratingResp.Data.Requester.Images[0].String(), qt.Equals, imageResp1.Data.Hash)
		qt.Assert(t, ratingResp.Data.Requester.Images[1].String(), qt.Equals, imageResp2.Data.Hash)

		// Test invalid image hash
		_, code = c.Request(http.MethodPost, ownerJWT,
			api.RateRequest{
				Rating:  4,
				Comment: "Rating with invalid image hash",
				Images:  []types.HexBytes{types.HexBytes("invalidhash123")},
			},
			"bookings", bookingID, "ratings",
		)
		qt.Assert(t, code, qt.Equals, 404) // Should fail with bad request
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

	t.Run("Community Membership Check", func(t *testing.T) {
		// Create users for testing
		ownerJWT, _ := c.RegisterAndLoginWithID("community-owner@test.com", "community-owner", "ownerpass")
		memberJWT, memberID := c.RegisterAndLoginWithID("community-member@test.com", "community-member", "memberpass")
		nonMemberJWT, _ := c.RegisterAndLoginWithID("community-nonmember@test.com", "community-nonmember", "nonmemberpass")

		// Create a community
		resp, code := c.Request(http.MethodPost, ownerJWT,
			api.CreateCommunityRequest{
				Name: "Booking Test Community",
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

		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		// Test case 1: Non-member trying to book a community tool (should fail)
		data, code := c.Request(http.MethodPost, nonMemberJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking from non-member",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 403) // Forbidden

		// Verify error message
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err = json.Unmarshal(data, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "user is not a member of the community this tool belongs to")

		// Test case 2: Member booking a community tool (should succeed)
		data, code = c.Request(http.MethodPost, memberJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking from member",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var bookingResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(data, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, bookingResp.Data.ToolID, qt.Equals, fmt.Sprint(toolID))

		// Test case 3: Non-member booking a non-community tool (should succeed)
		data, code = c.Request(http.MethodPost, nonMemberJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(nonCommunityToolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking for non-community tool",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(data, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, bookingResp.Data.ToolID, qt.Equals, fmt.Sprint(nonCommunityToolID))
	})

	t.Run("Nomadic Tool with Future Booking", func(t *testing.T) {
		// Create a new user for this test
		ownerJWT := c.RegisterAndLogin("nomadic-future-owner@test.com", "nomadic-future-owner", "ownerpass")
		renter1JWT := c.RegisterAndLogin("nomadic-future-renter1@test.com", "nomadic-future-renter1", "renterpass")
		renter2JWT := c.RegisterAndLogin("nomadic-future-renter2@test.com", "nomadic-future-renter2", "renterpass")

		// Create a nomadic tool
		createToolResp, code := c.Request(http.MethodPost, ownerJWT, map[string]interface{}{
			"title":         "Nomadic Tool with Future Booking",
			"description":   "This tool has a future booking",
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

		// Create a booking with future dates
		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		resp, code := c.Request(http.MethodPost, renter1JWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(nomadicToolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "First booking for nomadic tool test",
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

		// Now try to create a new booking for the same nomadic tool with overlapping dates
		// This should fail because the tool already has an accepted booking
		resp, code = c.Request(http.MethodPost, renter2JWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(nomadicToolID),
				StartDate: tomorrow.Add(12 * time.Hour).Unix(),
				EndDate:   dayAfterTomorrow.Add(12 * time.Hour).Unix(),
				Contact:   "test2@example.com",
				Comments:  "Second booking for nomadic tool test (should fail)",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 400, qt.Commentf("Response: %s", string(resp)))
	})
}

func TestBookingPagination(t *testing.T) {
	c := utils.NewTestService(t)

	// Create two users: tool owner and renter
	ownerJWT := c.RegisterAndLogin("pagination-owner@test.com", "pagination-owner", "ownerpass")
	renterJWT := c.RegisterAndLogin("pagination-renter@test.com", "pagination-renter", "renterpass")

	// Owner creates a tool
	toolID := c.CreateTool(ownerJWT, "Pagination Test Tool")

	// Create multiple bookings for testing pagination
	var bookingIDs []string
	for i := 0; i < 25; i++ {
		resp, code := c.Request(http.MethodPost, renterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: time.Now().Add(time.Duration(24*i) * time.Hour).Unix(),
				EndDate:   time.Now().Add(time.Duration((24*i)+2) * time.Hour).Unix(),
				Contact:   "test@example.com",
				Comments:  fmt.Sprintf("Test booking %d", i+1),
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var response struct {
			Data api.BookingResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &response)
		qt.Assert(t, err, qt.IsNil)
		bookingIDs = append(bookingIDs, response.Data.ID)
	}

	// Test outgoing requests pagination (renter's perspective)
	t.Run("Outgoing IncomingBookings Pagination", func(t *testing.T) {
		// Test first page with default page size
		resp, code := c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing")
		qt.Assert(t, code, qt.Equals, 200)

		var paginatedResp struct {
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		// Should return default page size (16) bookings
		qt.Assert(t, len(paginatedResp.Data.Bookings), qt.Equals, 16)
		qt.Assert(t, paginatedResp.Data.Pagination.Total, qt.Equals, int64(25))
		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 0)
		qt.Assert(t, paginatedResp.Data.Pagination.PageSize, qt.Equals, 16)
		qt.Assert(t, paginatedResp.Data.Pagination.Pages, qt.Equals, 2) // ceil(25/16) = 2

		// Test second page
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing?page=1")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		// Should return remaining 9 bookings
		qt.Assert(t, len(paginatedResp.Data.Bookings), qt.Equals, 9)
		qt.Assert(t, paginatedResp.Data.Pagination.Total, qt.Equals, int64(25))
		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 1)

		// Test custom page size
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing?page=0&pageSize=10")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, len(paginatedResp.Data.Bookings), qt.Equals, 10)
		qt.Assert(t, paginatedResp.Data.Pagination.PageSize, qt.Equals, 10)
		qt.Assert(t, paginatedResp.Data.Pagination.Pages, qt.Equals, 3) // ceil(25/10) = 3
	})

	// Test incoming requests pagination (owner's perspective)
	t.Run("Incoming IncomingBookings Pagination", func(t *testing.T) {
		// Test first page
		resp, code := c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests", "incoming?page=0&pageSize=5")
		qt.Assert(t, code, qt.Equals, 200)

		var paginatedResp struct {
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, len(paginatedResp.Data.Bookings), qt.Equals, 5)
		qt.Assert(t, paginatedResp.Data.Pagination.Total, qt.Equals, int64(25))
		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 0)
		qt.Assert(t, paginatedResp.Data.Pagination.PageSize, qt.Equals, 5)
		qt.Assert(t, paginatedResp.Data.Pagination.Pages, qt.Equals, 5) // ceil(25/5) = 5

		// Test last page
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests", "incoming?page=4&pageSize=5")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, len(paginatedResp.Data.Bookings), qt.Equals, 5)
		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 4)
	})

	// Test pending ratings pagination
	t.Run("Pending Ratings Pagination", func(t *testing.T) {
		// First, accept and return some bookings to make them eligible for rating
		for i := 0; i < 10; i++ {
			// Accept booking
			_, code := c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", bookingIDs[i])
			qt.Assert(t, code, qt.Equals, 200)

			// Mark as returned
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "RETURNED",
				}, "bookings", bookingIDs[i])
			qt.Assert(t, code, qt.Equals, 200)
		}

		// Test pending ratings pagination
		resp, code := c.Request(http.MethodGet, renterJWT, nil, "bookings", "ratings", "pending?page=0&pageSize=3")
		qt.Assert(t, code, qt.Equals, 200)

		var paginatedResp struct {
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, len(paginatedResp.Data.Bookings), qt.Equals, 3)
		qt.Assert(t, paginatedResp.Data.Pagination.Total, qt.Equals, int64(10))
		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 0)
		qt.Assert(t, paginatedResp.Data.Pagination.PageSize, qt.Equals, 3)
		qt.Assert(t, paginatedResp.Data.Pagination.Pages, qt.Equals, 4) // ceil(10/3) = 4

		// Test second page
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "ratings", "pending?page=1&pageSize=3")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, len(paginatedResp.Data.Bookings), qt.Equals, 3)
		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 1)
	})

	// Test edge cases
	t.Run("Edge Cases", func(t *testing.T) {
		// Test negative page number (should default to 0)
		resp, code := c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing?page=-1&pageSize=5")
		qt.Assert(t, code, qt.Equals, 200)

		var paginatedResp struct {
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, paginatedResp.Data.Pagination.Current, qt.Equals, 0) // Should default to 0

		// Test zero page size (should use default)
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing?page=0&pageSize=0")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, paginatedResp.Data.Pagination.PageSize, qt.Equals, 16) // Should use default page size

		// Test page beyond available data
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing?page=100&pageSize=5")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, len(paginatedResp.Data.Bookings), qt.Equals, 0)            // Should return empty array
		qt.Assert(t, paginatedResp.Data.Pagination.Total, qt.Equals, int64(25)) // Total should still be correct
	})

	// Test sorting (PENDING bookings should come first)
	t.Run("Sorting", func(t *testing.T) {
		// Accept some bookings to create a mix of statuses
		for i := 10; i < 15; i++ {
			_, code := c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", bookingIDs[i])
			qt.Assert(t, code, qt.Equals, 200)
		}

		// Get first page of incoming requests
		resp, code := c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests", "incoming?page=0&pageSize=20")
		qt.Assert(t, code, qt.Equals, 200)

		var paginatedResp struct {
			Data api.PaginatedBookingsResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &paginatedResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify that PENDING bookings come first
		pendingCount := 0
		acceptedCount := 0
		returnedCount := 0
		foundNonPending := false

		for _, booking := range paginatedResp.Data.Bookings {
			switch booking.BookingStatus {
			case "PENDING":
				// Once we've seen a non-pending booking, we shouldn't see any more pending ones
				qt.Assert(t, foundNonPending, qt.Equals, false, qt.Commentf("PENDING bookings should come first"))
				pendingCount++
			case "ACCEPTED":
				foundNonPending = true
				acceptedCount++
			case "RETURNED":
				foundNonPending = true
				returnedCount++
			}
		}

		// We should have some pending bookings (the ones not yet accepted)
		qt.Assert(t, pendingCount > 0, qt.IsTrue, qt.Commentf("Should have some pending bookings"))
		qt.Assert(t, acceptedCount > 0, qt.IsTrue, qt.Commentf("Should have some accepted bookings"))
		qt.Assert(t, returnedCount > 0, qt.IsTrue, qt.Commentf("Should have some returned bookings"))
	})
}

func TestBookingInactiveUserValidation(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users for testing
	activeOwnerJWT, activeOwnerID := c.RegisterAndLoginWithID("active-owner@test.com", "Active Owner", "password")
	activeRenterJWT, activeRenterID := c.RegisterAndLoginWithID("active-renter@test.com", "Active Renter", "password")
	inactiveOwnerJWT, _ := c.RegisterAndLoginWithID("inactive-owner@test.com", "Inactive Owner", "password")
	inactiveRenterJWT, _ := c.RegisterAndLoginWithID("inactive-renter@test.com", "Inactive Renter", "password")

	// Create tools from both active and inactive owners
	activeOwnerToolID := c.CreateTool(activeOwnerJWT, "Active Owner Tool")
	inactiveOwnerToolID := c.CreateTool(inactiveOwnerJWT, "Inactive Owner Tool")

	// Deactivate the inactive users
	_, code := c.Request(http.MethodPost, inactiveOwnerJWT,
		api.UserProfile{
			Active: &[]bool{false}[0], // Set active to false
		},
		"profile",
	)
	qt.Assert(t, code, qt.Equals, 200)

	_, code = c.Request(http.MethodPost, inactiveRenterJWT,
		api.UserProfile{
			Active: &[]bool{false}[0], // Set active to false
		},
		"profile",
	)
	qt.Assert(t, code, qt.Equals, 200)

	// Test booking dates
	tomorrow := time.Now().Add(24 * time.Hour)
	dayAfterTomorrow := time.Now().Add(48 * time.Hour)

	t.Run("Active user booking from active owner (should succeed)", func(t *testing.T) {
		resp, code := c.Request(http.MethodPost, activeRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(activeOwnerToolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking from active user to active owner",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var response struct {
			Data api.BookingResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &response)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, response.Data.ToolID, qt.Equals, fmt.Sprint(activeOwnerToolID))
		qt.Assert(t, response.Data.FromUserID, qt.Equals, activeRenterID)
		qt.Assert(t, response.Data.ToUserID, qt.Equals, activeOwnerID)
	})

	t.Run("Inactive user trying to create booking (should fail)", func(t *testing.T) {
		resp, code := c.Request(http.MethodPost, inactiveRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(activeOwnerToolID),
				StartDate: tomorrow.Add(72 * time.Hour).Unix(),
				EndDate:   dayAfterTomorrow.Add(72 * time.Hour).Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking from inactive user",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 403) // Forbidden

		// Verify error message
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err := json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "user account is inactive")
	})

	t.Run("Active user trying to book from inactive owner (should fail)", func(t *testing.T) {
		resp, code := c.Request(http.MethodPost, activeRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(inactiveOwnerToolID),
				StartDate: tomorrow.Add(96 * time.Hour).Unix(),
				EndDate:   dayAfterTomorrow.Add(96 * time.Hour).Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking from active user to inactive owner",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 403) // Forbidden

		// Verify error message
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err := json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "recipient user account is inactive")
	})

	t.Run("Inactive user trying to book from inactive owner (should fail with requester error)", func(t *testing.T) {
		resp, code := c.Request(http.MethodPost, inactiveRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(inactiveOwnerToolID),
				StartDate: tomorrow.Add(120 * time.Hour).Unix(),
				EndDate:   dayAfterTomorrow.Add(120 * time.Hour).Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking from inactive user to inactive owner",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 403) // Forbidden

		// Should fail with requester inactive error (checked first)
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err := json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "user account is inactive")
	})
}

func TestBookingInactiveUserValidationWithNomadicTools(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users for testing
	activeOwnerJWT, _ := c.RegisterAndLoginWithID("nomadic-active-owner@test.com", "Active Owner", "password")
	activeRenterJWT, activeRenterID := c.RegisterAndLoginWithID("nomadic-active-renter@test.com", "Active Renter", "password")
	inactiveActualUserJWT, inactiveActualUserID := c.RegisterAndLoginWithID(
		"nomadic-inactive-actual@test.com",
		"Inactive Actual User",
		"password",
	)

	// Create a nomadic tool from active owner
	createToolResp, code := c.Request(http.MethodPost, activeOwnerJWT, map[string]interface{}{
		"title":         "Nomadic Tool",
		"description":   "This is a nomadic tool for testing",
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

	// Test booking dates
	today := time.Now().Unix()
	oneDay := time.Now().Add(24 * time.Hour).Unix()
	twoDays := time.Now().Add(48 * time.Hour).Unix()
	threeDays := time.Now().Add(72 * time.Hour).Unix()
	fourDays := time.Now().Add(96 * time.Hour).Unix()
	fiveDays := time.Now().Add(120 * time.Hour).Unix()
	sixDays := time.Now().Add(148 * time.Hour).Unix()

	// Set the inactive user as the actual user of the nomadic tool
	resp, code := c.Request(http.MethodPost, inactiveActualUserJWT,
		api.CreateBookingRequest{
			ToolID:    fmt.Sprint(nomadicToolID),
			StartDate: today,
			EndDate:   today,
			Contact:   "test@example.com",
			Comments:  "Booking for nomadic tool test",
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
	_, code = c.Request(http.MethodPut, activeOwnerJWT,
		&api.BookingStatusUpdate{
			Status: "ACCEPTED",
		}, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	// Mark as picked by owner
	_, code = c.Request(http.MethodPut, activeOwnerJWT,
		&api.BookingStatusUpdate{
			Status: "PICKED",
		}, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	// Deactivate the actual user
	_, code = c.Request(http.MethodPost, inactiveActualUserJWT,
		api.UserProfile{
			Active: &[]bool{false}[0], // Set active to false
		},
		"profile",
	)
	qt.Assert(t, code, qt.Equals, 200)

	t.Run("Active user trying to book nomadic tool with inactive actual user (should fail)", func(t *testing.T) {
		resp, code := c.Request(http.MethodPost, activeRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(nomadicToolID),
				StartDate: oneDay,
				EndDate:   twoDays,
				Contact:   "test@example.com",
				Comments:  "Test booking nomadic tool with inactive actual user",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 403) // Forbidden

		// Verify error message
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err := json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "recipient user account is inactive")
	})

	t.Run("Reactivate actual user and booking should succeed", func(t *testing.T) {
		// Reactivate the actual user
		_, code := c.Request(http.MethodPost, inactiveActualUserJWT,
			api.UserProfile{
				Active: &[]bool{true}[0], // Set active to true
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Now the booking should succeed
		resp, code := c.Request(http.MethodPost, activeRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(nomadicToolID),
				StartDate: threeDays,
				EndDate:   fourDays,
				Contact:   "test@example.com",
				Comments:  "Test booking nomadic tool with reactivated actual user",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var response struct {
			Data api.BookingResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &response)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, response.Data.ToolID, qt.Equals, fmt.Sprint(nomadicToolID))
		qt.Assert(t, response.Data.FromUserID, qt.Equals, activeRenterID)
		// For nomadic tools with actual user, the booking goes to the actual user
		qt.Assert(t, response.Data.ToUserID, qt.Equals, inactiveActualUserID)
	})

	t.Run("Nomadic tool without actual user - inactive owner should fail", func(t *testing.T) {
		// Create another nomadic tool without actual user
		createToolResp, code := c.Request(http.MethodPost, activeOwnerJWT, map[string]interface{}{
			"title":         "Nomadic Tool No Actual User",
			"description":   "This is a nomadic tool without actual user",
			"toolCategory":  1,
			"toolValuation": 100,
			"isNomadic":     true,
		}, "tools")
		qt.Assert(t, code, qt.Equals, 200)

		err := json.Unmarshal(createToolResp, &toolIDResp)
		qt.Assert(t, err, qt.IsNil)
		nomadicToolID2 := toolIDResp.Data.ID

		// Deactivate the owner
		_, code = c.Request(http.MethodPost, activeOwnerJWT,
			api.UserProfile{
				Active: &[]bool{false}[0], // Set active to false
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Try to book the nomadic tool - should fail because owner is inactive
		resp, code := c.Request(http.MethodPost, activeRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(nomadicToolID2),
				StartDate: fiveDays,
				EndDate:   sixDays,
				Contact:   "test@example.com",
				Comments:  "Test booking nomadic tool with inactive owner",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 403) // Forbidden

		// Verify error message
		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err = json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "recipient user account is inactive")
	})
}

func TestBookingInactiveUserValidationEdgeCases(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users
	activeOwnerJWT, _ := c.RegisterAndLoginWithID("edge-active-owner@test.com", "Active Owner", "password")
	activeRenterJWT, _ := c.RegisterAndLoginWithID("edge-active-renter@test.com", "Active Renter", "password")

	// Create tool
	toolID := c.CreateTool(activeOwnerJWT, "Edge Case Tool")

	// Test booking dates
	tomorrow := time.Now().Add(24 * time.Hour)
	dayAfterTomorrow := time.Now().Add(48 * time.Hour)

	t.Run("User deactivated after booking creation but before acceptance", func(t *testing.T) {
		// Create booking while both users are active
		resp, code := c.Request(http.MethodPost, activeRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking before deactivation",
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

		// Deactivate the renter after booking creation
		_, code = c.Request(http.MethodPost, activeRenterJWT,
			api.UserProfile{
				Active: &[]bool{false}[0], // Set active to false
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Owner should still be able to accept the booking
		// (The validation only applies to booking creation, not status updates)
		_, code = c.Request(http.MethodPut, activeOwnerJWT,
			&api.BookingStatusUpdate{
				Status: "ACCEPTED",
			}, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		// Verify booking was accepted
		resp, code = c.Request(http.MethodGet, activeOwnerJWT, nil, "bookings", bookingID)
		qt.Assert(t, code, qt.Equals, 200)

		var bookingResp struct {
			Data api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &bookingResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "ACCEPTED")
	})

	t.Run("Validation order - requester checked before recipient", func(t *testing.T) {
		// Create another user to be the inactive owner
		inactiveOwnerJWT, _ := c.RegisterAndLoginWithID("edge-inactive-owner@test.com", "Inactive Owner", "password")
		inactiveRenterJWT, _ := c.RegisterAndLoginWithID("edge-inactive-renter@test.com", "Inactive Renter", "password")

		// Create tool from inactive owner
		inactiveToolID := c.CreateTool(inactiveOwnerJWT, "Inactive Owner Tool")

		// Deactivate both users
		_, code := c.Request(http.MethodPost, inactiveOwnerJWT,
			api.UserProfile{
				Active: &[]bool{false}[0],
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		_, code = c.Request(http.MethodPost, inactiveRenterJWT,
			api.UserProfile{
				Active: &[]bool{false}[0],
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Try to create booking - should fail with requester inactive error first
		resp, code := c.Request(http.MethodPost, inactiveRenterJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(inactiveToolID),
				StartDate: tomorrow.Add(168 * time.Hour).Unix(),
				EndDate:   dayAfterTomorrow.Add(168 * time.Hour).Unix(),
				Contact:   "test@example.com",
				Comments:  "Test validation order",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 403)

		var errorResp struct {
			Header api.ResponseHeader `json:"header"`
		}
		err := json.Unmarshal(resp, &errorResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, errorResp.Header.Success, qt.Equals, false)
		// Should get the requester inactive error, not the recipient inactive error
		qt.Assert(t, errorResp.Header.Message, qt.Contains, "user account is inactive")
		qt.Assert(t, errorResp.Header.Message, qt.Not(qt.Contains), "recipient")
	})
}

func TestBookingInactiveUserValidationUnit(t *testing.T) {
	c := utils.NewTestService(t)

	// Test that the validation logic correctly identifies inactive users
	t.Run("Unit test for inactive user validation", func(t *testing.T) {
		tomorrow := time.Now().Add(24 * time.Hour)
		dayAfterTomorrow := time.Now().Add(48 * time.Hour)

		// Create an active user
		activeUserJWT, activeUserID := c.RegisterAndLoginWithID("active@test.com", "Active User", "password")

		// Create an inactive user
		inactiveUserJWT, _ := c.RegisterAndLoginWithID("inactive@test.com", "Inactive User", "password")

		// Deactivate the user
		_, code := c.Request("POST", inactiveUserJWT,
			api.UserProfile{
				Active: &[]bool{false}[0],
			},
			"profile",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Create a tool from the active user
		toolID := c.CreateTool(activeUserJWT, "Test Tool")

		// Try to create a booking from the inactive user - should fail
		_, code = c.Request("POST", inactiveUserJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking from inactive user",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 403, qt.Commentf("Inactive user should not be able to create bookings"))

		// Verify that active user can still create bookings
		_, code = c.Request("POST", activeUserJWT,
			api.CreateBookingRequest{
				ToolID:    fmt.Sprint(toolID),
				StartDate: tomorrow.Unix(),
				EndDate:   dayAfterTomorrow.Unix(),
				Contact:   "test@example.com",
				Comments:  "Test booking from active user",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Active user should be able to create bookings"))

		t.Logf("✓ Inactive user validation working correctly")
		t.Logf("✓ Active user: %s can create bookings", activeUserID)
		t.Logf("✓ Inactive user cannot create bookings")
	})
}

func TestBookingDateConflicts(t *testing.T) {
	c := utils.NewTestService(t)

	// Create users for testing
	ownerJWT := c.RegisterAndLogin("conflicts-owner@test.com", "conflicts-owner", "ownerpass")
	renterJWT := c.RegisterAndLogin("conflicts-renter@test.com", "conflicts-renter", "renterpass")

	// Owner creates a tool
	toolID := c.CreateTool(ownerJWT, "Date Conflicts Test Tool")

	// Define base time periods for testing
	baseTime := time.Now().Add(24 * time.Hour) // Start tomorrow to avoid past date issues

	// Period 1: Day 1-3 (will be used as the reference booking)
	period1Start := baseTime
	period1End := baseTime.Add(48 * time.Hour) // 2 days duration

	// Period 2: Day 2-4 (overlaps with period 1)
	period2Start := baseTime.Add(24 * time.Hour)
	period2End := baseTime.Add(72 * time.Hour)

	// Period 3: Day 0-2 (overlaps with period 1, starts before)
	period3Start := baseTime.Add(-24 * time.Hour)
	period3End := baseTime.Add(24 * time.Hour)

	// Period 4: Day 1.5-2.5 (completely inside period 1)
	period4Start := baseTime.Add(12 * time.Hour)
	period4End := baseTime.Add(36 * time.Hour)

	// Period 5: Day 0-5 (completely contains period 1)
	period5Start := baseTime.Add(-24 * time.Hour)
	period5End := baseTime.Add(96 * time.Hour)

	// Period 6: Day 1-3 (exact same dates as period 1)
	period6Start := period1Start
	period6End := period1End

	t.Run("Basic Conflict Behavior", func(t *testing.T) {
		t.Run("PENDING booking overlap should succeed", func(t *testing.T) {
			// Create first booking (will be PENDING)
			resp1, code1 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(toolID),
					StartDate: period1Start.Unix(),
					EndDate:   period1End.Unix(),
					Contact:   "test@example.com",
					Comments:  "First booking (PENDING)",
				},
				"bookings",
			)
			qt.Assert(t, code1, qt.Equals, 200, qt.Commentf("First booking should succeed"))

			var booking1Response struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp1, &booking1Response)
			qt.Assert(t, err, qt.IsNil)
			booking1ID := booking1Response.Data.ID

			// Create overlapping booking while first is still PENDING (should succeed)
			_, code2 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(toolID),
					StartDate: period2Start.Unix(),
					EndDate:   period2End.Unix(),
					Contact:   "test@example.com",
					Comments:  "Second booking (overlapping with PENDING)",
				},
				"bookings",
			)
			qt.Assert(t, code2, qt.Equals, 200, qt.Commentf("Overlapping booking should succeed when first is PENDING"))

			// Clean up - cancel the first booking for next tests
			_, code := c.Request(http.MethodPut, renterJWT,
				&api.BookingStatusUpdate{
					Status: "CANCELLED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)
		})

		t.Run("ACCEPTED booking overlap should fail", func(t *testing.T) {
			// Create first booking
			resp1, code1 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(toolID),
					StartDate: period1Start.Unix(),
					EndDate:   period1End.Unix(),
					Contact:   "test@example.com",
					Comments:  "First booking (to be ACCEPTED)",
				},
				"bookings",
			)
			qt.Assert(t, code1, qt.Equals, 200)

			var booking1Response struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp1, &booking1Response)
			qt.Assert(t, err, qt.IsNil)
			booking1ID := booking1Response.Data.ID

			// Accept the first booking
			_, code := c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)

			// Try to create overlapping booking (should fail)
			_, code2 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(toolID),
					StartDate: period2Start.Unix(),
					EndDate:   period2End.Unix(),
					Contact:   "test@example.com",
					Comments:  "Second booking (should fail due to conflict)",
				},
				"bookings",
			)
			qt.Assert(t, code2, qt.Equals, 400, qt.Commentf("Overlapping booking should fail when first is ACCEPTED"))

			// Clean up - mark as returned for next tests
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "RETURNED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)
		})

		t.Run("Accept booking with conflict should fail", func(t *testing.T) {
			// Create a separate tool for this test to avoid conflicts with previous tests
			conflictToolID := c.CreateTool(ownerJWT, "Accept Conflict Test Tool")

			// Create and accept first booking
			resp1, code1 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(conflictToolID),
					StartDate: period1Start.Unix(),
					EndDate:   period1End.Unix(),
					Contact:   "test@example.com",
					Comments:  "First booking (to be ACCEPTED)",
				},
				"bookings",
			)
			qt.Assert(t, code1, qt.Equals, 200)

			var booking1Response struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp1, &booking1Response)
			qt.Assert(t, err, qt.IsNil)
			booking1ID := booking1Response.Data.ID

			_, code := c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)

			// Create second booking (will be PENDING) - this should fail since conflicts are checked on creation against ACCEPTED bookings
			_, code2 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(conflictToolID),
					StartDate: period2Start.Unix(),
					EndDate:   period2End.Unix(),
					Contact:   "test@example.com",
					Comments:  "Second booking (PENDING, overlapping)",
				},
				"bookings",
			)
			qt.Assert(t, code2, qt.Equals, 400, qt.Commentf("Creating overlapping booking should fail when there's an ACCEPTED booking"))

			// Clean up
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "RETURNED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)
		})
	})

	t.Run("Date Boundary Edge Cases", func(t *testing.T) {
		t.Run("Exact same dates should conflict", func(t *testing.T) {
			// Create a separate tool for this test
			exactDatesToolID := c.CreateTool(ownerJWT, "Exact Dates Test Tool")

			// Create and accept first booking
			resp1, code1 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(exactDatesToolID),
					StartDate: period6Start.Unix(),
					EndDate:   period6End.Unix(),
					Contact:   "test@example.com",
					Comments:  "First booking (exact dates)",
				},
				"bookings",
			)
			qt.Assert(t, code1, qt.Equals, 200)

			var booking1Response struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp1, &booking1Response)
			qt.Assert(t, err, qt.IsNil)
			booking1ID := booking1Response.Data.ID

			_, code := c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)

			// Try to create booking with exact same dates (should fail)
			_, code2 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(exactDatesToolID),
					StartDate: period6Start.Unix(),
					EndDate:   period6End.Unix(),
					Contact:   "test@example.com",
					Comments:  "Second booking (exact same dates)",
				},
				"bookings",
			)
			qt.Assert(t, code2, qt.Equals, 400, qt.Commentf("Booking with exact same dates should fail"))

			// Clean up
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "RETURNED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)
		})

		t.Run("Partial overlap - start before, end during", func(t *testing.T) {
			// Create a separate tool for this test
			partialOverlap1ToolID := c.CreateTool(ownerJWT, "Partial Overlap 1 Test Tool")

			// Create and accept first booking
			resp1, code1 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(partialOverlap1ToolID),
					StartDate: period1Start.Unix(),
					EndDate:   period1End.Unix(),
					Contact:   "test@example.com",
					Comments:  "First booking (reference)",
				},
				"bookings",
			)
			qt.Assert(t, code1, qt.Equals, 200)

			var booking1Response struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp1, &booking1Response)
			qt.Assert(t, err, qt.IsNil)
			booking1ID := booking1Response.Data.ID

			_, code := c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)

			// Try to create booking that starts before and ends during (should fail)
			_, code2 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(partialOverlap1ToolID),
					StartDate: period3Start.Unix(),
					EndDate:   period3End.Unix(),
					Contact:   "test@example.com",
					Comments:  "Overlapping booking (start before, end during)",
				},
				"bookings",
			)
			qt.Assert(t, code2, qt.Equals, 400, qt.Commentf("Partial overlap (start before, end during) should fail"))

			// Clean up
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "RETURNED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)
		})

		t.Run("Partial overlap - start during, end after", func(t *testing.T) {
			// Create a separate tool for this test
			partialOverlap2ToolID := c.CreateTool(ownerJWT, "Partial Overlap 2 Test Tool")

			// Create and accept first booking
			resp1, code1 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(partialOverlap2ToolID),
					StartDate: period1Start.Unix(),
					EndDate:   period1End.Unix(),
					Contact:   "test@example.com",
					Comments:  "First booking (reference)",
				},
				"bookings",
			)
			qt.Assert(t, code1, qt.Equals, 200)

			var booking1Response struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp1, &booking1Response)
			qt.Assert(t, err, qt.IsNil)
			booking1ID := booking1Response.Data.ID

			_, code := c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)

			// Try to create booking that starts during and ends after (should fail)
			_, code2 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(partialOverlap2ToolID),
					StartDate: period2Start.Unix(),
					EndDate:   period2End.Unix(),
					Contact:   "test@example.com",
					Comments:  "Overlapping booking (start during, end after)",
				},
				"bookings",
			)
			qt.Assert(t, code2, qt.Equals, 400, qt.Commentf("Partial overlap (start during, end after) should fail"))

			// Clean up
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "RETURNED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)
		})

		t.Run("Complete containment - new booking inside existing", func(t *testing.T) {
			// Create a separate tool for this test
			containmentToolID := c.CreateTool(ownerJWT, "Containment Test Tool")

			// Create and accept first booking
			resp1, code1 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(containmentToolID),
					StartDate: period1Start.Unix(),
					EndDate:   period1End.Unix(),
					Contact:   "test@example.com",
					Comments:  "First booking (container)",
				},
				"bookings",
			)
			qt.Assert(t, code1, qt.Equals, 200)

			var booking1Response struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp1, &booking1Response)
			qt.Assert(t, err, qt.IsNil)
			booking1ID := booking1Response.Data.ID

			_, code := c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)

			// Try to create booking completely inside existing one (should fail)
			_, code2 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(containmentToolID),
					StartDate: period4Start.Unix(),
					EndDate:   period4End.Unix(),
					Contact:   "test@example.com",
					Comments:  "Contained booking (inside existing)",
				},
				"bookings",
			)
			qt.Assert(t, code2, qt.Equals, 400, qt.Commentf("Booking completely inside existing should fail"))

			// Clean up
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "RETURNED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)
		})

		t.Run("Complete container - new booking contains existing", func(t *testing.T) {
			// Create a separate tool for this test
			containerToolID := c.CreateTool(ownerJWT, "Container Test Tool")

			// Create and accept first booking
			resp1, code1 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(containerToolID),
					StartDate: period1Start.Unix(),
					EndDate:   period1End.Unix(),
					Contact:   "test@example.com",
					Comments:  "First booking (to be contained)",
				},
				"bookings",
			)
			qt.Assert(t, code1, qt.Equals, 200)

			var booking1Response struct {
				Data api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp1, &booking1Response)
			qt.Assert(t, err, qt.IsNil)
			booking1ID := booking1Response.Data.ID

			_, code := c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "ACCEPTED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)

			// Try to create booking that completely contains existing one (should fail)
			_, code2 := c.Request(http.MethodPost, renterJWT,
				api.CreateBookingRequest{
					ToolID:    fmt.Sprint(containerToolID),
					StartDate: period5Start.Unix(),
					EndDate:   period5End.Unix(),
					Contact:   "test@example.com",
					Comments:  "Container booking (contains existing)",
				},
				"bookings",
			)
			qt.Assert(t, code2, qt.Equals, 400, qt.Commentf("Booking that contains existing should fail"))

			// Clean up
			_, code = c.Request(http.MethodPut, ownerJWT,
				&api.BookingStatusUpdate{
					Status: "RETURNED",
				}, "bookings", booking1ID)
			qt.Assert(t, code, qt.Equals, 200)
		})
	})
}
