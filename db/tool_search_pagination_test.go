package db

import (
	"context"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestSearchToolsPagination(t *testing.T) {
	ctx := context.Background()

	// Start MongoDB container
	container, err := StartMongoContainer(ctx)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to start MongoDB container"))
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	// Get MongoDB connection string
	mongoURI, err := container.Endpoint(ctx, "mongodb")
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get MongoDB connection string"))

	// Create a MongoDB client
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to create MongoDB client"))
	defer func() { _ = client.Disconnect(ctx) }()

	// Use a random database name for isolation
	dbName := RandomDatabaseName()
	database := client.Database(dbName)

	// Initialize ToolService
	toolService := NewToolService(&Database{
		Client:   client,
		Database: database,
	})

	// Create test location
	location := DBLocation{
		Type: "Point",
		Coordinates: []float64{
			2.492793,  // longitude
			41.695384, // latitude
		},
	}

	testToolsCount := 40
	// Initialize UserService and create a test user
	userService := NewUserService(&Database{
		Client:   client,
		Database: database,
	})

	// Create a test user
	userID, err := CreateTestUser(ctx, userService, "testuser@example.com", "Test User")
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to create test user"))

	// Insert test tools
	for i := 1; i <= testToolsCount; i++ {
		tool := &Tool{
			ID:           int64(i),
			UserID:       userID,
			Title:        "Test Tool " + string(rune(i+'A'-1)), // Tool A, Tool B, etc.
			Description:  "Test description for tool",
			IsAvailable:  true,
			Location:     location,
			ToolCategory: 1,
		}
		_, err := toolService.InsertTool(ctx, tool)
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert tool %d", i))
	}

	t.Run("Basic pagination without filters", func(t *testing.T) {
		// Test first page
		opts := SearchToolsOptions{
			Page: 0,
		}
		tools, total, err := toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, DefaultPageSize)
		qt.Assert(t, total, qt.Equals, int64(testToolsCount))

		// Test second page
		opts.Page = 1
		tools, total, err = toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, DefaultPageSize)
		qt.Assert(t, total, qt.Equals, int64(testToolsCount))

		// Test third page (partial)
		opts.Page = 2
		tools, total, err = toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, testToolsCount%DefaultPageSize)
		qt.Assert(t, total, qt.Equals, int64(testToolsCount))

		// Test page beyond available data
		opts.Page = 3
		tools, total, err = toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, 0)
		qt.Assert(t, total, qt.Equals, int64(testToolsCount))
	})

	t.Run("Pagination with search term", func(t *testing.T) {
		// Insert some tools with specific titles for search testing
		searchTools := []Tool{
			{
				ID: 100, UserID: userID, Title: "Special Hammer", Description: "A special hammer",
				IsAvailable: true, Location: location, ToolCategory: 1,
			},
			{
				ID: 101, UserID: userID, Title: "Special Drill", Description: "A special drill",
				IsAvailable: true, Location: location, ToolCategory: 1,
			},
			{
				ID: 102, UserID: userID, Title: "Special Saw", Description: "A special saw",
				IsAvailable: true, Location: location, ToolCategory: 1,
			},
			{
				ID: 103, UserID: userID, Title: "Special Wrench", Description: "A special wrench",
				IsAvailable: true, Location: location, ToolCategory: 1,
			},
			{
				ID: 104, UserID: userID, Title: "Special Screwdriver", Description: "A special screwdriver",
				IsAvailable: true, Location: location, ToolCategory: 1,
			},
		}

		for _, tool := range searchTools {
			_, err := toolService.InsertTool(ctx, &tool)
			qt.Assert(t, err, qt.IsNil)
		}

		// Search for "Special" tools with pagination
		opts := SearchToolsOptions{
			SearchTerm: "Special",
			Page:       0,
		}
		tools, total, err := toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, 5)
		qt.Assert(t, total, qt.Equals, int64(5))
	})

	t.Run("Negative page number", func(t *testing.T) {
		// Test with negative page (should default to 0)
		opts := SearchToolsOptions{
			Page: -1,
		}
		tools, total, err := toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, DefaultPageSize)
		qt.Assert(t, total, qt.Equals, int64(testToolsCount+5)) // 40 + 5 special tools
	})
}

func TestSearchToolsPaginationWithGeoNear(t *testing.T) {
	ctx := context.Background()

	// Start MongoDB container
	container, err := StartMongoContainer(ctx)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to start MongoDB container"))
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	// Get MongoDB connection string
	mongoURI, err := container.Endpoint(ctx, "mongodb")
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get MongoDB connection string"))

	// Create a MongoDB client
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to create MongoDB client"))
	defer func() { _ = client.Disconnect(ctx) }()

	// Use a random database name for isolation
	dbName := RandomDatabaseName()
	database := client.Database(dbName)

	// Initialize ToolService
	toolService := NewToolService(&Database{
		Client:   client,
		Database: database,
	})

	// Base location (Barcelona area)
	baseLocation := DBLocation{
		Type: "Point",
		Coordinates: []float64{
			2.492793,  // longitude
			41.695384, // latitude
		},
	}

	// Initialize UserService and create a test user
	userService := NewUserService(&Database{
		Client:   client,
		Database: database,
	})

	// Create a test user
	userID, err := CreateTestUser(ctx, userService, "geouser@example.com", "Geo Test User")
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to create test user"))

	// Create test tools at different distances but within search radius
	locations := []DBLocation{
		// Tools within 10km radius
		{Type: "Point", Coordinates: []float64{2.492793, 41.695384}}, // 0km
		{Type: "Point", Coordinates: []float64{2.492793, 41.705384}}, // ~1km north
		{Type: "Point", Coordinates: []float64{2.492793, 41.715384}}, // ~2km north
		{Type: "Point", Coordinates: []float64{2.492793, 41.725384}}, // ~3km north
		{Type: "Point", Coordinates: []float64{2.492793, 41.735384}}, // ~4km north
		{Type: "Point", Coordinates: []float64{2.492793, 41.745384}}, // ~5km north
		{Type: "Point", Coordinates: []float64{2.492793, 41.755384}}, // ~6km north
		{Type: "Point", Coordinates: []float64{2.492793, 41.765384}}, // ~7km north
		{Type: "Point", Coordinates: []float64{2.492793, 41.775384}}, // ~8km north
		{Type: "Point", Coordinates: []float64{2.492793, 41.785384}}, // ~9km north
	}

	// Insert 10 tools within 10km radius
	for i, loc := range locations {
		tool := &Tool{
			ID:           int64(i + 1),
			UserID:       userID,
			Title:        "Geo Tool " + strconv.Itoa(i),
			Description:  "Test geo tool",
			IsAvailable:  true,
			Location:     loc,
			ToolCategory: 1,
		}
		_, err := toolService.InsertTool(ctx, tool)
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert geo tool %d", i+1))
	}

	t.Run("GeoNear pagination", func(t *testing.T) {
		// Test first page with geospatial search
		opts := SearchToolsOptions{
			Distance: 10000, // 10km radius
			Location: &baseLocation,
			Page:     0,
		}
		tools, total, err := toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, 9)
		qt.Assert(t, total, qt.Equals, int64(9))

		// Test first page with geospatial search
		opts = SearchToolsOptions{
			Distance: 5000, // 5km radius
			Location: &baseLocation,
			Page:     0,
		}
		tools, total, err = toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, 5)
		qt.Assert(t, total, qt.Equals, int64(5))

		// Verify tools are still ordered by distance (first page should have closest tools)
		opts.Page = 0
		tools, _, err = toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, tools[0].Title, qt.Equals, "Geo Tool 0") // Closest tool
	})

	t.Run("GeoNear with search term and pagination", func(t *testing.T) {
		// Insert some tools with specific titles for combined geo + search testing
		specialGeoTools := []struct {
			id       int64
			title    string
			location DBLocation
		}{
			{200, "Special Geo Hammer", DBLocation{Type: "Point", Coordinates: []float64{2.492793, 41.700384}}}, // ~0.5km
			{201, "Special Geo Drill", DBLocation{Type: "Point", Coordinates: []float64{2.492793, 41.710384}}},  // ~1.5km
			{202, "Special Geo Saw", DBLocation{Type: "Point", Coordinates: []float64{2.492793, 41.720384}}},    // ~2.5km
		}

		for _, tool := range specialGeoTools {
			toolObj := &Tool{
				ID:           tool.id,
				UserID:       userID,
				Title:        tool.title,
				Description:  "Special geo tool",
				IsAvailable:  true,
				Location:     tool.location,
				ToolCategory: 1,
			}
			_, err := toolService.InsertTool(ctx, toolObj)
			qt.Assert(t, err, qt.IsNil)
		}

		// Search for "Special Geo" tools with geospatial constraints and pagination
		opts := SearchToolsOptions{
			SearchTerm: "Special Geo",
			Distance:   10000, // 10km radius
			Location:   &baseLocation,
			Page:       0,
		}
		tools, total, err := toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, 3)
		qt.Assert(t, total, qt.Equals, int64(3))

		// Verify tools are ordered by distance
		qt.Assert(t, tools[0].Title, qt.Equals, "Special Geo Hammer") // Closest
		qt.Assert(t, tools[1].Title, qt.Equals, "Special Geo Drill")  // Second closest
	})
}

func TestSearchToolsPaginationWithFilters(t *testing.T) {
	ctx := context.Background()

	// Start MongoDB container
	container, err := StartMongoContainer(ctx)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to start MongoDB container"))
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	// Get MongoDB connection string
	mongoURI, err := container.Endpoint(ctx, "mongodb")
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get MongoDB connection string"))

	// Create a MongoDB client
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to create MongoDB client"))
	defer func() { _ = client.Disconnect(ctx) }()

	// Use a random database name for isolation
	dbName := RandomDatabaseName()
	database := client.Database(dbName)

	// Initialize ToolService
	toolService := NewToolService(&Database{
		Client:   client,
		Database: database,
	})

	// Create test location
	location := DBLocation{
		Type: "Point",
		Coordinates: []float64{
			2.492793,  // longitude
			41.695384, // latitude
		},
	}

	// Initialize UserService and create a test user
	userService := NewUserService(&Database{
		Client:   client,
		Database: database,
	})

	// Create a test user
	userID, err := CreateTestUser(ctx, userService, "filteruser@example.com", "Filter Test User")
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to create test user"))

	// Insert tools with different categories and costs
	testTools := []Tool{
		// Category 1 tools (cheap)
		{
			ID: 1, UserID: userID, Title: "Cheap Hammer", Description: "Basic hammer",
			IsAvailable: true, Location: location, ToolCategory: 1, Cost: 100,
		},
		{
			ID: 2, UserID: userID, Title: "Cheap Drill", Description: "Basic drill",
			IsAvailable: true, Location: location, ToolCategory: 1, Cost: 150,
		},
		{
			ID: 3, UserID: userID, Title: "Cheap Saw", Description: "Basic saw",
			IsAvailable: true, Location: location, ToolCategory: 1, Cost: 200,
		},
		{
			ID: 4, UserID: userID, Title: "Cheap Wrench", Description: "Basic wrench",
			IsAvailable: true, Location: location, ToolCategory: 1, Cost: 120,
		},
		{
			ID: 5, UserID: userID, Title: "Cheap Screwdriver", Description: "Basic screwdriver",
			IsAvailable: true, Location: location, ToolCategory: 1, Cost: 80,
		},

		// Category 2 tools (expensive)
		{
			ID: 6, UserID: userID, Title: "Pro Hammer", Description: "Professional hammer",
			IsAvailable: true, Location: location, ToolCategory: 2, Cost: 500,
		},
		{
			ID: 7, UserID: userID, Title: "Pro Drill", Description: "Professional drill",
			IsAvailable: true, Location: location, ToolCategory: 2, Cost: 800,
		},
		{
			ID: 8, UserID: userID, Title: "Pro Saw", Description: "Professional saw",
			IsAvailable: true, Location: location, ToolCategory: 2, Cost: 1000,
		},
	}

	for _, tool := range testTools {
		_, err := toolService.InsertTool(ctx, &tool)
		qt.Assert(t, err, qt.IsNil)
	}

	t.Run("Pagination with category filter", func(t *testing.T) {
		// Search for category 1 tools with pagination
		opts := SearchToolsOptions{
			Categories: []int{1},
			Page:       0,
		}
		tools, total, err := toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, 5)
		qt.Assert(t, total, qt.Equals, int64(5))
	})

	t.Run("Pagination with cost filter", func(t *testing.T) {
		// Search for tools under 300 cost with pagination
		maxCost := uint64(300)
		opts := SearchToolsOptions{
			MaxCost: &maxCost,
			Page:    0,
		}
		tools, total, err := toolService.SearchTools(ctx, opts)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, 5)
		qt.Assert(t, total, qt.Equals, int64(5)) // 5 tools under 300
	})
}
