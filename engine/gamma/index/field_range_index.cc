/**
 * Copyright 2019 The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

#include "field_range_index.h"
#include <string.h>
#include <algorithm>
#include <cassert>
#include <cstddef>
#include <cstdint>
#include <ctime>
#include <functional>
#include <iostream>
#include <iterator>
#include <limits>
#include <numeric>
#include <sstream>
#include <typeinfo>
#include "log.h"
#include "threadskv10h.h"
#include "utils.h"

using std::string;

namespace tig_gamma {

class Node {
 public:
  Node() {
    size_ = 0;
    capacity_ = 0;
    data_ = nullptr;
    min_ = std::numeric_limits<int>::max();
    max_ = -1;
  }

  ~Node() { free(data_); }

  int Add(int val) {
    min_ = std::min(min_, val);
    max_ = std::max(max_, val);

    if (capacity_ == 0) {
      capacity_ = 512;
      data_ = (int *)malloc(capacity_ * sizeof(int));
    } else if (size_ >= capacity_) {
      capacity_ *= 2;
      int *data = (int *)malloc(capacity_ * sizeof(int));
      memcpy(data, data_, size_ * sizeof(int));
      int *old_data = data_;
      data_ = data;
      std::this_thread::sleep_for(std::chrono::seconds(1));
      free(old_data);
    }

    data_[size_] = val;
    ++size_;
    return 0;
  }

  int Min() { return min_; }
  int Max() { return max_; }

  int Size() { return size_; }

  int *Data() { return data_; }

 private:
  int min_;
  int max_;

  int size_;
  int capacity_;
  int *data_;
};

typedef struct {
  uint mainleafxtra;
  uint maxleaves;
  uint poolsize;
  uint leafxtra;
  uint mainpool;
  uint mainbits;
  uint bits;
  const char *kDelim;
} BTreeParameters;

class FieldRangeIndex {
 public:
  FieldRangeIndex(std::string &path, int idx, enum DataType field_type,
                  BTreeParameters &bt_param);
  ~FieldRangeIndex();

  int Add(unsigned char *key, uint key_len, int value);

  int Search(const string &low, const string &high, RangeQueryResult &result);

  int Search(const string &tags, RangeQueryResult &result);

 private:
  BtMgr *main_mgr_;
  BtMgr *cache_mgr_;
  bool is_numeric_;
  char *kDelim_;
  std::string path_;
};

FieldRangeIndex::FieldRangeIndex(std::string &path, int idx,
                                 enum DataType field_type,
                                 BTreeParameters &bt_param)
    : path_(path) {
  string cache_file = path + string("/cache_") + std::to_string(idx) + ".dis";
  string main_file = path + string("/main_") + std::to_string(idx) + ".dis";

  remove(cache_file.c_str());
  remove(main_file.c_str());

  cache_mgr_ = bt_mgr(const_cast<char *>(cache_file.c_str()), bt_param.bits,
                      bt_param.leafxtra, bt_param.poolsize);
  cache_mgr_->maxleaves = bt_param.maxleaves;
  main_mgr_ = bt_mgr(const_cast<char *>(main_file.c_str()), bt_param.mainbits,
                     bt_param.mainleafxtra, bt_param.mainpool);
  main_mgr_->maxleaves = bt_param.maxleaves;

  if (field_type == DataType::STRING) {
    is_numeric_ = false;
  } else {
    is_numeric_ = true;
  }
  kDelim_ = const_cast<char *>(bt_param.kDelim);
}

FieldRangeIndex::~FieldRangeIndex() {
  BtDb *bt = bt_open(cache_mgr_, main_mgr_);

  if (bt_startkey(bt, nullptr, 0) == 0) {
    while (bt_nextkey(bt)) {
      if (bt->phase == 1) {
        Node *p_node = nullptr;
        memcpy(&p_node, bt->mainval->value, sizeof(Node *));
        delete p_node;
      }
    }
  }

  bt_unlockpage(BtLockRead, bt->cacheset->latch, __LINE__);
  bt_unpinlatch(bt->cacheset->latch);

  bt_unlockpage(BtLockRead, bt->mainset->latch, __LINE__);
  bt_unpinlatch(bt->mainset->latch);
  bt_close(bt);

  if (cache_mgr_) {
    bt_mgrclose(cache_mgr_);
    cache_mgr_ = nullptr;
  }
  if (main_mgr_) {
    bt_mgrclose(main_mgr_);
    main_mgr_ = nullptr;
  }
}

static int ReverseEndian(const unsigned char *in, unsigned char *out,
                         uint len) {
  for (uint i = 0; i < len; ++i) {
    out[i] = in[len - i - 1];
  }

  unsigned char N = 0x80;
  out[0] += N;
  return 0;
}

int FieldRangeIndex::Add(unsigned char *key, uint key_len, int value) {
  BtDb *bt = bt_open(cache_mgr_, main_mgr_);
  unsigned char key2[key_len];

  std::function<void(unsigned char *, uint)> InsertToBt =
      [&](unsigned char *key_to_add, uint key_len) {
        Node *p_node = nullptr;
        int ret = bt_findkey(bt, key_to_add, key_len, (unsigned char *)&p_node,
                             sizeof(Node *));

        if (ret < 0) {
          p_node = new Node;
          BTERR bterr = bt_insertkey(bt->main, key_to_add, key_len, 0,
                                     static_cast<void *>(&p_node),
                                     sizeof(Node *), Unique);
          if (bterr) {
            LOG(ERROR) << "Error " << bt->mgr->err;
          }
        }
        p_node->Add(value);
      };

  if (is_numeric_) {
    ReverseEndian(key, key2, key_len);
    InsertToBt(key2, key_len);
  } else {
    char key_s[key_len + 1];
    memcpy(key_s, key, key_len);
    key_s[key_len] = 0;

    char *p, *k;
    k = strtok_r(key_s, kDelim_, &p);
    while (k != nullptr) {
      InsertToBt(reinterpret_cast<unsigned char *>(k), strlen(k));
      k = strtok_r(NULL, kDelim_, &p);
    }
  }

  bt_close(bt);

  return 0;
}

int FieldRangeIndex::Search(const string &lower, const string &upper,
                            RangeQueryResult &result) {
  if (!is_numeric_) {
    return Search(lower, result);
  }

  BtDb *bt = bt_open(cache_mgr_, main_mgr_);
  unsigned char key_l[lower.length()];
  unsigned char key_u[upper.length()];
  ReverseEndian(reinterpret_cast<const unsigned char *>(lower.data()), key_l,
                lower.length());
  ReverseEndian(reinterpret_cast<const unsigned char *>(upper.data()), key_u,
                upper.length());

  std::vector<Node *> lists;
  lists.reserve(1000000);

  int min_doc = std::numeric_limits<int>::max();
  int max_doc = 0;

  if (bt_startkey(bt, key_l, lower.length()) == 0) {
    while (bt_nextkey(bt)) {
      if (bt->phase == 1) {
        if (keycmp(bt->mainkey, key_u, upper.length()) > 0) {
          break;
        }
        Node *p_node = nullptr;
        memcpy(&p_node, bt->mainval->value, sizeof(Node *));
        lists.push_back(p_node);

        min_doc = std::min(min_doc, p_node->Min());
        max_doc = std::max(max_doc, p_node->Max());
      }
    }
  }

  bt_unlockpage(BtLockRead, bt->cacheset->latch, __LINE__);
  bt_unpinlatch(bt->cacheset->latch);

  bt_unlockpage(BtLockRead, bt->mainset->latch, __LINE__);
  bt_unpinlatch(bt->mainset->latch);
  bt_close(bt);

  if (max_doc - min_doc + 1 <= 0) {
    return 0;
  }

  result.SetRange(min_doc, max_doc);
  result.Resize();

  int list_size = lists.size();
  for (int i = 0; i < list_size; ++i) {
    Node *list = lists[i];
    int *data = list->Data();
    int size = list->Size();
    // for (int j = 0; j < size; ++j) {
    //   if (data[j] > max_doc) {
    //     continue;
    //   }
    //   result.Set(data[j] - min_doc);
    // }

#define HANDLE_ONE                 \
  do {                             \
    if (data[j] > max_doc) {       \
      continue;                    \
    }                              \
    result.Set(data[j] - min_doc); \
                                   \
    ++j;                           \
  } while (0)

    size_t j = 0;
    size_t loops = size / 4;
    for (size_t i = 0; i < loops; ++i) {
      HANDLE_ONE;  // 1
      HANDLE_ONE;  // 2
      HANDLE_ONE;  // 3
      HANDLE_ONE;  // 4
    }

    switch (size % 4) {
      case 3:
        HANDLE_ONE;
      case 2:
        HANDLE_ONE;
      case 1:
        HANDLE_ONE;
    }
#undef HANDLE_ONE
  }

  return max_doc - min_doc + 1;
}

int FieldRangeIndex::Search(const string &tags, RangeQueryResult &result) {
  std::vector<string> items = utils::split(tags, kDelim_);

  RangeQueryResult results_union[items.size()];

  for (size_t i = 0; i < items.size(); ++i) {
    string item = items[i];
    const unsigned char *key_tag =
        reinterpret_cast<const unsigned char *>(item.data());

    int min_doc = std::numeric_limits<int>::max();
    int max_doc = 0;

    Node *p_node = nullptr;
    BtDb *bt = bt_open(cache_mgr_, main_mgr_);
    int ret =
        bt_findkey(bt, const_cast<unsigned char *>(key_tag), item.length(),
                   (unsigned char *)&p_node, sizeof(Node *));
    bt_close(bt);

    if (ret < 0) {
      return 0;
    }
    min_doc = std::min(min_doc, p_node->Min());
    max_doc = std::max(max_doc, p_node->Max());

    if (max_doc - min_doc + 1 <= 0) {
      return 0;
    }

    results_union[i].SetRange(min_doc, max_doc);
    results_union[i].Resize();

    int size = p_node->Size();
    int *data = p_node->Data();
    for (int j = 0; j < size; ++j) {
      if (data[j] > max_doc) {
        continue;
      }
      results_union[i].Set(data[j] - min_doc);
    }
  }

  int min_doc = std::numeric_limits<int>::max();
  int max_doc = 0;
  for (size_t i = 0; i < items.size(); ++i) {
    min_doc = std::min(min_doc, results_union[i].Min());
    max_doc = std::max(max_doc, results_union[i].Max());
  }

  int retval = max_doc - min_doc + 1;
  if (retval <= 0) {
    return 0;
  }
  result.SetRange(min_doc, max_doc);
  result.Resize();
  for (size_t i = 0; i < items.size(); ++i) {
    int doc = results_union[i].Next();
    while (doc >= 0) {
      result.Set(doc - min_doc);
      doc = results_union[i].Next();
    }
  }

  return retval;
}

MultiFieldsRangeIndex::MultiFieldsRangeIndex(std::string &path,
                                             Profile *profile)
    : path_(path) {
  profile_ = profile;
  fields_.resize(profile->FieldsNum());
  std::fill(fields_.begin(), fields_.end(), nullptr);
}

MultiFieldsRangeIndex::~MultiFieldsRangeIndex() {
  for (size_t i = 0; i < fields_.size(); i++) {
    if (fields_[i]) {
      delete fields_[i];
      fields_[i] = nullptr;
    }
  }
}

int MultiFieldsRangeIndex::Add(int docid, int field) {
  FieldRangeIndex *index = fields_[field];
  if (index == nullptr) {
    return 0;
  }

  unsigned char *key;
  int key_len = 0;
  profile_->GetFieldRawValue(docid, field, &key, key_len);
  index->Add(key, key_len, docid);

  return 0;
}

int MultiFieldsRangeIndex::Search(const std::vector<FilterInfo> &filters,
                                  MultiRangeQueryResults &out) {
  out.Clear();
  int fsize = filters.size();

  if (1 == fsize) {
    auto &_ = filters[0];
    if (_.is_union) {
      out.SetFlags(out.Flags() | 0x4);
    }
    RangeQueryResult tmp(out.Flags());
    FieldRangeIndex *index = fields_[_.field];
    if (index == nullptr || _.field < 0) {
      return -1;
    }

    int retval = index->Search(_.lower_value, _.upper_value, tmp);
    if (retval > 0) {
      out.Add(tmp);
    }
    return retval;
  }

  RangeQueryResult results[fsize];
  int valuable_result = -1;
  // record the shortest docid list
  int shortest_idx = -1, shortest = std::numeric_limits<int>::max();

  for (int i = 0; i < fsize; ++i) {
    auto &filter = filters[i];

    FieldRangeIndex *index = fields_[filter.field];
    if (index == nullptr || filter.field < 0) {
      continue;
    }

    int flags = out.Flags();
    if (filter.is_union) {
      flags |= 0x4;
    }

    results[valuable_result + 1].SetFlags(flags);

    int retval = index->Search(filter.lower_value, filter.upper_value,
                               results[valuable_result + 1]);
    if (retval < 0) {
      ;
    } else if (retval == 0) {
      return 0;  // no intersection
    } else {
      valuable_result += 1;

      if (shortest > retval) {
        shortest = retval;
        shortest_idx = valuable_result;
      }
    }
  }

  if (valuable_result < 0) {
    return -1;  // universal set
  }

  // t.Stop();
  // t.Output();

  // When the shortest doc chain is long,
  // instead of calculating the intersection immediately, a lazy
  // mechanism is made.
  if (shortest > kLazyThreshold_) {
    for (int i = 0; i <= valuable_result; ++i) {
      out.Add(results[i]);
    }
    return 1;  // it's hard to count the return docs
  }

  RangeQueryResult tmp(out.Flags());
  int count = Intersect(results, valuable_result, shortest_idx, tmp);
  if (count > 0) {
    out.Add(tmp);
  }

  return count;
}

int MultiFieldsRangeIndex::Intersect(const RangeQueryResult *results, int j,
                                     int k, RangeQueryResult &out) const {
  assert(results != nullptr && j >= 0);

  // t.Start("Intersect");
  // I want to build a smaller bitmap ...
  int min_doc = results[0].Min();
  int max_doc = results[0].Max();

  for (int i = 1; i <= j; i++) {
    auto &r = results[i];

    // the maximum of the minimum(s)
    if (r.Min() > min_doc) {
      min_doc = r.Min();
    }
    // the minimum of the maximum(s)
    if (r.Max() < max_doc) {
      max_doc = r.Max();
    }
  }

  if (max_doc - min_doc + 1 <= 0) {
    return 0;
  }
  out.SetRange(min_doc, max_doc);
  out.Resize();

  // calculate the intersection with the shortest doc chain.
  int count = 0;
  int docID = results[k].Next();
  while (docID >= 0) {
    int i = 0;
    while (i <= j && results[i].Has(docID)) {
      i++;
    }
    if (i > j) {
      int pos = docID - min_doc;
      out.Set(pos);
      count++;
    }
    docID = results[k].Next();
  }

  // t.Stop();
  // t.Output();

  return count;
}

int MultiFieldsRangeIndex::AddField(int field, enum DataType field_type) {
  BTreeParameters bt_param;
  bt_param.mainleafxtra = 0;
  bt_param.maxleaves = 1000000;
  bt_param.poolsize = 500;
  bt_param.leafxtra = 0;
  bt_param.mainpool = 500;
  bt_param.mainbits = 16;
  bt_param.bits = 16;
  bt_param.kDelim = "\001";

  FieldRangeIndex *index =
      new FieldRangeIndex(path_, field, field_type, bt_param);
  fields_[field] = index;
  return 0;
}
}  // namespace tig_gamma
