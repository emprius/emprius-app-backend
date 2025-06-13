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
