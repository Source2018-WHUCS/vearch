from vearch.config import Config
from vearch.core.db import Database, Space
from vearch.core.vearch import Vearch
from vearch.schema.field import Field
from vearch.schema.space import SpaceSchema
from vearch.utils import DataType, MetricType, VectorInfo
from vearch.schema.index import IvfPQIndex, Index, ScalarIndex, HNSWIndex,IvfFlatIndex,BinaryIvfIndex,FlatIndex,GPUIvfPQIndex
from vearch.filter import Filter,Condition,FieldValue,Conditions
import logging
from typing import List
import json
from vearch.exception import SpaceException, DocumentException, VearchException


logger = logging.getLogger("vearch")


def create_space_schema() -> SpaceSchema:
    book_name = Field("book_name", DataType.STRING, desc="the name of book", index=ScalarIndex("book_name_idx"))
    book_num=Field("book_num", DataType.INTEGER, desc="the num of book",index=ScalarIndex("book_num_idx"))
    book_vector = Field("book_character", DataType.VECTOR,
                        FlatIndex("book_vec_idx", MetricType.Inner_product), dimension=512)
    ractor_address = Field("ractor_address", DataType.STRING, desc="the place of the book put")
    space_schema = SpaceSchema("book_info", fields=[book_name,book_num, book_vector, ractor_address])
    return space_schema


def create_database(vc: Vearch,db):
    logger.debug(vc.client.host)
    ret = vc.create_database(db)
    logger.debug(ret.dict_str())
    return ret

def list_databases(vc: Vearch):
    logger.debug(vc.client.host)
    ret = vc.list_databases()
    logger.debug(ret)
    return ret

    
def list_spaces(vc: Vearch,db):
    logger.debug(vc.client.host)
    ret = vc.list_spaces(db)
    logger.debug(ret)
    return ret

def create_space(vc: Vearch,db, space_schema):
    ret = vc.create_space(db, space_schema)
    print("######",ret.data, ret.msg)
    return ret
    
# def upsert_document(vc: Vearch) -> List:
#     import random
#     ractor = ["ractor_logical", "ractor_industry", "ractor_philosophy"]
#     book_name_template = "abcdefghijklmnopqrstuvwxyz0123456789"
#     data = []
#     num=[12,34,56,74,53,11,14,9]
#     for i in range(8):
#         book_item = ["".join(random.choices(book_name_template, k=8)),
#                      num[i],
#                      [random.uniform(0, 1) for _ in range(512)],
#                      ractor[random.randint(0, 2)]]
#         data.append(book_item)
#         logger.debug(book_item)
#     space = Space("database_test", "book_info")
#     ret = space.upsert(data)
#     assert len(ret.get_document_ids()) >= 0
#     if ret:
#         logger.debug("upsert result:" + str(ret.get_document_ids()))
#         return ret.get_document_ids()
#     return []


def upsert_document_from_vearch(vc: Vearch,db,space_name,data) -> List:
    ret = vc.upsert(db,space_name,data)
    if ret:
        print(ret.__dict__,ret.code)
        logger.debug("upsert result:" + str(ret.get_document_ids()))
        print(len(ret.get_document_ids()))

def query_documents_from_vearch(vc: Vearch,db,space_name,ids,data):
    ret = vc.query(db,space_name,ids,data)
    print(ret,ret.__dict__)
    print("query document",ret.documents)


def query_documents(ids: List):
    space = Space("database_test", "book_info")
    ret = space.query(ids)
    print("query document",ret.documents)


def search_documets():
    import random
    space = Space("database_test", "book_info")
    
    feature = [random.uniform(0, 1) for _ in range(512)]
    vi = VectorInfo("book_character", feature)
    ret = space.search(vector_infos=[vi, ],limit=7)
    print("search document",ret.documents)


def search_documets_from_vearch(vc: Vearch):
    import random
    feature = [random.uniform(0, 1) for _ in range(512)]
    vi = VectorInfo("book_character", feature)
    ret = vc.search("database_test", "book_info",vector_infos=[vi, ],limit=7)
    print("search document",ret.documents)

def query_documnet_by_filter_of_vearch(vc: Vearch,filters):
    ret = vc.query("database_test", "book_info",filter=filters,limit=2)
    print("search document",ret.documents)


def search_doc_by_filter_of_vearch(vc: Vearch, filters):
    import random
    feature = [random.uniform(0, 1) for _ in range(512)]
    vi = VectorInfo("book_character", feature)
    
    ret = vc.search("database_test", "book_info",vector_infos=[vi, ],filter=filters,limit=3)
    print("search document",ret.documents)

            
def is_database_exist(vc: Vearch,db):
    ret = vc.is_database_exist(db)
    return ret


def is_space_exist(vc: Vearch,db,space_name):
    ret,d,spaces = vc.is_space_exist(db,space_name)
    logger.debug(ret)
    return ret,d,spaces


def delete_space(vc: Vearch,db,space_name):
    ret = vc.drop_space(db,space_name)
    print(ret.__dict__,ret.data, ret.msg)


def drop_database(vc: Vearch,db):
    logger.debug(vc.client.host)
    ret = vc.drop_database(db)
    print(ret.__dict__,ret.code)
def query_documnet_by_filter(filters):
    
    space = Space("database_test", "book_info")
    ret = space.query(filter=filters,limit=2)
    print("query document",ret.documents)


def search_doc_by_filter(filters):
    import random
    space = Space("database_test", "book_info")
    feature = [random.uniform(0, 1) for _ in range(512)]
    vi = VectorInfo("book_character", feature)
    
    ret = space.search(vector_infos=[vi, ],filter=filters,limit=3)
    if ret is not None:
        print("search document",ret.documents)


if __name__ == "__main__":
    # should set your host url
    config = Config(host="http://liama-index-router.vectorbase.svc.sq01.n.jd.local", token="secret")
    vc = Vearch(config)
    db_exist_ret=is_database_exist(vc,"database_test_not_exist")
    print("is_database_exist",db_exist_ret)
    
    if not db_exist_ret:
        create_ret=create_database(vc,"database_test_not_exist")
        print("create_ret",create_ret.__dict__)
  
    dbs=list_databases(vc)
    print("dbs",dbs,dbs[0].__dict__)
    book_name = Field("book_name", DataType.STRING, desc="the name of book", index=ScalarIndex("book_name_idx"))
    book_num = Field("book_num", DataType.INTEGER, desc="the num of book",index=ScalarIndex("book_num_idx"))
    
    space_exist, res,_ = is_space_exist(vc,"database_test_not_exist","book_info_ivfpq")
    print(" is space exist:::",space_exist)
    # if not space_exist:
    # cr_ret_q=create_space(vc,"database_test_not_exist",SpaceSchema("book_info_ivfpq", [book_name, book_num,Field("book_character", DataType.VECTOR, IvfPQIndex("book_vec_idx", 10000, MetricType.Inner_product, 2048, 8), dimension=512)]))
    # print(cr_ret_q.__dict__)
    # cr_ret_t=create_space(vc,"database_test_not_exist",SpaceSchema("book_info_ivfflat", [book_name, book_num, Field("book_character", DataType.VECTOR, IvfFlatIndex("book_vec_idx", MetricType.Inner_product, 2048), dimension=512)]))
    # print(cr_ret_t.__dict__)
    
    # cr_ret_i=create_space(vc,"database_test_not_exist",SpaceSchema("book_info_biivf", [book_name, book_num, Field("book_character", DataType.VECTOR, BinaryIvfIndex("book_vec_idx", 2048), dimension=512)]))
    # print(cr_ret_i.__dict__)
    
    # cr_ret_f=create_space(vc,"database_test_not_exist",SpaceSchema("book_info_flat", [book_name, book_num, Field("book_character", DataType.VECTOR, FlatIndex("book_vec_idx", MetricType.Inner_product), dimension=512)]))
    # print(cr_ret_f.__dict__)
    # cr_ret_h=create_space(vc,"database_test_not_exist",SpaceSchema("book_info_hnsw", [book_name, book_num, Field("book_character", DataType.VECTOR, HNSWIndex("book_vec_idx", MetricType.Inner_product, 32, 40), dimension=512)]))
    # print(cr_ret_h.__dict__)
    # cr_ret_n=create_space(vc,"database_test_not_exist",SpaceSchema("book_info_ivfpq1", [book_name, book_num, Field("book_character", DataType.VECTOR, IvfPQIndex("book_vec_idx", 10000, MetricType.Inner_product, 2048, 8), dimension=512)]))
    # print(cr_ret_n.__dict__)
    
    # space_exist1,r1, schema = is_space_exist(vc,"database_test_not_exist","book_info_ivfpq")
    # print(" is space exist:¥¥¥¥::",space_exist1,r1,schema)
    # space_exist2, _,schema2 = is_space_exist(vc,"database_new","book_info_ivfpq")
    # print(" is space exist:%%%%::",space_exist2,schema2)
#     ##db在，表不在，不会报错
    # space_exist3,_, schema3 = is_space_exist(vc,"database_test_not_exist","book_info_ivfq")
    # print(" is space exist:****::",space_exist3,schema3)
    
# #     #db不再--BADCASE
    # space_exist4,_, schema4 = is_space_exist(vc,"database_test_n","book_info_ivfpq")
    # print(" is space exist@@@@:::",space_exist4,schema4)
    
#     sps=list_spaces(vc,"database_test_not_exist")
#     print(sps)
   
#     sps_bad=list_spaces(vc,"")
#     print(sps_bad)
    
    

#     book_name_template = ["qdbwjfwv","acwlvvq","cwvwvqqc","cwvwvveq","cevqbgnd","vrhuofnm","wkvwveve","cwbtjimu"]
#     num=[12,34,56,74,53,11,14,9]
#     data_complete = []  
#     data_miss = []  
#     import random
#     for i in range(8):
#         book_item = [book_name_template[i],
#                      num[i],
#                      [random.uniform(0, 1) for _ in range(512)]]
#         book_item_m = [book_name_template[i],
#                      [random.uniform(0, 1) for _ in range(512)]]
#     data_complete.append(book_item)
#     data_miss.append(book_item_m)
    
#     ids=upsert_document_from_vearch(vc,"database_test_not_exist","book_info_ivfpq",data_complete)
# #     #DADCASE--data is empty
#     ids_empty=upsert_document_from_vearch(vc,"database_test_not_exist","book_info_ivfpq",[])
#     ids_datamiss=upsert_document_from_vearch(vc,"database_test_not_exist","book_info_ivfpq",data_miss)
#     ids_db_notexist=upsert_document_from_vearch(vc,"database_test_n","book_info_ivfpq",data_complete)
#     ids_space_not_exist=upsert_document_from_vearch(vc,"database_test_not_exist","book_info_ivfq",data_miss)

    # conditons = [Condition(operator = '<', fv = FieldValue(field = "book_num",value = 35)),
    #          Condition(operator = '>', fv = FieldValue(field = "book_num",value = 12))
    #       ]
    # delete_filters = Filter(operator = "AND",conditions = conditons)
    # delete_ret=vc.delete("database_test_not_exist","book_info_ivfpq", filter = delete_filters)
    # print(delete_ret.__dict__)
    # delete_ret1=vc.delete("database_test_not_exist","book_info_ivfpq", document_ids=["cwevrevbee1134"],)
    # print(delete_ret1.__dict__,delete_ret1.code)
    
    conditons_query = [Condition(operator = '>', fv = FieldValue(field = "book_num",value = 0)),
                     Condition(operator = 'IN', fv = FieldValue(field = "book_name",value = ["bpww57nu","sykboivx","edjn9542"]))
                  ]
    query_filters = Filter(operator = "AND",conditions = conditons_query)
    ret_q=vc.query("database_test_not_exist","book_info_ivfpq", ["chjebvbevbhejbve"])
    print(ret_q.__dict__,ret_q.code)
    
#     #TODO:badcase
    # ret_q1=vc.query("database_test_no","book_info_ivfpq",["chjebvbevbhejbve"])
    # print(ret_q1.__dict__,ret_q1.code)
    # ret_q2=vc.query("database_test_not_exist","book_info_ivfpq")
    # print(ret_q2.__dict__,ret_q2.code)
    # ret_q3=vc.query("database_test_not_exist","book_info_ivpq",["chjebvbevbhejbve"])
    # print(ret_q3.__dict__,ret_q3.code)
    
    import random
    vi = VectorInfo("book_character", [random.uniform(0, 1) for _ in range(512)])
    conditons_search = [Condition(operator = '>', fv = FieldValue(field = "book_num",value = 0)),]
    search_filters = Filter(operator = "AND",conditions = conditons_search)
    ret_s=vc.search("database_test_n","book_info_ivpq",[vi,],search_filters,3)
    print(ret_s.__dict__,ret_s.code)
    
    ret=delete_space(vc,"db_test", "book_info_ivfflat")
    ret=delete_space(vc,"database_test_not_exist", "book_info_ivflat")
    ret=delete_space(vc,"database_test_not_exist", "book_info_ivfpq1")

    ret=delete_space(vc,"database_test_not_exist", "book_info_ivfpq")
    ret=delete_space(vc,"database_test_not_exist", "book_info_ivfflat")
    ret=delete_space(vc,"database_test_not_exist", "book_info_biivf")
    ret=delete_space(vc,"database_test_not_exist", "book_info_flat")
    ret=delete_space(vc,"database_test_not_exist", "book_info_hnsw")

    drop_database(vc,"database_test_not_exist")

