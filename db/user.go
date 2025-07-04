package db

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/emprius/emprius-app-backend/types"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// User represents the schema for the "users" collection.
type User struct {
	ID                      primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Email                   string              `bson:"email" json:"email"`
	Name                    string              `bson:"name" json:"name"`
	Community               string              `bson:"community,omitempty" json:"community,omitempty"`
	Password                []byte              `bson:"password" json:"-"` // Don't include password in JSON
	Tokens                  uint64              `bson:"tokens" json:"tokens" default:"1000"`
	Active                  bool                `bson:"active" json:"active" default:"true"`
	Rating                  int32               `bson:"rating" json:"rating" default:"50"`
	RatingCount             int                 `bson:"ratingCount" json:"ratingCount" default:"0"`
	AvatarHash              types.HexBytes      `bson:"avatarHash,omitempty" json:"avatarHash,omitempty"`
	Location                DBLocation          `bson:"location" json:"location"`
	ObfuscatedLocation      DBLocation          `bson:"obfuscatedLocation" json:"obfuscatedLocation"`
	Verified                bool                `bson:"verified" json:"verified" default:"false"`
	CreatedAt               time.Time           `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	LastSeen                time.Time           `bson:"lastSeen,omitempty" json:"lastSeen,omitempty"`
	Bio                     string              `bson:"bio,omitempty" json:"bio,omitempty"`
	Communities             []UserCommunityRole `bson:"communities,omitempty" json:"communities,omitempty"`
	NotificationPreferences map[string]bool     `bson:"notificationPreferences,omitempty" json:"notificationPreferences,omitempty"`
}

// UserCommunityRole represents a user's role in a community
type UserCommunityRole struct {
	ID   primitive.ObjectID `bson:"id" json:"id"`
	Role CommunityRole      `bson:"role" json:"role"`
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
	// Set CreatedAt and LastSeen to current time if not already set
	now := time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.LastSeen.IsZero() {
		user.LastSeen = now
	}
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

// GetUserByIDWithAccessControl retrieves a User by their ID with access control.
// Only allows access to inactive users if the requesting user is the same user.
func (s *UserService) GetUserByIDWithAccessControl(
	ctx context.Context,
	userID,
	requestingUserID primitive.ObjectID,
) (*User, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Allow access if user is active OR if requesting user is the same user
	if user.Active || userID == requestingUserID {
		return user, nil
	}

	// Deny access to inactive user by different user
	return nil, mongo.ErrNoDocuments
}

// GetUsers retrieves users whose names partially match the given string using aggregation pipeline
// Returns users and total count for pagination. Only searches if partialName is not empty.
// This method excludes inactive users from the results.
func (s *UserService) GetUsers(ctx context.Context, partialName string, page int) ([]*User, int64, error) {
	if page < 0 {
		page = 0
	}

	skip := page * DefaultPageSize

	// Create a case-insensitive regex pattern for partial name matching
	pattern := "(?i).*" + regexp.QuoteMeta(SanitizeString(partialName)) + ".*"
	regex := primitive.Regex{Pattern: pattern, Options: "i"}

	// Create the aggregation pipeline with $facet to get both data and count
	pipeline := mongo.Pipeline{
		// Stage 1: Match users by name AND active status
		bson.D{{Key: "$match", Value: bson.M{
			"name":   regex,
			"active": true, // Only include active users
		}}},
		// Stage 2: Sort by name
		bson.D{{Key: "$sort", Value: bson.D{{Key: "name", Value: 1}}}},
		// Stage 3: Use $facet to get both data and count
		bson.D{{Key: "$facet", Value: bson.D{
			{Key: "data", Value: bson.A{
				bson.D{{Key: "$skip", Value: int64(skip)}},
				bson.D{{Key: "$limit", Value: int64(DefaultPageSize)}},
			}},
			{Key: "count", Value: bson.A{
				bson.D{{Key: "$count", Value: "total"}},
			}},
		}}},
	}

	// Execute the aggregation pipeline
	cursor, err := s.Collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var result []struct {
		Data  []*User `bson:"data"`
		Count []struct {
			Total int64 `bson:"total"`
		} `bson:"count"`
	}

	if err := cursor.All(ctx, &result); err != nil {
		return nil, 0, err
	}

	var users []*User
	var total int64
	if len(result) > 0 {
		users = result[0].Data
		if len(result[0].Count) > 0 {
			total = result[0].Count[0].Total
		}
	}

	return users, total, nil
}

// AddUserToCommunity adds a community to a user's communities list
func (s *UserService) AddUserToCommunity(ctx context.Context, userID, communityID primitive.ObjectID, role CommunityRole) error {
	// Check if user already has this community
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Check if user already has this community
	for _, comm := range user.Communities {
		if comm.ID == communityID {
			// User already has this community, update role if different
			if comm.Role != role {
				return s.UpdateUserCommunityRole(ctx, userID, communityID, role)
			}
			return nil // User already has this community with the same role
		}
	}

	// Add community to user's communities
	_, err = s.Collection.UpdateOne(
		ctx,
		bson.M{"_id": userID},
		bson.M{
			"$push": bson.M{
				"communities": UserCommunityRole{
					ID:   communityID,
					Role: role,
				},
			},
		},
	)
	return err
}

// RemoveUserFromCommunity removes a community from a user's communities list
func (s *UserService) RemoveUserFromCommunity(ctx context.Context, userID, communityID primitive.ObjectID) error {
	_, err := s.Collection.UpdateOne(
		ctx,
		bson.M{"_id": userID},
		bson.M{
			"$pull": bson.M{
				"communities": bson.M{"id": communityID},
			},
		},
	)
	return err
}

// UpdateUserCommunityRole updates a user's role in a community
func (s *UserService) UpdateUserCommunityRole(
	ctx context.Context, userID,
	communityID primitive.ObjectID,
	role CommunityRole,
) error {
	_, err := s.Collection.UpdateOne(
		ctx,
		bson.M{
			"_id":            userID,
			"communities.id": communityID,
		},
		bson.M{
			"$set": bson.M{
				"communities.$.role": role,
			},
		},
	)
	return err
}

// GetUserCommunities retrieves all communities a user is a member of
func (s *UserService) GetUserCommunities(ctx context.Context, userID primitive.ObjectID) ([]UserCommunityRole, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return user.Communities, nil
}

// GetDefaultNotificationPreferences returns the default notification preferences for new users
func GetDefaultNotificationPreferences() map[string]bool {
	return types.GetDefaultNotificationPreferences()
}

// UpdateNotificationPreferences updates a user's notification preferences
func (s *UserService) UpdateNotificationPreferences(
	ctx context.Context,
	userID primitive.ObjectID,
	preferences map[string]bool,
) error {
	// Get current preferences to merge with new ones
	currentPreferences, err := s.GetNotificationPreferences(ctx, userID)
	if err != nil {
		return err
	}

	// Merge new preferences with current ones
	for key, value := range preferences {
		currentPreferences[key] = value
	}

	_, err = s.Collection.UpdateOne(
		ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{"notificationPreferences": currentPreferences}},
	)
	return err
}

// GetNotificationPreferences retrieves a user's notification preferences, returning defaults if none exist
func (s *UserService) GetNotificationPreferences(ctx context.Context, userID primitive.ObjectID) (map[string]bool, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// If user has no notification preferences set, return defaults
	if len(user.NotificationPreferences) == 0 {
		return GetDefaultNotificationPreferences(), nil
	}

	// Merge with defaults to ensure all notification types are present
	defaults := GetDefaultNotificationPreferences()
	for key, defaultValue := range defaults {
		if _, exists := user.NotificationPreferences[key]; !exists {
			user.NotificationPreferences[key] = defaultValue
		}
	}

	return user.NotificationPreferences, nil
}
