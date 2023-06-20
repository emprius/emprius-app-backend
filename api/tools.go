package api

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/types"
	"github.com/rs/zerolog/log"
)

const (
	maxAllowedToolDistance = 200000 // m
)

func (a *API) toolCategories() []db.ToolCategory {
	doc, err := a.database.Query("SELECT * FROM toolCategory")
	if err != nil {
		panic(err)
	}
	defer doc.Close()
	categories := []db.ToolCategory{}
	if err := document.ScanIterator(doc, &categories); err != nil {
		panic(err)
	}
	return categories
}

func (a *API) addTool(t *Tool, userID string) error {
	// check if images are in database
	dbImages, err := a.imageListFromSlice(t.Images)
	if err != nil {
		return err
	}
	if t.Title == "" || t.Description == "" {
		return fmt.Errorf("title and description must not be empty")
	}
	if t.EstimatedValue == 0 {
		return fmt.Errorf("estimated value must be greater than 0")
	}
	if t.MayBeFree == nil {
		return fmt.Errorf("may be free must not be nil")
	}
	if t.AskWithFee == nil {
		return fmt.Errorf("ask with fee must not be nil")
	}
	if t.Cost == nil {
		return fmt.Errorf("cost must not be nil")
	}
	user, err := a.userByEmail(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	if !db.WithinCircumference(user.Location, t.Location, maxAllowedToolDistance) {
		return fmt.Errorf("tool location is too far away, more than %d km", maxAllowedToolDistance)
	}

	if t.Category < 0 || t.Category >= len(a.toolCategories()) {
		return fmt.Errorf("invalid category %d", t.Category)
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

	if err := a.database.Exec("INSERT INTO tool VALUES ?", &dbTool); err != nil {
		return fmt.Errorf("could not insert tool to database: %w", err)
	}

	return nil
}

func toolID(ownerID string, title string) int64 {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s-%s", ownerID, title)))
	hash := hasher.Sum(nil)
	// Convert the first 4 bytes of the hash to an absolute int64
	return int64(math.Abs(float64(int64(binary.BigEndian.Uint64(hash[:8])))))
}

func (a *API) tool(id int64) (*db.Tool, error) {
	doc, err := a.database.QueryDocument("SELECT * FROM tool WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	tool := db.Tool{}
	if err := document.StructScan(doc, &tool); err != nil {
		return nil, err
	}
	return &tool, nil
}

func (a *API) toolsByUerID(id string) ([]db.Tool, error) {
	doc, err := a.database.Query("SELECT * FROM tool WHERE userId = ?", id)
	if err != nil {
		return nil, err
	}
	defer doc.Close()
	tools := []db.Tool{}
	if err := document.ScanIterator(doc, &tools); err != nil {
		return nil, err
	}
	return tools, nil
}

func (a *API) tools() ([]db.Tool, error) {
	doc, err := a.database.Query("SELECT * FROM tool")
	if err != nil {
		return nil, err
	}
	defer doc.Close()
	tools := []db.Tool{}
	if err := document.ScanIterator(doc, &tools); err != nil {
		return nil, err
	}
	return tools, nil
}

func (a *API) toolsByDistance(location db.Location, distance int) ([]db.Tool, error) {
	result, err := a.database.Query("SELECT * FROM tool")
	if err != nil {
		return nil, err
	}
	defer result.Close()
	var inRangeTools []db.Tool
	err = result.Iterate(func(d types.Document) error {
		var t db.Tool
		err := document.StructScan(d, &t)
		if err != nil {
			return err
		}
		if db.WithinCircumference(t.Location, location, distance) {
			inRangeTools = append(inRangeTools, t)
		}
		return nil
	})

	return inRangeTools, nil
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
		dbImages, err := a.imageListFromSlice(newTool.Images)
		if err != nil {
			return err
		}
		tool.Images = dbImages
	}
	return a.database.Exec(`UPDATE tool SET title = ?, 
	description = ?, isAvailable = ?, mayBeFree = ?, 
	askWithFee = ?, cost = ?, toolCategory = ?, 
	estimatedValue = ?, height = ?, weight = ?, 
	images = ?, location = ? WHERE id = ?`,
		tool.Title, tool.Description, tool.IsAvailable, tool.MayBeFree,
		tool.AskWithFee, tool.Cost, tool.ToolCategory,
		tool.EstimatedValue, tool.Height, tool.Weight, tool.Images, tool.Location, id)
}

// TODO: this is very naive, we should use a proper SQL query
func (a *API) toolSearch(query *ToolSearch, userLocation *db.Location) ([]db.Tool, error) {
	tools, err := a.tools()
	if err != nil {
		return nil, fmt.Errorf("could not get tools: %w", err)
	}

	var filteredTools []db.Tool
	for _, tool := range tools {
		if len(query.Categories) != 0 {
			found := false
			for _, category := range query.Categories {
				if tool.ToolCategory == category {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if query.MayBeFree != nil && tool.MayBeFree != *query.MayBeFree {
			continue
		}
		if query.MaxCost != nil && tool.Cost > *query.MaxCost {
			continue
		}
		if query.Distance != 0 && !db.WithinCircumference(tool.Location, *userLocation, query.Distance) {
			continue
		}
		filteredTools = append(filteredTools, tool)
	}
	return filteredTools, nil
}

func (a *API) deleteTool(id int64) error {
	return a.database.Exec("DELETE FROM tool WHERE id = ?", id)
}

// GET /tools returns tools owned by the user
func (a *API) ownToolsHandler(r *Request) (interface{}, error) {
	tools, err := a.toolsByUerID(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("could not get tools: %w", err)
	}
	return tools, nil
}

// GET /tools/:id returns a tool by id
func (a *API) toolHandler(r *Request) (interface{}, error) {
	id, err := strconv.ParseInt(r.Context.URLParam("id"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %w", err)
	}
	tool, err := a.tool(id)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %w", err)
	}
	return tool, nil
}

// GET /tools/user/:id returns tools owned by the user
func (a *API) userToolsHandler(r *Request) (interface{}, error) {
	tools, err := a.toolsByUerID(r.Context.URLParam("id"))
	if err != nil {
		return nil, fmt.Errorf("could not get tools: %w", err)
	}
	return tools, nil
}

// GET /tools/search filters tools
func (a *API) toolSearchHandler(r *Request) (interface{}, error) {
	query := ToolSearch{}
	if err := json.Unmarshal(r.Data, &query); err != nil {
		return nil, fmt.Errorf("could not parse query: %w", err)
	}
	user, err := a.userByEmail(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("could not get user: %w", err)
	}
	tools, err := a.toolSearch(&query, &user.Location)
	if err != nil {
		return nil, fmt.Errorf("could not search tools: %w", err)
	}
	return tools, nil
}

// POST /tools adds a new tool
func (a *API) addToolHandler(r *Request) (interface{}, error) {
	t := Tool{}
	if err := json.Unmarshal(r.Data, &t); err != nil {
		return nil, fmt.Errorf("could not parse tool: %w", err)
	}
	if err := a.addTool(&t, r.UserID); err != nil {
		return nil, fmt.Errorf("could not add tool: %w", err)
	}
	return nil, nil
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
