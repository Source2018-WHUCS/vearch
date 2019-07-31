#include "test.h"
#include <cmath>
#include <fcntl.h>
#include <functional>
#include <future>
#include <sys/mman.h>
#include <fstream>

namespace Test {
#define ADD_DOC_NUM 8000000

int nprobe = 12;
int doc_id = 0;
int d = 512;
int max_doc_size = 21000000;
std::vector<string> fields_vec = {"sku", "_id", "cid1", "cid2", "cid3"};
std::vector<enum DataType> fields_type = {STRING, STRING, INT, INT, INT};
string vector_name = "abc";

std::vector<string> profiles(max_doc_size *fields_vec.size());
float *feature;
std::vector<std::string> sku_lines;

char *docids_bitmap_;

int AddDoc(void *engine, int doc_num, int interval = 0) {
  for (int i = 0; i < doc_num; ++i) {
    double start = utils::getmillisecs();
    Field **fields = MakeFields(fields_vec.size() + 1);

    for (size_t j = 0; j < fields_vec.size(); ++j) {
      enum DataType data_type = fields_type[j];
      ByteArray *name = StringToByteArray(fields_vec[j]);
      ByteArray *value;

      string &data = profiles[(uint64_t)doc_id * fields_vec.size() + j];
      if (fields_type[j] == INT) {
        value = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
        value->value = static_cast<char *>(malloc(sizeof(int)));
        value->len = sizeof(int);
        int v = atoi(data.c_str());
        memcpy(value->value, &v, value->len);
      } else if (fields_type[j] == LONG) {
        value = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
        value->value = static_cast<char *>(malloc(sizeof(long)));
        value->len = sizeof(long);
        long v = atol(data.c_str());
        memcpy(value->value, &v, value->len);
      } else {
        value = StringToByteArray(data);
      }
      ByteArray *source = StringToByteArray(string("aaa"));
      Field *field = MakeField(name, value, source, data_type);
      SetField(fields, j, field);
    }

    ByteArray *value = FloatToByteArray(feature + (uint64_t)doc_id * d, d);
    ByteArray *name = StringToByteArray(vector_name);
    ByteArray *source = StringToByteArray(string("aaa"));
    Field *field = MakeField(name, value, source, VECTOR);
    SetField(fields, fields_vec.size(), field);

    Doc *doc = MakeDoc(fields, fields_vec.size() + 1);
    AddOrUpdateDoc(engine, doc);
    DestroyDoc(doc);
    ++doc_id;
    double elap = utils::getmillisecs() - start;
    if (i % 10000 == 0) {
      LOG(INFO) << "AddDoc use [" << elap << "]ms";
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(interval));
  }
  return 0;
}

void AddOneDoc(void *engine) {
  Field **fields = MakeFields(fields_vec.size() + 1);
  int doc_id = 1;
  for (size_t j = 0; j < fields_vec.size(); ++j) {
    enum DataType data_type = fields_type[j];
    ByteArray *name = StringToByteArray(fields_vec[j]);
    ByteArray *value;

    string &data = profiles[(uint64_t)doc_id * fields_vec.size() + j];
    if (fields_type[j] == INT) {
      value = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
      value->value = static_cast<char *>(malloc(sizeof(int)));
      value->len = sizeof(int);
      int v = atoi(data.c_str());
      memcpy(value->value, &v, value->len);
    } else if (fields_type[j] == LONG) {
      value = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
      value->value = static_cast<char *>(malloc(sizeof(long)));
      value->len = sizeof(long);
      long v = atol(data.c_str());
      memcpy(value->value, &v, value->len);
    } else {
      value = StringToByteArray(data);
    }
    ByteArray *source = StringToByteArray(string("bbb"));
    Field *field = MakeField(name, value, source, data_type);
    SetField(fields, j, field);
  }

  ByteArray *value = FloatToByteArray(feature + (uint64_t)doc_id * d, d);
  ByteArray *name = StringToByteArray(vector_name);
  ByteArray *source = StringToByteArray(string("bbb"));
  Field *field = MakeField(name, value, source, VECTOR);
  SetField(fields, fields_vec.size(), field);

  Doc *doc = MakeDoc(fields, fields_vec.size() + 1);
  AddOrUpdateDoc(engine, doc);
  DestroyDoc(doc);
  ++doc_id;
}

int SearchThread(void *engine, int num) {
  int idx = 0;
  double time = 0;
  int ret = 0;
  int failed_count = 0;
  while (idx < num) {
    double start = utils::getmillisecs();
    VectorQuery **vector_querys = MakeVectorQuerys(1);
    ByteArray *value = FloatToByteArray(feature + (uint64_t)idx * d, d);

    VectorQuery *vector_query = MakeVectorQuery(StringToByteArray(vector_name),
                                                value, 0, 10000, 0.1, 0);
    SetVectorQuery(vector_querys, 0, vector_query);

    string line = sku_lines[idx];
    auto profile = std::move(utils::split(line, "\t"));
    string c1_lower = profile[4];
    string c1_upper = profile[4];
    // LOG(INFO) << "idx=" << idx << ", cid3=" << c1_lower;
    string cid = "cid3";
    RangeFilter **range_filters = MakeRangeFilters(1);
    RangeFilter *range_filter =
        MakeRangeFilter(StringToByteArray(cid), StringToByteArray(c1_lower),
                        StringToByteArray(c1_upper), false, true);
    SetRangeFilter(range_filters, 0, range_filter);
    // Request *request = MakeRequest(10, vector_querys, 1, nullptr, 0,
    //                                range_filters, 1, nullptr, 0, 1);
    Request *request = MakeRequest(10, vector_querys, 1, nullptr, 0, nullptr, 0,
                                   nullptr, 0, 1);

    Response *response = Search(engine, request);
    string msg = std::to_string(idx) + ", ";
    for (int i = 0; i < response->req_num; ++i) {
      SearchResult *results = GetSearchResult(response, i);
      if (results->result_num <= 0) {
        continue;
      }
      for (int j = 0; j < results->result_num; ++j) {
        ResultItem *result_item = GetResultItem(results, j);
        msg += string("score [") + std::to_string(result_item->score) + "], ";

        printDoc(result_item->doc, msg);
        msg += "\n";
      }
      if (abs(GetResultItem(results, 0)->score - 1.0) < 0.00001) {
        if (idx % 1000 == 0) {
          LOG(INFO) << msg;
        }
      } else {
        if (!bitmap::test(docids_bitmap_, idx)) {
          LOG(ERROR) << msg;
          bitmap::set(docids_bitmap_, idx);
          failed_count++;
        }
      }
    }

    DestroyRequest(request);
    DestroyResponse(response);
    double elap = utils::getmillisecs() - start;
    time += elap;
    if (idx % 1000 == 0) {
      LOG(INFO) << "search time [" << time / 1000 << "]";
      time = 0;
    }
    if (++idx >= doc_id) {
      idx = 0;
      break;
    }
  }
  return failed_count;
}

void Search(void *engine) {
  int idx = 1;
  double time = 0;
  double start = utils::getmillisecs();
  VectorQuery **vector_querys = MakeVectorQuerys(1);
  ByteArray *value = FloatToByteArray(feature + (uint64_t)idx * d, d);
  VectorQuery *vector_query =
      MakeVectorQuery(StringToByteArray(vector_name), value, 0, 10000, 0.1, 0);
  SetVectorQuery(vector_querys, 0, vector_query);

  string c1_lower = "4855";
  string c1_upper = "4855";
  string cid = "cid2";
  RangeFilter **range_filters = MakeRangeFilters(1);
  RangeFilter *range_filter =
      MakeRangeFilter(StringToByteArray(cid), StringToByteArray(c1_lower),
                      StringToByteArray(c1_upper), false, true);
  SetRangeFilter(range_filters, 0, range_filter);
  // Request *request = MakeRequest(10, vector_querys, 1, nullptr, 0,
  //                                range_filters, 1, nullptr, 0, 1);
  Request *request =
      MakeRequest(10, vector_querys, 1, nullptr, 0, nullptr, 0, nullptr, 0, 1);

  Response *response = Search(engine, request);
  string msg = std::to_string(idx) + ", ";
  for (int i = 0; i < response->req_num; ++i) {
    SearchResult *results = GetSearchResult(response, i);
    if (results->result_num <= 0) {
      continue;
    }
    for (int j = 0; j < results->result_num; ++j) {
      ResultItem *result_item = GetResultItem(results, j);
      msg += string("score [") + std::to_string(result_item->score) + "], ";

      printDoc(result_item->doc, msg);
      msg += "\n";
    }
    LOG(INFO) << msg;
  }

  DestroyRequest(request);
  DestroyResponse(response);
}

void UpdateThread(void *engine) {
  int doc_id = 1;
  Field **fields = MakeFields(fields_vec.size() + 1);

  for (size_t j = 0; j < fields_vec.size(); ++j) {
    enum DataType data_type = fields_type[j];
    ByteArray *name = StringToByteArray(fields_vec[j]);
    ByteArray *value;

    string &data = profiles[(uint64_t)doc_id * fields_vec.size() + j];
    if (fields_type[j] == INT) {
      value = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
      value->value = static_cast<char *>(malloc(sizeof(int)));
      value->len = sizeof(int);
      int v = atoi("88888");
      memcpy(value->value, &v, value->len);
    } else if (fields_type[j] == LONG) {
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

  ByteArray *value = FloatToByteArray(feature + (uint64_t)doc_id * d, d);
  ByteArray *name = StringToByteArray(vector_name);
  ByteArray *source = StringToByteArray(string("ccccccccccc"));
  Field *field = MakeField(name, value, source, VECTOR);
  SetField(fields, fields_vec.size(), field);

  Doc *doc = MakeDoc(fields, fields_vec.size() + 1);
  UpdateDoc(engine, doc);
  DestroyDoc(doc);
}

void *engine = nullptr;
long idx = 0;

TEST(Search, Init) {
  setvbuf(stdout, (char *)NULL, _IONBF, 0);
  int bitmap_bytes_size = 0;
  docids_bitmap_ = bitmap::create(max_doc_size, bitmap_bytes_size);
  ASSERT_NE(docids_bitmap_, nullptr);
  string path = "files";
  string log_dir = "log";
  Config *config = MakeConfig(StringToByteArray(path), max_doc_size);
  SetLogDictionary(StringToByteArray(log_dir));
  engine = Init(config);
  DestroyConfig(config);
  EXPECT_NE(engine, nullptr);
}

TEST(Search, CreateTable) {
  ByteArray *table_name = MakeByteArray("test", 4);
  FieldInfo **field_infos = MakeFieldInfos(fields_vec.size());

  for (size_t i = 0; i < fields_vec.size(); ++i) {
    FieldInfo *field_info =
        MakeFieldInfo(StringToByteArray(fields_vec[i]), fields_type[i], 1);
    SetFieldInfo(field_infos, i, field_info);
  }

  VectorInfo **vectors_info = MakeVectorInfos(1);
  string model_id = "model";
  string retrieval_type = "IVFPQ";
  string store_type = "MemoryOnly";
  VectorInfo *vector_info = MakeVectorInfo(
      StringToByteArray(vector_name), FLOAT, d, StringToByteArray(model_id),
      StringToByteArray(retrieval_type), StringToByteArray(store_type));
  SetVectorInfo(vectors_info, 0, vector_info);

  Table *table =
    MakeTable(table_name, field_infos, fields_vec.size(), vectors_info, 1, nprobe);
  enum ResponseCode ret = CreateTable(engine, table);
  DestroyTable(table);
  EXPECT_EQ(ret, 0);
}

TEST(Search, Add) {
  string profile_file = "sku_url_cid0_1.txt";
  string feature_file = "feat_same0_0.dat";

  sku_lines.reserve(100000);
  std::ifstream fin;
  fin.open(profile_file.c_str());
  std::string str;
  while (!fin.eof()) {
    std::getline(fin, str);
    if (str == "")
      break;
    sku_lines.push_back(str);
    auto profile = std::move(utils::split(str, "\t"));
    int i = 0;
    for (const auto &p : profile) {
      profiles[idx * fields_vec.size() + i] = p;
      ++i;
      if (i > fields_vec.size() - 1) {
        break;
      }
    }

    ++idx;
    if (idx >= max_doc_size) {
      break;
    }
  }
  LOG(INFO) << idx;
  fin.close();

  int fd = open(feature_file.c_str(), O_RDONLY, 0);
  feature = static_cast<float *>(
      mmap(NULL, idx * sizeof(float) * d, PROT_READ, MAP_SHARED, fd, 0));
  close(fd);

  int ret = AddDoc(engine, 10000 * 10);
  EXPECT_EQ(ret, 0);
}

TEST(Search, BuildIndex) {
  std::thread t(BuildIndex, engine);
  t.detach();

  while (GetIndexStatus(engine) != INDEXED) {
    std::this_thread::sleep_for(std::chrono::seconds(2));
  }

  // string docid =
  // "jfs/t17635/268/1735762492/316241/41ed9df9/5ad612cfN2a96dc78.jpg";
  // ByteArray *value = StringToByteArray(docid);
  // Doc *doc = GetDocByID(engine, value);
  // DelDoc(engine, value);
  // doc = GetDocByID(engine, value);

  // std::thread t_add1(AddDocThread, engine);
  // t_add1.join();

  // std::thread t_add(AddDocThread, engine);
  // t_add.detach();

  LOG(INFO) << "Indexed!";
}

TEST(Search, SearchThread) {
  // Search(engine);
  // UpdateThread(engine);

  // AddOneDoc(engine);
  // Search(engine);
  int search_thread_num = 1;
  std::thread t_searchs[search_thread_num];

  int search_num = 10000 * 5;
  std::function<int()> func_search =
      std::bind(SearchThread, engine, search_num);
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
  std::thread add_thread(add_func, engine);

  // get search results
  for (int i = 0; i < search_thread_num; ++i) {
    search_futures[i].wait();
    EXPECT_LE(search_futures[i].get(), 500);
    t_searchs[i].join();
  }

  add_thread.join();
}

TEST(Search, Dump) {
  int ret = Dump(engine);
  EXPECT_EQ(ret, 0);

  Close(engine);
  engine = nullptr;
  delete docids_bitmap_;
}

TEST(Search, Load) {
  setvbuf(stdout, (char *)NULL, _IONBF, 0);
  int bitmap_bytes_size = 0;
  docids_bitmap_ = bitmap::create(max_doc_size, bitmap_bytes_size);
  ASSERT_NE(docids_bitmap_, nullptr);
  string path = "files";
  string log_dir = "log";
  Config *config = MakeConfig(StringToByteArray(path), max_doc_size);
  engine = Init(config);
  DestroyConfig(config);
  ASSERT_NE(engine, nullptr);

  int ret = Load(engine);
  ASSERT_EQ(ret, 0);
}

TEST(Search, BuildIndexAfterLoad) {
  std::thread t(BuildIndex, engine);
  t.detach();

  while (GetIndexStatus(engine) != INDEXED) {
    std::this_thread::sleep_for(std::chrono::seconds(2));
  }

  LOG(INFO) << "Indexed finished after load!";
}

TEST(Search, SearchThreadAfterLoad) {
  int search_thread_num = 1;
  std::thread t_searchs[search_thread_num];

  int search_num = 10000 * 5;
  std::function<int()> func_search = std::bind(SearchThread, engine, search_num);
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
  std::thread add_thread(add_func, engine);

  // get search results
  for (int i = 0; i < search_thread_num; ++i) {
    search_futures[i].wait();
    EXPECT_LE(search_futures[i].get(), 500);
    t_searchs[i].join();
  }

  add_thread.join();
}

TEST(Search, DumpAfterLoad) {
  int ret = Dump(engine);
  EXPECT_EQ(ret, 0);
}

TEST(Search, Close) {
  Close(engine);
  engine = nullptr;
  delete docids_bitmap_;
  munmap(feature, idx * sizeof(float) * d);
}

int main(int argc, char **argv) {
    testing::InitGoogleTest(&argc, argv);
    return RUN_ALL_TESTS();
}

} // namespace Test
