// This file implement the functions to grab pm2.5 data from US embassy.
// Current support cities: Beijing, Chengdu, Guangzhou, Shenyang, Shanghai.

package pm25

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

type USEmbassyStation struct {
	StationInfo
	TwitterId string // twitter id to publish pm2.5 data
}

var DEBUG = true

var usembassyStationJSONData = []string{
	"{\"Name\":\"Beijing US Embassy\", \"City\":\"Beijing\", \"Location\":{\"Latitude\":39.959491, \"Longitude\":116.466354}, \"TwitterId\":\"beijingair\"}",
	"{\"Name\":\"Chengdu US Embassy\", \"City\":\"Chengdu\", \"Location\":{\"Latitude\":30.634367, \"Longitude\":104.068969}, \"TwitterId\":\"cgchengduair\"}",
	"{\"Name\":\"Guangzhou US Embassy\", \"City\":\"Guangzhou\", \"Location\":{\"Latitude\":23.11226, \"Longitude\":113.243954}, \"TwitterId\":\"Guangzhou_Air\"}",
	"{\"Name\":\"Shanghai US Embassy\", \"City\":\"Shanghai\", \"Location\":{\"Latitude\":31.209296, \"Longitude\":121.447202}, \"TwitterId\":\"CGShanghaiAir\"}",
	"{\"Name\":\"Shenyang US Embassy\", \"City\":\"Shenyang\", \"Location\":{\"Latitude\":41.786545, \"Longitude\":123.42622}, \"TwitterId\":\"Shenyang_Air\"}",
}

// Key = city, Value = USEmbassyStation struct pointer
var usembassyStations = make(map[string]*USEmbassyStation)

var mainURL = "https://twitter.com/i/profiles/show/"
var subURL = "/timeline?include_available_features=1&include_entities=1"
var patternHasMoreItems = `"hasMoreItems":(true|false)`
var patternMaxID = `^{"max_id":"(?P<max_id>\d*)"`
var patternHourly = `data-tweet-id=\\"(?P<id>\d*)\\"(.*?)(?P<time>\d{2}-\d{2}-\d{4} \d{2}:\d{2}); PM2\.5; (?P<concentration>\d*\.\d*); (?P<aqi>\d*);`
var patternAvg = `data-tweet-id=\\"(?P<id>\d*)\\"(.*?)(?P<avgstarttime>\d{2}-\d{2}-\d{4} \d{2}:\d{2}) to (?P<avgendtime>\d{2}-\d{2}-\d{4} \d{2}:\d{2}); PM2\.5 24hr avg; (?P<avgconcertration>\d*\.\d*); (?P<avgaqi>\d*);`
var patternAnalyzeHourlyTime = `(?P<month>\d{2})-(?P<date>\d{2})-(?P<year>\d{4}) (?P<hour>\d{2}):(?P<minute>\d{2})`

// Analyze matched string and save the data into leveldb.
func (station USEmbassyStation) save(time string, pm25HourlyData string, aqi string) (err error) {
	return nil
}

// This function grab pm2.5 data from US embassy and save the data into leveldb.
func (station USEmbassyStation) grabData(maxIDStr string) (err error) {
	hasMoreItems := false
	newMaxIDStr := ""
	var newMaxID, maxIDHourly, maxIDAvg int64 = -1, -1, -1

	if len(station.TwitterId) == 0 {
		msg := "TwitterId is empty."
		fmt.Println(msg)
		return errors.New(msg)
	}

	url := mainURL + station.TwitterId + subURL
	if maxIDStr != "" {
		url = fmt.Sprintf("%s&max_id=%s", url, maxIDStr)
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

	// search hasMoreItems
	re := regexp.MustCompile(patternHasMoreItems)
	matched := re.FindStringSubmatch(s)
	if len(matched) == 2 {
		if matched[1] == "true" {
			hasMoreItems = true
		}
	}

	if DEBUG {
		fmt.Printf("hasMoreItems: %v\n", hasMoreItems)
	}

	// search max_id
	re = regexp.MustCompile(patternMaxID)
	matched = re.FindStringSubmatch(s)
	if len(matched) != 0 {
		newMaxIDStr = matched[1]
		fmt.Printf("max_id found: %s\n", newMaxIDStr)
	}

	// hourly pm2.5 data
	re = regexp.MustCompile(patternHourly)
	matchedHourly := re.FindAllStringSubmatch(s, -1)

	for i := 0; i < len(matchedHourly); i++ {
		tm := matchedHourly[i][3]
		// Format time to year-month-date hour.
		re := regexp.MustCompile(patternAnalyzeHourlyTime)
		matched := re.FindStringSubmatch(tm)
		if len(matched) == 0 {
			return errors.New("Time format is incorrect.")
		}
		month := matched[1]
		date := matched[2]
		year := matched[3]
		hour := matched[4]
		newTm := fmt.Sprintf("%s-%s-%s %s", year, month, date, hour) // Time format: year-month-date hour. Ex: 2013-12-01 13

		pm25HourlyData := matchedHourly[i][4]
		aqi := matchedHourly[i][5]

		if DEBUG {
			fmt.Printf("%s pm2.5: %s, aqi: %s\n", newTm, pm25HourlyData, aqi)
		}
		if err = station.save(newTm, pm25HourlyData, aqi); err != nil {
			return err
		}
	}

	// AVG concerntration and aqi of 12 hour
	re = regexp.MustCompile(patternAvg)
	matchedAvg := re.FindAllStringSubmatch(s, -1)

	if newMaxIDStr == "" {
		if len(matchedHourly) > 0 {
			maxIDHourly, _ = strconv.ParseInt(matchedHourly[len(matchedHourly)-1][1], 10, 64)
			fmt.Printf("max_id_single: %d\n", maxIDHourly)
		}

		if len(matchedAvg) > 0 {
			maxIDAvg, _ = strconv.ParseInt(matchedAvg[len(matchedAvg)-1][1], 10, 64)
			fmt.Printf("maxIDAvg: %d\n", maxIDAvg)
		}

		if maxIDHourly != -1 && maxIDAvg != -1 {
			if maxIDHourly < maxIDAvg {
				newMaxID = maxIDHourly - 1
			} else {
				newMaxID = maxIDAvg - 1
			}
		} else if maxIDAvg == -1 {
			// only hourly pm2.5 data
			newMaxID = maxIDHourly
		} else {
			// only avg pm2.5 data
			newMaxID = maxIDAvg
		}

		if newMaxID > 0 {
			newMaxIDStr = fmt.Sprintf("%d", newMaxID)
		}
		fmt.Printf("max_id = min(maxIDHourly, maxIDAvg) - 1 = %s\n", newMaxIDStr)
	}

	// Get more data
	if hasMoreItems {
		time.Sleep(100 * time.Millisecond)
		station.grabData(newMaxIDStr)
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

	for i := 0; i < len(usembassyStationJSONData); i++ {
		s := new(USEmbassyStation)
		err := json.Unmarshal([]byte(usembassyStationJSONData[i]), s)
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
