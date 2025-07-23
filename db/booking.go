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
	BookingStatusPicked    BookingStatus = "PICKED"
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
	IsNomadic     bool               `bson:"isNomadic" json:"isNomadic"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
	PickupPlace   *DBLocation        `bson:"pickupPlace,omitempty" json:"pickupPlace,omitempty"`
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
	IsNomadic bool      `bson:"isNomadic" json:"isNomadic"`
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
		IsNomadic:     req.IsNomadic,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Check for date conflicts
	conflictExists, err := s.CheckDateConflicts(ctx, booking.ToolID, booking.StartDate, booking.EndDate, primitive.NilObjectID)
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
func (s *BookingService) Get(ctx context.Context, id primitive.ObjectID) (*Booking, error) {
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

	var results []Booking
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, ErrBookingNotFound
	}
	return &results[0], nil
}

type BookingType string

const (
	OutgoingBookings BookingType = "fromUserId"
	IncomingBookings BookingType = "toUserId"
)

// GetUserBookings returns paginated bookings where the given user is either the requester (fromUserId)
// or the owner (toUserId), depending on the userField parameter.
// Bookings are ordered with PENDING status first, then sorted by createdAt date (newest first).
func (s *BookingService) GetUserBookings(
	ctx context.Context,
	userID primitive.ObjectID,
	userField BookingType,
	page int,
	pageSize int,
) ([]*Booking, int64, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	skip := page * pageSize

	// Build the aggregation pipeline
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{string(userField): userID}}},
		{{Key: "$addFields", Value: bson.M{
			"isPending": bson.M{
				"$cond": bson.M{
					"if": bson.M{
						"$and": []interface{}{
							bson.M{"$eq": []interface{}{"$bookingStatus", BookingStatusPending}},
							bson.M{"$gte": []interface{}{"$startDate", time.Now()}},
						},
					},
					"then": 1,
					"else": 0,
				},
			},
		}}},
		{{Key: "$sort", Value: bson.D{
			{Key: "isPending", Value: -1},
			{Key: "createdAt", Value: -1},
		}}},
		{{Key: "$facet", Value: bson.D{
			{Key: "data", Value: bson.A{
				bson.D{{Key: "$skip", Value: skip}},
				bson.D{{Key: "$limit", Value: pageSize}},
				bson.D{{Key: "$lookup", Value: bson.M{
					"from":         s.ratingsCollection.Name(),
					"localField":   "_id",
					"foreignField": "bookingId",
					"as":           "ratings",
				}}},
			}},
			{Key: "count", Value: bson.A{
				bson.D{{Key: "$count", Value: "total"}},
			}},
		}}},
	}

	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var result []struct {
		Data  []*Booking `bson:"data"`
		Count []struct {
			Total int64 `bson:"total"`
		} `bson:"count"`
	}

	if err := cursor.All(ctx, &result); err != nil {
		return nil, 0, err
	}

	var bookings []*Booking
	var total int64
	if len(result) > 0 {
		bookings = result[0].Data
		if len(result[0].Count) > 0 {
			total = result[0].Count[0].Total
		}
	}

	return bookings, total, nil
}

// GetPendingBookingsForTool returns all pending bookings for a specific tool.
func (s *BookingService) GetPendingBookingsForTool(ctx context.Context, toolID string) ([]*Booking, error) {
	filter := bson.M{
		"toolId":        toolID,
		"bookingStatus": BookingStatusPending,
	}

	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var results []Booking
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	bookings := make([]*Booking, len(results))
	for i := range results {
		bookings[i] = &results[i]
	}
	return bookings, nil
}

// GetBookingsForTool returns all bookings for a specific tool.
func (s *BookingService) GetBookingsForTool(ctx context.Context, toolID string) ([]*Booking, error) {
	filter := bson.M{
		"toolId": toolID,
	}

	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var results []Booking
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	bookings := make([]*Booking, len(results))
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
	if status == BookingStatusAccepted || status == BookingStatusReturned || status == BookingStatusPicked {
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

		// If accepting, check for date conflicts and user tokens
		if status == BookingStatusAccepted {
			// Check for date conflicts before accepting
			conflictExists, err := s.CheckDateConflicts(ctx, booking.ToolID, booking.StartDate, booking.EndDate, booking.ID)
			if err != nil {
				return fmt.Errorf("error checking date conflicts: %w", err)
			}
			if conflictExists {
				return ErrBookingDatesConflict
			}

			userService := s.database.Collection("users")
			var fromUser User
			err = userService.FindOne(ctx, bson.M{"_id": booking.FromUserID}).Decode(&fromUser)
			if err != nil {
				return fmt.Errorf("error finding user: %w", err)
			}

			tokenCost := s.calculateTokenCost(booking, tool)
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

		// If returning or picking, add tokens to lending user
		if status == BookingStatusReturned || status == BookingStatusPicked {
			userService := s.database.Collection("users")
			tokenCost := s.calculateTokenCost(booking, tool)

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

	// todo(kon): should be handle BookingStatusPicked dates somehow?
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

// SetPickupPlace sets the pickup place for a booking
func (s *BookingService) SetPickupPlace(ctx context.Context, id primitive.ObjectID, location DBLocation) error {
	update := bson.M{
		"$set": bson.M{
			"pickupPlace": location,
			"updatedAt":   time.Now(),
		},
	}

	result, err := s.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrBookingNotFound
	}
	return nil
}

// UpdateFutureBookingsResult represents the result of updating future bookings
type UpdateFutureBookingsResult struct {
	ModifiedCount int64                `json:"modifiedCount"`
	FromUserIDs   []primitive.ObjectID `json:"fromUserIds"`
}

// UpdateFutureBookingsActualHolder gets all future bookings for a tool and updates their actual holder in one operation.
// It excludes the specified booking ID to avoid updating the booking that was just marked as PICKED.
func (s *BookingService) UpdateFutureBookingsActualHolder(
	ctx context.Context,
	toolID string,
	fromDate time.Time,
	newToUserID primitive.ObjectID,
	excludeBookingID primitive.ObjectID,
) (*UpdateFutureBookingsResult, error) {
	// First, get the fromUserIds of bookings that will be updated
	filter := bson.M{
		"toolId": toolID,
		"bookingStatus": bson.M{
			"$in": []BookingStatus{BookingStatusPending, BookingStatusAccepted},
		},
		"startDate": bson.M{"$gte": fromDate},
	}

	// Exclude the booking that was just marked as PICKED
	if excludeBookingID != primitive.NilObjectID {
		filter["_id"] = bson.M{"$ne": excludeBookingID}
	}

	// Use aggregation to get the fromUserIds before updating
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{
			"_id":         nil,
			"fromUserIds": bson.M{"$addToSet": "$fromUserId"},
			"count":       bson.M{"$sum": 1},
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

	var aggregationResult []struct {
		FromUserIDs []primitive.ObjectID `bson:"fromUserIds"`
		Count       int64                `bson:"count"`
	}

	if err := cursor.All(ctx, &aggregationResult); err != nil {
		return nil, err
	}

	var fromUserIDs []primitive.ObjectID
	var expectedCount int64

	if len(aggregationResult) > 0 {
		fromUserIDs = aggregationResult[0].FromUserIDs
		expectedCount = aggregationResult[0].Count
	}

	// If no bookings to update, return early
	if expectedCount == 0 {
		log.Debug().
			Str("toolId", toolID).
			Str("excludeBookingId", excludeBookingID.Hex()).
			Msg("No future bookings to update")

		return &UpdateFutureBookingsResult{
			ModifiedCount: 0,
			FromUserIDs:   []primitive.ObjectID{},
		}, nil
	}

	// Now perform the update
	update := bson.M{
		"$set": bson.M{
			"toUserId":  newToUserID,
			"updatedAt": time.Now(),
		},
	}

	result, err := s.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	log.Debug().
		Str("toolId", toolID).
		Str("excludeBookingId", excludeBookingID.Hex()).
		Int64("modifiedCount", result.ModifiedCount).
		Str("newToUserId", newToUserID.Hex()).
		Interface("fromUserIds", fromUserIDs).
		Msg("Updated future bookings actual holder")

	return &UpdateFutureBookingsResult{
		ModifiedCount: result.ModifiedCount,
		FromUserIDs:   fromUserIDs,
	}, nil
}

// checkDateConflicts checks if there are any conflicting bookings for the given tool and dates.
// It takes a tool ID, start and end times, and an optional booking ID to exclude from the check.
func (s *BookingService) CheckDateConflicts(
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
