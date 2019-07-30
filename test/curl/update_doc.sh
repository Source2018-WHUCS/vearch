
curl -H "content-type: application/json" -H "Authorization: Basic Y2I6MTIzNA==" -XPOST -d '
{
    "doc" : {
        "name" : "new_name"
    }
}
' http://127.0.0.1:9001/ansj/ansj/2/_update