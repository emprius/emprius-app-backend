package db

import (
	"context"
	"math"
	"regexp"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	// earthRadius is the radius of the earth in kilometers.
	earthRadius           = 6371
	microdegreesInDegree  = 1e6
	degreesInMicrodegrees = 1 / microdegreesInDegree
	kilometersInDegree    = 111.0 // Approximate conversion factor
)

// Location represents a geographical location in microdegrees.
type Location struct {
	Latitude  int64 `bson:"latitude" json:"latitude"`
	Longitude int64 `bson:"longitude" json:"longitude"`
}

// DateRange represents a range of dates using UNIX time format.
type DateRange struct {
	From uint32 `bson:"from" json:"from"`
	To   uint32 `bson:"to" json:"to"`
}

// Tool represents the schema for the "tools" collection.
type Tool struct {
	ID               int64              `bson:"_id" json:"id"`
	Title            string             `bson:"title" json:"title"`
	Description      string             `bson:"description" json:"description"`
	IsAvailable      bool               `bson:"isAvailable" json:"isAvailable"`
	MayBeFree        bool               `bson:"mayBeFree" json:"mayBeFree"`
	AskWithFee       bool               `bson:"askWithFee" json:"askWithFee"`
	Cost             uint64             `bson:"cost" json:"cost"`
	UserID           primitive.ObjectID `bson:"userId" json:"userId"`
	Images           []Image            `bson:"images" json:"images"`
	TransportOptions []Transport        `bson:"transportOptions" json:"transportOptions"`
	ToolCategory     int                `bson:"toolCategory" json:"toolCategory"`
	Location         Location           `bson:"location" json:"location"`
	Rating           int32              `bson:"rating" json:"rating"`
	EstimatedValue   uint64             `bson:"estimatedValue" json:"estimatedValue"`
	Height           uint32             `bson:"height" json:"height"`
	Weight           uint32             `bson:"weight" json:"weight"`
	ReservedDates    []DateRange        `bson:"reservedDates" json:"reservedDates"`
}

// SanitizeString removes all non-alphanumeric characters from a string, except for commas, dots, minus signs, and underscores.
func SanitizeString(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9,._\s-]+`)
	sanitized := reg.ReplaceAllString(s, "")
	return sanitized
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
func (s *ToolService) GetToolByID(ctx context.Context, id int64) (*Tool, error) {
	var tool Tool
	filter := bson.M{"_id": id}
	err := s.Collection.FindOne(ctx, filter).Decode(&tool)
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

// UpdateTool updates a Tool document by ID.
func (s *ToolService) UpdateTool(ctx context.Context, id int64, update bson.M) (*mongo.UpdateResult, error) {
	filter := bson.M{"_id": id}
	return s.Collection.UpdateOne(ctx, filter, bson.M{"$set": update})
}

// SearchToolsByLocation retrieves tools within a specified radius (in meters) from a given location.
func (s *ToolService) SearchToolsByLocation(ctx context.Context, location Location, radiusMeters int) ([]*Tool, error) {
	cursor, err := s.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var tools []*Tool
	for cursor.Next(ctx) {
		var tool Tool
		if err := cursor.Decode(&tool); err != nil {
			return nil, err
		}
		if WithinCircumference(tool.Location, location, radiusMeters) {
			tools = append(tools, &tool)
		}
	}
	return tools, nil
}

// GetAllTools retrieves all Tool documents.
func (s *ToolService) GetAllTools(ctx context.Context) ([]*Tool, error) {
	cursor, err := s.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

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

// GetToolsByUserID retrieves all tools owned by a specific user.
func (s *ToolService) GetToolsByUserID(ctx context.Context, userID primitive.ObjectID) ([]*Tool, error) {
	cursor, err := s.Collection.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

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

// UpdateToolFields updates specific fields of a tool.
func (s *ToolService) UpdateToolFields(ctx context.Context, id int64, updates map[string]interface{}) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": updates}
	log.Debug().
		Int64("id", id).
		Interface("updates", updates).
		Msg("updating tool fields")
	result, err := s.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	log.Debug().
		Int64("id", id).
		Int64("matchedCount", result.MatchedCount).
		Int64("modifiedCount", result.ModifiedCount).
		Msg("tool update result")
	return nil
}

// SearchToolsOptions represents the search criteria for tools.
type SearchToolsOptions struct {
	SearchTerm       string
	Categories       []int
	MayBeFree        *bool
	MaxCost          *uint64
	Distance         int
	Location         *Location
	TransportOptions []int
}

// SearchTools searches for tools based on various criteria.
func (s *ToolService) SearchTools(ctx context.Context, opts SearchToolsOptions) ([]*Tool, error) {
	// Build the filter
	filter := bson.M{}

	// Add title search if search term is provided
	if opts.SearchTerm != "" {
		sanitizedTerm := SanitizeString(opts.SearchTerm)
		log.Debug().
			Str("searchTerm", opts.SearchTerm).
			Str("sanitizedTerm", sanitizedTerm).
			Msg("building search filter")

		// Simple substring match
		filter["title"] = bson.M{
			"$regex": sanitizedTerm,
		}
		log.Debug().
			Interface("filter", filter).
			Msg("built search filter")
	}

	// Add category filter
	if len(opts.Categories) > 0 {
		filter["toolCategory"] = bson.M{"$in": opts.Categories}
	}

	// Add mayBeFree filter
	if opts.MayBeFree != nil {
		filter["mayBeFree"] = *opts.MayBeFree
	}

	// Add maxCost filter (only if greater than 0)
	if opts.MaxCost != nil && *opts.MaxCost > 0 {
		filter["cost"] = bson.M{"$lte": *opts.MaxCost}
	}

	// Add transport options filter
	if len(opts.TransportOptions) > 0 {
		filter["transportOptions.id"] = bson.M{"$in": opts.TransportOptions}
	}

	// Execute the query
	log.Debug().Interface("filter", filter).Msg("executing search with filter")
	cursor, err := s.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Warn().Msg("could not close db cursor")
		}
	}()

	// Decode results
	var tools []*Tool
	for cursor.Next(ctx) {
		var tool Tool
		if err := cursor.Decode(&tool); err != nil {
			return nil, err
		}
		log.Debug().
			Int64("id", tool.ID).
			Str("title", tool.Title).
			Msg("found matching tool")

		// Check distance if required (this can't be done in MongoDB query)
		if opts.Distance > 0 && opts.Location != nil {
			if !WithinCircumference(tool.Location, *opts.Location, opts.Distance) {
				continue
			}
		}

		tools = append(tools, &tool)
	}

	log.Debug().Int("total_tools", len(tools)).Msg("search completed")
	return tools, nil
}

// CountTools returns the total number of tools.
func (s *ToolService) CountTools(ctx context.Context) (int64, error) {
	return s.Collection.CountDocuments(ctx, bson.M{})
}

// WithinCircumference calculates if two Location points are within the same geographic circumference
// of diameter equal to the specified distance.
// The function takes in three arguments:
// - location1: a Location struct with latitude and longitude in microdegrees (1e-6 degrees)
// - location2: a Location struct with latitude and longitude in microdegrees (1e-6 degrees)
// - distance: an integer representing the diameter of the circumference in meters
// The function returns a boolean value indicating whether the two Location points are within the same
// circumference of diameter equal to the distance.
func WithinCircumference(point1, point2 Location, distance int) bool {
	// Convert the latitude and longitude of both points to radians
	lat1 := float64(point1.Latitude) / microdegreesInDegree * (math.Pi / 180)
	long1 := float64(point1.Longitude) / microdegreesInDegree * (math.Pi / 180)
	lat2 := float64(point2.Latitude) / microdegreesInDegree * (math.Pi / 180)
	long2 := float64(point2.Longitude) / microdegreesInDegree * (math.Pi / 180)

	// Calculate the distance between the two points using the Haversine formula
	a := math.Sin((lat2-lat1)/2)*math.Sin((lat2-lat1)/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin((long2-long1)/2)*math.Sin((long2-long1)/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	d := earthRadius * c * 1000 // distance in meters

	// Check if the distance between the two points is within the given circumference
	return d <= float64(distance)
}

// NewLocation creates a new location that is a certain distance (in kilometers)
// north and east from a starting location.
// The distance is approximated using a simple flat Earth model, which is reasonably
// accurate for small distances (up to a few hundred kilometers).
func NewLocation(start Location, distanceNorthKm, distanceEastKm float64) Location {
	latitudeChange := distanceNorthKm / kilometersInDegree
	longitudeChange := distanceEastKm / (kilometersInDegree * math.Cos(float64(start.Latitude)*degreesInMicrodegrees))
	return Location{
		Latitude:  start.Latitude + int64(latitudeChange*microdegreesInDegree),
		Longitude: start.Longitude + int64(longitudeChange*microdegreesInDegree),
	}
}
