package api

import (
	"time"

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
	Name           string    `json:"name"`
	Community      string    `json:"community"`
	Location       *Location `json:"location,omitempty"`
	Active         *bool     `json:"active,omitempty"`
	Avatar         []byte    `json:"avatar,omitempty"`
	Password       string    `json:"password,omitempty"`
	ActualPassword string    `json:"actualPassword,omitempty"`
	Bio            string    `json:"bio,omitempty"`
}

// User represents the user type
type User struct {
	ID          string         `json:"id"`
	Email       string         `json:"email"`
	Name        string         `json:"name"`
	Community   string         `json:"community"`
	Tokens      uint64         `json:"tokens"`
	Active      bool           `json:"active"`
	Rating      int            `json:"rating"`
	AvatarHash  types.HexBytes `json:"avatarHash"`
	Location    Location       `json:"location"`
	Verified    bool           `json:"verified"`
	CreatedAt   time.Time      `json:"createdAt"`
	LastSeen    time.Time      `json:"lastSeen"`
	Bio         string         `json:"bio"`
	RatingCount int            `json:"ratingCount"`
}

// FromDBUser converts a DB User to an API User
func (u *User) FromDBUser(dbu *db.User) *User {
	u.ID = dbu.ID.Hex()
	u.Email = dbu.Email
	u.Name = dbu.Name
	u.Community = dbu.Community
	u.Tokens = dbu.Tokens
	u.Active = dbu.Active
	u.Rating = int(dbu.Rating)
	u.AvatarHash = dbu.AvatarHash
	u.Location.FromDBLocation(dbu.Location)
	u.Verified = dbu.Verified

	// Set new fields from DB user or defaults if not present
	u.Bio = dbu.Bio
	u.CreatedAt = dbu.CreatedAt
	u.LastSeen = dbu.LastSeen
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
	Users []*User `json:"users"`
}

// Tool is the type of the tool
type Tool struct {
	ID               int64            `json:"id"`
	UserID           string           `json:"userId"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	IsAvailable      *bool            `json:"isAvailable"`
	MayBeFree        *bool            `json:"mayBeFree"`
	AskWithFee       *bool            `json:"askWithFee"`
	Cost             uint64           `json:"cost"`
	Images           []types.HexBytes `json:"images"`
	TransportOptions []int            `json:"transportOptions"`
	Category         int              `json:"toolCategory"`
	Location         Location         `json:"location"`
	EstimatedValue   *uint64          `json:"estimatedValue"`
	Height           uint32           `json:"height"`
	Weight           uint32           `json:"weight"`
	MaxDistance      uint32           `json:"maxDistance"`
	ReservedDates    []db.DateRange   `json:"reservedDates"`
}

// FromDBTool converts a DB Tool to an API Tool.
func (t *Tool) FromDBTool(dbt *db.Tool) *Tool {
	t.ID = dbt.ID
	t.UserID = dbt.UserID.Hex()
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
	t.Location.FromDBLocation(dbt.Location)
	t.EstimatedValue = &dbt.EstimatedValue
	t.Height = dbt.Height
	t.Weight = dbt.Weight
	t.MaxDistance = dbt.MaxDistance
	t.ReservedDates = dbt.ReservedDates
	return t
}

type ToolID struct {
	ID int64 `json:"id"`
}

type ToolsWrapper struct {
	Tools []*Tool `json:"tools"`
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
	ID            string `json:"id"`
	ToolID        string `json:"toolId"`
	FromUserID    string `json:"fromUserId"`
	ToUserID      string `json:"toUserId"`
	StartDate     int64  `json:"startDate"`
	EndDate       int64  `json:"endDate"`
	Contact       string `json:"contact"`
	Comments      string `json:"comments"`
	BookingStatus string `json:"bookingStatus"`
	IsRated       *bool  `json:"isRated"`

	// Legacy fields for backward compatibility
	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

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
