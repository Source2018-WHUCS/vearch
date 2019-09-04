### create database

````$xslt
curl -v --user "root:secret" -H "content-type: application/json" -XPUT -d'
{
    "name":"tpy"
}
' http://$IP:8817/db/_create
````
## create schema

````$xslt
curl -v --user "root:secret" -H "content-type: application/json" -XPUT -d'
  {
      "name": "tpy",
      "dynamic_schema": "strict",
      "partition_num": 2,
      "replica_num": 1,
      "engine": {"name":"gamma", "max_size":1000000,"nprobe":10,"metric_type":-1,"ncentroids":-1,"nsubvector":-1,"nbits_per_idx":-1},
      "properties": {
          "image_type": {
              "type": "keyword"
          },
          "fku": {
              "type": "integer",
              "index":"false"
          },
          "tags": {
              "type": "keyword",
              "array":true,
              "index":"true"
          },
          "image_vec": {
              "type": "vector",
              "model_id": "img",
              "dimension": 5000
          }
      }
  }
' http://$IP:8817/space/tpy/_create  
````



### insert data

````$xslt
curl -H "content-type: application/json" -XPOST -d'
{
    "area_code":"tpy",
    "product_code":"tpy",
    "image_type":"tpy",
    "image_name":"tpy",
    "image_vec": {
        "source":"test",
        "feature":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
    }
}
' http://11.3.149.73/tpy/tpy/1

````

### Search

````$xslt
# search
curl -H "content-type: application/json" -XPOST -d'

{
  "query": {
      "sum": [
        {
          "field": "feature1",
          "feature": [0,0,0,0,0],
          "boost":0.8,
        },
        {
          "field": "feature2",
          "feature": [0,0,0,0,0],
          "symbol":">=",
          "value":0.9
        }
      ],
      "filter":[
          {
              "range":{
                  "product_code":{
                      "gte":1,
                      "lte":3
                  }
              }
          },
          {
              "term":{
                "tags":["t1","t2"],
                "operator":"and"
              }
          }
       ]
      "direct_search_type":0,
      "online_log_level":"debug" 
  },
  "size":10,
   "sort" : [
       { "_score" : {"order" : "asc"} }
   ]
}
' http://11.3.149.73/tpy/tpy/_search?size=10
````

* filter->term-> operator [`and`, `or`] default `or` 

### delete Document
 
````$xslt
curl -XDELETE http://11.3.149.73/tpy/tpy/1
````