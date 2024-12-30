package db

import (
	"context"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Transport represents the schema for the "transports" collection.
type Transport struct {
	ID   int64  `bson:"id"`
	Name string `bson:"name"`
}

// TransportService provides methods to interact with the "transports" collection.
type TransportService struct {
	Collection *mongo.Collection
}

// NewTransportService creates a new TransportService.
func NewTransportService(db *Database) *TransportService {
	return &TransportService{
		Collection: db.Database.Collection("transports"),
	}
}

// InsertTransport inserts a new Transport document.
func (s *TransportService) InsertTransport(ctx context.Context, transport *Transport) (*mongo.InsertOneResult, error) {
	return s.Collection.InsertOne(ctx, transport)
}

// GetTransportByID retrieves a Transport by its ID.
func (s *TransportService) GetTransportByID(ctx context.Context, id int64) (*Transport, error) {
	var transport Transport
	filter := bson.M{"id": id}
	err := s.Collection.FindOne(ctx, filter).Decode(&transport)
	if err != nil {
		return nil, err
	}
	return &transport, nil
}

// GetAllTransports retrieves all Transport documents.
func (s *TransportService) GetAllTransports(ctx context.Context) ([]*Transport, error) {
	cursor, err := s.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("Error closing cursor")
		}
	}()

	var transports []*Transport
	for cursor.Next(ctx) {
		var transport Transport
		if err := cursor.Decode(&transport); err != nil {
			return nil, err
		}
		transports = append(transports, &transport)
	}
	return transports, nil
}
