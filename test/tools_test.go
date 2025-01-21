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

		// Create another tool for search tests
		_, code = c.Request(http.MethodPost, userJWT,
			map[string]interface{}{
				"title":          "Another Tool",
				"description":    "Another tool description",
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
			"tools",
		)
		qt.Assert(t, code, qt.Equals, 200)

		// Get tool by ID
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(toolID))
		qt.Assert(t, code, qt.Equals, 200)
		var getToolResp struct {
			Data api.Tool `json:"data"`
		}
		err = json.Unmarshal(resp, &getToolResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, getToolResp.Data.Title, qt.Equals, "Test Tool")

		// Edit tool
		resp, code = c.Request(http.MethodPut, userJWT,
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

		// Get ID of updated tool
		var updateResp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(resp, &updateResp)
		qt.Assert(t, err, qt.IsNil)
		updatedToolID := updateResp.Data.ID

		// Get updated tool
		resp, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(updatedToolID))
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
		qt.Assert(t, len(listToolsResp.Data.Tools), qt.Equals, 2)

		// Test various search scenarios
		t.Run("Search Tools", func(t *testing.T) {
			// Search by term
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

			// Search with distance and mayBeFree
			resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?distance=10&mayBeFree=false")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 2)

			// Search with term and distance
			resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?searchTerm=Another&distance=10")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 1)

			// Search with maxCost
			resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?maxCost=15")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 1)

			// Search with multiple parameters
			resp, code = c.Request(http.MethodGet, userJWT, nil, "tools/search?searchTerm=&distance=10&maxCost=0&mayBeFree=false")
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 2)

			// Search with non-matching term
			resp, code = c.Request(
				http.MethodGet,
				userJWT,
				nil,
				"tools/search?searchTerm=nonexistent&distance=10&maxCost=0&mayBeFree=false",
			)
			qt.Assert(t, code, qt.Equals, 200)
			err = json.Unmarshal(resp, &searchResp)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(searchResp.Data.Tools), qt.Equals, 0)
		})

		// Delete tool
		_, code = c.Request(http.MethodDelete, userJWT, nil, "tools", fmt.Sprint(updatedToolID))
		qt.Assert(t, code, qt.Equals, 200)

		// Verify tool is deleted
		_, code = c.Request(http.MethodGet, userJWT, nil, "tools", fmt.Sprint(updatedToolID))
		qt.Assert(t, code, qt.Equals, 404)
	})
}
