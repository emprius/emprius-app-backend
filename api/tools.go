package api

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	maxAllowedToolDistance = 200000 // m
)

func (a *API) toolCategories() []db.ToolCategory {
	categories, err := a.database.ToolCategoryService.GetAllToolCategories(context.Background())
	if err != nil {
		panic(err)
	}
	result := make([]db.ToolCategory, len(categories))
	for i, c := range categories {
		result[i] = *c
	}
	return result
}

func (a *API) addTool(t *Tool, userEmail string) (int64, error) {
	// check if images are in database
	images, err := a.imageListFromSlice(t.Images)
	if err != nil {
		return 0, err
	}
	dbImages := []db.Image{}
	for _, i := range images {
		dbImages = append(dbImages, db.Image{
			Hash: i.Hash,
			Name: i.Name,
		})
	}

	if t.Title == "" || t.Description == "" {
		return 0, ErrEmptyTitleOrDescription
	}
	if t.EstimatedValue == 0 {
		return 0, ErrInvalidEstimatedValue
	}
	if t.MayBeFree == nil {
		return 0, ErrMayBeFreeRequired
	}
	if t.AskWithFee == nil {
		return 0, ErrAskWithFeeRequired
	}
	if t.Cost == nil {
		return 0, ErrCostRequired
	}
	user, err := a.userByEmail(userEmail)
	if err != nil {
		return 0, ErrUserNotFound
	}
	if !db.WithinCircumference(user.Location, t.Location, maxAllowedToolDistance) {
		return 0, ErrToolLocationTooFar
	}

	if t.Category < 0 || t.Category >= len(a.toolCategories()) {
		return 0, ErrInvalidToolCategory
	}

	// Validate and convert transport options
	transports, err := a.database.TransportService.GetAllTransports(context.Background())
	if err != nil {
		return 0, ErrInternalServerError
	}
	validTransportIDs := make(map[int64]bool)
	for _, t := range transports {
		validTransportIDs[t.ID] = true
	}

	transportOptions := make([]db.Transport, len(t.TransportOptions))
	for i, id := range t.TransportOptions {
		if !validTransportIDs[int64(id)] {
			return 0, ErrInvalidTransportOption
		}
		transportOptions[i] = db.Transport{ID: int64(id)}
	}

	dbTool := db.Tool{
		ID:               toolID(userEmail, t.Title),
		UserID:           user.ID,
		Title:            db.SanitizeString(t.Title),
		Description:      t.Description,
		IsAvailable:      true,
		MayBeFree:        *t.MayBeFree,
		AskWithFee:       *t.AskWithFee,
		Cost:             *t.Cost,
		ToolCategory:     t.Category,
		Rating:           50,
		EstimatedValue:   t.EstimatedValue,
		Height:           t.Height,
		Weight:           t.Weight,
		Images:           dbImages,
		Location:         t.Location,
		TransportOptions: transportOptions,
	}
	log.Info().Msgf("adding tool to database, title: %s, user: %s, id: %d", t.Title, userEmail, dbTool.ID)

	_, err = a.database.ToolService.InsertTool(context.Background(), &dbTool)
	if err != nil {
		return 0, ErrCouldNotInsertToDatabase
	}

	return dbTool.ID, nil
}

func toolID(ownerID string, title string) int64 {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s-%s", ownerID, title)))
	hash := hasher.Sum(nil)
	// Convert the first 4 bytes of the hash to an absolute int64
	return int64(math.Abs(float64(int64(binary.BigEndian.Uint32(hash[:4])))))
}

func (a *API) tool(id int64) (*db.Tool, error) {
	tool, err := a.database.ToolService.GetToolByID(context.Background(), id)
	if err == mongo.ErrNoDocuments {
		return nil, ErrToolNotFound
	}
	if err != nil {
		return nil, ErrInternalServerError
	}
	return tool, nil
}

func (a *API) toolsByUerID(userEmail string) ([]db.Tool, error) {
	user, err := a.userByEmail(userEmail)
	if err != nil {
		return nil, ErrUserNotFound
	}
	tools, err := a.database.ToolService.GetToolsByUserID(context.Background(), user.ID)
	if err != nil {
		return nil, ErrInternalServerError
	}
	result := make([]db.Tool, len(tools))
	for i, t := range tools {
		result[i] = *t
	}
	return result, nil
}

func (a *API) editTool(id int64, newTool *Tool) error {
	tool, err := a.tool(id)
	if err != nil {
		return err
	}
	if tool == nil {
		return ErrToolNotFound
	}

	if newTool.Title != "" {
		tool.Title = newTool.Title
	}
	if newTool.Description != "" {
		tool.Description = newTool.Description
	}
	if newTool.MayBeFree != nil {
		tool.MayBeFree = *newTool.MayBeFree
	}
	if newTool.AskWithFee != nil {
		tool.AskWithFee = *newTool.AskWithFee
	}
	if newTool.Cost != nil {
		tool.Cost = *newTool.Cost
	}
	if newTool.EstimatedValue != 0 {
		tool.EstimatedValue = newTool.EstimatedValue
	}
	if newTool.Height != 0 {
		tool.Height = newTool.Height
	}
	if newTool.Weight != 0 {
		tool.Weight = newTool.Weight
	}
	if newTool.Category != 0 {
		if newTool.Category < 0 || newTool.Category >= len(a.toolCategories()) {
			return ErrInvalidToolCategory
		}
		tool.ToolCategory = newTool.Category
	}
	if newTool.Location.Latitude != 0 && newTool.Location.Longitude != 0 {
		tool.Location = newTool.Location
	}
	if newTool.IsAvailable != nil {
		tool.IsAvailable = *newTool.IsAvailable
	}
	if len(newTool.Images) > 0 {
		images, err := a.imageListFromSlice(newTool.Images)
		if err != nil {
			return err
		}
		dbImages := []db.Image{}
		for _, i := range images {
			dbImages = append(dbImages, db.Image{
				Hash: i.Hash,
				Name: i.Name,
			})
		}
		tool.Images = dbImages
	}
	if len(newTool.TransportOptions) > 0 {
		// Validate and convert transport options
		transports, err := a.database.TransportService.GetAllTransports(context.Background())
		if err != nil {
			return ErrInternalServerError
		}
		validTransportIDs := make(map[int64]bool)
		for _, t := range transports {
			validTransportIDs[t.ID] = true
		}

		transportOptions := make([]db.Transport, len(newTool.TransportOptions))
		for i, id := range newTool.TransportOptions {
			if !validTransportIDs[int64(id)] {
				return ErrInvalidTransportOption
			}
			transportOptions[i] = db.Transport{ID: int64(id)}
		}
		tool.TransportOptions = transportOptions
	}
	updates := map[string]interface{}{
		"title":            tool.Title,
		"description":      tool.Description,
		"isAvailable":      tool.IsAvailable,
		"mayBeFree":        tool.MayBeFree,
		"askWithFee":       tool.AskWithFee,
		"cost":             tool.Cost,
		"toolCategory":     tool.ToolCategory,
		"estimatedValue":   tool.EstimatedValue,
		"height":           tool.Height,
		"weight":           tool.Weight,
		"images":           tool.Images,
		"location":         tool.Location,
		"transportOptions": tool.TransportOptions,
	}
	err = a.database.ToolService.UpdateToolFields(context.Background(), id, updates)
	if err != nil {
		return ErrInternalServerError
	}
	return nil
}

func (a *API) toolSearch(query *ToolSearch, userLocation *db.Location) ([]db.Tool, error) {
	opts := db.SearchToolsOptions{
		Categories:       query.Categories,
		MayBeFree:        query.MayBeFree,
		MaxCost:          query.MaxCost,
		Distance:         query.Distance,
		Location:         userLocation,
		TransportOptions: query.TransportOptions,
	}
	tools, err := a.database.ToolService.SearchTools(context.Background(), opts)
	if err != nil {
		return nil, ErrInternalServerError
	}
	result := make([]db.Tool, len(tools))
	for i, t := range tools {
		result[i] = *t
	}
	return result, nil
}

func (a *API) deleteTool(id int64) error {
	filter := bson.M{"_id": id}
	_, err := a.database.ToolService.Collection.DeleteOne(context.Background(), filter)
	if err != nil {
		return ErrInternalServerError
	}
	return nil
}

// GET /tools returns tools owned by the user
func (a *API) ownToolsHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized
	}
	tools, err := a.toolsByUerID(r.UserID)
	if err != nil {
		return nil, err
	}
	return &ToolsWrapper{Tools: tools}, nil
}

// GET /tools/:id returns a tool by id
func (a *API) toolHandler(r *Request) (interface{}, error) {
	id, err := strconv.ParseInt(r.Context.URLParam("id"), 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData
	}
	tool, err := a.tool(id)
	if err != nil {
		return nil, err
	}
	return tool, nil
}

// GET /tools/user/:id returns tools owned by the user
func (a *API) userToolsHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized
	}
	tools, err := a.toolsByUerID(r.Context.URLParam("id"))
	if err != nil {
		return nil, err
	}
	return &ToolsWrapper{Tools: tools}, nil
}

// GET /tools/search filters tools
func (a *API) toolSearchHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized
	}

	searchTerm := r.Context.URLParam("searchTerm")
	maxCostStr := r.Context.URLParam("maxCost")
	mayBeFreeStr := r.Context.URLParam("maybeFree")
	availableFromStr := r.Context.URLParam("availableFrom")
	categoriesStr := r.Context.URLParam("categories")

	var maxCost *uint64
	if maxCostStr != "" {
		cost, err := strconv.ParseUint(maxCostStr, 10, 64)
		if err != nil {
			return nil, ErrInvalidRequestBodyData
		}
		maxCost = &cost
	}

	var mayBeFree *bool
	if mayBeFreeStr != "" {
		free, err := strconv.ParseBool(mayBeFreeStr)
		if err != nil {
			return nil, ErrInvalidRequestBodyData
		}
		mayBeFree = &free
	}

	var availableFrom int
	if availableFromStr != "" {
		from, err := strconv.Atoi(availableFromStr)
		if err != nil {
			return nil, ErrInvalidRequestBodyData
		}
		availableFrom = from
	}

	var categories []int
	if categoriesStr != "" {
		// Parse comma-separated list of categories
		catStrings := strings.Split(categoriesStr, ",")
		categories = make([]int, len(catStrings))
		for i, cat := range catStrings {
			val, err := strconv.Atoi(cat)
			if err != nil {
				return nil, ErrInvalidRequestBodyData
			}
			categories[i] = val
		}
	}

	// Parse transport options
	transportOptionsStr := r.Context.URLParam("transportOptions")
	var transportOptions []int
	if transportOptionsStr != "" {
		// Parse comma-separated list of transport options
		transportStrings := strings.Split(transportOptionsStr, ",")
		transportOptions = make([]int, len(transportStrings))
		for i, t := range transportStrings {
			val, err := strconv.Atoi(t)
			if err != nil {
				return nil, ErrInvalidRequestBodyData
			}
			transportOptions[i] = val
		}
	}

	query := ToolSearch{
		Term:             searchTerm,
		Categories:       categories,
		MaxCost:          maxCost,
		MayBeFree:        mayBeFree,
		AvailableFrom:    availableFrom,
		TransportOptions: transportOptions,
	}
	user, err := a.userByEmail(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	tools, err := a.toolSearch(&query, &user.Location)
	if err != nil {
		return nil, err
	}
	return &ToolsWrapper{Tools: tools}, nil
}

// POST /tools adds a new tool
func (a *API) addToolHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized
	}

	t := Tool{}
	if err := json.Unmarshal(r.Data, &t); err != nil {
		return nil, ErrInvalidRequestBodyData
	}
	id, err := a.addTool(&t, r.UserID)
	if err != nil {
		return nil, err
	}
	return &ToolID{ID: id}, nil
}

// DELETE /tools/:id deletes a tool
func (a *API) deleteToolHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized
	}

	id, err := strconv.ParseInt(r.Context.URLParam("id"), 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData
	}
	// check the tool is owned by the user
	tool, err := a.tool(id)
	if err != nil {
		return nil, err
	}
	user, err := a.userByEmail(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if tool.UserID != user.ID {
		return nil, ErrToolNotOwnedByUser
	}
	if err := a.deleteTool(id); err != nil {
		return nil, err
	}
	return nil, nil
}

// PUT /tools/:id edit a tool
func (a *API) editToolHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized
	}

	id, err := strconv.ParseInt(r.Context.URLParam("id"), 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData
	}
	// check the tool is owned by the user
	tool, err := a.tool(id)
	if err != nil {
		return nil, err
	}
	user, err := a.userByEmail(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if tool.UserID != user.ID {
		return nil, ErrToolNotOwnedByUser
	}
	t := Tool{}
	if err := json.Unmarshal(r.Data, &t); err != nil {
		return nil, ErrInvalidRequestBodyData
	}
	if err := a.editTool(id, &t); err != nil {
		return nil, err
	}
	return nil, nil
}
