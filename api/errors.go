package api

import (
	"fmt"
)

// HTTPError represents an error with an HTTP status code
type HTTPError struct {
	Code    int
	Message string
}

func (e *HTTPError) Error() string {
	return e.Message
}

var (
	ErrInvalidRegisterAuthToken = fmt.Errorf("invalid register auth token")
	ErrInvalidRequestBodyData   = fmt.Errorf("invalid request body data")
	ErrCouldNotInsertToDatabase = fmt.Errorf("could not insert to database")
	ErrWrongLogin               = fmt.Errorf("wrong password or email")
	ErrInvalidHash              = fmt.Errorf("invalid hash")
	ErrImageNotFound            = fmt.Errorf("image not found")
	ErrInvalidImageFormat       = fmt.Errorf("invalid image format")
	ErrInvalidJSON              = fmt.Errorf("invalid json body")
	ErrBookingDatesConflict     = fmt.Errorf("booking dates conflict with existing booking")
	ErrToolNotFound             = fmt.Errorf("tool not found")
	ErrUnauthorizedBooking      = fmt.Errorf("unauthorized booking operation")
	ErrInvalidBookingDates      = fmt.Errorf("invalid booking dates")
	ErrBookingNotFound          = fmt.Errorf("booking not found")
	ErrOnlyOwnerCanReturn       = fmt.Errorf("only tool owner can mark as returned")
	ErrBookingAlreadyReturned   = fmt.Errorf("booking already marked as returned")
	ErrInvalidRating            = fmt.Errorf("invalid rating value")
	ErrBookingAlreadyRated      = fmt.Errorf("booking already rated")
)
