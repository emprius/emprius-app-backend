package api

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.mongodb.org/mongo-driver/bson/primitive"

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

func TestToolAvailability(t *testing.T) {
	a := testAPI(t)

	// Create a test user
	err := a.addUser(&testUser1)
	qt.Assert(t, err, qt.IsNil)

	// Create a tool
	tool := testTool1
	toolID, err := a.addTool(&tool, testUser1.Email)
	qt.Assert(t, err, qt.IsNil)

	// Verify tool is available by default
	dbTool, err := a.tool(toolID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, dbTool.IsAvailable, qt.IsTrue)

	// Update tool to be unavailable
	updatedTool := Tool{
		IsAvailable: boolPtr(false),
	}
	err = a.editTool(toolID, &updatedTool)
	qt.Assert(t, err, qt.IsNil)

	// Verify tool is now unavailable
	dbTool, err = a.tool(toolID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, dbTool.IsAvailable, qt.IsFalse)
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

	// search tools by transport options
	tools, err = a.toolSearch(&ToolSearch{
		Distance:         120000,
		TransportOptions: []int{2},
	}, &testUser1.Location)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, tools, qt.HasLen, 1)
	qt.Assert(t, tools[0].Title, qt.Equals, testTool1.Title)

	// sarch tools with no results
	tools, err = a.toolSearch(&ToolSearch{
		Distance:  240000,
		MaxCost:   uint64Ptr(15),
		MayBeFree: boolPtr(false),
	}, &testUser1.Location)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, tools, qt.HasLen, 0)
}

func TestBookingDateConflicts(t *testing.T) {
	a := testAPI(t)

	// Create users
	err := a.addUser(&testUser1) // Tool owner
	qt.Assert(t, err, qt.IsNil)
	err = a.addUser(&testUser2) // Tool requester
	qt.Assert(t, err, qt.IsNil)

	// Create a tool
	toolID, err := a.addTool(&testTool1, testUser1.Email)
	qt.Assert(t, err, qt.IsNil)
	toolObjID := primitive.NewObjectIDFromTimestamp(time.Unix(toolID, 0))

	// Get user IDs
	user1, err := a.database.UserService.GetUserByEmail(context.Background(), testUser1.Email)
	qt.Assert(t, err, qt.IsNil)
	user2, err := a.database.UserService.GetUserByEmail(context.Background(), testUser2.Email)
	qt.Assert(t, err, qt.IsNil)

	startDate := time.Now().Add(24 * time.Hour)
	endDate := time.Now().Add(48 * time.Hour)

	// Create first booking request
	booking1 := &db.CreateBookingRequest{
		ToolID:    toolObjID,
		StartDate: startDate,
		EndDate:   endDate,
		Contact:   "test1@test.com",
		Comments:  "Test booking 1",
	}
	createdBooking1, err := a.database.BookingService.Create(context.Background(), booking1, user2.ID, user1.ID)
	qt.Assert(t, err, qt.IsNil)

	// Create second booking request for same dates (should be allowed since first is pending)
	booking2 := &db.CreateBookingRequest{
		ToolID:    toolObjID,
		StartDate: startDate,
		EndDate:   endDate,
		Contact:   "test2@test.com",
		Comments:  "Test booking 2",
	}
	createdBooking2, err := a.database.BookingService.Create(context.Background(), booking2, user2.ID, user1.ID)
	qt.Assert(t, err, qt.IsNil)

	// Accept first booking
	err = a.database.BookingService.UpdateStatus(context.Background(), createdBooking1.ID, db.BookingStatusAccepted)
	qt.Assert(t, err, qt.IsNil)

	// Try to create third booking for same dates (should fail since there's an accepted booking)
	booking3 := &db.CreateBookingRequest{
		ToolID:    toolObjID,
		StartDate: startDate,
		EndDate:   endDate,
		Contact:   "test3@test.com",
		Comments:  "Test booking 3",
	}
	_, err = a.database.BookingService.Create(context.Background(), booking3, user2.ID, user1.ID)
	qt.Assert(t, err, qt.ErrorMatches, "booking dates conflict with existing booking")

	// Verify the second booking can still be accepted or rejected
	err = a.database.BookingService.UpdateStatus(context.Background(), createdBooking2.ID, db.BookingStatusRejected)
	qt.Assert(t, err, qt.IsNil)
}

func TestBookingStatusTransitions(t *testing.T) {
	a := testAPI(t)

	// Create users
	err := a.addUser(&testUser1) // Tool owner
	qt.Assert(t, err, qt.IsNil)
	err = a.addUser(&testUser2) // Tool requester
	qt.Assert(t, err, qt.IsNil)

	// Create a tool and convert its ID to ObjectID
	toolID, err := a.addTool(&testTool1, testUser1.Email)
	qt.Assert(t, err, qt.IsNil)
	toolObjID := primitive.NewObjectIDFromTimestamp(time.Unix(toolID, 0))

	// Create a booking request
	booking := &db.CreateBookingRequest{
		ToolID:    toolObjID,
		StartDate: time.Now().Add(24 * time.Hour),
		EndDate:   time.Now().Add(48 * time.Hour),
		Contact:   "test@test.com",
		Comments:  "Test booking",
	}

	// Get user IDs
	user1, err := a.database.UserService.GetUserByEmail(context.Background(), testUser1.Email)
	qt.Assert(t, err, qt.IsNil)
	user2, err := a.database.UserService.GetUserByEmail(context.Background(), testUser2.Email)
	qt.Assert(t, err, qt.IsNil)

	// Create booking
	createdBooking, err := a.database.BookingService.Create(context.Background(), booking, user2.ID, user1.ID)
	qt.Assert(t, err, qt.IsNil)

	// Verify toolId is set correctly
	qt.Assert(t, createdBooking.ToolID, qt.Equals, toolObjID)

	// Get bookings through API endpoints to verify toolId in responses
	bookings, err := a.database.BookingService.GetUserRequests(context.Background(), user1.ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(bookings), qt.Equals, 1)
	qt.Assert(t, bookings[0].ToolID, qt.Equals, toolObjID)

	bookings, err = a.database.BookingService.GetUserPetitions(context.Background(), user2.ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(bookings), qt.Equals, 1)
	qt.Assert(t, bookings[0].ToolID, qt.Equals, toolObjID)

	// Test accepting a petition
	err = a.database.BookingService.UpdateStatus(context.Background(), createdBooking.ID, db.BookingStatusAccepted)
	qt.Assert(t, err, qt.IsNil)

	// Verify booking status
	updatedBooking, err := a.database.BookingService.Get(context.Background(), createdBooking.ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, updatedBooking.BookingStatus, qt.Equals, db.BookingStatusAccepted)

	// Create another booking for deny test
	booking2 := &db.CreateBookingRequest{
		ToolID:    toolObjID,
		StartDate: time.Now().Add(72 * time.Hour),
		EndDate:   time.Now().Add(96 * time.Hour),
		Contact:   "test@test.com",
		Comments:  "Test booking 2",
	}
	createdBooking2, err := a.database.BookingService.Create(context.Background(), booking2, user2.ID, user1.ID)
	qt.Assert(t, err, qt.IsNil)

	// Test denying a petition
	err = a.database.BookingService.UpdateStatus(context.Background(), createdBooking2.ID, db.BookingStatusRejected)
	qt.Assert(t, err, qt.IsNil)

	// Verify booking status
	updatedBooking2, err := a.database.BookingService.Get(context.Background(), createdBooking2.ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, updatedBooking2.BookingStatus, qt.Equals, db.BookingStatusRejected)

	// Create another booking for cancel test
	booking3 := &db.CreateBookingRequest{
		ToolID:    toolObjID,
		StartDate: time.Now().Add(120 * time.Hour),
		EndDate:   time.Now().Add(144 * time.Hour),
		Contact:   "test@test.com",
		Comments:  "Test booking 3",
	}
	createdBooking3, err := a.database.BookingService.Create(context.Background(), booking3, user2.ID, user1.ID)
	qt.Assert(t, err, qt.IsNil)

	// Test canceling a request
	err = a.database.BookingService.UpdateStatus(context.Background(), createdBooking3.ID, db.BookingStatusCancelled)
	qt.Assert(t, err, qt.IsNil)

	// Verify booking status
	updatedBooking3, err := a.database.BookingService.Get(context.Background(), createdBooking3.ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, updatedBooking3.BookingStatus, qt.Equals, db.BookingStatusCancelled)
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
