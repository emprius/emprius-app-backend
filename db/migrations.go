package db

import (
	"context"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
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
	// List of migration functions. Add new migrations here.
	migrations := []func(context.Context, *mongo.Database) error{
		migrateBookingRatings,
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

// migrateBookingRatings migrates legacy rating fields (ratedBy, rating, ratingComment, ratedAt)
// into the new Ratings array structure.
func migrateBookingRatings(ctx context.Context, db *mongo.Database) error {
	collection := db.Collection("bookings")

	// Select documents where the legacy "ratedBy" field exists as an ObjectID (BSON type 7),
	// which excludes documents already migrated (where ratedBy is an array).
	filter := bson.M{
		"ratedBy": bson.M{
			"$exists": true,
			"$type":   7,
		},
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx) // nolint: errcheck

	for cursor.Next(ctx) {
		var booking bson.M
		if err := cursor.Decode(&booking); err != nil {
			return err
		}

		var ratings []bson.M
		if ratedBy, ok := booking["ratedBy"]; ok {
			ratingDoc := bson.M{
				"userId": ratedBy,
			}
			if rating, ok := booking["rating"]; ok {
				ratingDoc["rating"] = rating
			}
			if ratingComment, ok := booking["ratingComment"]; ok {
				ratingDoc["ratingComment"] = ratingComment
			}
			if ratedAt, ok := booking["ratedAt"]; ok {
				ratingDoc["ratedAt"] = ratedAt
			}
			ratings = append(ratings, ratingDoc)
		}

		update := bson.M{
			"$set": bson.M{
				"ratings": ratings,
			},
			"$unset": bson.M{
				"ratedBy":       "",
				"rating":        "",
				"ratingComment": "",
				"ratedAt":       "",
			},
		}

		if _, err := collection.UpdateOne(ctx, bson.M{"_id": booking["_id"]}, update); err != nil {
			return err
		}
	}
	return cursor.Err()
}
