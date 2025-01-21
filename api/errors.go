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
func (e *HTTPError) WithErr(err error) *HTTPError {
	e.Message += ": " + err.Error()
	return e
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
		Message: "invalid email or password",
	}
)

// Request validation errors
var (
	ErrInvalidRequestBodyData = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid request body data",
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
		Message: "invalid rating value (must be between 1 and 5)",
	}
)

// Resource not found errors
var (
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
	ErrUserNotFound = &HTTPError{
		Code:    http.StatusNotFound,
		Message: "user not found",
	}
	ErrInvalidUserID = &HTTPError{
		Code:    http.StatusBadRequest,
		Message: "invalid user id format",
	}
)

// Permission errors
var (
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
)

// Conflict errors
var (
	ErrBookingDatesConflict = &HTTPError{
		Code:    http.StatusConflict,
		Message: "booking dates conflict with existing booking",
	}
	ErrBookingAlreadyReturned = &HTTPError{
		Code:    http.StatusConflict,
		Message: "booking already marked as returned",
	}
	ErrBookingAlreadyRated = &HTTPError{
		Code:    http.StatusConflict,
		Message: "booking already rated",
	}
	ErrCanOnlyAcceptPending = &HTTPError{
		Code:    http.StatusConflict,
		Message: "can only accept pending petitions",
	}
	ErrCanOnlyDenyPending = &HTTPError{
		Code:    http.StatusConflict,
		Message: "can only deny pending petitions",
	}
	ErrCanOnlyCancelPending = &HTTPError{
		Code:    http.StatusConflict,
		Message: "can only cancel pending requests",
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
)

// Tool validation errors
var (
	ErrEmptyTitleOrDescription = &HTTPError{
		Code:    http.StatusUnprocessableEntity,
		Message: "title and description must not be empty",
	}
	ErrInvalidEstimatedValue = &HTTPError{
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
)
