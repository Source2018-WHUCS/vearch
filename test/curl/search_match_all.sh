curl -v -H "content-type: application/json" -H "Authorization: Basic Y2I6MTIzNA==" -XPOST -d '
{
    "query":{
        "match_all":{
            "boost":1
        }
    }
}
' http://127.0.0.1:9001/ansj/ansj/_search?size=1 | python -m json.tool