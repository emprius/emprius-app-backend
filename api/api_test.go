package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/notifications/smtp"
	"github.com/emprius/emprius-app-backend/test/mail"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/types"
)

var testLatitudeA = db.NewLocation(
	41688407, // 41.688407 * 1e6
	2491027,  // 2.491027 * 1e6
)

var testLatitudeA200km = db.NewLocation(
	43488407, // 43.488407 * 1e6 (~200km north)
	2491027,  // 2.491027 * 1e6
)

var testUser1 = db.User{
	Name:      "bob",
	Community: "community1",
	Location:  testLatitudeA,
	Active:    true,
	Verified:  true,
	Email:     "bob@emprius.cat",
	Tokens:    1000,
}

var testUser2 = db.User{
	Name:      "alice",
	Community: "community1",
	Location:  testLatitudeA200km,
	Active:    true,
	Verified:  true,
	Email:     "alice@emprius.cat",
	Tokens:    1000,
}

func boolPtr(b bool) *bool {
	return &b
}

func uint64Ptr(i uint64) *uint64 {
	return &i
}

var testTool1 = Tool{
	Title:         "tool1",
	Description:   "tool1 description",
	MayBeFree:     boolPtr(true),
	AskWithFee:    boolPtr(false),
	ToolValuation: uint64Ptr(10000),
	Cost:          10000 / types.FactorCostToPrice,
	Images:        []types.HexBytes{},
	Location: Location{
		Latitude:  41778407, // 41.778407 * 1e6 (~10km north)
		Longitude: 2491027,  // 2.491027 * 1e6
	},
	Category:         1,
	TransportOptions: []int{1, 2},
}

const (
	adminEmail = "admin@test.com"
	adminUser  = "admin"
	adminPass  = "admin123"
)

// testMailService is the test mail service for the tests. Make it global so it
// can be accessed by the tests directly.
var testMailService *smtp.Email

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

	// start test mail server
	testMailServer, err := mail.StartMailService(ctx)
	if err != nil {
		panic(err)
	}
	// get the host, the SMTP port and the API port
	mailHost, err := testMailServer.Host(ctx)
	if err != nil {
		panic(err)
	}
	smtpPort, err := testMailServer.MappedPort(ctx, mail.MailSMTPPort)
	if err != nil {
		panic(err)
	}
	apiPort, err := testMailServer.MappedPort(ctx, mail.MailAPIPort)
	if err != nil {
		panic(err)
	}
	// create test mail service
	testMailService = new(smtp.Email)
	if err := testMailService.New(&smtp.Config{
		FromAddress:  adminEmail,
		SMTPUsername: adminUser,
		SMTPPassword: adminPass,
		SMTPServer:   mailHost,
		SMTPPort:     smtpPort.Int(),
		TestAPIPort:  apiPort.Int(),
	}); err != nil {
		panic(err)
	}

	a, err := New(&APIConfig{
		DB:                 database,
		JwtSecret:          "secret",
		RegisterToken:      "authtoken",
		MaxInviteCodes:     5,
		InviteCodeCooldown: 30,
		MailService:        testMailService,
	})

	qt.Assert(t, err, qt.IsNil)

	return a
}

func TestBookingDateConflicts(t *testing.T) {
	a := testAPI(t)

	// Create users and get their IDs
	_, err := a.addUser(&testUser1) // Tool owner
	qt.Assert(t, err, qt.IsNil)
	user1, err := a.database.UserService.GetUserByEmail(context.Background(), testUser1.Email)
	qt.Assert(t, err, qt.IsNil)

	_, err = a.addUser(&testUser2) // Tool requester
	qt.Assert(t, err, qt.IsNil)
	user2, err := a.database.UserService.GetUserByEmail(context.Background(), testUser2.Email)
	qt.Assert(t, err, qt.IsNil)

	// Create a tool
	toolID, err := a.addTool(&testTool1, user1.ID.Hex())
	qt.Assert(t, err, qt.IsNil)
	toolIDStr := fmt.Sprintf("%d", toolID)

	startDate := time.Now().Add(24 * time.Hour)
	endDate := time.Now().Add(48 * time.Hour)

	// Create first booking request
	booking1 := &db.CreateBookingRequest{
		ToolID:    toolIDStr,
		StartDate: startDate,
		EndDate:   endDate,
		Contact:   "test1@test.com",
		Comments:  "Test booking 1",
	}
	createdBooking1, err := a.database.BookingService.Create(context.Background(), booking1, user2.ID, user1.ID)
	qt.Assert(t, err, qt.IsNil)

	// Create second booking request for same dates (should be allowed since first is pending)
	booking2 := &db.CreateBookingRequest{
		ToolID:    toolIDStr,
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
		ToolID:    toolIDStr,
		StartDate: startDate,
		EndDate:   endDate,
		Contact:   "test3@test.com",
		Comments:  "Test booking 3",
	}
	_, err = a.database.BookingService.Create(context.Background(), booking3, user2.ID, user1.ID)
	qt.Assert(t, err, qt.ErrorMatches, db.ErrBookingDatesConflict.Error())

	// Verify the second booking can still be accepted or rejected
	err = a.database.BookingService.UpdateStatus(context.Background(), createdBooking2.ID, db.BookingStatusRejected)
	qt.Assert(t, err, qt.IsNil)
}

func TestBookingStatusTransitions(t *testing.T) {
	a := testAPI(t)

	// Create users and get their IDs
	id1, err := a.addUser(&testUser1) // Tool owner
	qt.Assert(t, err, qt.IsNil)
	user1, err := a.database.UserService.GetUserByEmail(context.Background(), testUser1.Email)
	qt.Assert(t, err, qt.IsNil)

	// Verify user ID is set correctly
	qt.Assert(t, user1.ID.Hex(), qt.Equals, id1.Hex())

	_, err = a.addUser(&testUser2) // Tool requester
	qt.Assert(t, err, qt.IsNil)
	user2, err := a.database.UserService.GetUserByEmail(context.Background(), testUser2.Email)
	qt.Assert(t, err, qt.IsNil)

	// Create a tool
	toolID, err := a.addTool(&testTool1, user1.ID.Hex())
	qt.Assert(t, err, qt.IsNil)
	toolIDStr := fmt.Sprintf("%d", toolID)

	// Create a booking request
	booking := &db.CreateBookingRequest{
		ToolID:    toolIDStr,
		StartDate: time.Now().Add(24 * time.Hour),
		EndDate:   time.Now().Add(48 * time.Hour),
		Contact:   "test@test.com",
		Comments:  "Test booking",
	}

	// Create booking
	createdBooking, err := a.database.BookingService.Create(context.Background(), booking, user2.ID, user1.ID)
	qt.Assert(t, err, qt.IsNil)

	// Verify toolId is set correctly
	qt.Assert(t, createdBooking.ToolID, qt.Equals, toolIDStr)

	// Get bookings through API endpoints to verify toolId in responses
	page := 0
	pageSize := db.DefaultPageSize
	bookings, _, err := a.database.BookingService.GetUserBookings(
		context.Background(),
		user1.ID,
		db.IncomingBookings,
		page,
		pageSize,
	)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(bookings), qt.Equals, 1)
	qt.Assert(t, bookings[0].ToolID, qt.Equals, toolIDStr)

	bookings, _, err = a.database.BookingService.GetUserBookings(context.Background(), user2.ID, db.OutgoingBookings, page, pageSize)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(bookings), qt.Equals, 1)
	qt.Assert(t, bookings[0].ToolID, qt.Equals, toolIDStr)

	// Test accepting a petition
	err = a.database.BookingService.UpdateStatus(context.Background(), createdBooking.ID, db.BookingStatusAccepted)
	qt.Assert(t, err, qt.IsNil)

	// Verify booking status
	updatedBooking, err := a.database.BookingService.Get(context.Background(), createdBooking.ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, updatedBooking.BookingStatus, qt.Equals, db.BookingStatusAccepted)

	// Create another booking for deny test
	booking2 := &db.CreateBookingRequest{
		ToolID:    toolIDStr,
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
		ToolID:    toolIDStr,
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

func TestHTTPErrorWithErr(t *testing.T) {
	c := qt.New(t)

	baseErr := &HTTPError{
		Code:    400,
		Message: "base error",
	}

	specificErr := fmt.Errorf("specific error details")

	resultErr := baseErr.WithErr(specificErr)

	c.Assert(resultErr.Message, qt.Equals, "base error: specific error details")
	c.Assert(resultErr.Code, qt.Equals, 400)
}

func TestImageErrors(t *testing.T) {
	c := qt.New(t)
	a := testAPI(t)

	// Test empty image data
	_, err := a.addImage("empty", []byte{})
	c.Assert(ErrInvalidImageFormat.IsErr(err), qt.IsTrue)

	// Test invalid image data
	_, err = a.addImage("invalid", []byte("not an image"))
	c.Assert(ErrInvalidImageFormat.IsErr(err), qt.IsTrue)

	// Test invalid image hash
	_, err = a.image([]byte("invalid hash"))
	c.Assert(ErrImageNotFound.IsErr(err), qt.IsTrue)
}

func TestRouterCreation(t *testing.T) {
	c := qt.New(t)
	a := testAPI(t)

	// Verify that the router is created successfully
	router := a.router()
	c.Assert(router, qt.Not(qt.IsNil))

	// Verify that the API instance has all the necessary registration methods
	// This is a compile-time check to ensure the methods exist
	_ = func() {
		r := chi.NewRouter()
		a.RegisterUserRoutes(r)
		a.RegisterToolRoutes(r)
		a.RegisterBookingRoutes(r)
		a.RegisterCommunityRoutes(r)
		a.RegisterImageRoutes(r)
		a.RegisterPublicUserRoutes(r)
		a.RegisterPublicImageRoutes(r)
	}
}
