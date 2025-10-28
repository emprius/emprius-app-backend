package digest_scheduler

import "time"

// TimeProvider interface allows for time mocking in tests
type TimeProvider interface {
	Now() time.Time
}

// RealTimeProvider provides actual system time
type RealTimeProvider struct{}

// Now returns the current system time
func (RealTimeProvider) Now() time.Time {
	return time.Now()
}
