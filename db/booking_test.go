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
			qt.Assert(t, updatedUser.RatingCount, qt.Equals, 1, qt.Commentf("RatingCount should be 1 after a single rating"))
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
		qt.Assert(t, updatedUser.RatingCount, qt.Equals, 2, qt.Commentf("RatingCount should be 2 after two ratings"))
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

func TestBookingService_GetUnifiedRatings(t *testing.T) {
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

	now := time.Now()

	// Create a booking in RETURNED state (pending rating)
	pendingRatingBooking := &Booking{
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
	_, err = bookingService.collection.InsertOne(ctx, pendingRatingBooking)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert pending rating booking"))

	// Create a booking in ACCEPTED state (not pending rating)
	acceptedBooking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "1",
		FromUserID:    fromUser.ID,
		ToUserID:      toUser.ID,
		StartDate:     now.Add(48 * time.Hour),
		EndDate:       now.Add(72 * time.Hour),
		Contact:       "test contact",
		Comments:      "test comments",
		BookingStatus: BookingStatusAccepted,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err = bookingService.collection.InsertOne(ctx, acceptedBooking)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert accepted booking"))

	// Create a rating for the accepted booking
	rating := &Rating{
		BookingID: acceptedBooking.ID,
		RaterID:   fromUser.ID,
		RateeID:   toUser.ID,
		Score:     5,
		Comment:   "Great tool!",
		CreatedAt: now,
	}
	_, err = bookingService.ratingsCollection.InsertOne(ctx, rating)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert rating"))

	// Test 1: Verify that GetPendingRatings returns the pending rating booking
	pendingRatings, err := bookingService.GetPendingRatings(ctx, fromUser.ID)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get pending ratings"))
	qt.Assert(t, len(pendingRatings), qt.Equals, 1, qt.Commentf("Expected 1 pending rating"))
	qt.Assert(t, pendingRatings[0].ID, qt.Equals, pendingRatingBooking.ID,
		qt.Commentf("Expected pending rating booking ID to match"))

	// Test 2: Verify that GetUnifiedRatings excludes the pending rating booking
	unifiedRatings, err := bookingService.GetUnifiedRatings(ctx, fromUser.ID)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get unified ratings"))

	// We should only see the accepted booking, not the pending rating booking
	qt.Assert(t, len(unifiedRatings), qt.Equals, 1, qt.Commentf("Expected 1 unified rating (only the accepted booking)"))

	// Verify that the unified rating is for the accepted booking
	qt.Assert(t, unifiedRatings[0].BookingID, qt.Equals, acceptedBooking.ID,
		qt.Commentf("Expected unified rating booking ID to match accepted booking ID"))

	// Verify that the pending rating booking is not included
	for _, ur := range unifiedRatings {
		qt.Assert(t, ur.BookingID, qt.Not(qt.Equals), pendingRatingBooking.ID,
			qt.Commentf("Pending rating booking should not be included in unified ratings"))
	}
}

func TestNomadicTool(t *testing.T) {
	ctx := context.Background()

	// Start MongoDB container
	container, err := StartMongoContainer(ctx)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to start MongoDB container"))
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	// Get MongoDB connection string
	mongoURI, err := container.Endpoint(ctx, "mongodb")
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get MongoDB container endpoint"))

	// Create a MongoDB client
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to connect to MongoDB"))
	defer func() { _ = client.Disconnect(ctx) }()

	// Use a random database name for isolation
	dbName := RandomDatabaseName()
	db := client.Database(dbName)

	// Create services
	bookingService := NewBookingService(db)
	userService := NewUserService(&Database{Database: db})
	toolService := NewToolService(&Database{Database: db})

	// Create test users: owner, first borrower, and second borrower
	owner := &User{
		ID:       primitive.NewObjectID(),
		Email:    "owner@test.com",
		Name:     "Owner",
		Active:   true,
		Rating:   50,
		Verified: true,
	}
	_, err = userService.InsertUser(ctx, owner)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert owner"))

	firstBorrower := &User{
		ID:       primitive.NewObjectID(),
		Email:    "borrower1@test.com",
		Name:     "First Borrower",
		Active:   true,
		Rating:   50,
		Verified: true,
	}
	_, err = userService.InsertUser(ctx, firstBorrower)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert first borrower"))

	secondBorrower := &User{
		ID:       primitive.NewObjectID(),
		Email:    "borrower2@test.com",
		Name:     "Second Borrower",
		Active:   true,
		Rating:   50,
		Verified: true,
	}
	_, err = userService.InsertUser(ctx, secondBorrower)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert second borrower"))

	// Create a nomadic test tool
	nomadicTool := &Tool{
		ID:             1,
		Title:          "Nomadic Tool",
		Description:    "This is a nomadic tool",
		IsAvailable:    true,
		UserID:         owner.ID,
		EstimatedValue: 10000,
		Nomadic:        true,          // This is a nomadic tool
		ReservedDates:  []DateRange{}, // Initialize empty reserved dates array
		Location: DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.1, 41.1}, // Initially at owner's location
		},
	}
	_, err = toolService.InsertTool(ctx, nomadicTool)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert nomadic tool"))

	now := time.Now()

	// First booking: first borrower requests the tool from the owner
	firstBooking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "1",
		FromUserID:    firstBorrower.ID,
		ToUserID:      owner.ID,
		StartDate:     now,
		EndDate:       now.Add(24 * time.Hour),
		Contact:       "test contact",
		Comments:      "test comments",
		BookingStatus: BookingStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err = bookingService.collection.InsertOne(ctx, firstBooking)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert first booking"))

	// Verify the booking has the correct user IDs
	qt.Assert(t, firstBooking.FromUserID, qt.Equals, firstBorrower.ID, qt.Commentf("First booking FromUserID should be first borrower"))
	qt.Assert(t, firstBooking.ToUserID, qt.Equals, owner.ID, qt.Commentf("First booking ToUserID should be owner"))

	// Accept the booking
	err = bookingService.UpdateStatus(ctx, firstBooking.ID, BookingStatusAccepted)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to accept first booking"))

	// Simulate picking up the tool - update tool location and actualUserId
	firstBorrowerLocation := DBLocation{
		Type:        "Point",
		Coordinates: []float64{2.2, 41.2}, // First borrower location
	}
	updates := map[string]interface{}{
		"location":     firstBorrowerLocation,
		"actualUserId": firstBorrower.ID,
	}
	err = toolService.UpdateToolFields(ctx, 1, updates)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to update tool location and actualUserId"))

	// Update booking status to PICKED
	err = bookingService.UpdateStatus(ctx, firstBooking.ID, BookingStatusPicked)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to update first booking status to PICKED"))

	// Verify tool location and actualUserId have been updated
	updatedTool, err := toolService.GetToolByID(ctx, 1)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get updated tool"))
	qt.Assert(t, updatedTool.Location.Coordinates[0], qt.Equals, firstBorrowerLocation.Coordinates[0], qt.Commentf("Tool location should be updated to first borrower's location"))
	qt.Assert(t, updatedTool.Location.Coordinates[1], qt.Equals, firstBorrowerLocation.Coordinates[1], qt.Commentf("Tool location should be updated to first borrower's location"))
	qt.Assert(t, updatedTool.ActualUserID, qt.Equals, firstBorrower.ID, qt.Commentf("Tool actualUserId should be updated to first borrower"))

	// Second booking: second borrower requests the tool from the first borrower
	secondBooking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "1",
		FromUserID:    secondBorrower.ID,
		ToUserID:      firstBorrower.ID, // Important: ToUserID is the current holder (first borrower), not the owner
		StartDate:     now.Add(48 * time.Hour),
		EndDate:       now.Add(72 * time.Hour),
		Contact:       "test contact 2",
		Comments:      "test comments 2",
		BookingStatus: BookingStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err = bookingService.collection.InsertOne(ctx, secondBooking)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert second booking"))

	// Verify the second booking has the correct user IDs
	qt.Assert(t, secondBooking.FromUserID, qt.Equals, secondBorrower.ID, qt.Commentf("Second booking FromUserID should be second borrower"))
	qt.Assert(t, secondBooking.ToUserID, qt.Equals, firstBorrower.ID, qt.Commentf("Second booking ToUserID should be first borrower (current holder), not the owner"))

	// Accept the second booking
	err = bookingService.UpdateStatus(ctx, secondBooking.ID, BookingStatusAccepted)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to accept second booking"))

	// Simulate picking up the tool by the second borrower
	secondBorrowerLocation := DBLocation{
		Type:        "Point",
		Coordinates: []float64{2.3, 41.3}, // Second borrower location
	}
	updates = map[string]interface{}{
		"location":     secondBorrowerLocation,
		"actualUserId": secondBorrower.ID,
	}
	err = toolService.UpdateToolFields(ctx, 1, updates)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to update tool location and actualUserId for second borrower"))

	// Update second booking status to PICKED
	err = bookingService.UpdateStatus(ctx, secondBooking.ID, BookingStatusPicked)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to update second booking status to PICKED"))

	// Verify tool location and actualUserId have been updated to second borrower
	finalTool, err := toolService.GetToolByID(ctx, 1)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get final tool"))
	qt.Assert(t, finalTool.Location.Coordinates[0], qt.Equals, secondBorrowerLocation.Coordinates[0], qt.Commentf("Tool location should be updated to second borrower's location"))
	qt.Assert(t, finalTool.Location.Coordinates[1], qt.Equals, secondBorrowerLocation.Coordinates[1], qt.Commentf("Tool location should be updated to second borrower's location"))
	qt.Assert(t, finalTool.ActualUserID, qt.Equals, secondBorrower.ID, qt.Commentf("Tool actualUserId should be updated to second borrower"))

	// Verify the nomadic attribute is correctly set in the tool
	qt.Assert(t, finalTool.Nomadic, qt.IsTrue, qt.Commentf("Tool should be nomadic"))
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

func TestBookingService_CountPendingActions(t *testing.T) {
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

	// Create services
	bookingService := NewBookingService(db)
	userService := NewUserService(&Database{Database: db})
	toolService := NewToolService(&Database{Database: db})

	// Create test users: owner, actual user, and requester
	owner := &User{
		ID:       primitive.NewObjectID(),
		Email:    "owner@test.com",
		Name:     "Owner",
		Active:   true,
		Verified: true,
	}
	_, err = userService.InsertUser(ctx, owner)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert owner"))

	actualUser := &User{
		ID:       primitive.NewObjectID(),
		Email:    "actual@test.com",
		Name:     "Actual User",
		Active:   true,
		Verified: true,
	}
	_, err = userService.InsertUser(ctx, actualUser)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert actual user"))

	requester := &User{
		ID:       primitive.NewObjectID(),
		Email:    "requester@test.com",
		Name:     "Requester",
		Active:   true,
		Verified: true,
	}
	_, err = userService.InsertUser(ctx, requester)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert requester"))

	// Create a nomadic tool
	nomadicTool := &Tool{
		ID:             1,
		Title:          "Nomadic Tool",
		Description:    "This is a nomadic tool",
		IsAvailable:    true,
		UserID:         owner.ID,
		ActualUserID:   actualUser.ID, // The actual user has the tool
		EstimatedValue: 10000,
		Nomadic:        true,
		Location: DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.1, 41.1},
		},
	}
	_, err = toolService.InsertTool(ctx, nomadicTool)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert nomadic tool"))

	// Create a non-nomadic tool
	nonNomadicTool := &Tool{
		ID:             2,
		Title:          "Regular Tool",
		Description:    "This is a regular tool",
		IsAvailable:    true,
		UserID:         owner.ID,
		EstimatedValue: 5000,
		Nomadic:        false,
		Location: DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.2, 41.2},
		},
	}
	_, err = toolService.InsertTool(ctx, nonNomadicTool)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert non-nomadic tool"))

	now := time.Now()

	// Create a pending booking for the nomadic tool
	nomadicBooking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "1", // Nomadic tool
		FromUserID:    requester.ID,
		ToUserID:      owner.ID, // Note: ToUserID is the owner, but the actual user should receive the request
		StartDate:     now,
		EndDate:       now.Add(24 * time.Hour),
		Contact:       "test contact",
		Comments:      "test comments",
		BookingStatus: BookingStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err = bookingService.collection.InsertOne(ctx, nomadicBooking)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert nomadic booking"))

	// Create a pending booking for the non-nomadic tool
	nonNomadicBooking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "2", // Non-nomadic tool
		FromUserID:    requester.ID,
		ToUserID:      owner.ID,
		StartDate:     now,
		EndDate:       now.Add(24 * time.Hour),
		Contact:       "test contact",
		Comments:      "test comments",
		BookingStatus: BookingStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err = bookingService.collection.InsertOne(ctx, nonNomadicBooking)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert non-nomadic booking"))

	// Test 1: Owner should see only the non-nomadic tool booking
	ownerPending, err := bookingService.CountPendingActions(ctx, owner.ID)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to count pending actions for owner"))
	qt.Assert(t, ownerPending.PendingRequestsCount, qt.Equals, int64(1), qt.Commentf("Owner should see 1 pending request (non-nomadic tool)"))

	// Test 2: Actual user should see the nomadic tool booking
	actualUserPending, err := bookingService.CountPendingActions(ctx, actualUser.ID)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to count pending actions for actual user"))
	qt.Assert(t, actualUserPending.PendingRequestsCount, qt.Equals, int64(1), qt.Commentf("Actual user should see 1 pending request (nomadic tool)"))

	// Test 3: Requester should see no pending requests
	requesterPending, err := bookingService.CountPendingActions(ctx, requester.ID)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to count pending actions for requester"))
	qt.Assert(t, requesterPending.PendingRequestsCount, qt.Equals, int64(0), qt.Commentf("Requester should see 0 pending requests"))
}
