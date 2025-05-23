package db

import (
	"context"
	"math"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestWithinCircumference(t *testing.T) {
	t.Run("Distance Calculations", func(t *testing.T) {
		// Base location (Barcelona area)
		base := DBLocation{
			Type: "Point",
			Coordinates: []float64{
				2.492793,  // longitude
				41.695384, // latitude
			},
		}

		// Test cases
		tests := []struct {
			name     string
			loc      DBLocation
			distance int
			want     bool
		}{
			{
				name: "Same point",
				loc: DBLocation{
					Type: "Point",
					Coordinates: []float64{
						2.492793,  // longitude
						41.695384, // latitude
					},
				},
				distance: 1000,
				want:     true,
			},
			{
				name: "Point at exactly 5km north",
				loc: DBLocation{
					Type: "Point",
					Coordinates: []float64{
						2.492793,  // longitude
						41.740384, // latitude (~5km north)
					},
				},
				distance: 5000,
				want:     true,
			},
			{
				name: "Point at 5.1km north (should fail for 5km radius)",
				loc: DBLocation{
					Type: "Point",
					Coordinates: []float64{
						2.492793,  // longitude
						41.741384, // latitude (~5.1km north)
					},
				},
				distance: 5000,
				want:     false,
			},
			{
				name: "Point at diagonal ~7km (5km north, 5km east)",
				loc: DBLocation{
					Type: "Point",
					Coordinates: []float64{
						2.557793,  // longitude (~5km east)
						41.740384, // latitude (~5km north)
					},
				},
				distance: 8000, // Should be within 8km radius
				want:     true,
			},
			{
				name: "Point at 10km north",
				loc: DBLocation{
					Type: "Point",
					Coordinates: []float64{
						2.492793,  // longitude
						41.785384, // latitude (~10km north)
					},
				},
				distance: 10000,
				want:     true,
			},
			{
				name: "Point at 10.1km north (should fail for 10km radius)",
				loc: DBLocation{
					Type: "Point",
					Coordinates: []float64{
						2.492793,  // longitude
						41.786384, // latitude (~10.1km north)
					},
				},
				distance: 10000,
				want:     false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := WithinCircumference(base, tt.loc, tt.distance)
				if got != tt.want {
					// Calculate actual distance for debugging
					lat1 := base.Coordinates[1]
					long1 := base.Coordinates[0]
					lat2 := tt.loc.Coordinates[1]
					long2 := tt.loc.Coordinates[0]

					// Convert to radians
					lat1Rad := lat1 * (math.Pi / 180)
					long1Rad := long1 * (math.Pi / 180)
					lat2Rad := lat2 * (math.Pi / 180)
					long2Rad := long2 * (math.Pi / 180)

					// Calculate distance using Haversine formula
					a := math.Sin((lat2Rad-lat1Rad)/2)*math.Sin((lat2Rad-lat1Rad)/2) +
						math.Cos(lat1Rad)*math.Cos(lat2Rad)*
							math.Sin((long2Rad-long1Rad)/2)*math.Sin((long2Rad-long1Rad)/2)
					c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
					d := earthRadius * c * 1000 // distance in meters

					t.Errorf("WithinCircumference() = %v, want %v (actual distance: %.2f meters)", got, tt.want, d)
				}
			})
		}
	})

	t.Run("Verify 5km Test Case", func(t *testing.T) {
		// Base location (Barcelona area)
		base := DBLocation{
			Type: "Point",
			Coordinates: []float64{
				2.492793,  // longitude
				41.695384, // latitude
			},
		}

		// Tool at 5km north
		toolAt5km := DBLocation{
			Type: "Point",
			Coordinates: []float64{
				2.492793,  // longitude
				41.740384, // latitude (~5km north)
			},
		}

		// Calculate actual distance
		lat1 := base.Coordinates[1]
		long1 := base.Coordinates[0]
		lat2 := toolAt5km.Coordinates[1]
		long2 := toolAt5km.Coordinates[0]

		// Convert to radians
		lat1Rad := lat1 * (math.Pi / 180)
		long1Rad := long1 * (math.Pi / 180)
		lat2Rad := lat2 * (math.Pi / 180)
		long2Rad := long2 * (math.Pi / 180)

		// Calculate distance using Haversine formula
		a := math.Sin((lat2Rad-lat1Rad)/2)*math.Sin((lat2Rad-lat1Rad)/2) +
			math.Cos(lat1Rad)*math.Cos(lat2Rad)*
				math.Sin((long2Rad-long1Rad)/2)*math.Sin((long2Rad-long1Rad)/2)
		c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
		d := earthRadius * c * 1000 // distance in meters

		t.Logf("Actual distance between points: %.2f meters", d)
		t.Logf("Base location: %.6f°N, %.6f°E", lat1, long1)
		t.Logf("Tool location: %.6f°N, %.6f°E", lat2, long2)
		t.Logf("Difference in latitude: %.6f°", lat2-lat1)

		// Verify the distance is close to 5km
		if math.Abs(d-5000) > 100 {
			t.Errorf("Distance is %.2f meters, expected close to 5000 meters", d)
		}

		// Verify WithinCircumference works correctly
		if !WithinCircumference(base, toolAt5km, 5000) {
			t.Error("WithinCircumference failed for 5km distance")
		}
	})
}

func TestSearchToolsByLocation(t *testing.T) {
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

	// Create test tools at different distances
	tools := []struct {
		name     string
		location DBLocation
		distance float64 // Expected distance in meters
	}{
		{
			name: "Tool at origin",
			location: DBLocation{
				Type: "Point",
				Coordinates: []float64{
					2.492793,  // longitude
					41.695384, // latitude
				},
			},
			distance: 0,
		},
		{
			name: "Tool at 5km north",
			location: DBLocation{
				Type: "Point",
				Coordinates: []float64{
					2.492793,  // longitude
					41.740384, // latitude (~5km north)
				},
			},
			distance: 5000,
		},
		{
			name: "Tool at 15km north",
			location: DBLocation{
				Type: "Point",
				Coordinates: []float64{
					2.492793,  // longitude
					41.830384, // latitude (~15km north)
				},
			},
			distance: 15000,
		},
		{
			name: "Tool at 25km north",
			location: DBLocation{
				Type: "Point",
				Coordinates: []float64{
					2.492793,  // longitude
					41.920384, // latitude (~25km north)
				},
			},
			distance: 25000,
		},
	}

	// Insert test tools
	for i, tt := range tools {
		tool := &Tool{
			ID:          int64(i + 1),
			Title:       tt.name,
			Description: "Test tool",
			IsAvailable: true,
			Location:    tt.location,
		}
		_, err := toolService.InsertTool(ctx, tool)
		qt.Assert(t, err, qt.IsNil)
	}

	// Test cases for different search radii
	tests := []struct {
		name          string
		radius        int
		wantToolCount int
		wantToolNames []string
	}{
		{
			name:          "Search within 1km",
			radius:        1000,
			wantToolCount: 1,
			wantToolNames: []string{"Tool at origin"},
		},
		{
			name:          "Search within 10km",
			radius:        10000,
			wantToolCount: 2,
			wantToolNames: []string{"Tool at origin", "Tool at 5km north"},
		},
		{
			name:          "Search within 20km",
			radius:        20000,
			wantToolCount: 3,
			wantToolNames: []string{"Tool at origin", "Tool at 5km north", "Tool at 15km north"},
		},
		{
			name:          "Search within 30km",
			radius:        30000,
			wantToolCount: 4,
			wantToolNames: []string{"Tool at origin", "Tool at 5km north", "Tool at 15km north", "Tool at 25km north"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundTools, err := toolService.SearchToolsByLocation(ctx, baseLocation, tt.radius)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(foundTools), qt.Equals, tt.wantToolCount)

			// Verify the correct tools were found
			foundNames := make(map[string]bool)
			for _, tool := range foundTools {
				foundNames[tool.Title] = true
			}
			for _, wantName := range tt.wantToolNames {
				if !foundNames[wantName] {
					t.Errorf("Expected to find tool %q but it was not in results", wantName)
				}
			}
		})
	}

	// Test that tools are returned in order of distance
	t.Run("Tools are ordered by distance", func(t *testing.T) {
		foundTools, err := toolService.SearchToolsByLocation(ctx, baseLocation, 30000)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(foundTools), qt.Equals, 4)

		expectedOrder := []string{
			"Tool at origin",
			"Tool at 5km north",
			"Tool at 15km north",
			"Tool at 25km north",
		}

		for i, want := range expectedOrder {
			if foundTools[i].Title != want {
				t.Errorf("Tool at position %d: got %q, want %q", i, foundTools[i].Title, want)
			}
		}
	})
}

func TestSearchTitleAndDescription(t *testing.T) {
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

	// Insert test tools
	location := DBLocation{
		Type: "Point",
		Coordinates: []float64{
			2.492793,  // longitude
			41.695384, // latitude
		},
	}

	tools := []Tool{
		{
			ID:          1,
			Title:       "AmazingTool",
			Description: "A great tool for professionals",
			IsAvailable: true,
			Location:    location,
		},
		{
			ID:          2,
			Title:       "Super RangeFinder",
			Description: "Find distances with precision",
			IsAvailable: true,
			Location:    location,
		},
		{
			ID:          3,
			Title:       "rangeX 3000",
			Description: "Powerful laser range device",
			IsAvailable: true,
			Location:    location,
		},
		{
			ID:          4,
			Title:       "Basic Hammer",
			Description: "Simple but effective hammer",
			IsAvailable: true,
			Location:    location,
		},
		{
			ID:          5,
			Title:       "DrillMaster",
			Description: "A powerful drill machine",
			IsAvailable: true,
			Location:    location,
		},
	}

	for _, tool := range tools {
		_, err := toolService.InsertTool(ctx, &tool)
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert tool: %s", tool.Title))
	}

	// Define search test cases
	tests := []struct {
		name        string
		searchTerm  string
		expectedIDs []int64
	}{
		{
			name:        "Case-insensitive match",
			searchTerm:  "amazing",
			expectedIDs: []int64{1}, // Matches "AmazingTool"
		},
		{
			name:        "Partial word match in title",
			searchTerm:  "range",
			expectedIDs: []int64{2, 3}, // Matches "Super RangeFinder" and "rangeX 3000"
		},
		{
			name:        "Search in description",
			searchTerm:  "powerful",
			expectedIDs: []int64{3, 5}, // Matches "Powerful laser range device" and "A powerful drill machine"
		},
		{
			name:        "No match scenario",
			searchTerm:  "nonexistent",
			expectedIDs: nil, // No tools should match
		},
	}

	// Execute search tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := SearchToolsOptions{SearchTerm: tt.searchTerm}
			foundTools, _, err := toolService.SearchTools(ctx, opts)
			qt.Assert(t, err, qt.IsNil, qt.Commentf("SearchTools failed with term: %s", tt.searchTerm))

			// Extract IDs from found tools
			var foundIDs []int64
			for _, tool := range foundTools {
				foundIDs = append(foundIDs, tool.ID)
			}

			// Verify expected results
			qt.Assert(t, foundIDs, qt.DeepEquals, tt.expectedIDs, qt.Commentf("SearchTerm: %s", tt.searchTerm))
		})
	}
}
