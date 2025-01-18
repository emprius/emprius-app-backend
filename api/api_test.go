package api

import (
	"context"
	"encoding/base64"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/types"
)

var testLatitudeA = db.Location{
	Latitude:  41688407,
	Longitude: 2491027,
}

var (
	// testLatitudeA10km is a location 10km north and 10km east from LatitudeA
	testLatitudeA10km = db.NewLocation(testLatitudeA, 10, 0)
	// testLatitudeA100km is a location 100km north and 100km east from LatitudeA
	testLatitudeA100km = db.NewLocation(testLatitudeA, 100, 0)
	// testLatitudeA200km is a location 200km north and 200km east from LatitudeA
	testLatitudeA200km = db.NewLocation(testLatitudeA, 200, 0)
)

var testUser1 = db.User{
	Name:      "bob",
	Community: "community1",
	Location:  testLatitudeA,
	Active:    true,
	Verified:  true,
	Email:     "bob@emprius.cat",
}

var testUser2 = db.User{
	Name:      "alice",
	Community: "community1",
	Location:  testLatitudeA200km,
	Active:    true,
	Verified:  true,
	Email:     "alice@emprius.cat",
}

func pngImageForTest() []byte {
	data, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
	)
	if err != nil {
		panic(err)
	}
	return data
}

func boolPtr(b bool) *bool {
	return &b
}

func uint64Ptr(i uint64) *uint64 {
	return &i
}

var testTool1 = Tool{
	Title:            "tool1",
	Description:      "tool1 description",
	MayBeFree:        boolPtr(true),
	AskWithFee:       boolPtr(false),
	EstimatedValue:   10000,
	Cost:             uint64Ptr(10),
	Images:           []types.HexBytes{},
	Location:         testLatitudeA10km,
	Category:         1,
	TransportOptions: []int{1, 2},
}

var testTool2 = Tool{
	Title:            "tool2",
	Description:      "tool2 description",
	MayBeFree:        boolPtr(true),
	AskWithFee:       boolPtr(false),
	EstimatedValue:   5000,
	Cost:             uint64Ptr(5),
	Images:           []types.HexBytes{},
	Location:         testLatitudeA100km,
	Category:         1,
	TransportOptions: []int{1},
}

var testTool3 = Tool{
	Title:            "tool3",
	Description:      "tool3 description",
	MayBeFree:        boolPtr(true),
	AskWithFee:       boolPtr(false),
	EstimatedValue:   5000,
	Cost:             uint64Ptr(5),
	Images:           []types.HexBytes{},
	Location:         testLatitudeA200km,
	Category:         1,
	TransportOptions: []int{1},
}

func testAPI(t *testing.T) *API {
	ctx := context.Background()

	// Start MongoDB container
	container, err := db.StartMongoContainer(ctx)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to start MongoDB container"))
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	// Get MongoDB connection string
	mongoURI, err := container.Endpoint(ctx, "mongodb")
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Failed to get MongoDB connection string"))

	// Create database
	database, err := db.New(mongoURI)
	qt.Assert(t, err, qt.IsNil)
	err = database.CreateTables()
	qt.Assert(t, err, qt.IsNil)

	return New("secret", "authtoken", database)
}

func TestTransportOptions(t *testing.T) {
	a := testAPI(t)

	// Create a test user
	err := a.addUser(&testUser1)
	qt.Assert(t, err, qt.IsNil)

	// Create a tool with transport options
	toolWithTransport := testTool1
	toolWithTransport.TransportOptions = []int{1, 2}

	// Add the tool
	toolID, err := a.addTool(&toolWithTransport, testUser1.Email)
	qt.Assert(t, err, qt.IsNil)

	// Retrieve the tool and verify transport options
	tool, err := a.tool(toolID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(tool.TransportOptions), qt.Equals, 2)
	qt.Assert(t, tool.TransportOptions[0].ID, qt.Equals, int64(1))
	qt.Assert(t, tool.TransportOptions[1].ID, qt.Equals, int64(2))

	// Edit the tool's transport options
	updatedTool := Tool{
		TransportOptions: []int{3},
	}
	err = a.editTool(toolID, &updatedTool)
	qt.Assert(t, err, qt.IsNil)

	// Verify the updated transport options
	tool, err = a.tool(toolID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(tool.TransportOptions), qt.Equals, 1)
	qt.Assert(t, tool.TransportOptions[0].ID, qt.Equals, int64(3))
}

func TestToolSearch(t *testing.T) {
	a := testAPI(t)
	// insert user1 and user2
	err := a.addUser(&testUser1)
	qt.Assert(t, err, qt.IsNil)
	err = a.addUser(&testUser2)
	qt.Assert(t, err, qt.IsNil)

	// insert tools
	_, err = a.addTool(&testTool1, testUser1.Email)
	qt.Assert(t, err, qt.IsNil)
	_, err = a.addTool(&testTool2, testUser2.Email)
	qt.Assert(t, err, qt.IsNil)
	_, err = a.addTool(&testTool3, testUser2.Email)
	qt.Assert(t, err, qt.IsNil)

	// get all tools
	tools, err := a.tools()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, tools, qt.HasLen, 3)

	// search tools for user2
	tools, err = a.toolsByUerID(testUser2.Email)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, tools, qt.HasLen, 2)
	qt.Assert(t, tools[0].Title, qt.Equals, testTool2.Title)
	qt.Assert(t, tools[1].Title, qt.Equals, testTool3.Title)

	// search tools in a radius of 120km from user1
	tools, err = a.toolsByDistance(testUser1.Location, 120000)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, tools, qt.HasLen, 2)
	qt.Assert(t, tools[0].Title, qt.Equals, testTool1.Title)
	qt.Assert(t, tools[1].Title, qt.Equals, testTool2.Title)

	// search tools in a radius of 50km from user1
	tools, err = a.toolsByDistance(testUser1.Location, 50000)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, tools, qt.HasLen, 1)
	qt.Assert(t, tools[0].Title, qt.Equals, testTool1.Title)

	// fetch tool1 by id
	tool, err := a.tool(tools[0].ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, tool.Title, qt.Equals, testTool1.Title)

	// search tools in a radius of 120km from user1 with a max cost of 5000
	tools, err = a.toolSearch(&ToolSearch{
		Distance:  120000,
		MaxCost:   uint64Ptr(5),
		MayBeFree: boolPtr(true),
	}, &testUser1.Location)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, tools, qt.HasLen, 1)
	qt.Assert(t, tools[0].Title, qt.Equals, testTool2.Title)

	// sarch tools with no results
	tools, err = a.toolSearch(&ToolSearch{
		Distance:  240000,
		MaxCost:   uint64Ptr(15),
		MayBeFree: boolPtr(false),
	}, &testUser1.Location)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, tools, qt.HasLen, 0)
}

func TestImage(t *testing.T) {
	a := testAPI(t)

	// insert image
	i, err := a.addImage("image1", pngImageForTest())
	qt.Assert(t, err, qt.IsNil)

	// get image
	image, err := a.image(i.Hash)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, image.Content, qt.DeepEquals, pngImageForTest())
}
