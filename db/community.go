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
	Image     types.HexBytes     `json:"images"`
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

// CommunityInviteWithDetails represents a community invitation with community details
type CommunityInviteWithDetails struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	CommunityID primitive.ObjectID `bson:"communityId" json:"communityId"`
	UserID      primitive.ObjectID `bson:"userId" json:"userId"`
	InviterID   primitive.ObjectID `bson:"inviterId" json:"inviterId"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	Status      InviteStatus       `bson:"status" json:"status"`
	Community   struct {
		Name  string         `bson:"name" json:"name"`
		Image types.HexBytes `bson:"image" json:"image"`
	} `bson:"community" json:"community"`
}

// InviteStatus represents the status of a community invitation
type InviteStatus string

const (
	InviteStatusPending  InviteStatus = "PENDING"
	InviteStatusAccepted InviteStatus = "ACCEPTED"
	InviteStatusRejected InviteStatus = "REJECTED"
	InviteStatusCanceled InviteStatus = "CANCELED"
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
func (s *CommunityService) CreateCommunity(
	ctx context.Context,
	name string, image types.HexBytes,
	ownerID primitive.ObjectID,
) (*Community, error) {
	now := time.Now()
	community := &Community{
		ID:        primitive.NewObjectID(),
		Name:      name,
		Image:     image,
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
func (s *CommunityService) UpdateCommunity(ctx context.Context, id primitive.ObjectID, name string, image types.HexBytes) error {
	update := bson.M{
		"$set": bson.M{
			"updatedAt": time.Now(),
		},
	}

	if name != "" {
		update["$set"].(bson.M)["name"] = name
	}

	if image != nil {
		update["$set"].(bson.M)["image"] = image
	}

	_, err := s.Collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

// DeleteCommunity deletes a community
func (s *CommunityService) DeleteCommunity(ctx context.Context, id primitive.ObjectID) error {
	// Get all users in the community
	users, _, err := s.GetCommunityUsers(ctx, id, 0, "")
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
func (s *CommunityService) GetCommunityUsers(
	ctx context.Context,
	communityID primitive.ObjectID,
	page int,
	term string,
) ([]*User, int64, error) {
	if page < 0 {
		page = 0
	}

	skip := page * DefaultPageSize

	// Create a case-insensitive regex pattern for partial name matching
	pattern := "(?i).*" + regexp.QuoteMeta(SanitizeString(term)) + ".*"
	regex := primitive.Regex{Pattern: pattern, Options: "i"}

	// Find users who belong to the specified community and have a name match
	filter := bson.M{
		"communities.id": communityID,
		"name":           regex,
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "name", Value: 1}}). // Sort by name
		SetSkip(int64(skip)).
		SetLimit(int64(DefaultPageSize))

	cursor, err := s.UserService.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var users []*User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, 0, err
	}

	// Get total count (without pagination)
	total, err := s.UserService.Collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// InviteUserToCommunity creates an invitation for a user to join a community
func (s *CommunityService) InviteUserToCommunity(
	ctx context.Context,
	communityID,
	userID,
	inviterID primitive.ObjectID,
) (*CommunityInvite, error) {
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

// GetUserPendingInvitesWithDetails retrieves all pending invites for a user with community details
func (s *CommunityService) GetUserPendingInvitesWithDetails(
	ctx context.Context,
	userID primitive.ObjectID,
) ([]*CommunityInviteWithDetails, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"userId": userID,
				"status": InviteStatusPending,
			},
		},
		{
			"$lookup": bson.M{
				"from":         "communities",
				"localField":   "communityId",
				"foreignField": "_id",
				"as":           "communityDetails",
			},
		},
		{
			"$unwind": "$communityDetails",
		},
		{
			"$addFields": bson.M{
				"community": bson.M{
					"name":  "$communityDetails.name",
					"image": "$communityDetails.image",
				},
			},
		},
		{
			"$project": bson.M{
				"communityDetails": 0,
			},
		},
	}

	cursor, err := s.InviteCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var invites []*CommunityInviteWithDetails
	if err := cursor.All(ctx, &invites); err != nil {
		return nil, err
	}
	return invites, nil
}

// GetInviteWithDetails retrieves a single invite with community details
func (s *CommunityService) GetInviteWithDetails(
	ctx context.Context,
	inviteID primitive.ObjectID,
) (*CommunityInviteWithDetails, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"_id": inviteID,
			},
		},
		{
			"$lookup": bson.M{
				"from":         "communities",
				"localField":   "communityId",
				"foreignField": "_id",
				"as":           "communityDetails",
			},
		},
		{
			"$unwind": "$communityDetails",
		},
		{
			"$addFields": bson.M{
				"community": bson.M{
					"name":  "$communityDetails.name",
					"image": "$communityDetails.image",
				},
			},
		},
		{
			"$project": bson.M{
				"communityDetails": 0,
			},
		},
	}

	cursor, err := s.InviteCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var invites []*CommunityInviteWithDetails
	if err := cursor.All(ctx, &invites); err != nil {
		return nil, err
	}

	if len(invites) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	return invites[0], nil
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

// CancelInvite cancels a community invitation (only the inviter can cancel)
func (s *CommunityService) CancelInvite(ctx context.Context, inviteID, inviterID primitive.ObjectID) error {
	// Get the invite
	var invite CommunityInvite
	err := s.InviteCollection.FindOne(ctx, bson.M{
		"_id":       inviteID,
		"inviterId": inviterID,
		"status":    InviteStatusPending,
	}).Decode(&invite)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("invite not found, not pending, or user is not the inviter")
		}
		return err
	}

	// Update invite status
	_, err = s.InviteCollection.UpdateOne(
		ctx,
		bson.M{"_id": inviteID},
		bson.M{"$set": bson.M{"status": InviteStatusCanceled}},
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

// GetCommunityToolsPaginated retrieves tools in a community with pagination and search
func (s *CommunityService) GetCommunityToolsPaginated(ctx context.Context,
	communityID primitive.ObjectID,
	page int,
	pageSize int,
	searchTerm string,
) ([]*Tool, int64, error) {
	if page < 0 {
		page = 0
	}

	if pageSize < 0 {
		pageSize = DefaultPageSize
	}

	skip := page * pageSize

	// Build the base filter
	filter := bson.M{"communities": communityID}

	// Add search filter if search term is provided
	if searchTerm != "" {
		searchTerm = SanitizeString(searchTerm)
		filter["$or"] = []bson.M{
			{"title": bson.M{"$regex": searchTerm, "$options": "i"}},
			{"description": bson.M{"$regex": searchTerm, "$options": "i"}},
		}
	}

	// Get total count
	total, err := s.ToolService.Collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	findOptions := options.Find()
	findOptions.SetSkip(int64(skip))
	findOptions.SetLimit(int64(pageSize))
	findOptions.SetSort(bson.D{{Key: "title", Value: 1}})
	cursor, err := s.ToolService.Collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var tools []*Tool
	if err := cursor.All(ctx, &tools); err != nil {
		return nil, 0, err
	}

	return tools, total, nil
}

// CommunityExists checks if a community exists
func (s *CommunityService) CommunityExists(ctx context.Context, communityID primitive.ObjectID) (bool, error) {
	count, err := s.Collection.CountDocuments(ctx, bson.M{"_id": communityID})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserCommunities retrieves all communities for a specific user
func (s *CommunityService) GetUserCommunities(
	ctx context.Context,
	userID primitive.ObjectID,
	page int,
	searchTerm string,
) ([]*Community, int64, error) {
	if page < 0 {
		page = 0
	}

	skip := page * DefaultPageSize

	// Build the match filter for user membership
	matchFilter := bson.M{
		"members": bson.M{
			"$elemMatch": bson.M{
				"_id": userID,
			},
		},
	}

	// Add search filter if search term is provided
	if searchTerm != "" {
		// Create a case-insensitive regex pattern for partial name matching
		pattern := "(?i).*" + regexp.QuoteMeta(SanitizeString(searchTerm)) + ".*"
		regex := primitive.Regex{Pattern: pattern, Options: "i"}
		matchFilter["name"] = regex
	}

	// Use aggregation pipeline with $facet to get both data and count
	pipeline := mongo.Pipeline{
		// Stage 1: Lookup users to find communities where the user is a member
		bson.D{{Key: "$lookup", Value: bson.M{
			"from":         "users",
			"localField":   "_id",
			"foreignField": "communities.id",
			"as":           "members",
		}}},
		// Stage 2: Match communities where the user is a member and optionally filter by name
		bson.D{{Key: "$match", Value: matchFilter}},
		// Stage 3: Sort by name
		bson.D{{Key: "$sort", Value: bson.M{
			"name": 1, // Sort by name ascending
		}}},
		// Stage 4: Use $facet to get both data and count
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
		Data  []*Community `bson:"data"`
		Count []struct {
			Total int64 `bson:"total"`
		} `bson:"count"`
	}

	if err := cursor.All(ctx, &result); err != nil {
		return nil, 0, err
	}

	var communities []*Community
	var total int64
	if len(result) > 0 {
		communities = result[0].Data
		if len(result[0].Count) > 0 {
			total = result[0].Count[0].Total
		}
	}

	return communities, total, nil
}

// GetCommunityWithMemberCount retrieves a community by ID with member count and tool count
func (s *CommunityService) GetCommunityWithMemberCount(
	ctx context.Context,
	id primitive.ObjectID,
) (*Community, int64, int64, error) {
	// Get the community
	community, err := s.GetCommunity(ctx, id)
	if err != nil {
		return nil, 0, 0, err
	}

	// Count the members
	membersCount, err := s.CountCommunityMembers(ctx, id)
	if err != nil {
		return nil, 0, 0, err
	}

	// Count the tools
	toolsCount, err := s.CountCommunityTools(ctx, id)
	if err != nil {
		return nil, 0, 0, err
	}

	return community, membersCount, toolsCount, nil
}

// GetUserCommunitiesWithMemberCount retrieves all communities for a specific user with member counts and tool counts
func (s *CommunityService) GetUserCommunitiesWithMemberCount(
	ctx context.Context,
	userID primitive.ObjectID,
	page int,
	searchTerm string,
) ([]*Community, map[primitive.ObjectID]int64, map[primitive.ObjectID]int64, int64, error) {
	// Get the communities
	communities, totalCommunities, err := s.GetUserCommunities(ctx, userID, page, searchTerm)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	// Count members and tools for each community
	memberCounts := make(map[primitive.ObjectID]int64, len(communities))
	toolCounts := make(map[primitive.ObjectID]int64, len(communities))

	for _, community := range communities {
		// Count members
		memberCount, err := s.CountCommunityMembers(ctx, community.ID)
		if err != nil {
			log.Error().Err(err).Str("communityId", community.ID.Hex()).Msg("Failed to count community members")
			memberCounts[community.ID] = 0
		} else {
			memberCounts[community.ID] = memberCount
		}

		// Count tools
		toolCount, err := s.CountCommunityTools(ctx, community.ID)
		if err != nil {
			log.Error().Err(err).Str("communityId", community.ID.Hex()).Msg("Failed to count community tools")
			toolCounts[community.ID] = 0
		} else {
			toolCounts[community.ID] = toolCount
		}
	}

	return communities, memberCounts, toolCounts, totalCommunities, nil
}

// CountCommunityMembers counts the number of users in a community
func (s *CommunityService) CountCommunityMembers(ctx context.Context, communityID primitive.ObjectID) (int64, error) {
	// Find users with this community in their communities array
	filter := bson.M{"communities.id": communityID}
	count, err := s.UserService.Collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountCommunityTools counts the number of tools in a community
func (s *CommunityService) CountCommunityTools(ctx context.Context, communityID primitive.ObjectID) (int64, error) {
	// Find tools with this community in their communities array
	filter := bson.M{"communities": communityID}
	count, err := s.ToolService.Collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}
