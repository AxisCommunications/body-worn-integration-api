package main

import "encoding/json"
import "fmt"
import "os"
import "sort"
import "strconv"
import "strings"
import "time"

type CoordinateEntries struct {
	CoordinateEntries []CoordinateEntry
}

type CoordinateEntry struct {
	LocationWKT      LocationWKT
	SecondsFromStart float64
	Timestamp        time.Time
}

type GeoJson struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type Feature struct {
	Type       string   `json:"type"`
	Geometry   Geometry `json:"geometry"`
	Properties Property `json:"properties"`
}

type Geometry struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

type Property struct {
	Prop0 string `json:"prop0"`
}

type StartPos struct {
	Coordinates []float64
}

type LocationWKT struct {
	Long float64
	Lat  float64
}

func convertToGeoJson(inputPath string) (GeoJson, error) {
	c, err := readGvlFile(inputPath)
	if err != nil {
		return GeoJson{}, err
	}
	geometry := Geometry{Type: "LineString"}
	for _, entry := range c.CoordinateEntries {
		geometry.Coordinates = append(geometry.Coordinates, []float64{entry.LocationWKT.Long, entry.LocationWKT.Lat})
	}

	return GeoJson{
		Type: "FeatureCollection",
		Features: []Feature{
			Feature{
				Type:       "Feature",
				Geometry:   geometry,
				Properties: Property{"value0"},
			},
		},
	}, nil
}

func readGvlFile(path string) (*CoordinateEntries, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %v\n", path, err)
	}
	c := CoordinateEntries{}
	err = json.Unmarshal(data, &c)
	if err != nil {
		return nil, fmt.Errorf("error parsing json: %v\n", err)
	}
	sort.Slice(c.CoordinateEntries, func(i, j int) bool {
		return c.CoordinateEntries[i].SecondsFromStart < c.CoordinateEntries[j].SecondsFromStart
	})
	return &c, nil
}

func (w *LocationWKT) UnmarshalJSON(data []byte) error {
	long, lat, err := parseWkt(string(data))
	if err != nil {
		return err
	}
	w.Long = long
	w.Lat = lat
	return nil
}

func parseWkt(wkt string) (longitude, latitude float64, err error) {
	data := strings.Split(wkt, "(")
	if len(data) != 2 {
		return 0, 0, fmt.Errorf("could not parse LocationWKT, unknown format")
	}
	w := strings.TrimSuffix(data[1], ")\"")
	coords := strings.Split(w, " ")
	longitude, err = strconv.ParseFloat(coords[0], 64)
	if err != nil {
		return 0, 0, err
	}
	latitude, err = strconv.ParseFloat(coords[1], 64)
	return longitude, latitude, err
}
