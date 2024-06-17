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


from multiprocessing import Pool
import logging
import time
import random
import json
from vearch.core.vearch import Vearch
from vearch.config import Config
from vearch.schema.field import Field
from vearch.schema.space import SpaceSchema
from vearch.utils import DataType, MetricType, VectorInfo
from vearch.schema.index import FlatIndex, ScalarIndex

from utils import parse_arguments, get_dataset_by_name


__description__ = """ benchmark for pysdk"""


def create_db_and_space(args):
    config = Config(host=args.url, token=args.password)
    vc = Vearch(config)

    ret = vc.create_database(args.db)
    assert ret.code == 0

    field_int = Field(
        "field_int",
        DataType.INTEGER,
        desc="the integer type field",
        index=ScalarIndex("field_int"),
    )
    field_long = Field(
        "field_long",
        DataType.LONG,
        desc="the long type field",
        index=ScalarIndex("field_long"),
    )
    field_float = Field(
        "field_float",
        DataType.FLOAT,
        desc="the float type field",
        index=ScalarIndex("field_float"),
    )
    field_double = Field(
        "field_double",
        DataType.DOUBLE,
        desc="the double type field",
        index=ScalarIndex("field_double"),
    )
    field_string = Field(
        "field_string",
        DataType.STRING,
        desc="the string type field",
        index=ScalarIndex("field_string"),
    )
    field_vector = Field(
        "field_vector",
        DataType.VECTOR,
        FlatIndex("field_vector", MetricType.L2),
        dimension=args.dimension,
    )
    space_schema = SpaceSchema(
        args.space,
        fields=[
            field_int,
            field_long,
            field_float,
            field_double,
            field_string,
            field_vector,
        ],
        partition_num=args.partition_num,
        replication_num=args.replica_num,
    )

    ret = vc.create_space(args.db, space_schema)
    assert ret.code == 0
    return vc


def waiting_train_finish(logger, args, timewait=5):
    if args.index_type == "FLAT" or args.index_type == "HNSW":
        return
    num = 0

    while num < args.partition_num:
        num = 0
        _, _, space = vc.is_space_exist(args.db, args.space)
        space = json.loads(space)
        partitions = space["partitions"]
        for p in partitions:
            num += p["index_status"]
        logger.debug("index status: %d" % (num))
        time.sleep(timewait)


def waiting_index_finish(logger, args, timewait=5):
    if args.index_type == "FLAT":
        return
    num = 0
    while num < args.nb:
        num = 0
        _, _, space = vc.is_space_exist(args.db, args.space)
        space = json.loads(space)
        partitions = space["partitions"]
        for p in partitions:
            num += p["index_num"]
        logger.debug("index num: %d" % (num))
        time.sleep(timewait)


def process_upsert_data(items):
    args, index, size, features = items
    data = []
    for j in range(size):
        param_dict = {}
        param_dict["_id"] = str(index * args.batch_size + j)
        param_dict["field_int"] = index * args.batch_size + j
        param_dict["field_vector"] = features[j]
        param_dict["field_long"] = param_dict["field_int"]
        param_dict["field_float"] = float(param_dict["field_int"])
        param_dict["field_double"] = float(param_dict["field_int"])
        param_dict["field_string"] = str(param_dict["field_int"])
        data.append(param_dict)

    rs = vc.upsert(args.db, args.space, data)
    if rs.code != 0:
        logger.error(rs.msg)
    if len(rs.get_document_ids()) != size:
        logger.debug(rs.get_document_ids())
    assert len(rs.get_document_ids()) == size


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

    _, _, space = vc.is_space_exist(args.db, args.space)
    total = json.loads(space)["doc_num"]
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
    rs = vc.query(
        args.db, args.space, document_ids=unique_keys, vector=args.vector_value
    )

    if len(rs.documents) != args.batch_size:
        logger.debug(rs.documents)
    assert len(rs.documents) == args.batch_size


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
    rs = vc.delete(args.db, args.space, unique_keys)

    if len(rs.document_ids) != args.batch_size:
        logger.debug(rs.document_ids)
    assert len(rs.document_ids) == args.batch_size


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
    vector_info = VectorInfo("field_vector", features)
    rs = vc.search(
        args.db,
        args.space,
        vector_infos=[vector_info],
        vector=args.vector_value,
        limit=args.limit,
    )
    if rs.code != 0:
        logger.error(rs.msg)
    if len(rs.documents) != args.batch_size:
        logger.debug(rs.documents)
    assert len(rs.documents) == args.batch_size


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

    vc = create_db_and_space(args)

    upsert(args, xb)

    train_and_build_index(args)

    query(args)

    batch_size = args.batch_size
    args.batch_size = 1
    search(args, xq)

    args.batch_size = batch_size
    delete(args)

    vc.drop_space(args.db, args.space)
    vc.drop_database(args.db)
