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

namespace Test {

struct Options {
  Options() {
    nprobe = 10;
    doc_id = 0;
    d = 512;
    max_doc_size = 10000 * 10;
    add_doc_num = 10000 * 10;
    search_num = 10000 * 10;
    fields_vec = {"sku", "_id", "cid1", "cid2", "cid3"};
    fields_type = {STRING, STRING, INT, INT, INT};
    vector_name = "abc";
    profiles.resize(max_doc_size *fields_vec.size());
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
  string vector_name;

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

      string &data = opt.profiles[(uint64_t)opt.doc_id * opt.fields_vec.size() + j];
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
      ByteArray *source = StringToByteArray(string("jfs/t1/46413/10/6998/121644/5d493cfaE53b7c078/c4e2526e8f8a698f.jpg"));
      Field *field = MakeField(name, value, source, data_type);
      SetField(fields, j, field);
    }

    ByteArray *value = FloatToByteArray(opt.feature + (uint64_t)opt.doc_id * opt.d, opt.d);
    ByteArray *name = StringToByteArray(opt.vector_name);
    ByteArray *source = StringToByteArray(string("jfs/t1/46413/10/6998/121644/5d493cfaE53b7c078/c4e2526e8f8a698f.jpg"));
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

int SearchThread(void *engine, int num) {
  int idx = 0;
  double time = 0;
  int failed_count = 0;
  int req_num = 1000;
  while (idx < num) {
    double start = utils::getmillisecs();
    VectorQuery **vector_querys = MakeVectorQuerys(1);
    ByteArray *value = FloatToByteArray(opt.feature + (uint64_t)idx * opt.d, opt.d * req_num);
    VectorQuery *vector_query = MakeVectorQuery(StringToByteArray(opt.vector_name),
                                                value, 0, 10000, 0.1, 0);
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
    //                                range_filters, 1, nullptr, 0, req_num, 0, nullptr);
    Request *request = MakeRequest(10, vector_querys, 1, nullptr, 0, nullptr,
                                   0, nullptr, 0, req_num, 0, nullptr);

    Response *response = Search(engine, request);
    string msg = std::to_string(idx) + ", ";
    // for (int i = 0; i < response->req_num; ++i) {
    for (int i = 0; i < 1; ++i) {
      SearchResult *results = GetSearchResult(response, i);
      if (results->result_num <= 0) {
        continue;
      }
      for (int j = 0; j < results->result_num; ++j) {
        ResultItem *result_item = GetResultItem(results, j);
        msg += string("score [") + std::to_string(result_item->score) + "], ";
        msg += string("total [") + std::to_string(results->total) + "], ";

        printDoc(result_item->doc, msg);
        msg += "\n";
      }
      if (abs(GetResultItem(results, 0)->score - 1.0) < 0.00001) {
        if (idx % 100000 == 0) {
          LOG(INFO) << msg;
        }
      } else {
        if (!bitmap::test(opt.docids_bitmap_, idx)) {
          LOG(ERROR) << msg;
          bitmap::set(opt.docids_bitmap_, idx);
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
    if (idx >= opt.doc_id) {
      idx = 0;
      break;
    }
  }
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

  ByteArray *value = FloatToByteArray(opt.feature + (uint64_t)doc_id * opt.d, opt.d);
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
  int ret = bitmap::create(opt.docids_bitmap_, bitmap_bytes_size, opt.max_doc_size);
  ASSERT_NE(opt.docids_bitmap_, nullptr);
  string path = "files";
  string log_dir = "log";
  Config *config = MakeConfig(StringToByteArray(path), opt.max_doc_size);
  SetLogDictionary(StringToByteArray(log_dir));
  opt.engine = Init(config);
  DestroyConfig(config);
  EXPECT_NE(opt.engine, nullptr);
}

TEST(Search, CreateTable) {
  ByteArray *table_name = MakeByteArray("test", 4);
  FieldInfo **field_infos = MakeFieldInfos(opt.fields_vec.size());

  for (size_t i = 0; i < opt.fields_vec.size(); ++i) {
    FieldInfo *field_info =
        MakeFieldInfo(StringToByteArray(opt.fields_vec[i]), opt.fields_type[i], 1);
    SetFieldInfo(field_infos, i, field_info);
  }

  VectorInfo **vectors_info = MakeVectorInfos(1);
  string model_id = "model";
  string retrieval_type = "IVFPQ";
  // string retrieval_type = "GPU_IVFPQ";
  string store_type = "MemoryOnly";
  VectorInfo *vector_info = MakeVectorInfo(
      StringToByteArray(opt.vector_name), FLOAT, opt.d, StringToByteArray(model_id),
      StringToByteArray(retrieval_type), StringToByteArray(store_type));
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
    if (idx >= opt.add_doc_num) {
      break;
    }
  }
  if (opt.add_doc_num < idx) {
    opt.add_doc_num = idx;
  }
  LOG(INFO) << opt.add_doc_num;
  fin.close();

  int fd = open(feature_file.c_str(), O_RDONLY, 0);
  opt.feature = static_cast<float *>(
      mmap(NULL, opt.add_doc_num * sizeof(float) * opt.d, PROT_READ, MAP_SHARED, fd, 0));
  close(fd);

  int ret = AddDoc(opt.engine, opt.add_doc_num);
  EXPECT_EQ(ret, 0);
}

TEST(Search, BuildIndex) {
  std::thread t(BuildIndex, opt.engine);
  t.detach();


  // int search_thread_num = 1;
  // std::thread t_searchs[search_thread_num];

  // std::function<int()> func_search =
  //     std::bind(SearchThread, opt.engine, opt.search_num);
  // std::future<int> search_futures[search_thread_num];
  // std::packaged_task<int()> tasks[search_thread_num];

  // for (int i = 0; i < search_thread_num; ++i) {
  //   tasks[i] = std::packaged_task<int()>(func_search);
  //   search_futures[i] = tasks[i].get_future();
  //   t_searchs[i] = std::thread(std::move(tasks[i]));
  // }

  while (GetIndexStatus(opt.engine) != INDEXED) {
    std::this_thread::sleep_for(std::chrono::seconds(2));
  }

  // string docid =
  // "jfs/t17635/268/1735762492/316241/41ed9df9/5ad612cfN2a96dc78.jpg";
  // ByteArray *value = StringToByteArray(docid);
  // Doc *doc = GetDocByID(opt.engine, value);
  // DelDoc(opt.engine, value);
  // doc = GetDocByID(opt.engine, value);

  // std::thread t_add1(AddDocThread, opt.engine);
  // t_add1.join();

  // std::thread t_add(AddDocThread, opt.engine);
  // t_add.detach();

  // for (int i = 0; i < search_thread_num; ++i) {
  //   search_futures[i].wait();
  //   EXPECT_LE(search_futures[i].get(), 500);
  //   t_searchs[i].join();
  // }
  LOG(INFO) << "Indexed!";
}

TEST(Search, SearchThread) {
  // Search(opt.engine);
  // UpdateThread(opt.engine);

  // AddOneDoc(opt.engine);
  // Search(opt.engine);
  int search_thread_num = 1;
  std::thread t_searchs[search_thread_num];

  std::function<int()> func_search =
      std::bind(SearchThread, opt.engine, opt.search_num);
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
/*
TEST(Search, Dump) {
  int ret = Dump(opt.engine);
  EXPECT_EQ(ret, 0);

  Close(opt.engine);
  opt.engine = nullptr;
  delete opt.docids_bitmap_;
}

TEST(Search, Load) {
  setvbuf(stdout, (char *)NULL, _IONBF, 0);
  int bitmap_bytes_size = 0;
  opt.docids_bitmap_ = bitmap::create(opt.max_doc_size, bitmap_bytes_size);
  ASSERT_NE(opt.docids_bitmap_, nullptr);
  string path = "files";
  string log_dir = "log";
  Config *config = MakeConfig(StringToByteArray(path), opt.max_doc_size);
  opt.engine = Init(config);
  DestroyConfig(config);
  ASSERT_NE(opt.engine, nullptr);

  int ret = Load(opt.engine);
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

  int search_num = 10000 * 5;
  std::function<int()> func_search =
      std::bind(SearchThread, opt.engine, search_num);
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
  std::thread add_thread(add_func, opt.engine);

  // get search results
  for (int i = 0; i < search_thread_num; ++i) {
    search_futures[i].wait();
    EXPECT_LE(search_futures[i].get(), 500);
    t_searchs[i].join();
  }

  add_thread.join();
}

TEST(Search, DumpAfterLoad) {
  int ret = Dump(opt.engine);
  EXPECT_EQ(ret, 0);
}

TEST(Search, Close) {
  Close(opt.engine);
  opt.engine = nullptr;
  delete opt.docids_bitmap_;
  munmap(opt.feature, opt.add_doc_num * sizeof(float) * opt.d);
}
*/

int main(int argc, char **argv) {
  testing::InitGoogleTest(&argc, argv);
  return RUN_ALL_TESTS();
}

} // namespace Test
