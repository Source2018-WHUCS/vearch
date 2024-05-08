from vearch.config import Config
from vearch.core.db import Database, Space
from vearch.core.vearch import Vearch
from vearch.schema.field import Field
from vearch.schema.space import SpaceSchema
from vearch.utils import DataType, MetricType, VectorInfo
from vearch.schema.index import IvfPQIndex, Index, ScalarIndex
from vearch.filter import Filter,Condition,FieldValue,Conditions
import logging
from typing import List
import json

logger = logging.getLogger("vearch")


def create_space_schema() -> SpaceSchema:
    book_name = Field("book_name", DataType.STRING, desc="the name of book", index=ScalarIndex("book_name_idx"))
    book_num=Field("book_num", DataType.INTEGER, desc="the num of book",index=ScalarIndex("book_num_idx"))
    book_vector = Field("book_character", DataType.VECTOR,
                        IvfPQIndex("book_vec_idx", 10000, MetricType.Inner_product, 2048, 8), dimension=512)
    ractor_address = Field("ractor_address", DataType.STRING, desc="the place of the book put")
    space_schema = SpaceSchema("book_info", fields=[book_name,book_num, book_vector, ractor_address])
    return space_schema


def create_database(vc: Vearch):
    logger.debug(vc.client.host)
    ret = vc.create_database("database1")
    logger.debug(ret.dict_str())


def list_databases(vc: Vearch):
    logger.debug(vc.client.host)
    ret = vc.list_databases()
    logger.debug(ret)


def create_space(vc: Vearch):
    space_schema = create_space_schema()
    ret = vc.create_space("database1", space_schema)
    print("######",ret.text, ret.err_msg)


def upsert_document(vc: Vearch) -> List:
    import random
    ractor = ["ractor_logical", "ractor_industry", "ractor_philosophy"]
    book_name_template = "abcdefghijklmnopqrstuvwxyz0123456789"
    data = []
    num=[12,34,56,74,53,11,14,9]
    for i in range(8):
        book_item = ["".join(random.choices(book_name_template, k=8)),
                     num[i],
                     [random.uniform(0, 1) for _ in range(512)],
                     ractor[random.randint(0, 2)]]
        data.append(book_item)
        logger.debug(book_item)
    space = Space("database1", "book_info")
    ret = space.upsert_doc(data)
    if ret:
        logger.debug("upsert result:" + str(ret.get_document_ids()))
        return ret.get_document_ids()
    return []


def query_documents(ids: List):
    space = Space("database1", "book_info")
    ret = space.query(ids)
    for doc in json.loads(ret)["documents"]:
        logger.debug(doc)


def search_documets():
    import random
    space = Space("database1", "book_info")
    feature = [random.uniform(0, 1) for _ in range(512)]
    vi = VectorInfo("book_character", feature)
    ret = space.search(vector_infos=[vi, ],limit=7)
    for doc in ret:
        print("search document&&&&&&",doc)


def is_database_exist(vc: Vearch):
    ret = vc.is_database_exist("database1")
    return ret


def is_space_exist(vc: Vearch):
    ret = vc.is_space_exist("database1", "book_info")
    logger.debug(ret)
    return ret


def delete_space(vc: Vearch):
    ret = vc.drop_space("database1", "book_info")
    print(ret.text, ret.err_msg)


def drop_database(vc: Vearch):
    logger.debug(vc.client.host)
    ret = vc.drop_database("database1")
    logger.debug(ret.dict_str())

def turn_data_to_filter(post_data):
    conditons=[Condition(item["operator"],FieldValue(item["field"],item["value"])) for item in post_data["conditions"]]
    filters=Filter(post_data["operator"],conditons)
    return filters
    
def query_documnet_by_filter(ids,post_data):
   
    filters=turn_data_to_filter(post_data) 
    space = Space("database1", "book_info")
    ret = space.query(filter=filters,limit=2)
    for doc in json.loads(ret)["documents"]:
        logger.debug(doc)

def search_doc_by_filter(post_data):
    import random
    space = Space("database1", "book_info")
    feature = [random.uniform(0, 1) for _ in range(512)]
    vi = VectorInfo("book_character", feature)
    filters=turn_data_to_filter(post_data)  
    
    ret = space.search(vector_infos=[vi, ],filter=filters,limit=3)
    if ret is not None:
        for doc in ret:
            print("search document-00--=",doc)

if __name__ == "__main__":
    """
    curl --location 'http://test-api-interface-1-router.vectorbase.svc.sq01.n.jd.local/cluster/stats' \
--header 'Authorization: secret' \
--data ''"""

    config = Config(host="http://test-api-interface-1-router.vectorbase.svc.sq01.n.jd.local", token="secret")
    vc = Vearch(config)
    # print("is_database_exist",is_database_exist(vc))
    # if not is_database_exist(vc):
    #     create_database(vc)
    # print("**is_database_exist",is_database_exist(vc))
    # list_databases(vc)
    # space_exist, _ = is_space_exist(vc)
    # print("*****frist is space exist:::",space_exist)
    # if not space_exist:
    #     create_space(vc)
    # space_exist2, _ = is_space_exist(vc)
    # print("*****second is space exist:::",space_exist2)
    # ids = upsert_document(vc)
    # print("docment_id",ids)
    # query_documents(ids[:3])
    # query_documents(['7471538621046543493',"chjwgvqovhqjvwqj",])
    # #
    filter_expr={
        "operator": "AND",
        "conditions": [
            {
                "operator": ">",
                "field": "book_num",
                "value": 18
            },
            # {
            #     "operator": "<=",
            #     "field": "book_num",
            #     "value": 34
            # },
            {
                "operator": "IN",
                "field": "book_name",
                "value": ["hlnuoe8l","edjn9542"]
            },
        ]
    }
    # ids=['4204319368660532182', '282353212133560098', '-6006257984836246952', '6647000306584268890', '48938021142755877', '1687032939197080263', '-2829022327783810394', '1015957419330146982']
    query_documnet_by_filter(["1176972494740639417",'-8276417892909676457','8296119589647582104'],filter_expr)
    search_documets()
    search_doc_by_filter(filter_expr)
    delete_space(vc)
    drop_database(vc)
# vc.drop_database("database1")
# db = Database(name="fjakjfks")
# db.create()
# space_schema = create_space_schema()
# print(space_schema.dict())
# inv_pq = IvfPQIndex("book_vec_idx", 7000, MetricType.Inner_product, 40, 700)
# print(isinstance(inv_pq, Index))

data = "/ vearch / space / {sapce_name} / field / {field}"
# / vearch / space / {sapce_name} / index / {index}
