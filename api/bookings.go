package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/emprius/emprius-app-backend/db"
)

// convertBookingToResponse converts a db.Booking to a BookingResponse
func convertBookingToResponse(booking *db.Booking) BookingResponse {
	return BookingResponse{
		ID:            booking.ID.Hex(),
		ToolID:        booking.ToolID.Hex(),
		FromUserID:    booking.FromUserID.Hex(),
		ToUserID:      booking.ToUserID.Hex(),
		StartDate:     booking.StartDate.Unix(),
		EndDate:       booking.EndDate.Unix(),
		Contact:       booking.Contact,
		Comments:      booking.Comments,
		BookingStatus: string(booking.BookingStatus),
		CreatedAt:     booking.CreatedAt,
		UpdatedAt:     booking.UpdatedAt,
	}
}

// HandleGetBookingRequests handles GET /bookings/requests
func (a *API) HandleGetBookingRequests(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	// Get user from database
	user, err := a.database.UserService.GetUserByEmail(r.Context.Request.Context(), r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	bookings, err := a.database.BookingService.GetUserRequests(r.Context.Request.Context(), user.ID)
	if err != nil {
		return nil, err
	}

	response := make([]BookingResponse, len(bookings))
	for i, booking := range bookings {
		response[i] = convertBookingToResponse(booking)
	}

	return response, nil
}

// HandleGetBookingPetitions handles GET /bookings/petitions
func (a *API) HandleGetBookingPetitions(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	// Get user from database
	user, err := a.database.UserService.GetUserByEmail(r.Context.Request.Context(), r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	bookings, err := a.database.BookingService.GetUserPetitions(r.Context.Request.Context(), user.ID)
	if err != nil {
		return nil, err
	}

	response := make([]BookingResponse, len(bookings))
	for i, booking := range bookings {
		response[i] = convertBookingToResponse(booking)
	}

	return response, nil
}

// HandleGetBooking handles GET /bookings/{bookingId}
func (a *API) HandleGetBooking(r *Request) (interface{}, error) {
	bookingID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "bookingId"))
	if err != nil {
		return nil, fmt.Errorf("invalid booking ID")
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), bookingID)
	if err != nil {
		return nil, err
	}
	if booking == nil {
		return nil, fmt.Errorf("booking not found")
	}

	return convertBookingToResponse(booking), nil
}

// HandleAcceptPetition handles POST /bookings/petitions/{petitionId}/accept
func (a *API) HandleAcceptPetition(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	// Get user from database
	user, err := a.database.UserService.GetUserByEmail(r.Context.Request.Context(), r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	petitionID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "petitionId"))
	if err != nil {
		return nil, fmt.Errorf("invalid petition ID")
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), petitionID)
	if err != nil {
		return nil, err
	}
	if booking == nil {
		return nil, fmt.Errorf("booking not found")
	}

	// Verify user is the tool owner
	if booking.ToUserID != user.ID {
		return nil, &HTTPError{
			Code:    403,
			Message: "only tool owner can accept petitions",
		}
	}

	// Verify booking is in PENDING state
	if booking.BookingStatus != db.BookingStatusPending {
		return nil, fmt.Errorf("can only accept pending petitions")
	}

	err = a.database.BookingService.UpdateStatus(r.Context.Request.Context(), petitionID, db.BookingStatusAccepted)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// HandleDenyPetition handles POST /bookings/petitions/{petitionId}/deny
func (a *API) HandleDenyPetition(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	// Get user from database
	user, err := a.database.UserService.GetUserByEmail(r.Context.Request.Context(), r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	petitionID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "petitionId"))
	if err != nil {
		return nil, fmt.Errorf("invalid petition ID")
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), petitionID)
	if err != nil {
		return nil, err
	}
	if booking == nil {
		return nil, fmt.Errorf("booking not found")
	}

	// Verify user is the tool owner
	if booking.ToUserID != user.ID {
		return nil, &HTTPError{
			Code:    403,
			Message: "only tool owner can deny petitions",
		}
	}

	// Verify booking is in PENDING state
	if booking.BookingStatus != db.BookingStatusPending {
		return nil, fmt.Errorf("can only deny pending petitions")
	}

	err = a.database.BookingService.UpdateStatus(r.Context.Request.Context(), petitionID, db.BookingStatusRejected)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// HandleCancelRequest handles POST /bookings/request/{petitionId}/cancel
func (a *API) HandleCancelRequest(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	// Get user from database
	user, err := a.database.UserService.GetUserByEmail(r.Context.Request.Context(), r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	petitionID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "petitionId"))
	if err != nil {
		return nil, fmt.Errorf("invalid petition ID")
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), petitionID)
	if err != nil {
		return nil, err
	}
	if booking == nil {
		return nil, fmt.Errorf("booking not found")
	}

	// Verify user is the requester
	if booking.FromUserID != user.ID {
		return nil, &HTTPError{
			Code:    403,
			Message: "only requester can cancel their requests",
		}
	}

	// Verify booking is in PENDING state
	if booking.BookingStatus != db.BookingStatusPending {
		return nil, fmt.Errorf("can only cancel pending requests")
	}

	err = a.database.BookingService.UpdateStatus(r.Context.Request.Context(), petitionID, db.BookingStatusCancelled)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// HandleReturnBooking handles POST /bookings/{bookingId}/return
func (a *API) HandleReturnBooking(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	// Get user from database
	user, err := a.database.UserService.GetUserByEmail(r.Context.Request.Context(), r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	bookingID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "bookingId"))
	if err != nil {
		return nil, fmt.Errorf("invalid booking ID")
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), bookingID)
	if err != nil {
		return nil, err
	}
	if booking == nil {
		return nil, fmt.Errorf("booking not found")
	}

	// Verify user is the tool owner
	if booking.ToUserID != user.ID {
		return nil, &HTTPError{
			Code:    403,
			Message: "only tool owner can mark as returned",
		}
	}

	err = a.database.BookingService.UpdateStatus(r.Context.Request.Context(), bookingID, db.BookingStatusReturned)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// HandleGetPendingRatings handles GET /bookings/rates
func (a *API) HandleGetPendingRatings(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	// Get user from database
	user, err := a.database.UserService.GetUserByEmail(r.Context.Request.Context(), r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	bookings, err := a.database.BookingService.GetPendingRatings(r.Context.Request.Context(), user.ID)
	if err != nil {
		return nil, err
	}

	response := make([]BookingResponse, len(bookings))
	for i, booking := range bookings {
		response[i] = convertBookingToResponse(booking)
	}

	return response, nil
}

// RateRequest represents the request body for rating a booking
type RateRequest struct {
	Rating    int    `json:"rating"`
	BookingID string `json:"bookingId"`
}

// HandleCreateBooking handles POST /bookings
func (a *API) HandleCreateBooking(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	// Get user from database
	fromUser, err := a.database.UserService.GetUserByEmail(r.Context.Request.Context(), r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	var req CreateBookingRequest
	if err := json.Unmarshal(r.Data, &req); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}

	toolID, err := strconv.ParseInt(req.ToolID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid tool ID")
	}

	// Get tool to verify it exists and get owner ID
	tool, err := a.database.ToolService.GetToolByID(r.Context.Request.Context(), toolID)
	if err != nil {
		return nil, err
	}
	if tool == nil {
		return nil, fmt.Errorf("tool not found")
	}

	toUser, err := a.database.UserService.GetUserByEmail(r.Context.Request.Context(), tool.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid tool owner ID: %w", err)
	}

	// Create booking request
	dbReq := &db.CreateBookingRequest{
		ToolID:    primitive.NewObjectID(), // Generate new ID for the booking
		StartDate: time.Unix(req.StartDate, 0),
		EndDate:   time.Unix(req.EndDate, 0),
		Contact:   req.Contact,
		Comments:  req.Comments,
	}

	booking, err := a.database.BookingService.Create(r.Context.Request.Context(), dbReq, fromUser.ID, toUser.ID)
	if err != nil {
		return nil, err
	}

	return convertBookingToResponse(booking), nil
}

// HandleRateBooking handles POST /bookings/rates
func (a *API) HandleRateBooking(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	// Get user from database
	user, err := a.database.UserService.GetUserByEmail(r.Context.Request.Context(), r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	var rateReq RateRequest
	if err := json.Unmarshal(r.Data, &rateReq); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}

	bookingID, err := primitive.ObjectIDFromHex(rateReq.BookingID)
	if err != nil {
		return nil, fmt.Errorf("invalid booking ID")
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), bookingID)
	if err != nil {
		return nil, err
	}
	if booking == nil {
		return nil, fmt.Errorf("booking not found")
	}

	// Verify user is involved in the booking
	if booking.FromUserID != user.ID && booking.ToUserID != user.ID {
		return nil, fmt.Errorf("user not involved in booking")
	}

	// TODO: Implement rating logic once rating schema is defined

	return nil, nil
}
