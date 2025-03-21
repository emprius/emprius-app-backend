package db

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// BookingStatus represents the current state of a booking
type BookingStatus string

const (
	BookingStatusPending   BookingStatus = "PENDING"
	BookingStatusAccepted  BookingStatus = "ACCEPTED"
	BookingStatusRejected  BookingStatus = "REJECTED"
	BookingStatusCancelled BookingStatus = "CANCELLED"
	BookingStatusReturned  BookingStatus = "RETURNED"
)

// Booking represents a tool booking in the system.
type Booking struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	ToolID        string             `bson:"toolId" json:"toolId"`
	FromUserID    primitive.ObjectID `bson:"fromUserId" json:"fromUserId"`
	ToUserID      primitive.ObjectID `bson:"toUserId" json:"toUserId"`
	StartDate     time.Time          `bson:"startDate" json:"startDate"`
	EndDate       time.Time          `bson:"endDate" json:"endDate"`
	Contact       string             `bson:"contact" json:"contact"`
	Comments      string             `bson:"comments" json:"comments"`
	BookingStatus BookingStatus      `bson:"bookingStatus" json:"bookingStatus"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// BookingWithRatings is a composite type that embeds a Booking and its associated ratings.
type BookingWithRatings struct {
	Booking `bson:",inline"`
	Ratings []*BookingRating `bson:"ratings" json:"ratings"`
}

// BookingService handles all booking related database operations
type BookingService struct {
	collection *mongo.Collection
	database   *mongo.Database

	ratingsCollection *mongo.Collection
}

// NewBookingService creates a new BookingService instance
func NewBookingService(db *mongo.Database) *BookingService {
	collection := db.Collection("bookings")

	// Create indexes
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "toolId", Value: 1},
				{Key: "startDate", Value: 1},
				{Key: "endDate", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "fromUserId", Value: 1},
				{Key: "createdAt", Value: -1}, // For efficient sorting by date
			},
		},
		{
			Keys: bson.D{
				{Key: "toUserId", Value: 1},
				{Key: "createdAt", Value: -1}, // For efficient sorting by date
			},
		},
	}

	_, err := collection.Indexes().CreateMany(context.Background(), indexes)
	if err != nil {
		panic(err)
	}

	return &BookingService{
		collection:        collection,
		database:          db,
		ratingsCollection: newRatingCollection(db),
	}
}

// CreateBookingRequest represents the request to create a new booking
type CreateBookingRequest struct {
	ToolID    string    `bson:"toolId" json:"toolId"`
	StartDate time.Time `bson:"startDate" json:"startDate"`
	EndDate   time.Time `bson:"endDate" json:"endDate"`
	Contact   string    `bson:"contact" json:"contact"`
	Comments  string    `bson:"comments" json:"comments"`
}

// CountPendingActionsResponse represents the response for CountPendingActions
type CountPendingActionsResponse struct {
	PendingRatingsCount  int64 `json:"pendingRatingsCount"`
	PendingRequestsCount int64 `json:"pendingRequestsCount"`
}

// Create creates a new booking
func (s *BookingService) Create(
	ctx context.Context,
	req *CreateBookingRequest,
	fromUserID, toUserID primitive.ObjectID,
) (*Booking, error) {
	// Set timestamps
	now := time.Now()

	booking := &Booking{
		ToolID:        req.ToolID,
		FromUserID:    fromUserID,
		ToUserID:      toUserID,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		Contact:       req.Contact,
		Comments:      req.Comments,
		BookingStatus: BookingStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Check for date conflicts
	conflictExists, err := s.checkDateConflicts(ctx, booking.ToolID, booking.StartDate, booking.EndDate, primitive.NilObjectID)
	if err != nil {
		return nil, err
	}
	if conflictExists {
		return nil, ErrBookingDatesConflict
	}

	result, err := s.collection.InsertOne(ctx, booking)
	if err != nil {
		return nil, err
	}

	booking.ID = result.InsertedID.(primitive.ObjectID)
	return booking, nil
}

// Get retrieves a booking (by ID) along with its related ratings in one query.
func (s *BookingService) Get(ctx context.Context, id primitive.ObjectID) (*BookingWithRatings, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"_id": id}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         s.ratingsCollection.Name(), // ratings collection
			"localField":   "_id",
			"foreignField": "bookingId",
			"as":           "ratings",
		}}},
	}
	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var results []BookingWithRatings
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, ErrBookingNotFound
	}
	return &results[0], nil
}

// GetUserBookings returns paginated bookings for a user along with their associated ratings.
func (s *BookingService) GetUserBookings(
	ctx context.Context,
	userID primitive.ObjectID,
	page int,
) ([]*BookingWithRatings, error) {
	if page < 0 {
		page = 0
	}
	skip := page * defaultPageSize

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"$or": []bson.M{
				{"fromUserId": userID},
				{"toUserId": userID},
			},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "createdAt", Value: -1}}}},
		{{Key: "$skip", Value: skip}},
		{{Key: "$limit", Value: defaultPageSize}},
		{{Key: "$lookup", Value: bson.M{
			"from":         s.ratingsCollection.Name(),
			"localField":   "_id",
			"foreignField": "bookingId",
			"as":           "ratings",
		}}},
	}

	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var results []BookingWithRatings
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	// Convert slice to []*BookingWithRatings
	bookings := make([]*BookingWithRatings, len(results))
	for i := range results {
		bookings[i] = &results[i]
	}
	return bookings, nil
}

// GetUserRequests returns all bookings where the given user is the owner (toUserId)
// along with their associated ratings.
// Bookings are ordered with PENDING status first, then sorted by createdAt date (newest first).
func (s *BookingService) GetUserRequests(ctx context.Context, userID primitive.ObjectID) ([]*BookingWithRatings, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"toUserId": userID}}},
		// Add a field to use for sorting (1 for PENDING status, 0 for others)
		{{Key: "$addFields", Value: bson.M{
			"isPending": bson.M{
				"$cond": bson.M{
					"if":   bson.M{"$eq": []interface{}{"$bookingStatus", BookingStatusPending}},
					"then": 1,
					"else": 0,
				},
			},
		}}},
		// Sort by isPending (descending to put PENDING first), then by createdAt (newest first)
		{{Key: "$sort", Value: bson.D{
			{Key: "isPending", Value: -1},
			{Key: "createdAt", Value: -1},
		}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         s.ratingsCollection.Name(),
			"localField":   "_id",
			"foreignField": "bookingId",
			"as":           "ratings",
		}}},
	}
	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var results []BookingWithRatings
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	bookings := make([]*BookingWithRatings, len(results))
	for i := range results {
		bookings[i] = &results[i]
	}
	return bookings, nil
}

// GetUserPetitions returns all bookings where the given user is the requester (fromUserId)
// along with their associated ratings.
// Bookings are ordered with PENDING status first, then sorted by createdAt date (newest first).
func (s *BookingService) GetUserPetitions(ctx context.Context, userID primitive.ObjectID) ([]*BookingWithRatings, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"fromUserId": userID}}},
		// Add a field to use for sorting (1 for PENDING status, 0 for others)
		{{Key: "$addFields", Value: bson.M{
			"isPending": bson.M{
				"$cond": bson.M{
					"if":   bson.M{"$eq": []interface{}{"$bookingStatus", BookingStatusPending}},
					"then": 1,
					"else": 0,
				},
			},
		}}},
		// Sort by isPending (descending to put PENDING first), then by createdAt (newest first)
		{{Key: "$sort", Value: bson.D{
			{Key: "isPending", Value: -1},
			{Key: "createdAt", Value: -1},
		}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         s.ratingsCollection.Name(),
			"localField":   "_id",
			"foreignField": "bookingId",
			"as":           "ratings",
		}}},
	}
	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var results []BookingWithRatings
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	bookings := make([]*BookingWithRatings, len(results))
	for i := range results {
		bookings[i] = &results[i]
	}
	return bookings, nil
}

// calculateTokenCost calculates the total token cost for a booking
func (s *BookingService) calculateTokenCost(booking *Booking, tool *Tool) uint64 {
	days := uint64(math.Ceil(booking.EndDate.Sub(booking.StartDate).Hours() / 24))
	return tool.Cost * days
}

// UpdateStatus updates the booking status and handles any related updates
func (s *BookingService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status BookingStatus) error {
	// Get the booking first
	booking, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if booking == nil {
		return ErrBookingNotFound
	}

	// If accepting booking or returning, we need the tool information
	var tool *Tool
	if status == BookingStatusAccepted || status == BookingStatusReturned {
		toolService := NewToolService(&Database{Database: s.database})

		// Convert tool ID from string to int64
		toolID, err := strconv.ParseInt(booking.ToolID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid tool ID: %w", err)
		}

		// Get tool
		tool, err = toolService.GetToolByID(ctx, toolID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return fmt.Errorf("tool not found: %d", toolID)
			}
			return fmt.Errorf("error finding tool: %w", err)
		}

		// If accepting, check if user has enough tokens
		if status == BookingStatusAccepted {
			userService := s.database.Collection("users")
			var fromUser User
			err = userService.FindOne(ctx, bson.M{"_id": booking.FromUserID}).Decode(&fromUser)
			if err != nil {
				return fmt.Errorf("error finding user: %w", err)
			}

			tokenCost := s.calculateTokenCost(&booking.Booking, tool)
			if fromUser.Tokens < tokenCost {
				return fmt.Errorf("insufficient tokens: user has %d, needs %d", fromUser.Tokens, tokenCost)
			}

			// Deduct tokens from renting user
			_, err = userService.UpdateOne(ctx,
				bson.M{"_id": booking.FromUserID},
				bson.M{"$inc": bson.M{"tokens": -int64(tokenCost)}},
			)
			if err != nil {
				return fmt.Errorf("error updating user tokens: %w", err)
			}
		}

		// If returning, add tokens to lending user
		if status == BookingStatusReturned {
			userService := s.database.Collection("users")
			tokenCost := s.calculateTokenCost(&booking.Booking, tool)

			_, err = userService.UpdateOne(ctx,
				bson.M{"_id": booking.ToUserID},
				bson.M{"$inc": bson.M{"tokens": int64(tokenCost)}},
			)
			if err != nil {
				return fmt.Errorf("error updating user tokens: %w", err)
			}
		}
	}

	// Update booking status
	update := bson.M{
		"$set": bson.M{
			"bookingStatus": status,
			"updatedAt":     time.Now(),
		},
	}

	result, err := s.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrBookingNotFound
	}

	// Handle tool's reserved dates based on status
	if status == BookingStatusAccepted || status == BookingStatusReturned {
		toolService := s.database.Collection("tools")
		toolID, _ := strconv.ParseInt(booking.ToolID, 10, 64) // Error already checked above

		var update bson.M
		if status == BookingStatusAccepted {
			// Add reserved dates to tool
			update = bson.M{
				"$push": bson.M{
					"reservedDates": bson.M{
						"from": uint32(booking.StartDate.Unix()),
						"to":   uint32(booking.EndDate.Unix()),
					},
				},
			}
		} else { // BookingStatusReturned
			// Remove reserved dates from tool
			update = bson.M{
				"$pull": bson.M{
					"reservedDates": bson.M{
						"from": uint32(booking.StartDate.Unix()),
						"to":   uint32(booking.EndDate.Unix()),
					},
				},
			}
		}

		_, err = toolService.UpdateOne(ctx, bson.M{"_id": toolID}, update)
		if err != nil {
			// If tool update fails, revert booking status
			revertUpdate := bson.M{
				"$set": bson.M{
					"bookingStatus": booking.BookingStatus, // Revert to previous status
					"updatedAt":     time.Now(),
				},
			}
			_, revertErr := s.collection.UpdateOne(ctx, bson.M{"_id": id}, revertUpdate)
			if revertErr != nil {
				return fmt.Errorf("failed to update tool and revert booking status: %v, %v", err, revertErr)
			}
			return fmt.Errorf("could not update tool reserved dates: %w", err)
		}
	}

	return nil
}

// checkDateConflicts checks if there are any conflicting bookings for the given tool and dates.
// It takes a tool ID, start and end times, and an optional booking ID to exclude from the check.
func (s *BookingService) checkDateConflicts(
	ctx context.Context,
	toolID string,
	start, end time.Time,
	excludeID primitive.ObjectID,
) (bool, error) {
	filter := bson.M{
		"toolId":        toolID,
		"bookingStatus": BookingStatusAccepted,
		"$or": []bson.M{
			{
				"startDate": bson.M{"$lte": end},
				"endDate":   bson.M{"$gte": start},
			},
		},
	}

	// Exclude the current booking if updating
	if excludeID != primitive.NilObjectID {
		filter["_id"] = bson.M{"$ne": excludeID}
	}

	count, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
