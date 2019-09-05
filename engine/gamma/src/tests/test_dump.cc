/**
 * Copyright (c) The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

#include "test.h"
#include <cmath>
#include <fcntl.h>
#include <fstream>
#include <functional>
#include <future>
#include <sys/mman.h>
#include <gtest/gtest.h>

namespace Test {

struct Options {
  Options() {
    nprobe = 10;
    doc_id = 0;
    d = 512;
    max_doc_size = 10000 * 30;
    add_doc_num = 10000 * 10;
    search_num = 10000 * 1;
    fields_vec = {"sku", "_id", "cid1", "cid2", "cid3"};
    fields_type = {STRING, STRING, INT, INT, INT};
    vector_name = "abc";
    path = "files";
    string log_dir = "log";
    model_id = "model";
    retrieval_type = "IVFPQ"; // GPU_IVFPQ
    store_type = "MemoryOnly";
    profiles.resize(max_doc_size * fields_vec.size());
    engine = nullptr;
  }

  int nprobe;
  int doc_id;
  int d;
  int max_doc_size;
  long add_doc_num;
  int search_num;
  std::vector<string> fields_vec;
  std::vector<enum DataType> fields_type;
  string path;
  string log_dir;
  string vector_name;
  string model_id;
  string retrieval_type;
  string store_type;

  std::vector<string> profiles;
  float *feature;

  char *docids_bitmap_;
  void *engine;
};

static struct Options opt;

int AddDoc(void *engine, int doc_num, int interval = 0) {
  for (int i = 0; i < doc_num; ++i) {
    double start = utils::getmillisecs();
    Field **fields = MakeFields(opt.fields_vec.size() + 1);

    for (size_t j = 0; j < opt.fields_vec.size(); ++j) {
      enum DataType data_type = opt.fields_type[j];
      ByteArray *name = StringToByteArray(opt.fields_vec[j]);
      ByteArray *value;

      string &data =
          opt.profiles[(uint64_t)opt.doc_id * opt.fields_vec.size() + j];
      if (opt.fields_type[j] == INT) {
        value = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
        value->value = static_cast<char *>(malloc(sizeof(int)));
        value->len = sizeof(int);
        int v = atoi(data.c_str());
        memcpy(value->value, &v, value->len);
      } else if (opt.fields_type[j] == LONG) {
        value = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
        value->value = static_cast<char *>(malloc(sizeof(long)));
        value->len = sizeof(long);
        long v = atol(data.c_str());
        memcpy(value->value, &v, value->len);
      } else {
        value = StringToByteArray(data);
      }
      ByteArray *source =
          StringToByteArray(string("jfs/t1/46413/10/6998/121644/"
                                   "5d493cfaE53b7c078/c4e2526e8f8a698f.jpg"));
      Field *field = MakeField(name, value, source, data_type);
      SetField(fields, j, field);
    }

    ByteArray *value =
        FloatToByteArray(opt.feature + (uint64_t)opt.doc_id * opt.d, opt.d);
    ByteArray *name = StringToByteArray(opt.vector_name);
    ByteArray *source = StringToByteArray(string(
        "jfs/t1/46413/10/6998/121644/5d493cfaE53b7c078/c4e2526e8f8a698f.jpg"));
    Field *field = MakeField(name, value, source, VECTOR);
    SetField(fields, opt.fields_vec.size(), field);

    Doc *doc = MakeDoc(fields, opt.fields_vec.size() + 1);
    AddOrUpdateDoc(engine, doc);
    DestroyDoc(doc);
    ++opt.doc_id;
    double elap = utils::getmillisecs() - start;
    if (i % 10000 == 0) {
      LOG(INFO) << "AddDoc use [" << elap << "]ms";
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(interval));
  }
  return 0;
}

int SearchThread(void *engine, int num, int start_id) {
  int idx = 0;
  double time = 0;
  int failed_count = 0;
  int req_num = 1000;
  string error;
  while (idx < num) {
    double start = utils::getmillisecs();
    VectorQuery **vector_querys = MakeVectorQuerys(1);
    int docid = start_id + idx;
    ByteArray *value = FloatToByteArray(opt.feature + (uint64_t)docid * opt.d,
                                        opt.d * req_num);
    VectorQuery *vector_query = MakeVectorQuery(
        StringToByteArray(opt.vector_name), value, 0, 10000, 0.1, 0);
    SetVectorQuery(vector_querys, 0, vector_query);

    Request *request = MakeRequest(10, vector_querys, 1, nullptr, 0, nullptr, 0,
                                   nullptr, 0, req_num, 0, nullptr);

    Response *response = Search(engine, request);
    for (int i = 0; i < response->req_num; ++i) {
      int ii = docid + i;
      string msg = "docid=" + std::to_string(ii) + ", ";
      SearchResult *results = GetSearchResult(response, i);
      if (results->result_num <= 0) {
        continue;
      }
      msg += string("total [") + std::to_string(results->total) + "], ";
      msg +=
          string("result_num [") + std::to_string(results->result_num) + "], ";
      for (int j = 0; j < results->result_num; ++j) {
        ResultItem *result_item = GetResultItem(results, j);
        msg += string("score [") + std::to_string(result_item->score) + "], ";
        printDoc(result_item->doc, msg);
        msg += "\n";
      }
      if (abs(GetResultItem(results, 0)->score - 1.0) < 0.001) {
        if (ii % 100000 == 0) {
          LOG(INFO) << msg;
        }
      } else {
        if (!bitmap::test(opt.docids_bitmap_, ii)) {
          LOG(ERROR) << msg;
          error += std::to_string(ii) + ",";
          bitmap::set(opt.docids_bitmap_, ii);
          failed_count++;
        }
      }
    }

    DestroyRequest(request);
    DestroyResponse(response);
    double elap = utils::getmillisecs() - start;
    time += elap;
    if (idx % 10000 == 0) {
      LOG(INFO) << "search time [" << time / 10000 << "]ms";
      time = 0;
    }
    idx += req_num;
  }
  LOG(ERROR) << error;
  return failed_count;
}

int SearchOne(void *engine, int docid) {
  int req_num = 1;
  double start = utils::getmillisecs();
  VectorQuery **vector_querys = MakeVectorQuerys(1);
  ByteArray *value =
      FloatToByteArray(opt.feature + (uint64_t)docid * opt.d, opt.d * req_num);
  VectorQuery *vector_query = MakeVectorQuery(
      StringToByteArray(opt.vector_name), value, 0, 10000, 0.1, 0);
  SetVectorQuery(vector_querys, 0, vector_query);

  Request *request = MakeRequest(10, vector_querys, 1, nullptr, 0, nullptr, 0,
                                 nullptr, 0, req_num, 0, nullptr);

  Response *response = Search(engine, request);

  string msg = "docid=" + std::to_string(docid) + ", ";
  SearchResult *results = GetSearchResult(response, 0);
  if (results->result_num <= 0) {
    return 1;
  }
  msg += string("total [") + std::to_string(results->total) + "], ";
  msg += string("result_num [") + std::to_string(results->result_num) + "], ";
  for (int j = 0; j < results->result_num; ++j) {
    ResultItem *result_item = GetResultItem(results, j);
    msg += string("score [") + std::to_string(result_item->score) + "], ";
    printDoc(result_item->doc, msg);
    msg += "\n";
  }
  double elap = utils::getmillisecs() - start;
  LOG(INFO) << "search time [" << elap << "]ms";
  DestroyRequest(request);
  LOG(INFO) << msg;
  if (abs(GetResultItem(results, 0)->score - 1.0) < 0.001) {
    DestroyResponse(response);
    return 0;
  } else {
    DestroyResponse(response);
    return 1;
  }
}

int RandomSearchThread(void *engine, int num, int max_id) {
  int idx = 0;
  double time = 0;
  int failed_count = 0;
  int req_num = 100;
  string error;
  std::srand(std::time(nullptr));
  while (idx < num) {
    double start = utils::getmillisecs();
    VectorQuery **vector_querys = MakeVectorQuerys(1);
    int docid = std::rand() % max_id;
    ByteArray *value = FloatToByteArray(opt.feature + (uint64_t)docid * opt.d,
                                        opt.d * req_num);
    VectorQuery *vector_query = MakeVectorQuery(
        StringToByteArray(opt.vector_name), value, 0, 10000, 0.1, 0);
    SetVectorQuery(vector_querys, 0, vector_query);

    // string c1_lower = opt.profiles[idx * (opt.fields_vec.size()) + 4];
    // string c1_upper = opt.profiles[idx * (opt.fields_vec.size()) + 4];
    // LOG(INFO) << "idx=" << idx << ", cid3=" << c1_lower;
    // string cid = "cid3";
    // RangeFilter **range_filters = MakeRangeFilters(1);
    // RangeFilter *range_filter =
    //     MakeRangeFilter(StringToByteArray(cid), StringToByteArray(c1_lower),
    //                     StringToByteArray(c1_upper), false, true);
    // SetRangeFilter(range_filters, 0, range_filter);
    // Request *request = MakeRequest(10, vector_querys, 1, nullptr, 0,
    //                                range_filters, 1, nullptr, 0, req_num, 0,
    //                                nullptr);
    Request *request = MakeRequest(10, vector_querys, 1, nullptr, 0, nullptr, 0,
                                   nullptr, 0, req_num, 0, nullptr);

    Response *response = Search(engine, request);
    for (int i = 0; i < response->req_num; ++i) {
      int ii = docid + i;
      string msg = "docid=" + std::to_string(ii) + ", ";
      SearchResult *results = GetSearchResult(response, i);
      if (results->result_num <= 0) {
        continue;
      }
      msg += string("total [") + std::to_string(results->total) + "], ";
      msg +=
          string("result_num [") + std::to_string(results->result_num) + "], ";
      for (int j = 0; j < results->result_num; ++j) {
        ResultItem *result_item = GetResultItem(results, j);
        msg += string("score [") + std::to_string(result_item->score) + "], ";
        printDoc(result_item->doc, msg);
        msg += "\n";
      }
      if (abs(GetResultItem(results, 0)->score - 1.0) < 0.001) {
        if (ii % 10000 == 0) {
          LOG(INFO) << msg;
        }
      } else {
        if (!bitmap::test(opt.docids_bitmap_, ii)) {
          LOG(ERROR) << msg;
          error += std::to_string(ii) + ",";
          bitmap::set(opt.docids_bitmap_, ii);
          failed_count++;
        }
      }
    }

    DestroyRequest(request);
    DestroyResponse(response);
    double elap = utils::getmillisecs() - start;
    time += elap;
    if (idx % 10000 == 0) {
      LOG(INFO) << "search time [" << time / 10000 << "]ms";
      time = 0;
    }
    idx += req_num;
  }
  LOG(ERROR) << error;
  return failed_count;
}

void UpdateThread(void *engine) {
  int doc_id = 1;
  Field **fields = MakeFields(opt.fields_vec.size() + 1);

  for (size_t j = 0; j < opt.fields_vec.size(); ++j) {
    enum DataType data_type = opt.fields_type[j];
    ByteArray *name = StringToByteArray(opt.fields_vec[j]);
    ByteArray *value;

    string &data = opt.profiles[(uint64_t)doc_id * opt.fields_vec.size() + j];
    if (opt.fields_type[j] == INT) {
      value = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
      value->value = static_cast<char *>(malloc(sizeof(int)));
      value->len = sizeof(int);
      int v = atoi("88888");
      memcpy(value->value, &v, value->len);
    } else if (opt.fields_type[j] == LONG) {
      value = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
      value->value = static_cast<char *>(malloc(sizeof(long)));
      value->len = sizeof(long);
      long v = atol(data.c_str());
      memcpy(value->value, &v, value->len);
    } else {
      value = StringToByteArray(data);
    }
    ByteArray *source = StringToByteArray(string("cccccccccccccccc"));
    Field *field = MakeField(name, value, source, data_type);
    SetField(fields, j, field);
  }

  ByteArray *value =
      FloatToByteArray(opt.feature + (uint64_t)doc_id * opt.d, opt.d);
  ByteArray *name = StringToByteArray(opt.vector_name);
  ByteArray *source = StringToByteArray(string("ccccccccccc"));
  Field *field = MakeField(name, value, source, VECTOR);
  SetField(fields, opt.fields_vec.size(), field);

  Doc *doc = MakeDoc(fields, opt.fields_vec.size() + 1);
  UpdateDoc(engine, doc);
  DestroyDoc(doc);
}

TEST(Search, Init) {
  setvbuf(stdout, (char *)NULL, _IONBF, 0);
  int bitmap_bytes_size = 0;
  int ret =
      bitmap::create(opt.docids_bitmap_, bitmap_bytes_size, opt.max_doc_size);
  if (ret != 0) {
    LOG(ERROR) << "Create bitmap failed!";
  }
  ASSERT_NE(opt.docids_bitmap_, nullptr);
  Config *config = MakeConfig(StringToByteArray(opt.path), opt.max_doc_size);
  SetLogDictionary(StringToByteArray(opt.log_dir));
  opt.engine = Init(config);
  DestroyConfig(config);
  EXPECT_NE(opt.engine, nullptr);
}

TEST(Search, CreateTable) {
  ByteArray *table_name = MakeByteArray("test", 4);
  FieldInfo **field_infos = MakeFieldInfos(opt.fields_vec.size());

  for (size_t i = 0; i < opt.fields_vec.size(); ++i) {
    BOOL do_index = TRUE;
    if (opt.fields_type[i] == STRING)
      do_index = FALSE;
    FieldInfo *field_info = MakeFieldInfo(StringToByteArray(opt.fields_vec[i]),
                                          opt.fields_type[i], do_index);
    SetFieldInfo(field_infos, i, field_info);
  }

  VectorInfo **vectors_info = MakeVectorInfos(1);
  VectorInfo *vector_info = MakeVectorInfo(
      StringToByteArray(opt.vector_name), FLOAT, opt.d,
      StringToByteArray(opt.model_id), StringToByteArray(opt.retrieval_type),
      StringToByteArray(opt.store_type));
  SetVectorInfo(vectors_info, 0, vector_info);

  Table *table = MakeTable(table_name, field_infos, opt.fields_vec.size(),
                           vectors_info, 1, kIVFPQParam);
  enum ResponseCode ret = CreateTable(opt.engine, table);
  DestroyTable(table);
  EXPECT_EQ(ret, 0);
}

TEST(Search, Add) {
  string profile_file = "profile_100m.txt";
  string feature_file = "feat_100m_512float.dat";

  int idx = 0;
  std::ifstream fin;
  fin.open(profile_file.c_str());
  std::string str;
  while (!fin.eof()) {
    std::getline(fin, str);
    if (str == "")
      break;
    auto profile = std::move(utils::split(str, "\t"));
    size_t i = 0;
    for (const auto &p : profile) {
      opt.profiles[idx * opt.fields_vec.size() + i] = p;
      ++i;
      if (i > opt.fields_vec.size() - 1) {
        break;
      }
    }

    ++idx;
    if (idx >= opt.max_doc_size) {
      break;
    }
  }
  if (opt.max_doc_size < idx) {
    opt.max_doc_size = idx;
  }
  LOG(INFO) << opt.max_doc_size;
  fin.close();

  int fd = open(feature_file.c_str(), O_RDONLY, 0);
  opt.feature =
      static_cast<float *>(mmap(NULL, opt.max_doc_size * sizeof(float) * opt.d,
                                PROT_READ, MAP_SHARED, fd, 0));
  close(fd);

  int ret = AddDoc(opt.engine, opt.add_doc_num);
  EXPECT_EQ(ret, 0);
}

TEST(Search, BuildIndex) {
  std::thread t(BuildIndex, opt.engine);
  t.detach();

  while (GetIndexStatus(opt.engine) != INDEXED) {
    std::this_thread::sleep_for(std::chrono::seconds(2));
  }

  LOG(INFO) << "Indexed!";
}

TEST(Search, SearchThread) {
  int search_thread_num = 1;
  std::thread t_searchs[search_thread_num];

  std::srand(std::time(nullptr));
  int idx = std::rand() % 10;
  int start_id = idx * 10000;

  LOG(INFO) << "SearchThread start id=" << start_id;
  std::function<int()> func_search =
      std::bind(SearchThread, opt.engine, 10000, start_id);
  std::future<int> search_futures[search_thread_num];
  std::packaged_task<int()> tasks[search_thread_num];

  for (int i = 0; i < search_thread_num; ++i) {
    tasks[i] = std::packaged_task<int()>(func_search);
    search_futures[i] = tasks[i].get_future();
    t_searchs[i] = std::thread(std::move(tasks[i]));
  }

  std::this_thread::sleep_for(std::chrono::seconds(2));

  // std::function<int(void *)> add_func =
  //     std::bind(AddDoc, std::placeholders::_1, 1 * 1, 1);
  // std::thread add_thread(add_func, opt.engine);

  // get search results
  for (int i = 0; i < search_thread_num; ++i) {
    search_futures[i].wait();
    EXPECT_LE(search_futures[i].get(), 500);
    t_searchs[i].join();
  }

  // add_thread.join();
}

TEST(Search, Dump) {
  int ret = Dump(opt.engine);
  EXPECT_EQ(ret, 0);

  ret = Dump(opt.engine);
  EXPECT_EQ(ret, 0);

  ret = AddDoc(opt.engine, opt.add_doc_num);
  EXPECT_EQ(ret, 0);

  std::this_thread::sleep_for(std::chrono::seconds(5));

  ret = Dump(opt.engine);
  EXPECT_EQ(ret, 0);

  Close(opt.engine);
  opt.engine = nullptr;
  delete opt.docids_bitmap_;
}

TEST(Search, Load) {
  int bitmap_bytes_size = 0;
  int ret =
      bitmap::create(opt.docids_bitmap_, bitmap_bytes_size, opt.max_doc_size);
  if (ret != 0) {
    LOG(ERROR) << "Create bitmap failed!";
  }
  ASSERT_NE(opt.docids_bitmap_, nullptr);
  Config *config = MakeConfig(StringToByteArray(opt.path), opt.max_doc_size);
  opt.engine = Init(config);
  DestroyConfig(config);
  ASSERT_NE(opt.engine, nullptr);

  ret = Load(opt.engine);
  ASSERT_EQ(ret, 0);
}

TEST(Search, BuildIndexAfterLoad) {
  std::thread t(BuildIndex, opt.engine);
  t.detach();
  while (GetIndexStatus(opt.engine) != INDEXED) {
    std::this_thread::sleep_for(std::chrono::seconds(2));
  }

  LOG(INFO) << "Indexed finished after load!";
}

TEST(Search, SearchThreadAfterLoad) {
  int search_thread_num = 1;
  std::thread t_searchs[search_thread_num];

  std::srand(std::time(nullptr));
  int idx = std::rand() % 8;
  int start_id = 100000 + idx * 10000;
  LOG(INFO) << "SearchThreadAfterLoad start id=" << start_id;
  std::function<int()> func_search =
      std::bind(SearchThread, opt.engine, 20000, start_id);
  std::future<int> search_futures[search_thread_num];
  std::packaged_task<int()> tasks[search_thread_num];

  for (int i = 0; i < search_thread_num; ++i) {
    tasks[i] = std::packaged_task<int()>(func_search);
    search_futures[i] = tasks[i].get_future();
    t_searchs[i] = std::thread(std::move(tasks[i]));
  }

  std::this_thread::sleep_for(std::chrono::seconds(2));

  std::function<int(void *)> add_func =
      std::bind(AddDoc, std::placeholders::_1, 10000 * 1, 1);
  // std::thread add_thread(add_func, opt.engine);

  // get search results
  for (int i = 0; i < search_thread_num; ++i) {
    search_futures[i].wait();
    EXPECT_LE(search_futures[i].get(), 500);
    t_searchs[i].join();
  }

  // add_thread.join();
}

TEST(Search, DumpAfterLoad) {
  int ret = Dump(opt.engine);
  EXPECT_EQ(ret, 0);

  ret = AddDoc(opt.engine, opt.add_doc_num);
  EXPECT_EQ(ret, 0);

  std::this_thread::sleep_for(std::chrono::seconds(4));

  ret = Dump(opt.engine);
  EXPECT_EQ(ret, 0);

  Close(opt.engine);
  opt.engine = nullptr;
  delete opt.docids_bitmap_;
}

TEST(Search, SecondLoadAndSearch) {
  int bitmap_bytes_size = 0;
  int ret =
      bitmap::create(opt.docids_bitmap_, bitmap_bytes_size, opt.max_doc_size);
  if (ret != 0) {
    LOG(ERROR) << "Create bitmap failed!";
  }
  ASSERT_NE(opt.docids_bitmap_, nullptr);
  Config *config = MakeConfig(StringToByteArray(opt.path), opt.max_doc_size);
  opt.engine = Init(config);
  DestroyConfig(config);
  ASSERT_NE(opt.engine, nullptr);

  // load
  ret = Load(opt.engine);
  ASSERT_EQ(ret, 0);

  // build index
  std::thread t(BuildIndex, opt.engine);
  t.detach();
  while (GetIndexStatus(opt.engine) != INDEXED) {
    std::this_thread::sleep_for(std::chrono::seconds(2));
  }
  LOG(INFO) << "Indexed finished after load!";

  // SearchOne(opt.engine, 244016);

  // search

  int search_thread_num = 1;
  std::thread t_searchs[search_thread_num];

  LOG(INFO) << "################second load search number=" << opt.doc_id;
  std::function<int()> func_search =
      std::bind(SearchThread, opt.engine, 300000, 0);
  std::future<int> search_futures[search_thread_num];
  std::packaged_task<int()> tasks[search_thread_num];

  for (int i = 0; i < search_thread_num; ++i) {
    tasks[i] = std::packaged_task<int()>(func_search);
    search_futures[i] = tasks[i].get_future();
    t_searchs[i] = std::thread(std::move(tasks[i]));
  }

  // get search results
  for (int i = 0; i < search_thread_num; ++i) {
    search_futures[i].wait();
    EXPECT_LE(search_futures[i].get(), 500);
    t_searchs[i].join();
  }
}

TEST(Search, Close) {
  Close(opt.engine);
  opt.engine = nullptr;
  delete opt.docids_bitmap_;
  munmap(opt.feature, opt.max_doc_size * sizeof(float) * opt.d);
}

int main(int argc, char **argv) {
  testing::InitGoogleTest(&argc, argv);
  return RUN_ALL_TESTS();
}

} // namespace Test
