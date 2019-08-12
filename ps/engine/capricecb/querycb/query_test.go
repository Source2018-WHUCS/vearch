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

package querycb

import (
	"fmt"
	mapping "github.com/tiglabs/baudengine/ps/engine/mapping"
	"testing"
)

func TestParseQuery(t *testing.T) {

	indexMapping := mapping.NewIndexMapping()

	builder := NewQueryBuilder(indexMapping)

	query := `{
        "filtered":{
            "filter":{
                "bool":{
                    "must":[
                        {
                            "query":{
                                "match":{
                                    "app-name":{
                                        "query":"logbook-api",
                                        "type":"phrase"
                                    }
                                }
                            }
                        },
                        {
                            "range":{
                                "time":{
                                    "format":"epoch_millis",
                                    "gte":1552354269000,
                                    "lte":1552355169000
                                }
                            }
                        }
                    ],
                    "should":[
                        {
                            "bool":{
                                "must":[
                                    {
                                        "query":{
                                            "match":{
                                                "file":{
                                                    "query":"/export/servers/logbook-api/logs/run.log",
                                                    "type":"phrase"
                                                }
                                            }
                                        }
                                    }
                                ],
                                "should":[
                                    {
                                        "query":{
                                            "match":{
                                                "ip":{
                                                    "query":"10.183.29.107",
                                                    "type":"phrase"
                                                }
                                            }
                                        }
                                    }
                                ]
                            }
                        }
                    ],
                    "must_not":[

                    ]
                }
            },
            "query":{
                "query_string":{
                    "query":"exception",
                    "analyze_wildcard":true
                }
            }
        }
    }`

	parseQuery, err := builder.ParseQuery([]byte(query))

	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(parseQuery)
}
