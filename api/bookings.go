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

// convertBookingToResponse converts a db.Booking and an associated db.BookingRating
// into a BookingResponse. If rating is nil, the booking is considered not yet rated.
func convertBookingToResponse(booking *db.BookingWithRatings) *BookingResponse {
	return &BookingResponse{
		ID:            booking.ID.Hex(),
		ToolID:        booking.ToolID,
		FromUserID:    booking.FromUserID.Hex(),
		ToUserID:      booking.ToUserID.Hex(),
		StartDate:     booking.StartDate.Unix(),
		EndDate:       booking.EndDate.Unix(),
		Contact:       booking.Contact,
		Comments:      booking.Comments,
		BookingStatus: string(booking.BookingStatus),
		CreatedAt:     booking.CreatedAt.Unix(),
		UpdatedAt:     booking.UpdatedAt.Unix(),
		IsRated:       len(booking.Ratings) > 0,
	}
}

// HandleGetOutgoingRequests handles GET /bookings/requests/outgoing
func (a *API) HandleGetOutgoingRequests(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user from database
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	bookings, err := a.database.BookingService.GetUserPetitions(r.Context.Request.Context(), user.ObjectID())
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	response := make([]*BookingResponse, len(bookings))
	for i, booking := range bookings {
		response[i] = convertBookingToResponse(booking)
	}

	return response, nil
}

// HandleGetIncomingRequests handles GET /bookings/requests/incoming
func (a *API) HandleGetIncomingRequests(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user from database
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	bookings, err := a.database.BookingService.GetUserRequests(r.Context.Request.Context(), user.ObjectID())
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	response := make([]*BookingResponse, len(bookings))
	for i, booking := range bookings {
		response[i] = convertBookingToResponse(booking)
	}

	return response, nil
}

// HandleGetBooking handles GET /bookings/{bookingId}
func (a *API) HandleGetBooking(r *Request) (interface{}, error) {
	bookingID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "bookingId"))
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), bookingID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	if booking == nil {
		return nil, ErrBookingNotFound.WithErr(fmt.Errorf("booking with id %s not found", bookingID.Hex()))
	}

	return convertBookingToResponse(booking), nil
}

// HandleAcceptPetition handles POST /bookings/petitions/{petitionId}/accept
func (a *API) HandleAcceptPetition(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user from database
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	petitionID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "petitionId"))
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), petitionID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	if booking == nil {
		return nil, ErrBookingNotFound.WithErr(fmt.Errorf("booking with id %s not found", petitionID.Hex()))
	}

	// Verify user is the tool owner
	if booking.ToUserID != user.ObjectID() {
		return nil, ErrOnlyOwnerCanAccept.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
	}

	// Verify booking is in PENDING state
	if booking.BookingStatus != db.BookingStatusPending {
		return nil, ErrCanOnlyAcceptPending.WithErr(fmt.Errorf("booking status is %s", booking.BookingStatus))
	}

	err = a.database.BookingService.UpdateStatus(r.Context.Request.Context(), petitionID, db.BookingStatusAccepted)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return nil, nil
}

// HandleDenyPetition handles POST /bookings/petitions/{petitionId}/deny
func (a *API) HandleDenyPetition(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user from database
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	petitionID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "petitionId"))
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), petitionID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	if booking == nil {
		return nil, ErrBookingNotFound.WithErr(fmt.Errorf("booking with id %s not found", petitionID.Hex()))
	}

	// Verify user is the tool owner
	if booking.ToUserID != user.ObjectID() {
		return nil, ErrOnlyOwnerCanDeny.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
	}

	// Verify booking is in PENDING state
	if booking.BookingStatus != db.BookingStatusPending {
		return nil, ErrCanOnlyDenyPending.WithErr(fmt.Errorf("booking status is %s", booking.BookingStatus))
	}

	err = a.database.BookingService.UpdateStatus(r.Context.Request.Context(), petitionID, db.BookingStatusRejected)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return nil, nil
}

// HandleCancelRequest handles POST /bookings/request/{petitionId}/cancel
func (a *API) HandleCancelRequest(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user from database
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	petitionID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "petitionId"))
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), petitionID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	if booking == nil {
		return nil, ErrBookingNotFound.WithErr(fmt.Errorf("booking with id %s not found", petitionID.Hex()))
	}

	// Verify user is the requester
	if booking.FromUserID != user.ObjectID() {
		return nil, ErrOnlyRequesterCanCancel.WithErr(fmt.Errorf("user %s is not the requester", user.ID))
	}

	// Verify booking is in PENDING state
	if booking.BookingStatus != db.BookingStatusPending {
		return nil, ErrCanOnlyCancelPending.WithErr(fmt.Errorf("booking status is %s", booking.BookingStatus))
	}

	err = a.database.BookingService.UpdateStatus(r.Context.Request.Context(), petitionID, db.BookingStatusCancelled)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return nil, nil
}

// HandleUpdateBookingStatus handles POST /bookings/{bookingId}
// This replaces the individual verb-based endpoints (accept, deny, cancel, return)
func (a *API) HandleUpdateBookingStatus(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user from database
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	bookingID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "bookingId"))
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), bookingID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	if booking == nil {
		return nil, ErrBookingNotFound.WithErr(fmt.Errorf("booking with id %s not found", bookingID.Hex()))
	}

	// Parse the status update request
	var statusUpdate BookingStatusUpdate
	if err := json.Unmarshal(r.Data, &statusUpdate); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Validate the requested status
	var newStatus db.BookingStatus
	switch statusUpdate.Status {
	case "ACCEPTED":
		newStatus = db.BookingStatusAccepted
		// Verify user is the tool owner
		if booking.ToUserID != user.ObjectID() {
			return nil, ErrOnlyOwnerCanAccept.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
		}
		// Verify booking is in PENDING state
		if booking.BookingStatus != db.BookingStatusPending {
			return nil, ErrCanOnlyAcceptPending.WithErr(fmt.Errorf("booking status is %s", booking.BookingStatus))
		}
	case "REJECTED":
		newStatus = db.BookingStatusRejected
		// Verify user is the tool owner
		if booking.ToUserID != user.ObjectID() {
			return nil, ErrOnlyOwnerCanDeny.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
		}
		// Verify booking is in PENDING state
		if booking.BookingStatus != db.BookingStatusPending {
			return nil, ErrCanOnlyDenyPending.WithErr(fmt.Errorf("booking status is %s", booking.BookingStatus))
		}
	case "CANCELLED":
		newStatus = db.BookingStatusCancelled
		// Verify user is the requester
		if booking.FromUserID != user.ObjectID() {
			return nil, ErrOnlyRequesterCanCancel.WithErr(fmt.Errorf("user %s is not the requester", user.ID))
		}
		// Verify booking is in PENDING state
		if booking.BookingStatus != db.BookingStatusPending {
			return nil, ErrCanOnlyCancelPending.WithErr(fmt.Errorf("booking status is %s", booking.BookingStatus))
		}
	case "RETURNED":
		newStatus = db.BookingStatusReturned
		// Verify user is the tool owner
		if booking.ToUserID != user.ObjectID() {
			return nil, ErrOnlyOwnerCanReturn.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
		}
		// Verify booking is in ACCEPTED state
		if booking.BookingStatus != db.BookingStatusAccepted {
			return nil, ErrInvalidBookingStatus.WithErr(fmt.Errorf("booking must be in ACCEPTED state to be returned"))
		}
	default:
		return nil, ErrInvalidBookingStatus.WithErr(fmt.Errorf("invalid status: %s", statusUpdate.Status))
	}

	// Update the booking status
	err = a.database.BookingService.UpdateStatus(r.Context.Request.Context(), bookingID, newStatus)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Get the updated booking to return
	updatedBooking, err := a.database.BookingService.Get(r.Context.Request.Context(), bookingID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return convertBookingToResponse(updatedBooking), nil
}

// HandleGetPendingRatings handles GET /bookings/ratings/pending
func (a *API) HandleGetPendingRatings(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user from database
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	bookings, err := a.database.BookingService.GetPendingRatings(
		r.Context.Request.Context(),
		user.ObjectID(),
	)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	response := make([]*BookingResponse, len(bookings))
	for i, booking := range bookings {
		response[i] = convertBookingToResponse(&db.BookingWithRatings{Booking: *booking})
	}

	return response, nil
}

// HandleGetUserRatings handles GET /users/{id}/ratings
// Returns a unified list of all ratings (both submitted and received) for a user
func (a *API) HandleGetUserRatings(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user ID from URL
	userID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "id"))
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get unified ratings
	unifiedRatings, err := a.database.BookingService.GetUnifiedRatings(r.Context.Request.Context(), userID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	if unifiedRatings == nil {
		// Return empty array instead of nil
		unifiedRatings = make([]*db.UnifiedRating, 0)
	}

	return unifiedRatings, nil
}

// HandleGetBookingRatings handles GET /bookings/{bookingId}/ratings
func (a *API) HandleGetBookingRatings(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get booking ID from URL
	bookingID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "bookingId"))
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Check if booking exists
	booking, err := a.database.BookingService.Get(r.Context.Request.Context(), bookingID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	if booking == nil {
		return nil, ErrBookingNotFound.WithErr(fmt.Errorf("booking with id %s not found", bookingID.Hex()))
	}

	// Get unified rating for the booking
	unifiedRating, err := a.database.BookingService.GetRatingsByBookingID(r.Context.Request.Context(), bookingID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Return the unified rating directly
	return unifiedRating, nil
}

// RateRequest represents the request body for rating a booking
type RateRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

// HandleCreateBooking handles POST /bookings
func (a *API) HandleCreateBooking(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user from database
	fromUser, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	var req CreateBookingRequest
	if err := json.Unmarshal(r.Data, &req); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	toolID, err := strconv.ParseInt(req.ToolID, 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("invalid tool ID format: %s", req.ToolID))
	}

	// Get tool to verify it exists and get owner ID
	tool, err := a.database.ToolService.GetToolByID(r.Context.Request.Context(), toolID)
	if err != nil {
		return nil, ErrBookingNotFound.WithErr(err)
	}
	if tool == nil {
		return nil, ErrToolNotFound.WithErr(fmt.Errorf("tool with id %d not found", toolID))
	}

	toUser, err := a.database.UserService.GetUserByID(r.Context.Request.Context(), tool.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(fmt.Errorf("tool owner not found: %w", err))
	}

	// Validate dates
	startDate := time.Unix(req.StartDate, 0)
	endDate := time.Unix(req.EndDate, 0)
	now := time.Now().Truncate(24 * time.Hour) // Truncate to start of day for comparison

	if startDate.Before(now) {
		return nil, ErrInvalidBookingDates.WithErr(fmt.Errorf("start date must not be before today"))
	}

	if endDate.Before(startDate) {
		return nil, ErrInvalidBookingDates.WithErr(fmt.Errorf("end date must not be before start date"))
	}

	// Create booking request
	dbReq := &db.CreateBookingRequest{
		ToolID:    fmt.Sprintf("%d", toolID),
		StartDate: startDate,
		EndDate:   endDate,
		Contact:   req.Contact,
		Comments:  req.Comments,
	}
	booking, err := a.database.BookingService.Create(r.Context.Request.Context(), dbReq, fromUser.ObjectID(), toUser.ID)
	if err != nil {
		if err.Error() == "booking dates conflict with existing booking" {
			return nil, ErrBookingDatesConflict.WithErr(err)
		}
		return nil, ErrInternalServerError.WithErr(err)
	}

	return convertBookingToResponse(&db.BookingWithRatings{Booking: *booking, Ratings: nil}), nil
}

// HandleRateBooking handles POST /bookings/{bookingId}/ratings
func (a *API) HandleRateBooking(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user from database
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	var rateReq RateRequest
	if err := json.Unmarshal(r.Data, &rateReq); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Validate rating value
	if rateReq.Rating < 1 || rateReq.Rating > 5 {
		return nil, ErrInvalidRating.WithErr(fmt.Errorf("rating value %d is not between 1 and 5", rateReq.Rating))
	}

	bookingID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "bookingId"))
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Rate the booking
	err = a.database.BookingService.RateBooking(
		r.Context.Request.Context(),
		bookingID,
		user.ObjectID(),
		rateReq.Rating,
		rateReq.Comment,
	)
	if err != nil {
		switch {
		case err == db.ErrBookingNotFound:
			return nil, ErrBookingNotFound.WithErr(err)
		case err.Error() == "booking must be in RETURNED state to be rated":
			return nil, ErrInvalidBookingStatus.WithErr(err)
		case err.Error() == "user has already rated this booking":
			return nil, ErrAlreadyRated.WithErr(err)
		case err.Error() == "user is not involved in this booking":
			return nil, ErrUserNotInvolved.WithErr(err)
		default:
			return nil, ErrInternalServerError.WithErr(err)
		}
	}

	return nil, nil
}

// HandleCountPendingActions handles GET /profile/pendings
func (a *API) HandleCountPendingActions(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}
	uID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	pending, err := a.database.BookingService.CountPendingActions(r.Context.Request.Context(), uID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	return pending, nil
}
