package api

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/types"
	"github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGetCommunityToolsHandler(t *testing.T) {
	c := quicktest.New(t)
	api := testAPI(t)

	// Create test user
	testUser := db.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: []byte("hashedpassword"),
		Active:   true,
		Location: db.NewLocation(40000000, -74000000), // New York coordinates
	}
	userID, err := api.addUser(&testUser)
	c.Assert(err, quicktest.IsNil)

	// Create test community
	community, err := api.database.CommunityService.CreateCommunity(
		context.Background(),
		"Test Community",
		types.HexBytes{},
		userID,
	)
	c.Assert(err, quicktest.IsNil)

	// Create test tools
	tools := []*db.Tool{
		{
			ID:          1001,
			UserID:      userID,
			Title:       "Hammer",
			Description: "A useful hammer for construction",
			IsAvailable: true,
			MayBeFree:   true,
			Cost:        100,
			Location:    db.NewLocation(40000000, -74000000),
			Communities: []primitive.ObjectID{community.ID},
		},
		{
			ID:          1002,
			UserID:      userID,
			Title:       "Drill",
			Description: "Electric drill for drilling holes",
			IsAvailable: true,
			MayBeFree:   false,
			Cost:        200,
			Location:    db.NewLocation(40000000, -74000000),
			Communities: []primitive.ObjectID{community.ID},
		},
		{
			ID:          1003,
			UserID:      userID,
			Title:       "Saw",
			Description: "Hand saw for cutting wood",
			IsAvailable: true,
			MayBeFree:   true,
			Cost:        150,
			Location:    db.NewLocation(40000000, -74000000),
			Communities: []primitive.ObjectID{community.ID},
		},
	}

	// Insert tools
	for _, tool := range tools {
		_, err := api.database.ToolService.InsertTool(context.Background(), tool)
		c.Assert(err, quicktest.IsNil)
	}

	t.Run("successful pagination without search", func(t *testing.T) {
		// Create request
		req := httptest.NewRequest("GET", fmt.Sprintf("/communities/%s/tools?page=0&pageSize=2", community.ID.Hex()), nil)

		// Create chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("communityId", community.ID.Hex())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Create API request
		apiReq := &Request{
			UserID:  userID.Hex(),
			Context: &HTTPContext{Request: req},
		}

		// Call handler
		result, err := api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.IsNil)

		// Verify response
		response, ok := result.(*PaginatedToolsResponse)
		c.Assert(ok, quicktest.IsTrue)
		c.Assert(len(response.Tools), quicktest.Equals, 2)
		c.Assert(response.Pagination.Current, quicktest.Equals, 0)
		c.Assert(response.Pagination.PageSize, quicktest.Equals, 2)
		c.Assert(response.Pagination.Total, quicktest.Equals, int64(3))
		c.Assert(response.Pagination.Pages, quicktest.Equals, 2)
	})

	t.Run("successful pagination with search", func(t *testing.T) {
		// Create request with search term
		req := httptest.NewRequest("GET", fmt.Sprintf("/communities/%s/tools?page=0&pageSize=10&term=drill", community.ID.Hex()), nil)

		// Create chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("communityId", community.ID.Hex())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Create API request
		apiReq := &Request{
			UserID:  userID.Hex(),
			Context: &HTTPContext{Request: req},
		}

		// Call handler
		result, err := api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.IsNil)

		// Verify response
		response, ok := result.(*PaginatedToolsResponse)
		c.Assert(ok, quicktest.IsTrue)
		c.Assert(len(response.Tools), quicktest.Equals, 1) // Should return 1 tool matching "drill"
		c.Assert(response.Tools[0].Title, quicktest.Equals, "Drill")
		c.Assert(response.Pagination.Total, quicktest.Equals, int64(1))
	})

	t.Run("search by description", func(t *testing.T) {
		// Create request with search term matching description
		req := httptest.NewRequest(
			"GET",
			fmt.Sprintf("/communities/%s/tools?page=0&pageSize=10&term=construction",
				community.ID.Hex(),
			), nil)

		// Create chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("communityId", community.ID.Hex())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Create API request
		apiReq := &Request{
			UserID:  userID.Hex(),
			Context: &HTTPContext{Request: req},
		}

		// Call handler
		result, err := api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.IsNil)

		// Verify response
		response, ok := result.(*PaginatedToolsResponse)
		c.Assert(ok, quicktest.IsTrue)
		c.Assert(len(response.Tools), quicktest.Equals, 1) // Should return 1 tool matching "construction"
		c.Assert(response.Tools[0].Title, quicktest.Equals, "Hammer")
	})

	t.Run("empty search results", func(t *testing.T) {
		// Create request with search term that doesn't match anything
		req := httptest.NewRequest(
			"GET",
			fmt.Sprintf("/communities/%s/tools?page=0&pageSize=10&term=nonexistent",
				community.ID.Hex(),
			), nil)

		// Create chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("communityId", community.ID.Hex())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Create API request
		apiReq := &Request{
			UserID:  userID.Hex(),
			Context: &HTTPContext{Request: req},
		}

		// Call handler
		result, err := api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.IsNil)

		// Verify response
		response, ok := result.(*PaginatedToolsResponse)
		c.Assert(ok, quicktest.IsTrue)
		c.Assert(len(response.Tools), quicktest.Equals, 0) // Should return no tools
		c.Assert(response.Pagination.Total, quicktest.Equals, int64(0))
	})

	t.Run("unauthorized user", func(t *testing.T) {
		// Create request without user ID
		req := httptest.NewRequest("GET", fmt.Sprintf("/communities/%s/tools", community.ID.Hex()), nil)

		// Create chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("communityId", community.ID.Hex())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Create API request without UserID
		apiReq := &Request{
			UserID:  "",
			Context: &HTTPContext{Request: req},
		}

		// Call handler
		_, err := api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.Not(quicktest.IsNil))
		c.Assert(err.Error(), quicktest.Contains, "user not authenticated")
	})

	t.Run("invalid community ID", func(t *testing.T) {
		// Create request with invalid community ID
		req := httptest.NewRequest("GET", "/communities/invalid-id/tools", nil)

		// Create chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("communityId", "invalid-id")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Create API request
		apiReq := &Request{
			UserID:  userID.Hex(),
			Context: &HTTPContext{Request: req},
		}

		// Call handler
		_, err := api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.Not(quicktest.IsNil))
		c.Assert(err.Error(), quicktest.Contains, "invalid")
	})

	t.Run("nonexistent community", func(t *testing.T) {
		// Create request with nonexistent community ID
		nonexistentID := primitive.NewObjectID()
		req := httptest.NewRequest("GET", fmt.Sprintf("/communities/%s/tools", nonexistentID.Hex()), nil)

		// Create chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("communityId", nonexistentID.Hex())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Create API request
		apiReq := &Request{
			UserID:  userID.Hex(),
			Context: &HTTPContext{Request: req},
		}

		// Call handler
		_, err := api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.Not(quicktest.IsNil))
		c.Assert(err.Error(), quicktest.Contains, "Not member of the community")
	})

	t.Run("case insensitive search", func(t *testing.T) {
		// Create request with uppercase search term
		req := httptest.NewRequest("GET", fmt.Sprintf("/communities/%s/tools?page=0&pageSize=10&term=HAMMER", community.ID.Hex()), nil)

		// Create chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("communityId", community.ID.Hex())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Create API request
		apiReq := &Request{
			UserID:  userID.Hex(),
			Context: &HTTPContext{Request: req},
		}

		// Call handler
		result, err := api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.IsNil)

		// Verify response
		response, ok := result.(*PaginatedToolsResponse)
		c.Assert(ok, quicktest.IsTrue)
		c.Assert(len(response.Tools), quicktest.Equals, 1) // Should return 1 tool matching "HAMMER" (case insensitive)
		c.Assert(response.Tools[0].Title, quicktest.Equals, "Hammer")
	})
}

func TestGetCommunityToolsHandlerIntegration(t *testing.T) {
	c := quicktest.New(t)
	api := testAPI(t)

	// Create test user
	testUser := db.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: []byte("hashedpassword"),
		Active:   true,
		Location: db.NewLocation(40000000, -74000000),
	}
	userID, err := api.addUser(&testUser)
	c.Assert(err, quicktest.IsNil)

	// Create test community
	community, err := api.database.CommunityService.CreateCommunity(
		context.Background(),
		"Test Community",
		types.HexBytes{},
		userID,
	)
	c.Assert(err, quicktest.IsNil)

	// Create many tools for pagination testing
	for i := 1; i <= 25; i++ {
		tool := &db.Tool{
			ID:          int64(2000 + i),
			UserID:      userID,
			Title:       fmt.Sprintf("Tool %d", i),
			Description: fmt.Sprintf("Description for tool %d", i),
			IsAvailable: true,
			MayBeFree:   i%2 == 0, // Every other tool is free
			Cost:        uint64(100 * i),
			Location:    db.NewLocation(40000000, -74000000),
			Communities: []primitive.ObjectID{community.ID},
		}
		_, err := api.database.ToolService.InsertTool(context.Background(), tool)
		c.Assert(err, quicktest.IsNil)
	}

	t.Run("pagination across multiple pages", func(t *testing.T) {
		// Test first page
		req := httptest.NewRequest("GET", fmt.Sprintf("/communities/%s/tools?page=0&pageSize=10", community.ID.Hex()), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("communityId", community.ID.Hex())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		apiReq := &Request{
			UserID:  userID.Hex(),
			Context: &HTTPContext{Request: req},
		}

		result, err := api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.IsNil)

		response, ok := result.(*PaginatedToolsResponse)
		c.Assert(ok, quicktest.IsTrue)
		c.Assert(len(response.Tools), quicktest.Equals, 10)
		c.Assert(response.Pagination.Current, quicktest.Equals, 0)
		c.Assert(response.Pagination.Total, quicktest.Equals, int64(25))
		c.Assert(response.Pagination.Pages, quicktest.Equals, 3) // 25 tools / 10 per page = 3 pages

		// Test last page
		req = httptest.NewRequest("GET", fmt.Sprintf("/communities/%s/tools?page=2&pageSize=10", community.ID.Hex()), nil)
		rctx = chi.NewRouteContext()
		rctx.URLParams.Add("communityId", community.ID.Hex())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		apiReq = &Request{
			UserID:  userID.Hex(),
			Context: &HTTPContext{Request: req},
		}

		result, err = api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.IsNil)

		response, ok = result.(*PaginatedToolsResponse)
		c.Assert(ok, quicktest.IsTrue)
		c.Assert(len(response.Tools), quicktest.Equals, 5) // Last page should have 5 tools
		c.Assert(response.Pagination.Current, quicktest.Equals, 2)
	})

	t.Run("search with pagination", func(t *testing.T) {
		// Search for tools with "1" in the title (Tool 1, Tool 10-19, Tool 21)
		req := httptest.NewRequest("GET", fmt.Sprintf("/communities/%s/tools?page=0&pageSize=5&term=1", community.ID.Hex()), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("communityId", community.ID.Hex())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		apiReq := &Request{
			UserID:  userID.Hex(),
			Context: &HTTPContext{Request: req},
		}

		result, err := api.getCommunityToolsHandler(apiReq)
		c.Assert(err, quicktest.IsNil)

		response, ok := result.(*PaginatedToolsResponse)
		c.Assert(ok, quicktest.IsTrue)
		c.Assert(len(response.Tools), quicktest.Equals, 5)          // First 5 matching tools
		c.Assert(response.Pagination.Total >= 12, quicktest.IsTrue) // At least Tool 1, 10-19, 21
	})
}
