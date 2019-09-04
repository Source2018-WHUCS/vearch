/**
 * Copyright (c) The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

//
// g++ -O3 -std=c++11 test_ni.cc  -I../util
//
#include <algorithm>
#include <iostream>
#include <memory>
#include <omp.h>
#include <string>
#include <sys/time.h>
#include <unistd.h>
#include <vector>

#include "numeric_index.h"
#include "random.h"

#pragma GCC diagnostic ignored "-Wwrite-strings"

using namespace std;
using tig_gamma::NI::FilterInfo;
using tig_gamma::NI::lexical_cast;
using utils::Timer;

#define NI_TEST

// TODO so much ugly code

namespace test_NI {

enum FType { UNKNOWN = 0, INT, LONG, FLOAT, DOUBLE, STRING };

class Profile {
public:
  Profile() : random_(0x12345678) {}

  void Init(const int nDocs) {
    if (nDocs <= 0) {
      Init();
      return;
    }

    doc_num_ = nDocs > 0 ? nDocs : 0;
    field_num_ = 0;

    // attr_idx_map_["cid3_field"] = field_num_++;
    // attr_type_map_["cid3_field"] = FType::INT;

    attr_idx_map_["price_field"] = field_num_++;
    attr_type_map_["price_field"] = FType::FLOAT;

    // attr_idx_map_["sale_field"] = field_num_++;
    // attr_type_map_["sale_field"] = FType::LONG;

    values_.resize(field_num_);
    for (unsigned i = 0; i < field_num_; i++) {
      values_[i] = vector<int>(doc_num_);
      for (unsigned j = 0; j < doc_num_; j++) {
        values_[i][j] =
            // 100 * i + j % 43 + static_cast<int>(random_.Uniform(34));
            100000 * i + j % 50000 + static_cast<int>(random_.Uniform(2019));
      }
    }
  }

  void Init() {
    doc_num_ = 0;
    field_num_ = 0;

    attr_idx_map_["cid3_field"] = field_num_++;
    attr_type_map_["cid3_field"] = FType::INT;

    ifstream ifs("./cid3s.txt");
    if (!ifs) {
      cerr << "can not open cid3s.txt\n";
      return;
    }

    values_.resize(field_num_);

    string line;
    while (std::getline(ifs, line)) {
      int cid3 = std::stoi(line);
      values_[0].push_back(cid3);
      doc_num_++;
    }
  }

  std::map<std::string, FType> &GetAttrType() { return attr_type_map_; }

  FType GetFieldType(const std::string &field) {
    auto it = attr_type_map_.find(field);
    return it == attr_type_map_.end() ? FType::UNKNOWN : it->second;
  }

  int GetAttrIdx(const std::string &field) const {
    const auto &iter = attr_idx_map_.find(field);
    return (iter != attr_idx_map_.end()) ? iter->second : -1;
  }

  unsigned long GetDocNum() { return doc_num_; }

  template <typename T>
  bool GetField(const int docid, const int field_id, T &value) const {
    if ((docid < 0 || docid >= doc_num_) or
        (field_id < 0 || field_id >= field_num_))
      return false;

    value = values_[field_id][docid];
    return true;
  }

private:
  std::map<std::string, int> attr_idx_map_;
  std::map<std::string, FType> attr_type_map_;
  unsigned long doc_num_;
  uint8_t field_num_;

  utils::Random random_;
  std::vector<vector<int>> values_;
};

typedef std::shared_ptr<Profile> ProfilePtr;
typedef std::shared_ptr<tig_gamma::NI::Indexes> NumIndexPtr;

class Index {
public:
  Index(const int nDocs) { Init(nDocs); }

  void Init(const int nDocs) {
    profile_ = std::make_shared<Profile>();
    profile_->Init(nDocs);

    numeric_indexes_ = std::make_shared<tig_gamma::NI::Indexes>();
  }

  template <typename T>
  void Check(const std::vector<int> &docs, int field_id,
             const char *lower_value, const char *upper_value) {
    T l, u;
    bool rv = lexical_cast(lower_value, l) && lexical_cast(upper_value, u);
    if (not rv) {
      std::cerr << "lexical_cast error\n";
      return;
    }

    for (auto doc : docs) {
      T value;
      if (profile_->GetField(doc, field_id, value)) {
        if (value < l || value > u) {
          std::cerr << "***** check error: doc (" << doc << ") " << value
                    << " not in [" << l << ", " << u << "]\n";
        }
      }
    }

    std::cout << "check1 done!\n";

    int nDocs = profile_->GetDocNum();
    vector<int> all_docs;
    for (int docid = 0; docid < nDocs; docid++) {
      T value;
      if (profile_->GetField(docid, field_id, value)) {
        if (value >= l && value <= u) {
          all_docs.push_back(docid);
        }
      }
    }

    all_docs.shrink_to_fit();

    std::cerr << "all_docs=" << all_docs.size() << ", docs=" << docs.size()
              << "\n";

    if (docs != all_docs) {
      std::cerr << "***** check error: nDocs=" << nDocs
                << ", docs=" << docs.size() << "\n";
    }
    std::cout << "check2 done!\n";
  }

  void RunCheck(const std::vector<int> &docs, const char *field,
                const char *lower_value, const char *upper_value) {
    std::cout << "run_check ...\n";
    int field_id = profile_->GetAttrIdx(field);
    if (field_id < 0) {
      return;
    }

    FType type = profile_->GetFieldType(field);
    switch (type) {
    case FType::INT:
      Check<int>(docs, field_id, lower_value, upper_value);
      break;
    case FType::LONG:
      Check<long>(docs, field_id, lower_value, upper_value);
      break;
    case FType::FLOAT:
      Check<float>(docs, field_id, lower_value, upper_value);
      break;
    case FType::DOUBLE:
      Check<double>(docs, field_id, lower_value, upper_value);
      break;
    default:
      std::cout << "UNKNOWN field type\n";
      break;
    }

    std::cout << "run_check done!\n";
  }

  vector<int> TestFilter(const char *field, const char *lower_value,
                         const char *upper_value, int flags = 0) {
    FilterInfo filter;

    filter.field = (char *)field;
    filter.lower_value = (char *)lower_value;
    filter.upper_value = (char *)upper_value;

    vector<int> docs;
    int retval = Work(std::vector<FilterInfo>{filter}, docs, flags);
    if (retval > 0) {
      RunCheck(docs, field, lower_value, upper_value);
    }
    return docs;
  }

  vector<int> TestFilters(const std::vector<FilterInfo> &filters,
                          int flags = 0) {
    vector<int> docs;
    int retval = Work(filters, docs, flags);
    if (retval > 0) {
      RunCheck(docs, "price_field", "9310", "10007");
    }
    return docs;
  }

  int Work(const vector<FilterInfo> &filters, vector<int> &fDocs,
           int flags = 0) {
    fDocs.clear();
    if (filters.empty()) {
      return -1;
    }

    int retval = -1;
    tig_gamma::NI::RangeQueryResult query_result;

    // 0x1 -> search block-skiplist index
    // 0x2 -> search rt->skiplist index
    // default to search both
    if (flags) {
      cout << ">>> flags: " << flags << "\n";
      query_result.SetFlags(flags);
    }

    Timer t;
    t.Start("Work");
    retval = GetNumIndex()->Search(filters, query_result);
    t.Stop("Work");
    t.Output();

    if (retval > 0) {
      fDocs = query_result.ToDocs();
      cout << "retval -> " << retval << ", result count -> " << fDocs.size()
           << "\n";
    }

    std::cout << "$$$ retval -> " << retval << "\n";
    return fDocs.size();
  }

  int IndexingFields() {
    int retvals = 0;
    auto &attr_type = profile_->GetAttrType();
    for (const auto &it : attr_type) {
      int retval = 0;
      switch (it.second) {
      case FType::INT:
        retval = _IndexingField<int>(it.first);
        break;
      case FType::LONG:
        retval = _IndexingField<long>(it.first);
        break;
      case FType::FLOAT:
        retval = _IndexingField<float>(it.first);
        break;
      case FType::DOUBLE:
        retval = _IndexingField<double>(it.first);
        break;
      default:
        break;
      }
      retvals += retval;
    }
    return retvals;
  }

  NumIndexPtr GetNumIndex() { return numeric_indexes_; }

  void Output() { numeric_indexes_->Output(); }

private:
  ProfilePtr profile_;
  NumIndexPtr numeric_indexes_;

  template <typename T> int _IndexingField(const std::string &field) {
    assert(numeric_indexes_ != nullptr);

    int field_id = profile_->GetAttrIdx(field);
    if (field_id < 0) {
      return -1;
    }

    std::function<T(const int)> cb = [&, field_id](const int docid) -> T {
      T value(0);
      if (!profile_->GetField<T>(docid, field_id, value)) {
        std::cerr << "GetField(" << docid << ", " << field_id << ") error.\n";
      }
      return value;
    };

    Timer t;
    t.Start("Build");
    int retval =
        numeric_indexes_->Indexing<T>(field, profile_->GetDocNum(), cb);
    if (retval < 0) {
      return -1;
    }
    t.Stop();

    t.Start("Add");
    int doc_num = profile_->GetDocNum();
    for (int docid = 0; docid < doc_num; docid++) {
      T value;
      if (profile_->GetField<T>(docid, field_id, value)) {
        numeric_indexes_->Add(docid, field, string((char *)&value, sizeof(T)));
      }
    }
    t.Stop();

    t.Output();
    return 0;
  }
};

} // namespace test_NI

#ifdef NI_TEST

void Output(const vector<int> &fDocs, const char *tag) {
  if (fDocs.empty()) {
    cout << "=> no result\n";
    return;
  }

  bool x = true;
  int cnt = (int)fDocs.size();

  cout << tag << " => " << fDocs[0];
  for (int i = 1; i < cnt; i++) {
    if (i < 50 || i > cnt - 50) {
      cout << "," << fDocs[i];
    } else {
      if (x) {
        cout << " ... ";
        x = false;
      }
    }
  }
  cout << "\n";
}

int main(int argc, char *argv[]) {
  using namespace test_NI;

  int nDocs = 10000000;
  if (argc > 1) {
    nDocs = std::stoi(argv[1]);
  }

  string lower_value("20"), upper_value("15192");

  if (argc > 2) {
    lower_value = argv[2];
    upper_value = lower_value;
  }

  if (argc > 3) {
    upper_value = argv[3];
  }

  cout << "nDocs=" << nDocs << ", lower_value=" << lower_value
       << ", upper_value=" << upper_value << "\n";

  Index index(nDocs);

  int ret = index.IndexingFields();
  if (ret < 0) {
    cerr << "indexing fields error\n";
    return -1;
  }

  index.Output();

  index.TestFilter("cid3_field", "12139", "12139", 0x1);
  std::cerr
      << "-----------------------------------------------------------------\n";
  index.TestFilter("cid3_field", "12139", "12139", 0x2);

  std::cerr
      << "-----------------------------------------------------------------\n";

  index.TestFilter("not_exist_filed", "0", "0", 0x1);
  index.TestFilter("price_field", lower_value.c_str(), upper_value.c_str(),
                   0x1);
  std::cerr
      << "-----------------------------------------------------------------\n";

  index.TestFilter("price_field", lower_value.c_str(), upper_value.c_str(),
                   0x2);

  std::cerr
      << "-----------------------------------------------------------------\n";

  std::vector<FilterInfo> filters;
  FilterInfo filter;

  filter.field = "price_field";
  filter.lower_value = "14";
  filter.upper_value = "10007";
  filters.emplace_back(filter);

  filter.field = "price_field";
  filter.lower_value = "596";
  filter.upper_value = "23100";
  filters.emplace_back(filter);

  filter.field = "price_field";
  filter.lower_value = "9310";
  filter.upper_value = "15290";
  filters.emplace_back(filter);

  std::vector<int> docs_1 =
      index.TestFilter("price_field", "9310", "10007", 0x2);
  std::vector<int> docs_2 = index.TestFilters(filters, 0x2);

  if (docs_1 != docs_2) {
    std::cerr << "***************** multiple fields check error*************\n";
    Output(docs_1, "docs_1");
    Output(docs_2, "docs_2");
  }
  return 0;
}

#endif
