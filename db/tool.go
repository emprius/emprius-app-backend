package db

import (
	"context"
	"math"
	"regexp"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// earthRadius is the approximate radius of the Earth in kilometers.
	earthRadius           = 6371
	microdegreesInDegree  = 1e6
	degreesInMicrodegrees = 1 / microdegreesInDegree
	kilometersInDegree    = 111.0 // approximate
	distanceMargin        = 1.01  // 1% margin to account for floating-point imprecision
)

// DBLocation represents a geographical location in GeoJSON format.
type DBLocation struct {
	Type        string    `bson:"type" json:"-"`
	Coordinates []float64 `bson:"coordinates" json:"-"`
}

// NewLocation creates a new GeoJSON Point location from microdegrees.
func NewLocation(latitudeMicro, longitudeMicro int64) DBLocation {
	return DBLocation{
		Type: "Point",
		Coordinates: []float64{
			float64(longitudeMicro) / microdegreesInDegree, // GeoJSON: [longitude, latitude]
			float64(latitudeMicro) / microdegreesInDegree,
		},
	}
}

// GetCoordinates returns the latitude and longitude in microdegrees.
func (l DBLocation) GetCoordinates() (latitudeMicro, longitudeMicro int64) {
	if len(l.Coordinates) == 2 {
		return int64(l.Coordinates[1] * microdegreesInDegree),
			int64(l.Coordinates[0] * microdegreesInDegree)
	}
	return 0, 0
}

// DateRange represents a range of dates in UNIX timestamp format.
type DateRange struct {
	From uint32 `bson:"from" json:"from"`
	To   uint32 `bson:"to" json:"to"`
}

// Tool represents the schema for the "tools" collection.
type Tool struct {
	ID               int64                `bson:"_id" json:"id"`
	Title            string               `bson:"title" json:"title"`
	Description      string               `bson:"description" json:"description"`
	IsAvailable      bool                 `bson:"isAvailable" json:"isAvailable"`
	MayBeFree        bool                 `bson:"mayBeFree" json:"mayBeFree"`
	AskWithFee       bool                 `bson:"askWithFee" json:"askWithFee"`
	Cost             uint64               `bson:"cost" json:"cost"`
	UserID           primitive.ObjectID   `bson:"userId" json:"userId"`
	ActualUserID     primitive.ObjectID   `bson:"actualUserId,omitempty" json:"actualUserId,omitempty"`
	Images           []Image              `bson:"images" json:"images"`
	TransportOptions []Transport          `bson:"transportOptions" json:"transportOptions"`
	ToolCategory     int                  `bson:"toolCategory" json:"toolCategory"`
	Location         DBLocation           `bson:"location" json:"-"`
	Rating           int32                `bson:"rating" json:"rating"`
	EstimatedValue   uint64               `bson:"estimatedValue" json:"estimatedValue"`
	Height           uint32               `bson:"height" json:"height"`
	Weight           uint32               `bson:"weight" json:"weight"`
	MaxDistance      uint32               `bson:"maxDistance" json:"maxDistance"`
	ReservedDates    []DateRange          `bson:"reservedDates" json:"reservedDates"`
	IsNomadic        bool                 `bson:"isNomadic" json:"isNomadic"`
	Communities      []primitive.ObjectID `bson:"communities,omitempty" json:"communities,omitempty"`
}

// SanitizeString ensures the search term is safe for use in regex.
func SanitizeString(s string) string {
	reg := regexp.MustCompile(`[^0-9\p{L},._\s-]+`)
	return reg.ReplaceAllString(s, "")
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

// InsertTool inserts a new Tool document, ensuring a 2dsphere index.
func (s *ToolService) InsertTool(ctx context.Context, tool *Tool) (*mongo.InsertOneResult, error) {
	_, err := s.Collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "location", Value: "2dsphere"}},
		Options: options.Index(),
	})
	if err != nil {
		return nil, err
	}
	return s.Collection.InsertOne(ctx, tool)
}

// GetToolByID retrieves a Tool by its ID.
func (s *ToolService) GetToolByID(ctx context.Context, id int64) (*Tool, error) {
	var tool Tool
	err := s.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&tool)
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

// SearchToolsByLocation finds tools within a given radius (in meters) from a Location.
func (s *ToolService) SearchToolsByLocation(ctx context.Context, location DBLocation, radiusMeters int) ([]*Tool, error) {
	pipeline := []bson.D{{
		{Key: "$geoNear", Value: bson.D{
			{Key: "near", Value: location},
			{Key: "distanceField", Value: "distance"},
			{Key: "maxDistance", Value: radiusMeters},
			{Key: "spherical", Value: true},
			{Key: "distanceMultiplier", Value: 0.001}, // meters => kilometers
		}},
	}}

	cursor, err := s.Collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var tools []*Tool
	if err := cursor.All(ctx, &tools); err != nil {
		return nil, err
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
		if closeErr := cursor.Close(ctx); closeErr != nil {
			log.Error().Err(closeErr).Msg("Error closing cursor")
		}
	}()

	var tools []*Tool
	if err := cursor.All(ctx, &tools); err != nil {
		return nil, err
	}
	return tools, nil
}

// GetToolsByUserID retrieves all tools owned by a given user.
func (s *ToolService) GetToolsByUserID(ctx context.Context, userID primitive.ObjectID) ([]*Tool, error) {
	cursor, err := s.Collection.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := cursor.Close(ctx); closeErr != nil {
			log.Error().Err(closeErr).Msg("Error closing cursor")
		}
	}()

	var tools []*Tool
	if err := cursor.All(ctx, &tools); err != nil {
		return nil, err
	}
	return tools, nil
}

// GetToolsByCommunityID retrieves all tools shared within a specific community.
func (s *ToolService) GetToolsByCommunityID(ctx context.Context, communityID primitive.ObjectID) ([]*Tool, error) {
	cursor, err := s.Collection.Find(ctx, bson.M{"communities": communityID})
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := cursor.Close(ctx); closeErr != nil {
			log.Error().Err(closeErr).Msg("Error closing cursor")
		}
	}()

	var tools []*Tool
	if err := cursor.All(ctx, &tools); err != nil {
		return nil, err
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

// SearchToolsOptions represents the criteria for searching tools.
type SearchToolsOptions struct {
	SearchTerm       string
	Categories       []int
	MayBeFree        *bool
	MaxCost          *uint64
	Distance         int
	Location         *DBLocation
	TransportOptions []int
	CommunityID      *primitive.ObjectID
	UserID           *primitive.ObjectID // User ID for community membership filtering
}

// SearchTools finds tools by title, description, categories, cost, distance, etc.
func (s *ToolService) SearchTools(ctx context.Context, opts SearchToolsOptions) ([]*Tool, error) {
	filter := bson.D{}

	// Title and Description Search (Case-Insensitive, Partial Word Matching)
	if opts.SearchTerm != "" {
		term := "(?i).*" + regexp.QuoteMeta(SanitizeString(opts.SearchTerm)) + ".*"
		regex := primitive.Regex{Pattern: term, Options: "i"} // Case insensitive search

		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "title", Value: regex}},
			bson.D{{Key: "description", Value: regex}},
		}})
	}

	// Category Filter
	if len(opts.Categories) > 0 {
		filter = append(filter, bson.E{Key: "toolCategory", Value: bson.D{{Key: "$in", Value: opts.Categories}}})
	}

	// MayBeFree Filter
	if opts.MayBeFree != nil {
		filter = append(filter, bson.E{Key: "mayBeFree", Value: *opts.MayBeFree})
	}

	// MaxCost Filter
	if opts.MaxCost != nil && *opts.MaxCost > 0 {
		filter = append(filter, bson.E{Key: "cost", Value: bson.D{{Key: "$lte", Value: *opts.MaxCost}}})
	}

	// Transport Options Filter
	if len(opts.TransportOptions) > 0 {
		filter = append(filter, bson.E{Key: "transportOptions.id", Value: bson.D{{Key: "$in", Value: opts.TransportOptions}}})
	}

	// Only Available Tools
	filter = append(filter, bson.E{Key: "isAvailable", Value: true})

	// Community Filter
	if opts.CommunityID != nil {
		filter = append(filter, bson.E{Key: "communities", Value: *opts.CommunityID})
	}

	// Distance + Location Handling using $geoNear
	if opts.Distance > 0 && opts.Location != nil {
		pipeline := mongo.Pipeline{
			bson.D{
				{Key: "$geoNear", Value: bson.D{
					{Key: "near", Value: opts.Location},
					{Key: "distanceField", Value: "distance"},
					{Key: "maxDistance", Value: float64(opts.Distance)}, // meters
					{Key: "spherical", Value: true},
					{Key: "distanceMultiplier", Value: 0.001}, // meters -> km
					{Key: "query", Value: filter},
				}},
			},
		}

		log.Debug().Interface("pipeline", pipeline).Msg("Executing geoNear pipeline")

		cursor, err := s.Collection.Aggregate(ctx, pipeline)
		if err != nil {
			return nil, err
		}
		defer cursor.Close(ctx) //nolint:errcheck

		var tools []*Tool
		if err := cursor.All(ctx, &tools); err != nil {
			return nil, err
		}
		log.Debug().Int("total_tools", len(tools)).Msg("Search completed with geoNear")

		// Filter tools by community membership if user ID is provided
		if opts.UserID != nil {
			return s.filterToolsByCommunityMembership(ctx, tools, *opts.UserID)
		}

		return tools, nil
	}

	// Otherwise, perform a normal Find query
	log.Debug().Interface("filter", filter).Msg("Executing search with filter")

	cursor, err := s.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var tools []*Tool
	if err := cursor.All(ctx, &tools); err != nil {
		return nil, err
	}
	log.Debug().Int("total_tools", len(tools)).Msg("Search completed")

	// Filter tools by community membership if user ID is provided
	if opts.UserID != nil {
		return s.filterToolsByCommunityMembership(ctx, tools, *opts.UserID)
	}

	return tools, nil
}

// filterToolsByCommunityMembership filters tools based on user's community membership.
// It returns only tools that either:
// 1. Don't belong to any community, or
// 2. Belong to at least one community where the user is a member
func (s *ToolService) filterToolsByCommunityMembership(
	ctx context.Context, tools []*Tool,
	userID primitive.ObjectID,
) ([]*Tool, error) {
	// Get the user to check their communities
	userService := NewUserService(&Database{Database: s.Collection.Database()})
	userCommunities, err := userService.GetUserCommunities(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Create a map of community IDs the user is a member of for quick lookup
	userCommunityMap := make(map[string]bool)
	for _, community := range userCommunities {
		userCommunityMap[community.ID.Hex()] = true
	}

	// Filter the tools
	var filteredTools []*Tool
	for _, tool := range tools {
		// If the tool has no communities, include it
		if len(tool.Communities) == 0 {
			filteredTools = append(filteredTools, tool)
			continue
		}

		// Check if the user is a member of at least one of the tool's communities
		userIsMember := false
		for _, communityID := range tool.Communities {
			if userCommunityMap[communityID.Hex()] {
				userIsMember = true
				break
			}
		}

		// Include the tool only if the user is a member of at least one of its communities
		if userIsMember {
			filteredTools = append(filteredTools, tool)
		}
	}

	return filteredTools, nil
}

// CountTools returns the total number of tool documents.
func (s *ToolService) CountTools(ctx context.Context) (int64, error) {
	return s.Collection.CountDocuments(ctx, bson.M{})
}

// UpdateToolCommunities updates the communities a tool belongs to
func (s *ToolService) UpdateToolCommunities(ctx context.Context, toolID int64, communityIDs []primitive.ObjectID) error {
	// Update the tool's communities field
	_, err := s.Collection.UpdateOne(
		ctx,
		bson.M{"_id": toolID},
		bson.M{"$set": bson.M{"communities": communityIDs}},
	)
	return err
}

// WithinCircumference checks if two GeoJSON points are within a given radius (meters).
// This uses the Haversine formula and a small distanceMargin to account for rounding.
func WithinCircumference(point1, point2 DBLocation, distance int) bool {
	if len(point1.Coordinates) != 2 || len(point2.Coordinates) != 2 {
		return false
	}

	// GeoJSON: [longitude, latitude]
	long1, lat1 := point1.Coordinates[0], point1.Coordinates[1]
	long2, lat2 := point2.Coordinates[0], point2.Coordinates[1]

	// Convert degrees to radians
	lat1Rad := lat1 * (math.Pi / 180)
	long1Rad := long1 * (math.Pi / 180)
	lat2Rad := lat2 * (math.Pi / 180)
	long2Rad := long2 * (math.Pi / 180)

	// Haversine formula
	dLat := lat2Rad - lat1Rad
	dLong := long2Rad - long1Rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLong/2)*math.Sin(dLong/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distanceMeters := earthRadius * c * 1000

	within := distanceMeters <= float64(distance)*distanceMargin
	log.Debug().
		Float64("lat1", lat1).
		Float64("long1", long1).
		Float64("lat2", lat2).
		Float64("long2", long2).
		Float64("distance_meters", distanceMeters).
		Int("radius_meters", distance).
		Bool("within_radius", within).
		Msg("distance calculation")

	return within
}
