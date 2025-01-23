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

	if t.Title == "" || t.Description == "" {
		return 0, ErrEmptyTitleOrDescription.WithErr(fmt.Errorf("title or description is empty"))
	}
	if t.EstimatedValue == 0 {
		return 0, ErrInvalidEstimatedValue.WithErr(fmt.Errorf("estimated value must be greater than 0"))
	}
	if t.MayBeFree == nil {
		return 0, ErrMayBeFreeRequired.WithErr(fmt.Errorf("may be free field is required"))
	}
	if t.AskWithFee == nil {
		return 0, ErrAskWithFeeRequired.WithErr(fmt.Errorf("ask with fee field is required"))
	}
	if t.Cost == nil {
		return 0, ErrCostRequired.WithErr(fmt.Errorf("cost field is required"))
	}
	user, err := a.getUserByID(userID)
	if err != nil {
		return 0, ErrUserNotFound.WithErr(err)
	}
	if !db.WithinCircumference(user.Location, t.Location, maxAllowedToolDistance) {
		return 0, ErrToolLocationTooFar.WithErr(fmt.Errorf(
			"tool location is more than %d meters away from user location",
			maxAllowedToolDistance,
		))
	}

	if t.Category < 0 || t.Category >= len(a.toolCategories()) {
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

	dbTool := db.Tool{
		ID:               toolID(userID, t.Title),
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
	log.Info().Msgf("adding tool to database, title: %s, user: %s, id: %d", t.Title, userID, dbTool.ID)

	_, err = a.database.ToolService.InsertTool(context.Background(), &dbTool)
	if err != nil {
		return 0, ErrCouldNotInsertToDatabase.WithErr(err)
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
		return nil, ErrToolNotFound.WithErr(fmt.Errorf("tool with id %d not found", id))
	}
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	return tool, nil
}

func (a *API) toolsByUserID(userID string) ([]db.Tool, error) {
	user, err := a.getUserByID(userID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}
	tools, err := a.database.ToolService.GetToolsByUserID(context.Background(), user.ID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	result := make([]db.Tool, len(tools))
	for i, t := range tools {
		result[i] = *t
	}
	return result, nil
}

func (a *API) editTool(id int64, newTool *Tool, userID string) (int64, error) {
	tool, err := a.tool(id)
	if err != nil {
		return 0, err
	}
	if tool == nil {
		return 0, ErrToolNotFound.WithErr(fmt.Errorf("tool with id %d not found", id))
	}

	if newTool.Title != "" {
		// If title changes, we need to update the ID since it's derived from the title
		oldID := tool.ID
		tool.Title = db.SanitizeString(newTool.Title)
		tool.ID = toolID(userID, tool.Title)
		// Delete the old tool and insert the new one with updated ID
		if err := a.deleteTool(oldID); err != nil {
			return 0, err
		}
		_, err = a.database.ToolService.InsertTool(context.Background(), tool)
		if err != nil {
			return 0, ErrInternalServerError.WithErr(err)
		}
		return tool.ID, nil
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
			return 0, ErrInvalidToolCategory.WithErr(fmt.Errorf("category %d is not valid", newTool.Category))
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

func (a *API) toolSearch(query *ToolSearch, userLocation *db.Location) ([]db.Tool, error) {
	opts := db.SearchToolsOptions{
		SearchTerm:       query.SearchTerm,
		Categories:       query.Categories,
		MayBeFree:        query.MayBeFree,
		MaxCost:          query.MaxCost,
		Distance:         query.Distance,
		Location:         userLocation,
		TransportOptions: query.TransportOptions,
	}
	tools, err := a.database.ToolService.SearchTools(context.Background(), opts)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
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
		return ErrInternalServerError.WithErr(err)
	}
	return nil
}

// GET /tools returns tools owned by the user
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

// GET /tools/:id returns a tool by id
func (a *API) toolHandler(r *Request) (interface{}, error) {
	id, err := strconv.ParseInt(r.Context.URLParam("id"), 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
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
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}
	tools, err := a.toolsByUserID(r.Context.URLParam("id"))
	if err != nil {
		return nil, err
	}
	return &ToolsWrapper{Tools: tools}, nil
}

// GET /tools/search filters tools
func (a *API) toolSearchHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	log.Debug().
		Str("url", r.Context.Request.URL.String()).
		Str("query", r.Context.Request.URL.RawQuery).
		Msg("received search request")

	searchTerm := r.Context.URLParam("searchTerm")
	log.Debug().
		Str("rawSearchTerm", searchTerm).
		Msg("parsed search term")

	searchTerm = db.SanitizeString(searchTerm)
	log.Debug().
		Str("sanitizedSearchTerm", searchTerm).
		Msg("sanitized search term")

	// Get distance parameter
	distanceStr := r.Context.URLParam("distance")
	var distance int
	if distanceStr != "" {
		var err error
		distance, err = strconv.Atoi(distanceStr)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("invalid distance value: %s", distanceStr))
		}
	}

	maxCostStr := r.Context.URLParam("maxCost")
	mayBeFreeStr := r.Context.URLParam("maybeFree")
	availableFromStr := r.Context.URLParam("availableFrom")
	categoriesStr := r.Context.URLParam("categories")

	var maxCost *uint64
	if maxCostStr != "" {
		cost, err := strconv.ParseUint(maxCostStr, 10, 64)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(err)
		}
		maxCost = &cost
	}

	var mayBeFree *bool
	if mayBeFreeStr != "" {
		free, err := strconv.ParseBool(mayBeFreeStr)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(err)
		}
		mayBeFree = &free
	}

	var availableFrom int
	if availableFromStr != "" {
		from, err := strconv.Atoi(availableFromStr)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(err)
		}
		availableFrom = from
	}

	var categories []int
	// Check for array-style categories[] parameters
	categoryParams := r.Context.Request.URL.Query()["categories[]"]
	if len(categoryParams) > 0 {
		categories = make([]int, len(categoryParams))
		for i, cat := range categoryParams {
			val, err := strconv.Atoi(cat)
			if err != nil {
				return nil, ErrInvalidRequestBodyData.WithErr(err)
			}
			categories[i] = val
		}
	} else if categoriesStr != "" {
		// Fallback to comma-separated list
		catStrings := strings.Split(categoriesStr, ",")
		categories = make([]int, len(catStrings))
		for i, cat := range catStrings {
			val, err := strconv.Atoi(cat)
			if err != nil {
				return nil, ErrInvalidRequestBodyData.WithErr(err)
			}
			categories[i] = val
		}
	}

	// Parse transport options
	var transportOptions []int
	transportParams := r.Context.Request.URL.Query()["transports[]"]
	if len(transportParams) > 0 {
		transportOptions = make([]int, len(transportParams))
		for i, t := range transportParams {
			val, err := strconv.Atoi(t)
			if err != nil {
				return nil, ErrInvalidRequestBodyData.WithErr(err)
			}
			transportOptions[i] = val
		}
	} else {
		transportOptionsStr := r.Context.URLParam("transportOptions")
		if transportOptionsStr != "" {
			// Fallback to comma-separated list
			transportStrings := strings.Split(transportOptionsStr, ",")
			transportOptions = make([]int, len(transportStrings))
			for i, t := range transportStrings {
				val, err := strconv.Atoi(t)
				if err != nil {
					return nil, ErrInvalidRequestBodyData.WithErr(err)
				}
				transportOptions[i] = val
			}
		}
	}

	query := ToolSearch{
		SearchTerm:       searchTerm,
		Categories:       categories,
		MaxCost:          maxCost,
		MayBeFree:        mayBeFree,
		AvailableFrom:    availableFrom,
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

// POST /tools adds a new tool
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

// DELETE /tools/:id deletes a tool
func (a *API) deleteToolHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	id, err := strconv.ParseInt(r.Context.URLParam("id"), 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}
	// check the tool is owned by the user
	tool, err := a.tool(id)
	if err != nil {
		return nil, err
	}
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}
	if tool.UserID != user.ID {
		return nil, ErrToolNotOwnedByUser.WithErr(fmt.Errorf("tool with id %d is not owned by user %s", id, user.ID.Hex()))
	}
	if err := a.deleteTool(id); err != nil {
		return nil, err
	}
	return nil, nil
}

// PUT /tools/:id edit a tool
func (a *API) editToolHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	id, err := strconv.ParseInt(r.Context.URLParam("id"), 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}
	// check the tool is owned by the user
	tool, err := a.tool(id)
	if err != nil {
		return nil, err
	}
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}
	if tool.UserID != user.ID {
		return nil, ErrToolNotOwnedByUser.WithErr(fmt.Errorf("tool with id %d is not owned by user %s", id, user.ID.Hex()))
	}
	t := Tool{}
	if err := json.Unmarshal(r.Data, &t); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}
	newID, err := a.editTool(id, &t, r.UserID)
	if err != nil {
		return nil, err
	}
	return &ToolID{ID: newID}, nil
}
