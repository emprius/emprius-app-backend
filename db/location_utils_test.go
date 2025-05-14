package db

import (
	"math"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGenerateObfuscatedLocation(t *testing.T) {
	// Test case 1: Basic functionality
	t.Run("Basic Functionality", func(t *testing.T) {
		// Create a test location
		originalLocation := DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.1734, 41.3851}, // Barcelona coordinates
		}
		entityID := "test-entity-id"
		salt := "test-salt"
		radiusMeters := 1000.0

		// Generate obfuscated location
		obfuscatedLocation := GenerateObfuscatedLocation(originalLocation, entityID, salt, radiusMeters)

		// Verify the obfuscated location is different from the original
		if obfuscatedLocation.Coordinates[0] == originalLocation.Coordinates[0] &&
			obfuscatedLocation.Coordinates[1] == originalLocation.Coordinates[1] {
			t.Errorf("Obfuscated location should be different from original location")
		}

		// Verify the obfuscated location is within the specified radius
		distance := calculateDistance(
			originalLocation.Coordinates[1], originalLocation.Coordinates[0],
			obfuscatedLocation.Coordinates[1], obfuscatedLocation.Coordinates[0],
		)
		if distance > radiusMeters {
			t.Errorf("Obfuscated location should be within %f meters of original location, but was %f meters away",
				radiusMeters, distance)
		}
	})

	// Test case 2: Deterministic output for the same input
	t.Run("Deterministic Output", func(t *testing.T) {
		// Create a test location
		originalLocation := DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.1734, 41.3851}, // Barcelona coordinates
		}
		entityID := "test-entity-id"
		salt := "test-salt"
		radiusMeters := 1000.0

		// Generate obfuscated location twice with the same inputs
		obfuscatedLocation1 := GenerateObfuscatedLocation(originalLocation, entityID, salt, radiusMeters)
		obfuscatedLocation2 := GenerateObfuscatedLocation(originalLocation, entityID, salt, radiusMeters)

		// Verify both obfuscated locations are the same
		if obfuscatedLocation1.Coordinates[0] != obfuscatedLocation2.Coordinates[0] ||
			obfuscatedLocation1.Coordinates[1] != obfuscatedLocation2.Coordinates[1] {
			t.Errorf("Obfuscated locations should be the same for the same inputs")
		}
	})

	// Test case 3: Different outputs for different entity IDs
	t.Run("Different Entity IDs", func(t *testing.T) {
		// Create a test location
		originalLocation := DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.1734, 41.3851}, // Barcelona coordinates
		}
		entityID1 := "test-entity-id-1"
		entityID2 := "test-entity-id-2"
		salt := "test-salt"
		radiusMeters := 1000.0

		// Generate obfuscated locations with different entity IDs
		obfuscatedLocation1 := GenerateObfuscatedLocation(originalLocation, entityID1, salt, radiusMeters)
		obfuscatedLocation2 := GenerateObfuscatedLocation(originalLocation, entityID2, salt, radiusMeters)

		// Verify the obfuscated locations are different
		if obfuscatedLocation1.Coordinates[0] == obfuscatedLocation2.Coordinates[0] &&
			obfuscatedLocation1.Coordinates[1] == obfuscatedLocation2.Coordinates[1] {
			t.Errorf("Obfuscated locations should be different for different entity IDs")
		}
	})

	// Test case 4: Different outputs for different salts
	t.Run("Different Salts", func(t *testing.T) {
		// Create a test location
		originalLocation := DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.1734, 41.3851}, // Barcelona coordinates
		}
		entityID := "test-entity-id"
		salt1 := "test-salt-1"
		salt2 := "test-salt-2"
		radiusMeters := 1000.0

		// Generate obfuscated locations with different salts
		obfuscatedLocation1 := GenerateObfuscatedLocation(originalLocation, entityID, salt1, radiusMeters)
		obfuscatedLocation2 := GenerateObfuscatedLocation(originalLocation, entityID, salt2, radiusMeters)

		// Verify the obfuscated locations are different
		if obfuscatedLocation1.Coordinates[0] == obfuscatedLocation2.Coordinates[0] &&
			obfuscatedLocation1.Coordinates[1] == obfuscatedLocation2.Coordinates[1] {
			t.Errorf("Obfuscated locations should be different for different salts")
		}
	})

	// Test case 5: Different outputs for different radii
	t.Run("Different Radii", func(t *testing.T) {
		// Create a test location
		originalLocation := DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.1734, 41.3851}, // Barcelona coordinates
		}
		entityID := "test-entity-id"
		salt := "test-salt"
		radiusMeters1 := 1000.0
		radiusMeters2 := 2000.0

		// Generate obfuscated locations with different radii
		obfuscatedLocation1 := GenerateObfuscatedLocation(originalLocation, entityID, salt, radiusMeters1)
		obfuscatedLocation2 := GenerateObfuscatedLocation(originalLocation, entityID, salt, radiusMeters2)

		// Verify the obfuscated locations are different
		if obfuscatedLocation1.Coordinates[0] == obfuscatedLocation2.Coordinates[0] &&
			obfuscatedLocation1.Coordinates[1] == obfuscatedLocation2.Coordinates[1] {
			t.Errorf("Obfuscated locations should be different for different radii")
		}

		// Verify both obfuscated locations are within their respective radii
		distance1 := calculateDistance(
			originalLocation.Coordinates[1], originalLocation.Coordinates[0],
			obfuscatedLocation1.Coordinates[1], obfuscatedLocation1.Coordinates[0],
		)
		if distance1 > radiusMeters1 {
			t.Errorf("Obfuscated location 1 should be within %f meters of original location, but was %f meters away",
				radiusMeters1, distance1)
		}

		distance2 := calculateDistance(
			originalLocation.Coordinates[1], originalLocation.Coordinates[0],
			obfuscatedLocation2.Coordinates[1], obfuscatedLocation2.Coordinates[0],
		)
		if distance2 > radiusMeters2 {
			t.Errorf("Obfuscated location 2 should be within %f meters of original location, but was %f meters away",
				radiusMeters2, distance2)
		}
	})
}

func TestObfuscateLocation(t *testing.T) {
	// Test case 1: Basic functionality
	t.Run("Basic Functionality", func(t *testing.T) {
		// Create a test location
		originalLocation := DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.1734, 41.3851}, // Barcelona coordinates
		}
		seedId := primitive.NewObjectID()

		// Generate obfuscated location
		obfuscatedLocation := ObfuscateLocation(originalLocation, seedId)

		// Verify the obfuscated location is different from the original
		if obfuscatedLocation.Coordinates[0] == originalLocation.Coordinates[0] &&
			obfuscatedLocation.Coordinates[1] == originalLocation.Coordinates[1] {
			t.Errorf("Obfuscated location should be different from original location")
		}

		// Verify the obfuscated location is within the default radius
		distance := calculateDistance(
			originalLocation.Coordinates[1], originalLocation.Coordinates[0],
			obfuscatedLocation.Coordinates[1], obfuscatedLocation.Coordinates[0],
		)
		if distance > DefaultObfuscationRadiusMeters {
			t.Errorf("Obfuscated location should be within %v meters of original location, but was %f meters away",
				DefaultObfuscationRadiusMeters, distance)
		}
	})

	// Test case 2: Deterministic output for the same input
	t.Run("Deterministic Output", func(t *testing.T) {
		// Create a test location
		originalLocation := DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.1734, 41.3851}, // Barcelona coordinates
		}
		seedId := primitive.NewObjectID()

		// Generate obfuscated location twice with the same inputs
		obfuscatedLocation1 := ObfuscateLocation(originalLocation, seedId)
		obfuscatedLocation2 := ObfuscateLocation(originalLocation, seedId)

		// Verify both obfuscated locations are the same
		if obfuscatedLocation1.Coordinates[0] != obfuscatedLocation2.Coordinates[0] ||
			obfuscatedLocation1.Coordinates[1] != obfuscatedLocation2.Coordinates[1] {
			t.Errorf("Obfuscated locations should be the same for the same inputs")
		}
	})

	// Test case 3: Different outputs for different seed IDs
	t.Run("Different Seed IDs", func(t *testing.T) {
		// Create a test location
		originalLocation := DBLocation{
			Type:        "Point",
			Coordinates: []float64{2.1734, 41.3851}, // Barcelona coordinates
		}
		seedId1 := primitive.NewObjectID()
		seedId2 := primitive.NewObjectID()

		// Generate obfuscated locations with different seed IDs
		obfuscatedLocation1 := ObfuscateLocation(originalLocation, seedId1)
		obfuscatedLocation2 := ObfuscateLocation(originalLocation, seedId2)

		// Verify the obfuscated locations are different
		if obfuscatedLocation1.Coordinates[0] == obfuscatedLocation2.Coordinates[0] &&
			obfuscatedLocation1.Coordinates[1] == obfuscatedLocation2.Coordinates[1] {
			t.Errorf("Obfuscated locations should be different for different seed IDs")
		}
	})
}

// Helper function to calculate the distance between two points in meters
// using the Haversine formula
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371000 // Earth radius in meters

	// Convert degrees to radians
	lat1Rad := lat1 * (math.Pi / 180)
	lon1Rad := lon1 * (math.Pi / 180)
	lat2Rad := lat2 * (math.Pi / 180)
	lon2Rad := lon2 * (math.Pi / 180)

	// Haversine formula
	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadius * c

	return distance
}
