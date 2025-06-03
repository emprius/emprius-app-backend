package db

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestGetPendingRatingsWithPickedStatus(t *testing.T) {
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

	// Create booking service
	bookingService := NewBookingService(db)

	// Create test users
	userID1 := primitive.NewObjectID()
	userID2 := primitive.NewObjectID()

	// Create test bookings with different statuses
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)

	// Create a returned booking
	returnedBooking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "1",
		FromUserID:    userID1,
		ToUserID:      userID2,
		StartDate:     now,
		EndDate:       tomorrow,
		BookingStatus: BookingStatusReturned,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Create a picked booking
	pickedBooking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "2",
		FromUserID:    userID1,
		ToUserID:      userID2,
		StartDate:     now,
		EndDate:       tomorrow,
		BookingStatus: BookingStatusPicked,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Create a pending booking (should not be returned)
	pendingBooking := &Booking{
		ID:            primitive.NewObjectID(),
		ToolID:        "3",
		FromUserID:    userID1,
		ToUserID:      userID2,
		StartDate:     now,
		EndDate:       tomorrow,
		BookingStatus: BookingStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Insert bookings
	_, err = bookingService.collection.InsertOne(context.Background(), returnedBooking)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert returned booking"))

	_, err = bookingService.collection.InsertOne(context.Background(), pickedBooking)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert picked booking"))

	_, err = bookingService.collection.InsertOne(context.Background(), pendingBooking)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert pending booking"))

	// Test GetPendingRatings for userID1
	page := 0
	pageSize := DefaultPageSize
	pendingRatings, _, err := bookingService.GetPendingRatings(context.Background(), userID1, page, pageSize)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get pending ratings"))

	// Should return both returned and picked bookings
	qt.Assert(t, len(pendingRatings), qt.Equals, 2, qt.Commentf("Should return both returned and picked bookings"))

	// Verify that the returned bookings have the correct statuses
	foundReturned := false
	foundPicked := false

	for _, booking := range pendingRatings {
		if booking.ID == returnedBooking.ID {
			qt.Assert(t, booking.BookingStatus, qt.Equals, BookingStatusReturned)
			foundReturned = true
		}
		if booking.ID == pickedBooking.ID {
			qt.Assert(t, booking.BookingStatus, qt.Equals, BookingStatusPicked)
			foundPicked = true
		}
	}

	qt.Assert(t, foundReturned, qt.IsTrue, qt.Commentf("Returned booking should be included in pending ratings"))
	qt.Assert(t, foundPicked, qt.IsTrue, qt.Commentf("Picked booking should be included in pending ratings"))

	// Now add a rating for the returned booking
	rating := &Rating{
		BookingID: returnedBooking.ID,
		RaterID:   userID1,
		RateeID:   userID2,
		Score:     5,
		CreatedAt: now,
	}

	_, err = bookingService.ratingsCollection.InsertOne(context.Background(), rating)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to insert rating"))

	// Test GetPendingRatings again
	pendingRatings, _, err = bookingService.GetPendingRatings(context.Background(), userID1, page, pageSize)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get pending ratings after adding rating"))

	// Should only return the picked booking now
	qt.Assert(t, len(pendingRatings), qt.Equals, 1, qt.Commentf("Should only return the picked booking now"))
	qt.Assert(t, pendingRatings[0].ID, qt.Equals, pickedBooking.ID)
	qt.Assert(t, pendingRatings[0].BookingStatus, qt.Equals, BookingStatusPicked)
}
