package db

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/emprius/emprius-app-backend/types"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Rating represents a rating given by a user for a booking.
type Rating struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	BookingID primitive.ObjectID `bson:"bookingId" json:"bookingId"`
	RaterID   primitive.ObjectID `bson:"raterId" json:"raterId"`
	RateeID   primitive.ObjectID `bson:"rateeId" json:"rateeId"`
	Score     int                `bson:"score" json:"score"`
	Comment   string             `bson:"comment,omitempty" json:"comment,omitempty"`
	Images    []Image            `bson:"images,omitempty" json:"images,omitempty"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

// BookingRating represents the legacy rating model for API compatibility
type BookingRating struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	BookingID     primitive.ObjectID `bson:"bookingId" json:"bookingId"`
	FromUserID    primitive.ObjectID `bson:"fromUserId" json:"fromUserId"`
	ToUserID      primitive.ObjectID `bson:"toUserId" json:"toUserId"`
	Rating        int                `bson:"rating" json:"rating"`
	RatingComment string             `bson:"ratingComment,omitempty" json:"ratingComment,omitempty"`
	Images        []Image            `bson:"images,omitempty" json:"images,omitempty"`
	RatedAt       time.Time          `bson:"ratedAt" json:"ratedAt"`
}

// UnifiedRating represents a unified view of ratings for a booking, grouping owner and requester ratings
type UnifiedRating struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	BookingID primitive.ObjectID `bson:"bookingId" json:"bookingId"`
	ToolID    string             `bson:"toolId" json:"toolId"`
	Owner     *RatingParty       `bson:"owner,omitempty" json:"owner,omitempty"`
	Requester *RatingParty       `bson:"requester,omitempty" json:"requester,omitempty"`
}

// RatingParty represents one party's rating in a unified rating view
type RatingParty struct {
	ID            primitive.ObjectID `bson:"id" json:"id"`
	Rating        *int               `bson:"rating,omitempty" json:"rating,omitempty"`
	RatingComment *string            `bson:"ratingComment,omitempty" json:"ratingComment,omitempty"`
	RatedAt       *int64             `bson:"ratedAt,omitempty" json:"ratedAt,omitempty"`
	Images        []types.HexBytes   `bson:"images,omitempty" json:"images,omitempty"`
}

func newRatingCollection(db *mongo.Database) *mongo.Collection {
	collection := db.Collection("ratings")

	// Create indexes for efficient queries
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "bookingId", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "raterId", Value: 1},
				{Key: "createdAt", Value: -1}, // For efficient sorting by date
			},
		},
		{
			Keys: bson.D{
				{Key: "rateeId", Value: 1},
				{Key: "createdAt", Value: -1}, // For efficient sorting by date
			},
		},
		{
			// Unique compound index to ensure one rating per booking per rater-ratee pair
			Keys: bson.D{
				{Key: "bookingId", Value: 1},
				{Key: "raterId", Value: 1},
				{Key: "rateeId", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := collection.Indexes().CreateMany(context.Background(), indexes)
	if err != nil {
		panic(err)
	}

	return collection
}

// GetPendingRatings retrieves paginated bookings that need to be rated by the user
func (s *BookingService) GetPendingRatings(
	ctx context.Context,
	userID primitive.ObjectID,
	page int,
	pageSize int,
) ([]*Booking, int64, error) {
	// Ensure page is not negative
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	skip := page * pageSize

	// Use an aggregation pipeline to efficiently find bookings that need to be rated
	pipeline := mongo.Pipeline{
		// Stage 1: Match returned or picked bookings where the user is involved
		{{Key: "$match", Value: bson.M{
			"bookingStatus": bson.M{"$in": []BookingStatus{BookingStatusReturned, BookingStatusPicked}},
			"$or": []bson.M{
				{"fromUserId": userID},
				{"toUserId": userID},
			},
		}}},
		// Stage 2: Add fields to determine the counterparty ID
		{{Key: "$addFields", Value: bson.M{
			"counterpartyId": bson.M{
				"$cond": bson.M{
					"if":   bson.M{"$eq": []interface{}{"$fromUserId", userID}},
					"then": "$toUserId",
					"else": "$fromUserId",
				},
			},
		}}},
		// Stage 3: Lookup ratings for this booking by this user
		{{Key: "$lookup", Value: bson.M{
			"from": s.ratingsCollection.Name(),
			"let":  bson.M{"bookingId": "$_id", "counterpartyId": "$counterpartyId"},
			"pipeline": bson.A{
				bson.M{
					"$match": bson.M{
						"$expr": bson.M{
							"$and": bson.A{
								bson.M{"$eq": []interface{}{"$bookingId", "$$bookingId"}},
								bson.M{"$eq": []interface{}{"$raterId", userID}},
								bson.M{"$eq": []interface{}{"$rateeId", "$$counterpartyId"}},
							},
						},
					},
				},
			},
			"as": "userRatings",
		}}},
		// Stage 4: Filter to only include bookings with no ratings by this user
		{{Key: "$match", Value: bson.M{
			"userRatings": bson.M{"$size": 0},
		}}},
		// Sort by createdAt in descending order (newest first)
		{{Key: "$sort", Value: bson.D{{Key: "createdAt", Value: -1}}}},
		// Stage 5: Use $facet to get both data and count
		{{Key: "$facet", Value: bson.D{
			{Key: "data", Value: bson.A{
				bson.D{{Key: "$skip", Value: skip}},
				bson.D{{Key: "$limit", Value: pageSize}},
				bson.D{{Key: "$project", Value: bson.M{
					"counterpartyId": 0,
					"userRatings":    0,
				}}},
			}},
			{Key: "count", Value: bson.A{
				bson.D{{Key: "$count", Value: "total"}},
			}},
		}}},
	}

	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		log.Error().Err(err).
			Str("userId", userID.Hex()).
			Msg("Failed to aggregate pending ratings")
		return nil, 0, fmt.Errorf("failed to find pending ratings: %w", err)
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

	if err = cursor.All(ctx, &result); err != nil {
		log.Error().Err(err).
			Str("userId", userID.Hex()).
			Msg("Failed to decode pending ratings")
		return nil, 0, fmt.Errorf("failed to decode pending ratings: %w", err)
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

// GetSubmittedRatings retrieves ratings submitted by the user
func (s *BookingService) GetSubmittedRatings(ctx context.Context, userID primitive.ObjectID) ([]*BookingRating, error) {
	filter := bson.M{
		"raterId": userID,
		"rateeId": bson.M{"$ne": userID}, // exclude self-ratings
	}

	// Use options to sort by createdAt in descending order (newest first)
	findOptions := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})

	cursor, err := s.ratingsCollection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var ratings []*Rating
	if err = cursor.All(ctx, &ratings); err != nil {
		return nil, err
	}

	// Convert to BookingRating for API compatibility
	return s.convertToBookingRatings(ratings), nil
}

// GetReceivedRatings retrieves ratings received by the user
func (s *BookingService) GetReceivedRatings(ctx context.Context, userID primitive.ObjectID) ([]*BookingRating, error) {
	filter := bson.M{
		"rateeId": userID,
		"raterId": bson.M{"$ne": userID}, // exclude self-ratings
	}

	// Use options to sort by createdAt in descending order (newest first)
	findOptions := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})

	cursor, err := s.ratingsCollection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var ratings []*Rating
	if err = cursor.All(ctx, &ratings); err != nil {
		return nil, err
	}

	// Convert to BookingRating for API compatibility
	return s.convertToBookingRatings(ratings), nil
}

// createUnifiedRatings processes bookings and ratings to create unified rating objects
func (s *BookingService) createUnifiedRatings(bookings []*Booking, ratings []*Rating) []*UnifiedRating {
	// If no bookings found, return empty array
	if len(bookings) == 0 {
		return []*UnifiedRating{}
	}

	// Create a map of booking IDs for efficient lookup
	bookingMap := make(map[primitive.ObjectID]*Booking)
	for _, booking := range bookings {
		bookingMap[booking.ID] = booking
	}

	// Group ratings by booking ID
	ratingsByBooking := make(map[primitive.ObjectID][]*Rating)
	for _, rating := range ratings {
		ratingsByBooking[rating.BookingID] = append(ratingsByBooking[rating.BookingID], rating)
	}

	// Create unified ratings
	var unifiedRatings []*UnifiedRating

	// Sort bookings by createdAt in descending order (newest first)
	var sortedBookings []*Booking
	for _, booking := range bookingMap {
		sortedBookings = append(sortedBookings, booking)
	}

	// Sort the bookings by createdAt in descending order
	sort.Slice(sortedBookings, func(i, j int) bool {
		return sortedBookings[i].CreatedAt.After(sortedBookings[j].CreatedAt)
	})

	// Process bookings in sorted order
	for _, booking := range sortedBookings {
		bookingID := booking.ID

		// Check if there are any ratings for this booking
		bookingRatings, hasRatings := ratingsByBooking[bookingID]
		if !hasRatings {
			// Skip bookings with no ratings
			continue
		}

		unified := &UnifiedRating{
			ID:        bookingID,
			BookingID: bookingID,
			ToolID:    booking.ToolID,
		}

		// Determine who is the owner and who is the requester
		ownerID := booking.ToUserID
		requesterID := booking.FromUserID

		// Initialize owner and requester
		unified.Owner = &RatingParty{
			ID: ownerID,
		}
		unified.Requester = &RatingParty{
			ID: requesterID,
		}

		// Add ratings if they exist
		for _, r := range bookingRatings {
			if r.RaterID == ownerID && r.RateeID == requesterID {
				// Owner rating the requester
				ratedAt := r.CreatedAt.Unix()
				comment := r.Comment
				score := r.Score
				unified.Owner.Rating = &score
				unified.Owner.RatingComment = &comment
				unified.Owner.RatedAt = &ratedAt
				for i := range r.Images {
					unified.Owner.Images = append(unified.Owner.Images, r.Images[i].Hash)
				}
			} else if r.RaterID == requesterID && r.RateeID == ownerID {
				// Requester rating the owner
				ratedAt := r.CreatedAt.Unix()
				comment := r.Comment
				score := r.Score
				unified.Requester.Rating = &score
				unified.Requester.RatingComment = &comment
				unified.Requester.RatedAt = &ratedAt
				for i := range r.Images {
					unified.Requester.Images = append(unified.Requester.Images, r.Images[i].Hash)
				}
			}
		}

		unifiedRatings = append(unifiedRatings, unified)
	}

	return unifiedRatings
}

// GetRatingsByToolID retrieves all ratings associated with a specific tool ID
func (s *BookingService) GetRatingsByToolID(
	ctx context.Context,
	toolID string,
	page int,
	pageSize int,
) ([]*UnifiedRating, int64, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	skip := page * pageSize

	// Get all bookings for this tool
	filter := bson.M{"toolId": toolID}
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var bookings []*Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		return nil, 0, err
	}

	// If no bookings found, return empty array
	if len(bookings) == 0 {
		return []*UnifiedRating{}, 0, nil
	}

	// Get all ratings for these bookings
	var bookingIDs []primitive.ObjectID
	for _, booking := range bookings {
		bookingIDs = append(bookingIDs, booking.ID)
	}

	ratingFilter := bson.M{
		"bookingId": bson.M{"$in": bookingIDs},
	}

	// Set up options for pagination
	findOptions := options.Find()
	findOptions.SetSkip(int64(skip))
	findOptions.SetLimit(int64(pageSize))
	// Use options to sort by createdAt in descending order (newest first)
	findOptions.SetSort(bson.D{{Key: "createdAt", Value: -1}})

	ratingCursor, err := s.ratingsCollection.Find(ctx, ratingFilter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err := ratingCursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing rating cursor")
		}
	}()

	var ratings []*Rating
	if err = ratingCursor.All(ctx, &ratings); err != nil {
		return nil, 0, err
	}

	total, err := s.ratingsCollection.CountDocuments(ctx, ratingFilter)

	return s.createUnifiedRatings(bookings, ratings), total, nil
}

// GetRatingsByBookingID retrieves all ratings associated with a specific booking ID
func (s *BookingService) GetRatingsByBookingID(ctx context.Context, bookingID primitive.ObjectID) (*UnifiedRating, error) {
	// First get the booking to have access to the tool ID and user IDs
	var booking *Booking
	err := s.collection.FindOne(ctx, bson.M{"_id": bookingID}).Decode(&booking)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrBookingNotFound
		}
		return nil, err
	}

	// Get all ratings for this booking
	filter := bson.M{
		"bookingId": bookingID,
	}

	// Use options to sort by createdAt in descending order (newest first)
	findOptions := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})

	cursor, err := s.ratingsCollection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var ratings []*Rating
	if err = cursor.All(ctx, &ratings); err != nil {
		return nil, err
	}

	// Create a basic UnifiedRating with booking information
	unifiedRating := &UnifiedRating{
		ID:        bookingID,
		BookingID: bookingID,
		ToolID:    booking.ToolID,
		Owner: &RatingParty{
			ID: booking.ToUserID,
		},
		Requester: &RatingParty{
			ID: booking.FromUserID,
		},
	}

	// If there are ratings, use the createUnifiedRatings helper to populate the rating details
	if len(ratings) > 0 {
		unifiedRatings := s.createUnifiedRatings([]*Booking{booking}, ratings)
		if len(unifiedRatings) > 0 {
			unifiedRating = unifiedRatings[0]
		}
	}

	return unifiedRating, nil
}

// GetRatingsByUserId retrieves all ratings for a user (both submitted and received) and groups them by booking
func (s *BookingService) GetRatingsByUserId(
	ctx context.Context,
	userID primitive.ObjectID,
	page int,
	pageSize int,
) ([]*UnifiedRating, int64, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	skip := page * pageSize

	// Get all bookings where the user is involved
	filter := bson.M{
		"$or": []bson.M{
			{"fromUserId": userID},
			{"toUserId": userID},
		},
	}

	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var bookings []*Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		return nil, 0, err
	}

	// If no bookings found, return empty array
	if len(bookings) == 0 {
		return []*UnifiedRating{}, 0, nil
	}

	// Get all ratings for these bookings
	var bookingIDs []primitive.ObjectID
	for _, booking := range bookings {
		bookingIDs = append(bookingIDs, booking.ID)
	}

	ratingFilter := bson.M{
		"bookingId": bson.M{"$in": bookingIDs},
	}

	// Set up options for pagination
	findOptions := options.Find()
	findOptions.SetSkip(int64(skip))
	findOptions.SetLimit(int64(pageSize))
	// Use options to sort by createdAt in descending order (newest first)
	findOptions.SetSort(bson.D{{Key: "createdAt", Value: -1}})

	ratingCursor, err := s.ratingsCollection.Find(ctx, ratingFilter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err := ratingCursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing rating cursor")
		}
	}()

	var ratings []*Rating
	if err = ratingCursor.All(ctx, &ratings); err != nil {
		return nil, 0, err
	}

	total, err := s.ratingsCollection.CountDocuments(ctx, ratingFilter)

	return s.createUnifiedRatings(bookings, ratings), total, nil
}

// RateBooking creates a new rating for a booking
func (s *BookingService) RateBooking(
	ctx context.Context,
	bookingID primitive.ObjectID,
	raterID primitive.ObjectID,
	score int,
	comment string,
	images []Image,
) error {
	// Get the booking
	var booking Booking
	err := s.collection.FindOne(ctx, bson.M{"_id": bookingID}).Decode(&booking)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return ErrBookingNotFound
		}
		return err
	}

	// Verify booking is in RETURNED state
	if booking.BookingStatus != BookingStatusReturned && booking.BookingStatus != BookingStatusPicked {
		return fmt.Errorf("booking must be in RETURNED or PICKED state to be rated")
	}

	// Verify that the user is involved in the booking
	if booking.FromUserID != raterID && booking.ToUserID != raterID {
		return fmt.Errorf("user is not involved in this booking")
	}

	// Determine the ratee (counterparty)
	var rateeID primitive.ObjectID
	if booking.FromUserID == raterID {
		rateeID = booking.ToUserID
	} else {
		rateeID = booking.FromUserID
	}

	// Handle nil images
	if images == nil {
		images = []Image{}
	}

	// Check that the user has not already submitted a rating for this booking
	var existingRating Rating
	err = s.ratingsCollection.FindOne(ctx, bson.M{
		"bookingId": bookingID,
		"raterId":   raterID,
		"rateeId":   rateeID,
	}).Decode(&existingRating)

	if err != mongo.ErrNoDocuments {
		if err == nil {
			return fmt.Errorf("user has already rated this booking")
		}
		return err
	}

	// Validate rating value
	if score < 1 || score > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}

	// Create and insert the new rating
	now := time.Now()
	newRating := Rating{
		BookingID: bookingID,
		RaterID:   raterID,
		RateeID:   rateeID,
		Score:     score,
		Comment:   comment,
		CreatedAt: now,
		Images:    images,
	}

	_, err = s.ratingsCollection.InsertOne(ctx, newRating)
	if err != nil {
		return err
	}

	// Update overall ratings for the tool and for the recipient
	return s.updateRatings(ctx, &booking)
}

// updateRatings updates the overall ratings for a tool and user after a new rating is submitted
func (s *BookingService) updateRatings(ctx context.Context, booking *Booking) error {
	if booking == nil {
		log.Error().Msg("Cannot update ratings: booking is nil")
		return fmt.Errorf("booking cannot be nil")
	}

	log.Debug().
		Str("bookingId", booking.ID.Hex()).
		Str("toolId", booking.ToolID).
		Str("ownerId", booking.ToUserID.Hex()).
		Msg("Updating ratings for booking")

	// Update the tool's rating
	if err := s.updateToolRating(ctx, booking); err != nil {
		log.Error().Err(err).
			Str("bookingId", booking.ID.Hex()).
			Str("toolId", booking.ToolID).
			Msg("Failed to update tool rating")
		return fmt.Errorf("failed to update tool rating: %w", err)
	}

	// Update the owner's overall rating
	if err := s.updateUserRating(ctx, booking); err != nil {
		log.Error().Err(err).
			Str("bookingId", booking.ID.Hex()).
			Str("userId", booking.ToUserID.Hex()).
			Msg("Failed to update user rating")
		return fmt.Errorf("failed to update user rating: %w", err)
	}

	log.Debug().
		Str("bookingId", booking.ID.Hex()).
		Msg("Successfully updated ratings for booking")
	return nil
}

// updateToolRating updates the average rating for a tool
func (s *BookingService) updateToolRating(ctx context.Context, booking *Booking) error {
	// We perform a $lookup to join ratings with bookings and then filter by toolId
	toolPipeline := mongo.Pipeline{
		{{Key: "$lookup", Value: bson.M{
			"from":         "bookings",
			"localField":   "bookingId",
			"foreignField": "_id",
			"as":           "booking",
		}}},
		{{Key: "$unwind", Value: "$booking"}},
		{{Key: "$match", Value: bson.M{
			"booking.toolId": booking.ToolID,
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       nil,
			"avgRating": bson.M{"$avg": "$score"}, // Using score field from Rating
			"count":     bson.M{"$sum": 1},        // Count the number of ratings
		}}},
	}

	toolCursor, err := s.ratingsCollection.Aggregate(ctx, toolPipeline)
	if err != nil {
		return fmt.Errorf("failed to calculate tool average rating: %w", err)
	}
	defer func() {
		if err := toolCursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing tool cursor")
		}
	}()

	var toolResults []struct {
		AvgRating float64 `bson:"avgRating"`
		Count     int     `bson:"count"`
	}
	if err = toolCursor.All(ctx, &toolResults); err != nil {
		return fmt.Errorf("failed to decode tool average rating: %w", err)
	}

	if len(toolResults) > 0 {
		toolService := s.database.Collection("tools")
		toolID, err := strconv.ParseInt(booking.ToolID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid tool ID: %w", err)
		}
		avg := toolResults[0].AvgRating
		count := toolResults[0].Count

		log.Debug().
			Str("toolId", booking.ToolID).
			Float64("avgRating", avg).
			Int("ratingCount", count).
			Msg("Updating tool rating")

		_, err = toolService.UpdateOne(ctx, bson.M{"_id": toolID}, bson.M{
			"$set": bson.M{"rating": int32(math.Round(avg))},
		})
		if err != nil {
			return fmt.Errorf("failed to update tool rating in database: %w", err)
		}
	} else {
		log.Debug().
			Str("toolId", booking.ToolID).
			Msg("No ratings found for tool")
	}

	return nil
}

// updateUserRating updates the average rating for a user
func (s *BookingService) updateUserRating(ctx context.Context, booking *Booking) error {
	// Only consider ratings where the owner (toUserId) is the recipient (rateeId)
	userPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"rateeId": booking.ToUserID,
			"raterId": bson.M{"$ne": booking.ToUserID}, // exclude self-ratings
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       nil,
			"avgRating": bson.M{"$avg": "$score"}, // Using score field from Rating
			"count":     bson.M{"$sum": 1},        // Count the number of ratings
		}}},
	}

	userCursor, err := s.ratingsCollection.Aggregate(ctx, userPipeline)
	if err != nil {
		return fmt.Errorf("failed to calculate user average rating: %w", err)
	}
	defer func() {
		if err := userCursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing user cursor")
		}
	}()

	var userResults []struct {
		AvgRating float64 `bson:"avgRating"`
		Count     int     `bson:"count"`
	}
	if err = userCursor.All(ctx, &userResults); err != nil {
		return fmt.Errorf("failed to decode user average rating: %w", err)
	}

	if len(userResults) > 0 {
		// Convert the average rating (out of 5) to a percentage
		avg := userResults[0].AvgRating
		count := userResults[0].Count
		overall := int32(math.Round((avg / 5.0) * 100))

		log.Debug().
			Str("userId", booking.ToUserID.Hex()).
			Float64("avgRating", avg).
			Int32("overallPercentage", overall).
			Int("ratingCount", count).
			Msg("Updating user rating")

		userService := s.database.Collection("users")
		_, err = userService.UpdateOne(ctx, bson.M{"_id": booking.ToUserID}, bson.M{
			"$set": bson.M{
				"rating":      overall,
				"ratingCount": count,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to update user rating in database: %w", err)
		}
	} else {
		log.Debug().
			Str("userId", booking.ToUserID.Hex()).
			Msg("No ratings found for user")
	}

	return nil
}

// CountPendingActions counts the number of pending actions for a user
func (s *BookingService) CountPendingActions(
	ctx context.Context,
	userID primitive.ObjectID,
) (*CountPendingActionsResponse, error) {
	pipeline := mongo.Pipeline{
		{{
			Key: "$facet", Value: bson.D{
				{Key: "pendingRatings", Value: bson.A{
					bson.D{{Key: "$match", Value: bson.M{
						"bookingStatus": bson.M{"$in": []BookingStatus{BookingStatusReturned, BookingStatusPicked}},
						"$or": []bson.M{
							{"fromUserId": userID},
							{"toUserId": userID},
						},
					}}},
					bson.D{{Key: "$lookup", Value: bson.M{
						"from":         s.ratingsCollection.Name(),
						"localField":   "_id",
						"foreignField": "bookingId",
						"as":           "ratings",
					}}},
					// Add a field that counts ratings submitted by the user
					bson.D{{Key: "$addFields", Value: bson.M{
						"userRatingCount": bson.M{
							"$size": bson.M{
								"$filter": bson.M{
									"input": "$ratings",
									"as":    "r",
									"cond":  bson.M{"$eq": []interface{}{"$$r.raterId", userID}},
								},
							},
						},
					}}},
					bson.D{{Key: "$match", Value: bson.M{
						"userRatingCount": bson.M{"$lt": 1},
					}}},
					bson.D{{Key: "$count", Value: "count"}},
				}},

				{Key: "pendingRequests", Value: bson.A{
					bson.D{{Key: "$match", Value: bson.M{
						"toUserId":      userID,
						"bookingStatus": BookingStatusPending,
					}}},
					bson.D{{Key: "$count", Value: "count"}},
				}},
			},
		}},
		{{
			Key: "$project", Value: bson.D{
				{Key: "pendingRatingsCount", Value: bson.M{"$arrayElemAt": bson.A{"$pendingRatings.count", 0}}},
				{Key: "pendingRequestsCount", Value: bson.M{"$arrayElemAt": bson.A{"$pendingRequests.count", 0}}},
			},
		}},
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

// convertToBookingRatings converts Rating objects to BookingRating objects for API compatibility
func (s *BookingService) convertToBookingRatings(ratings []*Rating) []*BookingRating {
	bookingRatings := make([]*BookingRating, len(ratings))
	for i, r := range ratings {
		bookingRatings[i] = &BookingRating{
			ID:            r.ID,
			BookingID:     r.BookingID,
			FromUserID:    r.RaterID,
			ToUserID:      r.RateeID,
			Rating:        r.Score,
			RatingComment: r.Comment,
			Images:        r.Images,
			RatedAt:       r.CreatedAt,
		}
	}
	return bookingRatings
}
