#
# Copyright 2019 The Vearch Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
# implied. See the License for the specific language governing
# permissions and limitations under the License.

# -*- coding: UTF-8 -*-

import requests
import random
from multiprocessing import Pool
import logging
import time
import sys

from utils import parse_arguments, get_dataset_by_name


__description__ = """ benchmark for restful api"""


def create_db(args):
    url = f"{args.url}/dbs/" + args.db
    resp = requests.post(url, auth=(args.user, args.password))
    return resp


def create_space(args, space_config: dict):
    url = f"{args.url}/dbs/{args.db}/spaces"
    resp = requests.post(url, auth=(args.user, args.password), json=space_config)
    return resp


def get_space(args):
    url = f"{args.url}/dbs/{args.db}/spaces/{args.space}"
    resp = requests.get(url, auth=(args.user, args.password))
    return resp


def drop_db(args):
    url = f"{args.url}/dbs/{args.db}"
    resp = requests.delete(url, auth=(args.user, args.password))
    assert resp.json()["code"] == 0


def drop_space(args):
    url = f"{args.url}/dbs/{args.db}/spaces/{args.space}"
    resp = requests.delete(url, auth=(args.user, args.password))
    assert resp.json()["code"] == 0


def destroy(args):
    drop_space(args)
    drop_db(args)


def create_db_and_space(args):
    properties = {}
    properties["fields"] = [
        {"name": "field_int", "type": "integer"},
        {"name": "field_long", "type": "long"},
        {"name": "field_float", "type": "float"},
        {"name": "field_double", "type": "double"},
        {
            "name": "field_string",
            "type": "string",
            "index": {"name": "field_string", "type": "SCALAR"},
        },
        {
            "name": "field_vector",
            "type": "vector",
            "index": {
                "name": "gamma",
                "type": args.index_type,
                "params": {
                    "metric_type": "L2",
                },
            },
            "dimension": args.dimension,
        },
    ]
    space_config = {
        "name": args.space,
        "partition_num": args.partition_num,
        "replica_num": args.replica_num,
        "fields": properties["fields"],
    }
    response = create_db(args)
    assert response.json()["code"] == 0

    response = create_space(args, space_config)
    assert response.json()["code"] == 0


def waiting_train_finish(logger, args, timewait=5):
    if args.index_type == "FLAT" or args.index_type == "HNSW":
        return
    url = args.url + "/dbs/" + args.db + "/spaces/" + args.space
    num = 0

    while num < args.partition_num:
        num = 0
        response = requests.get(url, auth=(args.user, args.password))
        partitions = response.json()["data"]["partitions"]
        for p in partitions:
            num += p["index_status"]
        logger.debug("index status: %d" % (num))
        time.sleep(timewait)


def waiting_index_finish(logger, args, timewait=5):
    if args.index_type == "FLAT":
        return
    url = args.url + "/dbs/" + args.db + "/spaces/" + args.space
    num = 0
    while num < args.nb:
        num = 0
        response = requests.get(url, auth=(args.user, args.password))
        partitions = response.json()["data"]["partitions"]
        for p in partitions:
            num += p["index_num"]
        logger.debug("index num: %d" % (num))
        time.sleep(timewait)


def process_upsert_data(items):
    args, index, size, features = items
    url = args.url + "/document/upsert"
    data = {}
    data["db_name"] = args.db
    data["space_name"] = args.space
    data["documents"] = []
    for j in range(size):
        param_dict = {}
        param_dict["_id"] = str(index * args.batch_size + j)
        param_dict["field_int"] = index * args.batch_size + j
        param_dict["field_vector"] = features[j]
        param_dict["field_long"] = param_dict["field_int"]
        param_dict["field_float"] = float(param_dict["field_int"])
        param_dict["field_double"] = float(param_dict["field_int"])
        param_dict["field_string"] = str(param_dict["field_int"])
        data["documents"].append(param_dict)

    rs = requests.post(url, auth=(args.user, args.password), json=data)
    if rs.json()["code"] != 0:
        logger.error(rs.json())
    if rs.json()["data"]["total"] != size:
        logger.debug(rs.json())
    assert rs.json()["data"]["total"] == size


def upsert(args, xb):
    pool = Pool(args.pool_size)
    total_data = []
    total_batch = int(args.nb / args.batch_size)

    for i in range(total_batch):
        total_data.append(
            (
                args,
                i,
                args.batch_size,
                xb[i * args.batch_size : (i + 1) * args.batch_size].tolist(),
            )
        )

    remain = args.nb % args.batch_size
    if remain != 0:
        total_data.append(
            (args, total_batch, remain, xb[total_batch * args.batch_size :].tolist())
        )

    start = time.time()
    results = pool.map(process_upsert_data, total_data)
    pool.close()
    pool.join()
    end = time.time()

    total = get_space(args).json()["data"]["doc_num"]
    logger.info(
        "nb: %d, batch size:%d, upsert cost: %.4f seconds, QPS: %.4f, pool size: %d"
        % (
            total,
            args.batch_size,
            end - start,
            total / (end - start),
            args.pool_size,
        )
    )


def get_timewait(args):
    if args.nb <= 10000:
        return 1
    elif args.nb <= 10 * 10000:
        return 2
    elif args.nb <= 100 * 10000:
        return 5
    elif args.nb <= 1000 * 10000:
        return 10
    else:
        return 50


def train_and_build_index(args):
    timewait = get_timewait(args)
    start = time.time()
    waiting_train_finish(logger, args, timewait)
    end = time.time()

    logger.info(
        "nb: %d, batch size:%d, train index cost: %.4f seconds, QPS: %.4f"
        % (
            args.nb,
            args.batch_size,
            end - start,
            args.nb / (end - start),
        )
    )

    start = time.time()
    waiting_index_finish(logger, args, timewait)
    end = time.time()

    logger.info(
        "nb: %d, batch size:%d, build index cost: %.4f seconds, QPS: %.4f"
        % (
            args.nb,
            args.batch_size,
            end - start,
            args.nb / (end - start),
        )
    )


def process_query_data(items):
    args, unique_keys = items
    url = args.url + "/document/query"
    data = {}
    data["db_name"] = args.db
    data["space_name"] = args.space
    data["document_ids"] = unique_keys
    data["vector_value"] = args.vector_value

    rs = requests.post(url, auth=(args.user, args.password), json=data)
    if len(rs.json()["data"]["documents"]) != args.batch_size:
        logger.debug(rs.json())
    assert len(rs.json()["data"]["documents"]) == args.batch_size


def query(args):
    pool = Pool(args.pool_size)
    total_data = []
    # There may be some left, but won't deal with it
    total_batch = int(args.nq / args.batch_size)
    unique_ids = random.sample(range(0, args.nb), args.nq)
    unique_keys = [str(i) for i in unique_ids]
    for i in range(total_batch):
        total_data.append(
            (args, unique_keys[i * args.batch_size : (i + 1) * args.batch_size])
        )

    start = time.time()
    results = pool.map(process_query_data, total_data)
    pool.close()
    pool.join()
    end = time.time()

    logger.info(
        "nq: %d, batch size:%d, query cost: %.4f seconds, QPS: %.4f, pool size: %d"
        % (
            args.nq,
            args.batch_size,
            end - start,
            args.nq / (end - start),
            args.pool_size,
        )
    )


def process_delete_data(items):
    args, unique_keys = items
    url = args.url + "/document/delete"
    data = {}
    data["db_name"] = args.db
    data["space_name"] = args.space
    data["document_ids"] = unique_keys

    rs = requests.post(url, auth=(args.user, args.password), json=data)
    if rs.json()["data"]["total"] != args.batch_size:
        logger.debug(rs.json())
    assert rs.json()["data"]["total"] == args.batch_size


def delete(args):
    pool = Pool(args.pool_size)
    total_data = []
    # There may be some left, but won't deal with it
    total_batch = int(args.nq / args.batch_size)
    unique_ids = random.sample(range(0, args.nb), args.nq)
    unique_keys = [str(i) for i in unique_ids]
    for i in range(total_batch):
        total_data.append(
            (args, unique_keys[i * args.batch_size : (i + 1) * args.batch_size])
        )

    start = time.time()
    results = pool.map(process_delete_data, total_data)
    pool.close()
    pool.join()
    end = time.time()

    logger.info(
        "nq: %d, batch size:%d, delete cost: %.4f seconds, QPS: %.4f, pool size: %d"
        % (
            args.nq,
            args.batch_size,
            end - start,
            args.nq / (end - start),
            args.pool_size,
        )
    )


def process_search_data(items):
    args, features = items
    url = args.url + "/document/search"
    data = {}
    data["db_name"] = args.db
    data["space_name"] = args.space
    data["vectors"] = [{"field": "field_vector", "feature": features}]
    data["limit"] = args.limit

    rs = requests.post(url, auth=(args.user, args.password), json=data)
    if rs.json()["code"] != 0:
        logger.error(rs.json())
    if len(rs.json()["data"]["documents"]) != args.batch_size:
        logger.debug(rs.json())
    assert len(rs.json()["data"]["documents"]) == args.batch_size


def search(args, xq):
    pool = Pool(args.pool_size)
    total_data = []
    total_batch = int(args.nq / args.batch_size)
    for i in range(total_batch):
        total_data.append(
            (
                args,
                xq[i * args.batch_size : (i + 1) * args.batch_size].flatten().tolist(),
            )
        )

    start = time.time()
    results = pool.map(process_search_data, total_data)
    pool.close()
    pool.join()
    end = time.time()

    logger.info(
        "nq: %d, batch size:%d, search cost: %.4f seconds, QPS: %.4f, pool size: %d"
        % (
            args.nq,
            args.batch_size,
            end - start,
            args.nq / (end - start),
            args.pool_size,
        )
    )


if __name__ == "__main__":
    args = parse_arguments()

    logger = logging.getLogger(__name__)
    logger.setLevel(args.log_level)
    formatter = logging.Formatter(
        "%(asctime)s %(name)s:%(lineno)s %(levelname)s %(message)s"
    )
    if args.output != "":
        handler = logging.FileHandler(args.output, "a")
        handler.setFormatter(formatter)
        logger.addHandler(handler)
    else:
        handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(formatter)
    logger.addHandler(handler)

    xb, xq, gt = get_dataset_by_name(logger, args)

    args_str = ", ".join(f"{key}={value}" for key, value in vars(args).items())
    logger.info(f"args: {args_str}")

    create_db_and_space(args)

    upsert(args, xb)

    train_and_build_index(args)

    query(args)

    batch_size = args.batch_size
    args.batch_size = 1
    search(args, xq)

    args.batch_size = batch_size
    delete(args)

    destroy(args)
