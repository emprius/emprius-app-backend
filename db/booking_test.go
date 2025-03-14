package db

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func clearBookingsAndRatings(s *BookingService, t *testing.T) {
	// Delete all bookings.
	_, err := s.collection.DeleteMany(context.Background(), bson.M{})
	qt.Assert(t, err, qt.IsNil, qt.Commentf("failed to clear bookings: %v", err))
	// Delete all ratings.
	_, err = s.ratingsCollection.DeleteMany(context.Background(), bson.M{})
	qt.Assert(t, err, qt.IsNil, qt.Commentf("failed to clear ratings: %v", err))
}

func TestBookingService_RatingCalculation(t *testing.T) {
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
	db := client.Database(dbName)

	bookingService := NewBookingService(db)
	userService := NewUserService(&Database{Database: db})
	toolService := NewToolService(&Database{Database: db})

	// Create test users
	fromUser := &User{
		ID:       primitive.NewObjectID(),
		Email:    "renter@test.com",
		Name:     "Renter",
		Active:   true,
		Rating:   50,
		Verified: true,
	}
	_, err = userService.InsertUser(ctx, fromUser)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert renter"))

	toUser := &User{
		ID:       primitive.NewObjectID(),
		Email:    "owner@test.com",
		Name:     "Owner",
		Active:   true,
		Rating:   50,
		Verified: true,
	}
	_, err = userService.InsertUser(ctx, toUser)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert owner"))

	// Create test tool
	tool := &Tool{
		ID:             1,
		Title:          "Test Tool",
		Description:    "Test Description",
		IsAvailable:    true,
		UserID:         toUser.ID,
		EstimatedValue: 10000,
		Location: DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.492793, 41.695384},
		},
	}
	_, err = toolService.InsertTool(ctx, tool)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert tool"))

	now := time.Now()

	testCases := []struct {
		name           string
		rating         int
		expectedRating int32 // Expected overall owner rating (in percentage)
	}{
		{"5 stars = 100%", 5, 100},
		{"4 stars = 80%", 4, 80},
		{"3 stars = 60%", 3, 60},
		{"2 stars = 40%", 2, 40},
		{"1 star = 20%", 1, 20},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up both bookings and ratings.
			clearBookingsAndRatings(bookingService, t)

			// Reset owner's rating to initial value.
			_, err = userService.Collection.UpdateOne(ctx, bson.M{"_id": toUser.ID}, bson.M{"$set": bson.M{"rating": 50}})
			qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to reset owner rating"))

			// Create a new booking (with status RETURNED to allow rating).
			booking := &Booking{
				ID:            primitive.NewObjectID(),
				ToolID:        "1",
				FromUserID:    fromUser.ID,
				ToUserID:      toUser.ID,
				StartDate:     now,
				EndDate:       now.Add(24 * time.Hour),
				Contact:       "test contact",
				Comments:      "test comments",
				BookingStatus: BookingStatusReturned,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			_, err = bookingService.collection.InsertOne(ctx, booking)
			qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert booking"))

			// Rate the booking from the renter.
			err = bookingService.RateBooking(ctx, booking.ID, fromUser.ID, tc.rating, "Test comment")
			qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to rate booking"))

			// Verify that the owner's overall rating has been updated correctly.
			var updatedUser User
			err = userService.Collection.FindOne(ctx, bson.M{"_id": toUser.ID}).Decode(&updatedUser)
			qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get updated owner"))
			qt.Assert(t, updatedUser.Rating, qt.Equals, tc.expectedRating)
		})
	}

	t.Run("Multiple ratings average", func(t *testing.T) {
		// Clean up bookings and ratings.
		clearBookingsAndRatings(bookingService, t)

		// Reset owner's rating.
		_, err = userService.Collection.UpdateOne(ctx, bson.M{"_id": toUser.ID}, bson.M{"$set": bson.M{"rating": 50}})
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to reset owner rating"))

		// Create a booking (booking2) and rate it with 3 stars.
		booking2 := &Booking{
			ID:            primitive.NewObjectID(),
			ToolID:        "1",
			FromUserID:    fromUser.ID,
			ToUserID:      toUser.ID,
			StartDate:     now.Add(48 * time.Hour),
			EndDate:       now.Add(72 * time.Hour),
			Contact:       "test contact",
			Comments:      "test comments",
			BookingStatus: BookingStatusReturned,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		_, err = bookingService.collection.InsertOne(ctx, booking2)
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert booking2"))

		err = bookingService.RateBooking(ctx, booking2.ID, fromUser.ID, 3, "Test comment")
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to rate booking2"))

		// Create another booking (booking1) and rate it with 5 stars.
		booking1 := &Booking{
			ID:            primitive.NewObjectID(),
			ToolID:        "1",
			FromUserID:    fromUser.ID,
			ToUserID:      toUser.ID,
			StartDate:     now,
			EndDate:       now.Add(24 * time.Hour),
			Contact:       "test contact",
			Comments:      "test comments",
			BookingStatus: BookingStatusReturned,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		_, err = bookingService.collection.InsertOne(ctx, booking1)
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert booking1"))

		err = bookingService.RateBooking(ctx, booking1.ID, fromUser.ID, 5, "Test comment")
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to rate booking1"))

		// Expected overall rating is the average of 3 and 5 which is 4 stars → 80%.
		var updatedUser User
		err = userService.Collection.FindOne(ctx, bson.M{"_id": toUser.ID}).Decode(&updatedUser)
		qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get updated owner after multiple ratings"))
		qt.Assert(t, updatedUser.Rating, qt.Equals, int32(80))
	})
}

func TestBookingService_GetPendingRatings(t *testing.T) {
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
	db := client.Database(dbName)

	bookingService := NewBookingService(db)

	// Create test users
	fromUser := &User{
		ID: primitive.NewObjectID(),
	}

	toUser := &User{
		ID: primitive.NewObjectID(),
	}

	// Create test booking
	now := time.Now()
	booking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "1",
		FromUserID:    fromUser.ID,
		ToUserID:      toUser.ID,
		StartDate:     now,
		EndDate:       now.Add(24 * time.Hour),
		Contact:       "test contact",
		Comments:      "test comments",
		BookingStatus: BookingStatusReturned,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err = bookingService.collection.InsertOne(ctx, booking)
	assert.NoError(t, err)

	// Test 1: Initially both users should see the pending rating
	fromUserPending, err := bookingService.GetPendingRatings(ctx, fromUser.ID)
	assert.NoError(t, err)
	assert.Len(t, fromUserPending, 1, "Renter should see pending rating initially")

	toUserPending, err := bookingService.GetPendingRatings(ctx, toUser.ID)
	assert.NoError(t, err)
	assert.Len(t, toUserPending, 1, "Owner should see pending rating initially")

	// Test 2: After renter rates, only owner should see pending rating
	err = bookingService.RateBooking(ctx, booking.ID, fromUser.ID, 5, "Great tool!")
	assert.NoError(t, err)

	fromUserPending, err = bookingService.GetPendingRatings(ctx, fromUser.ID)
	assert.NoError(t, err)
	assert.Len(t, fromUserPending, 0, "Renter should not see pending rating after rating")

	toUserPending, err = bookingService.GetPendingRatings(ctx, toUser.ID)
	assert.NoError(t, err)
	assert.Len(t, toUserPending, 1, "Owner should still see pending rating")

	// Test 3: After both rate, neither should see pending rating
	err = bookingService.RateBooking(ctx, booking.ID, toUser.ID, 5, "Great renter!")
	assert.NoError(t, err)

	fromUserPending, err = bookingService.GetPendingRatings(ctx, fromUser.ID)
	assert.NoError(t, err)
	assert.Len(t, fromUserPending, 0, "Renter should not see pending rating after both rated")

	toUserPending, err = bookingService.GetPendingRatings(ctx, toUser.ID)
	assert.NoError(t, err)
	assert.Len(t, toUserPending, 0, "Owner should not see pending rating after both rated")
}

func TestBookingService_TokenCalculation(t *testing.T) {
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
	db := client.Database(dbName)

	bookingService := NewBookingService(db)

	// Test cases for different durations
	testCases := []struct {
		name     string
		duration time.Duration
		cost     uint64
		expected uint64
	}{
		{
			name:     "1 day exactly",
			duration: 24 * time.Hour,
			cost:     100,
			expected: 100,
		},
		{
			name:     "2 days exactly",
			duration: 48 * time.Hour,
			cost:     100,
			expected: 200,
		},
		{
			name:     "1.5 days (rounds up)",
			duration: 36 * time.Hour,
			cost:     100,
			expected: 200,
		},
		{
			name:     "23 hours (rounds up to 1 day)",
			duration: 23 * time.Hour,
			cost:     100,
			expected: 100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			booking := &Booking{
				StartDate: now,
				EndDate:   now.Add(tc.duration),
			}
			tool := &Tool{
				Cost: tc.cost,
			}

			result := bookingService.calculateTokenCost(booking, tool)
			assert.Equal(t, tc.expected, result)
		})
	}
}
