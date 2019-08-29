

### insert data with plugin

````$xslt
curl -H "content-type: application/json" -XPOST -d'
{
    "area_code":"tpy",
    "product_code":"tpy",
    "image_type":"tpy",
    "image_name":"tpy",
    "tags":["t1","t2","t3"],
    "image_vec": {
        "source":"http://www.xxxx.com/abc.jpg",
        "model":"vgg16"
    }
}
' http://11.3.149.73/tpy/tpy/1

````



### Search with plugin

````$xslt
# search
curl -H "content-type: application/json" -XPOST -d'

{
  "query": {
      "sum": [
        {
          "field": "feature1",
          "source":"http://www.xxxx.com/abc.jpg",
          "model":"vgg16"
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
