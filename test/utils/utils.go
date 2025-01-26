package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/service"
	qt "github.com/frankban/quicktest"
)

const (
	jwtSecret = "secret"
	// RegisterToken is the test register token for authentication.
	RegisterToken = "registerToken"
)

// TestService is a test service for the API.
type TestService struct {
	s   *service.Service
	t   *testing.T
	url string
	c   *http.Client
}

// NewTestService creates a new test service.
func NewTestService(t *testing.T) *TestService {
	ctx := context.Background()

	// Start MongoDB container
	container, err := db.StartMongoContainer(ctx)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to start MongoDB container"))
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	// Get MongoDB connection string
	mongoURI, err := container.Endpoint(ctx, "mongodb")
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get MongoDB connection string"))

	s, err := service.New(mongoURI, jwtSecret, RegisterToken, true)
	qt.Assert(t, err, qt.IsNil)
	rand.NewSource(time.Now().UnixNano())
	port := 20000 + rand.New(rand.NewSource(time.Now().UnixNano())).Intn(8192)
	s.Start("127.0.0.1", port)
	time.Sleep(time.Second * 1) // Wait for HTTP server to start
	return &TestService{
		s:   s,
		t:   t,
		url: fmt.Sprintf("http://localhost:%d", port),
		c:   http.DefaultClient,
	}
}

// Request sends a request to the service and returns the response body and status code.
// The body is expected to be a JSON object or null.
// If jwt is not empty, it will be sent as a Bearer token.
func (s *TestService) Request(method, jwt string, jsonBody any, urlPath ...string) ([]byte, int) {
	body, err := json.Marshal(jsonBody)
	qt.Assert(s.t, err, qt.IsNil)
	u, err := url.Parse(s.url)
	qt.Assert(s.t, err, qt.IsNil)
	// Handle the case where the last path component contains query parameters
	lastIndex := len(urlPath) - 1
	if lastIndex >= 0 && strings.Contains(urlPath[lastIndex], "?") {
		parts := strings.SplitN(urlPath[lastIndex], "?", 2)
		urlPath[lastIndex] = parts[0]
		u.Path = path.Join(u.Path, path.Join(urlPath...))
		u.RawQuery = parts[1]
	} else {
		u.Path = path.Join(u.Path, path.Join(urlPath...))
	}
	headers := http.Header{}
	if jwt != "" {
		headers = http.Header{"Authorization": []string{"Bearer " + jwt}}
	}
	req, err := http.NewRequest(method, u.String(), bytes.NewReader(body))
	qt.Assert(s.t, err, qt.IsNil)
	req.Header = headers
	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.c.Do(req)
	if err != nil {
		s.t.Logf("http error: %v", err)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		s.t.Logf("read error: %v", err)
	}
	return data, resp.StatusCode
}

// RegisterAndLogin registers a new user and returns the JWT token
func (s *TestService) RegisterAndLogin(email, name, password string) string {
	jwt, _ := s.RegisterAndLoginWithID(email, name, password)
	return jwt
}

// RegisterAndLoginWithID registers a new user and returns the JWT token and user ID
func (s *TestService) RegisterAndLoginWithID(email, name, password string) (string, string) {
	// Register
	_, code := s.Request(http.MethodPost, "",
		&api.Register{
			UserEmail:         email,
			RegisterAuthToken: RegisterToken,
			UserProfile: api.UserProfile{
				Name:      name,
				Community: "testCommunity",
				Password:  password,
				Location: &api.Location{
					Type: "Point",
					Coordinates: []float64{
						2.492793,  // longitude
						41.695384, // latitude
					},
				},
			},
		},
		"register",
	)
	qt.Assert(s.t, code, qt.Equals, 200)

	// Login
	resp, code := s.Request(http.MethodPost, "",
		&api.Login{
			Email:    email,
			Password: password,
		},
		"login",
	)
	qt.Assert(s.t, code, qt.Equals, 200)

	var loginResponse struct {
		Data api.LoginResponse `json:"data"`
	}
	err := json.Unmarshal(resp, &loginResponse)
	qt.Assert(s.t, err, qt.IsNil)
	jwt := loginResponse.Data.Token

	// Get profile to get user ID
	resp, code = s.Request(http.MethodGet, jwt, nil, "profile")
	qt.Assert(s.t, code, qt.Equals, 200)

	var profileResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	err = json.Unmarshal(resp, &profileResponse)
	qt.Assert(s.t, err, qt.IsNil)

	return jwt, profileResponse.Data.ID
}

// CreateTool creates a new tool and returns its ID
func (s *TestService) CreateTool(jwt string, title string) int64 {
	resp, code := s.Request(http.MethodPost, jwt,
		map[string]interface{}{
			"title":          title,
			"description":    "Test tool",
			"mayBeFree":      true,
			"askWithFee":     false,
			"cost":           10,
			"category":       1,
			"estimatedValue": 20,
			"height":         30,
			"weight":         40,
			"location": map[string]interface{}{
				"type": "Point",
				"coordinates": []float64{
					2.492793,  // longitude
					41.695384, // latitude
				},
			},
		},
		"tools",
	)
	qt.Assert(s.t, code, qt.Equals, 200)

	var response struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	err := json.Unmarshal(resp, &response)
	qt.Assert(s.t, err, qt.IsNil)
	return response.Data.ID
}
