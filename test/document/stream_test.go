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

package document

import (
	"bufio"
	"fmt"
	. "github.com/vearch/vearch/test"
	"net/http"
	"testing"
)

func TestLogbookStream(t *testing.T) {
	client := InitLogbookBegin()

	data := `
	{
		"query": {
			"range" : {
				"time" : {
					"gte" : 1553225598000
				}
			}
		}
	}
	`
	response, err := client.StreamSearch(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(response.Body)

	count := 0

	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			t.Fatal(err)
		}
		if len(line) == 0 {
			break
		}
		count++
		fmt.Println(string(line))
	}

	fmt.Println(count)
}
