package utils

import (
	"sync"
	"time"
)

// MockTimeProvider provides a mockable time for testing
type MockTimeProvider struct {
	currentTime time.Time
	mu          sync.RWMutex
}

// Now returns the current mocked time
func (m *MockTimeProvider) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentTime
}

// AdvanceTime advances the mock time by the given duration
func (m *MockTimeProvider) AdvanceTime(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentTime = m.currentTime.Add(d)
}

// SetTime sets the mock time to a specific time
func (m *MockTimeProvider) SetTime(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentTime = t
}
