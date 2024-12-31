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

		// Try to create overlapping booking
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
		qt.Assert(t, code, qt.Equals, 400)

		// Get booking requests (owner)
		resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests")
		qt.Assert(t, code, qt.Equals, 200)
		var requestsResp struct {
			Data []api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &requestsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(requestsResp.Data), qt.Equals, 1)

		// Get booking petitions (renter)
		resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "petitions")
		qt.Assert(t, code, qt.Equals, 200)
		var petitionsResp struct {
			Data []api.BookingResponse `json:"data"`
		}
		err = json.Unmarshal(resp, &petitionsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(petitionsResp.Data), qt.Equals, 1)

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
