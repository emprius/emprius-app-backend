package api

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/types"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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

func (a *API) addTool(t *Tool, userID string) (int64, error) {
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

	if t.Title == "" {
		return 0, ErrEmptyTitleOrDescription
	}

	if t.EstimatedValue == nil {
		return 0, ErrInvalidEstimatedValue
	}

	user, err := a.getUserByID(userID)
	if err != nil {
		return 0, ErrUserNotFound.WithErr(err)
	}
	categories := a.toolCategories()
	validCategory := false
	for _, cat := range categories {
		if cat.ID == t.Category {
			validCategory = true
			break
		}
	}
	if !validCategory {
		return 0, ErrInvalidToolCategory.WithErr(fmt.Errorf("category %d is not valid", t.Category))
	}

	// Validate and convert transport options
	transports, err := a.database.TransportService.GetAllTransports(context.Background())
	if err != nil {
		return 0, ErrInternalServerError.WithErr(err)
	}
	validTransportIDs := make(map[int64]bool)
	for _, t := range transports {
		validTransportIDs[t.ID] = true
	}

	transportOptions := make([]db.Transport, len(t.TransportOptions))
	for i, id := range t.TransportOptions {
		if !validTransportIDs[int64(id)] {
			return 0, ErrInvalidTransportOption.WithErr(fmt.Errorf("transport option %d is not valid", id))
		}
		transportOptions[i] = db.Transport{ID: int64(id)}
	}

	// set default values
	if t.MayBeFree == nil {
		t.MayBeFree = new(bool)
		*t.MayBeFree = true
	}
	if t.AskWithFee == nil {
		t.AskWithFee = new(bool)
	}

	// Set the cost based on the estimated value.
	if *t.EstimatedValue != 0 {
		t.Cost = *t.EstimatedValue / types.FactorCostToPrice
		if t.Cost == 0 {
			t.Cost = 1
		}
	} else {
		t.Cost = 0
	}

	// Set the availability to true by default
	if t.IsAvailable == nil {
		t.IsAvailable = new(bool)
		*t.IsAvailable = true
	}

	dbTool := db.Tool{
		ID:               toolID(userID),
		UserID:           user.ObjectID(),
		Title:            db.SanitizeString(t.Title),
		Description:      t.Description,
		IsAvailable:      *t.IsAvailable,
		MayBeFree:        *t.MayBeFree,
		AskWithFee:       *t.AskWithFee,
		Cost:             t.Cost,
		ToolCategory:     t.Category,
		Rating:           50,
		EstimatedValue:   *t.EstimatedValue,
		Height:           t.Height,
		Weight:           t.Weight,
		Images:           dbImages,
		Location:         t.Location.ToDBLocation(),
		TransportOptions: transportOptions,
		ReservedDates:    []db.DateRange{}, // Initialize empty array
	}
	log.Info().Msgf("adding tool to database, title: %s, user: %s, id: %d", t.Title, userID, dbTool.ID)

	_, err = a.database.ToolService.InsertTool(context.Background(), &dbTool)
	if err != nil {
		return 0, ErrCouldNotInsertToDatabase.WithErr(err)
	}

	return dbTool.ID, nil
}

func toolID(ownerID string) int64 {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s%d", ownerID, time.Now().UnixNano())))
	hash := hasher.Sum(nil)
	// Convert the first 4 bytes of the hash to an absolute int64
	return int64(math.Abs(float64(int64(binary.BigEndian.Uint32(hash[:4])))))
}

func (a *API) toolFromDB(id int64) (*db.Tool, error) {
	tool, err := a.database.ToolService.GetToolByID(context.Background(), id)
	if err == mongo.ErrNoDocuments {
		return nil, ErrToolNotFound.WithErr(fmt.Errorf("tool with id %d not found", id))
	}
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	return tool, nil
}

func (a *API) tool(id int64) (*Tool, error) {
	tool, err := a.toolFromDB(id)
	if err != nil {
		return nil, err
	}
	return new(Tool).FromDBTool(tool), nil
}

func (a *API) toolsByUserID(userID string) ([]*Tool, error) {
	user, err := a.getUserByID(userID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}
	tools, err := a.database.ToolService.GetToolsByUserID(context.Background(), user.ObjectID())
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	result := []*Tool{}
	for _, t := range tools {
		result = append(result, new(Tool).FromDBTool(t))
	}
	return result, nil
}

func (a *API) editTool(id int64, newTool *Tool) (int64, error) {
	tool, err := a.toolFromDB(id)
	if err != nil {
		return 0, err
	}
	if tool == nil {
		return 0, ErrToolNotFound.WithErr(fmt.Errorf("tool with id %d not found", id))
	}

	// Create a copy of the tool for potential restoration
	oldTool := *tool

	// Update all provided fields
	if newTool.Title != "" {
		tool.Title = db.SanitizeString(newTool.Title)
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
	if newTool.EstimatedValue != nil {
		tool.EstimatedValue = *newTool.EstimatedValue
		tool.Cost = *newTool.EstimatedValue / types.FactorCostToPrice
	}
	if newTool.Height != 0 {
		tool.Height = newTool.Height
	}
	if newTool.Weight != 0 {
		tool.Weight = newTool.Weight
	}
	if newTool.Category != 0 {
		categories := a.toolCategories()
		validCategory := false
		for _, cat := range categories {
			if cat.ID == newTool.Category {
				validCategory = true
				break
			}
		}
		if !validCategory {
			return 0, ErrInvalidToolCategory.WithErr(fmt.Errorf("category %d is not valid", newTool.Category))
		}
		tool.ToolCategory = newTool.Category
	}
	if newTool.Location.Latitude != 0 || newTool.Location.Longitude != 0 {
		tool.Location = newTool.Location.ToDBLocation()
	}
	if newTool.IsAvailable != nil {
		tool.IsAvailable = *newTool.IsAvailable
	}
	if len(newTool.Images) > 0 {
		images, err := a.imageListFromSlice(newTool.Images)
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
		tool.Images = dbImages
	}
	if len(newTool.TransportOptions) > 0 {
		// Validate and convert transport options
		transports, err := a.database.TransportService.GetAllTransports(context.Background())
		if err != nil {
			return 0, ErrInternalServerError.WithErr(err)
		}
		validTransportIDs := make(map[int64]bool)
		for _, t := range transports {
			validTransportIDs[t.ID] = true
		}

		transportOptions := make([]db.Transport, len(newTool.TransportOptions))
		for i, id := range newTool.TransportOptions {
			if !validTransportIDs[int64(id)] {
				return 0, ErrInvalidTransportOption.WithErr(fmt.Errorf("transport option %d is not valid", id))
			}
			transportOptions[i] = db.Transport{ID: int64(id)}
		}
		tool.TransportOptions = transportOptions
	}

	// If title changed, we need to handle the tool replacement
	if newTool.Title != "" {
		// Delete the old tool first
		if err := a.deleteTool(oldTool.ID); err != nil {
			return 0, err
		}

		// Insert the new tool
		_, err = a.database.ToolService.InsertTool(context.Background(), tool)
		if err != nil {
			// If insertion fails, try to restore the old tool
			_, restoreErr := a.database.ToolService.InsertTool(context.Background(), &oldTool)
			if restoreErr != nil {
				// Log the restore error but return the original error
				log.Error().Err(restoreErr).Msg("failed to restore old tool after update failure")
			}
			return 0, ErrInternalServerError.WithErr(err)
		}
		return tool.ID, nil
	}

	// For updates without title change, just update the fields
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
		return 0, ErrInternalServerError.WithErr(err)
	}
	return id, nil
}

func (a *API) toolSearch(query *ToolSearch, userLocation *Location) ([]*Tool, error) {
	// Convert user location to GeoJSON format for MongoDB
	searchLocation := db.NewLocation(userLocation.Latitude, userLocation.Longitude)

	opts := db.SearchToolsOptions{
		SearchTerm:       query.SearchTerm,
		Categories:       query.Categories,
		MayBeFree:        query.MayBeFree,
		MaxCost:          query.MaxCost,
		Distance:         query.Distance,
		Location:         &searchLocation,
		TransportOptions: query.TransportOptions,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	tools, err := a.database.ToolService.SearchTools(ctx, opts)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	result := []*Tool{}
	for _, t := range tools {
		result = append(result, new(Tool).FromDBTool(t))
	}
	return result, nil
}

func (a *API) deleteTool(id int64) error {
	filter := bson.M{"_id": id}
	result, err := a.database.ToolService.Collection.DeleteOne(context.Background(), filter)
	if err != nil {
		return ErrInternalServerError.WithErr(err)
	}
	if result.DeletedCount == 0 {
		return ErrToolNotFound.WithErr(fmt.Errorf("tool with id %d not found", id))
	}
	return nil
}

func (a *API) ownToolsHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}
	tools, err := a.toolsByUserID(r.UserID)
	if err != nil {
		return nil, err
	}
	return &ToolsWrapper{Tools: tools}, nil
}

func (a *API) toolHandler(r *Request) (interface{}, error) {
	idParam := r.Context.URLParam("id")
	if idParam == nil {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("missing tool id"))
	}
	id, err := strconv.ParseInt(idParam[0], 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}
	tool, err := a.tool(id)
	if err != nil {
		return nil, err
	}
	return tool, nil
}

func (a *API) userToolsHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	id := r.Context.URLParam("id")
	if id == nil {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("missing user id"))
	}

	tools, err := a.toolsByUserID(id[0])
	if err != nil {
		return nil, err
	}
	return &ToolsWrapper{Tools: tools}, nil
}

func (a *API) toolSearchHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized
	}

	log.Debug().
		Str("query", r.Context.Request.URL.RawQuery).
		Msg("received search request")

	searchTermStr := r.Context.URLParam("term")
	distanceStr := r.Context.URLParam("distance")
	maxCostStr := r.Context.URLParam("maxCost")
	mayBeFreeStr := r.Context.URLParam("maybeFree")
	categoriesStr := r.Context.URLParam("categories")
	transportsStr := r.Context.URLParam("transports")

	// Parse search term
	searchTerm := ""
	if searchTermStr != nil {
		searchTerm = db.SanitizeString(searchTermStr[0])
	}

	// Parse distance parameter (in meters)
	var distance int
	if distanceStr != nil {
		var err error
		distance, err = strconv.Atoi(distanceStr[0])
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("invalid distance value: %s", distanceStr))
		}
	}

	// Parse maxCost parameter
	var maxCost *uint64
	if maxCostStr != nil {
		cost, err := strconv.ParseUint(maxCostStr[0], 10, 64)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(err)
		}
		maxCost = &cost
	}

	// Parse mayBeFree parameter
	var mayBeFree *bool
	if mayBeFreeStr != nil {
		free, err := strconv.ParseBool(mayBeFreeStr[0])
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(err)
		}
		mayBeFree = &free
	}

	// Parse categories from array-style parameters
	var categories []int
	for _, cat := range categoriesStr {
		val, err := strconv.Atoi(cat)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(err)
		}
		categories = append(categories, val)
	}

	// Parse transport options from array-style parameters
	var transportOptions []int
	for _, t := range transportsStr {
		val, err := strconv.Atoi(t)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(err)
		}
		transportOptions = append(transportOptions, val)
	}

	query := ToolSearch{
		SearchTerm:       searchTerm,
		Categories:       categories,
		MaxCost:          maxCost,
		MayBeFree:        mayBeFree,
		Distance:         distance,
		TransportOptions: transportOptions,
	}
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}
	tools, err := a.toolSearch(&query, &user.Location)
	if err != nil {
		return nil, err
	}
	return &ToolsWrapper{Tools: tools}, nil
}

func (a *API) addToolHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	t := Tool{}
	if err := json.Unmarshal(r.Data, &t); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}
	id, err := a.addTool(&t, r.UserID)
	if err != nil {
		return nil, err
	}
	return &ToolID{ID: id}, nil
}

func (a *API) deleteToolHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}
	idParam := r.Context.URLParam("id")
	if idParam == nil {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("missing tool id"))
	}

	id, err := strconv.ParseInt(idParam[0], 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}
	// check the tool is owned by the user
	tool, err := a.toolFromDB(id)
	if err != nil {
		return nil, err
	}
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}
	if tool.UserID != user.ObjectID() {
		return nil, ErrToolNotOwnedByUser.WithErr(fmt.Errorf("tool with id %d is not owned by user %s", id, user.ID))
	}
	if err := a.deleteTool(id); err != nil {
		return nil, err
	}
	return nil, nil
}

func (a *API) editToolHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}
	idParam := r.Context.URLParam("id")
	if idParam == nil {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("missing tool id"))
	}
	id, err := strconv.ParseInt(idParam[0], 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}
	// check the tool is owned by the user
	tool, err := a.toolFromDB(id)
	if err != nil {
		return nil, err
	}
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}
	if tool.UserID != user.ObjectID() {
		return nil, ErrToolNotOwnedByUser.WithErr(fmt.Errorf("tool with id %d is not owned by user %s", id, user.ID))
	}
	t := Tool{}
	if err := json.Unmarshal(r.Data, &t); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}
	newID, err := a.editTool(id, &t)
	if err != nil {
		return nil, err
	}
	return &ToolID{ID: newID}, nil
}
