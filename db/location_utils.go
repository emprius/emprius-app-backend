package db

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/rand"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// DefaultObfuscationRadiusMeters is the default radius for location obfuscation in meters
	DefaultObfuscationRadiusMeters = 1000
)

// locationSalt is used to add randomness to the obfuscation algorithm
// It will be set during initialization
var locationSalt = "emprius-location-salt-v1" // Default value for backward compatibility

// SetLocationSalt sets the salt used for location obfuscation
func SetLocationSalt(salt string) {
	locationSalt = salt
}

// GenerateObfuscatedLocation generates a deterministically randomized location within the specified radius
func GenerateObfuscatedLocation(location DBLocation, entityID string, salt string, radiusMeters float64) DBLocation {
	// Create a deterministic seed from the entity ID and salt
	hasher := sha256.New()
	hasher.Write([]byte(entityID + salt))
	hashBytes := hasher.Sum(nil)

	// Use the first 8 bytes as a seed for the random number generator
	seed := int64(binary.BigEndian.Uint64(hashBytes[:8]))
	rng := rand.New(rand.NewSource(seed))

	// Generate random angle and distance within the radius
	angle := rng.Float64() * 2 * math.Pi
	// Use square root to ensure uniform distribution across the circle area
	distance := math.Sqrt(rng.Float64()) * radiusMeters

	// Convert distance from meters to degrees
	// Approximate conversion: 1 degree ≈ 111 km at the equator
	latOffset := distance * math.Cos(angle) / (111000)
	// Longitude degrees vary with latitude
	latRadians := location.Coordinates[1] * (math.Pi / 180)
	longOffset := distance * math.Sin(angle) / (111000 * math.Cos(latRadians))

	// Create new obfuscated location
	return DBLocation{
		Type: "Point",
		Coordinates: []float64{
			location.Coordinates[0] + longOffset, // Longitude
			location.Coordinates[1] + latOffset,  // Latitude
		},
	}
}

// ObfuscateLocation generates an obfuscated location for a user
func ObfuscateLocation(location DBLocation, seedId primitive.ObjectID) DBLocation {
	return GenerateObfuscatedLocation(location, seedId.Hex(), locationSalt, DefaultObfuscationRadiusMeters)
}
