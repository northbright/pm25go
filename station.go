package pm25

type Position struct {
    Latitude float64
    Longitude float64
}

type StationInfo struct {
    Name string
    City string
    Location Position
}

type Station interface {
    save(time string, pm25_hourly_data string, aqi string) (err error)
    GrabData() (err error)
}
