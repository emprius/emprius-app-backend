package api

import (
	"net/http"
	"strings"
)

// HTTPError represents an error with an HTTP status code
type HTTPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *HTTPError) Error() string {
	return e.Message
}

// IsErr checks if the HTTPError is the same as the given error.
// It compares the error code and the base error message, without taking into account the additional error details
// introduced by WithErr.
func (e *HTTPError) IsErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Split(e.Error(), ":")[0] == strings.Split(err.Error(), ":")[0]
}

// WithErr appends an error message to the HTTPError message.
// Returns a copy of the HTTPError with the appended error message.
func (e *HTTPError) WithErr(err error) *HTTPError {
	return &HTTPError{
		Code:    e.Code,
		Message: e.Message + ": " + err.Error(),
	}
}

// Authentication errors
var (
	ErrUnauthorized = &HTTPError{
		Code:    http.StatusUnauthorized,
		Message: "unauthorized access",
	}
	ErrInvalidRegisterAuthToken = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid registration token",
	}
	ErrWrongLogin = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid credentials",
	}
)

// Request validation errors
var (
	ErrInvalidRequestBodyData = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid request body data",
	}
	ErrActualPasswordRequired = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "actual password is required",
	}
	ErrInvalidActualPassword = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "invalid actual password",
	}
	ErrInvalidJSON = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid JSON body",
	}
	ErrInvalidImageFormat = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid image format",
	}
	ErrInvalidHash = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid hash",
	}
	ErrInvalidBookingDates = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid booking dates",
	}
	ErrInvalidRating = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid rating value",
	}

	ErrInvalidBookingStatus = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid booking status",
	}

	ErrAlreadyRated = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "booking already rated",
	}
)

// Resource not found or empty errors
var (
	ErrNoContent = &HTTPError{
		Code:    http.StatusNoContent,
		Message: "no content",
	}
	ErrImageNotFound = &HTTPError{
		Code:    http.StatusNotFound,
		Message: "image not found",
	}
	ErrToolNotFound = &HTTPError{
		Code:    http.StatusNotFound,
		Message: "tool not found",
	}
	ErrBookingNotFound = &HTTPError{
		Code:    http.StatusNotFound,
		Message: "booking not found",
	}
	ErrRatingNotFound = &HTTPError{
		Code:    http.StatusNotFound,
		Message: "rating not found",
	}
	ErrUserNotFound = &HTTPError{
		Code:    http.StatusNotFound,
		Message: "user not found",
	}
	ErrCommunityNotFound = &HTTPError{
		Code:    http.StatusNotFound,
		Message: "community not found",
	}
	ErrInvalidUserID = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid user id format",
	}
)

// Permission errors
var (
	ErrUserNotCommunityMember = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "user is not a member of the community this tool belongs to",
	}
	ErrToolNotOwnedByUser = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "tool not owned by user",
	}
	ErrOnlyOwnerCanReturn = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "only tool owner can mark as returned",
	}
	ErrOnlyOwnerCanAccept = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "only tool owner can accept petitions",
	}
	ErrOnlyOwnerCanDeny = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "only tool owner can deny petitions",
	}
	ErrOnlyRequesterCanCancel = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "only requester can cancel their requests",
	}
	ErrUserNotInvolved = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "user not involved in booking",
	}
	ErrUserInactive = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "user account is inactive",
	}
	ErrRecipientUserInactive = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "recipient user account is inactive",
	}
)

// Conflict errors
var (
	ErrEmailChangeNotAllowed = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "email change not allowed",
	}
	ErrBookingDatesConflict = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "booking dates conflict with existing booking",
	}
	ErrBookingAlreadyReturned = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "booking already marked as returned",
	}
	ErrBookingAlreadyRated = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "booking already rated",
	}
	ErrCanOnlyAcceptPending = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "can only accept pending petitions",
	}
	ErrCanOnlyDenyPending = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "can only deny pending petitions",
	}
	ErrCanOnlyCancelPending = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "can only cancel pending requests",
	}
	ErrPasswordTooShort = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "password must be at least 8 characters long",
	}
	ErrMalformedEmail = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "malformed email address",
	}
	ErrLocationNotSet = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "Location is not set",
	}
	ErrInvalidInviteCode = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid invite code",
	}
	ErrInviteCodeAlreadyUsed = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invite code already used",
	}
	ErrHasUnusedInviteCodes = &HTTPError{
		Code:    http.StatusConflict,
		Message: "you still have unused invite codes",
	}
	ErrTooManyInviteCodeRequests = &HTTPError{
		Code:    http.StatusConflict,
		Message: "too many invite code requests, please try again later",
	}
	ErrCanOnlyPickAccepted = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "can only pick accepted bookings",
	}
)

// Server errors
var (
	ErrCouldNotInsertToDatabase = &HTTPError{
		Code:    http.StatusInternalServerError,
		Message: "could not insert to database",
	}
	ErrInternalServerError = &HTTPError{
		Code:    http.StatusInternalServerError,
		Message: "internal server error",
	}
	ErrTooManyRequests = &HTTPError{
		Code:    http.StatusTooManyRequests,
		Message: "too many requests",
	}
)

// Tool validation errors
var (
	ErrEmptyTitleOrDescription = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "title and description must not be empty",
	}
	ErrInvalidToolValuationValue = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "estimated value must be greater than 0",
	}
	ErrMayBeFreeRequired = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "may be free must not be nil",
	}
	ErrAskWithFeeRequired = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "ask with fee must not be nil",
	}
	ErrCostRequired = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "cost must not be nil",
	}
	ErrToolLocationTooFar = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "tool location is too far away",
	}
	ErrInvalidToolCategory = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "invalid tool category",
	}
	ErrInvalidTransportOption = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "invalid transport option",
	}
	ErrToolNotNomadic = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "tool is not nomadic",
	}
	ErrToolNomadic = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "tool is nomadic",
	}
	ErrNomadicToolWithPastBooking = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "nomadic tool cannot be booked when there is a booking planned or in process",
	}
	ErrOnlyOwnerCanChangeNomadicStatus = &HTTPError{
		Code:    http.StatusForbidden,
		Message: "only the owner can change a tool from nomadic to non-nomadic",
	}
	ErrCannotChangeNomadicWithPendingBookings = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "cannot change nomadic status when there are pending bookings",
	}
)
