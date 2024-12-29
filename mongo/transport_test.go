package db

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestTransportService(t *testing.T) {
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

	// Initialize TransportService
	transportService := NewTransportService(&Database{
		Client:   client,
		Database: database,
	})

	// Create unique index on ID field
	_, err = transportService.Collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create unique index"))

	c.Run("Insert and Retrieve Transport", func(c *qt.C) {
		transport := &Transport{
			ID:   1,
			Name: "Test Transport",
		}

		// Insert Transport
		insertResult, err := transportService.InsertTransport(ctx, transport)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert transport"))
		c.Assert(insertResult.InsertedID, qt.Not(qt.IsNil), qt.Commentf("Insert result ID is nil"))

		// Retrieve Transport by ID
		retrievedTransport, err := transportService.GetTransportByID(ctx, transport.ID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve transport by ID"))
		c.Assert(retrievedTransport.ID, qt.Equals, transport.ID, qt.Commentf("IDs do not match"))
		c.Assert(retrievedTransport.Name, qt.Equals, transport.Name, qt.Commentf("Names do not match"))
	})

	c.Run("Get All Transports", func(c *qt.C) {
		// Insert additional transports
		transports := []*Transport{
			{
				ID:   2,
				Name: "Transport One",
			},
			{
				ID:   3,
				Name: "Transport Two",
			},
		}

		for _, t := range transports {
			_, err := transportService.InsertTransport(ctx, t)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert test transport"))
		}

		// Retrieve all transports
		allTransports, err := transportService.GetAllTransports(ctx)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve all transports"))
		c.Assert(len(allTransports) >= 3, qt.Equals, true, qt.Commentf("Expected at least 3 transports in database"))

		// Verify the content of retrieved transports
		foundTransports := make(map[string]bool)
		for _, t := range allTransports {
			foundTransports[t.Name] = true
		}

		c.Assert(foundTransports["Transport One"], qt.Equals, true, qt.Commentf("Transport One not found in results"))
		c.Assert(foundTransports["Transport Two"], qt.Equals, true, qt.Commentf("Transport Two not found in results"))
	})

	c.Run("Get Non-existent Transport", func(c *qt.C) {
		// Try to retrieve a transport with a non-existent ID
		_, err := transportService.GetTransportByID(ctx, 999)
		c.Assert(err, qt.Equals, mongo.ErrNoDocuments, qt.Commentf("Expected no documents error"))
	})

	c.Run("Insert Duplicate Transport ID", func(c *qt.C) {
		transport := &Transport{
			ID:   4,
			Name: "Original Transport",
		}

		// Insert first transport
		_, err := transportService.InsertTransport(ctx, transport)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to insert first transport"))

		// Try to insert transport with same ID
		duplicate := &Transport{
			ID:   4,
			Name: "Duplicate Transport",
		}
		_, err = transportService.InsertTransport(ctx, duplicate)
		c.Assert(err, qt.Not(qt.IsNil), qt.Commentf("Expected error when inserting duplicate transport ID"))
	})
}
