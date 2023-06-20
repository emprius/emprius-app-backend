package api

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/emprius/emprius-app-backend/db"
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
	Images:           [][]byte{},
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
	Images:           [][]byte{},
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
	Images:           [][]byte{},
	Location:         testLatitudeA200km,
	Category:         1,
	TransportOptions: []int{1},
}

func testAPI(t *testing.T) *API {
	database, err := db.New(":memory:")
	qt.Assert(t, err, qt.IsNil)
	err = database.CreateTables()
	qt.Assert(t, err, qt.IsNil)
	return New("secret", "authtoken", database)
}

func TestToolSearch(t *testing.T) {
	a := testAPI(t)
	// insert user1 and user2
	err := a.addUser(&testUser1)
	qt.Assert(t, err, qt.IsNil)
	err = a.addUser(&testUser2)
	qt.Assert(t, err, qt.IsNil)

	// insert tools
	err = a.addTool(&testTool1, testUser1.Email)
	qt.Assert(t, err, qt.IsNil)
	err = a.addTool(&testTool2, testUser2.Email)
	qt.Assert(t, err, qt.IsNil)
	err = a.addTool(&testTool3, testUser2.Email)
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
	i, err := a.addImage("image1", []byte("image1"))
	qt.Assert(t, err, qt.IsNil)

	// get image
	image, err := a.image(i.Hash)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, image.Content, qt.DeepEquals, []byte("image1"))
}
