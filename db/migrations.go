package db

import (
	"context"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const migrationCollectionName = "migrations"

// Migration record stores applied migration info.
type Migration struct {
	ID        string    `bson:"_id"`
	AppliedAt time.Time `bson:"appliedAt"`
}

// RunMigrations iterates through all migration functions and applies those not yet applied.
func RunMigrations(ctx context.Context, db *mongo.Database) error {
	// List of migration functions. Disable the old one and add the new migration.
	migrations := []func(context.Context, *mongo.Database) error{
		migrateBookingRatingsToNewCollection, // New migration for the new ratings structure.
	}

	for i, migration := range migrations {
		migrationID := "migration_" + strconv.Itoa(i+1)
		applied, err := isMigrationApplied(ctx, db, migrationID)
		if err != nil {
			return err
		}
		if applied {
			log.Debug().Msgf("migration %s already applied", migrationID)
			continue
		}

		log.Info().Msgf("Applying migration %s", migrationID)
		if err := migration(ctx, db); err != nil {
			return err
		}
		if err := markMigrationApplied(ctx, db, migrationID); err != nil {
			return err
		}
	}
	return nil
}

func isMigrationApplied(ctx context.Context, db *mongo.Database, migrationID string) (bool, error) {
	var result Migration
	err := db.Collection(migrationCollectionName).FindOne(ctx, bson.M{"_id": migrationID}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func markMigrationApplied(ctx context.Context, db *mongo.Database, migrationID string) error {
	_, err := db.Collection(migrationCollectionName).InsertOne(ctx, Migration{
		ID:        migrationID,
		AppliedAt: time.Now(),
	})
	return err
}

// migrateBookingRatingsToNewCollection migrates legacy rating fields stored in the bookings collection
// into the new ratings collection. For each booking document that has a legacy rating (legacy fields exist
// as a single rating stored under "ratedBy", etc.), a new rating document is inserted into the ratings collection.
// The new document uses BookingID = booking._id, and determines the counterparty (ToUserID) based on whether
// the legacy ratedBy equals booking.FromUserID or booking.ToUserID. Then the legacy fields are removed.
func migrateBookingRatingsToNewCollection(ctx context.Context, db *mongo.Database) error {
	bookingsColl := db.Collection("bookings")
	ratingsColl := db.Collection("ratings")

	// Select documents where the legacy "ratedBy" field exists as an ObjectID (BSON type 7)
	// (thus not already migrated).
	filter := bson.M{
		"ratedBy": bson.M{
			"$exists": true,
			"$type":   7,
		},
	}

	cursor, err := bookingsColl.Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var booking bson.M
		if err := cursor.Decode(&booking); err != nil {
			return err
		}

		bookingID, ok := booking["_id"].(primitive.ObjectID)
		if !ok {
			continue
		}

		legacyRatedBy, ok := booking["ratedBy"].(primitive.ObjectID)
		if !ok {
			continue
		}

		// Extract rating value from legacy "rating" field.
		legacyRating, ok := booking["rating"]
		if !ok {
			continue
		}
		var ratingValue int
		switch v := legacyRating.(type) {
		case int32:
			ratingValue = int(v)
		case int64:
			ratingValue = int(v)
		case float64:
			ratingValue = int(v)
		default:
			continue
		}

		legacyRatingComment, _ := booking["ratingComment"].(string)
		legacyRatedAt, _ := booking["ratedAt"].(time.Time)

		// Get booking's fromUserId and toUserId.
		fromUserID, _ := booking["fromUserId"].(primitive.ObjectID)
		toUserID, _ := booking["toUserId"].(primitive.ObjectID)
		var cp primitive.ObjectID
		if legacyRatedBy == fromUserID {
			cp = toUserID
		} else {
			cp = fromUserID
		}

		newRating := bson.M{
			"bookingId":     bookingID,
			"fromUserId":    legacyRatedBy,
			"toUserId":      cp,
			"rating":        ratingValue,
			"ratingComment": legacyRatingComment,
			"ratedAt":       legacyRatedAt,
		}
		_, err := ratingsColl.InsertOne(ctx, newRating)
		if err != nil {
			return err
		}

		// Remove the legacy fields from the booking.
		update := bson.M{
			"$unset": bson.M{
				"ratedBy":       "",
				"rating":        "",
				"ratingComment": "",
				"ratedAt":       "",
				"ratings":       "", // In case a legacy ratings array exists.
			},
		}
		_, err = bookingsColl.UpdateOne(ctx, bson.M{"_id": bookingID}, update)
		if err != nil {
			return err
		}
	}
	return cursor.Err()
}
