# Copyright 2019 The Vearch Authors. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# ==============================================================================

import os
import sys
import cv2
import json
import queue
import torch
import base64
import requests
import urllib.request
import importlib
import numpy as np
import threading
import shortuuid
import argparse
import traceback
import concurrent.futures
from multiprocessing import Queue, Process
from asyncio.futures import wrap_future
# from sklearn.preprocessing import normalize
from concurrent.futures import ThreadPoolExecutor

import tornado
import tornado.web
import tornado.httpserver
import tornado.httpclient
from tornado import gen

from config import config

DEBUG = True
_INSERT = "insert"
_SEARCH = "search"
_HEADERS = {"content-type": "application/json"}

def get_model(model_path):
    """get model by model_name
    Args:
        model_name: the name by user define
    Returns:
        the model
    Raises:
        ModuleNotFoundError model not exist
    """
    try:
        model = importlib.import_module(f"{model_path}")
    except ModuleNotFoundError:
        raise ModuleNotFoundError(f"{model_path} is not existed")
    return model


class ImageDealProcess(Process):

    def __init__(self, input_queue, output_queue, result_queue):
        Process.__init__(self)
        self.input_queue = input_queue
        self.output_queue = output_queue
        self.result_queue = result_queue

    def read_image(self, image_url):
        if image_url.startswith("http"):
            resp = urllib.request.urlopen(image_url).read()
        else:
            with open(image_url, "rb") as f:
                resp = f.read()
        return resp

    def deal(self, url):
        if len(url) > 100:
            resp = base64.b64decode(url)
        else:
            resp = self.read_image(url)
        img = np.asarray(bytearray(resp), dtype="uint8")
        img = cv2.imdecode(img, cv2.IMREAD_COLOR)
        # img = cv2.resize(img, (self.img_size, self.img_size), interpolation=cv2.INTER_LINEAR)
        return img

    def run(self):
        """Parallel deal image"""
        executor = ThreadPoolExecutor(50)

        def parallel_deal(uuid, method, data):
            try:
                imageurl = data["imageurl"]
                # print(imageurl)
                img = self.deal(imageurl)
                self.output_queue.put((uuid, method, data, img))
            except Exception as err:
                if DEBUG:
                    traceback.print_exc()
                self.result_queue.put((uuid, False, str(err)))

        while True:
            uuid, method, data = self.input_queue.get()
            # print(uuid, url)
            executor.submit(parallel_deal, uuid, method, data)


def watch_thread_queue(thread_queue, process_queue):
    """Get data from Process queue and put it in Thread queue
    Args:
        thread_queue  : Thread  queue
        process_queue : Process queue
    """
    while True:
        res = process_queue.get()
        thread_queue.put(res)


class DetectProcess(Process):

    def __init__(self, gpu_id, detect_model, input_queue, output_queue, result_queue, loc_queue):
        Process.__init__(self)
        self.gpu_id = gpu_id
        self.detect_model = detect_model
        self.input_queue = input_queue
        self.output_queue = output_queue
        self.result_queue = result_queue
        self.loc_queue = loc_queue
        self.watch_queue = queue.Queue()

    def build(self):
        # os.environ['CUDA_VISIBLE_DEVICES'] = "1"
        if self.detect_model is None:
            self.model = None
        else:
            os.environ['CUDA_VISIBLE_DEVICES'] = self.gpu_id
            self.model = get_model(f"model.image_detect.{self.detect_model}").load_model()
        t = threading.Thread(target=watch_thread_queue, args=(self.watch_queue, self.input_queue))
        t.start()

    def detect(self, uuid, method, data, image):
        """Detect loc and label by image
        """
        detection_flag = data.pop("detection", True)
        if detection_flag:
            if self.model is None:
                raise Exception("detect model is not defined")
            results = self.model.detect(image)
            if len(results) == 0:
                results = [[None, 1.00, None]]
        else:
            if "boundingbox" in data and data["boundingbox"] is not None:
                bbox = list(map(int, map(float, data.pop("boundingbox").split(","))))
            else:
                bbox = None
                # bbox = [0, 0, image.shape[1], image.shape[0]]
            if "label" in data:
                label = data.pop("label")
            else:
                label = None
            results = [[label, 1.00, bbox]]
        results = list(sorted(results, key=lambda d:d[1], reverse=True))
        if method == _INSERT:
            for res in results:
                label, score, bbox = res
                self.output_queue.put((uuid, method, data, image, label, bbox))
                break
        elif method == _SEARCH:
            label, score, bbox = results[0]
            self.output_queue.put((uuid, method, data, image, label, bbox))
        else:
            self.loc_queue.put((uuid, True, json.dumps(results)))

    def run(self):
        self.build()
        while True:
            uuid, method, data, image = self.watch_queue.get()
            try:
                self.detect(uuid, method, data, image)
            except Exception as err:
                if DEBUG:
                    traceback.print_exc()
                self.result_queue.put((uuid, False, str(err)))


class CropAndResizeProcess(Process):
    
    def __init__(self, model_name, input_queue, output_queue, result_queue):
        Process.__init__(self)
        self.model_name = model_name
        self.input_queue = input_queue
        self.output_queue = output_queue
        self.result_queue = result_queue

    def deal(self, image, loc):
        if loc is None:
            img_crop = image
        else:
            x_min, y_min, x_max, y_max = map(int, map(float, loc))
            img_crop = image[y_min:y_max, x_min:x_max]
        # img_resize = cv2.resize(img_crop, (self.image_size, self.image_size), interpolation=cv2.INTER_LINEAR)
        img_resize = self.model.preprocess_input(img_crop)
        return img_resize

    def run(self):
        self.model = get_model("model.image_retrieval." + self.model_name).load_model()
        while True:
            uuid, method, data, image, label, loc = self.input_queue.get()
            # print(uuid, url, image.shape, label, loc)
            try:
                img_resize = self.deal(image, loc)
                self.output_queue.put((uuid, method, data, img_resize, label, loc))
            except Exception as err:
                if DEBUG:
                    traceback.print_exc()
                self.result_queue.put((uuid, False, str(err)))


class ExtractProcess(Process):

    def __init__(self, gpu_id, model_name, input_queue, output_queue):
        Process.__init__(self)
        self.gpu_id = gpu_id
        self.model_name = model_name
        self.input_queue = input_queue
        self.output_queue = output_queue
        self.watch_queue = queue.Queue()

    def build(self):
        # os.environ['CUDA_VISIBLE_DEVICES'] = "1"
        os.environ['CUDA_VISIBLE_DEVICES'] = self.gpu_id
        self.model = get_model("model.image_retrieval." + self.model_name).load_model()
        self.model.load_model()
        # print(self.model)
        t = threading.Thread(target=watch_thread_queue, args=(self.watch_queue, self.input_queue))
        t.start()

    def normlize(self, mat):
        return mat/np.linalg.norm(mat, axis=1)[:,np.newaxis]

    def exect(self, imgs):
        # print(len(imgs), self.watch_queue.qsize())
        # imgs = np.array(imgs, dtype=np.float64)
        # imgs = self.model.preprocess_input(imgs)
        # imgs = torch.from_numpy(imgs)
        # imgs = imgs.permute(0,3,1,2).float()
        # print(imgs.shape, imgs)
        feats_list = self.model.forward(imgs)
        # feats_list = self.model.torch2list(feats_torch)
        # result = normalize(feats_list, norm='l2', axis=1)
        result = self.normlize(feats_list)
        return result.tolist()

    def get_batch(self):
        is_block = True
        batch_list = []
        for _ in range(config.batch_size):
            try:
                uuid, method, data, img_resize, label, loc = self.watch_queue.get(block=is_block)
                batch_list.append((uuid, method, data, img_resize, label, loc))
            except:
                break
            is_block = False
        return batch_list

    def run(self):
        self.build()
        while True:
            batch_list = self.get_batch()
            try:
                feat_list = self.exect([d[3] for d in batch_list])
            except:
                if DEBUG:
                    traceback.print_exc()
                feat_list = len(batch_list) * [None]

            for idx in range(len(batch_list)):
                uuid, method, data, img_resize, label, loc = batch_list[idx]
                self.output_queue.put((uuid, method, data, label, loc, feat_list[idx]))


class PackageProcess(Process):
    def __init__(self, input_queue, output_queue):
        Process.__init__(self)
        self.input_queue = input_queue
        self.output_queue = output_queue
        self.watch_queue = queue.Queue()

    def run(self):
        executor = ThreadPoolExecutor(50)
        # Put data in the process queue into the thread queue
        t = threading.Thread(target=watch_thread_queue, args=(self.watch_queue, self.input_queue))
        t.start()

        def deal(uuid, ip, request_body):
            # Repackage the request body and send it to VectorDB
            # and put the response into `response_queue`
            try:
                # print("response body", uuid, ip, _HEADERS, request_body)
                response_body = requests.post(ip, headers=_HEADERS, data=json.dumps(request_body))
                # print("response_body", response_body)
                # print("response_body", response_body.text)
                self.output_queue.put((uuid, True, response_body.json()))
            except Exception as err:
                if DEBUG:
                    traceback.print_exc()
                self.output_queue.put((uuid, False, str(err)))

        while True:
            uuid, method, data, label, loc, feat = self.watch_queue.get()
            db_name = data.pop("db")
            space_name = data.pop("space")
            imageurl = data["imageurl"]
            if method == _INSERT:
                ip = f"{config.ip_insert}/{db_name}/{space_name}"
                if "_id" in data:
                    _id = data.pop("_id")
                    ip = f"{ip}/{_id}"
                if loc is not None:
                    loc = list(map(str, loc))
                    data["boundingbox"] = ",".join(loc)
                if label is not None:
                    data["label"] = label
                data["feature"] = {"source": imageurl, "feature": feat}
            else:
                size = data.pop("size", 10)
                min_score = data.pop("score", 0)
                filters = data.pop("filter", None)
                ip = f"{config.ip_insert}/{db_name}/{space_name}/_search?size={size}"
                data = {"query": {"sum": [{"feature": feat, "field": "feature", "min_score": min_score, "max_score": 1.0}],"filter":filters}}
            executor.submit(deal, uuid, ip, data)


class TornadoProcess(Process):

    def __init__(self, port, input_queue, output_queue, loc_queue, futures_dict):
        Process.__init__(self)
        self.port = port   # run the server on given port
        self.watch_queue = queue.Queue()
        self.input_queue = input_queue
        self.output_queue = output_queue
        self.loc_queue = loc_queue
        self.futures_dict = futures_dict

    def run(self):
        t1 = threading.Thread(target=set_result_thread, args=(self.output_queue, self.futures_dict))
        t1.start()
        t1 = threading.Thread(target=set_result_thread, args=(self.loc_queue, self.futures_dict))
        t1.start()
        app = tornado.web.Application(
            [(f'/.*', TableHandler, dict(input_queue=self.input_queue, futures_dict=self.futures_dict))]
        )
        sockets = tornado.netutil.bind_sockets(self.port)
        server = tornado.httpserver.HTTPServer(app)
        server.add_sockets(sockets)
        tornado.ioloop.IOLoop.instance().start()


def set_result_thread(result_queue, futures_dict):
    while True:
        uuid, flag, result = result_queue.get()
        if uuid in futures_dict:
            if not flag:
                result = json.dumps({"status": 550, "error":{"index": "", "index_uuid": "", "shard": "0", "type": "", "reason": result}})
                # result = json.dumps(
                # {"_index": db_name,"_type":tb_name,"status":550,
                # "error":{"index":"","index_uuid":"","shard":"0","type":"","reason":result}})
            futures_dict[uuid].set_result(result)
            del futures_dict[uuid]


class TableHandler(tornado.web.RequestHandler):

    def initialize(self, input_queue, futures_dict):
        self.input_queue = input_queue
        self.futures_dict = futures_dict

    def parse_uri(self):
        # print(self.request.uri)
        self.uri = self.request.uri.split("?")[0].split("/")
        # print(self.uri)
        self.db_name, self.space_name, self.operate = self.uri[1:4]

    async def get(self):
        response = requests.get(f"{config.ip_insert}/{self.request.uri}", headers=_HEADERS)
        self.write(response.text)

    async def delete(self):
        response = requests.delete(f"{config.ip_insert}/{self.request.uri}", headers=_HEADERS)
        data = response.json()
        if data["status"] == 200:
            result = {"code": 200, "msg": "success"}
        else:
            result = {"code": data["status"], "msg": data["error"]["reason"]}

        self.write(json.dumps(result))

    async def put(self):
        method = "PUT"
        await self.deal_operate(method)

    async def post(self):
        method = "POST"
        await self.deal_operate(method)

    async def deal_operate(self, method):
        try:
            self.parse_uri()
            # print(self.request.body)
            if self.request.body:
                request_body = json.loads(self.request.body)
            else:
                request_body = None
            if self.operate == "_create":
                message = self._create(request_body)
            elif self.operate == "_delete":
                message = self._delete(request_body)
            elif self.operate == "_search":
                message = await self._search(request_body)
            elif self.operate == "_detect":
                message = await self._detect(request_body)
            elif self.operate == "_update":
                request_body["_id"] = self.get_argument("id")
                message = await self._insert(request_body)
            elif self.operate == "_insert":
                message = await self._insert(request_body)
            else:
                message = "request is wrong"

        except Exception as err:
            if DEBUG:
                traceback.print_exc()
            message = str(err)
            # self.set_status(201, message)
        finally:
            self.write(message)

    async def deal(self, method, data):
        data["db"] = self.db_name
        data["space"] = self.space_name
        uuid = shortuuid.uuid()
        future = concurrent.futures.Future()
        self.futures_dict[uuid] = future
        self.input_queue.put((uuid, method, data))
        result = await wrap_future(future)
        # del self.futures_dict[uuid]
        return result

    def _create(self, data):
        '''Create Database and Space
        Args:
            data: Dict
        '''

        db_flag = data.pop("db", True)
        if db_flag:
            res = requests.put(
                        config.ip_address + ":443/db/_create",
                        headers=_HEADERS,
                        data=json.dumps({"name": self.db_name})
                    )
            result_db = json.loads(res.text)
            if result_db["code"] != 200:
                return result_db
        else:
            result_db = {"msg": None}

        # create space
        feature_default = {"type": "vector",
                           "model_id": "vgg16",
                           "dimension": 512,
                           "filed": "imageurl"
                           }
        columns_default = {"imageurl": {"type":"keyword"},
                           "boundingbox": {"type": "keyword"},
                           "label": {"type": "keyword"}
                           }

        method = data.pop("method", "innerproduct")
        feature = data.pop("feature", feature_default)
        columns = data.pop("columns", columns_default)

        # columns
        assert "label" in columns, "label not in columns"
        assert "imageurl" in columns, "imageurl not in columns"
        assert "boundingbox" in columns, "boundingbox not in columns"

        # feature
        assert "model_id" in feature, "model_id not in feature"
        assert "dimension" in feature, "dimension not in feature"

        data = {
                "name": self.space_name,
                "engine": {"name": "gamma"},
                "properties": {}
                }

        data["properties"].update(columns)
        data["properties"]["feature"] = feature

        res = requests.put(
                    f"{config.ip_address}:443/space/{self.db_name}/_create?timeout=600",
                    headers=_HEADERS,
                    data=json.dumps(data)
                )
        result_space = json.loads(res.text)
        if result_space["code"] != 200:
            return result_space

        result = {"code": 200, "db_msg": result_db["msg"], "space_msg": result_space["msg"]}
        return result

    def _delete(self, data):
        space_flag = data.pop("space", True)
        if space_flag:
            res = requests.delete(f"{config.ip_address}:443/space/{self.db_name}/{self.space_name}")
            result_space = json.loads(res.text)
            if result_space["code"] != 200:
                return result_space
        else:
            result_space = {"msg": None}

        db_flag = data.pop("db", True)
        if db_flag:
            res = requests.delete(f"{config.ip_address}:443/db/{self.db_name}")
            result_db = json.loads(res.text)
            if result_db["code"] != 200:
                return result_db
        else:
            result_db = {"msg": None}

        result = {"code": 200, "db_msg": result_db["msg"], "space_msg": result_space["msg"]}
        return result

    async def _search(self, data):
        # url = request_body["url"]
        # response_body = await self.deal("search", url)
        response_body = await self.deal("search", data)
        return json.dumps(response_body)


    async def _insert(self, data):
        method = data.pop("method", "single")
        assert "imageurl" in data, "imageurl not in requests body"
        result = {"db": self.db_name, "space": self.space_name, "ids": [], "successful":0}

        def pkg_result(response_body):
            # print(response_body)
            assert "_id" in response_body, response_body
            _id = response_body["_id"]
            if response_body["status"] == 201:
                result["ids"].append({_id: "successful"})
                result["successful"] = result.get("successful") + 1
            else:
                result["ids"].append({_id: response_body["error"]["reason"]})

        if method == "single":
            # url = data["imageurl"]
            response_body = await self.deal("insert", data)
            pkg_result(response_body)

        elif method == "bulk":
            imagefile = data.pop("imageurl")
            if not os.path.exists(imagefile):
                raise Exception(f"{imagefile} not exists!")
            with open(imagefile, 'r') as fr:
                col_names = fr.readline().strip().split(",")
                body_list = [dict(zip(col_names, col_values.strip().split(","))) for col_values in fr.readlines()]
            response_body_list = await gen.multi([self.deal("insert", data) for data in body_list])
            for response_body in response_body_list:
                pkg_result(response_body)

        return result


def parse_argument():
    parser = argparse.ArgumentParser()
    parser.add_argument("--debug", action="store_true", help="debug model")
    parser.add_argument("--port", type=str, default=4101, help="Port the server run on")
    parser.add_argument("--gpu", type=str, default="0", help="Which GPU the server run on")
    # parser.add_argument("--feature_model", type=str, default="vgg16", help="Which feature model you need")
    # parser.add_argument("--detect_model", type=str, default="keras_yoyo", help="Which detect model you need")
    args = parser.parse_args()
    return args


def main():
    args = parse_argument()
    port = args.port
    gpu = args.gpu
    # model_name = args.feature_model
    # model_module = get_model("model.image_retrieval." + model_name)
    # detect_model = args.detect_model
    # gpu = config.gpu
    # port = config.port
    detect_model = config.detect_model
    extract_model = config.extract_model

    process_list = []
    url_queue = Queue()
    result_queue = Queue()
    download_queue = Queue()
    detect_queue = Queue()
    crop_resize_queue = Queue()
    extract_queue = Queue()
    loc_queue = Queue()
    image_deal_process = ImageDealProcess(url_queue, download_queue, result_queue)
    process_list.append(image_deal_process)
    detect_process = DetectProcess(gpu, detect_model, download_queue, detect_queue, result_queue, loc_queue)
    process_list.append(detect_process)
    crop_resize_process = CropAndResizeProcess(extract_model, detect_queue, crop_resize_queue, result_queue)
    process_list.append(crop_resize_process)
    extract_process = ExtractProcess(gpu, extract_model, crop_resize_queue, extract_queue)
    process_list.append(extract_process)
    package_process = PackageProcess(extract_queue, result_queue)
    process_list.append(package_process)

    # tornado_process = TornadoProcess(port, url_queue, result_queue, loc_queue, dict())
    # process_list.append(tornado_process)
    for p in process_list:
        p.daemon = True
        p.start()

    is_alive = True
    for p in process_list:
        print(p, p.is_alive())
        if not p.is_alive():
            is_alive = False

    if is_alive:
        tornado_process = TornadoProcess(port, url_queue, result_queue, loc_queue, dict())
        tornado_process.start()
        tornado_process.join()


if __name__ == "__main__":
    main()




