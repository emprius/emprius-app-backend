package api

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Response is the default response of the API
type Response struct {
	Header ResponseHeader `json:"header"`
	Data   any            `json:"data,omitempty"`
}

// ResponseHeader is the header of the response
type ResponseHeader struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	ErrorCode int    `json:"errorCode,omitempty"`
}

// BinaryResponse represents a binary response that should be sent directly to the client
type BinaryResponse struct {
	ContentType string
	Data        []byte
}

// StatusResponse represents a response with a custom HTTP status code
type StatusResponse struct {
	StatusCode int
	Data       interface{}
}

type Register struct {
	UserEmail         string `json:"email"`
	RegisterAuthToken string `json:"invitationToken"`
	UserProfile
	Tokens uint64 `json:"tokens,omitempty"`
}

type Login struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token    string    `json:"token"`
	Expirity time.Time `json:"expirity"`
}

// NotificationPreferences represents user notification settings
type NotificationPreferences map[string]bool

type AdditionalContacts map[string]string

func (ac AdditionalContacts) Validate() error {
	for key, value := range ac {
		if len(key) == 0 || len(key) > 50 {
			return fmt.Errorf("contact key '%s' is invalid: must be non-empty and under 50 characters", key)
		}
		if len(value) == 0 || len(value) > 50 {
			return fmt.Errorf("value for key '%s' is invalid: must be non-empty and under 50 characters", key)
		}
	}

	return nil
}

// Location represents a geographical location
type Location struct {
	Latitude  int64 `json:"latitude"`  // Latitude in microdegrees
	Longitude int64 `json:"longitude"` // Longitude in microdegrees
}

// ToDBLocation converts an API Location to a DB Location
func (l *Location) ToDBLocation() db.DBLocation {
	if l == nil {
		return db.DBLocation{
			Type:        "Point",
			Coordinates: []float64{0, 0},
		}
	}
	return db.NewLocation(l.Latitude, l.Longitude)
}

// FromDBLocation converts a DB Location to an API Location
func (l *Location) FromDBLocation(dbloc db.DBLocation) *Location {
	lat, long := dbloc.GetCoordinates()
	l.Latitude = lat
	l.Longitude = long
	return l
}

type UserProfile struct {
	Name                    string                  `json:"name"`
	Community               string                  `json:"community"`
	Location                *Location               `json:"location,omitempty"`
	Active                  *bool                   `json:"active,omitempty"`
	Avatar                  []byte                  `json:"avatar,omitempty"`
	Password                string                  `json:"password,omitempty"`
	ActualPassword          string                  `json:"actualPassword,omitempty"`
	Bio                     string                  `json:"bio,omitempty"`
	NotificationPreferences NotificationPreferences `json:"notificationPreferences,omitempty"`
	InviteCodes             []*SimpleInviteCode     `json:"inviteCodes,omitempty"`
	AdditionalContacts      AdditionalContacts      `json:"additionalContacts,omitempty"`
	LanguageCode            string                  `json:"lang,omitempty"`
}

// UserCommunityInfo represents a user's role in a community
type UserCommunityInfo struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

// User preview for list calls (does not send full information)
type UserPreview struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	AvatarHash  types.HexBytes `json:"avatarHash"`
	RatingCount int            `json:"ratingCount"`
	Rating      int            `json:"rating"`
	Active      bool           `json:"active"`
	Karma       int64          `json:"karma"`
}

// User represents the user type
type User struct {
	UserPreview
	Email                   string                  `json:"email"`
	Community               string                  `json:"community"`
	Tokens                  uint64                  `json:"tokens"`
	Location                Location                `json:"location"`
	Verified                bool                    `json:"verified"`
	CreatedAt               time.Time               `json:"createdAt"`
	LastSeen                time.Time               `json:"lastSeen"`
	Bio                     string                  `json:"bio"`
	InviteCodes             []*SimpleInviteCode     `json:"inviteCodes,omitempty"`
	Communities             []UserCommunityInfo     `json:"communities,omitempty"`
	NotificationPreferences NotificationPreferences `json:"notificationPreferences,omitempty"`
	AdditionalContacts      AdditionalContacts      `json:"additionalContacts,omitempty"`
	LanguageCode            string                  `json:"lang,omitempty"`
	UnreadMessageCount      *UnreadMessageSummary   `json:"unreadMessageCount,omitempty"`
}

// SimpleInviteCode represents a simplified invitation code with only essential fields
type SimpleInviteCode struct {
	Code      string    `json:"code"`
	CreatedOn time.Time `json:"createdOn"`
}

// InviteCode represents an invitation code
type InviteCode struct {
	ID        string     `json:"id"`
	Code      string     `json:"code"`
	UsedByID  *string    `json:"usedById,omitempty"`
	UsedOn    *time.Time `json:"usedOn,omitempty"`
	CreatedOn time.Time  `json:"createdOn"`
}

// FromDBInviteCode converts a DB InviteCode to an API InviteCode
func (i *InviteCode) FromDBInviteCode(dbic *db.InviteCode) *InviteCode {
	i.ID = dbic.ID.Hex()
	i.Code = dbic.Code
	i.CreatedOn = dbic.CreatedOn

	if dbic.UsedByID != nil {
		usedByID := dbic.UsedByID.Hex()
		i.UsedByID = &usedByID
	}

	i.UsedOn = dbic.UsedOn

	return i
}

// FromDBUserPreview converts a DB User to an API UserPreview
func (up *UserPreview) FromDBUserPreview(dbu *db.User) *UserPreview {
	up.ID = dbu.ID.Hex()
	up.Name = dbu.Name
	up.AvatarHash = dbu.AvatarHash
	up.Rating = int(dbu.Rating)
	up.RatingCount = dbu.RatingCount
	up.Active = dbu.Active
	up.Karma = dbu.Karma
	return up
}

// FromDBUser converts a DB User to an API User (full version)
// If includePrivateData is true, private data like notification preferences and useRealLocation will be included
func (u *User) FromDBUser(dbu *db.User, includePrivateData bool, includeAdditionalContacts bool) *User {
	// First fill UserPreview fields
	u.FromDBUserPreview(dbu)

	// Then fill additional User fields
	u.Email = dbu.Email
	u.Community = dbu.Community
	u.Tokens = dbu.Tokens

	// Use real location if explicitly requested, otherwise use obfuscated location
	if includePrivateData {
		u.Location.FromDBLocation(dbu.Location)
		u.NotificationPreferences = dbu.NotificationPreferences
		u.LanguageCode = dbu.LanguageCode
	} else {
		u.Location.FromDBLocation(dbu.ObfuscatedLocation)
	}

	if includeAdditionalContacts {
		u.AdditionalContacts = dbu.AdditionalContacts
	}

	u.Verified = dbu.Verified
	u.Bio = dbu.Bio
	u.CreatedAt = dbu.CreatedAt
	u.LastSeen = dbu.LastSeen

	// Convert communities
	if len(dbu.Communities) > 0 {
		u.Communities = make([]UserCommunityInfo, len(dbu.Communities))
		for i, comm := range dbu.Communities {
			u.Communities[i] = UserCommunityInfo{
				ID:   comm.ID.Hex(),
				Role: string(comm.Role),
			}
		}
	}

	// Use the rating count from the database user object (already calculated and stored)
	u.RatingCount = dbu.RatingCount

	return u
}

// ObjectID returns the ObjectID of the user, or a nil ObjectID if the ID is not a valid ObjectID.
// This is useful for converting the ID to an ObjectID for use in database queries.
func (u *User) ObjectID() primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(u.ID)
	if err != nil {
		return primitive.NilObjectID
	}
	return id
}

type UsersWrapper struct {
	Users      []*User        `json:"users"`
	Pagination PaginationInfo `json:"pagination"`
}

// ToolHistoryEntry represents an entry in a nomadic tool's history
type ToolHistoryEntry struct {
	ID         string   `json:"id"`
	UserID     string   `json:"userId"`
	UserName   string   `json:"userName"`
	PickupDate int64    `json:"pickupDate"` // Unix timestamp
	Location   Location `json:"location"`
	BookingID  string   `json:"bookingId,omitempty"`
}

// Tool is the type of the tool
type Tool struct {
	ID                 int64              `json:"id"`
	UserID             string             `json:"userId"`
	UserActive         bool               `json:"userActive"`
	ActualUserID       string             `json:"actualUserId,omitempty"`
	ActualUserActive   bool               `json:"actualUserActive,omitempty"`
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	IsAvailable        *bool              `json:"isAvailable"`
	MayBeFree          *bool              `json:"mayBeFree"`
	AskWithFee         *bool              `json:"askWithFee"`
	Images             []types.HexBytes   `json:"images"`
	TransportOptions   []int              `json:"transportOptions"`
	Category           int                `json:"toolCategory"`
	Location           Location           `json:"location"`
	Cost               uint64             `json:"cost"`
	ToolValuation      *uint64            `json:"toolValuation"`
	EstimatedDailyCost uint64             `json:"estimatedDailyCost"`
	Height             uint32             `json:"height"`
	Weight             uint32             `json:"weight"`
	MaxDistance        uint32             `json:"maxDistance"`
	ReservedDates      []db.DateRange     `json:"reservedDates"`
	IsNomadic          *bool              `json:"isNomadic"`
	Communities        []string           `json:"communities,omitempty"`
	HistoryEntries     []ToolHistoryEntry `json:"historyEntries,omitempty"`
}

// FromDBTool converts a DB Tool to an API Tool.
// If database is provided, it will populate UserActive and ActualUserActive fields.
// If useRealLocation is true, the real location is used instead of the obfuscated one
func (t *Tool) FromDBTool(dbt *db.Tool, database *db.Database, useRealLocation ...bool) *Tool {
	t.ID = dbt.ID
	t.UserID = dbt.UserID.Hex()
	if !dbt.ActualUserID.IsZero() {
		t.ActualUserID = dbt.ActualUserID.Hex()
	}
	t.Title = dbt.Title
	t.Description = dbt.Description
	t.IsAvailable = &dbt.IsAvailable
	t.MayBeFree = &dbt.MayBeFree
	t.AskWithFee = &dbt.AskWithFee
	t.Cost = dbt.Cost
	for i := range dbt.Images {
		t.Images = append(t.Images, dbt.Images[i].Hash)
	}
	for i := range dbt.TransportOptions {
		t.TransportOptions = append(t.TransportOptions, int(dbt.TransportOptions[i].ID))
	}
	t.Category = dbt.ToolCategory

	// Use real location if explicitly requested, otherwise use obfuscated location
	if len(useRealLocation) > 0 && useRealLocation[0] {
		t.Location.FromDBLocation(dbt.Location)
	} else {
		t.Location.FromDBLocation(dbt.ObfuscatedLocation)
	}

	t.ToolValuation = &dbt.ToolValuation
	t.EstimatedDailyCost = dbt.EstimatedDailyCost
	t.Height = dbt.Height
	t.Weight = dbt.Weight
	t.MaxDistance = dbt.MaxDistance
	t.ReservedDates = dbt.ReservedDates
	t.IsNomadic = &dbt.IsNomadic

	// Convert communities
	if len(dbt.Communities) > 0 {
		t.Communities = make([]string, len(dbt.Communities))
		for i, comm := range dbt.Communities {
			t.Communities[i] = comm.Hex()
		}
	}

	// Populate user activity status if database is provided
	if database != nil {
		// Check owner user activity status
		if !dbt.UserID.IsZero() {
			if user, err := database.UserService.GetUserByID(context.Background(), dbt.UserID); err == nil {
				t.UserActive = user.Active
			} else {
				// Default to false if user lookup fails
				t.UserActive = false
				log.Debug().Err(err).Str("userId", dbt.UserID.Hex()).Msg("Failed to get user for UserActive check")
			}
		}

		// Check actual user activity status
		if !dbt.ActualUserID.IsZero() {
			if actualUser, err := database.UserService.GetUserByID(context.Background(), dbt.ActualUserID); err == nil {
				t.ActualUserActive = actualUser.Active
			} else {
				// Default to false if user lookup fails
				t.ActualUserActive = false
				log.Debug().Err(err).Str("actualUserId", dbt.ActualUserID.Hex()).Msg("Failed to get actual user for ActualUserActive check")
			}
		}
	}

	return t
}

type ToolID struct {
	ID int64 `json:"id"`
}

// PaginationInfo contains pagination metadata
type PaginationInfo struct {
	Current  int   `json:"current"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
	Pages    int   `json:"pages"`
}

// PaginatedToolsResponse wraps tools with pagination info
type PaginatedToolsResponse struct {
	Tools      []*Tool        `json:"tools"`
	Pagination PaginationInfo `json:"pagination"`
}

// PaginatedBookingsResponse wraps bookings with pagination info
type PaginatedBookingsResponse struct {
	Bookings   []*BookingResponse `json:"bookings"`
	Pagination PaginationInfo     `json:"pagination"`
}

type PaginatedUnifiedRatingsResponse struct {
	Ratings    []*db.UnifiedRating `json:"ratings"`
	Pagination PaginationInfo      `json:"pagination"`
}

// ToolSearch is the type of the tool search
type ToolSearch struct {
	SearchTerm       string  `json:"searchTerm"`
	Categories       []int   `json:"categories"`
	Distance         int     `json:"distance"`
	MaxCost          *uint64 `json:"maxCost"`
	MayBeFree        *bool   `json:"mayBeFree"`
	AvailableFrom    int     `json:"availableFrom"`
	TransportOptions []int   `json:"transportOptions"`
	CommunityID      string  `json:"communityId,omitempty"`
	Page             int     `json:"page,omitempty"`
}

type Info struct {
	Users      int               `json:"users"`
	Tools      int               `json:"tools"`
	Categories []db.ToolCategory `json:"categories"`
	Transports []db.Transport    `json:"transports"`
}

// CreateBookingRequest represents the request to create a new booking
type CreateBookingRequest struct {
	ToolID    string `json:"toolId"`
	StartDate int64  `json:"startDate"`
	EndDate   int64  `json:"endDate"`
	Contact   string `json:"contact"`
	Comments  string `json:"comments"`
}

// BookingResponse represents the API response for a booking
type BookingResponse struct {
	ID            string    `json:"id"`
	ToolID        string    `json:"toolId"`
	FromUserID    string    `json:"fromUserId"`
	ToUserID      string    `json:"toUserId"`
	StartDate     int64     `json:"startDate"`
	EndDate       int64     `json:"endDate"`
	Contact       string    `json:"contact"`
	Comments      string    `json:"comments"`
	BookingStatus string    `json:"bookingStatus"`
	IsRated       *bool     `json:"isRated"`
	IsNomadic     bool      `json:"isNomadic"`
	PickupPlace   *Location `json:"pickupPlace,omitempty"`

	// Legacy fields for backward compatibility
	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

// Booking status constants for API
const (
	BookingStatusAccepted  = "ACCEPTED"
	BookingStatusRejected  = "REJECTED"
	BookingStatusCancelled = "CANCELLED"
	BookingStatusReturned  = "RETURNED"
	BookingStatusPending   = "PENDING"
	BookingStatusPicked    = "PICKED"
)

// BookingStatusUpdate represents a request to update a booking's status
type BookingStatusUpdate struct {
	Status string `json:"status"`
}

// Rating represents a rating for a booking
type Rating struct {
	ID         string           `json:"id"`
	BookingID  string           `json:"bookingId"`
	ToolID     string           `json:"toolId"`
	Rating     int              `json:"rating"`
	Comment    string           `json:"comment"`
	Images     []types.HexBytes `json:"images"`
	FromUserID string           `json:"fromUserId"`
	ToUserID   string           `json:"toUserId"`
	RatedAt    int64            `json:"ratedAt"`
}

func (r *Rating) FromDB(b *db.BookingRating) *Rating {
	if b == nil {
		return new(Rating)
	}
	r.ID = b.ID.Hex()
	r.BookingID = b.BookingID.Hex()
	r.Rating = b.Rating
	r.Comment = b.RatingComment
	for i := range b.Images {
		r.Images = append(r.Images, b.Images[i].Hash)
	}
	r.FromUserID = b.FromUserID.Hex()
	r.ToUserID = b.ToUserID.Hex()
	r.RatedAt = b.RatedAt.Unix()
	return r
}

// PendingActionsResponse represents the response for pending actions
type PendingActionsResponse struct {
	PendingRatingsCount  int64 `json:"pendingRatingsCount"`
	PendingRequestsCount int64 `json:"pendingRequestsCount"`
	PendingInvitesCount  int64 `json:"pendingInvitesCount"`
}

// Message-related request/response types

// Message type constants
const (
	MessageTypePrivate   = "private"
	MessageTypeCommunity = "community"
	MessageTypeGeneral   = "general"
	MessageTypeAll       = "all"
)

// SendMessageRequest represents a request to send a message
type SendMessageRequest struct {
	Type        string   `json:"type"`                  // private|community|general
	RecipientID string   `json:"recipientId,omitempty"` // User ID for private, Community ID for community
	Content     string   `json:"content,omitempty"`
	ImageHashes []string `json:"imageHashes,omitempty"` // Hashes from image upload
	Images      []string `json:"images,omitempty"`      // Alternative field name for compatibility
	ReplyToID   string   `json:"replyToId,omitempty"`
}

// Validate validates the send message request
func (r *SendMessageRequest) Validate() error {
	if r.Type != "private" && r.Type != "community" && r.Type != "general" {
		return fmt.Errorf("invalid message type")
	}

	// Check if we have content or images (support both field names)
	totalImages := len(r.ImageHashes) + len(r.Images)
	if len(r.Content) == 0 && totalImages == 0 {
		return fmt.Errorf("message must have either content or images")
	}

	if len(r.Content) > 5000 {
		return fmt.Errorf("message content cannot exceed 5000 characters")
	}

	if r.Type == "private" && r.RecipientID == "" {
		return fmt.Errorf("recipient ID is required for private messages")
	}

	if r.Type == "community" && r.RecipientID == "" {
		return fmt.Errorf("recipient ID (community ID) is required for community messages")
	}

	return nil
}

// MessageResponse represents a message in API responses
type MessageResponse struct {
	ID            string           `json:"id"`
	Type          string           `json:"type"`
	SenderID      string           `json:"senderId"`
	SenderName    string           `json:"senderName"`
	RecipientID   string           `json:"recipientId,omitempty"`
	RecipientName string           `json:"recipientName"`
	CommunityID   string           `json:"communityId,omitempty"`
	Content       string           `json:"content,omitempty"`
	Images        []types.HexBytes `json:"images,omitempty"`
	CreatedAt     int64            `json:"createdAt"`
	IsRead        bool             `json:"isRead"`
	ReplyToID     string           `json:"replyToId,omitempty"`
}

// FromDB converts a database message to API response
// If userID is provided, it will determine the read status for that user
func (m *MessageResponse) FromDB(dbMessage *db.Message, database *db.Database, userID ...primitive.ObjectID) *MessageResponse {
	m.ID = dbMessage.ID.Hex()
	m.Type = string(dbMessage.Type)
	m.SenderID = dbMessage.SenderID.Hex()
	m.Content = dbMessage.Content
	m.Images = dbMessage.Images
	m.CreatedAt = dbMessage.CreatedAt.Unix()

	// Get sender name
	if sender, err := database.UserService.GetUserByID(context.Background(), dbMessage.SenderID); err == nil {
		m.SenderName = sender.Name
	}

	// Get recipient name only if recipient exists
	if dbMessage.RecipientID != nil {
		m.RecipientID = dbMessage.RecipientID.Hex()
		if recipient, err := database.UserService.GetUserByID(context.Background(), *dbMessage.RecipientID); err == nil {
			m.RecipientName = recipient.Name
		}
	}

	if dbMessage.CommunityID != nil {
		m.CommunityID = dbMessage.CommunityID.Hex()
	}

	if dbMessage.ReplyToID != nil {
		m.ReplyToID = dbMessage.ReplyToID.Hex()
	}

	// Determine read status if userID is provided
	// isRead indicates whether the OTHER user (recipient) has read the message
	if len(userID) > 0 && !userID[0].IsZero() {
		currentUserID := userID[0]

		// Determine which user's read status to check
		var userToCheck primitive.ObjectID

		if dbMessage.SenderID == currentUserID {
			// Current user is the sender - check if RECIPIENT has read it
			if dbMessage.RecipientID != nil && !dbMessage.RecipientID.IsZero() {
				userToCheck = *dbMessage.RecipientID
			} else {
				// For general/community messages, sender cannot know if others read it
				m.IsRead = false
				return m
			}
		} else {
			// Current user is the recipient/viewer - check if THEY have read it
			userToCheck = currentUserID
		}

		// Check the read status in the database
		var readStatus db.MessageReadStatus
		err := database.MessageService.ReadStatusCollection.FindOne(
			context.Background(),
			map[string]interface{}{
				"userId":          userToCheck,
				"conversationKey": dbMessage.ConversationKey,
			},
		).Decode(&readStatus)

		if err == nil {
			// Message is read if its ID is <= lastReadId
			// MongoDB ObjectIDs are chronologically ordered, so we can compare them directly
			m.IsRead = dbMessage.ID.Hex() <= readStatus.LastReadID.Hex()
		} else {
			// No read status found, message is unread
			m.IsRead = false
		}
	} else {
		// No user context provided, default to false
		m.IsRead = false
	}

	return m
}

// ConversationResponse represents a conversation in API responses
type ConversationResponse struct {
	ID              string          `json:"id"`
	Type            string          `json:"type"`
	Participants    []UserPreview   `json:"participants,omitempty"`
	Community       *CommunityInfo  `json:"community,omitempty"`
	LastMessage     MessageResponse `json:"lastMessage"`
	UnreadCount     int64           `json:"unreadCount"`
	MessageCount    int64           `json:"messageCount"`
	LastMessageTime int64           `json:"lastMessageTime"`
}

// CommunityInfo represents basic community information
type CommunityInfo struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Image types.HexBytes `json:"image,omitempty"`
}

// FromDB converts a database conversation to API response
func (c *ConversationResponse) FromDB(dbConv *db.Conversation, database *db.Database, userID primitive.ObjectID) *ConversationResponse {
	c.ID = dbConv.ID.Hex()
	c.Type = string(dbConv.Type)
	c.MessageCount = dbConv.MessageCount
	c.LastMessageTime = dbConv.LastMessageTime.Unix()

	// Get participants for private conversations
	if dbConv.Type == db.MessageTypePrivate && len(dbConv.Participants) > 0 {
		c.Participants = make([]UserPreview, len(dbConv.Participants))
		for i, participantID := range dbConv.Participants {
			if user, err := database.UserService.GetUserByID(context.Background(), participantID); err == nil {
				c.Participants[i].FromDBUserPreview(user)
			}
		}
	}

	// Get community info for community conversations
	if dbConv.Type == db.MessageTypeCommunity && dbConv.CommunityID != nil {
		if community, err := database.CommunityService.GetCommunity(context.Background(), *dbConv.CommunityID); err == nil {
			c.Community = &CommunityInfo{
				ID:    community.ID.Hex(),
				Name:  community.Name,
				Image: community.Image,
			}
		}
	}

	// Get last message
	var lastMessage db.Message
	if err := database.MessageService.Collection.FindOne(
		context.Background(),
		map[string]interface{}{"_id": dbConv.LastMessageID},
	).Decode(&lastMessage); err == nil {
		c.LastMessage.FromDB(&lastMessage, database)
	}

	// Get unread count for this user
	// Prepare parameters for conversation key generation
	var senderID, recipientID primitive.ObjectID
	if len(dbConv.Participants) > 0 {
		senderID = dbConv.Participants[0]
		if len(dbConv.Participants) > 1 {
			recipientID = dbConv.Participants[1]
		}
	}
	conversationKey := db.GenerateConversationKeyFromData(dbConv.Type, senderID, recipientID, dbConv.CommunityID)

	var readStatus db.MessageReadStatus
	if err := database.MessageService.ReadStatusCollection.FindOne(
		context.Background(),
		map[string]interface{}{
			"userId":          userID,
			"conversationKey": conversationKey,
		},
	).Decode(&readStatus); err == nil {
		c.UnreadCount = readStatus.UnreadCount
	} else {
		// If no read status exists, all messages are unread
		c.UnreadCount = dbConv.MessageCount
	}

	return c
}

// UnreadMessageSummary represents unread message counts
type UnreadMessageSummary struct {
	Total        int64            `json:"total"`
	Private      int64            `json:"private"`
	Communities  map[string]int64 `json:"communities,omitempty"`
	GeneralForum int64            `json:"generalForum"`
}

// PaginatedMessagesResponse wraps messages with pagination info
type PaginatedMessagesResponse struct {
	Messages   []*MessageResponse `json:"messages"`
	Pagination PaginationInfo     `json:"pagination"`
}

// PaginatedConversationsResponse wraps conversations with pagination info
type PaginatedConversationsResponse struct {
	Conversations []*ConversationResponse `json:"conversations"`
	Pagination    PaginationInfo          `json:"pagination"`
}

// MarkMessagesReadRequest represents a request to mark messages as read
type MarkMessagesReadRequest struct {
	Type             string `json:"type,omitempty"`
	ConversationWith string `json:"conversationWith,omitempty"` // User ID for private, Community ID for community
}
