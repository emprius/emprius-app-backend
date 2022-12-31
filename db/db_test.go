package db

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/types"
)

var (
	image1 = Image{
		Hash:    []byte("hash1"),
		Name:    "image1",
		Content: []byte("content1"),
		Link:    "http://image1.com",
	}
	image2 = Image{
		Hash:    []byte("hash2"),
		Name:    "image2",
		Content: []byte("content2"),
		Link:    "http://image2.com",
	}

	transport1 = Transport{
		ID:   1,
		Name: "transport1",
	}

	transport2 = Transport{
		ID:   2,
		Name: "transport2",
	}

	dateRange1 = DateRange{
		From: 123,
		To:   456,
	}

	dateRange2 = DateRange{
		From: 789,
		To:   1011,
	}

	location1 = Location{ // 41.688407, 2.491027 (Sant Celoni)
		Latitude:  41688407,
		Longitude: 2491027,
	}

	// distance from location1 is 50 km
	location2 = Location{ // 41.749846, 1.825959 (Manresa)
		Latitude:  41749846,
		Longitude: 1825959,
	}

	// at 24km from location2 and 35km from location1
	location3 = Location{ // 41.809433, 2.096000 (Moia)
		Latitude:  41809433,
		Longitude: 2096000,
	}

	tool1 = Tool{
		ID:               1,
		Title:            "tool1",
		Description:      "description1",
		IsAvailable:      true,
		MayBeFree:        true,
		AskWithFee:       false,
		Cost:             10,
		UserID:           2,
		Images:           []Image{image1, image2},
		TransportOptions: []Transport{transport1, transport2},
		ToolCategory:     1,
		Location:         location1,
		Rating:           50,
		EstimatedValue:   100,
		Height:           20,
		Weight:           30,
		ReservedDates:    []DateRange{dateRange1, dateRange2},
	}

	tool2 = Tool{
		ID:               2,
		Title:            "tool2",
		Description:      "description2",
		IsAvailable:      false,
		MayBeFree:        false,
		AskWithFee:       true,
		Cost:             20,
		UserID:           3,
		Images:           []Image{image1},
		TransportOptions: []Transport{transport2},
		ToolCategory:     2,
		Location:         location2,
		Rating:           75,
		EstimatedValue:   200,
		Height:           25,
		Weight:           35,
		ReservedDates:    []DateRange{dateRange1},
	}
)

func TestInsertAndRetrieveImage(t *testing.T) {
	db, err := New(":memory:")
	qt.Assert(t, err, qt.IsNil)
	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
	}()

	err = createImageTable(db)
	qt.Assert(t, err, qt.IsNil)

	// insert image1
	err = db.Exec("INSERT INTO image VALUES ?", &image1)
	qt.Assert(t, err, qt.IsNil)

	// retrieve image1
	var retrievedImage Image
	result, err := db.Query("SELECT * FROM image WHERE hash = ?", image1.Hash)
	qt.Assert(t, err, qt.IsNil)
	err = result.Iterate(func(d types.Document) error { return document.StructScan(d, &retrievedImage) })
	qt.Assert(t, err, qt.IsNil)

	// compare retrieved image with original image
	qt.Assert(t, image1, qt.DeepEquals, retrievedImage, qt.Commentf("image: %+v", retrievedImage))
}

func TestInsertAndRetrieveTransport(t *testing.T) {
	db, err := New(":memory:")
	qt.Assert(t, err, qt.IsNil)
	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
	}()

	err = createTransportTables(db)
	qt.Assert(t, err, qt.IsNil)

	// insert transport1
	err = db.Exec("INSERT INTO transport VALUES ?", &transport1)
	qt.Assert(t, err, qt.IsNil)

	// retrieve transport1
	var retrievedTransport Transport
	result, err := db.Query("SELECT * FROM transport WHERE name = ?", transport1.Name)
	qt.Assert(t, err, qt.IsNil)
	err = result.Iterate(func(d types.Document) error { return document.StructScan(d, &retrievedTransport) })
	qt.Assert(t, err, qt.IsNil)

	// compare retrieved transport with original transport
	qt.Assert(t, transport1, qt.DeepEquals, retrievedTransport)
}

func TestInsertAndRetrieveTool(t *testing.T) {
	db, err := New(":memory:")
	qt.Assert(t, err, qt.IsNil)
	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
	}()

	err = createToolTables(db)
	qt.Assert(t, err, qt.IsNil)

	// insert tool1
	err = db.Exec("INSERT INTO tool VALUES ?", &tool1)
	qt.Assert(t, err, qt.IsNil)

	// retrieve tool1
	var retrievedTool Tool
	result, err := db.Query("SELECT * FROM tool WHERE id = ?", tool1.ID)
	qt.Assert(t, err, qt.IsNil)
	err = result.Iterate(func(d types.Document) error { return document.StructScan(d, &retrievedTool) })
	qt.Assert(t, err, qt.IsNil)

	// compare retrieved tool with original tool
	qt.Assert(t, tool1, qt.DeepEquals, retrievedTool)
}

func TestInsertAndRetrieveMultipleImages(t *testing.T) {
	db, err := New(":memory:")
	qt.Assert(t, err, qt.IsNil)
	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
	}()

	err = createImageTable(db)
	qt.Assert(t, err, qt.IsNil)

	// insert image1 and image2
	err = db.Exec("INSERT INTO image VALUES ?, ?", &image1, &image2)
	qt.Assert(t, err, qt.IsNil)

	// retrieve image1 and image2
	var retrievedImages []Image
	result, err := db.Query("SELECT * FROM image")
	qt.Assert(t, err, qt.IsNil)
	err = result.Iterate(func(d types.Document) error {
		var retrievedImage Image
		err := document.StructScan(d, &retrievedImage)
		if err != nil {
			return err
		}
		retrievedImages = append(retrievedImages, retrievedImage)
		return nil
	})
	qt.Assert(t, err, qt.IsNil)

	// compare retrieved images with original images
	qt.Assert(t, len(retrievedImages), qt.Equals, 2)
	qt.Assert(t, retrievedImages[0], qt.DeepEquals, image1)
	qt.Assert(t, retrievedImages[1], qt.DeepEquals, image2)
}

func TestInsertAndRetrieveToolWithMinimalFields(t *testing.T) {
	db, err := New(":memory:")
	qt.Assert(t, err, qt.IsNil)
	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
	}()

	err = createToolTables(db)
	qt.Assert(t, err, qt.IsNil)

	// insert tool2 with minimal fields
	tool2Minimal := Tool{
		ID:    2,
		Title: "tool2",
	}
	err = db.Exec("INSERT INTO tool VALUES ?", &tool2Minimal)
	qt.Assert(t, err, qt.IsNil)

	// retrieve tool2
	var retrievedTool Tool
	result, err := db.Query("SELECT * FROM tool WHERE id = ?", tool2Minimal.ID)
	qt.Assert(t, err, qt.IsNil)
	err = result.Iterate(func(d types.Document) error { return document.StructScan(d, &retrievedTool) })
	qt.Assert(t, err, qt.IsNil)

	// compare retrieved tool with original tool
	qt.Assert(t, tool2Minimal, qt.DeepEquals, retrievedTool)
}

func TestUpdateTool(t *testing.T) {
	db, err := New(":memory:")
	qt.Assert(t, err, qt.IsNil)
	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
	}()

	err = createToolTables(db)
	qt.Assert(t, err, qt.IsNil)

	// insert tool1
	err = db.Exec("INSERT INTO tool VALUES ?", &tool1)
	qt.Assert(t, err, qt.IsNil)

	// retrieve tool1
	var retrievedTool Tool
	result, err := db.Query("SELECT * FROM tool WHERE id = ?", tool1.ID)
	qt.Assert(t, err, qt.IsNil)
	err = result.Iterate(func(d types.Document) error { return document.StructScan(d, &retrievedTool) })
	qt.Assert(t, err, qt.IsNil)

	// compare retrieved tool with original tool
	qt.Assert(t, tool1, qt.DeepEquals, retrievedTool)

	// update tool1
	tool1.Title = "updated title"
	err = db.Exec("UPDATE tool SET title = ? WHERE id = ?", tool1.Title, tool1.ID)
	qt.Assert(t, err, qt.IsNil)

	// retrieve updated tool1
	result, err = db.Query("SELECT * FROM tool WHERE id = ?", tool1.ID)
	qt.Assert(t, err, qt.IsNil)
	err = result.Iterate(func(d types.Document) error { return document.StructScan(d, &retrievedTool) })
	qt.Assert(t, err, qt.IsNil)

	// compare retrieved tool with updated tool
	qt.Assert(t, tool1, qt.DeepEquals, retrievedTool)
}

func TestToolSearchByLocation(t *testing.T) {
	db, err := New(":memory:")
	qt.Assert(t, err, qt.IsNil)
	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
	}()

	err = createToolTables(db)
	qt.Assert(t, err, qt.IsNil)

	// insert tool1
	err = db.Exec("INSERT INTO tool VALUES ?", &tool1)
	qt.Assert(t, err, qt.IsNil)

	// insert tool2
	err = db.Exec("INSERT INTO tool VALUES ?", &tool2)
	qt.Assert(t, err, qt.IsNil)

	locationOfUser := location3

	// search for tools within range of 26km (should be tool2 only)
	rangeInMeters := 26000
	query := "SELECT * FROM tool"
	result, err := db.Query(query)
	qt.Assert(t, err, qt.IsNil)
	var retrievedTools []Tool
	err = result.Iterate(func(d types.Document) error {
		var t Tool
		err := document.StructScan(d, &t)
		if err != nil {
			return err
		}
		if withinCircumference(t.Location, locationOfUser, rangeInMeters) {
			retrievedTools = append(retrievedTools, t)
		}
		return nil
	})
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, len(retrievedTools), qt.Equals, 1)
	qt.Assert(t, retrievedTools[0], qt.DeepEquals, tool2)

	// search for tools within range of 40km (should be tool1 and tool2)
	rangeInMeters = 40000
	query = "SELECT * FROM tool"
	result, err = db.Query(query)
	qt.Assert(t, err, qt.IsNil)
	retrievedTools = []Tool{}
	err = result.Iterate(func(d types.Document) error {
		var t Tool
		err := document.StructScan(d, &t)
		if err != nil {
			return err
		}
		if withinCircumference(t.Location, locationOfUser, rangeInMeters) {
			retrievedTools = append(retrievedTools, t)
		}
		return nil
	})
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, len(retrievedTools), qt.Equals, 2)
}
