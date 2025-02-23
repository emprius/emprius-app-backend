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
	assert.NoError(t, err)

	toUser := &User{
		ID:       primitive.NewObjectID(),
		Email:    "owner@test.com",
		Name:     "Owner",
		Active:   true,
		Rating:   50,
		Verified: true,
	}
	_, err = userService.InsertUser(ctx, toUser)
	assert.NoError(t, err)

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
	assert.NoError(t, err)

	// Test cases for different ratings
	now := time.Now()
	testCases := []struct {
		name           string
		rating         int
		expectedRating int32 // Expected user rating (0-100)
	}{
		{
			name:           "5 stars = 100%",
			rating:         5,
			expectedRating: 100,
		},
		{
			name:           "4 stars = 80%",
			rating:         4,
			expectedRating: 80,
		},
		{
			name:           "3 stars = 60%",
			rating:         3,
			expectedRating: 60,
		},
		{
			name:           "2 stars = 40%",
			rating:         2,
			expectedRating: 40,
		},
		{
			name:           "1 star = 20%",
			rating:         1,
			expectedRating: 20,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset user's rating to initial value
			_, err = userService.Collection.UpdateOne(
				ctx,
				bson.M{"_id": toUser.ID},
				bson.M{"$set": bson.M{"rating": 50}},
			)
			assert.NoError(t, err)

			// Clear all existing bookings
			_, err = bookingService.collection.DeleteMany(ctx, bson.M{})
			assert.NoError(t, err)
			// Create a new booking for each test case
			booking := &Booking{
				ID:            primitive.NewObjectID(),
				ToolID:        "1",
				FromUserID:    fromUser.ID,
				ToUserID:      toUser.ID,
				StartDate:     now,
				EndDate:       now.Add(24 * time.Hour),
				Contact:       "test contact",
				Comments:      "test comments",
				BookingStatus: BookingStatusReturned, // Set to RETURNED to allow rating
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			_, err = bookingService.collection.InsertOne(ctx, booking)
			assert.NoError(t, err)

			// Rate the booking
			err = bookingService.RateBooking(ctx, booking.ID, fromUser.ID, tc.rating, "Test comment")
			assert.NoError(t, err)

			// Verify user's rating was updated correctly
			var updatedUser User
			err = userService.Collection.FindOne(ctx, bson.M{"_id": toUser.ID}).Decode(&updatedUser)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedRating, updatedUser.Rating)
		})
	}

	// Test multiple ratings average
	t.Run("Multiple ratings average", func(t *testing.T) {
		// Reset user's rating to initial value
		_, err = userService.Collection.UpdateOne(
			ctx,
			bson.M{"_id": toUser.ID},
			bson.M{"$set": bson.M{"rating": 50}},
		)
		assert.NoError(t, err)

		// Clear all existing bookings
		_, err = bookingService.collection.DeleteMany(ctx, bson.M{})
		assert.NoError(t, err)
		// Create another booking
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
		assert.NoError(t, err)

		// Rate with 3 stars (60%)
		err = bookingService.RateBooking(ctx, booking2.ID, fromUser.ID, 3, "Test comment")
		assert.NoError(t, err)

		// Create first booking
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
		assert.NoError(t, err)

		// Rate first booking with 5 stars (100%)
		err = bookingService.RateBooking(ctx, booking1.ID, fromUser.ID, 5, "Test comment")
		assert.NoError(t, err)

		// Verify user's rating is average of both ratings (80%)
		var updatedUser User
		err = userService.Collection.FindOne(ctx, bson.M{"_id": toUser.ID}).Decode(&updatedUser)
		assert.NoError(t, err)
		assert.Equal(t, int32(80), updatedUser.Rating)
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
		ID:       primitive.NewObjectID(),
		Email:    "renter@test.com",
		Name:     "Renter",
		Active:   true,
		Verified: true,
	}

	toUser := &User{
		ID:       primitive.NewObjectID(),
		Email:    "owner@test.com",
		Name:     "Owner",
		Active:   true,
		Verified: true,
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
