
curl -H "content-type: application/json" -H "Authorization: Basic Y2I6MTIzNA==" -XPOST -d '
{ "index" : { "_index" : "ansj", "_id" : "1" } }
{ "field1" : "value1" }
{ "update" : {"_id" : "1", "_index" : "ansj"} }
{ "doc" : {"field2" : "value2"} }
' http://127.0.0.1:9001/ansj/ansj/_bulk