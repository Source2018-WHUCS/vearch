//  Copyright (c) 2014 Couchbase, Inc.
// Modified work copyright (C) 2018 The ChuBao Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mapping

import (
	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/analysis/datetime/optional"
	"github.com/tiglabs/baudengine/proto/entity"
	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/log"
	"github.com/valyala/fastjson"
)

const DefaultField = "_all"
const DefaultAnalyzer = "keyword"
const DefaultTokenizer = "keyword"
const DefaultDateTimeParser = optional.Name

// An IndexMapping controls how objects are placed
// into an index.
// First the type of the object is determined.
// Once the type is know, the appropriate
// DocumentMapping is selected by the type.
// If no mapping was determined for that type,
// a DefaultMapping will be used.
type IndexMapping struct {
	DocumentMapping *DocumentMapping `json:"doc_mapping"`
	//it is not config in index
	DefaultAnalyzerName       string                      `json:"-"`
	DefaultAnalyzer           *analysis.Analyzer          `json:"-"`
	DefaultDateTimeParserName string                      `json:"-"`
	DefaultDateTimeParser     analysis.DateTimeParser     `json:"-"`
	DefaultField              string                      `json:"-"`
	StoreDynamic              bool                        `json:"-"`
	DocValuesDynamic          bool                        `json:"-"`
	DynamicSchema             entity.DynamicType          `json:"-"`
	fieldCacher               map[string]*DocumentMapping `json:"-"`
}

// NewIndexMapping creates a new IndexMapping that will use all the default indexing rules
func NewIndexMapping() *IndexMapping {
	dtp, err := cache.DateTimeParserNamed(DefaultDateTimeParser)
	if err != nil {
		log.Error("init default analyzer err name[%s]", DefaultAnalyzer)
	}
	mapping := &IndexMapping{
		DocumentMapping:       NewDocumentMapping(),
		DefaultDateTimeParser: dtp,
		DefaultField:          DefaultField,
		DynamicSchema:         "true",
		StoreDynamic:          false,
		DocValuesDynamic:      true,
	}
	if err := mapping.SetDefaultAnalyzer(DefaultAnalyzer); err != nil {
		log.Error("init analyzer %v fail cause: %v", DefaultAnalyzer, err)
	}
	return mapping
}

//you can use it like dm.DocumentMappingForField("person.name")
func (im *IndexMapping) GetField(path string) *FieldMapping {
	mapping := im.fieldCacher[path]
	if mapping != nil {
		return mapping.Field
	}
	return nil
}

//you can use it like dm.DocumentMappingForField("person.name")
func (im *IndexMapping) GetDocument(path string) *DocumentMapping {
	return im.fieldCacher[path]
}

//you can use it like dm.DocumentMappingForField("person.name")
func (im *IndexMapping) GetAnalyzerName(path string) string {
	field := im.GetField(path)
	if field != nil {
		if field.FieldMappingI.FieldType() == pspb.FieldType_TEXT {
			return field.FieldMappingI.(*TextFieldMapping).Analyzer
		} else if field.FieldMappingI.FieldType() == pspb.FieldType_KEYWORD {
			return keyword.Name
		} else {
			log.Error("can not cast field:[%s] , type is:[%s], to textField so can not set analyzer", path, field.FieldType())
		}
	}
	return im.DefaultAnalyzerName
}

func (im *IndexMapping) GetAnalyzerByPoint(name *string) (*analysis.Analyzer, error) {
	if name == nil {
		return im.DefaultAnalyzer, nil
	}
	return im.GetAnalyzer(*name)
}

//you can use it like dm.DocumentMappingForField("person.name")
func (im *IndexMapping) GetAnalyzer(name string) (*analysis.Analyzer, error) {
	if name == "" || name == im.DefaultAnalyzerName {
		return im.DefaultAnalyzer, nil
	}
	return cache.AnalyzerNamed(name)
}

func (im *IndexMapping) SetDefaultAnalyzer(name string) (err error) {
	im.DefaultAnalyzer, err = cache.AnalyzerNamed(name)
	if err != nil {
		return
	}
	im.DefaultAnalyzerName = name
	return
}

func (im *IndexMapping) GetDateTimeParser(name string) (analysis.DateTimeParser, error) {
	if name == "" || name == im.DefaultDateTimeParserName {
		return im.DefaultDateTimeParser, nil
	}
	dateTimeParser, err := cache.DateTimeParserNamed(name)
	if err != nil {
		return nil, err
	}
	return dateTimeParser, nil
}

func (im *IndexMapping) SetDefaultDateTimeParse(name string) (err error) {
	im.DefaultDateTimeParser, err = cache.DateTimeParserNamed(name)
	if err != nil {
		return
	}
	im.DefaultDateTimeParserName = name
	return
}

func (im *IndexMapping) InitFieldCache() {
	if im.fieldCacher != nil {
		log.Error("can use this initFieldCache more than once")
		return
	}
	temp := make(map[string]*DocumentMapping)
	_initFieldCache(temp, im.DocumentMapping.Properties, "")
	im.fieldCacher = temp
}

func _initFieldCache(temp map[string]*DocumentMapping, mappings map[string]*DocumentMapping, path string) {
	if mappings == nil {
		return
	}
	if path != "" {
		path += pathSeparator
	}

	for name, dm := range mappings {
		temp[path+name] = dm
		_initFieldCache(temp, dm.Properties, path+name)
	}
}

// wrapper to satisfy new interface
func (im *IndexMapping) DefaultSearchField() string {
	return im.DefaultField
}

type walkContext struct {
	im            *IndexMapping
	dynamic       entity.DynamicType
	copyTo        map[string][]*fastjson.Value
	Fields        []*pspb.Field
	DynamicFields map[string]pspb.FieldType
	Err           error
}

const (
	t entity.DynamicType = "true"
	s entity.DynamicType = "strict"
)

func (ctx *walkContext) isDynamic() bool {
	return ctx.dynamic == t
}

func (ctx *walkContext) isStrict() bool {
	return ctx.dynamic == s
}

func (ctx *walkContext) AddField(f *pspb.Field) {
	ctx.Fields = append(ctx.Fields, f)
}

func (ctx *walkContext) AddDynamicField(f *pspb.Field) {
	if ctx.isDynamic() {
		if ctx.DynamicFields == nil {
			ctx.DynamicFields = make(map[string]pspb.FieldType)
		}
		ctx.DynamicFields[f.Name] = f.Type
	}
}

func (ctx *walkContext) CopyTo(fieldName string, v *fastjson.Value) {
	ctx.copyTo[fieldName] = append(ctx.copyTo[fieldName], v)
}

func (im *IndexMapping) newWalkContext(dynamic entity.DynamicType) *walkContext {
	return &walkContext{
		im:      im,
		dynamic: dynamic,
		copyTo:  make(map[string][]*fastjson.Value),
	}
}

func (im *IndexMapping) GetFieldsAnalyzer() (map[string]*analysis.Analyzer, error) {
	result := make(map[string]*analysis.Analyzer)
	for f, d := range im.fieldCacher {
		if d.Field.FieldType() != pspb.FieldType_TEXT && d.Field.FieldType() != pspb.FieldType_KEYWORD{
			continue
		}
		analyzer, err := im.GetAnalyzer(im.GetAnalyzerName(f))
		if err != nil {
			return nil, err
		}
		result[f] = analyzer
	}
	return result, nil
}

func (im *IndexMapping) GetFieldsType() map[string]pspb.FieldType {
	result := make(map[string]pspb.FieldType)
	for f, fm := range im.fieldCacher {
		result[f] = pspb.FieldType(fm.Field.FieldType())
	}
	return result
}

func (im *IndexMapping) RangeField(f func(key string, value *DocumentMapping) error) error {
	for k, v := range im.fieldCacher {
		if err := f(k, v); err != nil {
			return err
		}
	}
	return nil
}

func Space2Mapping(space *entity.Space) (indexMapping *IndexMapping, err error) {
	var docMapping *DocumentMapping

	docMapping, err = ParseSchema(space.Properties)
	if err != nil {
		return nil, err
	}

	indexMapping = NewIndexMapping()
	indexMapping.DocumentMapping = docMapping
	indexMapping.DynamicSchema = space.DynamicSchema
	if err := indexMapping.SetDefaultAnalyzer(space.DefaultAnalyzer); err != nil {
		return nil, err
	}
	if err := indexMapping.SetDefaultDateTimeParse(space.DefaultDateTimeParser); err != nil {
		return nil, err
	}
	indexMapping.DefaultField = space.DefaultField
	indexMapping.StoreDynamic = space.StoreDynamic
	indexMapping.DocValuesDynamic = *space.DocValuesDynamic
	indexMapping.DynamicSchema = space.DynamicSchema

	indexMapping.InitFieldCache()

	return indexMapping, nil
}
