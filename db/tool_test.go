package db

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Helper function to create ObjectID for testing
func createTestObjectID(id string) primitive.ObjectID {
	objectID, _ := primitive.ObjectIDFromHex(fmt.Sprintf("%024s", id))
	return objectID
}

// toolID generates a unique int64 ID for a tool based on owner ID and title
func toolID(ownerID string, title string) int64 {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s-%s", ownerID, title)))
	hash := hasher.Sum(nil)
	// Convert the first 4 bytes of the hash to an absolute int64
	return int64(math.Abs(float64(int64(binary.BigEndian.Uint32(hash[:4])))))
}

func TestToolService(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Start MongoDB container
	container, err := StartMongoContainer(ctx)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to start MongoDB container"))
	defer func() { _ = container.Terminate(ctx) }()

	// Get MongoDB connection string
	mongoURI, err := container.Endpoint(ctx, "mongodb")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to get MongoDB connection string"))

	// Create a MongoDB client
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create MongoDB client"))
	defer func() { _ = client.Disconnect(ctx) }()

	// Use a random database name for isolation
	dbName := RandomDatabaseName()
	database := client.Database(dbName)

	// Initialize ToolService
	toolService := NewToolService(&Database{
		Client:   client,
		Database: database,
	})

	c.Run("Insert and Retrieve Tool", func(c *qt.C) {
		tool := &Tool{
			Title:       "Test Tool",
			Description: "A tool for testing",
			IsAvailable: true,
			MayBeFree:   true,
			AskWithFee:  false,
			Cost:        1000,
			UserID:      createTestObjectID("123"),
			Images: []Image{
				{
					Hash:    []byte("testhash"),
					Name:    "tool_image.jpg",
					Content: []byte("testcontent"),
					Link:    "https://example.com/tool_image.jpg",
				},
			},
			TransportOptions: []Transport{
				{ID: 1, Name: "Car"},
			},
			ToolCategory:   1,
			Location:       Location{Latitude: 123456, Longitude: 654321},
			Rating:         4,
			EstimatedValue: 50000,
			Height:         100,
			Weight:         500,
			ReservedDates: []DateRange{
				{From: 1640995200, To: 1641081600}, // Example: Dec 31, 2021 to Jan 1, 2022
			},
		}

		// Insert Tool
		insertResult, err := toolService.InsertTool(ctx, tool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert tool"))
		c.Assert(insertResult.InsertedID, qt.Not(qt.IsNil), qt.Commentf("Insert result ID is nil"))

		// Retrieve Tool by ID
		toolID := insertResult.InsertedID.(int64)
		retrievedTool, err := toolService.GetToolByID(ctx, toolID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve tool by ID"))
		c.Assert(retrievedTool.Title, qt.Equals, tool.Title, qt.Commentf("Titles do not match"))
		c.Assert(retrievedTool.Description, qt.Equals, tool.Description, qt.Commentf("Descriptions do not match"))
		c.Assert(retrievedTool.Location, qt.DeepEquals, tool.Location, qt.Commentf("Locations do not match"))
	})

	c.Run("Update Tool", func(c *qt.C) {
		// Insert initial tool
		tool := &Tool{
			ID:          toolID("updateuser123", "Update Tool"),
			Title:       "Update Tool",
			Description: "A tool to update",
			IsAvailable: true,
			Cost:        2000,
			UserID:      createTestObjectID("456"),
			Location:    Location{Latitude: 111222, Longitude: 333444},
		}

		insertResult, err := toolService.InsertTool(ctx, tool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert tool for update test"))

		// Update tool fields
		toolID := insertResult.InsertedID.(int64)
		update := bson.M{
			"title":       "Updated Tool",
			"description": "Updated description",
			"cost":        2500,
			"rating":      5,
		}

		updateResult, err := toolService.UpdateTool(ctx, toolID, update)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to update tool"))
		c.Assert(updateResult.ModifiedCount, qt.Equals, int64(1), qt.Commentf("Expected 1 document to be modified"))

		// Verify update
		updatedTool, err := toolService.GetToolByID(ctx, toolID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve updated tool"))
		c.Assert(updatedTool.Title, qt.Equals, "Updated Tool", qt.Commentf("Title was not updated"))
		c.Assert(updatedTool.Description, qt.Equals, "Updated description", qt.Commentf("Description was not updated"))
		c.Assert(updatedTool.Cost, qt.Equals, uint64(2500), qt.Commentf("Cost was not updated"))
		c.Assert(updatedTool.Rating, qt.Equals, int32(5), qt.Commentf("Rating was not updated"))
	})

	c.Run("Search Tools By Location", func(c *qt.C) {
		// Insert tools at different locations
		tools := []*Tool{
			{
				ID:       toolID("user1", "Nearby Tool 1"),
				Title:    "Nearby Tool 1",
				UserID:   createTestObjectID("001"),
				Location: Location{Latitude: 100000, Longitude: 100000},
			},
			{
				ID:       toolID("user2", "Nearby Tool 2"),
				Title:    "Nearby Tool 2",
				UserID:   createTestObjectID("002"),
				Location: Location{Latitude: 100100, Longitude: 100100},
			},
			{
				ID:       toolID("user3", "Far Tool"),
				Title:    "Far Tool",
				UserID:   createTestObjectID("003"),
				Location: Location{Latitude: 200000, Longitude: 200000},
			},
		}

		for _, t := range tools {
			_, err := toolService.InsertTool(ctx, t)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert test tool"))
		}

		// Search for tools near location
		searchLocation := Location{Latitude: 100000, Longitude: 100000}
		foundTools, err := toolService.SearchToolsByLocation(ctx, searchLocation, 1000)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to search tools by location"))
		c.Assert(len(foundTools) >= 2, qt.Equals, true, qt.Commentf("Expected at least 2 nearby tools"))

		// Verify found tools are the nearby ones
		foundTitles := make(map[string]bool)
		for _, t := range foundTools {
			foundTitles[t.Title] = true
		}
		c.Assert(foundTitles["Nearby Tool 1"], qt.Equals, true, qt.Commentf("Nearby Tool 1 not found"))
		c.Assert(foundTitles["Nearby Tool 2"], qt.Equals, true, qt.Commentf("Nearby Tool 2 not found"))
	})

	c.Run("Get All Tools", func(c *qt.C) {
		// Insert additional tools
		tools := []*Tool{
			{
				ID:          toolID("user4", "List Tool 1"),
				Title:       "List Tool 1",
				Description: "First tool for listing",
				UserID:      createTestObjectID("004"),
				IsAvailable: true,
			},
			{
				ID:          toolID("user5", "List Tool 2"),
				Title:       "List Tool 2",
				Description: "Second tool for listing",
				UserID:      createTestObjectID("005"),
				IsAvailable: true,
			},
		}

		for _, t := range tools {
			_, err := toolService.InsertTool(ctx, t)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert test tool"))
		}

		// Retrieve all tools
		allTools, err := toolService.GetAllTools(ctx)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve all tools"))
		c.Assert(len(allTools) >= 5, qt.Equals, true, qt.Commentf("Expected at least 5 tools in database"))

		// Verify the content of retrieved tools
		foundTools := make(map[string]bool)
		for _, t := range allTools {
			foundTools[t.Title] = true
		}

		c.Assert(foundTools["List Tool 1"], qt.Equals, true, qt.Commentf("List Tool 1 not found in results"))
		c.Assert(foundTools["List Tool 2"], qt.Equals, true, qt.Commentf("List Tool 2 not found in results"))
	})

	c.Run("Get Non-existent Tool", func(c *qt.C) {
		// Try to retrieve a tool with a non-existent ID
		nonExistentID := int64(999999)
		_, err := toolService.GetToolByID(ctx, nonExistentID)
		c.Assert(err, qt.Equals, mongo.ErrNoDocuments, qt.Commentf("Expected no documents error"))
	})
}
