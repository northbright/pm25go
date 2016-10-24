package pm25

// Position contains GEO position information.
type Position struct {
	Latitude  float64
	Longitude float64
}

// StationInfo contains information of a station.
type StationInfo struct {
	Name     string
	City     string
	Location Position
}

// Station represents one station.
type Station interface {
	save(time string, pm25HourlyData string, aqi string) (err error)
	GrabData() (err error)
}
