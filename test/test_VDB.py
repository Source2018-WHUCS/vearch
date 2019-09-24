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
def test_状态查看():
    logger.info("集群信息")
    url = "http://" + ip_db + "/_cluster/stats"
    response = requests.get(url)
    print("状态查看---\n" + response.text)
    assert response.status_code == 200

def test_健康状态():
    url = "http://" + ip_db + "/_cluster/health"
    response = requests.get(url)
    print("健康状态---\n" + response.text)
    assert response.status_code == 200

def test_端口状态():
    url = "http://" + ip_db + "/list/server"
    response = requests.get(url)
    print("端口状态---\n" + response.text)
    assert response.status_code == 200

def test_查看库列表():
    url = "http://" + ip_db + "/list/db"
    response = requests.get(url)
    print("查看库列表---\n" + response.text)
    assert response.status_code == 200

def test_创建库():
    logger.info("------------")
    url = "http://" + ip_db + "/db/_create"
    headers = {"content-type": "application/json"}
    data = {
        'name':db_name
    }
    response = requests.put(url, headers=headers, data=json.dumps(data))
    print("创建库---\n" + response.text)
    assert response.status_code == 200

def test_查看库():
    url = "http://" + ip_db + "/db/" + db_name
    response = requests.get(url)
    print("查看库---\n" + response.text)
    assert response.status_code == 200

def test_查看指定空间():
    url = "http://" + ip_db + "/list/space?db=" + db_name
    response = requests.get(url)
    print("查看指定空间---\n" + response.text)
    assert response.status_code == 200

def test_创建空间():
    logger.info("创建空间")
    url = "http://" + ip_db + "/space/" + db_name +"/_create"
    headers = {"content-type": "application/json"}
    data = {
        "name": space_name,
        "dynamic_schema": "strict",
        "partition_num": 2, #"partition_num": 2-6之间
        "replica_num": 1,
        "engine": {"name":"gamma", "index_size":100000, "max_size":7000000},
        "properties": {
            "pro": {
                "type" : "integer",
                "index" : "false"
            },
            "i_url": {
                "type": "keyword"
            },
            "fid1": {
                "type": "integer"
            },
            "fid2": {
                "type": "integer"
            },
            "fid3": {
                "type": "integer"
            },
            "su" : {
                "type": "integer",
                "index" : "false"
            },
            "b_id": {
                "type": "integer",
                "index" : "false"
            },
            "status" : {
                "type" : "integer",
                "index" : "false"
            },
            "feature": {
                "type": "vector",
                "model_id": "img",
                "dimension": 512
            }
        },
        "models": [{
            "model_id": "vgg16",
            "fields": ["url"],
            "out": "feature"
        }]
    }
    response = requests.put(url, headers=headers, data=json.dumps(data))
    print("创建空间---\n" + response.text)
    assert response.status_code == 200

def test_查看空间():
    url = "http://" + ip_db + "/space/"+db_name+"/" + space_name
    response = requests.get(url)
    print("查看空间---\n" + response.text)
    assert response.status_code == 200

logger.info("router(PS)模块")
def test_添加数据1():
    logger.info("添加数据")
    headers = {"content-type": "application/json"}
    fileData = "data1.txt"
    fileFeature = "feature1.txt"
    with open(fileData, "r") as dataLine1, open(fileFeature,"r") as fileFeature1:
        for dataLine in dataLine1:
            i = 1
            url = "http://" + ip_data + "/" + db_name + "/" + space_name + "/" + str(i)
            feature = fileFeature1.readline()
            feature = feature.replace('(', '[')
            feature = feature.replace(')', ']')
            list = dataLine.split("\t")
            pro = list[0]
            source = list[1]
            fid1 = list[2]
            fid2 = list[3]
            fid3 = list[4]
            su = list[5]
            b_id = list[8].strip()
            data = {
                "pro": pro,
                "su": su,
                "feature": [{
                    "source": source,
                    "feature": feature
                }],
                "b_id": b_id,
                "fid1": fid1,
                "fid2": fid2,
                "fid3": fid3
            }
            data["feature"][0]["feature"] = json.loads(data["feature"][0]["feature"])
            response = requests.post(url, headers=headers, data=json.dumps(data))
            print("添加数据_指定id---\n" + response.text)
            assert response.status_code == 200
            i = i + 1
def test_查询数据():
    logger.info("查询数据")
    for i in range(100001):
        url = "http://" + ip_data + "/" + db_name + "/" + space_name + "/" + str(i)
        response = requests.get(url)
        print("查询数据_根据id查询---\n" + response.text)
        assert response.status_code == 200

def test_添加数据():
    logger.info("添加数据")
    headers = {"content-type": "application/json"}
    fileData = "data1.txt"
    fileFeature = "feature1.txt"
    with open(fileData, "r") as dataLine1, open(fileFeature,"r") as fileFeature1:
        for dataLine in dataLine1:
            url = "http://" + ip_data +  "/" + db_name + "/" + space_name
            feature = fileFeature1.readline()
            feature = feature.replace('(', '[')
            feature = feature.replace(')', ']')
            list = dataLine.split("\t")
            pro = list[0]
            source = list[1]
            fid1 = list[2]
            fid2 = list[3]
            fid3 = list[4]
            su = list[5]
            b_id = list[8].strip()
            data = {
                "pro": pro,
                "su": su,
                "feature": [{
                    "source": source,
                    "feature": feature
                }],
                "b_id": b_id,
                "fid1": fid1,
                "fid2": fid2,
                "fid3": fid3
            }
            data["feature"][0]["feature"] = json.loads(data["feature"][0]["feature"])
            response = requests.post(url, headers=headers, data=json.dumps(data))
            print("添加数据_不指定id---\n" + response.text)
            assert response.status_code == 200

def test_批量添加():
    logger.info("批量添加")
    url = "http://" + ip_data + "/"+db_name+"/"+space_name+"/_bulk"
    headers = {"content-type": "application/json"}
    fileData = "data1.txt"
    fileFeature = "feature1.txt"
    with open(fileData, "r") as dataLine1, open(fileFeature,"r") as fileFeature1:
        for dataLine in dataLine1:
            feature = fileFeature1.readline()
            feature = feature.replace('(', '[')
            feature = feature.replace(')', ']')
            list = dataLine.split("\t")
            pro = list[0]
            source = list[1]
            fid1 = list[2]
            fid2 = list[3]
            fid3 = list[4]
            su = list[5]
            b_id = list[8].strip()
            data = {
                "pro": pro,
                "su": su,
                "feature": [{
                    "source": source,
                    "feature": feature
                }],
                "b_id": b_id,
                "fid1": fid1,
                "fid2": fid2,
                "fid3": fid3
            }
            data["feature"][0]["feature"] = json.loads(data["feature"][0]["feature"])
            response = requests.post(url, headers=headers, data=json.dumps(data))
            print("批量添加---\n" + response.text)
            assert response.status_code == 200

def test_更新文档():
    logger.info("更新文档")
    url = "http://" + ip_data + "/"+db_name+"/"+space_name+"/1"
    headers = {"content-type": "application/json"}
    fileData = "data1.txt"
    fileFeature = "feature1.txt"
    with open(fileData, "r") as dataLine1, open(fileFeature,"r") as fileFeature1:
        for dataLine in dataLine1:
            feature = fileFeature1.readline()
            feature = feature.replace('(', '[')
            feature = feature.replace(')', ']')
            list = dataLine.split("\t")
            pro = list[0]
            source = list[1]
            fid1 = list[2]
            fid2 = list[3]
            fid3 = list[4]
            su = list[5]
            b_id = list[8].strip()
            data = {
                "pro": pro,
                "su": su,
                "feature": [{
                    "source": source,
                    "feature": feature
                }],
                "b_id": b_id,
                "fid1": fid1,
                "fid2": fid2,
                "fid3": fid3
            }
            data["feature"][0]["feature"] = json.loads(data["feature"][0]["feature"])
            response = requests.post(url, headers=headers, data=json.dumps(data))
            print("更新文档---\n" + response.text)
            assert response.status_code == 200

def test_删除文档():
    logger.info("删除文档")
    url = "http://" + ip_data + "/"+db_name+"/"+space_name+"/1"
    response = requests.delete(url)
    print("删除文档---\n" + response.text)
    assert response.status_code == 200


def test_查询数据使用特征查询():
    url = "http://" + ip_data + "/"+db_name+"/"+space_name+"/_search?size=100"
    headers = {"content-type": "application/json"}
    fileData = "data1.txt"
    fileFeature = "feature1.txt"
    with open(fileData, "r") as dataLine1, open(fileFeature,"r") as fileFeature1:
        for dataLine in dataLine1:
            feature = fileFeature1.readline()
            feature = feature.replace('(', '[')
            feature = feature.replace(')', ']')
            data = {
                "query": {
                    "sum" :[{
                        "field": "feature",
                        "feature": feature
                    }]
                }
            }
            data["query"]["sum"][0]["feature"] = json.loads(data["query"]["sum"][0]["feature"])
            response = requests.post(url, headers=headers, data=json.dumps(data))
            print("查询数据_使用特征查询---\n" + response.text)
            assert response.status_code == 200

def test_查询数据使用特征带数值过滤字段():
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
            print("查询数据_使用特征带数值过来字段查询---\n" + response.text)
            assert response.status_code == 200


def test_删除空间():
    url = "http://" + ip_db + "/space/"+db_name+"/"+space_name
    response = requests.delete(url)
    print("删除空间---\n" + response.text)
    assert response.status_code == 200

def test_删除库():
    url = "http://" + ip_db + "/db/"+db_name
    response = requests.delete(url)
    print("删除库---\n" + response.text)
    assert response.status_code == 200