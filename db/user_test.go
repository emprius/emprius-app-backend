package db

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestUserService(t *testing.T) {
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

	// Initialize UserService
	userService := NewUserService(&Database{
		Client:   client,
		Database: database,
	})

	// Test UserService methods
	c.Run("Insert and Retrieve User", func(c *qt.C) {
		user := &User{
			Email:      "test@example.com",
			Name:       "Test User",
			Community:  "Test Community",
			Password:   []byte("hashedpassword"),
			Tokens:     100,
			Active:     true,
			Rating:     80,
			AvatarHash: []byte("avatarhash"),
			Location: DBLocation{
				Type: "Point",
				Coordinates: []float64{
					2.492793,  // longitude
					41.695384, // latitude
				},
			},
			Verified: true,
		}

		// Insert User
		insertResult, err := userService.InsertUser(ctx, user)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert user"))
		c.Assert(insertResult.InsertedID, qt.Not(qt.IsNil), qt.Commentf("Insert result ID is nil"))

		// Retrieve User by Email
		retrievedUser, err := userService.GetUserByEmail(ctx, user.Email)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve user by email"))
		c.Assert(retrievedUser.Email, qt.Equals, user.Email, qt.Commentf("Emails do not match"))
		c.Assert(retrievedUser.Name, qt.Equals, user.Name, qt.Commentf("Names do not match"))
		c.Assert(retrievedUser.Location, qt.DeepEquals, user.Location, qt.Commentf("Locations do not match"))
	})

	c.Run("Update User", func(c *qt.C) {
		// Insert initial user
		user := &User{
			Email:     "update@example.com",
			Name:      "Update Test",
			Community: "Update Community",
			Password:  []byte("updatepassword"),
			Tokens:    50,
			Active:    true,
			Rating:    70,
			Location: DBLocation{
				Type: "Point",
				Coordinates: []float64{
					2.492793,  // longitude
					41.695384, // latitude
				},
			},
			Verified: false,
		}

		insertResult, err := userService.InsertUser(ctx, user)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert user for update test"))

		// Update user fields
		userID := insertResult.InsertedID.(primitive.ObjectID)
		update := bson.M{
			"name":      "Updated Name",
			"community": "Updated Community",
			"tokens":    75,
			"rating":    85,
		}

		updateResult, err := userService.UpdateUser(ctx, userID, update)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to update user"))
		c.Assert(updateResult.ModifiedCount, qt.Equals, int64(1), qt.Commentf("Expected 1 document to be modified"))

		// Verify update
		updatedUser, err := userService.GetUserByEmail(ctx, user.Email)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve updated user"))
		c.Assert(updatedUser.Name, qt.Equals, "Updated Name", qt.Commentf("Name was not updated"))
		c.Assert(updatedUser.Community, qt.Equals, "Updated Community", qt.Commentf("Community was not updated"))
		c.Assert(updatedUser.Tokens, qt.Equals, uint64(75), qt.Commentf("Tokens were not updated"))
		c.Assert(updatedUser.Rating, qt.Equals, int32(85), qt.Commentf("Rating was not updated"))
	})

	c.Run("Get All Communities", func(c *qt.C) {
		// Insert additional users
		users := []*User{
			{
				Email:    "user1@example.com",
				Name:     "User One",
				Password: []byte("pass1"),
				Active:   true,
			},
			{
				Email:    "user2@example.com",
				Name:     "User Two",
				Password: []byte("pass2"),
				Active:   true,
			},
		}

		for _, u := range users {
			_, err := userService.InsertUser(ctx, u)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert test user"))
		}

		// Retrieve first page of users
		allUsers, _, err := userService.GetUsers(ctx, "", 0)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve users"))
		c.Assert(len(allUsers) >= 3, qt.Equals, true, qt.Commentf("Expected at least 3 users in first page"))
	})

	c.Run("Delete User", func(c *qt.C) {
		// Insert user to delete
		user := &User{
			Email:    "delete@example.com",
			Name:     "Delete Test",
			Password: []byte("deletepass"),
			Active:   true,
		}

		insertResult, err := userService.InsertUser(ctx, user)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert user for deletion test"))

		userID := insertResult.InsertedID.(primitive.ObjectID)

		// Delete user
		deleteResult, err := userService.DeleteUser(ctx, userID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to delete user"))
		c.Assert(deleteResult.DeletedCount, qt.Equals, int64(1), qt.Commentf("Expected 1 document to be deleted"))

		// Verify deletion
		_, err = userService.GetUserByEmail(ctx, user.Email)
		c.Assert(err, qt.Not(qt.IsNil), qt.Commentf("Expected error when retrieving deleted user"))
		c.Assert(err, qt.Equals, mongo.ErrNoDocuments, qt.Commentf("Expected no documents error"))
	})

	c.Run("Get Communities By Partial Name", func(c *qt.C) {
		// Insert users with different names for testing partial name search
		usersToInsert := []*User{
			{
				Email:    "john.doe@example.com",
				Name:     "John Doe",
				Password: []byte("password1"),
				Active:   true,
			},
			{
				Email:    "jane.doe@example.com",
				Name:     "Jane Doe",
				Password: []byte("password2"),
				Active:   true,
			},
			{
				Email:    "alice.smith@example.com",
				Name:     "Alice Smith",
				Password: []byte("password3"),
				Active:   true,
			},
			{
				Email:    "bob.johnson@example.com",
				Name:     "Bob Johnson",
				Password: []byte("password4"),
				Active:   true,
			},
		}

		for _, u := range usersToInsert {
			_, err := userService.InsertUser(ctx, u)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert test user"))
		}

		// Test partial name search with "doe" - should match "John Doe" and "Jane Doe"
		doeUsers, doeTotal, err := userService.GetUsers(ctx, "doe", 0)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get users by partial name"))

		// Verify we got at least 2 users with "Doe" in their name
		doeCount := 0
		for _, u := range doeUsers {
			if u.Name == "John Doe" || u.Name == "Jane Doe" {
				doeCount++
			}
		}
		c.Assert(doeCount >= 2, qt.Equals, true, qt.Commentf("Expected at least 2 users with 'Doe' in their name"))
		c.Assert(doeTotal >= 2, qt.Equals, true, qt.Commentf("Expected total count to be at least 2"))

		// Test partial name search with "smith" - should match "Alice Smith"
		smithUsers, smithTotal, err := userService.GetUsers(ctx, "smith", 0)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get users by partial name"))

		// Verify we got at least 1 user with "Smith" in their name
		smithFound := false
		for _, u := range smithUsers {
			if u.Name == "Alice Smith" {
				smithFound = true
				break
			}
		}
		c.Assert(smithFound, qt.Equals, true, qt.Commentf("Expected to find user with 'Smith' in their name"))
		c.Assert(smithTotal >= 1, qt.Equals, true, qt.Commentf("Expected total count to be at least 1"))

		// Test pagination by getting first page with limit 1
		// This is a bit tricky to test directly since we're using a constant for page size
		// Instead, we'll just verify that calling with different page numbers returns different results
		page0Users, page0Total, err := userService.GetUsers(ctx, "doe", 0)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get users by partial name (page 0)"))

		// Test with invalid page number (should default to page 0)
		invalidPageUsers, invalidPageTotal, err := userService.GetUsers(ctx, "doe", -1)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get users by partial name (invalid page)"))
		c.Assert(len(invalidPageUsers), qt.Equals, len(page0Users), qt.Commentf("Expected same results for invalid page and page 0"))
		c.Assert(invalidPageTotal, qt.Equals, page0Total, qt.Commentf("Expected same total count for invalid page and page 0"))

		// Test empty string search - should return normal results
		emptyUsers, emptyTotal, err := userService.GetUsers(ctx, "", 0)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get users by empty partial name"))
		c.Assert(len(emptyUsers), qt.Equals, 8, qt.Commentf("Expected 8 users for empty search"))
		c.Assert(emptyTotal, qt.Equals, int64(8), qt.Commentf("Expected total count to be 8 for empty search"))

		// Test whitespace-only string search - should return empty results
		whitespaceUsers, whitespaceTotal, err := userService.GetUsers(ctx, "   ", 0)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get users by whitespace partial name"))
		c.Assert(len(whitespaceUsers), qt.Equals, 0, qt.Commentf("Expected no users for whitespace search"))
		c.Assert(whitespaceTotal, qt.Equals, int64(0), qt.Commentf("Expected total count to be 0 for whitespace search"))
	})
}

func TestUpdateUser_LocationUpdate(t *testing.T) {
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

	// Initialize services
	db := &Database{
		Client:   client,
		Database: database,
	}
	userService := NewUserService(db)
	toolService := NewToolService(db)

	c.Run("Updates Owned Tools", func(c *qt.C) {
		// Create a user with a specific location
		userLocation := NewLocation(41385063, 2173404) // Barcelona coordinates
		user := &User{
			Email:    "test@example.com",
			Name:     "Test User",
			Location: userLocation,
			Salt:     "testsalt",
			Active:   true,
		}

		userResult, err := userService.InsertUser(ctx, user)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert user"))
		userID := userResult.InsertedID.(primitive.ObjectID)

		// Create tools owned by the user with the same location
		tool1 := &Tool{
			ID:       1,
			Title:    "Tool 1",
			UserID:   userID,
			Location: userLocation,
		}
		tool2 := &Tool{
			ID:       2,
			Title:    "Tool 2",
			UserID:   userID,
			Location: userLocation,
		}
		// Create a tool with different location (should not be updated)
		differentLocation := NewLocation(40416775, -3703790) // Madrid coordinates
		tool3 := &Tool{
			ID:       3,
			Title:    "Tool 3",
			UserID:   userID,
			Location: differentLocation,
		}

		_, err = toolService.InsertTool(ctx, tool1)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert tool1"))
		_, err = toolService.InsertTool(ctx, tool2)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert tool2"))
		_, err = toolService.InsertTool(ctx, tool3)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert tool3"))

		// Update user location
		newLocation := NewLocation(48856614, 2352222) // Paris coordinates
		update := bson.M{
			"location": newLocation,
		}

		_, err = userService.UpdateUser(ctx, userID, update)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to update user location"))

		// Verify that tools with matching location were updated
		updatedTool1, err := toolService.GetToolByID(ctx, 1)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get updated tool1"))
		c.Assert(updatedTool1.Location.Coordinates, qt.DeepEquals, newLocation.Coordinates, qt.Commentf("Tool1 location not updated"))

		updatedTool2, err := toolService.GetToolByID(ctx, 2)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get updated tool2"))
		c.Assert(updatedTool2.Location.Coordinates, qt.DeepEquals, newLocation.Coordinates, qt.Commentf("Tool2 location not updated"))

		// Verify that tool with different location was not updated
		updatedTool3, err := toolService.GetToolByID(ctx, 3)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get updated tool3"))
		c.Assert(updatedTool3.Location.Coordinates, qt.DeepEquals, differentLocation.Coordinates, qt.Commentf("Tool3 location should not have been updated"))
	})

	c.Run("Updates Nomadic Tools", func(c *qt.C) {
		// Create two users
		userLocation := NewLocation(41385063, 2173404) // Barcelona coordinates
		user1 := &User{
			Email:    "user1@example.com",
			Name:     "User 1",
			Location: userLocation,
			Salt:     "testsalt1",
			Active:   true,
		}
		user2 := &User{
			Email:    "user2@example.com",
			Name:     "User 2",
			Location: userLocation,
			Salt:     "testsalt2",
			Active:   true,
		}

		user1Result, err := userService.InsertUser(ctx, user1)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert user1"))
		user1ID := user1Result.InsertedID.(primitive.ObjectID)

		user2Result, err := userService.InsertUser(ctx, user2)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert user2"))
		user2ID := user2Result.InsertedID.(primitive.ObjectID)

		// Create a nomadic tool owned by user1 but currently held by user2
		nomadicTool := &Tool{
			ID:           4,
			Title:        "Nomadic Tool",
			UserID:       user1ID,      // Owned by user1
			ActualUserID: user2ID,      // Currently held by user2
			Location:     userLocation, // Same location as user2
			IsNomadic:    true,
		}

		_, err = toolService.InsertTool(ctx, nomadicTool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert nomadic tool"))

		// Update user2's location (the one currently holding the tool)
		newLocation := NewLocation(48856614, 2352222) // Paris coordinates
		update := bson.M{
			"location": newLocation,
		}

		_, err = userService.UpdateUser(ctx, user2ID, update)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to update user2 location"))

		// Verify that the nomadic tool location was updated
		updatedTool, err := toolService.GetToolByID(ctx, 4)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get updated nomadic tool"))
		c.Assert(updatedTool.Location.Coordinates, qt.DeepEquals, newLocation.Coordinates, qt.Commentf("Nomadic tool location not updated"))
	})

	c.Run("Mixed Scenario", func(c *qt.C) {
		// Create users
		userLocation := NewLocation(41385063, 2173404) // Barcelona coordinates
		user1 := &User{
			Email:    "user1mixed@example.com",
			Name:     "User 1 Mixed",
			Location: userLocation,
			Salt:     "testsalt1mixed",
			Active:   true,
		}
		user2 := &User{
			Email:    "user2mixed@example.com",
			Name:     "User 2 Mixed",
			Location: userLocation,
			Salt:     "testsalt2mixed",
			Active:   true,
		}

		user1Result, err := userService.InsertUser(ctx, user1)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert user1 for mixed scenario"))
		user1ID := user1Result.InsertedID.(primitive.ObjectID)

		user2Result, err := userService.InsertUser(ctx, user2)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert user2 for mixed scenario"))
		user2ID := user2Result.InsertedID.(primitive.ObjectID)

		// Create various tools
		differentLocation := NewLocation(40416775, -3703790) // Madrid coordinates

		// Tool owned by user1 with matching location (should be updated)
		ownedTool := &Tool{
			ID:       5,
			Title:    "Owned Tool",
			UserID:   user1ID,
			Location: userLocation,
		}

		// Tool owned by user1 with different location (should not be updated)
		ownedToolDifferent := &Tool{
			ID:       6,
			Title:    "Owned Tool Different",
			UserID:   user1ID,
			Location: differentLocation,
		}

		// Nomadic tool held by user1 with matching location (should be updated)
		nomadicTool := &Tool{
			ID:           7,
			Title:        "Nomadic Tool",
			UserID:       user2ID,      // Owned by user2
			ActualUserID: user1ID,      // Currently held by user1
			Location:     userLocation, // Same location as user1
			IsNomadic:    true,
		}

		// Tool owned by different user (should not be updated)
		otherUserTool := &Tool{
			ID:       8,
			Title:    "Other User Tool",
			UserID:   user2ID,
			Location: userLocation,
		}

		_, err = toolService.InsertTool(ctx, ownedTool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert owned tool"))
		_, err = toolService.InsertTool(ctx, ownedToolDifferent)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert owned tool different"))
		_, err = toolService.InsertTool(ctx, nomadicTool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert nomadic tool"))
		_, err = toolService.InsertTool(ctx, otherUserTool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert other user tool"))

		// Update user1's location
		newLocation := NewLocation(48856614, 2352222) // Paris coordinates
		update := bson.M{
			"location": newLocation,
		}

		_, err = userService.UpdateUser(ctx, user1ID, update)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to update user1 location"))

		// Verify owned tool with matching location was updated
		updatedOwnedTool, err := toolService.GetToolByID(ctx, 5)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get updated owned tool"))
		c.Assert(updatedOwnedTool.Location.Coordinates, qt.DeepEquals, newLocation.Coordinates, qt.Commentf("Owned tool location not updated"))

		// Verify owned tool with different location was not updated
		updatedOwnedToolDifferent, err := toolService.GetToolByID(ctx, 6)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get owned tool different"))
		c.Assert(updatedOwnedToolDifferent.Location.Coordinates, qt.DeepEquals, differentLocation.Coordinates, qt.Commentf("Owned tool different location should not have been updated"))

		// Verify nomadic tool held by user1 was updated
		updatedNomadicTool, err := toolService.GetToolByID(ctx, 7)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get updated nomadic tool"))
		c.Assert(updatedNomadicTool.Location.Coordinates, qt.DeepEquals, newLocation.Coordinates, qt.Commentf("Nomadic tool location not updated"))

		// Verify tool owned by different user was not updated
		updatedOtherUserTool, err := toolService.GetToolByID(ctx, 8)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get other user tool"))
		c.Assert(updatedOtherUserTool.Location.Coordinates, qt.DeepEquals, userLocation.Coordinates, qt.Commentf("Other user tool location should not have been updated"))
	})

	c.Run("Non-Location Update Does Not Affect Tools", func(c *qt.C) {
		// Create a user with a specific location
		userLocation := NewLocation(41385063, 2173404) // Barcelona coordinates
		user := &User{
			Email:    "nonlocation@example.com",
			Name:     "Non Location Test User",
			Location: userLocation,
			Salt:     "testsalt",
			Active:   true,
		}

		userResult, err := userService.InsertUser(ctx, user)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert user for non-location test"))
		userID := userResult.InsertedID.(primitive.ObjectID)

		// Create a tool owned by the user
		tool := &Tool{
			ID:       9,
			Title:    "Test Tool",
			UserID:   userID,
			Location: userLocation,
		}

		_, err = toolService.InsertTool(ctx, tool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert tool for non-location test"))

		// Update user name (not location)
		update := bson.M{
			"name": "Updated Name",
		}

		_, err = userService.UpdateUser(ctx, userID, update)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to update user name"))

		// Verify that tool location was not changed
		updatedTool, err := toolService.GetToolByID(ctx, 9)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get tool after non-location update"))
		c.Assert(updatedTool.Location.Coordinates, qt.DeepEquals, userLocation.Coordinates, qt.Commentf("Tool location should not have changed"))

		// Verify user name was updated
		updatedUser, err := userService.GetUserByID(ctx, userID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get updated user"))
		c.Assert(updatedUser.Name, qt.Equals, "Updated Name", qt.Commentf("User name was not updated"))
	})

	c.Run("Obfuscated Location Updated", func(c *qt.C) {
		// Create a user with a specific location
		userLocation := NewLocation(41385063, 2173404) // Barcelona coordinates
		user := &User{
			Email:    "obfuscated@example.com",
			Name:     "Obfuscated Test User",
			Location: userLocation,
			Salt:     "testsalt",
			Active:   true,
		}

		userResult, err := userService.InsertUser(ctx, user)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert user for obfuscated test"))
		userID := userResult.InsertedID.(primitive.ObjectID)

		// Create a tool owned by the user
		tool := &Tool{
			ID:                 10,
			Title:              "Test Tool",
			UserID:             userID,
			Location:           userLocation,
			ObfuscatedLocation: ObfuscateLocation(userLocation, userID, "testsalt"),
		}

		_, err = toolService.InsertTool(ctx, tool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert tool for obfuscated test"))

		// Update user location
		newLocation := NewLocation(48856614, 2352222) // Paris coordinates
		newObfuscatedLocation := ObfuscateLocation(newLocation, userID, "testsalt")
		update := bson.M{
			"location":           newLocation,
			"obfuscatedLocation": newObfuscatedLocation,
		}

		_, err = userService.UpdateUser(ctx, userID, update)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to update user location with obfuscated"))

		// Verify that tool's obfuscated location was also updated
		updatedTool, err := toolService.GetToolByID(ctx, 10)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get tool after obfuscated location update"))
		c.Assert(updatedTool.Location.Coordinates, qt.DeepEquals, newLocation.Coordinates, qt.Commentf("Tool location not updated"))
		c.Assert(updatedTool.ObfuscatedLocation.Coordinates, qt.DeepEquals, newObfuscatedLocation.Coordinates, qt.Commentf("Tool obfuscated location not updated"))
	})
}

func TestLocationsEqual(t *testing.T) {
	c := qt.New(t)

	loc1 := NewLocation(41385063, 2173404)
	loc2 := NewLocation(41385063, 2173404)
	loc3 := NewLocation(40416775, -3703790)

	c.Assert(locationsEqual(loc1, loc2), qt.Equals, true, qt.Commentf("Same locations should be equal"))
	c.Assert(locationsEqual(loc1, loc3), qt.Equals, false, qt.Commentf("Different locations should not be equal"))

	// Test with invalid coordinates
	invalidLoc := DBLocation{Type: "Point", Coordinates: []float64{1.0}}
	c.Assert(locationsEqual(loc1, invalidLoc), qt.Equals, false, qt.Commentf("Location with invalid coordinates should not be equal"))
	c.Assert(locationsEqual(invalidLoc, loc1), qt.Equals, false, qt.Commentf("Invalid location should not be equal to valid location"))
}
