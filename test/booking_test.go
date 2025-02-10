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
	ownerJWT := c.RegisterAndLogin("owner@test.com", "owner", "ownerpass")
	renterJWT, renterID := c.RegisterAndLoginWithID("renter@test.com", "renter", "renterpass")

	// Owner creates a tool
	toolID := c.CreateTool(ownerJWT, "Test Tool")

	t.Run("Create Booking", func(t *testing.T) {
		// Try to create booking without auth
		_, code := c.Request(http.MethodPost, "",
			map[string]interface{}{
				"toolId":    fmt.Sprint(toolID),
				"startDate": time.Now().Add(24 * time.Hour).Unix(),
				"endDate":   time.Now().Add(48 * time.Hour).Unix(),
				"contact":   "test@example.com",
				"comments":  "Test booking",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 401)

		// Create booking with auth
		resp, code := c.Request(http.MethodPost, renterJWT,
			map[string]interface{}{
				"toolId":    fmt.Sprint(toolID),
				"startDate": time.Now().Add(24 * time.Hour).Unix(),
				"endDate":   time.Now().Add(48 * time.Hour).Unix(),
				"contact":   "test@example.com",
				"comments":  "Test booking",
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
			map[string]interface{}{
				"toolId":    fmt.Sprint(toolID),
				"startDate": time.Now().Add(36 * time.Hour).Unix(),
				"endDate":   time.Now().Add(60 * time.Hour).Unix(),
				"contact":   "test2@example.com",
				"comments":  "Test booking 2",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Accept first booking
		_, code = c.Request(http.MethodPost, ownerJWT, nil, "bookings", "petitions", bookingID, "accept")
		qt.Assert(t, code, qt.Equals, 200)

		// Try to create another overlapping booking (should fail since there's an accepted booking)
		data, code := c.Request(http.MethodPost, renterJWT,
			map[string]interface{}{
				"toolId":    fmt.Sprint(toolID),
				"startDate": time.Now().Add(36 * time.Hour).Unix(),
				"endDate":   time.Now().Add(60 * time.Hour).Unix(),
				"contact":   "test3@example.com",
				"comments":  "Test booking 3",
			},
			"bookings",
		)
		qt.Assert(t, code, qt.Equals, 500, qt.Commentf("Response: %s", string(data)))

		// Get booking requests (owner) - should show both pending and accepted bookings
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests")
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

		// Get booking petitions (renter) - should show both pending and accepted bookings
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "petitions")
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
		_, code = c.Request(http.MethodPost, renterJWT, nil, "bookings", bookingID, "return")
		qt.Assert(t, code, qt.Equals, 403)

		// Mark as returned by owner
		_, code = c.Request(http.MethodPost, ownerJWT, nil, "bookings", bookingID, "return")
		qt.Assert(t, code, qt.Equals, 200)

		// Get pending ratings
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "rates")
		qt.Assert(t, code, qt.Equals, 200)
		var ratingsResp struct {
			Data []api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &ratingsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(ratingsResp.Data), qt.Equals, 1)

		// Submit rating
		_, code = c.Request(http.MethodPost, renterJWT,
			map[string]interface{}{
				"rating": 5,
			},
			"bookings", bookingID, "rate",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Test deny petition
		t.Run("Deny Petition", func(t *testing.T) {
			// Create a new booking to deny
			resp, code := c.Request(http.MethodPost, renterJWT,
				map[string]interface{}{
					"toolId":    fmt.Sprint(toolID),
					"startDate": time.Now().Add(72 * time.Hour).Unix(),
					"endDate":   time.Now().Add(96 * time.Hour).Unix(),
					"contact":   "test@example.com",
					"comments":  "Test booking to deny",
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
			_, code = c.Request(http.MethodPost, "", nil, "bookings", "petitions", denyBookingID, "deny")
			qt.Assert(t, code, qt.Equals, 401)

			// Try to deny as renter (should fail)
			_, code = c.Request(http.MethodPost, renterJWT, nil, "bookings", "petitions", denyBookingID, "deny")
			qt.Assert(t, code, qt.Equals, 403)

			// Deny as owner
			_, code = c.Request(http.MethodPost, ownerJWT, nil, "bookings", "petitions", denyBookingID, "deny")
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
				map[string]interface{}{
					"toolId":    fmt.Sprint(toolID),
					"startDate": time.Now().Add(120 * time.Hour).Unix(),
					"endDate":   time.Now().Add(144 * time.Hour).Unix(),
					"contact":   "test@example.com",
					"comments":  "Test booking to cancel",
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
			_, code = c.Request(http.MethodPost, "", nil, "bookings", "request", cancelBookingID, "cancel")
			qt.Assert(t, code, qt.Equals, 401)

			// Try to cancel as owner (should fail)
			_, code = c.Request(http.MethodPost, ownerJWT, nil, "bookings", "request", cancelBookingID, "cancel")
			qt.Assert(t, code, qt.Equals, 403)

			// Cancel as renter
			_, code = c.Request(http.MethodPost, renterJWT, nil, "bookings", "request", cancelBookingID, "cancel")
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

		// Test paginated user bookings
		t.Run("Get User Bookings", func(t *testing.T) {
			// Get first page of bookings
			resp, code := c.Request(http.MethodGet, renterJWT, nil, "bookings", "user", renterID, "?page=0")
			qt.Assert(t, code, qt.Equals, 200)
			var pageResp struct {
				Data []api.BookingResponse `json:"data"`
			}
			err := json.Unmarshal(resp, &pageResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(pageResp.Data), qt.Equals, 4) // Should show all bookings (accepted, pending, denied, and cancelled)

			// Verify bookings are ordered by date (newest first)
			if len(pageResp.Data) > 1 {
				for i := 1; i < len(pageResp.Data); i++ {
					qt.Assert(t, pageResp.Data[i-1].CreatedAt.After(pageResp.Data[i].CreatedAt), qt.IsTrue)
				}
			}

			// Test invalid page number
			_, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "user", renterID, "?page=-1")
			qt.Assert(t, code, qt.Equals, 400)

			// Test with non-existent user ID
			_, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "user", "invalid-id")
			qt.Assert(t, code, qt.Equals, 400)

			// Test without authentication
			_, code = c.Request(http.MethodGet, "", nil, "bookings", "user", renterID)
			qt.Assert(t, code, qt.Equals, 401)
		})

		// Test count pending actions
		t.Run("Count Pending Actions", func(t *testing.T) {
			// Create two users: tool owner and renter
			ownerJWT := c.RegisterAndLogin("owner2@test.com", "owner2", "ownerpass")
			renterJWT := c.RegisterAndLogin("renter2@test.com", "renter2", "renterpass")

			// Try to get count without auth
			_, code := c.Request(http.MethodGet, "", nil, "bookings", "pendings")
			qt.Assert(t, code, qt.Equals, 401)

			// Create a tool for owner
			toolID := c.CreateTool(ownerJWT, "Test Tool")

			// Get count for owner (should have 0 pending booking)
			resp, code := c.Request(http.MethodGet, ownerJWT, nil, "bookings", "pendings")
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
				map[string]interface{}{
					"toolId":    fmt.Sprint(toolID),
					"startDate": time.Now().Add(168 * time.Hour).Unix(),
					"endDate":   time.Now().Add(192 * time.Hour).Unix(),
					"contact":   "test@example.com",
					"comments":  "Another pending booking",
				},
				"bookings",
			)
			qt.Assert(t, code, qt.Equals, 200)

			// Verify owner now has 1 pending actions
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "pendings")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &countResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, countResp.Data.PendingRequestsCount, qt.Equals, int64(1))
			qt.Assert(t, countResp.Data.PendingRatingsCount, qt.Equals, int64(0))

			// Get booking requests (owner)
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests")
			qt.Assert(t, code, qt.Equals, 200)
			var petitionsResp struct {
				Data []api.BookingResponse `json:"data"`
			}
			err = json.Unmarshal(resp, &petitionsResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(petitionsResp.Data), qt.Equals, 1)

			// Accept the booking request
			bookingID := petitionsResp.Data[0].ID
			_, code = c.Request(http.MethodPost, ownerJWT, nil, "bookings", "petitions", bookingID, "accept")
			qt.Assert(t, code, qt.Equals, 200)

			// Verify owner now has 0 pending request and 1 pending rating
			resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "pendings")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &countResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, countResp.Data.PendingRequestsCount, qt.Equals, int64(0))
			qt.Assert(t, countResp.Data.PendingRatingsCount, qt.Equals, int64(1))
		})
	})
}
