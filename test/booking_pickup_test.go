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

func TestBookingPickupPlace(t *testing.T) {
	c := utils.NewTestService(t)

	// Create two users: tool owner and renter
	ownerJWT, _ := c.RegisterAndLoginWithID("pickup-owner@test.com", "pickup-owner", "ownerpass")
	renterJWT, _ := c.RegisterAndLoginWithID("pickup-renter@test.com", "pickup-renter", "renterpass")

	// Create a third user who is not involved in the booking
	uninvolvedJWT, _ := c.RegisterAndLoginWithID("pickup-uninvolved@test.com", "pickup-uninvolved", "uninvolvedpass")

	// Owner creates a tool
	toolID := c.CreateTool(ownerJWT, "Pickup Test Tool")

	// Renter creates a booking
	tomorrow := time.Now().Add(24 * time.Hour)
	dayAfterTomorrow := time.Now().Add(48 * time.Hour)

	resp, code := c.Request(http.MethodPost, renterJWT,
		api.CreateBookingRequest{
			ToolID:    fmt.Sprint(toolID),
			StartDate: tomorrow.Unix(),
			EndDate:   dayAfterTomorrow.Unix(),
			Contact:   "test@example.com",
			Comments:  "Test booking for pickup place",
		},
		"bookings",
	)
	qt.Assert(t, code, qt.Equals, 200)

	var bookingResp struct {
		Data api.BookingResponse `json:"data"`
	}
	var notInvolvedResp struct {
		Data api.BookingResponse `json:"data"`
	}
	err := json.Unmarshal(resp, &bookingResp)
	qt.Assert(t, err, qt.IsNil)
	bookingID := bookingResp.Data.ID

	// Verify pickup place is not included in the response for a pending booking
	qt.Assert(t, bookingResp.Data.PickupPlace, qt.IsNil)

	// Owner accepts the booking
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "ACCEPTED",
		}, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	// Test 1: Renter (involved user) gets the booking with pickup place
	resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	err = json.Unmarshal(resp, &bookingResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "ACCEPTED")
	// Pickup place should be included for involved user (renter)
	qt.Assert(t, bookingResp.Data.PickupPlace, qt.Not(qt.IsNil))

	// Test 2: Owner (involved user) gets the booking with pickup place
	resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	err = json.Unmarshal(resp, &bookingResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "ACCEPTED")
	// Pickup place should be included for involved user (owner)
	qt.Assert(t, bookingResp.Data.PickupPlace, qt.Not(qt.IsNil))

	// Test 3: Uninvolved user gets the booking without pickup place
	resp, code = c.Request(http.MethodGet, uninvolvedJWT, nil, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	err = json.Unmarshal(resp, &notInvolvedResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, notInvolvedResp.Data.BookingStatus, qt.Equals, "ACCEPTED")
	// Pickup place should not be included for uninvolved user
	qt.Assert(t, notInvolvedResp.Data.PickupPlace, qt.IsNil)

	// Test 4: Check pickup place in outgoing requests list for renter
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
			// Pickup place should be included in outgoing requests for involved user
			qt.Assert(t, booking.PickupPlace, qt.Not(qt.IsNil))
			break
		}
	}
	// Booking should be found in outgoing requests
	qt.Assert(t, foundBooking, qt.IsTrue)

	// Test 5: Check pickup place in incoming requests list for owner
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
			// Pickup place should be included in incoming requests for involved user
			qt.Assert(t, booking.PickupPlace, qt.Not(qt.IsNil))
			break
		}
	}
	// Booking should be found in incoming requests
	qt.Assert(t, foundBooking, qt.IsTrue)

	// Test 6: Mark booking as returned and verify pickup place is no longer included
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "RETURNED",
		}, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	err = json.Unmarshal(resp, &notInvolvedResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, notInvolvedResp.Data.BookingStatus, qt.Equals, "RETURNED")
	// Pickup place should be included for involved user (owner)
	qt.Assert(t, bookingResp.Data.PickupPlace, qt.Not(qt.IsNil))
}

func TestNomadicToolPickupPlace(t *testing.T) {
	c := utils.NewTestService(t)

	// Create two users: tool owner and renter
	ownerJWT, _ := c.RegisterAndLoginWithID("nomadic-pickup-owner@test.com", "nomadic-pickup-owner", "ownerpass")
	renterJWT, _ := c.RegisterAndLoginWithID("nomadic-pickup-renter@test.com", "nomadic-pickup-renter", "renterpass")

	// Create a third user who is not involved in the booking
	uninvolvedJWT, _ := c.RegisterAndLoginWithID("nomadic-pickup-uninvolved@test.com", "nomadic-pickup-uninvolved", "uninvolvedpass")

	// Owner creates a nomadic tool
	createToolResp, code := c.Request(http.MethodPost, ownerJWT, map[string]interface{}{
		"title":          "Nomadic Pickup Test Tool",
		"description":    "This tool changes location when rented",
		"toolCategory":   1,
		"estimatedValue": 100,
		"isNomadic":      true,
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

	// Renter creates a booking
	tomorrow := time.Now().Add(24 * time.Hour)
	dayAfterTomorrow := time.Now().Add(48 * time.Hour)

	resp, code := c.Request(http.MethodPost, renterJWT,
		api.CreateBookingRequest{
			ToolID:    fmt.Sprint(nomadicToolID),
			StartDate: tomorrow.Unix(),
			EndDate:   dayAfterTomorrow.Unix(),
			Contact:   "test@example.com",
			Comments:  "Test booking for nomadic tool pickup place",
		},
		"bookings",
	)
	qt.Assert(t, code, qt.Equals, 200)

	var bookingResp struct {
		Data api.BookingResponse `json:"data"`
	}
	var notInvolvedResp struct {
		Data api.BookingResponse `json:"data"`
	}
	err = json.Unmarshal(resp, &bookingResp)
	qt.Assert(t, err, qt.IsNil)
	bookingID := bookingResp.Data.ID

	// Verify pickup place is not included in the response for a pending booking
	qt.Assert(t, bookingResp.Data.PickupPlace, qt.IsNil)

	// Owner accepts the booking
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "ACCEPTED",
		}, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	// Test 1: Renter (involved user) gets the booking with pickup place when ACCEPTED
	resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	err = json.Unmarshal(resp, &bookingResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "ACCEPTED")
	// Pickup place should be included for involved user (renter) when ACCEPTED
	qt.Assert(t, bookingResp.Data.PickupPlace, qt.Not(qt.IsNil))

	// Test 2: Owner (involved user) gets the booking with pickup place when ACCEPTED
	resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	err = json.Unmarshal(resp, &bookingResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "ACCEPTED")
	// Pickup place should be included for involved user (owner) when ACCEPTED
	qt.Assert(t, bookingResp.Data.PickupPlace, qt.Not(qt.IsNil))

	// Test 3: Uninvolved user gets the booking without pickup place when ACCEPTED
	resp, code = c.Request(http.MethodGet, uninvolvedJWT, nil, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	err = json.Unmarshal(resp, &notInvolvedResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, notInvolvedResp.Data.BookingStatus, qt.Equals, "ACCEPTED")
	// Pickup place should not be included for uninvolved user when ACCEPTED
	qt.Assert(t, notInvolvedResp.Data.PickupPlace, qt.IsNil)

	// Mark as picked by owner
	_, code = c.Request(http.MethodPut, ownerJWT,
		&api.BookingStatusUpdate{
			Status: "PICKED",
		}, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	// Test 4: Renter (involved user) gets the booking with pickup place when PICKED
	resp, code = c.Request(http.MethodGet, renterJWT, nil, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	err = json.Unmarshal(resp, &bookingResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "PICKED")
	// Pickup place should be included for involved user (renter) when PICKED
	qt.Assert(t, bookingResp.Data.PickupPlace, qt.Not(qt.IsNil))

	// Test 5: Owner (involved user) gets the booking with pickup place when PICKED
	resp, code = c.Request(http.MethodGet, ownerJWT, nil, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	err = json.Unmarshal(resp, &bookingResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, bookingResp.Data.BookingStatus, qt.Equals, "PICKED")
	// Pickup place should be included for involved user (owner) when PICKED
	qt.Assert(t, bookingResp.Data.PickupPlace, qt.Not(qt.IsNil))

	// Test 6: Uninvolved user gets the booking without pickup place when PICKED
	resp, code = c.Request(http.MethodGet, uninvolvedJWT, nil, "bookings", bookingID)
	qt.Assert(t, code, qt.Equals, 200)

	err = json.Unmarshal(resp, &notInvolvedResp)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, notInvolvedResp.Data.BookingStatus, qt.Equals, "PICKED")
	// Pickup place should not be included for uninvolved user when PICKED
	qt.Assert(t, notInvolvedResp.Data.PickupPlace, qt.IsNil)

	// Test 7: Check pickup place in outgoing requests list for renter when PICKED
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
			// Pickup place should be included in outgoing requests for involved user when PICKED
			qt.Assert(t, booking.PickupPlace, qt.Not(qt.IsNil))
			break
		}
	}
	// Booking should be found in outgoing requests
	qt.Assert(t, foundBooking, qt.IsTrue)

	// Test 8: Check pickup place in incoming requests list for owner when PICKED
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
			// Pickup place should be included in incoming requests for involved user when PICKED
			qt.Assert(t, booking.PickupPlace, qt.Not(qt.IsNil))
			break
		}
	}
	// Booking should be found in incoming requests
	qt.Assert(t, foundBooking, qt.IsTrue)
}
