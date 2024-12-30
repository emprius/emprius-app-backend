package db

import (
	"context"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ToolCategory represents the schema for the "tool_categories" collection.
type ToolCategory struct {
	ID   int    `bson:"id"`
	Name string `bson:"name"`
}

// ToolCategoryService provides methods to interact with the "tool_categories" collection.
type ToolCategoryService struct {
	Collection *mongo.Collection
}

// NewToolCategoryService creates a new ToolCategoryService.
func NewToolCategoryService(db *Database) *ToolCategoryService {
	return &ToolCategoryService{
		Collection: db.Database.Collection("tool_categories"),
	}
}

// InsertToolCategory inserts a new ToolCategory document.
func (s *ToolCategoryService) InsertToolCategory(ctx context.Context, category *ToolCategory) (*mongo.InsertOneResult, error) {
	return s.Collection.InsertOne(ctx, category)
}

// GetToolCategoryByID retrieves a ToolCategory by its ID.
func (s *ToolCategoryService) GetToolCategoryByID(ctx context.Context, id int) (*ToolCategory, error) {
	var category ToolCategory
	filter := bson.M{"id": id}
	err := s.Collection.FindOne(ctx, filter).Decode(&category)
	if err != nil {
		return nil, err
	}
	return &category, nil
}

// GetAllToolCategories retrieves all ToolCategory documents.
func (s *ToolCategoryService) GetAllToolCategories(ctx context.Context) ([]*ToolCategory, error) {
	cursor, err := s.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var categories []*ToolCategory
	for cursor.Next(ctx) {
		var category ToolCategory
		if err := cursor.Decode(&category); err != nil {
			return nil, err
		}
		categories = append(categories, &category)
	}
	return categories, nil
}

// InitializeDefaultCategories ensures the default tool categories exist in the collection.
func (s *ToolCategoryService) InitializeDefaultCategories(ctx context.Context, defaultCategories []string) error {
	for i, name := range defaultCategories {
		_, err := s.Collection.UpdateOne(
			ctx,
			bson.M{"id": i + 1},
			bson.M{"$setOnInsert": bson.M{"id": i + 1, "name": name}},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			return err
		}
	}
	return nil
}
