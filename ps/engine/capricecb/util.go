// Copyright 2018 The ChuBao Authors.
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

package capricecb

import (
	"fmt"
	"github.com/tiglabs/baudengine/proto/response"
	"github.com/tiglabs/caprice/util"
	"github.com/tiglabs/log"
	"time"

	"encoding/json"

	"github.com/pkg/errors"
	"github.com/tiglabs/baudengine/ps/engine/mapping"
	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/baudengine/util/bytes"
	"github.com/tiglabs/caprice"
	"github.com/tiglabs/caprice/document"
	"github.com/tiglabs/caprice/search/match"
)

func (ce *capriceEngine) Document2DocResult(dm *match.DocumentMatch, document *document.Document) *response.DocResult {
	result := &response.DocResult{
		Id:        document.DocID(),
		Found:     true,
		DB:        ce.space.DBId,
		Space:     ce.space.Id,
		Partition: ce.partitionID,
	}

	if field, ok := document.GetField(mapping.VersionField); ok {
		if v, ok := field.Value().(int64); !ok {
			result.Failure = response.NewEngineErr(errors.Errorf("invalid field value %v", field.Value()))
		} else {
			result.Version = int64(v)
		}
	}

	if field, ok := document.GetField(mapping.SlotField); ok {
		if v, ok := field.Value().(int64); !ok {
			result.Failure = response.NewEngineErr(errors.Errorf("invalid field value %v", field.Value()))
		} else {
			result.SlotID = uint32(v)
		}
	}

	if field, ok := document.GetField(mapping.SourceField); ok {
		if v, ok := field.Value().([]byte); !ok {
			result.Failure = response.NewEngineErr(errors.Errorf("invalid field value %v", field.Value()))
		} else {
			result.Source = json.RawMessage(v)
		}
	}

	result.Found = true

	result.Score = dm.Score
	result.SortValues = dm.Sort

	if dm.Expl != nil {
		result.Expl = dm.Expl
	}

	return result
}

func DocCmd2Document(docCmd *pspb.DocCmd) (*document.Document, error) {
	doc := document.NewDocument(docCmd.DocId)

	if docCmd.Version <= 0 {
		docCmd.Version = 1
	}

	//version
	doc.AddField(document.NewIntFieldWithIndexingOptions(mapping.VersionField, docCmd.Version, document.StoreField))
	//slot
	doc.AddField(document.NewIntFieldWithIndexingOptions(mapping.SlotField, int64(docCmd.Slot), document.StoreField))
	//source
	if docCmd.Source != nil {
		doc.AddField(document.NewTextFieldWithIndexingOptions(mapping.SourceField, docCmd.Source, document.StoreField))
	}

	fields := make(map[string]bool, len(docCmd.Fields))
	for _, f := range docCmd.Fields {
		var field document.Field
		if f.Value == nil {
			return nil, errors.New("miss field value")
		}
		//add field_name
		if !fields[f.Name] {
			doc.AddField(document.NewKeywordField(mapping.FieldNamesField, bytes.StringToByte(f.Name)))
			fields[f.Name] = true
		}

		switch f.Type {
		case pspb.FieldType_TEXT:
			field = document.NewTextFieldWithIndexingOptions(f.Name, []byte(f.Value.Text), document.IndexingOptions(f.Option))
		case pspb.FieldType_KEYWORD:
			field = document.NewKeywordFieldWithIndexingOptions(f.Name, []byte(f.Value.Text), document.IndexingOptions(f.Option))
		case pspb.FieldType_FLOAT:
			field = document.NewFloatFieldWithIndexingOptions(f.Name, f.Value.Float, document.IndexingOptions(f.Option))
		case pspb.FieldType_INT:
			field = document.NewIntFieldWithIndexingOptions(f.Name, f.Value.Int, document.IndexingOptions(f.Option))
		case pspb.FieldType_DATE:
			var err error
			if f.Value.Time == nil {
				return nil, errors.New("miss date field value")
			}
			date := time.Unix(f.Value.Time.Sec, f.Value.Time.Usec)
			field, err = document.NewDateTimeFieldWithIndexingOptions(f.Name, date, document.IndexingOptions(f.Option))
			if err != nil {
				return nil, err
			}
		case pspb.FieldType_BOOL:
			field = document.NewBooleanFieldWithIndexingOptions(f.Name, f.Value.Bool, document.IndexingOptions(f.Option))
		case pspb.FieldType_GEOPOINT:
			if f.Value.Geo == nil {
				return nil, errors.New("miss geo point field value")
			}
			field = document.NewGeoPointFieldWithIndexingOptions(f.Name, f.Value.Geo.Lon, f.Value.Geo.Lat, document.IndexingOptions(f.Option))
		default:
			log.Debug("caprice invalid field type %v", f.Type)
		}
		if field != nil {
			doc.AddField(field)
		}
	}

	return doc, nil
}

func (ce *capriceEngine) DocCmd2DocResult(docCmd *pspb.DocCmd) *response.DocResult {
	return &response.DocResult{
		Id:        docCmd.DocId,
		DB:        ce.space.DBId,
		Space:     ce.space.Id,
		Found:     true,
		Partition: ce.partitionID,
		Version:   docCmd.Version,
		SlotID:    docCmd.Slot,
		Type:      docCmd.Type,
	}
}

func SearchResult2SearchResponse(result *caprice.SearchResult, hits response.Hits) *response.SearchResponse {
	return &response.SearchResponse{
		Status:   result.Status,
		Hits:     hits,
		Total:    result.Total,
		MaxScore: result.MaxScore,
		Aggs:     result.Aggregators,
	}
}

func ReadFieldValue(mp *mapping.IndexMapping, doc *document.Document, field string) (result []*pspb.Field, err error) {

	fvs, found := doc.GetFields(field)

	result = make([]*pspb.Field, 0, 3)

	if found {
		for _, f := range fvs {
			myField := &pspb.Field{Value: new(pspb.FieldValue)}
			switch f.Type() {
			case document.Field_Type_Int:
				myField.Type = pspb.FieldType_INT
				myField.Value.Int = f.Value().(int64)
			case document.Field_Type_Float:
				myField.Type = pspb.FieldType_FLOAT
				myField.Value.Float = f.Value().(float64)
			case document.Field_Type_Bool:
				myField.Type = pspb.FieldType_BOOL
				myField.Value.Bool = f.Value().(bool)
			case document.Field_Type_Date:
				myField.Type = pspb.FieldType_DATE
				myField.Value.Time = &pspb.TimeStamp{Usec: f.Value().(time.Time).UnixNano()}
			case document.Field_Type_Geo:
				myField.Type = pspb.FieldType_GEOPOINT
				loc := f.Value().([]float64)
				myField.Value.Geo = &pspb.Geo{Lon: loc[0], Lat: loc[1]}
			case document.Field_Type_Text:
				myField.Type = pspb.FieldType_TEXT
				myField.Value.Text = f.Value().(string)
			case document.Field_Type_Keyword:
				myField.Type = pspb.FieldType_KEYWORD
				myField.Value.Text = f.Value().(string)
			case document.Field_Type_Nil:
				myField.Type = pspb.FieldType_NULL
			default:
				return nil, fmt.Errorf("unknow field type file:[%v] type:[%s]", f, f.Type())
			}
			result = append(result, myField)
		}
		return result, nil
	}

	fv, found := doc.GetField(mapping.SourceField)
	if found {
		source := string(fv.Value().([]byte))
		fields, _, err := mp.MapDocument(util.StringToByte(source))
		if err != nil {
			return nil, err
		}
		for _, f := range fields {
			if f.Name == field {
				result = append(result, f)
			}
		}
	}
	return result, nil
}
