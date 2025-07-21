package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/emprius/emprius-app-backend/notifications"
	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/types"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// validateBookingDates checks if the booking dates conflict with existing bookings or reserved dates
func (a *API) validateBookingDates(
	ctx context.Context,
	toolID string,
	startDate, endDate time.Time,
	excludeBookingID primitive.ObjectID,
) error {
	conflictExists, err := a.database.BookingService.CheckDateConflicts(ctx, toolID, startDate, endDate, excludeBookingID)
	if err != nil {
		return ErrInternalServerError.WithErr(fmt.Errorf("error checking date conflicts: %w", err))
	}
	if conflictExists {
		return ErrBookingDatesConflict.WithErr(fmt.Errorf("booking dates overlap with existing booking or reserved dates"))
	}
	return nil
}

// RegisterBookingRoutes registers all booking-related routes to the provided router group
func (a *API) RegisterBookingRoutes(r chi.Router) {
	// POST /bookings
	log.Info().Msg("register route POST /bookings")
	r.Post("/bookings", a.routerHandler(a.HandleCreateBooking))
	// GET /bookings/requests/outgoing
	log.Info().Msg("register route GET /bookings/requests/outgoing")
	r.Get("/bookings/requests/outgoing", a.routerHandler(a.HandleGetOutgoingRequests))
	// GET /bookings/requests/incoming
	log.Info().Msg("register route GET /bookings/requests/incoming")
	r.Get("/bookings/requests/incoming", a.routerHandler(a.HandleGetIncomingRequests))
	// PUT /bookings/{bookingId}
	log.Info().Msg("register route PUT /bookings/{bookingId}")
	r.Put("/bookings/{bookingId}", a.routerHandler(a.HandleUpdateBookingStatus))
	// GET /bookings/{bookingId}
	log.Info().Msg("register route GET /bookings/{bookingId}")
	r.Get("/bookings/{bookingId}", a.routerHandler(a.HandleGetBooking))
	// GET /bookings/ratings/pending
	log.Info().Msg("register route GET /bookings/ratings/pending")
	r.Get("/bookings/ratings/pending", a.routerHandler(a.HandleGetPendingRatings))
	// POST /bookings/{bookingId}/ratings
	log.Info().Msg("register route POST /bookings/{bookingId}/ratings")
	r.Post("/bookings/{bookingId}/ratings", a.routerHandler(a.HandleRateBooking))
	// GET /bookings/{bookingId}/ratings
	log.Info().Msg("register route GET /bookings/{bookingId}/ratings")
	r.Get("/bookings/{bookingId}/ratings", a.routerHandler(a.HandleGetBookingRatings))
}

// convertBookingToResponse converts a db.Booking into a BookingResponse.
// The IsRated field is set based on whether the authenticated user has rated the booking.
func (a *API) convertBookingToResponse(booking *db.Booking, authenticatedUserID ...string) *BookingResponse {
	isRated := false

	// Check if the booking is rated by the authenticated user
	if len(authenticatedUserID) > 0 && authenticatedUserID[0] != "" &&
		booking.BookingStatus == db.BookingStatusReturned || booking.BookingStatus == db.BookingStatusPicked {
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

	response := &BookingResponse{
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

	// Only include pickup place if the booking was accepted and the authenticated user is involved
	if booking.BookingStatus == db.BookingStatusAccepted ||
		booking.BookingStatus == db.BookingStatusReturned ||
		booking.BookingStatus == db.BookingStatusPicked &&
			booking.PickupPlace != nil &&
			len(authenticatedUserID) > 0 &&
			authenticatedUserID[0] != "" {

		userID, err := primitive.ObjectIDFromHex(authenticatedUserID[0])
		if err == nil {
			// Check if the user is involved in the booking
			isInvolved := booking.FromUserID == userID || booking.ToUserID == userID
			if isInvolved {
				// User is involved in the booking, include pickup place
				pickupPlace := &Location{}
				pickupPlace.FromDBLocation(*booking.PickupPlace)
				response.PickupPlace = pickupPlace
			}
		}
	}

	return response
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

	// Get pagination parameters
	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get paginated bookings
	bookings, total, err := a.database.BookingService.GetUserBookings(
		r.Context.Request.Context(),
		user.ID,
		db.OutgoingBookings,
		page,
		pageSize,
	)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return a.getBookingListPaginatedResponse(bookings, page, pageSize, total, r.UserID), nil
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

	// Get pagination parameters
	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get paginated bookings
	bookings, total, err := a.database.BookingService.GetUserBookings(
		r.Context.Request.Context(),
		user.ID,
		db.IncomingBookings,
		page,
		pageSize,
	)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return a.getBookingListPaginatedResponse(bookings, page, pageSize, total, r.UserID), nil
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

	// Get the renter user to update the tool obfuscatedLocation
	renter, err := a.getUserByID(booking.FromUserID.Hex())
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	// Validate the requested status
	var newStatus db.BookingStatus
	switch statusUpdate.Status {
	case BookingStatusAccepted:
		newStatus = db.BookingStatusAccepted
		// Verify user is the tool owner
		if booking.ToUserID != user.ID {
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
			isAuthorized = tool.ActualUserID == user.ID
		} else {
			// For non-nomadic tools or nomadic tools without an actual user, the owner should accept
			isAuthorized = booking.ToUserID == user.ID
		}

		if !isAuthorized {
			if tool.IsNomadic && !tool.ActualUserID.IsZero() {
				return nil, ErrOnlyOwnerCanAccept.WithErr(fmt.Errorf("user %s is not the actual user of this nomadic tool", user.ID))
			}
			return nil, ErrOnlyOwnerCanAccept.WithErr(fmt.Errorf("user %s is not the owner", user.ID))
		}

		// Validate booking dates against existing bookings and reserved dates before accepting
		if err := a.validateBookingDates(
			r.Context.Request.Context(), booking.ToolID, booking.StartDate, booking.EndDate, booking.ID,
		); err != nil {
			return nil, err
		}

		// Set the pickup place in the booking
		err = a.database.BookingService.SetPickupPlace(r.Context.Request.Context(), bookingID, tool.Location)
		if err != nil {
			return nil, ErrInternalServerError.WithErr(err)
		}

		// Update karma for non-nomadic tools only
		if !tool.IsNomadic {
			// Calculate days between booking dates
			days := int64(booking.EndDate.Sub(booking.StartDate).Hours() / 24)
			if days == 0 {
				days = 1 // Minimum 1 day
			}

			// Owner gains karma (loaning)
			ownerKarmaChange := days
			// Requester loses karma (requesting)
			requesterKarmaChange := -days

			// Update owner's karma
			err = a.database.UserService.UpdateUserKarma(r.Context.Request.Context(), booking.ToUserID, ownerKarmaChange)
			if err != nil {
				log.Error().Err(err).Str("userId",
					booking.ToUserID.Hex()).Int64("karmaChange", ownerKarmaChange).Msg("Failed to update owner karma")
				// Continue even if karma update fails
			} else {
				log.Debug().Str("userId", booking.ToUserID.Hex()).Int64("karmaChange", ownerKarmaChange).Msg("Updated owner karma")
			}

			// Update requester's karma
			err = a.database.UserService.UpdateUserKarma(r.Context.Request.Context(), booking.FromUserID, requesterKarmaChange)
			if err != nil {
				log.Error().Err(err).Str("userId",
					booking.FromUserID.Hex()).Int64("karmaChange", requesterKarmaChange).Msg("Failed to update requester karma")
				// Continue even if karma update fails
			} else {
				log.Debug().Str("userId", booking.FromUserID.Hex()).Int64("karmaChange", requesterKarmaChange).Msg("Updated requester karma")
			}
		}

		// Send the accepted notification to the requester
		if renter.NotificationPreferences[string(types.NotificationBookingAccepted)] {
			if err := a.sendMail(r.Context.Request.Context(), renter.Email, mailtemplates.BookingAcceptedMailNotification,
				struct {
					AppName    string
					AppUrl     string
					LogoURL    string
					ToolName   string
					FromDate   string
					ToDate     string
					ButtonUrl  string
					UserName   string
					UserUrl    string
					UserRating string
				}{
					mailtemplates.AppName,
					mailtemplates.AppUrl,
					mailtemplates.LogoURL,
					tool.Title,
					booking.StartDate.Format("02 Jan 2006"),
					booking.EndDate.Format("02 Jan 2006"),
					mailtemplates.BookingUrl,
					user.Name,
					fmt.Sprintf(mailtemplates.UserUrl, user.ID.Hex()),
					notifications.Stars(user.Rating),
				},
			); err != nil {
				log.Warn().Err(err).Msg("could not send booking accepted notification")
				// Continue even if email cannot be sent
			}
		}
	case BookingStatusRejected:
		newStatus = db.BookingStatusRejected
		// Verify user is the tool owner
		if booking.ToUserID != user.ID {
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
			isAuthorized = tool.ActualUserID == user.ID
		} else {
			// For non-nomadic tools or nomadic tools without an actual user, the owner should deny
			isAuthorized = booking.ToUserID == user.ID
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
		if booking.FromUserID != user.ID {
			return nil, ErrOnlyRequesterCanCancel.WithErr(fmt.Errorf("user %s is not the requester", user.ID))
		}
		// Verify booking is in PENDING state
		if booking.BookingStatus != db.BookingStatusPending {
			return nil, ErrCanOnlyCancelPending.WithErr(fmt.Errorf("booking status is %s", booking.BookingStatus))
		}
	case BookingStatusReturned:
		newStatus = db.BookingStatusReturned
		// Verify user is the tool owner
		if booking.ToUserID != user.ID {
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
	case BookingStatusPicked:
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
			isAuthorized = tool.ActualUserID == user.ID
		} else {
			// For nomadic tools without an actual user, the owner should mark as picked
			isAuthorized = booking.ToUserID == user.ID
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

		// Update the tool's obfuscatedLocation and actualUserId
		updates := map[string]interface{}{
			"obfuscatedLocation": renter.ObfuscatedLocation,
			"location":           renter.Location,
			"actualUserId":       booking.FromUserID,
		}

		err = a.database.ToolService.UpdateToolFields(r.Context.Request.Context(), toolID, updates)
		if err != nil {
			return nil, ErrInternalServerError.WithErr(err)
		}

		// Add a history entry for the tool
		err = a.database.ToolService.AddToolHistoryEntry(
			r.Context.Request.Context(),
			toolID,
			booking.FromUserID,
			renter.ObfuscatedLocation,
			bookingID,
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to add tool history entry")
			// Continue even if history entry fails
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

	// Get pagination parameters
	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get paginated bookings
	bookings, total, err := a.database.BookingService.GetPendingRatings(
		r.Context.Request.Context(),
		user.ID,
		page,
		pageSize,
	)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return a.getBookingListPaginatedResponse(bookings, page, pageSize, total, r.UserID), nil
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
	// For non-nomadic tools or nomadic tools without an actual user, send the booking to the owner
	toUserID = tool.UserID
	if tool.IsNomadic && !tool.ActualUserID.IsZero() {
		// For nomadic tools with an actual user, send the booking to the actual user
		toUserID = tool.ActualUserID
	}

	// Get the recipient user
	toUser, err := a.database.UserService.GetUserByID(r.Context.Request.Context(), toUserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(fmt.Errorf("recipient user not found: %w", err))
	}

	// Check if requesting user is active
	if !fromUser.Active {
		return nil, ErrUserInactive.WithErr(fmt.Errorf("requesting user is inactive"))
	}

	// Check if recipient user is active
	if !toUser.Active {
		return nil, ErrRecipientUserInactive.WithErr(fmt.Errorf("recipient user is inactive"))
	}

	// Check if the tool belongs to any communities
	if len(tool.Communities) > 0 {
		// Check if the user is a member of any of the tool's communities
		userIsMember := false
		for _, toolCommunityID := range tool.Communities {
			toolCommunityIDStr := toolCommunityID.Hex()
			for _, userCommunity := range fromUser.Communities {
				if toolCommunityIDStr == userCommunity.ID.Hex() {
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
		userDBLocation := fromUser.Location

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

	// Check if the tool is nomadic with past bookings
	if tool.IsNomadic && len(tool.ReservedDates) > 0 {
		// Find the last reserved date
		lastReservedDate := time.Time{}
		now := time.Now()

		for _, dateRange := range tool.ReservedDates {
			endDate := time.Unix(int64(dateRange.To), 0)
			if endDate.After(lastReservedDate) {
				lastReservedDate = endDate
			}
		}

		// If the last reserved date is before today, return an error
		if !lastReservedDate.IsZero() && lastReservedDate.After(now) {
			return nil, ErrNomadicToolWithPastBooking.WithErr(
				fmt.Errorf("nomadic tool cannot be booked when there is a booking planned or in process"),
			)
		}
	}

	// Validate booking dates against existing bookings and reserved dates
	if err := a.validateBookingDates(
		r.Context.Request.Context(), req.ToolID, startDate, endDate, primitive.NilObjectID,
	); err != nil {
		return nil, err
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
	booking, err := a.database.BookingService.Create(r.Context.Request.Context(), dbReq, fromUser.ID, toUser.ID)
	if err != nil {
		if err.Error() == "booking dates conflict with existing booking" {
			return nil, ErrBookingDatesConflict.WithErr(err)
		}
		return nil, ErrInternalServerError.WithErr(err)
	}

	if toUser.NotificationPreferences[string(types.NotificationNewIncomingRequest)] {
		// send the new request notification to the recipient
		if err := a.sendMail(r.Context.Request.Context(), toUser.Email, mailtemplates.NewIncomingRequestMailNotification,
			struct {
				AppName      string
				AppUrl       string
				LogoURL      string
				UserName     string
				UserUrl      string
				UserRating   string
				ToolName     string
				FromDate     string
				ToDate       string
				Comment      string
				WayOfContact string
				ButtonUrl    string
			}{
				mailtemplates.AppName,
				mailtemplates.AppUrl,
				mailtemplates.LogoURL,
				fromUser.Name,
				fmt.Sprintf(mailtemplates.UserUrl, fromUser.ID.Hex()),
				notifications.Stars(fromUser.Rating),
				tool.Title,
				startDate.Format("02 Jan 2006"),
				endDate.Format("02 Jan 2006"),
				req.Comments,
				req.Contact,
				mailtemplates.IncomingUrl,
			},
		); err != nil {
			log.Warn().Err(err).Msg("could not send new request notification")
			// Continue even if email cannot be sent
		}
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
		user.ID,
		rateReq.Rating,
		rateReq.Comment,
		dbImages,
	)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrBookingNotFound):
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
