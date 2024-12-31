package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BookingStatus represents the current state of a booking
type BookingStatus string

const (
	BookingStatusPending  BookingStatus = "PENDING"
	BookingStatusAccepted BookingStatus = "ACCEPTED"
	BookingStatusRejected BookingStatus = "REJECTED"
	BookingStatusReturned BookingStatus = "RETURNED"
)

// Booking represents a tool booking in the system
type Booking struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	ToolID        primitive.ObjectID `bson:"toolId"`
	FromUserID    primitive.ObjectID `bson:"fromUserId"`
	ToUserID      primitive.ObjectID `bson:"toUserId"`
	StartDate     time.Time          `bson:"startDate"`
	EndDate       time.Time          `bson:"endDate"`
	Contact       string             `bson:"contact"`
	Comments      string             `bson:"comments"`
	BookingStatus BookingStatus      `bson:"bookingStatus"`
	CreatedAt     time.Time          `bson:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt"`
}

// BookingService handles all booking related database operations
type BookingService struct {
	collection *mongo.Collection
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
			},
		},
		{
			Keys: bson.D{
				{Key: "toUserId", Value: 1},
			},
		},
	}

	_, err := collection.Indexes().CreateMany(context.Background(), indexes)
	if err != nil {
		panic(err)
	}

	return &BookingService{collection: collection}
}

// CreateBookingRequest represents the request to create a new booking
type CreateBookingRequest struct {
	ToolID    primitive.ObjectID `bson:"toolId"`
	StartDate time.Time          `bson:"startDate"`
	EndDate   time.Time          `bson:"endDate"`
	Contact   string             `bson:"contact"`
	Comments  string             `bson:"comments"`
}

// Create creates a new booking
func (s *BookingService) Create(ctx context.Context, req *CreateBookingRequest, fromUserID, toUserID primitive.ObjectID) (*Booking, error) {
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
		return nil, fmt.Errorf("booking dates conflict with existing booking")
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
		return nil, nil
	}
	return &booking, err
}

// GetUserRequests gets all booking requests for tools owned by the user
func (s *BookingService) GetUserRequests(ctx context.Context, userID primitive.ObjectID) ([]*Booking, error) {
	cursor, err := s.collection.Find(ctx, bson.M{
		"toUserId": userID,
	}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))

	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

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
	defer cursor.Close(ctx)

	var bookings []*Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		return nil, err
	}
	return bookings, nil
}

// UpdateStatus updates the booking status
func (s *BookingService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status BookingStatus) error {
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
		return fmt.Errorf("booking not found")
	}
	return nil
}

// checkDateConflicts checks if there are any conflicting bookings for the given tool and dates
func (s *BookingService) checkDateConflicts(ctx context.Context, toolID primitive.ObjectID, start, end time.Time, excludeID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"toolId": toolID,
		"bookingStatus": bson.M{
			"$in": []BookingStatus{BookingStatusPending, BookingStatusAccepted},
		},
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

// GetPendingRatings gets bookings that need to be rated by the user
func (s *BookingService) GetPendingRatings(ctx context.Context, userID primitive.ObjectID) ([]*Booking, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"fromUserId": userID},
			{"toUserId": userID},
		},
		"bookingStatus": BookingStatusReturned,
		// Add additional conditions for unrated bookings
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
	return bookings, nil
}
