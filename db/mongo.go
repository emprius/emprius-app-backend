package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoValue implements types.Value interface
type MongoValue struct {
	val interface{}
}

func (v MongoValue) Type() types.ValueType {
	switch v.val.(type) {
	case int, int32, int64:
		return types.IntegerValue
	case float32, float64:
		return types.DoubleValue
	case string:
		return types.TextValue
	case bool:
		return types.BooleanValue
	case []byte:
		return types.BlobValue
	default:
		return types.DocumentValue
	}
}

func (v MongoValue) V() interface{} {
	return v.val
}

func (v MongoValue) IsTruthy() bool {
	switch val := v.val.(type) {
	case bool:
		return val
	case int, int32, int64:
		return val != 0
	case float32, float64:
		return val != 0
	case string:
		return val != ""
	case []byte:
		return len(val) > 0
	default:
		return v.val != nil
	}
}

func (v MongoValue) String() string {
	return fmt.Sprintf("%v", v.val)
}

func (v MongoValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.val)
}

func (v MongoValue) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

func NewMongoValue(v interface{}) types.Value {
	return MongoValue{val: v}
}

// Result interface matches Genji's Result interface
type Result interface {
	document.Iterator
	Close() error
}

// Database struct encapsulates MongoDB client and database.
type Database struct {
	Client              *mongo.Client
	Database            *mongo.Database
	ToolService         *ToolService
	ToolCategoryService *ToolCategoryService
	ImageService        *ImageService
	TransportService    *TransportService
	UserService         *UserService
}

// New initializes a new MongoDB connection.
func New(uri string) (*Database, error) {
	// For in-memory testing, use a random database name
	if uri == ":memory:" {
		uri = "mongodb://localhost:27017"
	}

	client, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		return nil, err
	}

	// Use a random database name for isolation in tests
	dbName := RandomDatabaseName()
	db := client.Database(dbName)
	database := &Database{
		Client:   client,
		Database: db,
	}
	database.ToolService = NewToolService(database)
	database.ToolCategoryService = NewToolCategoryService(database)
	database.ImageService = NewImageService(database)
	database.TransportService = NewTransportService(database)
	database.UserService = NewUserService(database)
	return database, nil
}

// Close disconnects the MongoDB client.
func (db *Database) Close(ctx context.Context) error {
	return db.Client.Disconnect(ctx)
}

// CreateTables initializes all collections and indexes.
func (db *Database) CreateTables() error {
	return InitializeDatabase(db)
}

// MongoDocument implements types.Document interface
type MongoDocument struct {
	doc bson.D
}

func (d *MongoDocument) GetByField(name string) (types.Value, error) {
	for _, elem := range d.doc {
		if elem.Key == name {
			return NewMongoValue(elem.Value), nil
		}
	}
	return nil, fmt.Errorf("field not found: %s", name)
}

func (d *MongoDocument) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	for _, elem := range d.doc {
		m[elem.Key] = elem.Value
	}
	return json.Marshal(m)
}

func (d *MongoDocument) Iterate(fn func(field string, value types.Value) error) error {
	for _, elem := range d.doc {
		if err := fn(elem.Key, NewMongoValue(elem.Value)); err != nil {
			return err
		}
	}
	return nil
}

// MongoResult wraps mongo.SingleResult to implement Genji's Document interface
type MongoResult struct {
	result *mongo.SingleResult
	doc    *MongoDocument
}

func (r *MongoResult) GetByField(name string) (types.Value, error) {
	if r.doc == nil {
		var doc bson.D
		if err := r.result.Decode(&doc); err != nil {
			return nil, err
		}
		r.doc = &MongoDocument{doc: doc}
	}
	return r.doc.GetByField(name)
}

func (r *MongoResult) Iterate(fn func(field string, value types.Value) error) error {
	if r.doc == nil {
		var doc bson.D
		if err := r.result.Decode(&doc); err != nil {
			return err
		}
		r.doc = &MongoDocument{doc: doc}
	}
	return r.doc.Iterate(fn)
}

// MongoCursor wraps mongo.Cursor to implement Genji's Iterator interface
type MongoCursor struct {
	cursor *mongo.Cursor
	ctx    context.Context
}

func NewMongoCursor(ctx context.Context, cursor *mongo.Cursor) *MongoCursor {
	return &MongoCursor{
		cursor: cursor,
		ctx:    ctx,
	}
}

func (c *MongoCursor) Iterate(fn func(d types.Document) error) error {
	for c.cursor.Next(c.ctx) {
		var doc bson.D
		if err := c.cursor.Decode(&doc); err != nil {
			return err
		}
		if err := fn(&MongoDocument{doc: doc}); err != nil {
			return err
		}
	}
	return nil
}

func (c *MongoCursor) Close() error {
	return c.cursor.Close(c.ctx)
}

// Query executes a query and returns a cursor.
func (db *Database) Query(query string, args ...interface{}) (Result, error) {
	ctx := context.Background()
	var cursor *mongo.Cursor
	var err error

	// Parse the SQL-like query to determine collection and operation
	if query == "SELECT * FROM tool" {
		cursor, err = db.Database.Collection("tools").Find(ctx, bson.M{})
	} else if query == "SELECT * FROM toolCategory" {
		cursor, err = db.Database.Collection("tool_categories").Find(ctx, bson.M{})
	} else if len(args) > 0 && query == "SELECT * FROM tool WHERE userId = ?" {
		cursor, err = db.Database.Collection("tools").Find(ctx, bson.M{"userId": args[0]})
	} else {
		cursor, err = db.Database.Collection("tools").Find(ctx, bson.M{})
	}

	if err != nil {
		return nil, err
	}

	return NewMongoCursor(ctx, cursor), nil
}

// QueryDocument executes a query and returns a single document.
func (db *Database) QueryDocument(query string, args ...interface{}) (types.Document, error) {
	ctx := context.Background()
	var result *mongo.SingleResult

	// Parse the SQL-like query to determine collection and operation
	if len(args) > 0 && query == "SELECT * FROM tool WHERE id = ?" {
		result = db.Database.Collection("tools").FindOne(ctx, bson.M{"_id": args[0]})
	} else {
		result = db.Database.Collection("tools").FindOne(ctx, bson.M{})
	}

	var doc bson.D
	if err := result.Decode(&doc); err != nil {
		return nil, err
	}
	return &MongoDocument{doc: doc}, nil
}

// Exec executes a command.
func (db *Database) Exec(query string, args ...interface{}) error {
	ctx := context.Background()

	// Handle INSERT operations
	if len(args) > 0 {
		switch v := args[0].(type) {
		case *Tool:
			_, err := db.Database.Collection("tools").InsertOne(ctx, v)
			return err
		case *Image:
			_, err := db.Database.Collection("images").InsertOne(ctx, v)
			return err
		case *User:
			_, err := db.Database.Collection("users").InsertOne(ctx, v)
			return err
		}
	}

	// Handle DELETE operations
	if query == "DELETE FROM tool WHERE id = ?" && len(args) > 0 {
		_, err := db.Database.Collection("tools").DeleteOne(ctx, bson.M{"_id": args[0]})
		return err
	}

	// Handle UPDATE operations
	if strings.HasPrefix(query, "UPDATE tool SET") && len(args) > 11 {
		filter := bson.M{"_id": args[12]} // Last argument is the ID
		update := bson.M{
			"$set": bson.M{
				"title":          args[0],
				"description":    args[1],
				"isAvailable":    args[2],
				"mayBeFree":      args[3],
				"askWithFee":     args[4],
				"cost":           args[5],
				"toolCategory":   args[6],
				"estimatedValue": args[7],
				"height":         args[8],
				"weight":         args[9],
				"images":         args[10],
				"location":       args[11],
			},
		}
		_, err := db.Database.Collection("tools").UpdateOne(ctx, filter, update)
		return err
	}

	return nil
}
