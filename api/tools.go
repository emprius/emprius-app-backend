package api

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
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
		return 0, fmt.Errorf("title and description must not be empty")
	}
	if t.EstimatedValue == 0 {
		return 0, fmt.Errorf("estimated value must be greater than 0")
	}
	if t.MayBeFree == nil {
		return 0, fmt.Errorf("may be free must not be nil")
	}
	if t.AskWithFee == nil {
		return 0, fmt.Errorf("ask with fee must not be nil")
	}
	if t.Cost == nil {
		return 0, fmt.Errorf("cost must not be nil")
	}
	user, err := a.userByEmail(userID)
	if err != nil {
		return 0, fmt.Errorf("user not found: %w", err)
	}
	if !db.WithinCircumference(user.Location, t.Location, maxAllowedToolDistance) {
		return 0, fmt.Errorf("tool location is too far away, more than %d km", maxAllowedToolDistance)
	}

	if t.Category < 0 || t.Category >= len(a.toolCategories()) {
		return 0, fmt.Errorf("invalid category %d", t.Category)
	}
	dbTool := db.Tool{
		ID:             toolID(userID, t.Title),
		UserID:         userID,
		Title:          db.SanitizeString(t.Title),
		Description:    t.Description,
		IsAvailable:    true,
		MayBeFree:      *t.MayBeFree,
		AskWithFee:     *t.AskWithFee,
		Cost:           *t.Cost,
		ToolCategory:   t.Category,
		Rating:         50,
		EstimatedValue: t.EstimatedValue,
		Height:         t.Height,
		Weight:         t.Weight,
		Images:         dbImages,
		Location:       t.Location,
	}
	log.Info().Msgf("adding tool to database, title: %s, user: %s, id: %d", t.Title, userID, dbTool.ID)

	_, err = a.database.ToolService.InsertTool(context.Background(), &dbTool)
	if err != nil {
		return 0, fmt.Errorf("could not insert tool to database: %w", err)
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
	return a.database.ToolService.GetToolByID(context.Background(), id)
}

func (a *API) toolsByUerID(id string) ([]db.Tool, error) {
	tools, err := a.database.ToolService.GetToolsByUserID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	result := make([]db.Tool, len(tools))
	for i, t := range tools {
		result[i] = *t
	}
	return result, nil
}

func (a *API) tools() ([]db.Tool, error) {
	tools, err := a.database.ToolService.GetAllTools(context.Background())
	if err != nil {
		return nil, err
	}
	result := make([]db.Tool, len(tools))
	for i, t := range tools {
		result[i] = *t
	}
	return result, nil
}

func (a *API) toolsByDistance(location db.Location, distance int) ([]db.Tool, error) {
	tools, err := a.database.ToolService.SearchToolsByLocation(context.Background(), location, distance)
	if err != nil {
		return nil, err
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
		return fmt.Errorf("could not get tool: %w", err)
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
		tool.ToolCategory = newTool.Category
	}
	if newTool.Location.Latitude != 0 && newTool.Location.Longitude != 0 {
		tool.Location = newTool.Location
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
	updates := map[string]interface{}{
		"title":          tool.Title,
		"description":    tool.Description,
		"isAvailable":    tool.IsAvailable,
		"mayBeFree":      tool.MayBeFree,
		"askWithFee":     tool.AskWithFee,
		"cost":           tool.Cost,
		"toolCategory":   tool.ToolCategory,
		"estimatedValue": tool.EstimatedValue,
		"height":         tool.Height,
		"weight":         tool.Weight,
		"images":         tool.Images,
		"location":       tool.Location,
	}
	return a.database.ToolService.UpdateToolFields(context.Background(), id, updates)
}

// TODO: this is very naive, we should use a proper SQL query
func (a *API) toolSearch(query *ToolSearch, userLocation *db.Location) ([]db.Tool, error) {
	opts := db.SearchToolsOptions{
		Categories: query.Categories,
		MayBeFree:  query.MayBeFree,
		MaxCost:    query.MaxCost,
		Distance:   query.Distance,
		Location:   userLocation,
	}
	tools, err := a.database.ToolService.SearchTools(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("could not search tools: %w", err)
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
	return err
}

// GET /tools returns tools owned by the user
func (a *API) ownToolsHandler(r *Request) (interface{}, error) {
	tools, err := a.toolsByUerID(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("could not get tools: %w", err)
	}
	return &ToolsWrapper{Tools: tools}, nil
}

// GET /tools/:id returns a tool by id
func (a *API) toolHandler(r *Request) (interface{}, error) {
	id, err := strconv.ParseInt(r.Context.URLParam("id"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %w", err)
	}
	tool, err := a.tool(id)
	if err != nil {
		return nil, &HTTPError{Code: http.StatusNotFound, Message: fmt.Sprintf("tool %d not found: %s", id, err)}
	}
	return tool, nil
}

// GET /tools/user/:id returns tools owned by the user
func (a *API) userToolsHandler(r *Request) (interface{}, error) {
	tools, err := a.toolsByUerID(r.Context.URLParam("id"))
	if err != nil {
		return nil, fmt.Errorf("could not get tools: %w", err)
	}
	return &ToolsWrapper{Tools: tools}, nil
}

// GET /tools/search filters tools
func (a *API) toolSearchHandler(r *Request) (interface{}, error) {
	searchTerm := r.Context.URLParam("searchTerm")
	maxCostStr := r.Context.URLParam("maxCost")
	mayBeFreeStr := r.Context.URLParam("maybeFree")
	availableFromStr := r.Context.URLParam("availableFrom")
	categoriesStr := r.Context.URLParam("categories")

	var maxCost *uint64
	if maxCostStr != "" {
		cost, err := strconv.ParseUint(maxCostStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid maxCost: %w", err)
		}
		maxCost = &cost
	}

	var mayBeFree *bool
	if mayBeFreeStr != "" {
		free, err := strconv.ParseBool(mayBeFreeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid maybeFree: %w", err)
		}
		mayBeFree = &free
	}

	var availableFrom int
	if availableFromStr != "" {
		from, err := strconv.Atoi(availableFromStr)
		if err != nil {
			return nil, fmt.Errorf("invalid availableFrom: %w", err)
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
				return nil, fmt.Errorf("invalid category: %w", err)
			}
			categories[i] = val
		}
	}

	query := ToolSearch{
		Term:          searchTerm,
		Categories:    categories,
		MaxCost:       maxCost,
		MayBeFree:     mayBeFree,
		AvailableFrom: availableFrom,
	}
	user, err := a.userByEmail(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("could not get user: %w", err)
	}
	tools, err := a.toolSearch(&query, &user.Location)
	if err != nil {
		return nil, fmt.Errorf("could not search tools: %w", err)
	}
	return &ToolsWrapper{Tools: tools}, nil
}

// POST /tools adds a new tool
func (a *API) addToolHandler(r *Request) (interface{}, error) {
	t := Tool{}
	if err := json.Unmarshal(r.Data, &t); err != nil {
		return nil, fmt.Errorf("could not parse tool: %w", err)
	}
	id, err := a.addTool(&t, r.UserID)
	if err != nil {
		return nil, fmt.Errorf("could not add tool: %w", err)
	}
	return &ToolID{ID: id}, nil
}

// DELETE /tools/:id deletes a tool
func (a *API) deleteToolHandler(r *Request) (interface{}, error) {
	id, err := strconv.ParseInt(r.Context.URLParam("id"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %w", err)
	}
	// check the tool is owned by the user
	tool, err := a.tool(id)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %w", err)
	}
	if tool.UserID != r.UserID {
		return nil, fmt.Errorf("tool not owned by user")
	}
	if err := a.deleteTool(id); err != nil {
		return nil, fmt.Errorf("could not delete tool: %w", err)
	}
	return nil, nil
}

// PUT /tools/:id edit a tool
func (a *API) editToolHandler(r *Request) (interface{}, error) {
	id, err := strconv.ParseInt(r.Context.URLParam("id"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %w", err)
	}
	// check the tool is owned by the user
	tool, err := a.tool(id)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %w", err)
	}
	if tool.UserID != r.UserID {
		return nil, fmt.Errorf("tool not owned by user")
	}
	t := Tool{}
	if err := json.Unmarshal(r.Data, &t); err != nil {
		return nil, fmt.Errorf("could not parse tool: %w", err)
	}
	if err := a.editTool(id, &t); err != nil {
		return nil, fmt.Errorf("could not edit tool: %w", err)
	}
	return nil, nil
}
