package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// DatabaseName is the name of the MongoDB database.
	DatabaseName = "emprius-backend"
)

// Database struct encapsulates MongoDB client and database.
type Database struct {
	Client              *mongo.Client
	Database            *mongo.Database
	ToolService         *ToolService
	ToolCategoryService *ToolCategoryService
	ImageService        *ImageService
	TransportService    *TransportService
	UserService         *UserService
	BookingService      *BookingService
	InviteCodeService   *InviteCodeService
	CommunityService    *CommunityService
}

// New initializes a new MongoDB connection.
func New(uri string, secret ...string) (*Database, error) {
	// For in-memory testing, use a random database name
	if uri == ":memory:" {
		uri = "mongodb://localhost:27017"
	}

	// Set the location salt if provided
	if len(secret) > 0 && secret[0] != "" {
		SetLocationSalt(secret[0])
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	db := client.Database(DatabaseName)
	database := &Database{
		Client:   client,
		Database: db,
	}
	database.ToolService = NewToolService(database)
	database.ToolCategoryService = NewToolCategoryService(database)
	database.ImageService = NewImageService(database)
	database.TransportService = NewTransportService(database)
	database.UserService = NewUserService(database)
	database.BookingService = NewBookingService(database.Database)
	database.InviteCodeService = NewInviteCodeService(database)
	database.CommunityService = NewCommunityService(database)
	return database, nil
}

// Close disconnects the MongoDB client.
func (db *Database) Close(ctx context.Context) error {
	return db.Client.Disconnect(ctx)
}

// CreateTables initializes all collections and indexes.
func (db *Database) CreateTables() error {
	return InitializeDatabase(db)
}
