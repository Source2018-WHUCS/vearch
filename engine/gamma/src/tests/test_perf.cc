/**
 * Copyright (c) The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

#include "test.h"
#include "util/timer.h"
#include "util/utils.h"
#include <stdio.h>
#include <string>
#include <vector>

using std::map;
using std::string;
using std::vector;

constexpr int kDefaultMaxDocSize = 1000000;

namespace tig_gamma {

struct TableInfo {
  string table_name;
  map<string, DataType> field_mappings;

  struct MetaData {
    string vector_name;
    int dimension;
    string model_id;
    string retrieval_type;
    string store_type;
  } meta;
};

class PerfTest {
public:
  explicit PerfTest(int max_doc_size)
      : max_doc_size_(max_doc_size), engine_(nullptr), exit_(false) {}

  ~PerfTest() {
    exit_ = true;
    if (add_task_.joinable()) {
      add_task_.join();
    }

    Dump(engine_);
    Close(engine_);
  }

public:
  void Init(const string &path, const string &log_dir) {
    engine_ = _InitEngine(path, log_dir, max_doc_size_);
  }

  void CreateTable(TableInfo t) {
    table_info_ = t;
    _CreateTable(engine_, t);
  }

  void BuildIndex() {
    std::thread t(::BuildIndex, engine_);
    t.detach();

    while (GetIndexStatus(engine_) != INDEXED) {
      std::this_thread::sleep_for(std::chrono::seconds(2));
    }
    LOG(INFO) << ">>> Build Index Done.";
  }

  void StartAddThread() {
    add_task_ = std::thread([this]() {
      long fsize = utils::get_file_size("./data/vectors.dat");

      FILE *fp = fopen("./data/vectors.dat", fsize > 0 ? "rb" : "wb");
      if (not fp) {
        LOG(ERROR) << "open file error!";
        return;
      }

      size_t d = table_info_.meta.dimension;

      int times = max_doc_size_;
      while (not exit_ && times--) {
        int branch = 7;
        int product_code = 0;
        int type = 1;

        switch (times % 4) {
        case 0:
          product_code = 100;
          break;
        case 1:
          product_code = 101;
          break;
        case 2:
          product_code = 102;
          break;
        case 3:
          product_code = 102;
          type = 0;
          break;
        }

        vector<float> xb(d * 1);
        srand48(times);
        std::generate(xb.begin(), xb.end(), drand48);

        if (fsize > 0) {
          if (fread(xb.data(), sizeof(float), d, fp) != d) {
            LOG(ERROR) << "read file error!";
          }
        } else {
          if (fwrite(xb.data(), sizeof(float), d, fp) != d) {
            LOG(ERROR) << "write file error!";
          }
        }

        _AddDoc(engine_, table_info_,
                {
                    {"branch", branch},
                    {"product_code", product_code},
                    {"type", type},
                },
                xb);
      }

      fclose(fp);
      LOG(INFO) << ">>> Add finished.";
    });
    std::this_thread::sleep_for(std::chrono::seconds(10));
  }

  void StartAddThread2() {
    add_task_ = std::thread([this]() {
      long fsize = utils::get_file_size("./data/querys.dat");

      FILE *fp = fopen("./data/vectors.dat", fsize > 0 ? "rb" : "wb");
      if (not fp) {
        LOG(ERROR) << "open file error!";
        return;
      }

      size_t d = table_info_.meta.dimension;

      vector<float> xb(d * 1);
      srand48(1024);
      std::generate(xb.begin(), xb.end(), drand48);

      if (fsize > 0) {
        if (fread(xb.data(), sizeof(float), d, fp) != d) {
          LOG(ERROR) << "read file error!";
        }
      } else {
        if (fwrite(xb.data(), sizeof(float), d, fp) != d) {
          LOG(ERROR) << "write file error!";
        }
      }

      _AddDoc(engine_, table_info_,
              {
                  {"branch", 7},
                  {"product_code", 101},
                  {"type", 1},
              },
              xb);

      _AddDoc(engine_, table_info_,
              {
                  {"branch", 7},
                  {"product_code", 201},
                  {"type", 1},
              },
              xb);

      _AddDoc(engine_, table_info_,
              {
                  {"branch", 7},
                  {"product_code", 101},
                  {"type", 2},
              },
              xb);

      fclose(fp);
      LOG(INFO) << ">>> Add finished.";
    });
    std::this_thread::sleep_for(std::chrono::seconds(10));
  }

  double Search(const int count, int direct_search_type = 0) {
    assert(count > 0);
    long fsize = utils::get_file_size("./data/querys.dat");

    FILE *fp = fopen("./data/querys.dat", fsize > 0 ? "rb" : "wb");
    if (not fp) {
      LOG(ERROR) << "open file error!";
      return -1;
    }

    size_t d = table_info_.meta.dimension;
    double total_cost_ms = 0;
    bool the_1st = true;

    LOG(INFO) << ">>> Search ...";
    int times = count;
    while (times--) {
      vector<float> xb(d * 1);
      srand48(times);
      std::generate(xb.begin(), xb.end(), drand48);

      if (fsize > 0) {
        if (fread(xb.data(), sizeof(float), d, fp) != d) {
          LOG(ERROR) << "read file error!";
        }
      } else {
        if (fwrite(xb.data(), sizeof(float), d, fp) != d) {
          LOG(ERROR) << "write file error!";
        }
      }

      auto time_cost_ms = _Search(engine_, table_info_, xb, direct_search_type);
      LOG(INFO) << "time cost " << time_cost_ms << " ms.";

      if (the_1st) {
        the_1st = false; // discard the 1st one
      } else {
        total_cost_ms += time_cost_ms;
      }
    }

    fclose(fp);
    return (total_cost_ms / count);
  }

  void GetDoc(int doc_id) {
    ByteArray *key = StringToByteArray(std::to_string(doc_id));
    Doc *doc = GetDocByID(engine_, key);
    string msg;
    printDoc(doc, msg);
    LOG(INFO) << msg;
  }

private:
  static void *_InitEngine(const string &path, const string &log_dir,
                           int max_doc_size) {
    Config *config = MakeConfig(StringToByteArray(path), max_doc_size);
    SetLogDictionary(StringToByteArray(log_dir));
    void *engine = ::Init(config);
    DestroyConfig(config);
    return engine;
  }

  static void _CreateTable(void *engine, TableInfo &t) {
    int d = t.meta.dimension;
    ByteArray *table_name = StringToByteArray(t.table_name);

    FieldInfo **field_infos = MakeFieldInfos(t.field_mappings.size());
    int i = 0;
    for (auto _ : t.field_mappings) {
      ByteArray *name = StringToByteArray(_.first);
      DataType type = _.second;

      FieldInfo *field_info = MakeFieldInfo(name, type, 1);
      SetFieldInfo(field_infos, i++, field_info);
    }

    VectorInfo **vector_infos = MakeVectorInfos(1);
    VectorInfo *vector_info =
        MakeVectorInfo(StringToByteArray(t.meta.vector_name),
                       FLOAT, // data_type
                       d,     // dimension
                       StringToByteArray(t.meta.model_id),
                       StringToByteArray(t.meta.retrieval_type),
                       StringToByteArray(t.meta.store_type));
    SetVectorInfo(vector_infos, 0, vector_info);

    Table *table = MakeTable(table_name,
                             field_infos,             // fields
                             t.field_mappings.size(), // fields_num
                             vector_infos,            // vectors_info
                             1, kIVFPQParam);         // vectors_num
    ResponseCode code = ::CreateTable(engine, table);
    DestroyTable(table);

    if (code != 0) {
      printf("create table ret [%d]\n", code);
    }
  }

  static void _AddDoc(void *engine, TableInfo &t, map<string, int> vm,
                      const vector<float> &xb) {
    static int doc_id = 0;
    int field_num = t.field_mappings.size() + 1; // the last one is VECTOR

    size_t d = t.meta.dimension;
    Field **fields = MakeFields(field_num);
    int i = 0;
    for (auto _ : t.field_mappings) {
      ByteArray *name = StringToByteArray(_.first);
      DataType type = _.second;

      ByteArray *value = nullptr;
      if (_.first == "_id") {
        value = StringToByteArray("jsf/img/" + std::to_string(doc_id));
        doc_id++; // increment
      } else {
        if (type == INT) {
          value = ToByteArray<int>(vm[_.first]);
        } else {
          LOG(ERROR) << "not support.";
        }
      }

      Field *field = MakeField(name, value, NULL, type);
      SetField(fields, i++, field);
    }

    assert(xb.size() == d);
    ByteArray *value = FloatToByteArray(xb.data(), d);

    Field *field =
        MakeField(StringToByteArray(t.meta.vector_name), value, NULL, VECTOR);
    SetField(fields, i++, field);

    Doc *doc = MakeDoc(fields, field_num);
    ResponseCode code = AddDoc(engine, doc);
    DestroyDoc(doc);

    if (code != 0) {
      printf("add doc ret [%d]\n", code);
    }
  }

  static double _Search(void *engine, TableInfo &t, const vector<float> &xb,
                        int direct_search_type) {
    size_t d = t.meta.dimension;
    VectorQuery **querys = MakeVectorQuerys(1);

    assert(xb.size() == d);

    VectorQuery *query =
        MakeVectorQuery(StringToByteArray(t.meta.vector_name), // name
                        FloatToByteArray(xb.data(), d),        // value
                        0,                                     // min_score
                        10000,                                 // max_score
                        1, 0);                                 // boost
    SetVectorQuery(querys, 0, query);

    RangeFilter **filters = MakeRangeFilters(3);

    auto add_filter = [filters](int index, const string &field, int l,
                                int u = -1) -> void {
      u = (u < 0) ? l : u;
      RangeFilter *filter =
          MakeRangeFilter(StringToByteArray(field),             // field
                          StringToByteArray(std::to_string(l)), // lower_value
                          StringToByteArray(std::to_string(u)), // upper_value
                          1,                                    // include_lower
                          1);                                   // include_upper
      SetRangeFilter(filters, index, filter);
    };

    add_filter(0, "branch", 7);
    add_filter(1, "product_code", 101);
    add_filter(2, "type", 1);

    Request *request = MakeRequest(100,                // topk
                                   querys,             // vector querys
                                   1,                  // vector querys num
                                   nullptr,            // fields
                                   0,                  // fields_num
                                   filters,            // range_filters
                                   3,                  // range_filters_num
                                   nullptr,            // term_filters
                                   0,                  // term_filters_num
                                   1,                  // req_num
                                   direct_search_type, // direct_search_type
                                   StringToByteArray("debug"));

    auto t0 = utils::getmillisecs();
    Response *response = ::Search(engine, request);
    auto t1 = utils::getmillisecs();

    for (int i = 0; i < response->req_num; i++) {
      SearchResult *result = GetSearchResult(response, i);
      printf("request [%d] total call %d\n", i, result->total);

      for (int j = 0; j < result->result_num; j++) {
        ResultItem *result_item = GetResultItem(result, j);
        string msg =
            string("score [") + std::to_string(result_item->score) + "], ";
        printDoc(result_item->doc, msg);
        printf("%s\n", msg.c_str());
      }

      printf("response %s\n",
             string(result->msg->value, result->msg->len).c_str());
    }

    if (response->online_log_message) {
      printf("online debug message: %s\n",
             string(response->online_log_message->value,
                    response->online_log_message->len)
                 .c_str());
    }

    DestroyResponse(response);
    return (t1 - t0);
  }

private:
  int max_doc_size_;

  TableInfo table_info_;
  void *engine_;

  std::thread add_task_;
  volatile bool exit_;
};

} // namespace tig_gamma

using tig_gamma::PerfTest;

void test_perf(int max_doc_size, int direct_search_type) {
  tig_gamma::TableInfo t;
  t.table_name = "pac";
  t.field_mappings = {
      {"_id", STRING},
      {"branch", INT},
      {"product_code", INT},
      {"type", INT},
  };

  t.meta.vector_name = "image";
  t.meta.dimension = 512;
  t.meta.retrieval_type = "IVFPQ";
  t.meta.store_type = "MemoryOnly";
  t.meta.model_id = "VGG";

  PerfTest pt(max_doc_size);

  pt.Init("table", "logs");
  pt.CreateTable(t);
  LOG(INFO) << "Init & CreateTable done!";

  pt.StartAddThread();
  pt.BuildIndex();

  tig_gamma::NI::Timer t0;
  t0.Start("Search");
  auto avg_cost_ms = pt.Search(1000, direct_search_type);
  t0.Stop();
  t0.Output();

  LOG(INFO) << "finish test_perf, AVG cost -> " << avg_cost_ms << " ms.";
}

void test_bugfix() {
  tig_gamma::TableInfo t;
  t.table_name = "bugfix";
  t.field_mappings = {
      {"_id", STRING},
      {"branch", INT},
      {"product_code", INT},
      {"type", INT},
  };

  t.meta.vector_name = "image2";
  t.meta.dimension = 512;
  t.meta.retrieval_type = "IVFPQ";
  t.meta.store_type = "MemoryOnly";
  t.meta.model_id = "VGG";

  PerfTest pt(1000);

  pt.Init("table2", "logs");
  pt.CreateTable(t);
  LOG(INFO) << "Init & CreateTable done!";

  pt.StartAddThread2();
  // pt.BuildIndex();

  tig_gamma::NI::Timer t0;
  t0.Start("Search");
  auto avg_cost_ms = pt.Search(1, 0);
  t0.Stop();
  t0.Output();

  LOG(INFO) << "finish test_perf, AVG cost -> " << avg_cost_ms << " ms.";
}

int main(int argc, char *argv[]) {
  int tc = 0;
  int max_doc_size = kDefaultMaxDocSize;
  int direct_search_type = 0;

  if (argc > 1) {
    tc = std::stoi(argv[1]);
  }
  if (argc > 2) {
    max_doc_size = std::stoi(argv[2]);
  }
  if (argc > 3) {
    direct_search_type = std::stoi(argv[3]);
  }

  fprintf(stderr, "set max_doc_size to %d\n", max_doc_size);
  fprintf(stderr, "set direct_search_type to %d\n", direct_search_type);

  switch (tc) {
  case 0:
    test_perf(max_doc_size, direct_search_type);
    break;
  case 1:
    test_bugfix();
    break;
  default:
    break;
  }

  return 0;
}
