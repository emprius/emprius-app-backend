package api

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"

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
	return up
}

// FromDBUser converts a DB User to an API User (full version)
// If includePrivateData is true, private data like notification preferences and useRealLocation will be included
func (u *User) FromDBUser(dbu *db.User, dbc *db.Database, includePrivateData bool) *User {
	// First fill UserPreview fields
	u.FromDBUserPreview(dbu)

	// Then fill additional User fields
	u.Email = dbu.Email
	u.Community = dbu.Community
	u.Tokens = dbu.Tokens

	// Use real location if explicitly requested, otherwise use obfuscated location
	if includePrivateData {
		u.Location.FromDBLocation(dbu.Location)
		u.NotificationPreferences = NotificationPreferences(dbu.NotificationPreferences)
	} else {
		u.Location.FromDBLocation(dbu.ObfuscatedLocation)
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

	// Get rating count (number of ratings received by this user)
	filter := bson.M{
		"rateeId": u.ID,
		"raterId": bson.M{"$ne": u.ID}, // exclude self-ratings
	}

	ratingCount, err := dbc.Database.Collection("ratings").CountDocuments(context.Background(), filter)
	if err != nil {
		log.Error().Err(err).Str("userId", u.ID).Msg("Failed to count user ratings")
		// Continue even if count fails, just set to 0
		u.RatingCount = 0
	} else {
		u.RatingCount = int(ratingCount)
	}

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
	ActualUserID       string             `json:"actualUserId,omitempty"`
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
	IsNomadic          bool               `json:"isNomadic"`
	Communities        []string           `json:"communities,omitempty"`
	HistoryEntries     []ToolHistoryEntry `json:"historyEntries,omitempty"`
}

// FromDBTool converts a DB Tool to an API Tool.
// If useRealLocation is true, the real location is used instead of the obfuscated one
func (t *Tool) FromDBTool(dbt *db.Tool, useRealLocation ...bool) *Tool {
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
	t.IsNomadic = dbt.IsNomadic

	// Convert communities
	if len(dbt.Communities) > 0 {
		t.Communities = make([]string, len(dbt.Communities))
		for i, comm := range dbt.Communities {
			t.Communities[i] = comm.Hex()
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
