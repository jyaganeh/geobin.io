package main

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"sync"
	"testing"

	"github.com/bmizerany/assert"
)

func init() {
	// make the default for isDebug be true when running tests. If you run `go test -debug=false`
	// the tests will not print out the debug info.
	*isDebug = true
}

// Request tests

func testSlicesContainSameGeos(t *testing.T, a, b []Geo) {
	// If they aren't the same length, they don't have the same contents
	assert.Tf(t, len(a) == len(b), "Slices weren't the same length: \n%# v\n%# v\n", a, b)

Equal:
	for _, aVal := range a {
		for _, bVal := range b {
			if reflect.DeepEqual(aVal, bVal) {
				// found it, move along.
				continue Equal
			}
		}

		// We didn't find this one, fail the test and return
		t.Fatalf("Expected to find:\n%# v\n in results but did not.", aVal)
		return
	}
}

func testSlicesContainSameItems(t *testing.T, a, b []interface{}) {
	// If they aren't the same length, they don't have the same contents
	assert.T(t, len(a) == len(b))

Equal:
	for _, aVal := range a {
		for _, bVal := range b {
			if reflect.DeepEqual(aVal, bVal) {
				// found it, move along.
				continue Equal
			}
		}

		// We didn't find this one, fail the test and return
		t.Fatalf("Expected to find:\n%# v\n in results but did not.", aVal)
		return
	}
}

func TestRequestWithSingleObject(t *testing.T) {
	src := []byte(`{ "type": "Point", "coordinates": [100, 0] }`)

	expected := []interface{}{
		map[string]interface{}{
			"geo": map[string]interface{}{
				"type":        "Point",
				"coordinates": []interface{}{float64(100), float64(0)},
			},
			"path": make([]interface{}, 0),
		},
	}

	gr := NewGeobinRequest(0, nil, src)

	var got []interface{}
	gotBytes, _ := json.Marshal(gr.Geo)
	json.Unmarshal(gotBytes, &got)

	assert.Equal(t, expected, got)
}

func TestRequestWithMultipleObjects(t *testing.T) {
	src := []byte(`[
		{ "type": "Point", "coordinates": [100, 0] },
		{ "type": "Point", "coordinates": [0, 100] }
	]`)

	expected := []interface{}{
		map[string]interface{}{
			"geo": map[string]interface{}{
				"type":        "Point",
				"coordinates": []interface{}{float64(100), float64(0)},
			},
			"path": []interface{}{float64(0)},
		},
		map[string]interface{}{
			"geo": map[string]interface{}{
				"type":        "Point",
				"coordinates": []interface{}{float64(0), float64(100)},
			},
			"path": []interface{}{float64(1)},
		},
	}

	gr := NewGeobinRequest(0, nil, src)

	var got []interface{}
	gotBytes, _ := json.Marshal(gr.Geo)
	json.Unmarshal(gotBytes, &got)

	testSlicesContainSameItems(t, expected, got)
}

// Parsing tests

func TestParse(t *testing.T) {
	runTest := func(input interface{}, expected []Geo) {
		gr := GeobinRequest{
			wg: &sync.WaitGroup{},
			c:  make(chan Geo),
		}

		go func() {
			for {
				geo, ok := <-gr.c
				if !ok {
					debugLog("channel closed")
					return
				}

				gr.Geo = append(gr.Geo, geo)
				gr.wg.Done()
			}
		}()
		gr.parse(input, make([]interface{}, 0))
		gr.wg.Wait()
		close(gr.c)

		testSlicesContainSameGeos(t, expected, gr.Geo)
	}

	singleObject := map[string]interface{}{
		"x": float64(1),
		"y": float64(-1),
	}
	expected := []Geo{
		Geo{
			Geo: map[string]interface{}{
				"type":        "Point",
				"coordinates": []interface{}{singleObject["x"], singleObject["y"]},
			},
			Path: make([]interface{}, 0),
		},
	}
	debugLog("TestParse - singleObject")
	runTest(singleObject, expected)

	multipleObjects := []interface{}{
		map[string]interface{}{
			"x": float64(1),
			"y": float64(-1),
		},
		map[string]interface{}{
			"x": float64(2),
			"y": float64(-2),
		},
	}
	expected = []Geo{
		Geo{
			Geo: map[string]interface{}{
				"type": "Point",
				"coordinates": []interface{}{
					multipleObjects[0].(map[string]interface{})["x"],
					multipleObjects[0].(map[string]interface{})["y"]},
			},
			Path: []interface{}{0},
		},
		Geo{
			Geo: map[string]interface{}{
				"type": "Point",
				"coordinates": []interface{}{
					multipleObjects[1].(map[string]interface{})["x"],
					multipleObjects[1].(map[string]interface{})["y"]},
			},
			Path: []interface{}{1},
		},
	}
	debugLog("TestParse - multipleObjects")
	runTest(multipleObjects, expected)
}

func TestParseArray(t *testing.T) {
	verboseLog("TestParseArray")
	gr := GeobinRequest{
		wg: &sync.WaitGroup{},
		c:  make(chan Geo),
	}

	inputs := make([]interface{}, 0)
	expected := make([]Geo, 0)
	// Create 5 inputs and their equivalent expected outputs
	for i := 0; i < 5; i++ {
		inputs = append(inputs, map[string]interface{}{
			"x": float64(i),
			"y": float64(-i),
		})

		expected = append(expected, Geo{
			Geo: map[string]interface{}{
				"type": "Point",
				"coordinates": []interface{}{
					inputs[i].(map[string]interface{})["x"],
					inputs[i].(map[string]interface{})["y"],
				},
			},
			Path: []interface{}{i},
		})
	}

	go func() {
		for {
			geo, ok := <-gr.c
			if !ok {
				debugLog("channel closed")
				return
			}

			gr.Geo = append(gr.Geo, geo)
			gr.wg.Done()
		}
	}()
	gr.parseArray(inputs, make([]interface{}, 0))
	gr.wg.Wait()
	close(gr.c)

	testSlicesContainSameGeos(t, expected, gr.Geo)
}

func TestParseObject(t *testing.T) {
	runTest := func(input map[string]interface{}, expected []Geo, name string) {
		debugLog("TestParseObject -", name)
		gr := GeobinRequest{
			wg: &sync.WaitGroup{},
			c:  make(chan Geo),
		}
		gr.wg.Add(1)
		go func() {
			for {
				geo, ok := <-gr.c
				if !ok {
					return
				}

				gr.Geo = append(gr.Geo, geo)
				gr.wg.Done()
			}
		}()
		gr.parseObject(input, make([]interface{}, 0))
		gr.wg.Wait()
		close(gr.c)

		testSlicesContainSameGeos(t, expected, gr.Geo)
	}

	geoJson := map[string]interface{}{
		"type":        "Point",
		"coordinates": []interface{}{float64(1), float64(-1)},
	}
	runTest(geoJson, []Geo{
		Geo{
			Geo:  geoJson,
			Path: make([]interface{}, 0),
		},
	}, "geoJson")

	otherGeo := map[string]interface{}{
		"x": float64(2),
		"y": float64(-2),
	}
	runTest(otherGeo, []Geo{
		Geo{
			Geo: map[string]interface{}{
				"type":        "Point",
				"coordinates": []interface{}{otherGeo["x"], otherGeo["y"]},
			},
			Path: make([]interface{}, 0),
		},
	}, "otherGeo")

	nested := map[string]interface{}{
		"foo": "bar",
		"x":   1,
		"baz": map[string]interface{}{
			"geo": map[string]interface{}{
				"x": float64(3),
				"y": float64(-3),
			},
		},
	}
	runTest(nested, []Geo{
		Geo{
			Geo: map[string]interface{}{
				"type":        "Point",
				"coordinates": []interface{}{float64(3), float64(-3)},
			},
			Path: []interface{}{"baz", "geo"},
		},
	}, "nested")

	nestedInArray := map[string]interface{}{
		"foo": "bar",
		"geos": []interface{}{
			map[string]interface{}{
				"x": float64(40),
				"y": float64(-40),
			},
			map[string]interface{}{
				"x": float64(41),
				"y": float64(-41),
			},
		},
	}
	runTest(nestedInArray, []Geo{
		Geo{
			Geo: map[string]interface{}{
				"type":        "Point",
				"coordinates": []interface{}{float64(40), float64(-40)},
			},
			Path: []interface{}{"geos", 0},
		},
		Geo{
			Geo: map[string]interface{}{
				"type":        "Point",
				"coordinates": []interface{}{float64(41), float64(-41)},
			},
			Path: []interface{}{"geos", 1},
		},
	}, "nestedInArray")
}

// Geo Detection tests

func runIsOtherGeoTest(t *testing.T, o map[string]interface{}, shouldFind bool, exp *Geo) {
	res, got := isOtherGeo(o)
	assert.Equal(t, res, shouldFind)
	assert.Equal(t, exp, got)
}

// Ensure that we find lat and long values for all of the variations of the
// key names
func TestIsOtherGeoLatLngKeys(t *testing.T) {
	latKeys := []string{"lat", "latitude", "y"}
	lngKeys := []string{"lng", "lon", "long", "longitude", "x"}

	// For every combination of latKeys and lngKeys, we expect the same result
	exp := Geo{
		Geo: map[string]interface{}{
			"type":        "Point",
			"coordinates": []interface{}{float64(10), float64(-10)},
		},
	}

	debugLog("===Begin TestIsOtherGeoLatLngKeys===")
	for _, latKey := range latKeys {
		for _, lngKey := range lngKeys {
			runIsOtherGeoTest(t, map[string]interface{}{
				latKey: float64(-10),
				lngKey: float64(10),
			}, true, &exp)
		}
	}
	debugLog("===End TestIsOtherGeoLatLngKeys===")
}

// Ensure that we find dist values for all variations of the key name
func TestIsOtherGeoDistKeys(t *testing.T) {
	distKeys := []string{"dst", "dist", "distance", "rad", "radius", "acc", "accuracy"}

	// For each distKey, we expect the same result
	exp := Geo{
		Geo: map[string]interface{}{
			"type":        "Point",
			"coordinates": []interface{}{float64(10), float64(-10)},
		},
		Radius: float64(5),
	}

	debugLog("===Begin TestIsOtherGeoDistKeys===")
	for _, distKey := range distKeys {
		runIsOtherGeoTest(t, map[string]interface{}{
			"x":     float64(10),
			"y":     float64(-10),
			distKey: float64(5),
		}, true, &exp)
	}
	debugLog("===End TestIsOtherGeoDistKeys===")
}

// Ensure that we find geo objects for all variations of the key name.
func TestIsOtherGeoGeoKeys(t *testing.T) {
	geoKeys := []string{"geo", "loc", "location", "coord", "coordinate", "coords", "coordinates"}

	// For each geoKey we expect the same result
	exp := Geo{
		Geo: map[string]interface{}{
			"type":        "Point",
			"coordinates": []interface{}{float64(10), float64(-10)},
		},
	}

	debugLog("===Begin TestIsOtherGeoGeoKeys===")
	for _, geoKey := range geoKeys {
		runIsOtherGeoTest(t, map[string]interface{}{
			geoKey: []float64{10, -10},
		}, true, &exp)
	}
	debugLog("===End TestIsOtherGeoGeoKeys===")
}

// TODO: Move these tests to a geojson specific test file within the geojson lib
// that we write.
func TestIsGeojsonPoint(t *testing.T) {
	// Valid
	assert.T(t, isGeojson(map[string]interface{}{
		"type":        "Point",
		"coordinates": []float64{1, -1},
	}))

	// Invalid
	assert.T(t, !isGeojson(map[string]interface{}{
		"type":        "Point",
		"coordinates": "psyche!",
	}))
	/*
		Our current geojson lib sucks and says these are all valid. Will rewrite.
		assert.T(t, !isGeojson(map[string]interface{}{
			"type": "Point",
			"coordinates": []float64{1,2,3},
		}))
		assert.T(t, !isGeojson(map[string]interface{}{
			"type":        "Point",
			"coordinates": []float64{360, 360}, // invalid lat/long
		}))
	*/
}

/* Ignore these for now, will move to lib
func TestIsGeoJsonMultiPoint(t *testing.T) {
	// Valid
	assert.T(t, isGeojson(map[string]interface{}{
		"type": "MultiPoint",
		"coordinates": [][]float64{
			[]float64{1, -1},
			[]float64{2, -2},
			[]float64{3, -3},
		},
	}))

	// Invalid
	assert.T(t, !isGeojson(map[string]interface{}{
		"type":        "MultiPoint",
		"coordinates": []float64{0, 0},
	}))
}

func TestIsGeoJsonLineString(t *testing.T) {
	// Valid
	assert.T(t, isGeojson(map[string]interface{}{
		"type": "LineString",
		"coordinates": [][]float64{
			[]float64{0, 0},
			[]float64{1, 1},
			[]float64{2, 2},
		},
	}))

	// Invalid
	assert.T(t, !isGeojson(map[string]interface{}{
		"type":        "LineString",
		"coordinates": []float64{0, 0}, // coords should be an array of two or more arrays of 2 floats
	}))
	assert.T(t, !isGeojson(map[string]interface{}{
		"type":        "LineString",
		"coordinates": [][]float64{[]float64{0, 0}},
	}))
}
*/

// Other Geo Tests

func TestGTCallbackRequest(t *testing.T) {
	expected := []Geo{
		Geo{
			Geo: map[string]interface{}{
				"type":        "Point",
				"coordinates": []interface{}{-122.67545711249113, 45.51986460661744},
			},
			Radius: 8,
			Path:   []interface{}{"location"},
		},
		Geo{
			Geo: map[string]interface{}{
				"type":        "Point",
				"coordinates": []interface{}{-122.77545711249113, 45.41986460661744},
			},
			Path: []interface{}{"trigger", "condition", "geo"},
		},
	}

	js, err := ioutil.ReadFile("gtCallback.json")
	if err != nil {
		t.Error("Error reading gtCallback.json.", err)
		return
	}

	gr := NewGeobinRequest(0, nil, js)

	testSlicesContainSameGeos(t, expected, gr.Geo)
}
