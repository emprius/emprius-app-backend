package api

import (
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
	Name string `json:"name"`
	Data []byte `json:"data"`
	Hash string `json:"hash,omitempty"`
}

type UserProfile struct {
	Name      string       `json:"name"`
	Community string       `json:"community"`
	Location  *db.Location `json:"location"`
	Active    *bool        `json:"active"`
	Avatar    []byte       `json:"avatar"`
	Password  string       `json:"password"`
}

// Tool is the type of the tool
type Tool struct {
	Title            string      `json:"title"`
	Description      string      `json:"description"`
	MayBeFree        *bool       `json:"mayBeFree"`
	AskWithFee       *bool       `json:"askWithFee"`
	Cost             *uint64     `json:"cost"`
	Images           [][]byte    `json:"images"`
	TransportOptions []int       `json:"transportOptions"`
	Category         int         `json:"category"`
	Location         db.Location `json:"location"`
	EstimatedValue   uint64      `json:"estimatedValue"`
	Height           uint32      `json:"height"`
	Weight           uint32      `json:"weight"`
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
