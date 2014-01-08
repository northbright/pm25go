// This file implement the functions to grab pm2.5 data from US embassy.
// Current support cities: Beijing, Chengdu, Guangzhou, Shenyang, Shanghai.

package pm25

import (
    "errors"
    "fmt"
    "net/http"
    "io/ioutil"
    "regexp"
    "strconv"
    "time"
    "encoding/json"
)

type USEmbassyStation struct {
    StationInfo
    TwitterId string // twitter id to publish pm2.5 data
}

var DEBUG = true

var usembassy_station_json_data = []string{
    "{\"Name\":\"Beijing US Embassy\", \"City\":\"Beijing\", \"Location\":{\"Latitude\":39.959491, \"Longitude\":116.466354}, \"TwitterId\":\"beijingair\"}",
    "{\"Name\":\"Chengdu US Embassy\", \"City\":\"Chengdu\", \"Location\":{\"Latitude\":30.634367, \"Longitude\":104.068969}, \"TwitterId\":\"cgchengduair\"}",
    "{\"Name\":\"Guangzhou US Embassy\", \"City\":\"Guangzhou\", \"Location\":{\"Latitude\":23.11226, \"Longitude\":113.243954}, \"TwitterId\":\"Guangzhou_Air\"}",
    "{\"Name\":\"Shanghai US Embassy\", \"City\":\"Shanghai\", \"Location\":{\"Latitude\":31.209296, \"Longitude\":121.447202}, \"TwitterId\":\"CGShanghaiAir\"}",
    "{\"Name\":\"Shenyang US Embassy\", \"City\":\"Shenyang\", \"Location\":{\"Latitude\":41.786545, \"Longitude\":123.42622}, \"TwitterId\":\"Shenyang_Air\"}",
}

// Key = city, Value = USEmbassyStation struct pointer
var usembassyStations = make(map[string]*USEmbassyStation)

var main_url = "https://twitter.com/i/profiles/show/"
var sub_url = "/timeline?include_available_features=1&include_entities=1"
var pattern_has_more_items = `"has_more_items":(true|false)`
var pattern_max_id = `^{"max_id":"(?P<max_id>\d*)"`
var pattern_hourly = `data-tweet-id=\\"(?P<id>\d*)\\"(.*?)(?P<time>\d{2}-\d{2}-\d{4} \d{2}:\d{2}); PM2\.5; (?P<concentration>\d*\.\d*); (?P<aqi>\d*);`
var pattern_avg = `data-tweet-id=\\"(?P<id>\d*)\\"(.*?)(?P<avgstarttime>\d{2}-\d{2}-\d{4} \d{2}:\d{2}) to (?P<avgendtime>\d{2}-\d{2}-\d{4} \d{2}:\d{2}); PM2\.5 24hr avg; (?P<avgconcertration>\d*\.\d*); (?P<avgaqi>\d*);`
var pattern_analyze_hourly_time = `(?P<month>\d{2})-(?P<date>\d{2})-(?P<year>\d{4}) (?P<hour>\d{2}):(?P<minute>\d{2})`

// Analyze matched string and save the data into leveldb.
func (station USEmbassyStation) save(time string, pm25_hourly_data string, aqi string) (err error) {
    return nil
}

// This function grab pm2.5 data from US embassy and save the data into leveldb.
func (station USEmbassyStation) grabData(max_id_str string) (err error) {
    has_more_items := false
    new_max_id_str := ""
    var new_max_id, max_id_hourly, max_id_avg int64 = -1, -1, -1

    if len(station.TwitterId) == 0 {
        msg := "TwitterId is empty."
        fmt.Println(msg)
        return errors.New(msg)
    }

    url := main_url + station.TwitterId + sub_url
    if max_id_str != "" {
        url = fmt.Sprintf("%s&max_id=%s", url, max_id_str)
    }

    if DEBUG {
        fmt.Printf("url: %s\n", url)
    }

    res, err := http.Get(url)
    defer res.Body.Close()
    if err != nil {
        fmt.Println(err)
        return err
    }

    body, err := ioutil.ReadAll(res.Body)
    if err != nil {
        fmt.Println(err)
        return err
    }

    s := string(body)

    // search has_more_items
    re := regexp.MustCompile(pattern_has_more_items)
    matched := re.FindStringSubmatch(s)
    if len(matched) == 2 {
        if matched[1] == "true" {
            has_more_items = true
        }
    }

    if DEBUG {
        fmt.Printf("has_more_items: %v\n", has_more_items)
    }

    // search max_id
    re = regexp.MustCompile(pattern_max_id)
    matched = re.FindStringSubmatch(s)
    if len(matched) != 0 {
        new_max_id_str = matched[1]
        fmt.Printf("max_id found: %s\n", new_max_id_str)
    }

    // hourly pm2.5 data
    re = regexp.MustCompile(pattern_hourly)
    matchedHourly := re.FindAllStringSubmatch(s, -1)

    for i := 0; i < len(matchedHourly); i++ {
        tm := matchedHourly[i][3]
        // Format time to year-month-date hour.
        re := regexp.MustCompile(pattern_analyze_hourly_time)
        matched := re.FindStringSubmatch(tm)
        if len(matched) == 0 {
            return errors.New("Time format is incorrect.")
        }
        month := matched[1]
        date := matched[2]
        year := matched[3]
        hour := matched[4]
        new_tm := fmt.Sprintf("%s-%s-%s %s", year, month, date, hour)  // Time format: year-month-date hour. Ex: 2013-12-01 13

        pm25_hourly_data := matchedHourly[i][4]
        aqi := matchedHourly[i][5]

        if DEBUG {
            fmt.Printf("%s pm2.5: %s, aqi: %s\n", new_tm, pm25_hourly_data, aqi)
        }
        if err = station.save(new_tm, pm25_hourly_data, aqi); err != nil {
            return err
        }
    }

    // AVG concerntration and aqi of 12 hour
    re = regexp.MustCompile(pattern_avg)
    matchedAvg := re.FindAllStringSubmatch(s, -1)

    if new_max_id_str == "" {
        if len(matchedHourly) > 0 {
            max_id_hourly, _ = strconv.ParseInt(matchedHourly[len(matchedHourly) - 1][1], 10, 64)
            fmt.Printf("max_id_single: %d\n", max_id_hourly)
        }

        if len(matchedAvg) > 0 {
            max_id_avg, _ = strconv.ParseInt(matchedAvg[len(matchedAvg) - 1][1], 10, 64)
            fmt.Printf("max_id_avg: %d\n", max_id_avg)
        }

        if max_id_hourly != -1 && max_id_avg != -1 {
            if max_id_hourly < max_id_avg {
                new_max_id = max_id_hourly - 1
            }else {
                new_max_id = max_id_avg - 1
            }
        }else if max_id_avg == -1 {
            // only hourly pm2.5 data
            new_max_id = max_id_hourly
        }else {
            // only avg pm2.5 data
            new_max_id = max_id_avg
        }

        if new_max_id > 0 {
            new_max_id_str = fmt.Sprintf("%d", new_max_id)
        }
        fmt.Printf("max_id = min(max_id_hourly, max_id_avg) - 1 = %s\n", new_max_id_str)
    }

    // Get more data
    if has_more_items {
        time.Sleep(100 * time.Millisecond)
        station.grabData(new_max_id_str)
    }

    fmt.Println("No more items found. Exit.")

    return nil
}

// Grab PM2.5 data.
func (station USEmbassyStation) GrabData() (err error) {
    return station.grabData("")
}

// return USEmbassyStation by city name.
func GetUSEmbassyStation(city string) (err error, station *USEmbassyStation) {
    if _, ok := usembassyStations[city]; !ok {
        return errors.New("No such city."), &USEmbassyStation{}
    }

    return nil, usembassyStations[city]
}

func init() {
    fmt.Println("Init()")

    for i :=0; i < len(usembassy_station_json_data); i++ {
        s := new(USEmbassyStation)
        err := json.Unmarshal([]byte(usembassy_station_json_data[i]), s)
        if err != nil {
            fmt.Println(err)
        }

        usembassyStations[s.City] = s
    }

    if DEBUG {
        for k, v := range usembassyStations {
            fmt.Printf("k: %s, v: %v\n", k, v)
        }
    }
}
