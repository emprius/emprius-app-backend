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

func TestTools(t *testing.T) {
	c := utils.NewTestService(t)

	// Create a user
	userJWT := c.RegisterAndLogin("user@test.com", "user", "userpass")

	t.Run("Create and Manage Tools", func(t *testing.T) {
		// Try to create tool without auth
		_, code := c.Request(http.MethodPost, "",
			map[string]interface{}{
				"title":          "TestTool",
				"description":    "Test tool description",
				"mayBeFree":      true,
				"askWithFee":     false,
				"cost":           10,
				"category":       1,
				"estimatedValue": 20,
				"height":         30,
				"weight":         40,
				"location": map[string]int64{
					"latitude":  41695384000,
					"longitude": 2492793000,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 401)

		// Create tool with auth
		resp, code := c.Request(http.MethodPost, userJWT,
			map[string]interface{}{
				"title":          "Test Tool",
				"description":    "Test tool description",
				"mayBeFree":      true,
				"askWithFee":     false,
				"cost":           10,
				"category":       1,
				"estimatedValue": 20,
				"height":         30,
				"weight":         40,
				"location": map[string]int64{
					"latitude":  41695384000,
					"longitude": 2492793000,
				},
			},
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		var toolResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err := json.Unmarshal(resp, &toolResp)
		qt.Assert(t, err, qt.IsNil)
		toolID := toolResp.Data.ID

		// Get tool by ID
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200)
		var getToolResp struct {
			Data api.Tool `json:"data"`
		}
		err = json.Unmarshal(resp, &getToolResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, getToolResp.Data.Title, qt.Equals, "TestTool")

		// Edit tool
		_, code = c.Request(http.MethodPut, userJWT,
			map[string]interface{}{
				"title":          "Updated Tool",
				"description":    "Updated description",
				"mayBeFree":      false,
				"askWithFee":     true,
				"cost":           20,
				"category":       1,
				"estimatedValue": 30,
				"height":         40,
				"weight":         50,
				"location": map[string]int64{
					"latitude":  41695384000,
					"longitude": 2492793000,
				},
			},
			"tools", fmt.Sprint(toolID),
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get updated tool
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &getToolResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, getToolResp.Data.Title, qt.Equals, "Updated Tool")

		// List owned tools
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools")
		qt.Assert(t, code, qt.Equals, 200)
		var listToolsResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &listToolsResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(listToolsResp.Data.Tools), qt.Equals, 1)

		// Search tools
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?searchTerm=Updated")
		qt.Assert(t, code, qt.Equals, 200)
		var searchResp struct {
			Data struct {
				Tools []api.Tool `json:"tools"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &searchResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 1)

		// Delete tool
		_, code = c.Request(http.MethodDelete, userJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200)

		// Verify tool is deleted
		_, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 404)
	})
}
