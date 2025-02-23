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
	"go.mongodb.org/mongo-driver/mongo/options"
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

// BookingRating represents a rating given by a user for a booking.
type BookingRating struct {
	UserID        primitive.ObjectID `bson:"userId" json:"userId"`
	Rating        int                `bson:"rating" json:"rating"`
	RatingComment string             `bson:"ratingComment,omitempty" json:"ratingComment,omitempty"`
	RatedAt       time.Time          `bson:"ratedAt" json:"ratedAt"`
}

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
	// Ratings now holds zero, one, or two ratings (one from each involved user).
	Ratings   []BookingRating `bson:"ratings,omitempty" json:"ratings,omitempty"`
	CreatedAt time.Time       `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time       `bson:"updatedAt" json:"updatedAt"`
}

// BookingService handles all booking related database operations
type BookingService struct {
	collection *mongo.Collection
	database   *mongo.Database
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
		collection: collection,
		database:   db,
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

// Get retrieves a booking by ID
func (s *BookingService) Get(ctx context.Context, id primitive.ObjectID) (*Booking, error) {
	var booking Booking
	err := s.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&booking)
	if err == mongo.ErrNoDocuments {
		return nil, ErrBookingNotFound
	}
	return &booking, err
}

// GetUserBookings gets paginated bookings for a user (both requests and petitions)
func (s *BookingService) GetUserBookings(ctx context.Context, userID primitive.ObjectID, page int) ([]*Booking, error) {
	if page < 0 {
		page = 0
	}

	skip := page * defaultPageSize

	// Find bookings where user is either the requester or owner
	cursor, err := s.collection.Find(ctx,
		bson.M{
			"$or": []bson.M{
				{"fromUserId": userID},
				{"toUserId": userID},
			},
		},
		options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}). // Sort by date, newest first
			SetSkip(int64(skip)).
			SetLimit(int64(defaultPageSize)),
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var bookings []*Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		return nil, err
	}
	return bookings, nil
}

// GetUserRequests gets all booking requests for tools owned by the user
func (s *BookingService) GetUserRequests(ctx context.Context, userID primitive.ObjectID) ([]*Booking, error) {
	cursor, err := s.collection.Find(ctx, bson.M{
		"toUserId": userID,
	}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var bookings []*Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		return nil, err
	}
	return bookings, nil
}

// GetUserPetitions gets all bookings made by the user
func (s *BookingService) GetUserPetitions(ctx context.Context, userID primitive.ObjectID) ([]*Booking, error) {
	cursor, err := s.collection.Find(ctx, bson.M{
		"fromUserId": userID,
	}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var bookings []*Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		return nil, err
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

		// If returning, add tokens to lending user
		if status == BookingStatusReturned {
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

// GetPendingRatings retrieves all bookings that the given user still needs to rate.
func (s *BookingService) GetPendingRatings(ctx context.Context, userID primitive.ObjectID) ([]*Booking, error) {
	filter := bson.M{
		"bookingStatus": BookingStatusReturned,
		"$or": []bson.M{
			{"fromUserId": userID},
			{"toUserId": userID},
		},
		"$expr": bson.M{
			"$and": []interface{}{
				// Ensure fewer than 2 ratings exist.
				bson.M{
					"$lt": []interface{}{
						bson.M{"$size": bson.M{"$ifNull": []interface{}{"$ratings", []interface{}{}}}},
						2,
					},
				},
				// Ensure the current user has not rated it yet.
				bson.M{
					"$not": bson.M{
						"$in": []interface{}{
							userID,
							bson.M{
								"$map": bson.M{
									"input": bson.M{"$ifNull": []interface{}{"$ratings", []interface{}{}}},
									"as":    "r",
									"in":    "$$r.userId",
								},
							},
						},
					},
				},
			},
		},
	}

	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx) // nolint: errcheck

	var bookings []*Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		return nil, err
	}
	return bookings, nil
}

// GetSubmittedRatings retrieves bookings that have been rated by the user (excluding self-ratings).
func (s *BookingService) GetSubmittedRatings(ctx context.Context, userID primitive.ObjectID) ([]*Booking, error) {
	filter := bson.M{
		"ratings": bson.M{
			"$elemMatch": bson.M{"userId": userID},
		},
		"toUserId": bson.M{"$ne": userID}, // Exclude self-ratings.
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

	var bookings []*Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		return nil, err
	}
	return bookings, nil
}

// GetReceivedRatings retrieves bookings that have been rated where the user is the tool owner.
// If the owner has submitted a rating, that rating is returned in preference to the other party's rating.
func (s *BookingService) GetReceivedRatings(ctx context.Context, userID primitive.ObjectID) ([]*Booking, error) {
	// Retrieve all bookings where the tool owner is the current user and ratings exist.
	filter := bson.M{
		"toUserId": userID,
		"ratings":  bson.M{"$exists": true, "$not": bson.M{"$size": 0}},
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

	var bookings []*Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		return nil, err
	}

	// For each booking, if the owner has submitted a rating (i.e. rating.userId equals userID),
	// swap it to the first position so that convertBookingToResponse returns that rating.
	for _, booking := range bookings {
		for i, r := range booking.Ratings {
			if r.UserID == userID {
				if i != 0 {
					booking.Ratings[0], booking.Ratings[i] = booking.Ratings[i], booking.Ratings[0]
				}
				break
			}
		}
	}

	return bookings, nil
}

// RateBooking adds a rating to a booking and then updates the tool's and owner's ratings.
func (s *BookingService) RateBooking(
	ctx context.Context,
	bookingID primitive.ObjectID,
	userID primitive.ObjectID,
	rating int,
	comment string,
) error {
	// Get the booking.
	booking, err := s.Get(ctx, bookingID)
	if err != nil {
		return err
	}
	if booking == nil {
		return ErrBookingNotFound
	}

	// Verify booking is in RETURNED state.
	if booking.BookingStatus != BookingStatusReturned {
		return fmt.Errorf("booking must be in RETURNED state to be rated")
	}

	// Verify user is involved in the booking.
	if booking.FromUserID != userID && booking.ToUserID != userID {
		return fmt.Errorf("user is not involved in this booking")
	}

	// Verify the user hasn't already rated this booking.
	for _, r := range booking.Ratings {
		if r.UserID == userID {
			return fmt.Errorf("user has already rated this booking")
		}
	}

	// Verify rating value.
	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}

	// Create the new rating.
	now := time.Now()
	newRating := BookingRating{
		UserID:        userID,
		Rating:        rating,
		RatingComment: comment,
		RatedAt:       now,
	}

	// Push the new rating into the ratings array.
	update := bson.M{
		"$push": bson.M{
			"ratings": newRating,
		},
		"$set": bson.M{
			"updatedAt": now,
		},
	}

	result, err := s.collection.UpdateOne(ctx, bson.M{"_id": bookingID}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrBookingNotFound
	}

	// Update tool and owner ratings based on the new rating.
	if err := s.updateRatings(ctx, bookingID); err != nil {
		return fmt.Errorf("failed to update ratings: %w", err)
	}

	return nil
}

// updateRatings recalculates and updates the tool's average rating and the tool owner's overall rating
// based on the ratings stored in the booking's Ratings array.
func (s *BookingService) updateRatings(ctx context.Context, bookingID primitive.ObjectID) error {
	// Fetch updated booking.
	booking, err := s.Get(ctx, bookingID)
	if err != nil {
		return err
	}
	if booking == nil {
		return ErrBookingNotFound
	}

	// Update tool's rating.
	toolID, err := strconv.ParseInt(booking.ToolID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid tool ID: %w", err)
	}

	toolPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"toolId":        booking.ToolID,
			"bookingStatus": BookingStatusReturned,
			"ratings":       bson.M{"$exists": true, "$ne": []interface{}{}},
		}}},
		{{Key: "$unwind", Value: "$ratings"}},
		{{Key: "$group", Value: bson.M{
			"_id":         nil,
			"avgRating":   bson.M{"$avg": "$ratings.rating"},
			"ratingCount": bson.M{"$sum": 1},
		}}},
	}

	toolCursor, err := s.collection.Aggregate(ctx, toolPipeline)
	if err != nil {
		return fmt.Errorf("failed to calculate tool average rating: %w", err)
	}
	defer func() {
		if err := toolCursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing tool cursor")
		}
	}()

	var toolResults []struct {
		AvgRating   float64 `bson:"avgRating"`
		RatingCount int     `bson:"ratingCount"`
	}
	if err = toolCursor.All(ctx, &toolResults); err != nil {
		return fmt.Errorf("failed to decode tool average rating: %w", err)
	}

	if len(toolResults) > 0 {
		toolService := s.database.Collection("tools")
		_, err = toolService.UpdateOne(
			ctx,
			bson.M{"_id": toolID},
			bson.M{"$set": bson.M{"rating": int32(math.Round(toolResults[0].AvgRating))}},
		)
		if err != nil {
			return fmt.Errorf("failed to update tool rating: %w", err)
		}
	}

	// Calculate the tool owner's (user's) rating from received ratings.
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"toUserId":      booking.ToUserID,
			"bookingStatus": BookingStatusReturned,
			"ratings":       bson.M{"$exists": true, "$ne": []interface{}{}},
		}}},
		{{Key: "$unwind", Value: "$ratings"}},
		// Only consider ratings given by the other party.
		{{Key: "$match", Value: bson.M{
			"ratings.userId": bson.M{"$ne": booking.ToUserID},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":         nil,
			"avgRating":   bson.M{"$avg": "$ratings.rating"},
			"ratingCount": bson.M{"$sum": 1},
		}}},
	}

	userRatingCursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("failed to calculate user average rating: %w", err)
	}
	defer func() {
		if err := userRatingCursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing user rating cursor")
		}
	}()

	var userResults []struct {
		AvgRating   float64 `bson:"avgRating"`
		RatingCount int     `bson:"ratingCount"`
	}
	if err = userRatingCursor.All(ctx, &userResults); err != nil {
		return fmt.Errorf("failed to decode user average rating: %w", err)
	}

	if len(userResults) > 0 {
		// Convert the 5‑star average to a percentage (5 stars = 100%).
		userRating := int32(math.Round((userResults[0].AvgRating / 5.0) * 100))
		userService := s.database.Collection("users")
		_, err = userService.UpdateOne(
			ctx,
			bson.M{"_id": booking.ToUserID},
			bson.M{"$set": bson.M{"rating": userRating}},
		)
		if err != nil {
			return fmt.Errorf("failed to update user rating: %w", err)
		}
	}

	return nil
}

// CountPendingActions returns the count of pending ratings and booking requests for a user.
func (s *BookingService) CountPendingActions(
	ctx context.Context,
	userID primitive.ObjectID,
) (*CountPendingActionsResponse, error) {
	pipeline := mongo.Pipeline{
		{
			{Key: "$facet", Value: bson.D{
				{Key: "pendingRatings", Value: bson.A{
					bson.D{
						{Key: "$match", Value: bson.M{
							"bookingStatus": BookingStatusReturned,
							"$or": []bson.M{
								{"fromUserId": userID},
								{"toUserId": userID},
							},
							"$expr": bson.M{
								"$and": []interface{}{
									bson.M{
										"$lt": []interface{}{
											bson.M{"$size": bson.M{"$ifNull": []interface{}{"$ratings", []interface{}{}}}},
											2,
										},
									},
									bson.M{
										"$not": bson.M{
											"$in": []interface{}{
												userID,
												bson.M{
													"$map": bson.M{
														"input": bson.M{"$ifNull": []interface{}{"$ratings", []interface{}{}}},
														"as":    "r",
														"in":    "$$r.userId",
													},
												},
											},
										},
									},
								},
							},
						}},
					},
					bson.D{{Key: "$count", Value: "count"}},
				}},
				{Key: "pendingRequests", Value: bson.A{
					bson.D{
						{Key: "$match", Value: bson.M{
							"toUserId":      userID,
							"bookingStatus": BookingStatusPending,
						}},
					},
					bson.D{{Key: "$count", Value: "count"}},
				}},
			}},
		},
		{
			{Key: "$project", Value: bson.D{
				{Key: "pendingRatingsCount", Value: bson.M{"$arrayElemAt": bson.A{"$pendingRatings.count", 0}}},
				{Key: "pendingRequestsCount", Value: bson.M{"$arrayElemAt": bson.A{"$pendingRequests.count", 0}}},
			}},
		},
	}

	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate pending actions: %w", err)
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var result []CountPendingActionsResponse
	if err := cursor.All(ctx, &result); err != nil {
		return nil, fmt.Errorf("failed to parse aggregation result: %w", err)
	}

	if len(result) == 0 {
		return &CountPendingActionsResponse{
			PendingRatingsCount:  0,
			PendingRequestsCount: 0,
		}, nil
	}

	return &result[0], nil
}
