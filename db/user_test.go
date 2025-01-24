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
			Location:   Location{Latitude: 123456, Longitude: 654321},
			Verified:   true,
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
			Location:  Location{Latitude: 111222, Longitude: 333444},
			Verified:  false,
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

	c.Run("Get All Users", func(c *qt.C) {
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
		allUsers, err := userService.GetAllUsers(ctx, 0)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve users"))
		c.Assert(len(allUsers) >= 3, qt.Equals, true, qt.Commentf("Expected at least 3 users in first page"))

		// Test invalid page number
		invalidUsers, err := userService.GetAllUsers(ctx, -1)
		c.Assert(err, qt.IsNil, qt.Commentf("Expected success with invalid page number"))
		c.Assert(len(invalidUsers) >= 3, qt.Equals, true, qt.Commentf("Expected first page results for invalid page number"))
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
}
