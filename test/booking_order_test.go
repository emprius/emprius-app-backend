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

func TestBookingOrder(t *testing.T) {
	c := utils.NewTestService(t)
	var resp []byte
	var code int

	// Create two users: tool owner and renter
	ownerJWT := c.RegisterAndLogin("order-owner@test.com", "owner", "ownerpass")
	renterJWT, _ := c.RegisterAndLoginWithID("order-renter@test.com", "renter", "renterpass")

	// Owner creates a tool
	toolID := c.CreateTool(ownerJWT, "Order Test Tool")

	// First create a normal pending booking
	req1 := api.CreateBookingRequest{
		ToolID:    fmt.Sprint(toolID),
		StartDate: time.Now().Add(300 * time.Hour).Unix(),
		EndDate:   time.Now().Add(324 * time.Hour).Unix(),
		Contact:   "test@example.com",
		Comments:  "Pending booking 1",
	}
	_, code = c.Request(http.MethodPost, renterJWT, req1, "bookings")
	qt.Assert(t, code, qt.Equals, 200)

	// Create a second booking that will be accepted
	req2 := api.CreateBookingRequest{
		ToolID:    fmt.Sprint(toolID),
		StartDate: time.Now().Add(330 * time.Hour).Unix(),
		EndDate:   time.Now().Add(354 * time.Hour).Unix(),
		Contact:   "test@example.com",
		Comments:  "To be accepted booking",
	}
	resp, code = c.Request(http.MethodPost, renterJWT, req2, "bookings")
	qt.Assert(t, code, qt.Equals, 200)
	var createResp struct {
		Data api.BookingResponse `json:"data"`
	}
	err := json.Unmarshal(resp, &createResp)
	qt.Assert(t, err, qt.IsNil)
	bookingID2 := createResp.Data.ID

	// Create a third pending booking
	req3 := api.CreateBookingRequest{
		ToolID:    fmt.Sprint(toolID),
		StartDate: time.Now().Add(360 * time.Hour).Unix(),
		EndDate:   time.Now().Add(384 * time.Hour).Unix(),
		Contact:   "test@example.com",
		Comments:  "Pending booking 3",
	}
	_, code = c.Request(http.MethodPost, renterJWT, req3, "bookings")
	qt.Assert(t, code, qt.Equals, 200)

	// Accept the second booking
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "ACCEPTED",
		},
		"bookings", bookingID2)
	qt.Assert(t, code, qt.Equals, 200)

	// Now get the requests and check the order - for owner's view
	resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", "requests", "incoming")
	qt.Assert(t, code, qt.Equals, 200)

	var bookingsResp struct {
		Data []api.BookingResponse `json:"data"`
	}
	err = json.Unmarshal(resp, &bookingsResp)
	qt.Assert(t, err, qt.IsNil)

	// We should have at least 2 pending bookings and 1 accepted booking
	pendingCount := 0
	acceptedCount := 0
	for _, booking := range bookingsResp.Data {
		if booking.BookingStatus == string(db.BookingStatusPending) {
			pendingCount++
		} else if booking.BookingStatus == string(db.BookingStatusAccepted) {
			acceptedCount++
		}
	}

	qt.Assert(t, pendingCount >= 2, qt.IsTrue,
		qt.Commentf("Expected at least 2 PENDING bookings, got %d", pendingCount))
	qt.Assert(t, acceptedCount >= 1, qt.IsTrue,
		qt.Commentf("Expected at least 1 ACCEPTED booking, got %d", acceptedCount))

	// The first bookings should all be PENDING
	for i := 0; i < pendingCount; i++ {
		qt.Assert(t, bookingsResp.Data[i].BookingStatus, qt.Equals, string(db.BookingStatusPending),
			qt.Commentf("Expected booking at index %d to be PENDING, got %s",
				i, bookingsResp.Data[i].BookingStatus))
	}

	// Now check petitions (renter side) too
	resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", "requests", "outgoing")
	qt.Assert(t, code, qt.Equals, 200)

	var petitionsResp struct {
		Data []api.BookingResponse `json:"data"`
	}
	err = json.Unmarshal(resp, &petitionsResp)
	qt.Assert(t, err, qt.IsNil)

	// Reset counters
	pendingCount = 0
	acceptedCount = 0
	for _, booking := range petitionsResp.Data {
		if booking.BookingStatus == string(db.BookingStatusPending) {
			pendingCount++
		} else if booking.BookingStatus == string(db.BookingStatusAccepted) {
			acceptedCount++
		}
	}

	qt.Assert(t, pendingCount >= 2, qt.IsTrue)
	qt.Assert(t, acceptedCount >= 1, qt.IsTrue)

	// The first bookings should all be PENDING
	for i := 0; i < pendingCount; i++ {
		qt.Assert(t, petitionsResp.Data[i].BookingStatus, qt.Equals, string(db.BookingStatusPending),
			qt.Commentf("Expected booking at index %d to be PENDING, got %s",
				i, petitionsResp.Data[i].BookingStatus))
	}

	// Verify that accepted bookings come after pending bookings
	if pendingCount > 0 && acceptedCount > 0 {
		// Find the first accepted booking index
		firstAcceptedIdx := -1
		for i, booking := range petitionsResp.Data {
			if booking.BookingStatus == string(db.BookingStatusAccepted) {
				firstAcceptedIdx = i
				break
			}
		}

		qt.Assert(t, firstAcceptedIdx, qt.Equals, pendingCount,
			qt.Commentf("Expected first ACCEPTED booking at index %d, got it at %d",
				pendingCount, firstAcceptedIdx))
	}
}
