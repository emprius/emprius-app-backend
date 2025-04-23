package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/emprius/emprius-app-backend/types"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// convertBookingToResponse converts a db.Booking into a BookingResponse.
// The IsRated field is set based on whether the authenticated user has rated the booking.
func (a *API) convertBookingToResponse(booking *db.Booking, authenticatedUserID ...string) *BookingResponse {
	isRated := false

	// Check if the booking is rated by the authenticated user
	if len(authenticatedUserID) > 0 && authenticatedUserID[0] != "" &&
		booking.BookingStatus == db.BookingStatusReturned { // todo(kon): add picked status when nomadic tools implemented
		userID, err := primitive.ObjectIDFromHex(authenticatedUserID[0])
		if err == nil {
			// Get ratings for this booking
			ratings, err := a.database.BookingService.GetRatingsByBookingID(context.Background(), booking.ID)
			if err == nil && ratings != nil {
				// Check if the user is the owner or requester in this booking
				if booking.FromUserID == userID {
					// User is the requester, check if they have rated the owner
					isRated = ratings.Requester != nil && ratings.Requester.Rating != nil
				} else if booking.ToUserID == userID {
					// User is the owner, check if they have rated the requester
					isRated = ratings.Owner != nil && ratings.Owner.Rating != nil
				}
			}
		}
	}

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
		IsRated:       &isRated, // This field indicates if the booking is rated by the authenticated user
		IsNomadic:     booking.IsNomadic,
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
		response[i] = a.convertBookingToResponse(booking, r.UserID)
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
		response[i] = a.convertBookingToResponse(booking, r.UserID)
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

	return a.convertBookingToResponse(booking, r.UserID), nil
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
	case BookingStatusAccepted:
		newStatus = db.BookingStatusAccepted
		// Verify user is the tool owner
		if booking.ToUserID != user.ObjectID() {
			return nil, ErrOnlyOwnerCanAccept.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
		}
		// Verify booking is in PENDING state
		if booking.BookingStatus != db.BookingStatusPending {
			return nil, ErrCanOnlyAcceptPending.WithErr(fmt.Errorf("booking status is %s", booking.BookingStatus))
		}

		// Get the tool to check if it's nomadic and who is the actual user
		toolID, err := strconv.ParseInt(booking.ToolID, 10, 64)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("invalid tool ID: %s", booking.ToolID))
		}

		tool, err := a.database.ToolService.GetToolByID(r.Context.Request.Context(), toolID)
		if err != nil {
			return nil, ErrInternalServerError.WithErr(err)
		}
		if tool == nil {
			return nil, ErrToolNotFound.WithErr(fmt.Errorf("tool with id %d not found", toolID))
		}

		// Check if the user is authorized to accept the petition
		// For nomadic tools with an actual user set, the actual user should accept
		// For non-nomadic tools or nomadic tools without an actual user, the owner should accept
		isAuthorized := false
		if tool.IsNomadic && !tool.ActualUserID.IsZero() {
			// For nomadic tools with an actual user, the actual user should accept
			isAuthorized = tool.ActualUserID == user.ObjectID()
		} else {
			// For non-nomadic tools or nomadic tools without an actual user, the owner should accept
			isAuthorized = booking.ToUserID == user.ObjectID()
		}

		if !isAuthorized {
			if tool.IsNomadic && !tool.ActualUserID.IsZero() {
				return nil, ErrOnlyOwnerCanAccept.WithErr(fmt.Errorf("user %s is not the actual user of this nomadic tool", user.ID))
			}
			return nil, ErrOnlyOwnerCanAccept.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
		}
	case BookingStatusRejected:
		newStatus = db.BookingStatusRejected
		// Verify user is the tool owner
		if booking.ToUserID != user.ObjectID() {
			return nil, ErrOnlyOwnerCanDeny.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
		}
		// Verify booking is in PENDING state
		if booking.BookingStatus != db.BookingStatusPending {
			return nil, ErrCanOnlyDenyPending.WithErr(fmt.Errorf("booking status is %s", booking.BookingStatus))
		}

		// Get the tool to check if it's nomadic and who is the actual user
		toolID, err := strconv.ParseInt(booking.ToolID, 10, 64)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("invalid tool ID: %s", booking.ToolID))
		}

		tool, err := a.database.ToolService.GetToolByID(r.Context.Request.Context(), toolID)
		if err != nil {
			return nil, ErrInternalServerError.WithErr(err)
		}
		if tool == nil {
			return nil, ErrToolNotFound.WithErr(fmt.Errorf("tool with id %d not found", toolID))
		}

		// Check if the user is authorized to deny the petition
		// For nomadic tools with an actual user set, the actual user should deny
		// For non-nomadic tools or nomadic tools without an actual user, the owner should deny
		isAuthorized := false
		if tool.IsNomadic && !tool.ActualUserID.IsZero() {
			// For nomadic tools with an actual user, the actual user should deny
			isAuthorized = tool.ActualUserID == user.ObjectID()
		} else {
			// For non-nomadic tools or nomadic tools without an actual user, the owner should deny
			isAuthorized = booking.ToUserID == user.ObjectID()
		}

		if !isAuthorized {
			if tool.IsNomadic && !tool.ActualUserID.IsZero() {
				return nil, ErrOnlyOwnerCanDeny.WithErr(fmt.Errorf("user %s is not the actual user of this nomadic tool", user.ID))
			}
			return nil, ErrOnlyOwnerCanDeny.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
		}
	case BookingStatusCancelled:
		newStatus = db.BookingStatusCancelled
		// Verify user is the requester
		if booking.FromUserID != user.ObjectID() {
			return nil, ErrOnlyRequesterCanCancel.WithErr(fmt.Errorf("user %s is not the requester", user.ID))
		}
		// Verify booking is in PENDING state
		if booking.BookingStatus != db.BookingStatusPending {
			return nil, ErrCanOnlyCancelPending.WithErr(fmt.Errorf("booking status is %s", booking.BookingStatus))
		}
	case BookingStatusReturned:
		newStatus = db.BookingStatusReturned
		// Verify user is the tool owner
		if booking.ToUserID != user.ObjectID() {
			return nil, ErrOnlyOwnerCanReturn.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
		}
		// Verify booking is in ACCEPTED state
		if booking.BookingStatus != db.BookingStatusAccepted {
			return nil, ErrInvalidBookingStatus.WithErr(fmt.Errorf("booking must be in ACCEPTED state to be returned"))
		}

		// Get the tool to check if it is not nomadic
		toolID, err := strconv.ParseInt(booking.ToolID, 10, 64)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("invalid tool ID: %s", booking.ToolID))
		}

		tool, err := a.database.ToolService.GetToolByID(r.Context.Request.Context(), toolID)
		if err != nil {
			return nil, ErrInternalServerError.WithErr(err)
		}
		if tool == nil {
			return nil, ErrToolNotFound.WithErr(fmt.Errorf("tool with id %d not found", toolID))
		}
		// Check if the tool is nomadic
		if tool.IsNomadic {
			return nil, ErrToolNomadic
		}
	case "PICKED":
		newStatus = db.BookingStatusPicked

		// Get the tool to check if it's nomadic and who is the actual user
		toolID, err := strconv.ParseInt(booking.ToolID, 10, 64)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("invalid tool ID: %s", booking.ToolID))
		}

		tool, err := a.database.ToolService.GetToolByID(r.Context.Request.Context(), toolID)
		if err != nil {
			return nil, ErrInternalServerError.WithErr(err)
		}
		if tool == nil {
			return nil, ErrToolNotFound.WithErr(fmt.Errorf("tool with id %d not found", toolID))
		}

		// Check if the tool is nomadic
		if !tool.IsNomadic {
			return nil, ErrToolNotNomadic
		}

		// Check if the user is authorized to mark the booking as picked
		// For nomadic tools with an actual user set, the actual user should mark as picked
		// For nomadic tools without an actual user, the owner should mark as picked
		isAuthorized := false
		if !tool.ActualUserID.IsZero() {
			// For nomadic tools with an actual user, the actual user should mark as picked
			isAuthorized = tool.ActualUserID == user.ObjectID()
		} else {
			// For nomadic tools without an actual user, the owner should mark as picked
			isAuthorized = booking.ToUserID == user.ObjectID()
		}

		if !isAuthorized {
			if !tool.ActualUserID.IsZero() {
				return nil, ErrOnlyOwnerCanReturn.WithErr(fmt.Errorf("user %s is not the actual user of this nomadic tool", user.ID))
			}
			return nil, ErrOnlyOwnerCanReturn.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
		}

		// Verify booking is in ACCEPTED state
		if booking.BookingStatus != db.BookingStatusAccepted {
			return nil, ErrInvalidBookingStatus.WithErr(fmt.Errorf("booking status is %s, must be ACCEPTED", booking.BookingStatus))
		}

		// Get the renter user to update the tool location
		renter, err := a.getUserByID(booking.FromUserID.Hex())
		if err != nil {
			return nil, ErrUserNotFound.WithErr(err)
		}

		// Update the tool's location and actualUserId
		updates := map[string]interface{}{
			"location":     renter.Location.ToDBLocation(),
			"actualUserId": booking.FromUserID,
		}

		err = a.database.ToolService.UpdateToolFields(r.Context.Request.Context(), toolID, updates)
		if err != nil {
			return nil, ErrInternalServerError.WithErr(err)
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

	return a.convertBookingToResponse(updatedBooking, r.UserID), nil
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
		response[i] = a.convertBookingToResponse(booking, r.UserID)
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

	// Check if there are any actual ratings
	if (unifiedRating.Owner == nil || unifiedRating.Owner.Rating == nil) &&
		(unifiedRating.Requester == nil || unifiedRating.Requester.Rating == nil) {
		// No ratings found, return 204 No Content
		return nil, ErrNoContent.WithErr(fmt.Errorf("no ratings yet"))
	}

	// Return the unified rating directly
	return unifiedRating, nil
}

// RateRequest represents the request body for rating a booking
type RateRequest struct {
	Rating  int              `json:"rating"`
	Comment string           `json:"comment"`
	Images  []types.HexBytes `json:"images,omitempty"`
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

	// Determine the recipient of the booking
	var toUserID primitive.ObjectID
	if tool.IsNomadic && !tool.ActualUserID.IsZero() {
		// For nomadic tools with an actual user, send the booking to the actual user
		toUserID = tool.ActualUserID
	} else {
		// For non-nomadic tools or nomadic tools without an actual user, send the booking to the owner
		toUserID = tool.UserID
	}

	// Get the recipient user
	toUser, err := a.database.UserService.GetUserByID(r.Context.Request.Context(), toUserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(fmt.Errorf("recipient user not found: %w", err))
	}

	// Check if the tool belongs to any communities
	if len(tool.Communities) > 0 {
		// Check if the user is a member of any of the tool's communities
		userIsMember := false
		for _, toolCommunityID := range tool.Communities {
			toolCommunityIDStr := toolCommunityID.Hex()
			for _, userCommunity := range fromUser.Communities {
				if toolCommunityIDStr == userCommunity.ID {
					userIsMember = true
					break
				}
			}
			if userIsMember {
				break
			}
		}

		if !userIsMember {
			return nil, ErrUserNotCommunityMember.WithErr(
				fmt.Errorf("user is not a member of any community this tool belongs to"),
			)
		}
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

	// Check if the tool has a maximum distance restriction
	if tool.MaxDistance > 0 {
		// Convert API Location to DB Location
		userDBLocation := fromUser.Location.ToDBLocation()

		// Check if the distance between the user and the tool is within the maximum distance
		withinDistance := db.WithinCircumference(
			userDBLocation,
			tool.Location,
			int(tool.MaxDistance*1000), // Convert km to meters
		)

		if !withinDistance {
			return nil, ErrToolLocationTooFar.WithErr(
				fmt.Errorf("tool is too far away (max distance: %d km)", tool.MaxDistance),
			)
		}
	}

	// Create booking request
	dbReq := &db.CreateBookingRequest{
		ToolID:    fmt.Sprintf("%d", toolID),
		StartDate: startDate,
		EndDate:   endDate,
		Contact:   req.Contact,
		Comments:  req.Comments,
		IsNomadic: tool.IsNomadic,
	}
	booking, err := a.database.BookingService.Create(r.Context.Request.Context(), dbReq, fromUser.ObjectID(), toUser.ID)
	if err != nil {
		if err.Error() == "booking dates conflict with existing booking" {
			return nil, ErrBookingDatesConflict.WithErr(err)
		}
		return nil, ErrInternalServerError.WithErr(err)
	}

	return a.convertBookingToResponse(booking, r.UserID), nil
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

	// Validate image hashes if provided
	images, err := a.imageListFromSlice(rateReq.Images)
	if err != nil {
		return 0, err
	}
	dbImages := []db.Image{}
	for _, i := range images {
		dbImages = append(dbImages, db.Image{
			Hash: i.Hash,
			Name: i.Name,
		})
	}

	// Rate the booking
	err = a.database.BookingService.RateBooking(
		r.Context.Request.Context(),
		bookingID,
		user.ObjectID(),
		rateReq.Rating,
		rateReq.Comment,
		dbImages,
	)
	if err != nil {
		switch {
		case err == db.ErrBookingNotFound:
			return nil, ErrBookingNotFound.WithErr(err)
		case err.Error() == "booking must be in RETURNED or PICKED state to be rated":
			return nil, ErrInvalidBookingStatus.WithErr(err)
		case err.Error() == "user has already rated this booking":
			return nil, ErrAlreadyRated.WithErr(err)
		case err.Error() == "user is not involved in this booking":
			return nil, ErrUserNotInvolved.WithErr(err)
		default:
			return nil, ErrInternalServerError.WithErr(err)
		}
	}

	// Get the updated booking with the isRated flag set
	updatedBooking, err := a.database.BookingService.Get(r.Context.Request.Context(), bookingID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return a.convertBookingToResponse(updatedBooking, r.UserID), nil
}
