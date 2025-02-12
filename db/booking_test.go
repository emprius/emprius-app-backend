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

func TestBookingService_TokenOperations(t *testing.T) {
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
		Tokens:   1000,
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
		Tokens:   1000,
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
		Cost:           100, // 100 tokens per day
		UserID:         toUser.ID,
		EstimatedValue: 10000,
		Location: DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.492793, 41.695384},
		},
		ReservedDates: []DateRange{}, // Initialize empty array
	}
	_, err = toolService.InsertTool(ctx, tool)
	assert.NoError(t, err)

	// Create test booking for 2 days
	now := time.Now()
	booking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "1",
		FromUserID:    fromUser.ID,
		ToUserID:      toUser.ID,
		StartDate:     now,
		EndDate:       now.Add(48 * time.Hour), // 2 days
		Contact:       "test contact",
		Comments:      "test comments",
		BookingStatus: BookingStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err = bookingService.collection.InsertOne(ctx, booking)
	assert.NoError(t, err)

	// Test accepting booking deducts tokens from renter
	err = bookingService.UpdateStatus(ctx, booking.ID, BookingStatusAccepted)
	assert.NoError(t, err)

	// Verify renter's tokens were deducted (2 days * 100 tokens = 200 tokens)
	var updatedFromUser User
	err = userService.Collection.FindOne(ctx, bson.M{"_id": fromUser.ID}).Decode(&updatedFromUser)
	assert.NoError(t, err)
	assert.Equal(t, uint64(800), updatedFromUser.Tokens) // 1000 - 200

	// Test returning tool adds tokens to owner
	err = bookingService.UpdateStatus(ctx, booking.ID, BookingStatusReturned)
	assert.NoError(t, err)

	// Verify owner's tokens were increased
	var updatedToUser User
	err = userService.Collection.FindOne(ctx, bson.M{"_id": toUser.ID}).Decode(&updatedToUser)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1200), updatedToUser.Tokens) // 1000 + 200

	// Test insufficient tokens case
	poorUser := &User{
		ID:       primitive.NewObjectID(),
		Email:    "poor@test.com",
		Name:     "Poor User",
		Tokens:   50, // Not enough tokens for the booking
		Active:   true,
		Rating:   50,
		Verified: true,
	}
	_, err = userService.InsertUser(ctx, poorUser)
	assert.NoError(t, err)

	poorBooking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "1",
		FromUserID:    poorUser.ID,
		ToUserID:      toUser.ID,
		StartDate:     now,
		EndDate:       now.Add(48 * time.Hour), // 2 days
		Contact:       "test contact",
		Comments:      "test comments",
		BookingStatus: BookingStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err = bookingService.collection.InsertOne(ctx, poorBooking)
	assert.NoError(t, err)

	// Test accepting booking with insufficient tokens
	err = bookingService.UpdateStatus(ctx, poorBooking.ID, BookingStatusAccepted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient tokens")

	// Verify poor user's tokens remain unchanged
	var updatedPoorUser User
	err = userService.Collection.FindOne(ctx, bson.M{"_id": poorUser.ID}).Decode(&updatedPoorUser)
	assert.NoError(t, err)
	assert.Equal(t, uint64(50), updatedPoorUser.Tokens)
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
