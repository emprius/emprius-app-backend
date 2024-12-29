package db

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
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
