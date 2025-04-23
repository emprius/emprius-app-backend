package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// InviteCode represents the schema for the "invite_codes" collection.
type InviteCode struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Code      string              `bson:"code" json:"code"`
	OwnerID   primitive.ObjectID  `bson:"ownerId" json:"ownerId"`
	UsedByID  *primitive.ObjectID `bson:"usedById,omitempty" json:"usedById,omitempty"`
	UsedOn    *time.Time          `bson:"usedOn,omitempty" json:"usedOn,omitempty"`
	CreatedOn time.Time           `bson:"createdOn" json:"createdOn"`
}

// InviteCodeService provides methods to interact with the "invite_codes" collection.
type InviteCodeService struct {
	Collection *mongo.Collection
}

// NewInviteCodeService creates a new InviteCodeService.
func NewInviteCodeService(db *Database) *InviteCodeService {
	return &InviteCodeService{
		Collection: db.Database.Collection("invite_codes"),
	}
}

// GenerateInviteCode generates a random invite code.
func GenerateInviteCode() (string, error) {
	// Generate 8 random bytes
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Convert to a hex string
	code := hex.EncodeToString(bytes)
	// Format as XXXX-XXXX for readability
	return fmt.Sprintf("%s-%s", code[:4], code[4:]), nil
}

// CreateInviteCode creates a new invite code for a user.
func (s *InviteCodeService) CreateInviteCode(ctx context.Context, ownerID primitive.ObjectID) (*InviteCode, error) {
	code, err := GenerateInviteCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate invite code: %w", err)
	}

	inviteCode := &InviteCode{
		Code:      code,
		OwnerID:   ownerID,
		CreatedOn: time.Now(),
	}

	result, err := s.Collection.InsertOne(ctx, inviteCode)
	if err != nil {
		return nil, fmt.Errorf("failed to insert invite code: %w", err)
	}

	inviteCode.ID = result.InsertedID.(primitive.ObjectID)
	return inviteCode, nil
}

// GetInviteCodeByCode retrieves an invite code by its code value.
func (s *InviteCodeService) GetInviteCodeByCode(ctx context.Context, code string) (*InviteCode, error) {
	var inviteCode InviteCode
	filter := bson.M{"code": code}
	err := s.Collection.FindOne(ctx, filter).Decode(&inviteCode)
	if err != nil {
		return nil, err
	}
	return &inviteCode, nil
}

// GetUnusedInviteCodesByOwnerID retrieves all unused invite codes owned by a user.
func (s *InviteCodeService) GetUnusedInviteCodesByOwnerID(ctx context.Context, ownerID primitive.ObjectID) ([]*InviteCode, error) {
	filter := bson.M{
		"ownerId":  ownerID,
		"usedById": nil,
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdOn", Value: -1}})

	cursor, err := s.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var inviteCodes []*InviteCode
	for cursor.Next(ctx) {
		var inviteCode InviteCode
		if err := cursor.Decode(&inviteCode); err != nil {
			return nil, err
		}
		inviteCodes = append(inviteCodes, &inviteCode)
	}
	return inviteCodes, nil
}

// GetAllInviteCodesByOwnerID retrieves all invite codes owned by a user.
func (s *InviteCodeService) GetAllInviteCodesByOwnerID(ctx context.Context, ownerID primitive.ObjectID) ([]*InviteCode, error) {
	filter := bson.M{"ownerId": ownerID}
	opts := options.Find().SetSort(bson.D{{Key: "createdOn", Value: -1}})

	cursor, err := s.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var inviteCodes []*InviteCode
	for cursor.Next(ctx) {
		var inviteCode InviteCode
		if err := cursor.Decode(&inviteCode); err != nil {
			return nil, err
		}
		inviteCodes = append(inviteCodes, &inviteCode)
	}
	return inviteCodes, nil
}

// MarkCodeAsUsed marks an invite code as used by a user.
func (s *InviteCodeService) MarkCodeAsUsed(ctx context.Context, code string, userID primitive.ObjectID) error {
	filter := bson.M{"code": code, "usedById": nil}
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"usedById": userID,
			"usedOn":   now,
		},
	}

	result, err := s.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("invite code not found or already used")
	}

	return nil
}

// GetLastCodeRequestTime retrieves the creation time of the most recent invite code for a user.
func (s *InviteCodeService) GetLastCodeRequestTime(ctx context.Context, ownerID primitive.ObjectID) (*time.Time, error) {
	filter := bson.M{"ownerId": ownerID}
	opts := options.FindOne().SetSort(bson.D{{Key: "createdOn", Value: -1}})

	var inviteCode InviteCode
	err := s.Collection.FindOne(ctx, filter, opts).Decode(&inviteCode)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &inviteCode.CreatedOn, nil
}

// CreateInviteCodes creates multiple invite codes for a user.
func (s *InviteCodeService) CreateInviteCodes(ctx context.Context, ownerID primitive.ObjectID, count int) ([]*InviteCode, error) {
	var inviteCodes []*InviteCode
	for i := 0; i < count; i++ {
		inviteCode, err := s.CreateInviteCode(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		inviteCodes = append(inviteCodes, inviteCode)
	}
	return inviteCodes, nil
}
