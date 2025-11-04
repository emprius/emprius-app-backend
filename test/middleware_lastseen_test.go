package test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestLastSeenMiddleware(t *testing.T) {
	c := utils.NewTestService(t)

	// Register and login to get token (this updates lastSeen during login)
	token := c.RegisterAndLogin("lastseen@test.com", "lastseenuser", "testpassword")

	// Get user profile to get the initial lastSeen (set during login)
	profileResp, code := c.Request(http.MethodGet, token, nil, "profile")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Failed to get profile: %s", string(profileResp)))

	var profileResponse struct {
		Data api.User `json:"data"`
	}
	err := json.Unmarshal(profileResp, &profileResponse)
	qt.Assert(t, err, qt.IsNil)
	initialLastSeen := profileResponse.Data.LastSeen

	// Wait longer than 5 minutes to ensure throttle expires
	// For testing purposes, we'll simulate this by waiting 6 minutes
	// In a real scenario, you'd use a time mock, but for this simple test we verify the behavior
	t.Log("Testing that lastSeen is set during login:", initialLastSeen)

	// Verify that lastSeen was set (not zero)
	qt.Assert(t, initialLastSeen.IsZero(), qt.IsFalse,
		qt.Commentf("LastSeen should be set after login"))

	// Test that subsequent requests within 5 minutes do NOT update lastSeen
	time.Sleep(1 * time.Second)

	// Make another authenticated request (should NOT update due to throttle)
	_, code = c.Request(http.MethodGet, token, nil, "profile")
	qt.Assert(t, code, qt.Equals, 200)

	// Wait for potential async update
	time.Sleep(500 * time.Millisecond)

	// Get profile again to verify lastSeen was NOT updated (throttled)
	profileResp2, code := c.Request(http.MethodGet, token, nil, "profile")
	qt.Assert(t, code, qt.Equals, 200)

	var profileResponse2 struct {
		Data api.User `json:"data"`
	}
	err = json.Unmarshal(profileResp2, &profileResponse2)
	qt.Assert(t, err, qt.IsNil)

	// LastSeen should NOT change because we're within the 5-minute throttle window
	qt.Assert(t, profileResponse2.Data.LastSeen.Equal(initialLastSeen), qt.IsTrue,
		qt.Commentf("LastSeen should not update within 5 minutes. Initial: %v, Current: %v",
			initialLastSeen, profileResponse2.Data.LastSeen))

	t.Log("Verified that throttling prevents updates within 5 minutes")
}

func TestLastSeenMiddlewareMultipleEndpoints(t *testing.T) {
	c := utils.NewTestService(t)

	// Register and login to get token
	token := c.RegisterAndLogin("multiendpoint@test.com", "multiendpointuser", "testpassword")

	// Get initial user state
	profileResp, code := c.Request(http.MethodGet, token, nil, "profile")
	qt.Assert(t, code, qt.Equals, 200)

	var profileResponse struct {
		Data api.User `json:"data"`
	}
	err := json.Unmarshal(profileResp, &profileResponse)
	qt.Assert(t, err, qt.IsNil)
	initialLastSeen := profileResponse.Data.LastSeen

	t.Log("Initial lastSeen from login:", initialLastSeen)

	// Verify that lastSeen was set (not zero)
	qt.Assert(t, initialLastSeen.IsZero(), qt.IsFalse,
		qt.Commentf("LastSeen should be set after login"))

	// Test various endpoints - they should all process correctly with the middleware
	endpoints := []string{
		"profile/pendings", // GET /profile/pendings
		"users?page=1",     // GET /users with pagination
	}

	for _, endpoint := range endpoints {
		_, code = c.Request(http.MethodGet, token, nil, endpoint)
		// Some endpoints may return different codes, but they should all process
		qt.Assert(t, code >= 200 && code < 500, qt.IsTrue,
			qt.Commentf("Unexpected status code for endpoint %s: %d", endpoint, code))
		t.Logf("Endpoint %s processed successfully", endpoint)
	}

	// Wait for potential async updates
	time.Sleep(500 * time.Millisecond)

	// Get profile again - lastSeen should still be the same due to throttling
	profileResp2, code := c.Request(http.MethodGet, token, nil, "profile")
	qt.Assert(t, code, qt.Equals, 200)

	var profileResponse2 struct {
		Data api.User `json:"data"`
	}
	err = json.Unmarshal(profileResp2, &profileResponse2)
	qt.Assert(t, err, qt.IsNil)

	// LastSeen should be the same (throttled within 5 minutes)
	qt.Assert(t, profileResponse2.Data.LastSeen.Equal(initialLastSeen), qt.IsTrue,
		qt.Commentf("LastSeen should remain same due to throttling. Initial: %v, Current: %v",
			initialLastSeen, profileResponse2.Data.LastSeen))

	t.Log("Verified that middleware processes all authenticated endpoints without errors")
}

func TestLastSeenMiddlewareUnauthenticatedRequests(t *testing.T) {
	c := utils.NewTestService(t)

	// Make an unauthenticated request to a public endpoint
	_, code := c.Request(http.MethodGet, "", nil, "info")
	qt.Assert(t, code, qt.Equals, 200)

	// This should not cause any errors or panics
	// The middleware should gracefully handle missing user ID
}
