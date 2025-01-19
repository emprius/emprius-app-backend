package db

import "errors"

// Database-specific errors
var (
	ErrBookingDatesConflict = errors.New("booking dates conflict with existing booking")
	ErrBookingNotFound      = errors.New("booking not found")
	ErrInvalidBookingDates  = errors.New("invalid booking dates")
)
