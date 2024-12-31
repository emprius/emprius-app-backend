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

// BookingResponse represents the API response for a booking
type BookingResponse struct {
	ID            string    `json:"id"`
	ToolID        string    `json:"toolId"`
	FromUserID    string    `json:"fromUserId"`
	ToUserID      string    `json:"toUserId"`
	StartDate     int64     `json:"startDate"`
	EndDate       int64     `json:"endDate"`
	Contact       string    `json:"contact"`
	Comments      string    `json:"comments"`
	BookingStatus string    `json:"bookingStatus"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

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

	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}

	bookings, err := a.database.BookingService.GetUserRequests(r.Context.Request.Context(), userID)
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

	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}

	bookings, err := a.database.BookingService.GetUserPetitions(r.Context.Request.Context(), userID)
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

// HandleReturnBooking handles POST /bookings/{bookingId}/return
func (a *API) HandleReturnBooking(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
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
	if booking.ToUserID != userID {
		return nil, fmt.Errorf("only tool owner can mark as returned")
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

	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}

	bookings, err := a.database.BookingService.GetPendingRatings(r.Context.Request.Context(), userID)
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
	Rating int `json:"rating"`
}

// CreateBookingRequest represents the request to create a new booking
type CreateBookingRequest struct {
	ToolID    string `json:"toolId"`
	StartDate int64  `json:"startDate"`
	EndDate   int64  `json:"endDate"`
	Contact   string `json:"contact"`
	Comments  string `json:"comments"`
}

// HandleCreateBooking handles POST /bookings
func (a *API) HandleCreateBooking(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	fromUserID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
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

	toUserID, err := primitive.ObjectIDFromHex(tool.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid tool owner ID")
	}

	// Convert tool ID to ObjectID for booking
	toolObjID, err := primitive.ObjectIDFromHex(req.ToolID)
	if err != nil {
		return nil, fmt.Errorf("invalid tool ID format")
	}

	// Create booking request
	dbReq := &db.CreateBookingRequest{
		ToolID:    toolObjID,
		StartDate: time.Unix(req.StartDate, 0),
		EndDate:   time.Unix(req.EndDate, 0),
		Contact:   req.Contact,
		Comments:  req.Comments,
	}

	booking, err := a.database.BookingService.Create(r.Context.Request.Context(), dbReq, fromUserID, toUserID)
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

	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}

	bookingID, err := primitive.ObjectIDFromHex(r.Context.Request.URL.Query().Get("bookingId"))
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
	if booking.FromUserID != userID && booking.ToUserID != userID {
		return nil, fmt.Errorf("user not involved in booking")
	}

	var rateReq RateRequest
	if err := json.Unmarshal(r.Data, &rateReq); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}

	// TODO: Implement rating logic once rating schema is defined

	return nil, nil
}
