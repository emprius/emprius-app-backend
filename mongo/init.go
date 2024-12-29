package db

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Default categories and transports for initialization
var defaultToolCategories = []string{
	"other",
	"transport",
	"construction",
	"agriculture",
	"communication",
}

var defaultTransports = []string{
	"Car",
	"Van",
	"Truck",
}

// InitializeDatabase sets up the database with default data and ensures collections are ready for use.
func InitializeDatabase(db *Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create unique indexes
	if err := createUniqueIndexes(db, ctx); err != nil {
		return err
	}

	// Initialize Tool Categories
	toolCategoryService := NewToolCategoryService(db)
	err := toolCategoryService.InitializeDefaultCategories(ctx, defaultToolCategories)
	if err != nil {
		log.Printf("Error initializing tool categories: %v\n", err)
		return err
	}
	log.Println("Tool categories initialized.")

	// Initialize Transports
	transportService := NewTransportService(db)
	for i, name := range defaultTransports {
		_, err := transportService.InsertTransport(ctx, &Transport{
			ID:   int64(i + 1),
			Name: name,
		})
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			log.Printf("Error initializing transports: %v\n", err)
			return err
		}
	}
	log.Println("Transports initialized.")

	return nil
}

// createUniqueIndexes creates all required unique indexes for collections
func createUniqueIndexes(db *Database, ctx context.Context) error {
	// User collection indexes
	userColl := db.Database.Collection("users")
	_, err := userColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		log.Printf("Error creating user indexes: %v\n", err)
		return err
	}

	// Image collection indexes
	imageColl := db.Database.Collection("images")
	_, err = imageColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "hash", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		log.Printf("Error creating image indexes: %v\n", err)
		return err
	}

	// Transport collection indexes
	transportColl := db.Database.Collection("transports")
	_, err = transportColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		log.Printf("Error creating transport indexes: %v\n", err)
		return err
	}

	// Tool category collection indexes
	toolCategoryColl := db.Database.Collection("tool_categories")
	_, err = toolCategoryColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		log.Printf("Error creating tool category indexes: %v\n", err)
		return err
	}

	// Tool collection indexes
	toolColl := db.Database.Collection("tools")
	_, err = toolColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		log.Printf("Error creating tool indexes: %v\n", err)
		return err
	}

	log.Println("All indexes created successfully")
	return nil
}
