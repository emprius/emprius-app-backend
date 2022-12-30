package db

const (
	LatLongMultiplier = 1000000
)

// DateRange is a type that represents a date range in UNIX format time.
type DateRange struct {
	From uint32 `json:"from"`
	To   uint32 `json:"to"`
}

// Location is a type that represents a location, with latitude and longitude in microdegrees.
type Location struct {
	Latitude  int64 `json:"latitude"`
	Longitude int64 `json:"longitude"`
}
