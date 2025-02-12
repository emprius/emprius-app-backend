package db

import (
	"context"
	"strconv"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// createTestTool creates a test tool in the database
func createTestTool(ctx context.Context, db *mongo.Database, userID primitive.ObjectID) (int64, error) {
	toolService := db.Collection("tools")
	toolID := time.Now().UnixNano() % 10000000 // Generate a unique tool ID

	tool := &Tool{
		ID:            toolID,
		Title:         "Test Tool",
		Description:   "Test Description",
		UserID:        userID,
		IsAvailable:   true,
		ReservedDates: []DateRange{}, // Initialize as empty array
	}

	_, err := toolService.InsertOne(ctx, tool)
	if err != nil {
		return 0, err
	}

	return toolID, nil
}

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
		// Create test users and tool
		fromUserID := primitive.NewObjectID()
		toUserID := primitive.NewObjectID()
		toolID, err := createTestTool(ctx, database, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test tool"))

		req := &CreateBookingRequest{
			ToolID:    strconv.FormatInt(toolID, 10),
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
		// Create test users and tool
		fromUserID := primitive.NewObjectID()
		toUserID := primitive.NewObjectID()
		toolID, err := createTestTool(ctx, database, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test tool"))

		// Create and accept initial booking
		req1 := &CreateBookingRequest{
			ToolID:    strconv.FormatInt(toolID, 10),
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
			ToolID:    strconv.FormatInt(toolID, 10),
			StartDate: time.Now().Add(36 * time.Hour),
			EndDate:   time.Now().Add(60 * time.Hour),
			Contact:   "test2@example.com",
		}
		_, err = bookingService.Create(ctx, req2, primitive.NewObjectID(), toUserID)
		c.Assert(err, qt.Not(qt.IsNil), qt.Commentf("Expected error for overlapping booking"))
	})

	c.Run("Get User Requests", func(c *qt.C) {
		toUserID := primitive.NewObjectID()
		toolID, err := createTestTool(ctx, database, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test tool"))

		// Create multiple bookings
		for i := 0; i < 3; i++ {
			req := &CreateBookingRequest{
				ToolID:    strconv.FormatInt(toolID, 10),
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
		toUserID := primitive.NewObjectID()
		toolID, err := createTestTool(ctx, database, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test tool"))

		// Create multiple bookings
		for i := 0; i < 3; i++ {
			req := &CreateBookingRequest{
				ToolID:    strconv.FormatInt(toolID, 10),
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
		toUserID := primitive.NewObjectID()
		toolID, err := createTestTool(ctx, database, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test tool"))

		req := &CreateBookingRequest{
			ToolID:    strconv.FormatInt(toolID, 10),
			StartDate: time.Now().Add(24 * time.Hour),
			EndDate:   time.Now().Add(48 * time.Hour),
			Contact:   "test@example.com",
		}
		fromUserID := primitive.NewObjectID()
		booking, err := bookingService.Create(ctx, req, fromUserID, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create booking"))

		// Update status
		err = bookingService.UpdateStatus(ctx, booking.ID, BookingStatusAccepted)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to update booking status"))

		// Verify update
		updated, err := bookingService.Get(ctx, booking.ID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get updated booking"))
		c.Assert(updated.BookingStatus, qt.Equals, BookingStatusAccepted, qt.Commentf("Status should be updated to accepted"))
	})

	c.Run("Reserved Dates Management", func(c *qt.C) {
		fromUserID := primitive.NewObjectID()
		toUserID := primitive.NewObjectID()
		toolID, err := createTestTool(ctx, database, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test tool"))

		// Create a booking
		startDate := time.Now().Add(24 * time.Hour)
		endDate := time.Now().Add(48 * time.Hour)
		req := &CreateBookingRequest{
			ToolID:    strconv.FormatInt(toolID, 10),
			StartDate: startDate,
			EndDate:   endDate,
			Contact:   "test@example.com",
		}
		booking, err := bookingService.Create(ctx, req, fromUserID, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create booking"))

		// Accept the booking
		err = bookingService.UpdateStatus(ctx, booking.ID, BookingStatusAccepted)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to accept booking"))

		// Verify dates are added to tool's reservedDates
		var tool Tool
		err = database.Collection("tools").FindOne(ctx, bson.M{"_id": toolID}).Decode(&tool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get tool"))
		c.Assert(len(tool.ReservedDates), qt.Equals, 1, qt.Commentf("Tool should have one reserved date range"))
		c.Assert(tool.ReservedDates[0].From, qt.Equals, uint32(startDate.Unix()), qt.Commentf("Start date should match"))
		c.Assert(tool.ReservedDates[0].To, qt.Equals, uint32(endDate.Unix()), qt.Commentf("End date should match"))

		// Mark booking as returned
		err = bookingService.UpdateStatus(ctx, booking.ID, BookingStatusReturned)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to mark booking as returned"))

		// Verify dates are removed from tool's reservedDates
		err = database.Collection("tools").FindOne(ctx, bson.M{"_id": toolID}).Decode(&tool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get tool"))
		c.Assert(len(tool.ReservedDates), qt.Equals, 0, qt.Commentf("Tool should have no reserved dates after return"))
	})

	c.Run("Rating Functionality", func(c *qt.C) {
		fromUserID := primitive.NewObjectID()
		toUserID := primitive.NewObjectID()
		toolID, err := createTestTool(ctx, database, toUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test tool"))

		// Create multiple returned bookings
		var bookings []*Booking
		for i := 0; i < 3; i++ {
			req := &CreateBookingRequest{
				ToolID:    strconv.FormatInt(toolID, 10),
				StartDate: time.Now().Add(-time.Duration(i+2) * 24 * time.Hour),
				EndDate:   time.Now().Add(-time.Duration(i+1) * 24 * time.Hour),
				Contact:   "test@example.com",
			}
			booking, err := bookingService.Create(ctx, req, fromUserID, toUserID)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to create test booking"))

			// Mark as returned
			err = bookingService.UpdateStatus(ctx, booking.ID, BookingStatusReturned)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to update booking status"))

			bookings = append(bookings, booking)
		}

		// Test GetPendingRatings
		ratings, err := bookingService.GetPendingRatings(ctx, fromUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending ratings"))
		c.Assert(len(ratings), qt.Equals, 3, qt.Commentf("Expected three pending ratings"))

		// Rate first booking
		err = bookingService.RateBooking(ctx, bookings[0].ID, fromUserID, 5, "Great experience!")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to rate booking"))

		// Verify pending ratings decreased
		ratings, err = bookingService.GetPendingRatings(ctx, fromUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending ratings"))
		c.Assert(len(ratings), qt.Equals, 2, qt.Commentf("Expected two pending ratings after rating one"))

		// Rate second booking
		err = bookingService.RateBooking(ctx, bookings[1].ID, fromUserID, 4, "Good tool")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to rate booking"))

		// Verify pending ratings decreased again
		ratings, err = bookingService.GetPendingRatings(ctx, fromUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending ratings"))
		c.Assert(len(ratings), qt.Equals, 1, qt.Commentf("Expected one pending rating after rating two"))

		// Try to rate first booking again (should fail)
		err = bookingService.RateBooking(ctx, bookings[0].ID, fromUserID, 3, "Trying to rate again")
		c.Assert(err, qt.Not(qt.IsNil), qt.Commentf("Should not be able to rate twice"))
		c.Assert(err.Error(), qt.Equals, "user has already rated this booking")

		// Verify tool's rating was updated to average
		var tool Tool
		err = database.Collection("tools").FindOne(ctx, bson.M{"_id": toolID}).Decode(&tool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get tool"))
		c.Assert(tool.Rating, qt.Equals, int32(5), qt.Commentf("Tool rating should be updated to average"))

		// Rate third booking
		err = bookingService.RateBooking(ctx, bookings[2].ID, fromUserID, 3, "Average experience")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to rate booking"))

		// Verify no more pending ratings
		ratings, err = bookingService.GetPendingRatings(ctx, fromUserID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get pending ratings"))
		c.Assert(len(ratings), qt.Equals, 0, qt.Commentf("Expected no pending ratings after rating all"))

		// Verify final tool rating average
		err = database.Collection("tools").FindOne(ctx, bson.M{"_id": toolID}).Decode(&tool)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to get tool"))
		c.Assert(tool.Rating, qt.Equals, int32(4), qt.Commentf("Tool rating should be updated to final average"))
	})
}
