package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Location represents a geographical location in microdegrees.
type Location struct {
	Latitude  int64 `bson:"latitude"`
	Longitude int64 `bson:"longitude"`
}

// DateRange represents a range of dates using UNIX time format.
type DateRange struct {
	From uint32 `bson:"from"`
	To   uint32 `bson:"to"`
}

// Tool represents the schema for the "tools" collection.
type Tool struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	Title            string             `bson:"title"`
	Description      string             `bson:"description"`
	IsAvailable      bool               `bson:"isAvailable"`
	MayBeFree        bool               `bson:"mayBeFree"`
	AskWithFee       bool               `bson:"askWithFee"`
	Cost             uint64             `bson:"cost"`
	UserID           string             `bson:"userId"`
	Images           []Image            `bson:"images"`
	TransportOptions []Transport        `bson:"transportOptions"`
	ToolCategory     int                `bson:"toolCategory"`
	Location         Location           `bson:"location"`
	Rating           int32              `bson:"rating"`
	EstimatedValue   uint64             `bson:"estimatedValue"`
	Height           uint32             `bson:"height"`
	Weight           uint32             `bson:"weight"`
	ReservedDates    []DateRange        `bson:"reservedDates"`
}

// ToolService provides methods to interact with the "tools" collection.
type ToolService struct {
	Collection *mongo.Collection
}

// NewToolService creates a new ToolService.
func NewToolService(db *Database) *ToolService {
	return &ToolService{
		Collection: db.Database.Collection("tools"),
	}
}

// InsertTool inserts a new Tool document.
func (s *ToolService) InsertTool(ctx context.Context, tool *Tool) (*mongo.InsertOneResult, error) {
	return s.Collection.InsertOne(ctx, tool)
}

// GetToolByID retrieves a Tool by its ID.
func (s *ToolService) GetToolByID(ctx context.Context, id primitive.ObjectID) (*Tool, error) {
	var tool Tool
	filter := bson.M{"_id": id}
	err := s.Collection.FindOne(ctx, filter).Decode(&tool)
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

// UpdateTool updates a Tool document by ID.
func (s *ToolService) UpdateTool(ctx context.Context, id primitive.ObjectID, update bson.M) (*mongo.UpdateResult, error) {
	filter := bson.M{"_id": id}
	return s.Collection.UpdateOne(ctx, filter, bson.M{"$set": update})
}

// SearchToolsByLocation retrieves tools within a specified radius (in meters) from a given location.
func (s *ToolService) SearchToolsByLocation(ctx context.Context, location Location, radiusMeters int) ([]*Tool, error) {
	// Geo-query logic, assuming a flat Earth approximation.
	cursor, err := s.Collection.Find(ctx, bson.M{
		"location.latitude": bson.M{
			"$gte": location.Latitude - int64(radiusMeters),
			"$lte": location.Latitude + int64(radiusMeters),
		},
		"location.longitude": bson.M{
			"$gte": location.Longitude - int64(radiusMeters),
			"$lte": location.Longitude + int64(radiusMeters),
		},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tools []*Tool
	for cursor.Next(ctx) {
		var tool Tool
		if err := cursor.Decode(&tool); err != nil {
			return nil, err
		}
		tools = append(tools, &tool)
	}
	return tools, nil
}

// GetAllTools retrieves all Tool documents.
func (s *ToolService) GetAllTools(ctx context.Context) ([]*Tool, error) {
	cursor, err := s.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tools []*Tool
	for cursor.Next(ctx) {
		var tool Tool
		if err := cursor.Decode(&tool); err != nil {
			return nil, err
		}
		tools = append(tools, &tool)
	}
	return tools, nil
}
