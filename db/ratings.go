package db

import (
	"bytes"
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

// BookingRating represents a rating given by a user for a booking.
type BookingRating struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	BookingID        primitive.ObjectID `bson:"bookingId" json:"bookingId"`
	FromUserID       primitive.ObjectID `bson:"fromUserId" json:"fromUserId"`
	ToUserID         primitive.ObjectID `bson:"toUserId" json:"toUserId"`
	Rating           int                `bson:"rating" json:"rating"`
	RatingComment    string             `bson:"ratingComment,omitempty" json:"ratingComment,omitempty"`
	RatingHashImages []string           `bson:"ratingHashImages,omitempty" json:"ratingHashImages,omitempty"`
	RatedAt          time.Time          `bson:"ratedAt" json:"ratedAt"`
}

func newRatingCollection(db *mongo.Database) *mongo.Collection {
	collection := db.Collection("ratings")

	// Create indexes – note that we now index on "bookingId"
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "bookingId", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "fromUserId", Value: 1},
				{Key: "ratedAt", Value: -1}, // For efficient sorting by date
			},
		},
		{
			Keys: bson.D{
				{Key: "toUserId", Value: 1},
				{Key: "ratedAt", Value: -1}, // For efficient sorting by date
			},
		},
	}

	_, err := collection.Indexes().CreateMany(context.Background(), indexes)
	if err != nil {
		panic(err)
	}

	return collection
}

func (s *BookingService) GetPendingRatings(ctx context.Context, userID primitive.ObjectID) ([]*Booking, error) {
	// First, get all returned bookings in which the user is involved.
	filter := bson.M{
		"bookingStatus": BookingStatusReturned,
		"$or": []bson.M{
			{"fromUserId": userID},
			{"toUserId": userID},
		},
	}
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var bookings []*Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		return nil, err
	}

	var pending []*Booking
	// For each booking, determine the counterparty and check if a rating exists.
	for _, b := range bookings {
		var cp primitive.ObjectID
		if bytes.Equal(b.FromUserID[:], userID[:]) {
			cp = b.ToUserID
		} else {
			cp = b.FromUserID
		}
		// Check in the ratings collection if a rating exists for this booking.
		// Now we use "bookingId": b.ID.
		var r BookingRating
		err := s.ratingsCollection.FindOne(ctx, bson.M{
			"bookingId":  b.ID,
			"fromUserId": userID,
			"toUserId":   cp,
		}).Decode(&r)
		if err == mongo.ErrNoDocuments {
			pending = append(pending, b)
		} else if err != nil {
			return nil, err
		}
	}
	return pending, nil
}

func (s *BookingService) GetSubmittedRatings(ctx context.Context, userID primitive.ObjectID) ([]*BookingRating, error) {
	filter := bson.M{
		"fromUserId": userID,
		"toUserId":   bson.M{"$ne": userID}, // exclude self‑ratings.
	}
	cursor, err := s.ratingsCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var ratings []*BookingRating
	if err = cursor.All(ctx, &ratings); err != nil {
		return nil, err
	}
	return ratings, nil
}

func (s *BookingService) GetReceivedRatings(ctx context.Context, userID primitive.ObjectID) ([]*BookingRating, error) {
	filter := bson.M{
		"toUserId":   userID,
		"fromUserId": bson.M{"$ne": userID},
	}
	cursor, err := s.ratingsCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var ratings []*BookingRating
	if err = cursor.All(ctx, &ratings); err != nil {
		return nil, err
	}
	return ratings, nil
}

func (s *BookingService) RateBooking(
	ctx context.Context,
	bookingID primitive.ObjectID,
	userID primitive.ObjectID,
	rating int,
	comment string,
) error {
	// Get the booking.
	var booking Booking
	err := s.collection.FindOne(ctx, bson.M{"_id": bookingID}).Decode(&booking)
	if err != nil {
		return err
	}
	if booking.BookingStatus != BookingStatusReturned {
		return fmt.Errorf("booking must be in RETURNED state to be rated")
	}
	// Verify that the user is involved in the booking.
	if !(bytes.Equal(booking.FromUserID[:], userID[:]) || bytes.Equal(booking.ToUserID[:], userID[:])) {
		return fmt.Errorf("user is not involved in this booking")
	}
	// Determine counterparty.
	var cp primitive.ObjectID
	if bytes.Equal(booking.FromUserID[:], userID[:]) {
		cp = booking.ToUserID
	} else {
		cp = booking.FromUserID
	}
	// Check that the user has not already submitted a rating for this booking.
	var existing BookingRating
	err = s.ratingsCollection.FindOne(ctx, bson.M{
		"bookingId":  booking.ID,
		"fromUserId": userID,
		"toUserId":   cp,
	}).Decode(&existing)
	if err != mongo.ErrNoDocuments {
		if err == nil {
			return fmt.Errorf("user has already rated this booking")
		}
		return err
	}
	// Validate rating value.
	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	now := time.Now()
	newRating := BookingRating{
		BookingID:     booking.ID,
		FromUserID:    userID,
		ToUserID:      cp,
		Rating:        rating,
		RatingComment: comment,
		RatedAt:       now,
	}
	_, err = s.ratingsCollection.InsertOne(ctx, newRating)
	if err != nil {
		return err
	}
	// Update overall ratings for the tool and for the recipient.
	return s.updateRatings(ctx, &booking)
}

func (s *BookingService) updateRatings(ctx context.Context, booking *Booking) error {
	// Update the tool's rating.
	// We perform a $lookup to join ratings with bookings and then filter by toolId.
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
			"avgRating": bson.M{"$avg": "$rating"},
		}}},
	}
	toolCursor, err := s.ratingsCollection.Aggregate(ctx, toolPipeline)
	if err != nil {
		return fmt.Errorf("failed to calculate tool average rating: %w", err)
	}
	defer func() {
		_ = toolCursor.Close(ctx)
	}()
	var toolResults []struct {
		AvgRating float64 `bson:"avgRating"`
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
		_, err = toolService.UpdateOne(ctx, bson.M{"_id": toolID}, bson.M{
			"$set": bson.M{"rating": int32(math.Round(avg))},
		})
		if err != nil {
			return fmt.Errorf("failed to update tool rating: %w", err)
		}
	}

	// Update the owner's overall rating.
	// Only consider ratings where the owner (toUserId) is the recipient and the rating was submitted by someone else.
	userPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"toUserId":   booking.ToUserID,
			"fromUserId": bson.M{"$ne": booking.ToUserID},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       nil,
			"avgRating": bson.M{"$avg": "$rating"},
		}}},
	}
	userCursor, err := s.ratingsCollection.Aggregate(ctx, userPipeline)
	if err != nil {
		return fmt.Errorf("failed to calculate user average rating: %w", err)
	}
	defer func() {
		_ = userCursor.Close(ctx)
	}()
	var userResults []struct {
		AvgRating float64 `bson:"avgRating"`
	}
	if err = userCursor.All(ctx, &userResults); err != nil {
		return fmt.Errorf("failed to decode user average rating: %w", err)
	}
	if len(userResults) > 0 {
		// Convert the average rating (out of 5) to a percentage.
		overall := int32(math.Round((userResults[0].AvgRating / 5.0) * 100))
		userService := s.database.Collection("users")
		_, err = userService.UpdateOne(ctx, bson.M{"_id": booking.ToUserID}, bson.M{
			"$set": bson.M{"rating": overall},
		})
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

	cursor, err := s.ratingsCollection.Aggregate(ctx, pipeline)
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
