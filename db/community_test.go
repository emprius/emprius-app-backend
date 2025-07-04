package db

import (
	"context"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/types"
	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/bson"
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
	communities, memberCounts, toolCounts, _, err := db.CommunityService.GetUserCommunitiesWithMemberCount(ctx, owner3.ID, 0, "")
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
	_, _, toolCounts, _, err = db.CommunityService.GetUserCommunitiesWithMemberCount(ctx, owner3.ID, 0, "")
	if err != nil {
		t.Fatalf("Failed to get user communities with member count: %v", err)
	}

	// Verify the tool count (should be 1 now)
	if toolCounts[community.ID] != 1 {
		t.Errorf("Expected 1 tool, got %d", toolCounts[community.ID])
	}
}

func TestGetUserCommunities(t *testing.T) {
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

	communityService := db.CommunityService
	userService := db.UserService

	// Create test users
	user1 := &User{
		ID:       primitive.NewObjectID(),
		Email:    "user1@test.com",
		Name:     "User One",
		Password: []byte("password"),
		Tokens:   1000,
		Active:   true,
		Rating:   50,
		Location: NewLocation(40000000, 3000000), // Madrid coordinates in microdegrees
	}

	user2 := &User{
		ID:       primitive.NewObjectID(),
		Email:    "user2@test.com",
		Name:     "User Two",
		Password: []byte("password"),
		Tokens:   1000,
		Active:   true,
		Rating:   50,
		Location: NewLocation(41000000, 2000000), // Barcelona coordinates in microdegrees
	}

	// Insert users
	_, err = userService.InsertUser(ctx, user1)
	qt.Assert(t, err, qt.IsNil)
	_, err = userService.InsertUser(ctx, user2)
	qt.Assert(t, err, qt.IsNil)

	// Create test communities
	community1, err := communityService.CreateCommunity(
		ctx,
		"Community One",
		types.HexBytes{0x01, 0x02, 0x03},
		user1.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	community2, err := communityService.CreateCommunity(
		ctx,
		"Community Two",
		types.HexBytes{0x04, 0x05, 0x06},
		user1.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	community3, err := communityService.CreateCommunity(
		ctx,
		"Community Three",
		types.HexBytes{0x07, 0x08, 0x09},
		user2.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	// Add user1 to community3 as a regular user
	err = userService.AddUserToCommunity(ctx, user1.ID, community3.ID, CommunityRoleUser)
	qt.Assert(t, err, qt.IsNil)

	t.Run("GetUserCommunities returns correct communities and count for user1", func(t *testing.T) {
		communities, total, err := communityService.GetUserCommunities(ctx, user1.ID, 0, "")
		qt.Assert(t, err, qt.IsNil)

		// User1 should be in 3 communities (owner of 2, member of 1)
		qt.Assert(t, total, qt.Equals, int64(3))
		qt.Assert(t, len(communities), qt.Equals, 3)

		// Check that all expected communities are returned
		communityIDs := make(map[primitive.ObjectID]bool)
		for _, community := range communities {
			communityIDs[community.ID] = true
		}

		qt.Assert(t, communityIDs[community1.ID], qt.IsTrue)
		qt.Assert(t, communityIDs[community2.ID], qt.IsTrue)
		qt.Assert(t, communityIDs[community3.ID], qt.IsTrue)
	})

	t.Run("GetUserCommunities returns correct communities and count for user2", func(t *testing.T) {
		communities, total, err := communityService.GetUserCommunities(ctx, user2.ID, 0, "")
		qt.Assert(t, err, qt.IsNil)

		// User2 should be in 1 community (owner of 1)
		qt.Assert(t, total, qt.Equals, int64(1))
		qt.Assert(t, len(communities), qt.Equals, 1)
		qt.Assert(t, communities[0].ID, qt.Equals, community3.ID)
	})

	t.Run("GetUserCommunities with pagination", func(t *testing.T) {
		// Test pagination with page size 2
		communities, total, err := communityService.GetUserCommunities(ctx, user1.ID, 0, "")
		qt.Assert(t, err, qt.IsNil)

		// Total should still be 3, but we should get at most DefaultPageSize communities
		qt.Assert(t, total, qt.Equals, int64(3))
		qt.Assert(t, len(communities) <= DefaultPageSize, qt.IsTrue)

		// Test second page (should be empty if DefaultPageSize >= 3)
		if DefaultPageSize < 3 {
			communities2, total2, err := communityService.GetUserCommunities(ctx, user1.ID, 1, "")
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, total2, qt.Equals, int64(3)) // Total should remain the same
			qt.Assert(t, len(communities2) <= 3-DefaultPageSize, qt.IsTrue)
		}
	})

	t.Run("GetUserCommunities for non-existent user", func(t *testing.T) {
		nonExistentUserID := primitive.NewObjectID()
		communities, total, err := communityService.GetUserCommunities(ctx, nonExistentUserID, 0, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(communities), qt.Equals, 0)
	})

	t.Run("GetUserCommunities with negative page", func(t *testing.T) {
		communities, total, err := communityService.GetUserCommunities(ctx, user1.ID, -1, "")
		qt.Assert(t, err, qt.IsNil)

		// Should treat negative page as page 0
		qt.Assert(t, total, qt.Equals, int64(3))
		qt.Assert(t, len(communities), qt.Equals, 3)
	})

	t.Run("Communities are sorted by name", func(t *testing.T) {
		communities, _, err := communityService.GetUserCommunities(ctx, user1.ID, 0, "")
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(communities), qt.Equals, 3)

		// Check that communities are sorted by name
		for i := 1; i < len(communities); i++ {
			qt.Assert(t, communities[i-1].Name <= communities[i].Name, qt.IsTrue)
		}
	})
}

func TestGetUserCommunitiesWithSearch(t *testing.T) {
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

	communityService := db.CommunityService
	userService := db.UserService

	// Create test user
	user := &User{
		ID:       primitive.NewObjectID(),
		Email:    "search@test.com",
		Name:     "Search User",
		Password: []byte("password"),
		Tokens:   1000,
		Active:   true,
		Rating:   50,
		Location: NewLocation(40000000, 3000000),
	}

	_, err = userService.InsertUser(ctx, user)
	qt.Assert(t, err, qt.IsNil)

	// Create test communities with different names
	_, err = communityService.CreateCommunity(
		ctx,
		"Tech Community",
		types.HexBytes{0x01, 0x02, 0x03},
		user.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	_, err = communityService.CreateCommunity(
		ctx,
		"Sports Club",
		types.HexBytes{0x04, 0x05, 0x06},
		user.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	_, err = communityService.CreateCommunity(
		ctx,
		"Technology Enthusiasts",
		types.HexBytes{0x07, 0x08, 0x09},
		user.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	_, err = communityService.CreateCommunity(
		ctx,
		"Book Reading Group",
		types.HexBytes{0x0A, 0x0B, 0x0C},
		user.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	t.Run("Search with partial match 'tech'", func(t *testing.T) {
		communities, total, err := communityService.GetUserCommunities(ctx, user.ID, 0, "tech")
		qt.Assert(t, err, qt.IsNil)

		// Should find "Tech Community" and "Technology Enthusiasts"
		qt.Assert(t, total, qt.Equals, int64(2))
		qt.Assert(t, len(communities), qt.Equals, 2)

		// Check that the correct communities are returned
		foundNames := make(map[string]bool)
		for _, community := range communities {
			foundNames[community.Name] = true
		}

		qt.Assert(t, foundNames["Tech Community"], qt.IsTrue)
		qt.Assert(t, foundNames["Technology Enthusiasts"], qt.IsTrue)
	})

	t.Run("Search with case insensitive match 'SPORTS'", func(t *testing.T) {
		communities, total, err := communityService.GetUserCommunities(ctx, user.ID, 0, "SPORTS")
		qt.Assert(t, err, qt.IsNil)

		// Should find "Sports Club"
		qt.Assert(t, total, qt.Equals, int64(1))
		qt.Assert(t, len(communities), qt.Equals, 1)
		qt.Assert(t, communities[0].Name, qt.Equals, "Sports Club")
	})

	t.Run("Search with exact match 'Book Reading Group'", func(t *testing.T) {
		communities, total, err := communityService.GetUserCommunities(ctx, user.ID, 0, "Book Reading Group")
		qt.Assert(t, err, qt.IsNil)

		// Should find "Book Reading Group"
		qt.Assert(t, total, qt.Equals, int64(1))
		qt.Assert(t, len(communities), qt.Equals, 1)
		qt.Assert(t, communities[0].Name, qt.Equals, "Book Reading Group")
	})

	t.Run("Search with no matches", func(t *testing.T) {
		communities, total, err := communityService.GetUserCommunities(ctx, user.ID, 0, "nonexistent")
		qt.Assert(t, err, qt.IsNil)

		// Should find no communities
		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(communities), qt.Equals, 0)
	})

	t.Run("Search with empty string returns all", func(t *testing.T) {
		communities, total, err := communityService.GetUserCommunities(ctx, user.ID, 0, "")
		qt.Assert(t, err, qt.IsNil)

		// Should find all 4 communities
		qt.Assert(t, total, qt.Equals, int64(4))
		qt.Assert(t, len(communities), qt.Equals, 4)
	})

	t.Run("Search with special characters", func(t *testing.T) {
		// Create a community with special characters
		specialCommunity, err := communityService.CreateCommunity(
			ctx,
			"C++ Developers",
			types.HexBytes{0x0D, 0x0E, 0x0F},
			user.ID,
		)
		qt.Assert(t, err, qt.IsNil)

		communities, total, err := communityService.GetUserCommunities(ctx, user.ID, 0, "C++")
		qt.Assert(t, err, qt.IsNil)

		// Should find the special community (note: there might be other communities from previous tests)
		qt.Assert(t, total >= int64(1), qt.IsTrue)
		qt.Assert(t, len(communities) >= 1, qt.IsTrue)

		// Check that the special community is in the results
		found := false
		for _, community := range communities {
			if community.ID == specialCommunity.ID {
				found = true
				break
			}
		}
		qt.Assert(t, found, qt.IsTrue)
	})

	t.Run("Search results are sorted by name", func(t *testing.T) {
		communities, _, err := communityService.GetUserCommunities(ctx, user.ID, 0, "")
		qt.Assert(t, err, qt.IsNil)

		// Check that communities are sorted by name
		for i := 1; i < len(communities); i++ {
			qt.Assert(t, communities[i-1].Name <= communities[i].Name, qt.IsTrue)
		}
	})
}

func TestGetUserCommunitiesWithMemberCountAndSearch(t *testing.T) {
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

	communityService := db.CommunityService
	userService := db.UserService

	// Create test user
	user := &User{
		ID:       primitive.NewObjectID(),
		Email:    "searchcount@test.com",
		Name:     "Search Count User",
		Password: []byte("password"),
		Tokens:   1000,
		Active:   true,
		Rating:   50,
		Location: NewLocation(40000000, 3000000),
	}

	_, err = userService.InsertUser(ctx, user)
	qt.Assert(t, err, qt.IsNil)

	// Create test communities
	community1, err := communityService.CreateCommunity(
		ctx,
		"Development Team",
		types.HexBytes{0x01, 0x02, 0x03},
		user.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	_, err = communityService.CreateCommunity(
		ctx,
		"Marketing Group",
		types.HexBytes{0x04, 0x05, 0x06},
		user.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	t.Run("Search with member count returns correct data", func(t *testing.T) {
		communities, memberCounts, toolCounts, total, err := communityService.GetUserCommunitiesWithMemberCount(ctx, user.ID, 0, "dev")
		qt.Assert(t, err, qt.IsNil)

		// Should find "Development Team"
		qt.Assert(t, total, qt.Equals, int64(1))
		qt.Assert(t, len(communities), qt.Equals, 1)
		qt.Assert(t, communities[0].Name, qt.Equals, "Development Team")

		// Check member and tool counts
		qt.Assert(t, memberCounts[community1.ID], qt.Equals, int64(1)) // Just the owner
		qt.Assert(t, toolCounts[community1.ID], qt.Equals, int64(0))   // No tools initially
	})

	t.Run("Search with no matches returns empty with counts", func(t *testing.T) {
		communities, memberCounts, toolCounts, total, err := communityService.GetUserCommunitiesWithMemberCount(
			ctx,
			user.ID,
			0, "nonexistent",
		)
		qt.Assert(t, err, qt.IsNil)

		// Should find no communities
		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(communities), qt.Equals, 0)
		qt.Assert(t, len(memberCounts), qt.Equals, 0)
		qt.Assert(t, len(toolCounts), qt.Equals, 0)
	})
}

func TestGetUserCommunitiesEdgeCases(t *testing.T) {
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

	communityService := db.CommunityService
	userService := db.UserService

	t.Run("User with no communities", func(t *testing.T) {
		user := &User{
			ID:       primitive.NewObjectID(),
			Email:    "lonely@test.com",
			Name:     "Lonely User",
			Password: []byte("password"),
			Tokens:   1000,
			Active:   true,
			Rating:   50,
			Location: NewLocation(40000000, 3000000),
		}

		_, err := userService.InsertUser(ctx, user)
		qt.Assert(t, err, qt.IsNil)

		communities, total, err := communityService.GetUserCommunities(ctx, user.ID, 0, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(communities), qt.Equals, 0)
	})

	t.Run("User removed from community", func(t *testing.T) {
		// Create user and community
		user := &User{
			ID:       primitive.NewObjectID(),
			Email:    "removed@test.com",
			Name:     "Removed User",
			Password: []byte("password"),
			Tokens:   1000,
			Active:   true,
			Rating:   50,
			Location: NewLocation(40000000, 3000000),
		}

		_, err := userService.InsertUser(ctx, user)
		qt.Assert(t, err, qt.IsNil)

		community, err := communityService.CreateCommunity(
			ctx,
			"Test Community",
			types.HexBytes{0x01, 0x02, 0x03},
			user.ID,
		)
		qt.Assert(t, err, qt.IsNil)

		// Verify user is in community
		communities, total, err := communityService.GetUserCommunities(ctx, user.ID, 0, "")
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, total, qt.Equals, int64(1))
		qt.Assert(t, len(communities), qt.Equals, 1)

		// Remove user from community (this should succeed even though user is owner)
		err = userService.RemoveUserFromCommunity(ctx, user.ID, community.ID)
		qt.Assert(t, err, qt.IsNil)

		// Check communities again - user should have no communities after removal
		communities, total, err = communityService.GetUserCommunities(ctx, user.ID, 0, "")
		qt.Assert(t, err, qt.IsNil)
		// The count should reflect the actual state - user should have no communities after removal
		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(communities), qt.Equals, 0)
	})
}

func TestGetUserCommunitiesPerformance(t *testing.T) {
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

	communityService := db.CommunityService
	userService := db.UserService

	// Create a user
	user := &User{
		ID:       primitive.NewObjectID(),
		Email:    "perf@test.com",
		Name:     "Performance User",
		Password: []byte("password"),
		Tokens:   1000,
		Active:   true,
		Rating:   50,
		Location: NewLocation(40000000, 3000000),
	}

	_, err = userService.InsertUser(ctx, user)
	qt.Assert(t, err, qt.IsNil)

	// Create multiple communities for the user
	numCommunities := 25
	for i := 0; i < numCommunities; i++ {
		_, err := communityService.CreateCommunity(
			ctx,
			"Community "+string(rune('A'+i)),
			types.HexBytes{byte(i), byte(i + 1), byte(i + 2)},
			user.ID,
		)
		qt.Assert(t, err, qt.IsNil)
	}

	t.Run("Performance test with many communities", func(t *testing.T) {
		start := time.Now()
		communities, total, err := communityService.GetUserCommunities(ctx, user.ID, 0, "")
		duration := time.Since(start)

		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, total, qt.Equals, int64(numCommunities))

		// Should return at most DefaultPageSize communities
		expectedLen := numCommunities
		if DefaultPageSize < numCommunities {
			expectedLen = DefaultPageSize
		}
		qt.Assert(t, len(communities), qt.Equals, expectedLen)

		// Performance should be reasonable (less than 1 second for this test)
		qt.Assert(t, duration < time.Second, qt.IsTrue, qt.Commentf("Query took too long: %v", duration))

		t.Logf("Query took %v for %d communities", duration, numCommunities)
	})

	t.Run("Pagination consistency", func(t *testing.T) {
		// Get all communities across multiple pages
		var allCommunities []*Community
		page := 0
		var totalFromFirstPage int64

		for {
			communities, total, err := communityService.GetUserCommunities(ctx, user.ID, page, "")
			qt.Assert(t, err, qt.IsNil)

			if page == 0 {
				totalFromFirstPage = total
			} else {
				// Total should be consistent across pages
				qt.Assert(t, total, qt.Equals, totalFromFirstPage)
			}

			if len(communities) == 0 {
				break
			}

			allCommunities = append(allCommunities, communities...)
			page++

			// Safety check to avoid infinite loop
			if page > 10 {
				break
			}
		}

		// Should have collected all communities
		qt.Assert(t, len(allCommunities), qt.Equals, numCommunities)
		qt.Assert(t, totalFromFirstPage, qt.Equals, int64(numCommunities))
	})
}

func TestGetCommunityToolsPaginatedWithActiveUsers(t *testing.T) {
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

	communityService := db.CommunityService
	userService := db.UserService
	toolService := db.ToolService

	// Create test users - one active, one inactive
	activeUser := &User{
		ID:       primitive.NewObjectID(),
		Email:    "active@test.com",
		Name:     "Active User",
		Password: []byte("password"),
		Tokens:   1000,
		Active:   true,
		Rating:   50,
		Location: NewLocation(40000000, 3000000),
	}

	inactiveUser := &User{
		ID:       primitive.NewObjectID(),
		Email:    "inactive@test.com",
		Name:     "Inactive User",
		Password: []byte("password"),
		Tokens:   1000,
		Active:   false, // This user is inactive
		Rating:   50,
		Location: NewLocation(41000000, 2000000),
	}

	_, err = userService.InsertUser(ctx, activeUser)
	qt.Assert(t, err, qt.IsNil)
	_, err = userService.InsertUser(ctx, inactiveUser)
	qt.Assert(t, err, qt.IsNil)

	// Create a test community
	community, err := communityService.CreateCommunity(
		ctx,
		"Test Community",
		types.HexBytes{0x01, 0x02, 0x03},
		activeUser.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	// Add inactive user to community
	err = userService.AddUserToCommunity(ctx, inactiveUser.ID, community.ID, CommunityRoleUser)
	qt.Assert(t, err, qt.IsNil)

	// Create tools - some from active user, some from inactive user
	activeUserTool1 := &Tool{
		ID:          1,
		Title:       "Active User Tool 1",
		Description: "Tool from active user",
		UserID:      activeUser.ID,
		Communities: []primitive.ObjectID{},
	}

	activeUserTool2 := &Tool{
		ID:          2,
		Title:       "Active User Tool 2",
		Description: "Another tool from active user",
		UserID:      activeUser.ID,
		Communities: []primitive.ObjectID{},
	}

	inactiveUserTool1 := &Tool{
		ID:          3,
		Title:       "Inactive User Tool 1",
		Description: "Tool from inactive user",
		UserID:      inactiveUser.ID,
		Communities: []primitive.ObjectID{},
	}

	inactiveUserTool2 := &Tool{
		ID:          4,
		Title:       "Inactive User Tool 2",
		Description: "Another tool from inactive user",
		UserID:      inactiveUser.ID,
		Communities: []primitive.ObjectID{},
	}

	// Insert tools
	_, err = toolService.Collection.InsertOne(ctx, activeUserTool1)
	qt.Assert(t, err, qt.IsNil)
	_, err = toolService.Collection.InsertOne(ctx, activeUserTool2)
	qt.Assert(t, err, qt.IsNil)
	_, err = toolService.Collection.InsertOne(ctx, inactiveUserTool1)
	qt.Assert(t, err, qt.IsNil)
	_, err = toolService.Collection.InsertOne(ctx, inactiveUserTool2)
	qt.Assert(t, err, qt.IsNil)

	// Add all tools to the community
	err = communityService.AddToolToCommunity(ctx, activeUserTool1.ID, community.ID)
	qt.Assert(t, err, qt.IsNil)
	err = communityService.AddToolToCommunity(ctx, activeUserTool2.ID, community.ID)
	qt.Assert(t, err, qt.IsNil)
	err = communityService.AddToolToCommunity(ctx, inactiveUserTool1.ID, community.ID)
	qt.Assert(t, err, qt.IsNil)
	err = communityService.AddToolToCommunity(ctx, inactiveUserTool2.ID, community.ID)
	qt.Assert(t, err, qt.IsNil)

	t.Run("Only tools from active users are returned", func(t *testing.T) {
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, 10, "")
		qt.Assert(t, err, qt.IsNil)

		// Should only return tools from active users (2 tools)
		qt.Assert(t, total, qt.Equals, int64(2))
		qt.Assert(t, len(tools), qt.Equals, 2)

		// Verify that only active user tools are returned
		toolIDs := make(map[int64]bool)
		for _, tool := range tools {
			toolIDs[tool.ID] = true
		}

		qt.Assert(t, toolIDs[activeUserTool1.ID], qt.IsTrue)
		qt.Assert(t, toolIDs[activeUserTool2.ID], qt.IsTrue)
		qt.Assert(t, toolIDs[inactiveUserTool1.ID], qt.IsFalse)
		qt.Assert(t, toolIDs[inactiveUserTool2.ID], qt.IsFalse)
	})

	t.Run("Search functionality works with active user filtering", func(t *testing.T) {
		// Search for "Active" - should find tools from active user only
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, 10, "Active")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(2))
		qt.Assert(t, len(tools), qt.Equals, 2)

		// All returned tools should be from active user
		for _, tool := range tools {
			qt.Assert(t, tool.UserID, qt.Equals, activeUser.ID)
		}

		// Search for "Inactive" - should find no tools (inactive user tools are filtered out)
		tools, total, err = communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, 10, "Inactive")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(tools), qt.Equals, 0)
	})

	t.Run("Pagination works correctly with filtering", func(t *testing.T) {
		// Test pagination with page size 1
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, 1, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(2)) // Total should still be 2
		qt.Assert(t, len(tools), qt.Equals, 1)   // But only 1 tool per page

		// Get second page
		tools2, total2, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 1, 1, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total2, qt.Equals, int64(2)) // Total should still be 2
		qt.Assert(t, len(tools2), qt.Equals, 1)   // Second page should have 1 tool

		// Verify we got different tools
		qt.Assert(t, tools[0].ID != tools2[0].ID, qt.IsTrue)

		// Third page should be empty
		tools3, total3, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 2, 1, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total3, qt.Equals, int64(2)) // Total should still be 2
		qt.Assert(t, len(tools3), qt.Equals, 0)   // Third page should be empty
	})

	t.Run("Tools are sorted by title", func(t *testing.T) {
		tools, _, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, 10, "")
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, len(tools), qt.Equals, 2)

		// Tools should be sorted by title
		qt.Assert(t, tools[0].Title <= tools[1].Title, qt.IsTrue)
	})
}

func TestGetCommunityToolsPaginatedEdgeCases(t *testing.T) {
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

	communityService := db.CommunityService
	userService := db.UserService

	// Create test user
	user := &User{
		ID:       primitive.NewObjectID(),
		Email:    "edge@test.com",
		Name:     "Edge Case User",
		Password: []byte("password"),
		Tokens:   1000,
		Active:   true,
		Rating:   50,
		Location: NewLocation(40000000, 3000000),
	}

	_, err = userService.InsertUser(ctx, user)
	qt.Assert(t, err, qt.IsNil)

	// Create a test community
	community, err := communityService.CreateCommunity(
		ctx,
		"Edge Case Community",
		types.HexBytes{0x01, 0x02, 0x03},
		user.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	t.Run("Empty community returns no tools", func(t *testing.T) {
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, 10, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(tools), qt.Equals, 0)
	})

	t.Run("Community with only inactive user tools returns no tools", func(t *testing.T) {
		// Create inactive user
		inactiveUser := &User{
			ID:       primitive.NewObjectID(),
			Email:    "inactive-edge@test.com",
			Name:     "Inactive Edge User",
			Password: []byte("password"),
			Tokens:   1000,
			Active:   false,
			Rating:   50,
			Location: NewLocation(41000000, 2000000),
		}

		_, err := userService.InsertUser(ctx, inactiveUser)
		qt.Assert(t, err, qt.IsNil)

		// Add inactive user to community
		err = userService.AddUserToCommunity(ctx, inactiveUser.ID, community.ID, CommunityRoleUser)
		qt.Assert(t, err, qt.IsNil)

		// Create tool from inactive user
		inactiveTool := &Tool{
			ID:          100,
			Title:       "Inactive Tool",
			Description: "Tool from inactive user",
			UserID:      inactiveUser.ID,
			Communities: []primitive.ObjectID{},
		}

		_, err = db.ToolService.Collection.InsertOne(ctx, inactiveTool)
		qt.Assert(t, err, qt.IsNil)

		// Add tool to community
		err = communityService.AddToolToCommunity(ctx, inactiveTool.ID, community.ID)
		qt.Assert(t, err, qt.IsNil)

		// Should return no tools since the only tool is from inactive user
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, 10, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(tools), qt.Equals, 0)
	})

	t.Run("Negative page number is handled correctly", func(t *testing.T) {
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, -1, 10, "")
		qt.Assert(t, err, qt.IsNil)

		// Should treat negative page as page 0
		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(tools), qt.Equals, 0)
	})

	t.Run("Negative page size uses default", func(t *testing.T) {
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, -1, "")
		qt.Assert(t, err, qt.IsNil)

		// Should use default page size
		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(tools), qt.Equals, 0)
	})

	t.Run("Non-existent community returns no tools", func(t *testing.T) {
		nonExistentCommunityID := primitive.NewObjectID()
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, nonExistentCommunityID, 0, 10, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(tools), qt.Equals, 0)
	})
}

func TestGetCommunityToolsPaginatedUserStatusChange(t *testing.T) {
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

	communityService := db.CommunityService
	userService := db.UserService
	toolService := db.ToolService

	// Create test user (initially active)
	user := &User{
		ID:       primitive.NewObjectID(),
		Email:    "status@test.com",
		Name:     "Status Change User",
		Password: []byte("password"),
		Tokens:   1000,
		Active:   true,
		Rating:   50,
		Location: NewLocation(40000000, 3000000),
	}

	_, err = userService.InsertUser(ctx, user)
	qt.Assert(t, err, qt.IsNil)

	// Create a test community
	community, err := communityService.CreateCommunity(
		ctx,
		"Status Test Community",
		types.HexBytes{0x01, 0x02, 0x03},
		user.ID,
	)
	qt.Assert(t, err, qt.IsNil)

	// Create a tool from the user
	tool := &Tool{
		ID:          200,
		Title:       "Status Test Tool",
		Description: "Tool for testing status changes",
		UserID:      user.ID,
		Communities: []primitive.ObjectID{},
	}

	_, err = toolService.Collection.InsertOne(ctx, tool)
	qt.Assert(t, err, qt.IsNil)

	// Add tool to community
	err = communityService.AddToolToCommunity(ctx, tool.ID, community.ID)
	qt.Assert(t, err, qt.IsNil)

	t.Run("Tool is visible when user is active", func(t *testing.T) {
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, 10, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(1))
		qt.Assert(t, len(tools), qt.Equals, 1)
		qt.Assert(t, tools[0].ID, qt.Equals, tool.ID)
	})

	t.Run("Tool is hidden when user becomes inactive", func(t *testing.T) {
		// Deactivate the user
		_, err := userService.UpdateUser(ctx, user.ID, bson.M{"active": false})
		qt.Assert(t, err, qt.IsNil)

		// Tool should no longer be visible
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, 10, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(0))
		qt.Assert(t, len(tools), qt.Equals, 0)
	})

	t.Run("Tool becomes visible again when user is reactivated", func(t *testing.T) {
		// Reactivate the user
		_, err := userService.UpdateUser(ctx, user.ID, bson.M{"active": true})
		qt.Assert(t, err, qt.IsNil)

		// Tool should be visible again
		tools, total, err := communityService.GetCommunityToolsPaginated(ctx, community.ID, 0, 10, "")
		qt.Assert(t, err, qt.IsNil)

		qt.Assert(t, total, qt.Equals, int64(1))
		qt.Assert(t, len(tools), qt.Equals, 1)
		qt.Assert(t, tools[0].ID, qt.Equals, tool.ID)
	})
}
