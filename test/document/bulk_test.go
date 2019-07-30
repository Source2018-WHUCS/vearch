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
    . "github.com/tiglabs/baudengine/test"
    "context"
    "bytes"
    "fmt"
    "github.com/olivere/elastic"
    "github.com/spf13/cast"
    "github.com/tiglabs/baudengine/test/testutil"
    "github.com/tiglabs/baudengine/util/cbjson"
    tigos "github.com/tiglabs/baudengine/util/runtime/os"
    "github.com/tiglabs/log"
    "io/ioutil"
    "os"
    "strings"
    "testing"
    "time"
    "errors"
)

func TestBulk(t *testing.T) {
    client := InitBulk()

    for i := 1; i < 10; i++ {
        var buffer bytes.Buffer
        for j := 1; j < 100; j++ {
            metaIndex := map[string]interface{}{
                "_index": client.DbName,
                "_type":  client.SpaceName,
                "_id":    cast.ToString(i) + ":" + cast.ToString(j),
            }
            meta := map[string]interface{}{
                "index": metaIndex,
            }
            v, _ := cbjson.Marshal(meta)
            buffer.Write(v)
            buffer.WriteString("\n")
            value := map[string]interface{}{
                "age":  "10",
                "name": "test",
            }
            v, _ = cbjson.Marshal(value)
            buffer.Write(v)
            buffer.WriteString("\n")
        }

        resp, err := client.DocumentBulk(buffer.String())
        if err != nil {
            t.Fatal(err)
        }

        jsonMap, err := cbjson.ByteToJsonMap(resp.Resp)
        if err != nil {
            t.Fatal(err)
        }

        items := jsonMap.GetJsonArrMap("items")
        for _, v := range items {
            result := v.GetJsonMap("index").GetJsonValString("result")
            if result != "created" {
                c, _ := cbjson.Marshal(v)
                t.Fatal(fmt.Errorf("bulk error found. result not `created`, (%s)", string(c)))
            }
        }
    }
}


func TestLogbookBulkSuccess(t *testing.T) {
    //client := InitLogbookBegin()
    client := InitBulk()

    logbookESHosts := "http://" + testutil.C().RouterAddr
    esHosts := strings.Split(logbookESHosts, ",")
    esClient, err := elastic.NewClient(elastic.SetURL(esHosts...), elastic.SetHealthcheck(false), elastic.SetSniff(false),elastic.SetBasicAuth(testutil.C().UserName,testutil.C().UserPassword))

    if err != nil {
        fmt.Println(err)
        panic(err)
    }
    defer esClient.Stop()

    pwd := ""
    if currentPath, err := tigos.GetCurrentSourceCodePath(); err != nil {
        panic(err)
    } else {
        pwd = currentPath[0 : strings.LastIndex(currentPath, "/")+1]
    }
    corpusPath := pwd + "../corpus/logbook.json"

    f, err := os.Open(corpusPath)
    if err != nil {
        panic(err)
    }
    out, err := ioutil.ReadAll(f)
    if err != nil {
        panic(err)
    }
    fileContent := string(out)
    s := strings.Split(fileContent, "\n")

    log.Info("test es normal insert doc start.")
    var bulkSvc = esClient.Bulk()

    for i := 0; i < len(s); i++ {
        line := s[i]

        if len(line) < 1 {
            continue
        }

        doc := elastic.NewBulkIndexRequest().Index(client.DbName).Type(client.SpaceName).Doc(line)
        bulkSvc.Add(doc)
    }

    _, err = write(0, bulkSvc)
    if err != nil {
        t.Fatal(err)
    }
}


func TestLogbookBulkNoSpace(t *testing.T) {
    client := InitBulk()

    logbookESHosts := "http://" + testutil.C().RouterAddr
    esHosts := strings.Split(logbookESHosts, ",")
    esClient, err := elastic.NewClient(elastic.SetURL(esHosts...), elastic.SetHealthcheck(false), elastic.SetSniff(false))
    if err != nil {
        fmt.Println(err)
        panic(err)
    }
    defer esClient.Stop()

    pwd := ""
    if currentPath, err := tigos.GetCurrentSourceCodePath(); err != nil {
        panic(err)
    } else {
        pwd = currentPath[0 : strings.LastIndex(currentPath, "/")+1]
    }
    corpusPath := pwd + "../corpus/logbook.json"

    f, err := os.Open(corpusPath)
    if err != nil {
        panic(err)
    }
    out, err := ioutil.ReadAll(f)
    if err != nil {
        panic(err)
    }
    fileContent := string(out)
    s := strings.Split(fileContent, "\n")

    log.Info("test es normal insert doc start.")
    var bulkSvc = esClient.Bulk()

    for i := 0; i < 2; i++ {
        line := s[i]

        if len(line) < 1 {
            continue
        }

        doc := elastic.NewBulkIndexRequest().Index(client.DbName).Type("space_not_exists").Doc(line)
        bulkSvc.Add(doc)
    }

    sucCount, err := write(0, bulkSvc)
    fmt.Println(sucCount)
    if err != nil {
        fmt.Println(err)
    } else {
        t.Fatal(fmt.Errorf("ci test bulk fail（space not exists, so response must return err）."))
    }
}

func write(p int32, bulkSvc *elastic.BulkService) (int64, error) {
    reqTotal := bulkSvc.NumberOfActions()
    ctx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
    response, err := bulkSvc.Do(ctx)
    if err != nil {
        fmt.Errorf("[%v] Push msg into chubao is failed. err:%v\n", p, err)
        return 0, err
    }

    data, _ := cbjson.Marshal(response)
    fmt.Printf("response:%v\n", string(data))
    if response.Errors {
        fails := response.Failed()
        if len(fails) != 0 {
            for _, failed := range fails {
                if failed != nil {
                    fmt.Printf("[%v] error: %v\n", p, failed.Error.Reason)
                }
            }
        }
        // return sucCount
        return int64(reqTotal - len(fails)), errors.New("response return error")
    }

    //fmt.Printf("reqTotal=%v", reqTotal)
    return int64(reqTotal), nil
}