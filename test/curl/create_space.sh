
curl -v --user "root:secret" -H "content-type: application/json" -XPUT -d'
{
    "name": "ansj",
    "properties": {

        "title":{
            "type":"string"
        },
        "val":{
            "type":"integer"
        },
        "body":{
            "type":"string"
        }

    }
}
' http://127.0.0.1:8817/space/ansj/_create
