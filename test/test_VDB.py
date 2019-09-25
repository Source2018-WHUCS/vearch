# -*- coding: UTF-8 -*-

import logging
import pytest
import requests
import json
logging.basicConfig()
logger = logging.getLogger(__name__)

__author__ = 'wangjiangjuan'
__date__ = '2019-07-22 09:25:00'
__description__ = """ """

ip_db = "11.3.170.164:443"
ip_data = "11.3.170.164:80"
db_name = "test_vector_db"
space_name = "vector_space"


@pytest.mark.author('')
@pytest.mark.level(2)
@pytest.mark.cover(["VDB"])
def test_stats():
    logger.info("_cluster_information")
    url = "http://" + ip_db + "/_cluster/stats"
    response = requests.get(url)
    print("cluster_stats:" + response.text)
    assert response.status_code == 200

def test_health():
    url = "http://" + ip_db + "/_cluster/health"
    response = requests.get(url)
    print("cluster_health---\n" + response.text)
    assert response.status_code == 200

def test_server():
    url = "http://" + ip_db + "/list/server"
    response = requests.get(url)
    print("list_server---\n" + response.text)
    assert response.status_code == 200

def test_db():
    url = "http://" + ip_db + "/list/db"
    response = requests.get(url)
    print("list_db---\n" + response.text)
    assert response.status_code == 200

def test_createDB():
    logger.info("------------")
    url = "http://" + ip_db + "/db/_create"
    headers = {"content-type": "application/json"}
    data = {
        'name':db_name
    }
    response = requests.put(url, headers=headers, data=json.dumps(data))
    print("db_create---\n" + response.text)
    assert response.status_code == 200

def test_dbsearch():
    url = "http://" + ip_db + "/db/" + db_name
    response = requests.get(url)
    print("db_search---\n" + response.text)
    assert response.status_code == 200

def test_dbspace():
    url = "http://" + ip_db + "/list/space?db=" + db_name
    response = requests.get(url)
    print("space_search---\n" + response.text)
    assert response.status_code == 200

def test_createspace():
    url = "http://" + ip_db + "/space/" + db_name +"/_create"
    headers = {"content-type": "application/json"}
    data = {
        "name": space_name,
        "dynamic_schema": "strict",
        "partition_num": 2, #"partition_num": 2-6之间
        "replica_num": 1,
        "engine": {"name":"gamma", "index_size":8192, "max_size":10000},
        "properties": {
            "string": {
                "type" : "keyword",
                "index" : "true"
            },
            "int": {
                "type": "integer",
                "index" : "true"
            },
            "float": {
                "type": "float",
                "index" : "true"
            },
            "vector": {
                "type": "vector",
                "model_id": "img",
                "dimension": 128
            },
            "string_tags": {
                "type": "string",
                "array": True,
                "index" : "true"
            },
            "int_tags": {
                "type": "integer",
                "array": True,
                "index" : "true"
            },
            "float_tags" : {
                "type": "float",
                "array": True,
                "index" : "true"
            }
        },
        "models": [{
            "model_id": "vgg16",
            "fields": ["string"],
            "out": "feature"
        }]
    }
    print(url+"---"+json.dumps(data))
    response = requests.put(url, headers=headers, data=json.dumps(data))
    print("space_create---\n" + response.text)
    assert response.status_code == 200

def test_space():
    url = "http://" + ip_db + "/space/"+db_name+"/" + space_name
    response = requests.get(url)
    print("space---\n" + response.text)
    assert response.status_code == 200

logger.info("router(PS)")
def test_insertWithId():
    logger.info("insert")
    headers = {"content-type": "application/json"}
    fileData = "D:/tool/vectorbase/test/data/test1.json"
    with open(fileData, "r") as dataLine1:
        for dataLine in dataLine1:
            print(dataLine)
            idStr = dataLine.split(',', 1)[0].replace('{', '')
            id = eval(idStr.split(':')[1])
            data = "{"+dataLine.split(',', 1)[1]
            print("_id:" + id)
            print("_data:" + data)
            url = "http://" + ip_data + "/" + db_name + "/" + space_name + "/" + id
            response = requests.post(url, headers=headers, data=data)
            print("insertWithID:" + response.text)
            assert response.status_code == 200

def test_searchById():
    logger.info("test_searchById")
    fileData = "D:/tool/vectorbase/test/data/test1.json"
    with open(fileData, "r") as dataLine1:
        for dataLine in dataLine1:
            idStr = dataLine.split(',', 1)[0].replace('{', '')
            id = eval(idStr.split(':')[1])
            print("_id:" + id)
            url = "http://" + ip_data + "/" + db_name + "/" + space_name + "/" + id
            response = requests.get(url)
            print("searchById:" + response.text)
            assert response.status_code == 200

def test_insterNoId():
    logger.info("insertDataNoId")
    headers = {"content-type": "application/json"}
    fileData = "D:/tool/vectorbase/test/data/test1.json"
    with open(fileData, "r") as dataLine1:
        for dataLine in dataLine1:
            idStr = dataLine.split(',', 1)[0].replace('{', '')
            id = eval(idStr.split(':')[1])
            data = "{"+dataLine.split(',', 1)[1]
            url = "http://" + ip_data + "/" + db_name + "/" + space_name
            response = requests.post(url, headers=headers, data=data)
            print("insertNoID:" + response.text)
            assert response.status_code == 200

def test_searchByFeature():
    headers = {"content-type": "application/json"}
    url = "http://" + ip_data + "/"+db_name+"/"+space_name+"/_search?size=100"
    fileData = "D:/tool/vectorbase/test/data/test1.json"
    with open(fileData, "r") as dataLine1:
        for dataLine in dataLine1:
            print(dataLine)
            idStr = dataLine.split(',', 1)[0].replace('{', '')
            id = eval(idStr.split(':')[1])
            feature = "{"+dataLine.split(',', 1)[1]
            print("_id:" + id)
            print("_data:" + feature)
            feature = json.loads(feature)
            feature = feature["vector"]["feature"]
            data = {
                "query": {
                    "sum" :[{
                        "field": "feature",
                        "feature": feature
                    }]
                }
            }
            print(json.dumps(data))
            response = requests.post(url, headers=headers, data=json.dumps(data))
            print("searchByFeature---\n" + response.text)
            assert response.status_code == 200

def test_searchByFeatureandFilter():
    url = "http://" + ip_data + "/"+db_name+"/"+space_name+"/_search"
    headers = {"content-type": "application/json"}
    fileData = "data1.txt"
    fileFeature = "feature1.txt"
    with open(fileData, "r") as dataLine1, open(fileFeature,"r") as fileFeature1:
        for dataLine in dataLine1:
            feature = fileFeature1.readline()
            feature = feature.replace('(', '[')
            feature = feature.replace(')', ']')
            list = dataLine.split("\t")
            fid3 = list[4]
            data = {
                "query": {
                    "filter": [{
                        "fid3": fid3
                    }],
                    "sum" :[{
                        "field": "feature",
                        "feature": feature
                    }]
                }
            }
            data["query"]["sum"][0]["feature"] = json.loads(data["query"]["sum"][0]["feature"])
            response = requests.post(url, headers=headers, data=json.dumps(data))
            print("searchByFeatureandFilter:" + response.text)
            assert response.status_code == 200

def test_updateDoc():
    logger.info("updateDoc")
    headers = {"content-type": "application/json"}
    fileData = "D:/tool/vectorbase/test/data/test1.json"
    with open(fileData, "r") as dataLine1:
        for dataLine in dataLine1:
            idStr = dataLine.split(',', 1)[0].replace('{', '')
            id = eval(idStr.split(':')[1])
            data = "{"+dataLine.split(',', 1)[1]
            url = "http://" + ip_data + "/" + db_name + "/" + space_name + "/" + id
            response = requests.post(url, headers=headers, data=data)
            print("updateDoc:" + response.text)
            assert response.status_code == 200

def test_insertBulk():
    logger.info("insertBulk")
    url = "http://" + ip_data + "/"+db_name+"/"+space_name+"/_bulk"
    headers = {"content-type": "application/json"}
    fileData = "D:/tool/vectorbase/test/data/test1.json"
    with open(fileData, "r") as dataLine1:
        for dataLine in dataLine1:
            idStr = dataLine.split(',', 1)[0].replace('{', '')
            id = eval(idStr.split(':')[1])
            data = "{"+dataLine.split(',', 1)[1]
            response = requests.post(url, headers=headers, data=data)
            print("insertBulk:" + response.text)
            assert response.status_code == 200

def test_deleteDoc():
    logger.info("test_deleteDoc")
    fileData = "D:/tool/vectorbase/test/data/test1.json"
    with open(fileData, "r") as dataLine1:
        for dataLine in dataLine1:
            idStr = dataLine.split(',', 1)[0].replace('{', '')
            id = eval(idStr.split(':')[1])
            data = "{"+dataLine.split(',', 1)[1]
            url = "http://" + ip_data + "/" + db_name + "/" + space_name + "/" + id
            response = requests.delete(url)
            print("deleteDoc:" + response.text)
            assert response.status_code == 200

def test_deleteSpace():
    url = "http://" + ip_db + "/space/"+db_name+"/"+space_name
    response = requests.delete(url)
    print("deleteSpace:" + response.text)
    assert response.status_code == 200

def test_deleteDB():
    url = "http://" + ip_db + "/db/"+db_name
    response = requests.delete(url)
    print("deleteDB:" + response.text)
    assert response.status_code == 200