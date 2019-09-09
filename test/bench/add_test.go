// Copyright 2019 The Vearch Authors.
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

package bench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"testing"
)

var bytesData []byte

func do(id string) {
	client := &http.Client{}

	reader := bytes.NewReader(bytesData)
	req, err := http.NewRequest("POST", "http://cb:1234@127.0.0.1:9001/db1/space1/"+id, reader)
	if err != nil {
		log.Println(err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	defer func() {
		go func() {
			if resp == nil {
				return
			}

			all, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			if resp.StatusCode != 200 {
				fmt.Println(resp.StatusCode, string(all))
			}

			resp.Body.Close()
		}()
	}()

}

func init() {
	//test.InitSimpleBeginBySchema("db1", "space1", `{
	//	"city":{
	//		"type":"keyword"
	//	},
	//	"age":{
	//		"type":"integer"
	//	},
	//	"insert_time":{
	//		"type":"date"
	//	},
	//	"content":{
	//		"type":"text"
	//	}
	//}`)

	song := make(map[string]interface{})
	song["city"] = "Brogan"
	song["age"] = 32
	song["insert_time"] = "1985-08-28"
	song["content"] = "我爱北京"
	var err error
	bytesData, err = json.Marshal(song)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
}

func BenchmarkNoID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		do("")
	}
}

func BenchmarkID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		do(strconv.Itoa(i))
	}
}
