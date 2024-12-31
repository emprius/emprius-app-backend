package db

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// User represents the schema for the "users" collection.
type User struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Email      string             `bson:"email" json:"email"`
	Name       string             `bson:"name" json:"name"`
	Community  string             `bson:"community,omitempty" json:"community,omitempty"`
	Password   []byte             `bson:"password" json:"-"` // Don't include password in JSON
	Tokens     uint64             `bson:"tokens" json:"tokens" default:"1000"`
	Active     bool               `bson:"active" json:"active" default:"true"`
	Rating     int32              `bson:"rating" json:"rating" default:"50"`
	AvatarHash []byte             `bson:"avatarHash,omitempty" json:"avatarHash,omitempty"`
	Location   Location           `bson:"location" json:"location"`
	Verified   bool               `bson:"verified" json:"verified" default:"false"`
}

// Validate checks if the user data meets the required constraints
func (u *User) Validate() error {
	if len(u.Name) <= 2 || len(u.Name) >= 30 {
		return fmt.Errorf("name length must be between 2 and 30 characters")
	}
	if len(u.Email) <= 8 || len(u.Email) >= 30 {
		return fmt.Errorf("email length must be between 8 and 30 characters")
	}
	if u.Rating < 0 || u.Rating > 100 {
		return fmt.Errorf("rating must be between 0 and 100")
	}
	return nil
}

// UserService provides methods to interact with the "users" collection.
type UserService struct {
	Collection *mongo.Collection
}

// NewUserService creates a new UserService.
func NewUserService(db *Database) *UserService {
	return &UserService{
		Collection: db.Database.Collection("users"),
	}
}

// InsertUser inserts a new User document.
func (s *UserService) InsertUser(ctx context.Context, user *User) (*mongo.InsertOneResult, error) {
	return s.Collection.InsertOne(ctx, user)
}

// GetUserByEmail retrieves a User by their email address.
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	filter := bson.M{"email": email}
	err := s.Collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUser updates a User document by their ID.
func (s *UserService) UpdateUser(ctx context.Context, id primitive.ObjectID, update bson.M) (*mongo.UpdateResult, error) {
	filter := bson.M{"_id": id}
	return s.Collection.UpdateOne(ctx, filter, bson.M{"$set": update})
}

// GetAllUsers retrieves all User documents.
func (s *UserService) GetAllUsers(ctx context.Context) ([]*User, error) {
	cursor, err := s.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var users []*User
	for cursor.Next(ctx) {
		var user User
		if err := cursor.Decode(&user); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, nil
}

// DeleteUser deletes a User document by their ID.
func (s *UserService) DeleteUser(ctx context.Context, id primitive.ObjectID) (*mongo.DeleteResult, error) {
	filter := bson.M{"_id": id}
	return s.Collection.DeleteOne(ctx, filter)
}

// CountUsers returns the total number of users.
func (s *UserService) CountUsers(ctx context.Context) (int64, error) {
	return s.Collection.CountDocuments(ctx, bson.M{})
}

// GetUserByID retrieves a User by their ID.
func (s *UserService) GetUserByID(ctx context.Context, id primitive.ObjectID) (*User, error) {
	var user User
	filter := bson.M{"_id": id}
	err := s.Collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
