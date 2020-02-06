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
#include <condition_variable>
#include <cstddef>
#include <cstdint>
#include <ctime>
#include <functional>
#include <iostream>
#include <iterator>
#include <limits>
#include <mutex>
#include <numeric>
#include <sstream>
#include <typeinfo>
#include "log.h"
#include "bitmap.h"
#include "threadskv10h.h"
#include "utils.h"

using std::string;
using std::vector;

#define BM_OPERATE_TYPE long

namespace tig_gamma {

static void FreeNodeData(void *data) { free(data); }

class Node {
 public:
  Node() {
    size_ = 0;
    data_ = nullptr;
    min_ = std::numeric_limits<int>::max();
    max_ = -1;
  }

  ~Node() { free(data_); }

  int Add(int val) {
    int op_len = sizeof(BM_OPERATE_TYPE) * 8;

    if (size_ == 0) {
      min_ = val;
      max_ = val;
      min_aligned_ = (val / op_len) * op_len;
      max_aligned_ = (val / op_len + 1) * op_len - 1;
      int bytes_count = -1;
      if (bitmap::create(data_, bytes_count, max_aligned_ - min_aligned_ + 1) !=
          0) {
        LOG(ERROR) << "Cannot create bitmap!";
        return -1;
      }
      bitmap::set(data_, val - min_aligned_);
      ++size_;
      return 0;
    }

    if (val < min_aligned_) {
      char *data = nullptr;
      int min_aligned = (val / op_len) * op_len;

      int bytes_count = -1;
      if (bitmap::create(data, bytes_count, max_aligned_ - min_aligned + 1) !=
          0) {
        LOG(ERROR) << "Cannot create bitmap!";
        return -1;
      }

      BM_OPERATE_TYPE *op_data_dst = (BM_OPERATE_TYPE *)data;
      BM_OPERATE_TYPE *op_data_ori = (BM_OPERATE_TYPE *)data_;

      for (int i = 0; i < (max_aligned_ - min_aligned_ + 1) / op_len; ++i) {
        op_data_dst[i + (min_aligned_ - min_aligned) / op_len] = op_data_ori[i];
      }

      bitmap::set(data, val - min_aligned);
      auto old_data = data_;
      data_ = data;
      min_ = val;
      min_aligned_ = min_aligned;
      utils::AsyncWait(1000, FreeNodeData, (void *)old_data);
    } else if (val > max_aligned_) {
      char *data = nullptr;
      // 2X spare space to speed up insert
      int max_aligned = (val / op_len + 1) * op_len * 2 - 1;

      int bytes_count = -1;
      if (bitmap::create(data, bytes_count, max_aligned - min_aligned_ + 1) !=
          0) {
        LOG(ERROR) << "Cannot create bitmap!";
        return -1;
      }

      BM_OPERATE_TYPE *op_data_dst = (BM_OPERATE_TYPE *)data;
      BM_OPERATE_TYPE *op_data_ori = (BM_OPERATE_TYPE *)data_;

      for (int i = 0; i < (max_aligned_ - min_aligned_ + 1) / op_len; ++i) {
        op_data_dst[i] = op_data_ori[i];
      }

      bitmap::set(data, val - min_aligned_);
      auto old_data = data_;
      data_ = data;
      max_ = val;
      max_aligned_ = max_aligned;
      utils::AsyncWait(1000, FreeNodeData, (void *)old_data);
    } else {
      bitmap::set(data_, val - min_aligned_);
      min_ = std::min(min_, val);
      max_ = std::max(max_, val);
    }

    ++size_;
    return 0;
  }

  int Min() { return min_; }
  int Max() { return max_; }

  int MinAligned() { return min_aligned_; }
  int MaxAligned() { return max_aligned_; }

  int Size() { return size_; }

  char *Data() { return data_; }

 private:
  int min_;
  int max_;
  int min_aligned_;
  int max_aligned_;

  int size_;
  char *data_;
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
  FieldRangeIndex(std::string &path, int field_idx, enum DataType field_type,
                  BTreeParameters &bt_param);
  ~FieldRangeIndex();

  int Add(unsigned char *key, uint key_len, int value);

  int Search(const string &low, const string &high, RangeQueryResult *result);

  int Search(const string &tags, RangeQueryResult *result);

  bool IsNumeric() { return is_numeric_; }

  char *Delim() { return kDelim_; }

 private:
  BtMgr *main_mgr_;
  BtMgr *cache_mgr_;
  bool is_numeric_;
  char *kDelim_;
  std::string path_;
};

FieldRangeIndex::FieldRangeIndex(std::string &path, int field_idx,
                                 enum DataType field_type,
                                 BTreeParameters &bt_param)
    : path_(path) {
  string cache_file =
      path + string("/cache_") + std::to_string(field_idx) + ".dis";
  string main_file =
      path + string("/main_") + std::to_string(field_idx) + ".dis";

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

  std::function<void(unsigned char *, uint)> InsertToBt = [&](
      unsigned char *key_to_add, uint key_len) {
    Node *p_node = nullptr;
    int ret = bt_findkey(bt, key_to_add, key_len, (unsigned char *)&p_node,
                         sizeof(Node *));

    if (ret < 0) {
      p_node = new Node;
      BTERR bterr =
          bt_insertkey(bt->main, key_to_add, key_len, 0,
                       static_cast<void *>(&p_node), sizeof(Node *), Unique);
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
                            RangeQueryResult *result) {
  if (!is_numeric_) {
    return Search(lower, result);
  }

#ifdef PERFORMANCE_TESTING
  double start = utils::getmillisecs();
#endif
  BtDb *bt = bt_open(cache_mgr_, main_mgr_);
  unsigned char key_l[lower.length()];
  unsigned char key_u[upper.length()];
  ReverseEndian(reinterpret_cast<const unsigned char *>(lower.data()), key_l,
                lower.length());
  ReverseEndian(reinterpret_cast<const unsigned char *>(upper.data()), key_u,
                upper.length());

  std::vector<Node *> lists;

  int min_doc = std::numeric_limits<int>::max();
  int min_aligned = std::numeric_limits<int>::max();
  int max_doc = 0;
  int max_aligned = 0;

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
        min_aligned = std::min(min_aligned, p_node->MinAligned());
        max_doc = std::max(max_doc, p_node->Max());
        max_aligned = std::max(max_aligned, p_node->MaxAligned());
      }
    }
  }

  bt_unlockpage(BtLockRead, bt->cacheset->latch, __LINE__);
  bt_unpinlatch(bt->cacheset->latch);

  bt_unlockpage(BtLockRead, bt->mainset->latch, __LINE__);
  bt_unpinlatch(bt->mainset->latch);
  bt_close(bt);

#ifdef PERFORMANCE_TESTING
  double search_bt = utils::getmillisecs();
#endif
  if (max_doc - min_doc + 1 <= 0) {
    return 0;
  }

  result->SetRange(min_aligned, max_aligned);
  result->Resize();
#ifdef PERFORMANCE_TESTING
  double end_resize = utils::getmillisecs();
#endif

  auto &bit_map = result->Ref();
  int list_size = lists.size();

  int total = 0;

  int op_len = sizeof(BM_OPERATE_TYPE) * 8;
  for (int i = 0; i < list_size; ++i) {
    Node *list = lists[i];
    char *data = list->Data();
    int min = list->MinAligned();
    int max = list->MaxAligned();

    total += list->Size();

    if (min < min_aligned || max > max_aligned) {
      continue;
    }

    BM_OPERATE_TYPE *op_data_dst = (BM_OPERATE_TYPE *)bit_map;
    BM_OPERATE_TYPE *op_data_ori = (BM_OPERATE_TYPE *)data;
    int offset = (min - min_aligned) / op_len;
    for (int j = 0; j < (max - min + 1) / op_len; ++j) {
      op_data_dst[j + offset] |= op_data_ori[j];
    }
  }

  result->SetDocNum(total);

#ifdef PERFORMANCE_TESTING
  double end = utils::getmillisecs();
  LOG(INFO) << "bt cost [" << search_bt - start << "], resize cost ["
            << end_resize - search_bt << "], assemble result ["
            << end - end_resize << "], total [" << end - start << "]";
#endif
  return max_doc - min_doc + 1;
}

int FieldRangeIndex::Search(const string &tags, RangeQueryResult *result) {
  std::vector<string> items = utils::split(tags, kDelim_);

  RangeQueryResult results_union[items.size()];

  int op_len = sizeof(BM_OPERATE_TYPE) * 8;

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
      continue;
    }
    min_doc = std::min(min_doc, p_node->Min());
    max_doc = std::max(max_doc, p_node->Max());

    if (max_doc - min_doc + 1 <= 0) {
      return 0;
    }

    int min_aligned = p_node->MinAligned();
    int max_aligned = p_node->MaxAligned();

    results_union[i].SetRange(min_aligned, max_aligned);
    results_union[i].Resize();

    results_union[i].SetDocNum(p_node->Size());

    char *data = p_node->Data();
    char *&bitmap = results_union[i].Ref();
    BM_OPERATE_TYPE *op_data_dst = (BM_OPERATE_TYPE *)bitmap;
    BM_OPERATE_TYPE *op_data_ori = (BM_OPERATE_TYPE *)data;

    for (int j = 0; j < (max_aligned - min_aligned + 1) / op_len; ++j) {
      op_data_dst[j] = op_data_ori[j];
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

  int total = 0;

  result->SetRange(min_doc, max_doc);
  result->Resize();

  char *&bitmap = result->Ref();

  for (size_t i = 0; i < items.size(); ++i) {
    char *data = results_union[i].Ref();

    int min = results_union[i].Min();
    int max = results_union[i].Max();

    if (min < min_doc || max > max_doc) {
      continue;
    }

    total += results_union[i].Size();

    BM_OPERATE_TYPE *op_data_dst = (BM_OPERATE_TYPE *)bitmap;
    BM_OPERATE_TYPE *op_data_ori = (BM_OPERATE_TYPE *)data;

    int offset = (min - min_doc) / op_len;
    for (int j = 0; j < (max - min + 1) / op_len; ++j) {
      op_data_dst[j + offset] |= op_data_ori[j];
    }
  }

  result->SetDocNum(total);
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

int MultiFieldsRangeIndex::Search(const std::vector<FilterInfo> &origin_filters,
                                  MultiRangeQueryResults *out) {
  out->Clear();

  std::vector<FilterInfo> filters;

  for (const auto &filter : origin_filters) {
    FieldRangeIndex *index = fields_[filter.field];
    if (index == nullptr || filter.field < 0) {
      return -1;
    }
    if (not index->IsNumeric() && (filter.is_union == 0)) {
      // type is string and operator is "and", split this filter
      std::vector<string> items =
          utils::split(filter.lower_value, index->Delim());
      for (string &item : items) {
        FilterInfo f = filter;
        f.lower_value = item;
        filters.push_back(f);
      }
      continue;
    }
    filters.push_back(filter);
  }

  int fsize = filters.size();
  vector<RangeQueryResult *> results(fsize);

  if (1 == fsize) {
    auto &filter = filters[0];
    RangeQueryResult *result = new RangeQueryResult;
    FieldRangeIndex *index = fields_[filter.field];

    int retval = index->Search(filter.lower_value, filter.upper_value, result);
    if (retval > 0) {
      out->Add(result);
    }
    // result->Output();
    return retval;
  }

  for (int i = 0; i < fsize; ++i) {
    results[i] = new RangeQueryResult;
  }

  int valuable_result = -1;
  // record the shortest docid list
  int shortest_idx = -1, shortest = std::numeric_limits<int>::max();

  for (int i = 0; i < fsize; ++i) {
    auto &filter = filters[i];

    FieldRangeIndex *index = fields_[filter.field];
    if (index == nullptr || filter.field < 0) {
      continue;
    }

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

  RangeQueryResult *tmp = new RangeQueryResult;
  int count = Intersect(results.data(), valuable_result, shortest_idx, tmp);
  if (count > 0) {
    out->Add(tmp);
  }

  return count;
}

int MultiFieldsRangeIndex::Intersect(RangeQueryResult **results, int j,
                                     int shortest_idx, RangeQueryResult *out) {
  assert(results != nullptr && j >= 0);

  // I want to build a smaller bitmap ...
  int min_doc = results[0]->MinAligned();
  int max_doc = results[0]->MaxAligned();

  int total = results[0]->Size();

  // results[0]->Output();

  for (int i = 1; i <= j; i++) {
    RangeQueryResult *r = results[i];

    // the maximum of the minimum(s)
    if (r->MinAligned() > min_doc) {
      min_doc = r->MinAligned();
    }
    // the minimum of the maximum(s)
    if (r->MaxAligned() < max_doc) {
      max_doc = r->MaxAligned();
    }
  }

  if (max_doc - min_doc + 1 <= 0) {
    return 0;
  }
  out->SetRange(min_doc, max_doc);
  out->Resize();

  out->SetDocNum(total);

  char *&bitmap = out->Ref();

  int op_len = sizeof(BM_OPERATE_TYPE) * 8;

  BM_OPERATE_TYPE *op_data_dst = (BM_OPERATE_TYPE *)bitmap;

  // calculate the intersection with the shortest doc chain.
  {
    char *data = results[shortest_idx]->Ref();

    BM_OPERATE_TYPE *op_data_ori = (BM_OPERATE_TYPE *)data;
    for (int j = 0; j < (max_doc - min_doc + 1) / op_len; ++j) {
      op_data_dst[j] = op_data_ori[j];
    }
  }

  for (int i = 0; i < j; ++i) {
    if (i == shortest_idx) {
      continue;
    }
    char *data = results[i]->Ref();
    BM_OPERATE_TYPE *op_data_ori = (BM_OPERATE_TYPE *)data;
    int min = results[i]->MinAligned();
    int max = results[i]->MaxAligned();

    if (min < min_doc || max > max_doc) {
      continue;
    }

    int offset = (min - min_doc) / op_len;
    for (int j = 0; j < (max - min + 1) / op_len; ++j) {
      op_data_dst[j + offset] &= op_data_ori[j];
    }
  }

  return total;
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
