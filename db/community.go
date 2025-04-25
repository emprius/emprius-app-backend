package db

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CommunityRole represents a user's role in a community
type CommunityRole string

const (
	CommunityRoleOwner CommunityRole = "owner"
	CommunityRoleUser  CommunityRole = "user"
)

// Community represents a group of users that can share tools
type Community struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string             `bson:"name" json:"name"`
	ImageHash []byte             `bson:"imageHash,omitempty" json:"imageHash,omitempty"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
	OwnerID   primitive.ObjectID `bson:"ownerId" json:"ownerId"`
}

// CommunityInvite represents an invitation to join a community
type CommunityInvite struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	CommunityID primitive.ObjectID `bson:"communityId" json:"communityId"`
	UserID      primitive.ObjectID `bson:"userId" json:"userId"`
	InviterID   primitive.ObjectID `bson:"inviterId" json:"inviterId"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	Status      InviteStatus       `bson:"status" json:"status"`
}

// InviteStatus represents the status of a community invitation
type InviteStatus string

const (
	InviteStatusPending  InviteStatus = "pending"
	InviteStatusAccepted InviteStatus = "accepted"
	InviteStatusRejected InviteStatus = "rejected"
)

// CommunityService provides methods to interact with communities
type CommunityService struct {
	Collection       *mongo.Collection
	InviteCollection *mongo.Collection
	UserService      *UserService
	ToolService      *ToolService
}

// NewCommunityService creates a new CommunityService
func NewCommunityService(db *Database) *CommunityService {
	return &CommunityService{
		Collection:       db.Database.Collection("communities"),
		InviteCollection: db.Database.Collection("community_invites"),
		UserService:      db.UserService,
		ToolService:      db.ToolService,
	}
}

// CreateCommunity creates a new community
func (s *CommunityService) CreateCommunity(ctx context.Context, name string, imageHash []byte, ownerID primitive.ObjectID) (*Community, error) {
	now := time.Now()
	community := &Community{
		ID:        primitive.NewObjectID(),
		Name:      name,
		ImageHash: imageHash,
		CreatedAt: now,
		UpdatedAt: now,
		OwnerID:   ownerID,
	}

	_, err := s.Collection.InsertOne(ctx, community)
	if err != nil {
		return nil, err
	}

	// Add owner to community
	err = s.UserService.AddUserToCommunity(ctx, ownerID, community.ID, CommunityRoleOwner)
	if err != nil {
		// Rollback community creation if we can't add the owner
		_ = s.DeleteCommunity(ctx, community.ID)
		return nil, err
	}

	return community, nil
}

// GetCommunity retrieves a community by ID
func (s *CommunityService) GetCommunity(ctx context.Context, id primitive.ObjectID) (*Community, error) {
	var community Community
	err := s.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&community)
	if err != nil {
		return nil, err
	}
	return &community, nil
}

// UpdateCommunity updates a community
func (s *CommunityService) UpdateCommunity(ctx context.Context, id primitive.ObjectID, name string, imageHash []byte) error {
	update := bson.M{
		"$set": bson.M{
			"updatedAt": time.Now(),
		},
	}

	if name != "" {
		update["$set"].(bson.M)["name"] = name
	}

	if imageHash != nil {
		update["$set"].(bson.M)["imageHash"] = imageHash
	}

	_, err := s.Collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

// DeleteCommunity deletes a community
func (s *CommunityService) DeleteCommunity(ctx context.Context, id primitive.ObjectID) error {
	// Get all users in the community
	users, err := s.GetCommunityUsers(ctx, id, 0)
	if err != nil {
		return err
	}

	// Remove community from all users
	for _, user := range users {
		err = s.UserService.RemoveUserFromCommunity(ctx, user.ID, id)
		if err != nil {
			log.Error().Err(err).Str("userId", user.ID.Hex()).Str("communityId", id.Hex()).Msg("Failed to remove user from community")
		}
	}

	// Delete all invites for this community
	_, err = s.InviteCollection.DeleteMany(ctx, bson.M{"communityId": id})
	if err != nil {
		log.Error().Err(err).Str("communityId", id.Hex()).Msg("Failed to delete community invites")
	}

	// Delete the community
	_, err = s.Collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// GetCommunityUsers retrieves all users in a community
func (s *CommunityService) GetCommunityUsers(ctx context.Context, communityID primitive.ObjectID, page int) ([]*User, error) {
	if page < 0 {
		page = 0
	}

	skip := page * defaultPageSize

	// Find users with this community in their communities array
	filter := bson.M{"communities.id": communityID}
	opts := options.Find().
		SetSort(bson.D{{Key: "name", Value: 1}}). // Sort by name
		SetSkip(int64(skip)).
		SetLimit(int64(defaultPageSize))

	cursor, err := s.UserService.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var users []*User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// InviteUserToCommunity creates an invitation for a user to join a community
func (s *CommunityService) InviteUserToCommunity(ctx context.Context, communityID, userID, inviterID primitive.ObjectID) (*CommunityInvite, error) {
	// Check if community exists
	_, err := s.GetCommunity(ctx, communityID)
	if err != nil {
		return nil, err
	}

	// Check if user exists
	_, err = s.UserService.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Check if inviter is in the community
	inviter, err := s.UserService.GetUserByID(ctx, inviterID)
	if err != nil {
		return nil, err
	}

	var isMember bool
	for _, comm := range inviter.Communities {
		if comm.ID == communityID {
			isMember = true
			break
		}
	}

	if !isMember {
		return nil, fmt.Errorf("inviter is not a member of the community")
	}

	// Check if invitation already exists
	var existingInvite CommunityInvite
	err = s.InviteCollection.FindOne(ctx, bson.M{
		"communityId": communityID,
		"userId":      userID,
		"status":      InviteStatusPending,
	}).Decode(&existingInvite)

	if err == nil {
		// Invitation already exists
		return &existingInvite, nil
	} else if err != mongo.ErrNoDocuments {
		// Some other error occurred
		return nil, err
	}

	// Create new invitation
	invite := &CommunityInvite{
		ID:          primitive.NewObjectID(),
		CommunityID: communityID,
		UserID:      userID,
		InviterID:   inviterID,
		CreatedAt:   time.Now(),
		Status:      InviteStatusPending,
	}

	_, err = s.InviteCollection.InsertOne(ctx, invite)
	if err != nil {
		return nil, err
	}

	return invite, nil
}

// GetUserPendingInvites retrieves all pending invites for a user
func (s *CommunityService) GetUserPendingInvites(ctx context.Context, userID primitive.ObjectID) ([]*CommunityInvite, error) {
	filter := bson.M{
		"userId": userID,
		"status": InviteStatusPending,
	}

	cursor, err := s.InviteCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var invites []*CommunityInvite
	if err := cursor.All(ctx, &invites); err != nil {
		return nil, err
	}
	return invites, nil
}

// CountUserPendingInvites counts the number of pending invites for a user
func (s *CommunityService) CountUserPendingInvites(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	filter := bson.M{
		"userId": userID,
		"status": InviteStatusPending,
	}

	count, err := s.InviteCollection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// AcceptInvite accepts a community invitation
func (s *CommunityService) AcceptInvite(ctx context.Context, inviteID, userID primitive.ObjectID) error {
	// Get the invite
	var invite CommunityInvite
	err := s.InviteCollection.FindOne(ctx, bson.M{
		"_id":    inviteID,
		"userId": userID,
		"status": InviteStatusPending,
	}).Decode(&invite)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("invite not found or not pending")
		}
		return err
	}

	// Update invite status
	_, err = s.InviteCollection.UpdateOne(
		ctx,
		bson.M{"_id": inviteID},
		bson.M{"$set": bson.M{"status": InviteStatusAccepted}},
	)
	if err != nil {
		return err
	}

	// Add user to community
	return s.UserService.AddUserToCommunity(ctx, userID, invite.CommunityID, CommunityRoleUser)
}

// RejectInvite rejects a community invitation
func (s *CommunityService) RejectInvite(ctx context.Context, inviteID, userID primitive.ObjectID) error {
	// Get the invite
	var invite CommunityInvite
	err := s.InviteCollection.FindOne(ctx, bson.M{
		"_id":    inviteID,
		"userId": userID,
		"status": InviteStatusPending,
	}).Decode(&invite)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("invite not found or not pending")
		}
		return err
	}

	// Update invite status
	_, err = s.InviteCollection.UpdateOne(
		ctx,
		bson.M{"_id": inviteID},
		bson.M{"$set": bson.M{"status": InviteStatusRejected}},
	)
	return err
}

// LeaveCommunity removes a user from a community
func (s *CommunityService) LeaveCommunity(ctx context.Context, communityID, userID primitive.ObjectID) error {
	// Get the community
	community, err := s.GetCommunity(ctx, communityID)
	if err != nil {
		return err
	}

	// Check if user is the owner
	if community.OwnerID == userID {
		return fmt.Errorf("community owner cannot leave the community")
	}

	// Remove user from community
	return s.UserService.RemoveUserFromCommunity(ctx, userID, communityID)
}

// AddToolToCommunity adds a tool to a community
func (s *CommunityService) AddToolToCommunity(ctx context.Context, toolID int64, communityID primitive.ObjectID) error {
	// Get the tool
	tool, err := s.ToolService.GetToolByID(ctx, toolID)
	if err != nil {
		return err
	}

	// Check if tool already has this community
	for _, comm := range tool.Communities {
		if comm == communityID {
			return nil // Tool already in this community
		}
	}

	// Add community to tool's communities
	_, err = s.ToolService.Collection.UpdateOne(
		ctx,
		bson.M{"_id": toolID},
		bson.M{
			"$push": bson.M{
				"communities": communityID,
			},
		},
	)
	return err
}

// RemoveToolFromCommunity removes a tool from a community
func (s *CommunityService) RemoveToolFromCommunity(ctx context.Context, toolID int64, communityID primitive.ObjectID) error {
	// Remove community from tool's communities
	_, err := s.ToolService.Collection.UpdateOne(
		ctx,
		bson.M{"_id": toolID},
		bson.M{
			"$pull": bson.M{
				"communities": communityID,
			},
		},
	)
	return err
}

// GetCommunityTools retrieves all tools in a community
func (s *CommunityService) GetCommunityTools(ctx context.Context, communityID primitive.ObjectID) ([]*Tool, error) {
	// Find tools with this community in their communities array
	filter := bson.M{"communities": communityID}
	cursor, err := s.ToolService.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var tools []*Tool
	if err := cursor.All(ctx, &tools); err != nil {
		return nil, err
	}
	return tools, nil
}

// CommunityExists checks if a community exists
func (s *CommunityService) CommunityExists(ctx context.Context, communityID primitive.ObjectID) (bool, error) {
	count, err := s.Collection.CountDocuments(ctx, bson.M{"_id": communityID})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
