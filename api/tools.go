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
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// RegisterToolRoutes registers all tool-related routes to the provided router group
func (a *API) RegisterToolRoutes(r chi.Router) {
	// GET /tools
	log.Info().Msg("register route GET /tools")
	r.Get("/tools", a.routerHandler(a.ownToolsHandler))
	// GET /tools/search
	log.Info().Msg("register route GET /tools/search")
	r.Get("/tools/search", a.routerHandler(a.toolSearchHandler))
	// GET /tools/user/{id}
	log.Info().Msg("register route GET /tools/user/{id}")
	r.Get("/tools/user/{id}", a.routerHandler(a.userToolsHandler))
	// GET /tools/{id}
	log.Info().Msg("register route GET /tools/{id}")
	r.Get("/tools/{id}", a.routerHandler(a.toolHandler))
	// GET /tools/{id}/ratings
	log.Info().Msg("register route GET /tools/{id}/ratings")
	r.Get("/tools/{id}/ratings", a.routerHandler(a.HandleGetToolRatings))
	// GET /tools/{id}/history
	log.Info().Msg("register route GET /tools/{id}/history")
	r.Get("/tools/{id}/history", a.routerHandler(a.toolHistoryHandler))
	// POST /tools
	log.Info().Msg("register route POST /tools")
	r.Post("/tools", a.routerHandler(a.addToolHandler))
	// PUT /tools/{id}
	log.Info().Msg("register route PUT /tools/{id}")
	r.Put("/tools/{id}", a.routerHandler(a.editToolHandler))
	// DELETE /tools/{id}
	log.Info().Msg("register route DELETE /tools/{id}")
	r.Delete("/tools/{id}", a.routerHandler(a.deleteToolHandler))
}

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

	if t.ToolValuation == nil {
		return 0, ErrInvalidToolValuationValue
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
	if *t.ToolValuation != 0 {
		t.Cost = *t.ToolValuation / types.FactorCostToPrice
		if t.Cost == 0 {
			t.Cost = 1
		}
	} else {
		t.Cost = 0
	}
	// Set the estimated daily cost to the same as the cost
	t.EstimatedDailyCost = t.Cost

	// Set the availability to true by default
	if t.IsAvailable == nil {
		t.IsAvailable = new(bool)
		*t.IsAvailable = true
	}

	// Set the nomadic to false by default
	if t.IsNomadic == nil {
		t.IsNomadic = new(bool)
		*t.IsNomadic = false
	}

	// Create the tool with real location
	toolId := toolID(userID)
	realLocation := t.Location.ToDBLocation()

	dbTool := db.Tool{
		ID:                 toolId,
		UserID:             user.ID,
		Title:              db.SanitizeString(t.Title),
		Description:        t.Description,
		IsAvailable:        *t.IsAvailable,
		MayBeFree:          *t.MayBeFree,
		AskWithFee:         *t.AskWithFee,
		Cost:               t.Cost,
		EstimatedDailyCost: t.EstimatedDailyCost,
		ToolCategory:       t.Category,
		Rating:             50,
		ToolValuation:      *t.ToolValuation,
		Height:             t.Height,
		Weight:             t.Weight,
		MaxDistance:        t.MaxDistance,
		Images:             dbImages,
		Location:           realLocation,
		ObfuscatedLocation: db.ObfuscateLocation(realLocation, user.ID, user.Salt),
		TransportOptions:   transportOptions,
		ReservedDates:      []db.DateRange{}, // Initialize empty array
		IsNomadic:          *t.IsNomadic,
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
	_, err := fmt.Fprintf(hasher, "%s%d", ownerID, time.Now().UnixNano())
	if err != nil {
		log.Error().Err(err).Msg("Error writing to hasher")
	}
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

func (a *API) editTool(id int64, newTool *Tool, user *db.User) (int64, error) {
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
	if newTool.IsNomadic != nil {
		tool.IsNomadic = *newTool.IsNomadic
	}

	if newTool.ToolValuation != nil {
		tool.ToolValuation = *newTool.ToolValuation
		tool.Cost = *newTool.ToolValuation / types.FactorCostToPrice
		tool.EstimatedDailyCost = tool.Cost
	}
	if newTool.Cost != 0 {
		if newTool.Cost <= tool.EstimatedDailyCost {
			tool.Cost = newTool.Cost
		} else {
			tool.Cost = tool.EstimatedDailyCost
		}
	}
	if newTool.Height != 0 {
		tool.Height = newTool.Height
	}
	if newTool.Weight != 0 {
		tool.Weight = newTool.Weight
	}
	if newTool.MaxDistance != 0 {
		tool.MaxDistance = newTool.MaxDistance
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
		tool.ObfuscatedLocation = db.ObfuscateLocation(tool.Location, user.ID, user.Salt)
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
		"title":              tool.Title,
		"description":        tool.Description,
		"isAvailable":        tool.IsAvailable,
		"mayBeFree":          tool.MayBeFree,
		"askWithFee":         tool.AskWithFee,
		"cost":               tool.Cost,
		"estimatedDailyCost": tool.EstimatedDailyCost,
		"toolCategory":       tool.ToolCategory,
		"toolValuation":      tool.ToolValuation,
		"height":             tool.Height,
		"weight":             tool.Weight,
		"maxDistance":        tool.MaxDistance,
		"images":             tool.Images,
		"location":           tool.Location,
		"obfuscatedLocation": tool.ObfuscatedLocation,
		"transportOptions":   tool.TransportOptions,
		"isNomadic":          tool.IsNomadic,
		"actualUserId":       tool.ActualUserID,
	}
	err = a.database.ToolService.UpdateToolFields(context.Background(), id, updates)
	if err != nil {
		return 0, ErrInternalServerError.WithErr(err)
	}
	return id, nil
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

func (a *API) toolHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	idParam := r.Context.URLParam("id")
	if idParam == nil {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("missing tool id"))
	}

	// Get requesting user ID
	requestingUserID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	// Get the tool from the database with access control
	dbTool, err := a.GetToolByIDWithAccessControl(r, idParam)
	if err != nil {
		return nil, err
	}

	// Only show real location if user is authenticated and is the owner
	useRealLocation := !dbTool.IsNomadic && dbTool.UserID == requestingUserID
	// Or if the tool is nomadic and the user is the actual user
	if dbTool.IsNomadic && dbTool.ActualUserID == requestingUserID {
		useRealLocation = true
		// Or if the tool is nomadic and the user is the owner and the actual user is not set
	} else if dbTool.IsNomadic && dbTool.ActualUserID.IsZero() && dbTool.UserID == requestingUserID {
		useRealLocation = true
	}

	// Convert DB tool to API tool with appropriate location
	tool := new(Tool).FromDBTool(dbTool, a.database, useRealLocation)

	// Convert community ObjectIDs to strings
	communityIDs := make([]string, len(dbTool.Communities))
	for i, communityID := range dbTool.Communities {
		// This line uses primitive.ObjectID.Hex() method
		communityIDs[i] = communityID.Hex()
	}
	tool.Communities = communityIDs

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

	userID, err := primitive.ObjectIDFromHex(id[0])
	if err != nil {
		return nil, ErrUserNotFound.WithErr(fmt.Errorf("invalid user id format: %s", r.Context.URLParam("id")))
	}

	return a.getUserTools(r, userID)
}

func (a *API) ownToolsHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user ObjectID
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	return a.getUserTools(r, user.ID)
}

// Util function to DRY to get tools from a user with pagination and search term
func (a *API) getUserTools(r *Request, id primitive.ObjectID) (interface{}, error) {
	// Use access control method to check if user can be accessed
	_, err := a.GetUserByIDWithAccessControl(r, id)
	if err != nil {
		return nil, err
	}

	// Get pagination parameters
	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	searchTerm := *r.Context.GetSearchTerm()

	// Get paginated tools with access control
	tools, total, err := a.database.ToolService.GetToolsByUserIDPaginated(context.Background(), id, page, pageSize, searchTerm)
	if err != nil {
		return nil, err
	}

	return a.getToolListPaginatedResponse(tools, page, pageSize, total), nil
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

	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}
	userObjID := user.ID

	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	opts := db.SearchToolsOptions{
		SearchTerm:       searchTerm,
		Categories:       categories,
		MayBeFree:        mayBeFree,
		MaxCost:          maxCost,
		Distance:         distance,
		Location:         &user.Location,
		TransportOptions: transportOptions,
		UserID:           &userObjID,
		Page:             page,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	tools, total, err := a.database.ToolService.SearchTools(ctx, opts)
	if err != nil {
		return nil, err
	}

	return a.getToolListPaginatedResponse(tools, page, pageSize, total), nil
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

	// Handle communities if provided
	if len(t.Communities) > 0 {
		err = a.addToolToCommunity(r.Context.Request.Context(), id, t.Communities)
		if err != nil {
			return nil, err
		}
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
	if tool.UserID != user.ID {
		return nil, ErrToolNotOwnedByUser.WithErr(fmt.Errorf("tool with id %d is not owned by user %s", id, user.ID))
	}
	if err := a.deleteTool(id); err != nil {
		return nil, err
	}
	return nil, nil
}

// HandleGetToolRatings handles GET /tools/{id}/ratings
func (a *API) HandleGetToolRatings(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get tool ID from URL
	idParam := r.Context.URLParam("id")
	if idParam == nil {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("missing tool id"))
	}

	// Get the tool from the database with access control
	_, err := a.GetToolByIDWithAccessControl(r, idParam)
	if err != nil {
		return nil, err
	}

	// Get pagination parameters
	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get unified ratings for the tool with access control
	unifiedRatings, total, err := a.database.BookingService.GetRatingsByToolID(
		r.Context.Request.Context(),
		idParam[0],
		page,
		pageSize,
	)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	if unifiedRatings == nil {
		// Return empty array instead of nil
		unifiedRatings = make([]*db.UnifiedRating, 0)
	}

	return a.getUnifiedRatingsPaginatedResponse(unifiedRatings, page, pageSize, total), nil
}

// getToolHistory retrieves the history of a nomadic tool
func (a *API) getToolHistory(toolID int64) ([]ToolHistoryEntry, error) {
	// Get the tool history from the database
	dbEntries, err := a.database.ToolService.GetToolHistory(context.Background(), toolID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Convert DB entries to API entries
	entries := make([]ToolHistoryEntry, len(dbEntries))
	for i, entry := range dbEntries {
		entries[i] = ToolHistoryEntry{
			ID:         entry.ID.Hex(),
			UserID:     entry.UserID.Hex(),
			PickupDate: entry.PickupDate.Unix(),
		}
		entries[i].Location.FromDBLocation(entry.Location)

		if !entry.BookingID.IsZero() {
			entries[i].BookingID = entry.BookingID.Hex()
		}

		// Get user name
		user, err := a.getUserByID(entry.UserID.Hex())
		if err == nil {
			entries[i].UserName = user.Name
		}
	}

	return entries, nil
}

// toolHistoryHandler handles GET /tools/{id}/history
func (a *API) toolHistoryHandler(r *Request) (interface{}, error) {
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

	tool, err := a.GetToolByIDWithAccessControl(r, idParam)
	if err != nil {
		return nil, err
	}

	if !tool.IsNomadic {
		return nil, ErrToolNotNomadic.WithErr(fmt.Errorf("tool with id %d is not nomadic", id))
	}

	// Get the tool history
	history, err := a.getToolHistory(id)
	if err != nil {
		return nil, err
	}

	return history, nil
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
	if tool.UserID != user.ID {
		return nil, ErrToolNotOwnedByUser.WithErr(fmt.Errorf("tool with id %d is not owned by user %s", id, user.ID))
	}
	t := Tool{}
	if err := json.Unmarshal(r.Data, &t); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Check if trying to change nomadic status
	if t.IsNomadic != nil && tool.IsNomadic != *t.IsNomadic {
		// Only the owner can change nomadic status
		if tool.UserID != user.ID {
			return nil, ErrOnlyOwnerCanChangeNomadicStatus.WithErr(
				fmt.Errorf("only the owner can change a tool from nomadic to non-nomadic"),
			)
		}
		// Check for pending bookings
		pendingBookings, err := a.database.BookingService.GetPendingBookingsForTool(
			r.Context.Request.Context(),
			strconv.FormatInt(id, 10),
		)
		if err != nil {
			return nil, ErrInternalServerError.WithErr(err)
		}
		if len(pendingBookings) > 0 {
			return nil, ErrCannotChangeNomadicWithPendingBookings.WithErr(
				fmt.Errorf("cannot change nomadic status when there are pending bookings"),
			)
		}
	}

	err = a.addToolToCommunity(r.Context.Request.Context(), id, t.Communities)
	if err != nil {
		return nil, err
	}

	newID, err := a.editTool(id, &t, user)
	if err != nil {
		return nil, err
	}
	return &ToolID{ID: newID}, nil
}

func (a *API) GetToolByIDWithAccessControl(r *Request, toolId []string) (*db.Tool, error) {
	// Get requesting user ID
	requestingUserID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	id, err := strconv.ParseInt(toolId[0], 10, 64)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get the tool from the database with access control
	dbTool, err := a.database.ToolService.GetToolByIDWithAccessControl(r.Context.Request.Context(), id, requestingUserID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrToolNotFound.WithErr(fmt.Errorf("tool with id %s not found", toolId[0]))
		}
		return nil, ErrInternalServerError.WithErr(err)
	}
	return dbTool, nil
}
