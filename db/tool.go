package db

import (
	"encoding/json"
	"math"

	"github.com/rs/zerolog/log"
)

const (
	// earthRadius is the radius of the earth in kilometers.
	earthRadius           = 6371
	microdegreesInDegree  = 1e6
	degreesInMicrodegrees = 1 / microdegreesInDegree
	kilometersInDegree    = 111.0 // Approximate conversion factor
)

var initialToolCategories = []string{
	"other",
	"transport",
	"construction",
	"agriculture",
	"communication",
}

// ToolCategory contains the categories for the tools.
// The ID zero is reserved for non existing category.
type ToolCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func createToolCategoryTables(db *Database) error {
	log.Info().Msg("creating tool category tables")
	if err := db.Exec(`
	CREATE TABLE IF NOT EXISTS toolCategory (
		id    INTEGER PRIMARY KEY,
		name  TEXT NOT NULL UNIQUE
	)`); err != nil {
		return err
	}
	if _, err := db.QueryDocument("SELECT id FROM toolCategory"); err == nil {
		return err
	}
	for i, category := range initialToolCategories {
		if err := db.Exec(`INSERT INTO toolCategory (id,name) VALUES (?,?)`, i+1, category); err != nil {
			return err
		}
	}
	return nil
}

// Tool is a tool that can be borrowed by a user.
type Tool struct {
	ID               int64       `json:"id"`
	Title            string      `json:"title"`
	Description      string      `json:"description"`
	IsAvailable      bool        `json:"isAvailable" genji:"isAvailable"`
	MayBeFree        bool        `json:"mayBeFree" genji:"mayBeFree"`
	AskWithFee       bool        `json:"askWithFee" genji:"askWithFee"`
	Cost             uint64      `json:"cost"`
	UserID           string      `json:"userId" genji:"userId"`
	Images           []Image     `json:"images"`
	TransportOptions []Transport `json:"transportOptions" genji:"transportOptions"`
	ToolCategory     int         `json:"toolCategory" genji:"toolCategory"`
	Location         Location    `json:"location"`
	Rating           int32       `json:"rating"`
	EstimatedValue   uint64      `json:"estimatedValue" genji:"estimatedValue"`
	Height           uint32      `json:"height"`
	Weight           uint32      `json:"weight"`
	ReservedDates    []DateRange `json:"reservedDates" genji:"reservedDates"`
}

func (t Tool) String() string {
	data, err := json.Marshal(&t)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func createToolTables(db *Database) error {
	log.Info().Msg("creating tool tables")
	return db.Exec(`
	CREATE TABLE IF NOT EXISTS tool (
		id              INTEGER PRIMARY KEY,
		title           TEXT NOT NULL,
		description     TEXT NOT NULL,
		isAvailable     BOOLEAN NOT NULL,
		mayBeFree       BOOLEAN NOT NULL,
		askWithFee      BOOLEAN NOT NULL,
		cost            INTEGER NOT NULL,
		userId          TEXT NOT NULL,
		toolCategory    INTEGER NOT NULL,
		rating          INTEGER NOT NULL,
		estimatedValue  INTEGER NOT NULL,
		height          INTEGER,
		weight          INTEGER,
		images ARRAY,
		transportOptions ARRAY,
		reservedDates ARRAY,
		location  (
			latitude INTEGER NOT NULL,
			longitude INTEGER NOT NULL
		),
		CHECK(rating >= 0 AND rating <= 100)
	)`)
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

// NewLocation creates a new location that is a certain distance (in kilometers) north and east from a starting location.
// The distance is approximated using a simple flat Earth model, which is reasonably accurate for small distances (up to a few hundred kilometers).
func NewLocation(start Location, distanceNorthKm, distanceEastKm float64) Location {
	latitudeChange := distanceNorthKm / kilometersInDegree
	longitudeChange := distanceEastKm / (kilometersInDegree * math.Cos(float64(start.Latitude)*degreesInMicrodegrees))
	return Location{
		Latitude:  start.Latitude + int64(latitudeChange*microdegreesInDegree),
		Longitude: start.Longitude + int64(longitudeChange*microdegreesInDegree),
	}
}
