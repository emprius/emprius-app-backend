package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Image represents the schema for the "images" collection.
type Image struct {
	Hash    primitive.Binary `bson:"hash"`
	Name    string           `bson:"name"`
	Content primitive.Binary `bson:"content"`
	Link    string           `bson:"link"`
}

// ImageService provides methods to interact with the "images" collection.
type ImageService struct {
	Collection *mongo.Collection
}

// NewImageService creates a new ImageService.
func NewImageService(db *Database) *ImageService {
	return &ImageService{
		Collection: db.Database.Collection("images"),
	}
}

// InsertImage inserts a new Image document.
func (s *ImageService) InsertImage(ctx context.Context, image *Image) (*mongo.InsertOneResult, error) {
	return s.Collection.InsertOne(ctx, image)
}

// GetImage retrieves an Image by its hash.
func (s *ImageService) GetImage(ctx context.Context, hash []byte) (*Image, error) {
	var image Image
	filter := bson.M{"hash": primitive.Binary{Data: hash}}
	err := s.Collection.FindOne(ctx, filter).Decode(&image)
	if err != nil {
		return nil, err
	}
	return &image, nil
}

// GetAllImages retrieves all Image documents.
func (s *ImageService) GetAllImages(ctx context.Context) ([]*Image, error) {
	cursor, err := s.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var images []*Image
	for cursor.Next(ctx) {
		var image Image
		if err := cursor.Decode(&image); err != nil {
			return nil, err
		}
		images = append(images, &image)
	}
	return images, nil
}
