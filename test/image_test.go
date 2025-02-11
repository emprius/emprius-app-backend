package test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/test/utils"
	"github.com/emprius/emprius-app-backend/types"
	qt "github.com/frankban/quicktest"
)

func TestImageEndpoint(t *testing.T) {
	c := qt.New(t)
	s := utils.NewTestService(t)

	// Register and login a user
	jwt := s.RegisterAndLogin("test@test.com", "Test User", "password123")

	// Test image data (a small PNG)
	imageData := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	decodedImage, err := base64.StdEncoding.DecodeString(imageData)
	c.Assert(err, qt.IsNil)

	// Upload image
	resp, code := s.Request(http.MethodPost, jwt, &db.Image{
		Content: decodedImage,
		Name:    "test.jpg",
	}, "images")
	c.Assert(code, qt.Equals, 200)

	var uploadResp struct {
		Header struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		} `json:"header"`
		Data struct {
			Hash types.HexBytes `json:"hash"`
		} `json:"data"`
	}
	err = json.Unmarshal(resp, &uploadResp)
	c.Assert(err, qt.IsNil)
	c.Assert(uploadResp.Header.Success, qt.Equals, true)
	c.Assert(uploadResp.Data.Hash, qt.Not(qt.IsNil))

	// Get image by hash
	hashHex := uploadResp.Data.Hash.String()
	resp2, code := s.Request(http.MethodGet, "", nil, "images", hashHex)
	c.Assert(code, qt.Equals, 200)
	c.Assert(len(resp2), qt.Equals, len(decodedImage))
	c.Assert(resp2, qt.DeepEquals, decodedImage)

	// Test invalid hash
	_, code = s.Request(http.MethodGet, "", nil, "images", "invalid")
	c.Assert(code, qt.Equals, 400)
}
