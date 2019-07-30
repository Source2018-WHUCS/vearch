
curl -v --user "root:secret" -H "content-type: application/json" -XPUT -d'
{
  "name": "ansj"
}
' http://127.0.0.1:8817/db/_create
