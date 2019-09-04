package document

import (
	"bytes"
	"fmt"
	"github.com/spf13/cast"
	pkg "github.com/vearch/vearch/proto"
	"github.com/vearch/vearch/test"
	"github.com/vearch/vearch/test/testutil"
	"github.com/vearch/vearch/util/cbjson"
	"net/http"
	"testing"
)

func TestDelByQuery(t *testing.T) {
	client := test.InitBulk()

	addData(t, client)

	client.Flush()

	result, e := client.Search(http.MethodGet, "")
	if e != nil {
		t.Fatal(e.Error())
	}

	checkCount(t, result, 1000)


	result, e = client.DeleteByQuery(http.MethodPost, `
	{
		"query": {
			"range" : {
				"value" : {
					"lt" : 50
				}
			}
		}
	}
	`)
	if e != nil {
		t.Fatal(e.Error())
	}

	fmt.Println(string(result.Resp))

	client.Flush()

	result, e = client.Search(http.MethodGet, "")
	if e != nil {
		t.Fatal(e.Error())
	}

	checkCount(t, result, 500)

}

func addData(t *testing.T, client *testutil.CBClient) {

	for i := 0; i < 10; i++ {
		var buffer bytes.Buffer
		for j := 0; j < 100; j++ {
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
				"age":   "10",
				"value": j,
				"name":  "test",
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

func checkCount(t *testing.T, result *testutil.Response, count int) {
	fmt.Println(string(result.Resp))

	jsonMap, err := cbjson.ByteToJsonMap(result.Resp)
	if err != nil {
		t.Fatal(err)
	}

	hitsMap := jsonMap.GetJsonMap("hits")
	if hitsMap == nil {
		code, err := jsonMap.GetJsonValIntE("code")
		if err != nil {
			t.Fatal(err)
		}
		msg, err := jsonMap.GetJsonValStringE("msg")
		if err != nil {
			t.Fatal(err)
		}
		if code != pkg.ERRCODE_SUCCESS {
			t.Fatal(err, msg)
		}
	}

	total, err := hitsMap.GetJsonValIntE("total")
	if err != nil {
		t.Fatal(total, err)
	}
	if total != count {
		t.Fatal("check count err total:", total, "want:", count)
	}
}
