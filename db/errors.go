package db

// Database-specific errors
const (
	ErrBookingDatesConflict = "booking dates conflict with existing booking"
	ErrBookingNotFound      = "booking not found"
	ErrInvalidBookingDates  = "invalid booking dates"
)
