// Copyright 2018 The Couchbase Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
package mapping

import (
	"encoding/json"
	"fmt"
	"github.com/blevesearch/bleve/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/registry"
	"github.com/tiglabs/log"
	"testing"
	"time"
)

func init()  {
	registry.RegisterAnalyzer(DefaultAnalyzer,standard.AnalyzerConstructor)
}

type Obj struct {
	Text    string  `json:"text_obj"`
	Keyword string  `json:"keyword_obj"`
	Float   float64 `json:"float_obj"`
}

type ObjArray struct {
	Text    string  `json:"text_obj_array"`
	Keyword string  `json:"keyword_obj_array"`
	Float   float64 `json:"float_obj_array"`
}

type Geo struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Doc struct {
	Text          string    `json:"text"`
	Keyword       string    `json:"keyword"`
	Float         float64   `json:"float"`
	Int           int64     `json:"int"`
	Date          string    `json:"date"`
	Bool          bool      `json:"bool"`
	GeoString     string    `json:"geo_string"`
	GeoStringHash string    `json:"geo_string_hash"`
	GeoStruct     Geo       `json:"geo_struct"`
	GeoArray      []float64 `json:"geo_array"`

	// obj
	Obj Obj `json:"obj"`

	// array
	IntArray []int64 `json:"int_array"`

	ObjArray []ObjArray `json:"obj_array"`
}

func TestWalkDocument(t *testing.T) {
	m := NewIndexMapping()
	docMap := NewDocumentMapping()
	docMap.Properties = make(map[string]*DocumentMapping)
	// text
	textFm := NewFieldMapping("text", NewTextFieldMapping("text"))
	docMap.Properties["text"] = &DocumentMapping{Field:textFm}
	// keyword
	keywordFm := NewFieldMapping("keyword", NewKeywordFieldMapping("keyword"))
	docMap.Properties["keyword"] = &DocumentMapping{Field:keywordFm}
	// float
	floatFm := NewFieldMapping("float", NewFloatFieldMapping("float"))
	docMap.Properties["float"] = &DocumentMapping{Field:floatFm}
	// int
	intFm := NewFieldMapping("int", NewIntegerFieldMapping("int"))
	docMap.Properties["int"] = &DocumentMapping{Field:intFm}
	// date
	dateFm := NewFieldMapping("date", NewDateFieldMapping("date"))
	docMap.Properties["date"] = &DocumentMapping{Field:dateFm}
	// boolean
	boolFm := NewFieldMapping("bool", NewBooleanFieldMapping("bool"))
	docMap.Properties["bool"] = &DocumentMapping{Field:boolFm}
	// geo
	geoStringFm := NewFieldMapping("geo_string", NewGeoPointFieldMapping("geo_string"))
	docMap.Properties["geo_string"] = &DocumentMapping{Field:geoStringFm}
	geoStringHashFm := NewFieldMapping("geo_string_hash", NewGeoPointFieldMapping("geo_string_hash"))
	docMap.Properties["geo_string_hash"] = &DocumentMapping{Field:geoStringHashFm}
	geoStructFm := NewFieldMapping("geo_struct", NewGeoPointFieldMapping("geo_struct"))
	docMap.Properties["geo_struct"] = &DocumentMapping{Field:geoStructFm}
	geoArrayFm := NewFieldMapping("geo_array", NewGeoPointFieldMapping("geo_array"))
	docMap.Properties["geo_array"] = &DocumentMapping{Field:geoArrayFm}
	// object field
	objM := NewDocumentMapping()
	textObjFm := NewFieldMapping("text_obj", NewTextFieldMapping("text_obj"))
	docMap.Properties["text_obj"] = &DocumentMapping{Field:textObjFm}
	docMap.Properties["objm"] = objM
	// keyword
	keywordObjFm := NewFieldMapping("keyword_obj", NewTextFieldMapping("keyword_obj"))
	docMap.Properties["keyword_obj"] = &DocumentMapping{Field:keywordObjFm}
	// float
	floatObjFm := NewFieldMapping("float_obj", NewFloatFieldMapping("float_obj"))
	docMap.Properties["float_obj"] = &DocumentMapping{Field:floatObjFm}
	// array field
	intArrayFm := NewFieldMapping("int_array", NewFloatFieldMapping("int_array"))
	docMap.Properties["int_array"] = &DocumentMapping{Field:intArrayFm}
	// array object field
	objArrayM := NewDocumentMapping()
	textObjArrayFm := NewFieldMapping("text_obj_array", NewTextFieldMapping("text_obj_array"))
	docMap.Properties["text_obj_array"] = &DocumentMapping{Field:textObjArrayFm}
	// keyword
	keywordObjArrayFm := NewFieldMapping("keyword_obj_array", NewTextFieldMapping("keyword_obj_array"))
	docMap.Properties["keyword_obj_array"] = &DocumentMapping{Field:keywordObjArrayFm}
	// float
	floatObjArrayFm := NewFieldMapping("float_obj_array", NewFloatFieldMapping("float_obj_array"))
	docMap.Properties["float_obj_array"] = &DocumentMapping{Field:floatObjArrayFm}

	docMap.Properties["obj_array"] = objArrayM
	m.DocumentMapping = docMap
	docStrcut := &Doc{
		Text:          "hello",
		Keyword:       "key",
		Float:         12.3,
		Int:           123,
		Bool:          true,
		Date:          time.Now().UTC().Format(time.RFC3339Nano),
		GeoString:     "41.12,-71.34",
		GeoStringHash: "drm3btev3e86",
		// Geo-point expressed as an array with the format: [ lon, lat]
		GeoArray: []float64{41.12, -71.34},
		GeoStruct: Geo{
			Lat: -71.34,
			Lon: 41.12,
		},

		Obj: Obj{
			Text:    "hello_obj",
			Keyword: "key_obj",
			Float:   0.22,
		},

		IntArray: []int64{1, 2, 3, 4},

		ObjArray: []ObjArray{
			ObjArray{
				Text:    "hello_obj_1",
				Keyword: "key_obj_1",
				Float:   10.1,
			},
			ObjArray{
				Text:    "hello_obj_2",
				Keyword: "key_obj_2",
				Float:   10.2,
			},
		},
	}

	data, err := json.Marshal(docStrcut)
	if err != nil {
		t.Fatal(err)
	}
	fields, newSchema, err := m.MapDocument(data)

	if len(newSchema) > 0 {
		log.Error(fmt.Errorf("new schema err %v", newSchema).Error())
	}

	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fields {
		fmt.Println(f)
	}
}
