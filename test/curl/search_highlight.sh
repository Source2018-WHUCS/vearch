curl -v -H "content-type: application/json" -H "Authorization: Basic Y2I6MTIzNA==" -XPOST -d '
{
    "query":{
        "match":{
            "body":"no"
        }
    },
    "highlight":{
        "pre_tags":[
            "@highlighted-field@"
        ],
        "post_tags":[
            "@/highlighted-field@"
        ],
        "fields":{
            "body":{
            }
        },
        "require_field_match":false,
        "fragment_size":2147483647
    }
}
' http://127.0.0.1:9001/ansj/ansj/_search?size=1 | python -m json.tool
