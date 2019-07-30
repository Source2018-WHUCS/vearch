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

package document

import (
	"github.com/tiglabs/baudengine/ps/engine/mapping"
	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/baudengine/proto/response"
	. "github.com/tiglabs/baudengine/test"
	"github.com/tiglabs/baudengine/util/assert"
	"github.com/tiglabs/caprice/logger"
	"testing"
)

func TestDeleteAndGetDocument(t *testing.T) {
	dbName, spaceName := "TestDeleteAndGetDocument", "TestDeleteAndGetDocument"

	client := InitSimpleBeginByPartitionNum(dbName, spaceName, 20)

	value := map[string]interface{}{
		"Name":    "ansj",
		"Content": "12312312312",
	}

	docId := "testIDTestDeleteAndGetDocument"

	var (
		doc *response.DocResult
		err error
	)

	_, _ = client.DocumentDelete(docId)

	_, err = client.Flush()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100; i++ {

		_, err = client.DocumentCreate(docId, value)
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.DocumentDelete(docId)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Flush()
		if err != nil {
			t.Fatal(err)
		}
		logger.Info(string(response.Resp))
		doc, err = client.DocumentGet(docId)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Found {
			t.Fatal("del doc but found it err")
		}

		logger.Info("ok not found it")
	}

}

func TestDynamicSchema(t *testing.T) {
	dbName, spaceName := "TestDynamicMapping", "TestDynamicMapping"

	client := InitSimpleBeginByPartitionNum(dbName, spaceName, 20)

	value := map[string]interface{}{
		"Name":    "ansj",
		"Content": "12312312312",
		"int":123,
		"float":1.2345,
		"bool":true,
		"stringArr":[]string{"a","b","c"},
		"intArr":[]int64{1,2,3,4,5,5},

	}

	docId := "testIDTestDeleteAndGetDocument"

	var (
		err error
	)

	_, err = client.Flush()
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DocumentCreate(docId, value)
	if err != nil {
		t.Fatal(err)
	}

	space, err := client.SpaceGet(dbName, spaceName)

	documentMapping, err := mapping.ParseSchema(space.Properties)
	if err != nil {
		t.Fatal(err)
	}

	fm := documentMapping.Properties["Content"]
	assert.Equal(t, fm.Field.FieldType() , pspb.FieldType_TEXT , "err")

	fm = documentMapping.Properties["Name"]
	assert.Equal(t, fm.Field.FieldType() , pspb.FieldType_TEXT , "err")

	fm = documentMapping.Properties["int"]
	assert.Equal(t, fm.Field.FieldType() , pspb.FieldType_FLOAT, "err")

	fm = documentMapping.Properties["float"]
	assert.Equal(t, fm.Field.FieldType() , pspb.FieldType_FLOAT, "err")

	fm = documentMapping.Properties["bool"]
	assert.Equal(t, fm.Field.FieldType() , pspb.FieldType_BOOL, "err")

	fm = documentMapping.Properties["stringArr"]
	assert.Equal(t, fm.Field.FieldType() , pspb.FieldType_TEXT, "err")

	fm = documentMapping.Properties["intArr"]
	assert.Equal(t, fm.Field.FieldType() , pspb.FieldType_FLOAT, "err")


}

func TestDynamicSchemaStringArr(t *testing.T) {
	dbName, spaceName := "TestDynamicSchemaStringArr", "TestDynamicSchemaStringArr"

	client := InitSimpleBeginByPartitionNum(dbName, spaceName, 20)

	value := map[string]interface{}{"whiteList":[]string{"bjwanchuan","zhengjia105","renke8","bjzyhan","cdluxy","hewu7","cdtc","cdtangxiejun"}}

	docId := "TestDynamicSchemaStringArr"

	var (
		err error
	)

	_, err = client.Flush()
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DocumentCreate(docId, value)
	if err != nil {
		t.Fatal(err)
	}

	space, err := client.SpaceGet(dbName, spaceName)

	documentMapping, err := mapping.ParseSchema(space.Properties)
	if err != nil {
		t.Fatal(err)
	}

	fm := documentMapping.Properties["whiteList"]
	assert.Equal(t, fm.Field.FieldType() , pspb.FieldType_TEXT , "err")

}



