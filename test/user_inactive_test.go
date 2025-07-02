package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestInactiveUserAccessControl(t *testing.T) {
	c := utils.NewTestService(t)

	// Create active user
	activeUserJWT := c.RegisterAndLogin("active@test.com", "Active User", "password")

	// Create inactive user and then deactivate them
	inactiveUserJWT := c.RegisterAndLogin("inactive@test.com", "Inactive User", "password")

	// Deactivate the user by updating their profile
	_, code := c.Request(http.MethodPost, inactiveUserJWT,
		api.UserProfile{
			Active: &[]bool{false}[0], // Set active to false
		},
		"profile",
	)
	qt.Assert(t, code, qt.Equals, 200)

	// Get user IDs for testing
	var activeUserID, inactiveUserID string
	{
		resp, code := c.Request(http.MethodGet, activeUserJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)
		activeUserID = profileResp.Data.ID
	}
	{
		resp, code := c.Request(http.MethodGet, inactiveUserJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)
		inactiveUserID = profileResp.Data.ID
	}

	t.Run("GetUsers excludes inactive users", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, activeUserJWT, nil, "users")
		qt.Assert(t, code, qt.Equals, 200)

		var usersResp struct {
			Data struct {
				Users []*api.User `json:"users"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &usersResp)
		qt.Assert(t, err, qt.IsNil)

		// Should only return active user (inactive user should be hidden)
		qt.Assert(t, len(usersResp.Data.Users), qt.Equals, 1)
		qt.Assert(t, usersResp.Data.Users[0].ID, qt.Equals, activeUserID)
		qt.Assert(t, usersResp.Data.Users[0].Active, qt.Equals, true)
	})

	t.Run("GetUser allows access to active user", func(t *testing.T) {
		// Any user should be able to access active user
		resp, code := c.Request(http.MethodGet, inactiveUserJWT, nil, "users", activeUserID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, userResp.Data.ID, qt.Equals, activeUserID)
		qt.Assert(t, userResp.Data.Active, qt.Equals, true)
	})

	t.Run("GetUser denies access to inactive user by different user", func(t *testing.T) {
		// Active user should not be able to access inactive user
		_, code := c.Request(http.MethodGet, activeUserJWT, nil, "users", inactiveUserID)
		qt.Assert(t, code, qt.Equals, 404) // Should return 404 (user not found)
	})

	t.Run("GetUser allows access to inactive user by same user", func(t *testing.T) {
		// Inactive user should be able to access their own profile
		resp, code := c.Request(http.MethodGet, inactiveUserJWT, nil, "users", inactiveUserID)
		qt.Assert(t, code, qt.Equals, 200)

		var userResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &userResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, userResp.Data.ID, qt.Equals, inactiveUserID)
		qt.Assert(t, userResp.Data.Active, qt.Equals, false)
	})
}

func TestInactiveUserToolsAccessControl(t *testing.T) {
	c := utils.NewTestService(t)

	// Create active user and tool
	activeUserJWT := c.RegisterAndLogin("active@test.com", "Active User", "password")
	activeToolID := c.CreateTool(activeUserJWT, "Active User Tool")

	// Create inactive user and tool, then deactivate user
	inactiveUserJWT := c.RegisterAndLogin("inactive@test.com", "Inactive User", "password")
	inactiveToolID := c.CreateTool(inactiveUserJWT, "Inactive User Tool")

	// Deactivate the user
	_, code := c.Request(http.MethodPost, inactiveUserJWT,
		api.UserProfile{
			Active: &[]bool{false}[0], // Set active to false
		},
		"profile",
	)
	qt.Assert(t, code, qt.Equals, 200)

	// Get user IDs for testing
	var activeUserID, inactiveUserID string
	{
		resp, code := c.Request(http.MethodGet, activeUserJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)
		activeUserID = profileResp.Data.ID
	}
	{
		resp, code := c.Request(http.MethodGet, inactiveUserJWT, nil, "profile")
		qt.Assert(t, code, qt.Equals, 200)
		var profileResp struct {
			Data *api.User `json:"data"`
		}
		err := json.Unmarshal(resp, &profileResp)
		qt.Assert(t, err, qt.IsNil)
		inactiveUserID = profileResp.Data.ID
	}

	t.Run("GetUserTools allows access to active user tools", func(t *testing.T) {
		// Any user should be able to access tools from active user
		resp, code := c.Request(http.MethodGet, inactiveUserJWT, nil, "tools", "user", activeUserID)
		qt.Assert(t, code, qt.Equals, 200)

		var toolsResp struct {
			Data struct {
				Tools []*api.Tool `json:"tools"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(toolsResp.Data.Tools), qt.Equals, 1)
		qt.Assert(t, toolsResp.Data.Tools[0].ID, qt.Equals, activeToolID)
	})

	t.Run("GetUserTools denies access to inactive user tools by different user", func(t *testing.T) {
		// Active user should not be able to access tools from inactive user
		resp, code := c.Request(http.MethodGet, activeUserJWT, nil, "tools", "user", inactiveUserID)
		qt.Assert(t, code, qt.Equals, 200)

		var toolsResp struct {
			Data struct {
				Tools []*api.Tool `json:"tools"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(toolsResp.Data.Tools), qt.Equals, 0) // Should return empty list
	})

	t.Run("GetUserTools allows access to inactive user tools by same user", func(t *testing.T) {
		// Inactive user should be able to access their own tools
		resp, code := c.Request(http.MethodGet, inactiveUserJWT, nil, "tools", "user", inactiveUserID)
		qt.Assert(t, code, qt.Equals, 200)

		var toolsResp struct {
			Data struct {
				Tools []*api.Tool `json:"tools"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(toolsResp.Data.Tools), qt.Equals, 1)
		qt.Assert(t, toolsResp.Data.Tools[0].ID, qt.Equals, inactiveToolID)
	})

	t.Run("SearchTools excludes tools from inactive users", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, activeUserJWT, nil, "tools", "search")
		qt.Assert(t, code, qt.Equals, 200)

		var toolsResp struct {
			Data struct {
				Tools []*api.Tool `json:"tools"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)

		// Should only return tool from active user
		qt.Assert(t, len(toolsResp.Data.Tools), qt.Equals, 1)
		qt.Assert(t, toolsResp.Data.Tools[0].ID, qt.Equals, activeToolID)
	})

	t.Run("SearchTools includes own tools for inactive user", func(t *testing.T) {
		resp, code := c.Request(http.MethodGet, inactiveUserJWT, nil, "tools", "search")
		qt.Assert(t, code, qt.Equals, 200)

		var toolsResp struct {
			Data struct {
				Tools []*api.Tool `json:"tools"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)

		// Should return both tools (active user's tool + own tool)
		qt.Assert(t, len(toolsResp.Data.Tools), qt.Equals, 2)

		// Verify we have both tools
		toolIDs := make(map[int64]bool)
		for _, tool := range toolsResp.Data.Tools {
			toolIDs[tool.ID] = true
		}
		qt.Assert(t, toolIDs[activeToolID], qt.Equals, true)
		qt.Assert(t, toolIDs[inactiveToolID], qt.Equals, true)
	})
}

func TestInactiveUserToolAccess(t *testing.T) {
	// Setup test environment
	s := utils.NewTestService(t)

	// Create test users
	activeToken, _ := s.RegisterAndLoginWithID("active@test.com", "ActiveUser", "password")
	inactiveToken, inactiveUserID := s.RegisterAndLoginWithID("inactive@test.com", "InactiveUser", "password")
	otherToken, _ := s.RegisterAndLoginWithID("other@test.com", "OtherUser", "password")

	// Create test tools
	activeToolID := s.CreateTool(activeToken, "Active User Tool")
	inactiveToolID := s.CreateTool(inactiveToken, "Inactive User Tool")

	t.Run("GET /tools/{id} - Active user tool accessible to everyone", func(t *testing.T) {
		// Test with active user (owner)
		_, code := s.Request(http.MethodGet, activeToken, nil, "tools", fmt.Sprintf("%d", activeToolID))
		qt.Assert(t, code, qt.Equals, 200)

		// Test with other user
		_, code = s.Request(http.MethodGet, otherToken, nil, "tools", fmt.Sprintf("%d", activeToolID))
		qt.Assert(t, code, qt.Equals, 200)
	})

	t.Run("GET /tools/{id} - Inactive user tool access", func(t *testing.T) {
		// First, deactivate the user by updating their profile
		_, code := s.Request(http.MethodPost, inactiveToken, map[string]interface{}{
			"active": false,
		}, "profile")
		qt.Assert(t, code, qt.Equals, 200)

		// Test with inactive user (owner) - should still work
		_, code = s.Request(http.MethodGet, inactiveToken, nil, "tools", fmt.Sprintf("%d", inactiveToolID))
		qt.Assert(t, code, qt.Equals, 200)

		// Test with other user - should fail (not found)
		_, code = s.Request(http.MethodGet, otherToken, nil, "tools", fmt.Sprintf("%d", inactiveToolID))
		qt.Assert(t, code, qt.Equals, 404)
	})

	t.Run("GET /tools/{id}/ratings - Inactive user tool ratings access", func(t *testing.T) {
		// Test with inactive user (owner) - should work
		_, code := s.Request(http.MethodGet, inactiveToken, nil, "tools", fmt.Sprintf("%d/ratings", inactiveToolID))
		qt.Assert(t, code, qt.Equals, 200)

		// Test with other user - should return empty results
		_, code = s.Request(http.MethodGet, otherToken, nil, "tools", fmt.Sprintf("%d/ratings", inactiveToolID))
		qt.Assert(t, code, qt.Equals, 404)
	})

	t.Run("GET /tools/user/{id} - Inactive user tools access", func(t *testing.T) {
		// Test with inactive user (owner) - should work
		_, code := s.Request(http.MethodGet, inactiveToken, nil, "tools", "user", inactiveUserID)
		qt.Assert(t, code, qt.Equals, 200)

		// Test with other user - should return empty results
		_, code = s.Request(http.MethodGet, otherToken, nil, "tools", "user", inactiveUserID)
		qt.Assert(t, code, qt.Equals, 404)
	})

	t.Run("GET /tools/search - Should not return tools from inactive users", func(t *testing.T) {
		// Test with other user - should only see active user's tools
		resp, code := s.Request(http.MethodGet, otherToken, nil, "tools", "search")
		qt.Assert(t, code, qt.Equals, 200)

		var toolsResp struct {
			Data api.PaginatedToolsResponse `json:"data"`
		}
		err := json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)

		// Should only contain tools from active users (not the inactive user's tool)
		for _, tool := range toolsResp.Data.Tools {
			qt.Assert(t, tool.UserID, qt.Not(qt.Equals), inactiveUserID)
		}

		// Test with inactive user - should NOT see own tools plus active user tools
		resp, code = s.Request(http.MethodGet, inactiveToken, nil, "tools", "search")
		qt.Assert(t, code, qt.Equals, 200)

		err = json.Unmarshal(resp, &toolsResp)
		qt.Assert(t, err, qt.IsNil)

		// Should NOT contain the inactive user's own tools
		foundOwnTool := false
		for _, tool := range toolsResp.Data.Tools {
			if tool.UserID == inactiveUserID {
				foundOwnTool = true
				break
			}
		}
		qt.Assert(t, foundOwnTool, qt.IsFalse)
	})
}
