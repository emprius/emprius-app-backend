package db

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestImageService(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Start MongoDB container
	container, err := StartMongoContainer(ctx)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to start MongoDB container"))
	defer func() { _ = container.Terminate(ctx) }()

	// Get MongoDB connection string
	mongoURI, err := container.Endpoint(ctx, "mongodb")
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to get MongoDB connection string"))

	// Create a MongoDB client
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create MongoDB client"))
	defer func() { _ = client.Disconnect(ctx) }()

	// Use a random database name for isolation
	dbName := RandomDatabaseName()
	database := client.Database(dbName)

	// Initialize ImageService
	imageService := NewImageService(&Database{
		Client:   client,
		Database: database,
	})

	// Test ImageService methods
	c.Run("Insert and Retrieve Image", func(c *qt.C) {
		image := &Image{
			Hash:    []byte("testhash"),
			Name:    "test_image.jpg",
			Content: []byte("testcontent"),
			Link:    "https://example.com/test_image.jpg",
		}

		// Insert Image
		insertResult, err := imageService.InsertImage(ctx, image)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert image"))
		c.Assert(insertResult.InsertedID, qt.Not(qt.IsNil), qt.Commentf("Insert result ID is nil"))

		// Retrieve Image by Hash
		retrievedImage, err := imageService.GetImage(ctx, image.Hash)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve image by hash"))
		c.Assert(retrievedImage.Hash, qt.DeepEquals, image.Hash, qt.Commentf("Hashes do not match"))
		c.Assert(retrievedImage.Name, qt.Equals, image.Name, qt.Commentf("Names do not match"))
		c.Assert(retrievedImage.Content, qt.DeepEquals, image.Content, qt.Commentf("Contents do not match"))
		c.Assert(retrievedImage.Link, qt.Equals, image.Link, qt.Commentf("Links do not match"))
	})

	c.Run("Get All Images", func(c *qt.C) {
		// Insert additional images
		images := []*Image{
			{
				Hash:    []byte("hash1"),
				Name:    "image1.jpg",
				Content: []byte("content1"),
				Link:    "https://example.com/image1.jpg",
			},
			{
				Hash:    []byte("hash2"),
				Name:    "image2.jpg",
				Content: []byte("content2"),
				Link:    "https://example.com/image2.jpg",
			},
		}

		for _, img := range images {
			_, err := imageService.InsertImage(ctx, img)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert test image"))
		}

		// Retrieve all images
		allImages, err := imageService.GetAllImages(ctx)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve all images"))
		c.Assert(len(allImages) >= 3, qt.Equals, true, qt.Commentf("Expected at least 3 images in database"))

		// Verify the content of retrieved images
		foundImages := make(map[string]bool)
		for _, img := range allImages {
			foundImages[img.Name] = true
		}

		c.Assert(foundImages["image1.jpg"], qt.Equals, true, qt.Commentf("Image1 not found in results"))
		c.Assert(foundImages["image2.jpg"], qt.Equals, true, qt.Commentf("Image2 not found in results"))
	})

	c.Run("Get Non-existent Image", func(c *qt.C) {
		// Try to retrieve an image with a non-existent hash
		_, err := imageService.GetImage(ctx, []byte("nonexistenthash"))
		c.Assert(err, qt.Equals, mongo.ErrNoDocuments, qt.Commentf("Expected no documents error"))
	})
}
