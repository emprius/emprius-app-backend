package db

import (
	"context"
	"testing"

	"github.com/emprius/emprius-app-backend/types"
	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	testOwnerName  = "test-owner"
	testOwnerEmail = "test-owner@example.com"
	testToolTitle  = "Test Tool"
	testToolDesc   = "A tool for testing"
)

func TestCountCommunityTools(t *testing.T) {
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

	// Initialize Database with all services
	db := &Database{
		Client:   client,
		Database: database,
	}
	db.UserService = NewUserService(db)
	db.ToolService = NewToolService(db)
	db.CommunityService = NewCommunityService(db)

	// Create a test user (owner)
	var owner1 User
	owner1.ID = primitive.NewObjectID()
	owner1.Name = testOwnerName
	owner1.Email = testOwnerEmail
	_, err = db.UserService.Collection.InsertOne(ctx, &owner1)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create a test community
	community, err := db.CommunityService.CreateCommunity(ctx, "Test Community", types.HexBytes{}, owner1.ID)
	if err != nil {
		t.Fatalf("Failed to create test community: %v", err)
	}

	// Initially, there should be no tools in the community
	count, err := db.CommunityService.CountCommunityTools(ctx, community.ID)
	if err != nil {
		t.Fatalf("Failed to count community tools: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 tools, got %d", count)
	}

	// Create a test tool
	var tool Tool
	tool.ID = 1
	tool.Title = testToolTitle
	tool.Description = testToolDesc
	tool.UserID = owner1.ID
	tool.Communities = []primitive.ObjectID{}
	_, err = db.ToolService.Collection.InsertOne(ctx, &tool)
	if err != nil {
		t.Fatalf("Failed to create test tool: %v", err)
	}

	// Add the tool to the community
	err = db.CommunityService.AddToolToCommunity(ctx, tool.ID, community.ID)
	if err != nil {
		t.Fatalf("Failed to add tool to community: %v", err)
	}

	// Now there should be 1 tool in the community
	count, err = db.CommunityService.CountCommunityTools(ctx, community.ID)
	if err != nil {
		t.Fatalf("Failed to count community tools: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 tool, got %d", count)
	}

	// Create another test tool
	var tool2 Tool
	tool2.ID = 2
	tool2.Title = "Another Test Tool"
	tool2.Description = "Another tool for testing"
	tool2.UserID = owner1.ID
	tool2.Communities = []primitive.ObjectID{}
	_, err = db.ToolService.Collection.InsertOne(ctx, &tool2)
	if err != nil {
		t.Fatalf("Failed to create second test tool: %v", err)
	}

	// Add the second tool to the community
	err = db.CommunityService.AddToolToCommunity(ctx, tool2.ID, community.ID)
	if err != nil {
		t.Fatalf("Failed to add second tool to community: %v", err)
	}

	// Now there should be 2 tools in the community
	count, err = db.CommunityService.CountCommunityTools(ctx, community.ID)
	if err != nil {
		t.Fatalf("Failed to count community tools: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 tools, got %d", count)
	}

	// Remove a tool from the community
	err = db.CommunityService.RemoveToolFromCommunity(ctx, tool.ID, community.ID)
	if err != nil {
		t.Fatalf("Failed to remove tool from community: %v", err)
	}

	// Now there should be 1 tool in the community
	count, err = db.CommunityService.CountCommunityTools(ctx, community.ID)
	if err != nil {
		t.Fatalf("Failed to count community tools: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 tool, got %d", count)
	}
}

func TestGetCommunityWithMemberCount(t *testing.T) {
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

	// Initialize Database with all services
	db := &Database{
		Client:   client,
		Database: database,
	}
	db.UserService = NewUserService(db)
	db.ToolService = NewToolService(db)
	db.CommunityService = NewCommunityService(db)

	// Create a test user (owner)
	var owner2 User
	owner2.ID = primitive.NewObjectID()
	owner2.Name = testOwnerName
	owner2.Email = testOwnerEmail
	_, err = db.UserService.Collection.InsertOne(ctx, &owner2)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create a test community
	community, err := db.CommunityService.CreateCommunity(ctx, "Test Community", types.HexBytes{}, owner2.ID)
	if err != nil {
		t.Fatalf("Failed to create test community: %v", err)
	}

	// Get community with member count and tool count
	comm, membersCount, toolsCount, err := db.CommunityService.GetCommunityWithMemberCount(ctx, community.ID)
	if err != nil {
		t.Fatalf("Failed to get community with member count: %v", err)
	}

	// Verify the community data
	if comm.ID != community.ID {
		t.Errorf("Expected community ID %s, got %s", community.ID.Hex(), comm.ID.Hex())
	}
	if comm.Name != "Test Community" {
		t.Errorf("Expected community name 'Test Community', got '%s'", comm.Name)
	}

	// Verify the member count (should be 1 - just the owner)
	if membersCount != 1 {
		t.Errorf("Expected 1 member, got %d", membersCount)
	}

	// Verify the tool count (should be 0 initially)
	if toolsCount != 0 {
		t.Errorf("Expected 0 tools, got %d", toolsCount)
	}

	// Create a test tool
	var tool Tool
	tool.ID = 1
	tool.Title = testToolTitle
	tool.Description = testToolDesc
	tool.UserID = owner2.ID
	tool.Communities = []primitive.ObjectID{}
	_, err = db.ToolService.Collection.InsertOne(ctx, &tool)
	if err != nil {
		t.Fatalf("Failed to create test tool: %v", err)
	}

	// Add the tool to the community
	err = db.CommunityService.AddToolToCommunity(ctx, tool.ID, community.ID)
	if err != nil {
		t.Fatalf("Failed to add tool to community: %v", err)
	}

	// Get community with member count and tool count again
	_, _, toolsCount, err = db.CommunityService.GetCommunityWithMemberCount(ctx, community.ID)
	if err != nil {
		t.Fatalf("Failed to get community with member count: %v", err)
	}

	// Verify the tool count (should be 1 now)
	if toolsCount != 1 {
		t.Errorf("Expected 1 tool, got %d", toolsCount)
	}
}

func TestGetUserCommunitiesWithMemberCount(t *testing.T) {
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

	// Initialize Database with all services
	db := &Database{
		Client:   client,
		Database: database,
	}
	db.UserService = NewUserService(db)
	db.ToolService = NewToolService(db)
	db.CommunityService = NewCommunityService(db)

	// Create a test user (owner)
	var owner3 User
	owner3.ID = primitive.NewObjectID()
	owner3.Name = testOwnerName
	owner3.Email = testOwnerEmail
	_, err = db.UserService.Collection.InsertOne(ctx, &owner3)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create a test community
	community, err := db.CommunityService.CreateCommunity(ctx, "Test Community", types.HexBytes{}, owner3.ID)
	if err != nil {
		t.Fatalf("Failed to create test community: %v", err)
	}

	// Get user communities with member count and tool count
	communities, memberCounts, toolCounts, err := db.CommunityService.GetUserCommunitiesWithMemberCount(ctx, owner3.ID, 0)
	if err != nil {
		t.Fatalf("Failed to get user communities with member count: %v", err)
	}

	// Verify the communities data
	if len(communities) != 1 {
		t.Fatalf("Expected 1 community, got %d", len(communities))
	}
	if communities[0].ID != community.ID {
		t.Errorf("Expected community ID %s, got %s", community.ID.Hex(), communities[0].ID.Hex())
	}
	if communities[0].Name != "Test Community" {
		t.Errorf("Expected community name 'Test Community', got '%s'", communities[0].Name)
	}

	// Verify the member count (should be 1 - just the owner)
	if memberCounts[community.ID] != 1 {
		t.Errorf("Expected 1 member, got %d", memberCounts[community.ID])
	}

	// Verify the tool count (should be 0 initially)
	if toolCounts[community.ID] != 0 {
		t.Errorf("Expected 0 tools, got %d", toolCounts[community.ID])
	}

	// Create a test tool
	var tool Tool
	tool.ID = 1
	tool.Title = testToolTitle
	tool.Description = testToolDesc
	tool.UserID = owner3.ID
	tool.Communities = []primitive.ObjectID{}
	_, err = db.ToolService.Collection.InsertOne(ctx, &tool)
	if err != nil {
		t.Fatalf("Failed to create test tool: %v", err)
	}

	// Add the tool to the community
	err = db.CommunityService.AddToolToCommunity(ctx, tool.ID, community.ID)
	if err != nil {
		t.Fatalf("Failed to add tool to community: %v", err)
	}

	// Get user communities with member count and tool count again
	_, _, toolCounts, err = db.CommunityService.GetUserCommunitiesWithMemberCount(ctx, owner3.ID, 0)
	if err != nil {
		t.Fatalf("Failed to get user communities with member count: %v", err)
	}

	// Verify the tool count (should be 1 now)
	if toolCounts[community.ID] != 1 {
		t.Errorf("Expected 1 tool, got %d", toolCounts[community.ID])
	}
}
