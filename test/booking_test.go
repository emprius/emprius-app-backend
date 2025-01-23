package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestBookings(t *testing.T) {
	c := utils.NewTestService(t)

	// Create two users: tool owner and renter
	ownerJWT := c.RegisterAndLogin("owner@test.com", "owner", "ownerpass")
	renterJWT := c.RegisterAndLogin("renter@test.com", "renter", "renterpass")

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
				"rating":    5,
				"bookingId": bookingID,
			},
			"bookings", "rates",
		)
		qt.Assert(t, code, qt.Equals, 200)
	})
}
