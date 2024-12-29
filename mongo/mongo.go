package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Database struct encapsulates MongoDB client and database.
type Database struct {
	Client   *mongo.Client
	Database *mongo.Database
}

// NewDatabase initializes a new MongoDB connection.
func NewDatabase(uri, dbName string) (*Database, error) {
	client, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		return nil, err
	}

	db := client.Database(dbName)
	return &Database{Client: client, Database: db}, nil
}

// Close disconnects the MongoDB client.
func (db *Database) Close(ctx context.Context) error {
	return db.Client.Disconnect(ctx)
}
