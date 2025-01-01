package api

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/emprius/emprius-app-backend/db"
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
type Register struct {
	UserEmail         string `json:"email"`
	RegisterAuthToken string `json:"invitationToken"`
	UserProfile
}

type Login struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type LoginResponse struct {
	Token    string    `json:"token"`
	Expirity time.Time `json:"expirity"`
}

type Image struct {
	Name string   `json:"name"`
	Data []byte   `json:"data,omitempty"`
	Hash HexBytes `json:"hash,omitempty"`
}

type UserProfile struct {
	Name      string       `json:"name"`
	Community string       `json:"community"`
	Location  *db.Location `json:"location,omitempty"`
	Active    *bool        `json:"active,omitempty"`
	Avatar    HexBytes     `json:"avatar,omitempty"`
	Password  string       `json:"password,omitempty"`
}

type UsersWrapper struct {
	Users []db.User `json:"users"`
}

// Tool is the type of the tool
type Tool struct {
	ID               int64       `json:"id"`
	Title            string      `json:"title"`
	Description      string      `json:"description"`
	MayBeFree        *bool       `json:"mayBeFree"`
	AskWithFee       *bool       `json:"askWithFee"`
	Cost             *uint64     `json:"cost"`
	Images           []HexBytes  `json:"images"`
	TransportOptions []int       `json:"transportOptions"`
	Category         int         `json:"category"`
	Location         db.Location `json:"location"`
	EstimatedValue   uint64      `json:"estimatedValue"`
	Height           uint32      `json:"height"`
	Weight           uint32      `json:"weight"`
}

type ToolID struct {
	ID int64 `json:"id"`
}

type ToolsWrapper struct {
	Tools []db.Tool `json:"tools"`
}

// ToolSearch is the type of the tool search
type ToolSearch struct {
	Term          string  `json:"term"`
	Categories    []int   `json:"categories"`
	Distance      int     `json:"distance"`
	MaxCost       *uint64 `json:"maxCost"`
	MayBeFree     *bool   `json:"mayBeFree"`
	AvailableFrom int     `json:"availableFrom"`
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
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// HexBytes is a []byte which encodes as hexadecimal in json, as opposed to the
// base64 default.
type HexBytes []byte

func (b *HexBytes) String() string {
	return hex.EncodeToString(*b)
}

func (b HexBytes) MarshalJSON() ([]byte, error) {
	enc := make([]byte, hex.EncodedLen(len(b))+2)
	enc[0] = '"'
	hex.Encode(enc[1:], b)
	enc[len(enc)-1] = '"'
	return enc, nil
}

func (b *HexBytes) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid JSON string: %q", data)
	}
	data = data[1 : len(data)-1]

	// Strip a leading "0x" prefix, for backwards compatibility.
	if len(data) >= 2 && data[0] == '0' && (data[1] == 'x' || data[1] == 'X') {
		data = data[2:]
	}

	decLen := hex.DecodedLen(len(data))
	if cap(*b) < decLen {
		*b = make([]byte, decLen)
	}
	if _, err := hex.Decode(*b, data); err != nil {
		return err
	}
	return nil
}

// HexStringToHexBytes converts a hex string to a HexBytes.
// It strips a leading '0x' or '0X' if found, for backwards compatibility.
// Panics if the string is not a valid hex string.
func HexStringToHexBytes(hexString string) HexBytes {
	// Strip a leading "0x" prefix, for backwards compatibility.
	if len(hexString) >= 2 && hexString[0] == '0' && (hexString[1] == 'x' || hexString[1] == 'X') {
		hexString = hexString[2:]
	}
	b, err := hex.DecodeString(hexString)
	if err != nil {
		panic(err)
	}
	return b
}
