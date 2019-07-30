curl -v --user "root:secret" -H "content-type: application/json" -XGET  "http://127.0.0.1:8817/debug/journal/get?partition_id=1&from=4&size=1"
