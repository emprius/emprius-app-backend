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
	"livestock",
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

	// Run migrations first.
	if err := RunMigrations(ctx, db.Database); err != nil {
		return err
	}

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
	_, err = toolColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys: bson.D{
				{Key: "title", Value: "text"},
				{Key: "description", Value: "text"},
			},
			Options: options.Index().SetDefaultLanguage("none").SetLanguageOverride("none"),
		},
		{
			Keys: bson.D{
				{Key: "toolCategory", Value: 1},
				{Key: "cost", Value: 1},
				{Key: "mayBeFree", Value: 1},
			},
			Options: options.Index(),
		},
		{
			Keys: bson.D{
				{Key: "transportOptions.id", Value: 1},
			},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "location", Value: "2dsphere"}},
			Options: options.Index(),
		},
	})
	if err != nil {
		log.Printf("Error creating tool indexes: %v\n", err)
		return err
	}

	// Invite code collection indexes
	inviteCodeColl := db.Database.Collection("invite_codes")
	_, err = inviteCodeColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "code", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "ownerId", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "usedById", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "createdOn", Value: 1}},
			Options: options.Index(),
		},
	})
	if err != nil {
		log.Printf("Error creating invite code indexes: %v\n", err)
		return err
	}

	// Community collection indexes
	communityColl := db.Database.Collection("communities")
	_, err = communityColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "createdAt", Value: 1}},
			Options: options.Index(),
		},
	})
	if err != nil {
		log.Printf("Error creating community indexes: %v\n", err)
		return err
	}

	// Community member collection indexes
	communityMemberColl := db.Database.Collection("community_members")
	_, err = communityMemberColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "communityId", Value: 1},
				{Key: "userId", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "communityId", Value: 1}},
			Options: options.Index(),
		},
	})
	if err != nil {
		log.Printf("Error creating community member indexes: %v\n", err)
		return err
	}

	// Community invite collection indexes
	communityInviteColl := db.Database.Collection("community_invites")
	_, err = communityInviteColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "communityId", Value: 1},
				{Key: "toUserId", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "toUserId", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "fromUserId", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "createdAt", Value: 1}},
			Options: options.Index(),
		},
	})
	if err != nil {
		log.Printf("Error creating community invite indexes: %v\n", err)
		return err
	}

	log.Println("All indexes created successfully")
	return nil
}
