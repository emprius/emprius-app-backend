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
	"testing"
	"time"

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
	u.Path = path.Join(u.Path, path.Join(urlPath...))
	headers := http.Header{}
	if jwt != "" {
		headers = http.Header{"Authorization": []string{"Bearer " + jwt}}
	}
	req, err := http.NewRequest(method, u.String(), bytes.NewReader(body))
	qt.Assert(s.t, err, qt.IsNil)
	req.Header = headers
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
