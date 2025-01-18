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

func TestBookingService(t *testing.T) {
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

	// Initialize BookingService
	bookingService := NewBookingService(database)

	c.Run("Create and Get Booking", func(c *qt.C) {
		// Create test booking
		toolID := "123456"
		fromUserID := primitive.NewObjectID()
		toUserID := primitive.NewObjectID()

		req := &CreateBookingRequest{
			ToolID:    toolID,
			StartDate: time.Now().Add(24 * time.Hour),
			EndDate:   time.Now().Add(48 * time.Hour),
			Contact:   "test@example.com",
			Comments:  "Test booking",
		}

		// Create booking
		booking, err := bookingService.Create(ctx, req, fromUserID, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create booking"))
		c.Assert(booking.ID, qt.Not(qt.Equals), primitive.NilObjectID, qt.Commentf("Booking ID should not be nil"))
		c.Assert(booking.BookingStatus, qt.Equals, BookingStatusPending, qt.Commentf("Initial status should be pending"))

		// Get booking
		retrieved, err := bookingService.Get(ctx, booking.ID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get booking"))
		c.Assert(retrieved, qt.Not(qt.IsNil), qt.Commentf("Retrieved booking should not be nil"))
		c.Assert(retrieved.ID, qt.Equals, booking.ID, qt.Commentf("Booking IDs should match"))
		c.Assert(retrieved.ToolID, qt.Equals, booking.ToolID, qt.Commentf("Tool IDs should match"))
		c.Assert(retrieved.FromUserID, qt.Equals, booking.FromUserID, qt.Commentf("FromUserIDs should match"))
	})

	c.Run("Date Conflict Detection", func(c *qt.C) {
		toolID := "123456"
		fromUserID := primitive.NewObjectID()
		toUserID := primitive.NewObjectID()

		// Create and accept initial booking
		req1 := &CreateBookingRequest{
			ToolID:    toolID,
			StartDate: time.Now().Add(24 * time.Hour),
			EndDate:   time.Now().Add(48 * time.Hour),
			Contact:   "test1@example.com",
		}
		booking1, err := bookingService.Create(ctx, req1, fromUserID, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create first booking"))

		// Accept first booking
		err = bookingService.UpdateStatus(ctx, booking1.ID, BookingStatusAccepted)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to accept first booking"))

		// Try to create overlapping booking (should fail since first booking is accepted)
		req2 := &CreateBookingRequest{
			ToolID:    toolID,
			StartDate: time.Now().Add(36 * time.Hour),
			EndDate:   time.Now().Add(60 * time.Hour),
			Contact:   "test2@example.com",
		}
		_, err = bookingService.Create(ctx, req2, primitive.NewObjectID(), toUserID)
		c.Assert(err, qt.Not(qt.IsNil), qt.Commentf("Expected error for overlapping booking"))
	})

	c.Run("Get User Requests", func(c *qt.C) {
		toUserID := primitive.NewObjectID()

		// Create multiple bookings
		for i := 0; i < 3; i++ {
			req := &CreateBookingRequest{
				ToolID:    "789012",
				StartDate: time.Now().Add(time.Duration(i+1) * 24 * time.Hour),
				EndDate:   time.Now().Add(time.Duration(i+2) * 24 * time.Hour),
				Contact:   "test@example.com",
			}
			_, err := bookingService.Create(ctx, req, primitive.NewObjectID(), toUserID)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test booking"))
		}

		// Get requests
		requests, err := bookingService.GetUserRequests(ctx, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get user requests"))
		c.Assert(len(requests), qt.Equals, 3, qt.Commentf("Expected 3 requests"))
	})

	c.Run("Get User Petitions", func(c *qt.C) {
		fromUserID := primitive.NewObjectID()

		// Create multiple bookings
		for i := 0; i < 3; i++ {
			req := &CreateBookingRequest{
				ToolID:    "345678",
				StartDate: time.Now().Add(time.Duration(i+1) * 24 * time.Hour),
				EndDate:   time.Now().Add(time.Duration(i+2) * 24 * time.Hour),
				Contact:   "test@example.com",
			}
			_, err := bookingService.Create(ctx, req, fromUserID, primitive.NewObjectID())
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test booking"))
		}

		// Get petitions
		petitions, err := bookingService.GetUserPetitions(ctx, fromUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get user petitions"))
		c.Assert(len(petitions), qt.Equals, 3, qt.Commentf("Expected 3 petitions"))
	})

	c.Run("Update Booking Status", func(c *qt.C) {
		req := &CreateBookingRequest{
			ToolID:    "901234",
			StartDate: time.Now().Add(24 * time.Hour),
			EndDate:   time.Now().Add(48 * time.Hour),
			Contact:   "test@example.com",
		}
		booking, err := bookingService.Create(ctx, req, primitive.NewObjectID(), primitive.NewObjectID())
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create booking"))

		// Update status
		err = bookingService.UpdateStatus(ctx, booking.ID, BookingStatusAccepted)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to update booking status"))

		// Verify update
		updated, err := bookingService.Get(ctx, booking.ID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get updated booking"))
		c.Assert(updated.BookingStatus, qt.Equals, BookingStatusAccepted, qt.Commentf("Status should be updated to accepted"))
	})

	c.Run("Get Pending Ratings", func(c *qt.C) {
		userID := primitive.NewObjectID()

		// Create returned booking
		req := &CreateBookingRequest{
			ToolID:    "567890",
			StartDate: time.Now().Add(-48 * time.Hour),
			EndDate:   time.Now().Add(-24 * time.Hour),
			Contact:   "test@example.com",
		}
		booking, err := bookingService.Create(ctx, req, userID, primitive.NewObjectID())
		if err != nil {
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test booking"))
		}

		// Update status to returned
		err = bookingService.UpdateStatus(ctx, booking.ID, BookingStatusReturned)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test booking"))

		// Get pending ratings
		ratings, err := bookingService.GetPendingRatings(ctx, userID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending ratings"))
		c.Assert(len(ratings), qt.Not(qt.Equals), 0, qt.Commentf("Expected at least one pending rating"))
	})
}
