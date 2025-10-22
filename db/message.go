package db

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/emprius/emprius-app-backend/types"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MessageType represents the type of message
type MessageType string

const (
	MessageTypePrivate   MessageType = "private"
	MessageTypeCommunity MessageType = "community"
	MessageTypeGeneral   MessageType = "general"
	MessageTypeAll       MessageType = "all"
)

// Message represents a message in the system
type Message struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Type        MessageType         `bson:"type" json:"type"`
	SenderID    primitive.ObjectID  `bson:"senderId" json:"senderId"`
	RecipientID *primitive.ObjectID `bson:"recipientId,omitempty" json:"recipientId,omitempty"`
	CommunityID *primitive.ObjectID `bson:"communityId,omitempty" json:"communityId,omitempty"`

	// Content
	Content string           `bson:"content,omitempty" json:"content,omitempty"`
	Images  []types.HexBytes `bson:"images,omitempty" json:"images,omitempty"` // Image hashes

	// Threading
	ThreadID  *primitive.ObjectID `bson:"threadId,omitempty" json:"threadId,omitempty"`
	ReplyToID *primitive.ObjectID `bson:"replyToId,omitempty" json:"replyToId,omitempty"`

	// Metadata
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`

	// Optimization fields
	ConversationKey string `bson:"conversationKey" json:"-"` // For fast lookups
}

// MessageReadStatus tracks read status efficiently
type MessageReadStatus struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	UserID          primitive.ObjectID `bson:"userId"`
	ConversationKey string             `bson:"conversationKey"`
	LastReadID      primitive.ObjectID `bson:"lastReadId"`
	LastReadTime    time.Time          `bson:"lastReadTime"`
	UnreadCount     int64              `bson:"unreadCount"` // Cached for performance
}

// Conversation for optimized conversation listing
type Conversation struct {
	ID              primitive.ObjectID   `bson:"_id,omitempty"`
	Type            MessageType          `bson:"type"`
	Participants    []primitive.ObjectID `bson:"participants,omitempty"`
	CommunityID     *primitive.ObjectID  `bson:"communityId,omitempty"`
	LastMessageID   primitive.ObjectID   `bson:"lastMessageId"`
	LastMessageTime time.Time            `bson:"lastMessageTime"`
	LastSenderID    primitive.ObjectID   `bson:"lastSenderId"`
	MessageCount    int64                `bson:"messageCount"`
	CreatedAt       time.Time            `bson:"createdAt"`
	UpdatedAt       time.Time            `bson:"updatedAt"`
}

// MessageFilter represents filters for message queries
type MessageFilter struct {
	Type             MessageType
	UserID           primitive.ObjectID
	ConversationWith *primitive.ObjectID
	CommunityID      *primitive.ObjectID
	UnreadOnly       bool
}

// UnreadMessageSummary represents unread message counts
type UnreadMessageSummary struct {
	Total        int64            `json:"total"`
	Private      int64            `json:"private"`
	Communities  map[string]int64 `json:"communities,omitempty"`
	GeneralForum int64            `json:"generalForum"`
}

// MessageService provides methods to interact with messages
type MessageService struct {
	Collection             *mongo.Collection
	ReadStatusCollection   *mongo.Collection
	ConversationCollection *mongo.Collection
	UserService            *UserService
	CommunityService       *CommunityService
	ImageService           *ImageService
}

// NewMessageService creates a new MessageService
func NewMessageService(db *Database) *MessageService {
	return &MessageService{
		Collection:             db.Database.Collection("messages"),
		ReadStatusCollection:   db.Database.Collection("message_read_status"),
		ConversationCollection: db.Database.Collection("conversations"),
		UserService:            db.UserService,
		CommunityService:       db.CommunityService,
		ImageService:           db.ImageService,
	}
}

// Validate checks if the message data meets the required constraints
func (m *Message) Validate() error {
	// Content validation
	if len(m.Content) == 0 && len(m.Images) == 0 {
		return fmt.Errorf("message must have either content or images")
	}

	if len(m.Content) > 5000 {
		return fmt.Errorf("message content cannot exceed 5000 characters")
	}

	if len(m.Images) > 10 {
		return fmt.Errorf("message cannot have more than 10 images")
	}

	// Type-specific validation
	switch m.Type {
	case MessageTypePrivate:
		if m.RecipientID == nil || m.RecipientID.IsZero() {
			return fmt.Errorf("recipient ID is required for private messages")
		}
		if m.CommunityID != nil {
			return fmt.Errorf("community ID should not be set for private messages")
		}
	case MessageTypeCommunity:
		if m.CommunityID == nil || m.CommunityID.IsZero() {
			return fmt.Errorf("community ID is required for community messages")
		}
		if m.RecipientID != nil {
			return fmt.Errorf("recipient ID should not be set for community messages")
		}
	case MessageTypeGeneral:
		if m.RecipientID != nil || m.CommunityID != nil {
			return fmt.Errorf("general forum messages should not have recipient or community ID")
		}
	default:
		return fmt.Errorf("invalid message type: %s", m.Type)
	}

	return nil
}

// GenerateConversationKeyFromData generates a conversation key from the given parameters
// This is a utility function that can be used by both Message objects and other code that needs
// to generate conversation keys without creating a Message object.
func GenerateConversationKeyFromData(msgType MessageType, senderID, recipientID primitive.ObjectID, communityID *primitive.ObjectID) string {
	switch msgType {
	case MessageTypePrivate:
		if recipientID.IsZero() {
			return ""
		}
		// Sort IDs to ensure consistent key regardless of sender/recipient order
		ids := []string{senderID.Hex(), recipientID.Hex()}
		sort.Strings(ids)
		return fmt.Sprintf("private:%s:%s", ids[0], ids[1])
	case MessageTypeCommunity:
		if communityID == nil || communityID.IsZero() {
			return ""
		}
		return fmt.Sprintf("community:%s", communityID.Hex())
	case MessageTypeGeneral:
		return "general"
	default:
		return ""
	}
}

// GenerateConversationKey generates a deterministic key for conversation lookups
func (m *Message) GenerateConversationKey() string {
	recipientID := primitive.NilObjectID
	if m.RecipientID != nil {
		recipientID = *m.RecipientID
	}
	return GenerateConversationKeyFromData(m.Type, m.SenderID, recipientID, m.CommunityID)
}

// SendMessage sends a new message
func (s *MessageService) SendMessage(ctx context.Context, message *Message) (*Message, error) {
	// Validate message
	if err := message.Validate(); err != nil {
		return nil, err
	}

	// Set creation time
	message.CreatedAt = time.Now()

	// Generate conversation key
	message.ConversationKey = message.GenerateConversationKey()
	if message.ConversationKey == "" {
		return nil, fmt.Errorf("failed to generate conversation key")
	}

	// Validate permissions
	if err := s.validateSendPermissions(ctx, message); err != nil {
		return nil, err
	}

	// Validate image hashes if present
	if len(message.Images) > 0 {
		if err := s.validateImageHashes(ctx, message.Images); err != nil {
			return nil, err
		}
	}

	// Insert message
	result, err := s.Collection.InsertOne(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("failed to insert message: %w", err)
	}

	message.ID = result.InsertedID.(primitive.ObjectID)

	// Update or create conversation
	if err := s.updateConversation(ctx, message); err != nil {
		log.Error().Err(err).Msg("failed to update conversation")
		// Don't fail the message send for this
	}

	// Update unread counts for recipients
	if err := s.updateUnreadCounts(ctx, message); err != nil {
		log.Error().Err(err).Msg("failed to update unread counts")
		// Don't fail the message send for this
	}

	return message, nil
}

// GetMessages retrieves messages with pagination
func (s *MessageService) GetMessages(ctx context.Context, filter MessageFilter, page, pageSize int) ([]*Message, int64, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = DefaultPageSize
	}

	skip := page * pageSize

	// Build MongoDB filter
	mongoFilter := s.buildMessageFilter(filter)

	// Create aggregation pipeline for complex queries
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: mongoFilter}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "createdAt", Value: -1}}}},
		bson.D{{Key: "$facet", Value: bson.D{
			{Key: "data", Value: bson.A{
				bson.D{{Key: "$skip", Value: int64(skip)}},
				bson.D{{Key: "$limit", Value: int64(pageSize)}},
			}},
			{Key: "count", Value: bson.A{
				bson.D{{Key: "$count", Value: "total"}},
			}},
		}}},
	}

	cursor, err := s.Collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute message query: %w", err)
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("error closing cursor")
		}
	}()

	var result []struct {
		Data  []*Message `bson:"data"`
		Count []struct {
			Total int64 `bson:"total"`
		} `bson:"count"`
	}

	if err := cursor.All(ctx, &result); err != nil {
		return nil, 0, fmt.Errorf("failed to decode messages: %w", err)
	}

	var messages []*Message
	var total int64
	if len(result) > 0 {
		messages = result[0].Data
		if len(result[0].Count) > 0 {
			total = result[0].Count[0].Total
		}
	}

	return messages, total, nil
}

// MarkAsRead marks a specific message as read
func (s *MessageService) MarkAsRead(ctx context.Context, userID, messageID primitive.ObjectID) error {
	// Get the message to determine conversation key
	var message Message
	err := s.Collection.FindOne(ctx, bson.M{"_id": messageID}).Decode(&message)
	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}

	// Check if user has permission to read this message
	if !s.canUserReadMessage(ctx, userID, &message) {
		return fmt.Errorf("user does not have permission to read this message")
	}

	// Update read status
	return s.updateReadStatus(ctx, userID, message.ConversationKey, messageID)
}

// MarkConversationAsRead marks all messages in a conversation as read
func (s *MessageService) MarkConversationAsRead(ctx context.Context, userID primitive.ObjectID, conversationKey string) error {
	// Get the latest message in the conversation
	var latestMessage Message
	err := s.Collection.FindOne(
		ctx,
		bson.M{"conversationKey": conversationKey},
		options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}),
	).Decode(&latestMessage)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil // No messages to mark as read
		}
		return fmt.Errorf("failed to find latest message: %w", err)
	}

	// Check permission
	if !s.canUserReadMessage(ctx, userID, &latestMessage) {
		return fmt.Errorf("user does not have permission to read messages in this conversation")
	}

	// Update read status to latest message
	return s.updateReadStatus(ctx, userID, conversationKey, latestMessage.ID)
}

// GetUnreadCounts retrieves unread message counts for a user
func (s *MessageService) GetUnreadCounts(ctx context.Context, userID primitive.ObjectID) (*UnreadMessageSummary, error) {
	// Get all read statuses for the user
	cursor, err := s.ReadStatusCollection.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, fmt.Errorf("failed to get read statuses: %w", err)
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("error closing cursor")
		}
	}()

	var readStatuses []MessageReadStatus
	if err := cursor.All(ctx, &readStatuses); err != nil {
		return nil, fmt.Errorf("failed to decode read statuses: %w", err)
	}

	summary := &UnreadMessageSummary{
		Communities: make(map[string]int64),
	}

	for _, status := range readStatuses {
		if strings.HasPrefix(status.ConversationKey, "private:") {
			summary.Private += status.UnreadCount
		} else if strings.HasPrefix(status.ConversationKey, "community:") {
			communityID := strings.TrimPrefix(status.ConversationKey, "community:")
			summary.Communities[communityID] = status.UnreadCount
		} else if status.ConversationKey == "general" {
			summary.GeneralForum = status.UnreadCount
		}
		summary.Total += status.UnreadCount
	}

	return summary, nil
}

// SearchMessages searches for messages containing the specified query
func (s *MessageService) SearchMessages(ctx context.Context, userID primitive.ObjectID, query string,
	messageType MessageType, page, pageSize int,
) ([]*Message, int64, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = DefaultPageSize
	}

	skip := page * pageSize

	// Build search filter
	searchFilter := bson.M{
		"$text": bson.M{"$search": query},
	}

	// Add user permission filters
	var userFilter bson.M
	switch messageType {
	case MessageTypePrivate:
		userFilter = bson.M{
			"$or": []bson.M{
				{"senderId": userID},
				{"recipientId": userID},
			},
		}
	case MessageTypeCommunity:
		// Get user's communities for filtering
		userCommunities, err := s.UserService.GetUserCommunities(ctx, userID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get user communities: %w", err)
		}
		communityIDs := make([]primitive.ObjectID, len(userCommunities))
		for i, comm := range userCommunities {
			communityIDs[i] = comm.ID
		}
		userFilter = bson.M{
			"communityId": bson.M{"$in": communityIDs},
		}
	case MessageTypeGeneral:
		// All users can search general forum
		userFilter = bson.M{}
	default:
		// Search all message types the user has access to
		userCommunities, err := s.UserService.GetUserCommunities(ctx, userID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get user communities: %w", err)
		}
		communityIDs := make([]primitive.ObjectID, len(userCommunities))
		for i, comm := range userCommunities {
			communityIDs[i] = comm.ID
		}

		userFilter = bson.M{
			"$or": []bson.M{
				// Private messages
				{"type": MessageTypePrivate, "$or": []bson.M{
					{"senderId": userID},
					{"recipientId": userID},
				}},
				// Community messages
				{"type": MessageTypeCommunity, "communityId": bson.M{"$in": communityIDs}},
				// General forum messages
				{"type": MessageTypeGeneral},
			},
		}
	}

	// Combine search and user filters
	finalFilter := bson.M{
		"$and": []bson.M{searchFilter, userFilter},
	}

	// Add message type filter if specified
	if messageType != "" && messageType != MessageTypeAll {
		finalFilter["type"] = messageType
	}

	// Create aggregation pipeline
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: finalFilter}},
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "score", Value: bson.D{{Key: "$meta", Value: "textScore"}}},
			{Key: "createdAt", Value: -1},
		}}},
		bson.D{{Key: "$facet", Value: bson.D{
			{Key: "data", Value: bson.A{
				bson.D{{Key: "$skip", Value: int64(skip)}},
				bson.D{{Key: "$limit", Value: int64(pageSize)}},
			}},
			{Key: "count", Value: bson.A{
				bson.D{{Key: "$count", Value: "total"}},
			}},
		}}},
	}

	cursor, err := s.Collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("error closing cursor")
		}
	}()

	var result []struct {
		Data  []*Message `bson:"data"`
		Count []struct {
			Total int64 `bson:"total"`
		} `bson:"count"`
	}

	if err := cursor.All(ctx, &result); err != nil {
		return nil, 0, fmt.Errorf("failed to decode search results: %w", err)
	}

	var messages []*Message
	var total int64
	if len(result) > 0 {
		messages = result[0].Data
		if len(result[0].Count) > 0 {
			total = result[0].Count[0].Total
		}
	}

	return messages, total, nil
}

// GetConversations retrieves user's conversations with pagination
func (s *MessageService) GetConversations(ctx context.Context, userID primitive.ObjectID,
	convType MessageType, page, pageSize int,
) ([]*Conversation, int64, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = DefaultPageSize
	}

	skip := page * pageSize

	// Build filter
	filter := bson.M{}
	if convType != "all" {
		filter["type"] = convType
	}

	// Add user participation filter
	switch convType {
	case MessageTypePrivate:
		filter["participants"] = userID
	case MessageTypeCommunity:
		// Get user's communities
		communityIDs, err := s.UserService.GetUserCommunitiesIds(ctx, userID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get user communities: %w", err)
		}
		filter["communityId"] = bson.M{"$in": communityIDs}
	case MessageTypeAll:
		communityIDs, err := s.UserService.GetUserCommunitiesIds(ctx, userID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get user communities: %w", err)
		}
		filter["$or"] = []bson.M{
			{"participants": userID},                     // private convs
			{"communityId": bson.M{"$in": communityIDs}}, // community convs
			{"type": MessageTypeGeneral},                 // general forum
		}
	}

	// Use aggregation for pagination
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: filter}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "lastMessageTime", Value: -1}}}},
		bson.D{{Key: "$facet", Value: bson.D{
			{Key: "data", Value: bson.A{
				bson.D{{Key: "$skip", Value: int64(skip)}},
				bson.D{{Key: "$limit", Value: int64(pageSize)}},
			}},
			{Key: "count", Value: bson.A{
				bson.D{{Key: "$count", Value: "total"}},
			}},
		}}},
	}

	cursor, err := s.ConversationCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute conversation query: %w", err)
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("error closing cursor")
		}
	}()

	var result []struct {
		Data  []*Conversation `bson:"data"`
		Count []struct {
			Total int64 `bson:"total"`
		} `bson:"count"`
	}

	if err := cursor.All(ctx, &result); err != nil {
		return nil, 0, fmt.Errorf("failed to decode conversations: %w", err)
	}

	var conversations []*Conversation
	var total int64
	if len(result) > 0 {
		conversations = result[0].Data
		if len(result[0].Count) > 0 {
			total = result[0].Count[0].Total
		}
	}

	return conversations, total, nil
}

// Helper methods

func (s *MessageService) validateSendPermissions(ctx context.Context, message *Message) error {
	// Check if sender is active (for all message types)
	sender, err := s.UserService.GetUserByID(ctx, message.SenderID)
	if err != nil {
		return fmt.Errorf("sender not found: %w", err)
	}
	if !sender.Active {
		return fmt.Errorf("inactive users cannot send messages")
	}

	switch message.Type {
	case MessageTypePrivate:
		// Check if recipient exists and is active
		recipient, err := s.UserService.GetUserByID(ctx, *message.RecipientID)
		if err != nil {
			return fmt.Errorf("recipient not found: %w", err)
		}
		if !recipient.Active {
			return fmt.Errorf("cannot send message to inactive user")
		}

	case MessageTypeCommunity:
		// Check if sender is member of community
		userCommunities, err := s.UserService.GetUserCommunities(ctx, message.SenderID)
		if err != nil {
			return fmt.Errorf("failed to get user communities: %w", err)
		}

		isMember := false
		for _, comm := range userCommunities {
			if comm.ID == *message.CommunityID {
				isMember = true
				break
			}
		}

		if !isMember {
			return fmt.Errorf("user is not a member of this community")
		}

	case MessageTypeGeneral:
		// All authenticated users can send to general forum
		break
	}

	return nil
}

func (s *MessageService) validateImageHashes(ctx context.Context, imageHashes []types.HexBytes) error {
	for _, hash := range imageHashes {
		_, err := s.ImageService.GetImage(ctx, hash)
		if err != nil {
			return fmt.Errorf("image with hash %x not found: %w", hash, err)
		}
	}
	return nil
}

func (s *MessageService) buildMessageFilter(filter MessageFilter) bson.M {
	mongoFilter := bson.M{}

	// Type filter
	if filter.Type != "" {
		mongoFilter["type"] = filter.Type
	}

	// User-specific filters
	switch filter.Type {
	case MessageTypePrivate:
		if filter.ConversationWith != nil {
			// Specific private conversation
			conversationKey := s.generatePrivateConversationKey(filter.UserID, *filter.ConversationWith)
			mongoFilter["conversationKey"] = conversationKey
		} else {
			// All private messages for user
			mongoFilter["$or"] = []bson.M{
				{"senderId": filter.UserID},
				{"recipientId": filter.UserID},
			}
		}

	case MessageTypeCommunity:
		if filter.CommunityID != nil {
			mongoFilter["communityId"] = *filter.CommunityID
		}

	case MessageTypeGeneral:
		// No additional filters needed for general forum
	}

	return mongoFilter
}

func (s *MessageService) generatePrivateConversationKey(userID1, userID2 primitive.ObjectID) string {
	ids := []string{userID1.Hex(), userID2.Hex()}
	sort.Strings(ids)
	return fmt.Sprintf("private:%s:%s", ids[0], ids[1])
}

func (s *MessageService) canUserReadMessage(ctx context.Context, userID primitive.ObjectID, message *Message) bool {
	switch message.Type {
	case MessageTypePrivate:
		return message.SenderID == userID || (message.RecipientID != nil && *message.RecipientID == userID)
	case MessageTypeCommunity:
		if message.CommunityID == nil {
			return false
		}
		user, err := s.UserService.GetUserByID(ctx, userID)
		if err != nil {
			log.Error().Err(err).Msg("failed to get user")
			return false
		}
		for _, comm := range user.Communities {
			if comm.ID == *message.CommunityID {
				return true
			}
		}
		return false
	case MessageTypeGeneral:
		return true
	default:
		return false
	}
}

func (s *MessageService) updateConversation(ctx context.Context, message *Message) error {
	now := time.Now()

	// Prepare conversation document
	conversation := &Conversation{
		Type:            message.Type,
		LastMessageID:   message.ID,
		LastMessageTime: message.CreatedAt,
		LastSenderID:    message.SenderID,
		UpdatedAt:       now,
	}

	// Set type-specific fields
	switch message.Type {
	case MessageTypePrivate:
		conversation.Participants = []primitive.ObjectID{message.SenderID, *message.RecipientID}
	case MessageTypeCommunity:
		conversation.CommunityID = message.CommunityID
	}

	// Upsert conversation
	var filter bson.M
	switch message.Type {
	case MessageTypePrivate:
		// For private conversations, use a unique filter that doesn't conflict with $setOnInsert
		filter = bson.M{
			"type": message.Type,
			"$or": []bson.M{
				{"participants": bson.M{"$all": conversation.Participants, "$size": 2}},
				{"participants": bson.M{"$all": []primitive.ObjectID{conversation.Participants[1], conversation.Participants[0]}, "$size": 2}},
			},
		}
	case MessageTypeCommunity:
		filter = bson.M{
			"type":        message.Type,
			"communityId": message.CommunityID,
		}
	case MessageTypeGeneral:
		filter = bson.M{"type": message.Type}
	}

	update := bson.M{
		"$set": bson.M{
			"lastMessageId":   conversation.LastMessageID,
			"lastMessageTime": conversation.LastMessageTime,
			"lastSenderId":    conversation.LastSenderID,
			"updatedAt":       conversation.UpdatedAt,
		},
		"$inc": bson.M{
			"messageCount": 1,
		},
		"$setOnInsert": bson.M{
			"type":      conversation.Type,
			"createdAt": now,
		},
	}

	// Add type-specific fields to setOnInsert
	if conversation.Participants != nil {
		update["$setOnInsert"].(bson.M)["participants"] = conversation.Participants
	}
	if conversation.CommunityID != nil {
		update["$setOnInsert"].(bson.M)["communityId"] = conversation.CommunityID
	}

	_, err := s.ConversationCollection.UpdateOne(
		ctx,
		filter,
		update,
		options.Update().SetUpsert(true),
	)

	return err
}

func (s *MessageService) updateUnreadCounts(ctx context.Context, message *Message) error {
	// Get all users who should receive this message
	recipients, err := s.getMessageRecipients(ctx, message)
	if err != nil {
		return err
	}

	// Update unread counts for each recipient (excluding sender)
	for _, recipientID := range recipients {
		if recipientID == message.SenderID {
			continue // Don't increment unread count for sender
		}

		err := s.incrementUnreadCount(ctx, recipientID, message.ConversationKey)
		if err != nil {
			log.Error().Err(err).
				Str("userId", recipientID.Hex()).
				Str("conversationKey", message.ConversationKey).
				Msg("failed to increment unread count")
		}
	}

	return nil
}

func (s *MessageService) getMessageRecipients(ctx context.Context, message *Message) ([]primitive.ObjectID, error) {
	switch message.Type {
	case MessageTypePrivate:
		return []primitive.ObjectID{message.SenderID, *message.RecipientID}, nil

	case MessageTypeCommunity:
		// Get all community members
		members, _, err := s.CommunityService.GetCommunityUsers(ctx, *message.CommunityID, 0, "")
		if err != nil {
			return nil, err
		}

		recipients := make([]primitive.ObjectID, len(members))
		for i, member := range members {
			recipients[i] = member.ID
		}
		return recipients, nil

	case MessageTypeGeneral:
		// For general forum, get all active users
		// This ensures unread counts are tracked for everyone
		users, err := s.UserService.GetAllActiveUsers(ctx)
		if err != nil {
			return nil, err
		}

		recipients := make([]primitive.ObjectID, len(users))
		for i, user := range users {
			recipients[i] = user.ID
		}
		return recipients, nil

	default:
		return []primitive.ObjectID{}, nil
	}
}

func (s *MessageService) incrementUnreadCount(ctx context.Context, userID primitive.ObjectID, conversationKey string) error {
	filter := bson.M{
		"userId":          userID,
		"conversationKey": conversationKey,
	}

	update := bson.M{
		"$inc": bson.M{
			"unreadCount": 1,
		},
		"$setOnInsert": bson.M{
			"userId":          userID,
			"conversationKey": conversationKey,
			"lastReadTime":    time.Time{}, // Zero time indicates never read
		},
	}

	_, err := s.ReadStatusCollection.UpdateOne(
		ctx,
		filter,
		update,
		options.Update().SetUpsert(true),
	)

	return err
}

func (s *MessageService) updateReadStatus(ctx context.Context, userID primitive.ObjectID,
	conversationKey string, lastReadID primitive.ObjectID,
) error {
	filter := bson.M{
		"userId":          userID,
		"conversationKey": conversationKey,
	}

	update := bson.M{
		"$set": bson.M{
			"lastReadId":   lastReadID,
			"lastReadTime": time.Now(),
			"unreadCount":  0, // Reset unread count
		},
		"$setOnInsert": bson.M{
			"userId":          userID,
			"conversationKey": conversationKey,
		},
	}

	_, err := s.ReadStatusCollection.UpdateOne(
		ctx,
		filter,
		update,
		options.Update().SetUpsert(true),
	)

	return err
}
