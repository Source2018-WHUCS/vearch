/**
 * Copyright(C) JD.COM, all rights reserved.
 * Author: Chen Jianyu (chenjianyu@jd.com)
 */
#ifndef SRC_SEARCHER_INDEX_NUMERIC_INDEX_H_
#define SRC_SEARCHER_INDEX_NUMERIC_INDEX_H_

#include <CLI11.hpp> // for lexical_cast
#include <random.h>
#include <string.h> // for memcpy

#include <cassert>
#include <cstddef>
#include <cstdint>
#include <ctime>
#include <typeinfo>

#include <algorithm>
#include <atomic>
#include <functional>
#include <glog/logging.h>
#include <iostream>
#include <limits>
#include <map>
#include <string>
#include <vector>

#include "../util/timer.h" // for Timer

using CLI::detail::lexical_cast;

namespace tig_gamma {
namespace NI {

// 必须在性能和内存之间做下的折衷，
// 数据分布稠密时，内存浪费会比较少，反之，数据分布稀疏时，内存浪费会比较多
const int kDupNodeSize = 512;

typedef int64_t Long; // make cpplint happy

// used for dump & load
template <typename T> inline const char *TypeName() { return typeid(T).name(); }

template <> inline const char *TypeName<int>() { return "I"; }
template <> inline const char *TypeName<Long>() { return "L"; }
template <> inline const char *TypeName<float>() { return "F"; }
template <> inline const char *TypeName<double>() { return "D"; }

// forward declarations
template <typename K, typename V> class SkipList;
template <typename K, typename V> struct Node;

//======================================================================
// Numeric Index Implement
//======================================================================
struct RangeFilter {
  std::string field;
  std::string lower_value;
  std::string upper_value;
};

typedef std::vector<bool> BitmapType;

// do intersection immediately
class RangeQueryResultV1 {
public:
  RangeQueryResultV1() : flags_(0x1 | 0x2) { Clear(); }
  explicit RangeQueryResultV1(int flags) : flags_(flags) { Clear(); }

  inline bool Has(int doc) const;

  /**
   * @return docID in order, -1 for the end
   */
  int Next() const;

  /**
   * @return size of docID list
   */
  int Size() const;

  void Clear();

public:
  void SetRange(int x, int y) {
    min_ = std::min(min_, x);
    max_ = std::max(max_, y);
  }

  void Resize(bool init_value = false) {
    int n = max_ - min_ + 1;
    assert(n > 0);
    bitmap_.resize(n, init_value);
    docids.reserve(n / 10);  // reserve memory to store docids
  }

  void Set(int pos) { bitmap_[pos] = true; docids.push_back(pos + min_); }

  int Min() const { return min_; }
  int Max() const { return max_; }

  void SetFlags(int flags) {
    flags_ = flags; // test use only
  }
  int Flags() { return flags_; }

  BitmapType &Ref() { return bitmap_; }

  const std::vector<int>& GetDocIds() const {return docids;}

  /**
   * @return sorted docIDs
   */
  std::vector<int> ToDocs() const; // WARNING: build dynamically
  void Output();

private:
  int flags_;
  int min_;
  int max_;

  mutable int next_;
  mutable int n_doc_;

  BitmapType bitmap_;
  std::vector<int> docids;
};

inline bool RangeQueryResultV1::Has(int doc) const {
  if (doc < min_ || doc > max_) {
    return false;
  }

  doc -= min_;
  return bitmap_[doc];
}

inline int RangeQueryResultV1::Next() const {
  next_++;

  int size = bitmap_.size();
  while (next_ < size && not bitmap_[next_]) {
    next_++;
  }
  if (next_ >= size) {
    return -1;
  }

  int doc = next_ + min_;
  return doc;
}

inline void RangeQueryResultV1::Clear() {
  // flags_ = DO NOT CLEAR
  min_ = std::numeric_limits<int>::max();
  max_ = 0;
  next_ = -1;
  n_doc_ = -1;
  bitmap_.clear();
  docids.clear();
}

inline int RangeQueryResultV1::Size() const {
  if (n_doc_ >= 0) {
    return n_doc_;
  }

  n_doc_ = 0;
  for (auto i : bitmap_) {
    if (i) {
      n_doc_++;
    }
  }
  return n_doc_;
}

inline std::vector<int> RangeQueryResultV1::ToDocs() const {
  if (n_doc_ >= 0) {
    std::vector<int> docIDs(n_doc_);
    int j = 0;

    for (size_t i = 0; i < bitmap_.size(); i++) {
      if (bitmap_[i]) {
        docIDs[j++] = i + min_;
      }
    }

    assert(j == n_doc_);
    return docIDs;
  } else {
    std::vector<int> docIDs;

    for (size_t i = 0; i < bitmap_.size(); i++) {
      if (bitmap_[i]) {
        docIDs.emplace_back(i + min_);
      }
    }

    n_doc_ = static_cast<int>(docIDs.size());
    return docIDs;
  }
}

inline void RangeQueryResultV1::Output() {
  std::cout << "bitmap = [";
  for (size_t i = 0; i < bitmap_.size(); i++) {
    if (bitmap_[i]) {
      std::cout << " " << i;
    }
  }
  std::cout << " ]\n";
}

// do intersection lazily
class RangeQueryResult {
public:
  RangeQueryResult() : flags_(0x1 | 0x2) { Clear(); }

  // Take full advantage of multi-core while recalling
  bool Has(int doc) const {
    bool ret = true;
    for (auto &result : all_results_) {
      ret &= result.Has(doc);
    }
    return ret;
  }

  void Clear() {
    // flags_ = DO NOT CLEAR
    min_ = 0;
    max_ = std::numeric_limits<int>::max();
    all_results_.clear();
  }

public:
  void Add(const RangeQueryResultV1 &r) {
    all_results_.emplace_back(r);

    // 取最小值中的最大值
    if (r.Min() > min_) {
      min_ = r.Min();
    }
    // 取最大值中的最小值
    if (r.Max() < max_) {
      max_ = r.Max();
    }
  }

  void SetFlags(int flags) {
    flags_ = flags; // test use only
  }
  int Flags() { return flags_; }

  int Min() const { return min_; }
  int Max() const { return max_; }

  /** WARNING: build dynamically
   * @return sorted docIDs
   */
  std::vector<int> ToDocs() const {
    std::vector<int> docIDs;

    if (not all_results_.empty()) {
      for (int id = min_; id <= max_; id++) {
        if (Has(id)) {
          docIDs.emplace_back(id);
        }
      }
    }

    return docIDs;
  }

  const std::vector<RangeQueryResultV1>& GetAllResult() const {return all_results_;}

private:
  int flags_;
  int min_;
  int max_;

  std::vector<RangeQueryResultV1> all_results_;
};

inline void Output(const std::vector<int> &docs) {
  std::cout << "docs=";
  std::copy(docs.begin(), docs.end(),
            std::ostream_iterator<int>(std::cout, ","));
  std::cout << "\n";
}

//======================================================================
enum class IndexFieldType : uint8_t {
  UNKNOWN = 0,

  INT,
  LONG,
  FLOAT,
  DOUBLE,
};

struct IndexField {
  std::string name;
  IndexFieldType type;
};

class IndexIO {
public:
  int Write() { return -1; }
  int Read() { return -1; }

private:
  FILE *_fp;
};

struct Index {
  virtual ~Index() {}

  virtual int Search(const std::string &, const std::string &,
                     RangeQueryResultV1 &) const {
    return -1;
  }
  virtual void Add(const char *, int) {}
  virtual int Build() { return -1; }
  virtual size_t MemoryUsage() const { return 0; }
  virtual void Output(const std::string &) {}
  virtual int Dump(const IndexIO &) { return -1; }
  virtual int Load(const IndexIO &) { return -1; }

  IndexField field_;
};

struct Block {
  Long offset; // offset in docValues
  int size;

  int min_doc;
  int max_doc;
};

template <typename T> struct BlockSkipListIndex {
  BlockSkipListIndex() : size(0) {}

  std::vector<int> docIDs; // sorted by docValues
  Long size;

  T min_value;
  T max_value;

  std::vector<Block> blocks;
  SkipList<T, int> index;
};

template <typename T> class NumericIndex : public Index {
public:
  NumericIndex(const std::string &field, int n_docs);

  int Search(const std::string &lowerValue, const std::string &upperValue,
             RangeQueryResultV1 &result) const override {
    T l, u;
    bool rv = lexical_cast(lowerValue, l) && lexical_cast(upperValue, u);
    if (rv) {
      return _Search(l, u, result);
    }
    return -1;
  }

  void Add(const char *bytes, int docID) override {
    T value;
    memcpy(&value, bytes, sizeof(T));
    Add(value, docID);
  }

  void Add(T value, int docID) {
    // only rt
    rt_idx_.Insert(value, docID);
  }

  int Build() override;
  void Output(const std::string &tag) override;

  int Dump(const IndexIO &out) override;
  int Load(const IndexIO &in) override;

  // use callback instead of _raw
  void Set(std::function<T(const int)> callback) {
    getRaw_ = std::move(callback);
  }

  size_t MemoryUsage() const override {
    size_t bytes = bsl_idx_.size * sizeof(int) +
                   bsl_idx_.blocks.size() * sizeof(Block) +
                   bsl_idx_.index.MemoryUsage();
    bytes += rt_idx_.MemoryUsage();
    return bytes;
  }

private:
  int _Search(const T lowerValue, const T upperValue,
              RangeQueryResultV1 &result) const;

  int _Search(const BlockSkipListIndex<T> *index, const T lowerValue,
              const T upperValue, RangeQueryResultV1 &result) const;

  int _Search(const SkipList<T, int> *rt_idx, const T lowerValue,
              const T upperValue, RangeQueryResultV1 &result) const;

  // similar to std::lower_bound & std::upper_bound
  Long _LowerBound(std::function<T(int)> getValue, Long first, Long last,
                   const T &value) const;
  Long _UpperBound(std::function<T(int)> getValue, Long first, Long last,
                   const T &value) const;

private:
  std::function<T(const int)> getRaw_; // 取正排数据
  Long size_;

  static const int kBlockSize_ = 1024;

  // TODO dynamically merging
  BlockSkipListIndex<T> bsl_idx_;
  SkipList<T, int> rt_idx_;
};

template <typename T>
NumericIndex<T>::NumericIndex(const std::string &field, int n_docs)
    : size_(n_docs) {
  field_.name = field;
  const char *type = TypeName<T>();

  // it's enough to compare the first char only
  switch (type[0]) {
  case 'I':
    field_.type = IndexFieldType::INT;
    break;
  case 'L':
    field_.type = IndexFieldType::LONG;
    break;
  case 'F':
    field_.type = IndexFieldType::FLOAT;
    break;
  case 'D':
    field_.type = IndexFieldType::DOUBLE;
    break;
  default:
    field_.type = IndexFieldType::UNKNOWN;
    break;
  }

  rt_idx_.SetAllowDup(true);
}

template <typename T> int NumericIndex<T>::Build() {
  if (size_ < 1) {
    return -1;
  }

  bsl_idx_.size = size_;

  bsl_idx_.docIDs.resize(size_);
  std::iota(bsl_idx_.docIDs.begin(), bsl_idx_.docIDs.end(), 0);

  // sort _docIDs by _raw, use getRaw_(i) instead of _raw[i]
  std::sort(bsl_idx_.docIDs.begin(), bsl_idx_.docIDs.end(),
            [&](int i, int j) { return getRaw_(i) < getRaw_(j); });

  std::function<T(int)> getValue = [&](int n) -> T {
    int docid = bsl_idx_.docIDs[n]; // for performance, no validity checking
    return getRaw_(docid);
  };

  bsl_idx_.min_value = getValue(0);
  bsl_idx_.max_value = getValue(size_ - 1);

  std::cout << "min_value=" << bsl_idx_.min_value
            << ", max_value=" << bsl_idx_.max_value << "\n";

  // split to blocks
  bsl_idx_.blocks.reserve(1 + size_ / kBlockSize_);
  int nBlock = 0;

  T curr_blk_start = getValue(0);

  Long i = 0;
  Block blk;

  while (i < size_) {
    bsl_idx_.index.Insert(curr_blk_start, nBlock++);

    blk.offset = i;
    i += kBlockSize_;

    // the last block
    if (i >= size_) {
      blk.size = size_ - blk.offset;
      bsl_idx_.blocks.emplace_back(blk);
      break;
    }

    // keep the same values in the same block
    T prev_blk_end = getValue(i - 1);
    curr_blk_start = getValue(i);

    while (curr_blk_start == prev_blk_end) {
      i += 1;
      if (i >= size_) {
        break;
      }
      curr_blk_start = getValue(i);
    }

    blk.size = i - blk.offset;
    bsl_idx_.blocks.emplace_back(blk);
  }

  bsl_idx_.blocks.shrink_to_fit();

  // calculate min_doc & max_doc in each block
  // [begin, end)
  for (auto &blk : bsl_idx_.blocks) {
    auto begin = bsl_idx_.docIDs.cbegin() + blk.offset;
    auto end = begin + blk.size;

    blk.min_doc = *(std::min_element(begin, end));
    blk.max_doc = *(std::max_element(begin, end));

#ifdef DEBUG
    std::cout << "min_doc=" << blk.min_doc << ", max_doc=" << blk.max_doc
              << "\n";
#endif
  }

  std::cout << "Num of blocks: " << bsl_idx_.blocks.size()
            << ", capacity: " << bsl_idx_.blocks.capacity() << "\n";
  return 0;
}

template <typename T> void NumericIndex<T>::Output(const std::string &tag) {
  bsl_idx_.index.Output(tag);

  std::cout << "Num of blocks: " << bsl_idx_.blocks.size() << "\n";
  std::cout << "-------- only list the first 10 blocks ---------\n";
  int count = 0;
  for (auto &blk : bsl_idx_.blocks) {
    std::cout << "\toffset:" << blk.offset << ", size:" << blk.size << "\n";
    if (count++ > 10) {
      break;
    }
  }

  size_t bytes = rt_idx_.MemoryUsage();
  float MB = 1.0 * bytes / 1048576;
  std::cout << tag << " -> rt size: " << rt_idx_.Size()
            << ", memory usage: " << bytes << " bytes (" << MB << " MB).\n";
}

template <typename T> int NumericIndex<T>::Dump(const IndexIO &out) {
  // TODO
  // 目前依赖profile，在profile加载完成后，dynamically Build()
  return 0;
}

template <typename T> int NumericIndex<T>::Load(const IndexIO &in) {
  // TODO
  // 目前依赖profile，在profile加载完成后，dynamically Build()
  return 0;
}

template <typename T>
Long NumericIndex<T>::_LowerBound(std::function<T(int)> getValue, Long first,
                                  Long last, const T &value) const {
  // optimize
  if (getValue(first) >= value) {
    return first;
  } else if (getValue(last - 1) < value) {
    return last;
  }

  Long step(0), it(0);
  Long count = last - first;

  while (count > 0) {
    step = count / 2;
    it = first + step;
    if (getValue(it) < value) {
      first = ++it;
      count -= step + 1;
    } else {
      count = step;
    }
  }

  return first;
}

template <typename T>
Long NumericIndex<T>::_UpperBound(std::function<T(int)> getValue, Long first,
                                  Long last, const T &value) const {
  // optimize
  if (getValue(first) > value) {
    return first;
  } else if (getValue(last - 1) <= value) {
    return last;
  }

  Long step(0), it(0);
  Long count = last - first;

  while (count > 0) {
    step = count / 2;
    it = first + step;
    if (!(value < getValue(it))) {
      first = ++it;
      count -= step + 1;
    } else {
      count = step;
    }
  }

  return first;
}

static void _SetBitmap(const std::vector<int> &docIDs, Long begin, Long end,
                       RangeQueryResultV1 &result) {
  if (docIDs.empty())
    return;

  // Timer t;
  // t.Start("_SetBitmap");

  int min_doc = result.Min();

  // for (auto& docID : docIDs) {
  //     int pos = docID - min_doc;
  //     result.Set(pos);
  // }

  // BLOCK: optimized code of the above
  if (end > begin) {
#define SET_bitmap                                                             \
  do {                                                                         \
    int pos = docIDs[i] - min_doc;                                             \
    result.Set(pos);                                                           \
    i++;                                                                       \
  } while (0)

    // Duff's device, count must be greater than 0
    register int count = static_cast<int>(end - begin);
    register int n = (count + 7) / 8;
    register int i = begin;

    switch (count % 8) {
    case 0:
      do {
        SET_bitmap;
      case 7:
        SET_bitmap;
      case 6:
        SET_bitmap;
      case 5:
        SET_bitmap;
      case 4:
        SET_bitmap;
      case 3:
        SET_bitmap;
      case 2:
        SET_bitmap;
      case 1:
        SET_bitmap;
      } while (--n > 0);
    }
#undef SET_bitmap
  }

  // t.Stop();
  // t.Output();
}

template <typename T>
int NumericIndex<T>::_Search(const T lowerValue, const T upperValue,
                             RangeQueryResultV1 &result) const {
  result.Clear();

  if (lowerValue > upperValue) {
    return 0; // 无结果
  }

  T min_value = bsl_idx_.min_value;
  T max_value = bsl_idx_.max_value;

  if (rt_idx_.Size() > 0) {
    min_value = std::min(min_value, rt_idx_.First()->key);
    max_value = std::max(max_value, rt_idx_.Last()->key);
  }

  if (lowerValue > max_value || upperValue < min_value) {
    return 0; // 无结果
  }
  if (lowerValue <= min_value && max_value <= upperValue) {
    return -1; // 全集
  }

  // WARNING: 一个search请求始终使用同一份索引
  int count = 0;

  if (result.Flags() & 0x1) {
    count += _Search(&bsl_idx_, lowerValue, upperValue, result);
  }
  if (result.Flags() & 0x2) {
    count += _Search(&rt_idx_, lowerValue, upperValue, result);
  }

  return count;
}

template <typename T>
int NumericIndex<T>::_Search(const SkipList<T, int> *rt_idx, const T lowerValue,
                             const T upperValue,
                             RangeQueryResultV1 &result) const {
  // std::cout << "[INFO] search rt-skiplist index.\n";
  assert(rt_idx != nullptr);

  if (rt_idx->Size() < 1) {
    return -1;
  }

  T min_value = rt_idx->First()->key;
  T max_value = rt_idx->Last()->key;

  Node<T, int> *left = nullptr;
  Node<T, int> *right = nullptr;

  if (lowerValue < min_value /*&& upperValue <= max_value*/) {
    left = rt_idx->First();
    right = rt_idx->FindLessOrEqual(upperValue);
  } else if (/*lowerValue >= min_value &&*/ upperValue > max_value) {
    left = rt_idx->FindGreaterOrEqual(lowerValue);
    right = rt_idx->Last();
  } else { // min_value <= lowerValue && upperValue <= max_value
    left = rt_idx->FindGreaterOrEqual(lowerValue);
    right = rt_idx->FindLessOrEqual(upperValue);
  }

  assert(left != nullptr && right != nullptr);

  Node<T, int> *end = right->Next(0); // 开区间

  // TODO 改为可以动态增长的bitmap ?
  std::vector<int> docIDs; // unordered

  // I want to build a smaller bitmap ...
  int min_doc = std::numeric_limits<int>::max();
  int max_doc = 0;

  // Timer t;
  // t.Start("visit");

  // 不同值的docID列表(docID是无序的, docID总是比其相同值的docID小)
  for (; left != end; left = left->Next(0)) {
    docIDs.emplace_back(left->value);

    if (left->value < min_doc) {
      min_doc = left->value;
    }

    if (max_doc < left->value) {
      max_doc = left->value;
    }

    // 相同值的docID列表(docID是顺序的，最后一个相同值节点的docID是最大的)
    // WARNING 注意节点访问的线程安全问题
    auto dup = left->Dup();
    if (dup) {
      auto node = dup->head;
      for (auto tail = dup->Tail(); node != tail; node = node->next) {
        docIDs.insert(docIDs.end(), std::begin(node->ids), std::end(node->ids));
      }

      for (auto id : node->ids) {
        if (id < 0)
          break;

        docIDs.emplace_back(id);
      }

      if (max_doc < docIDs.back()) {
        max_doc = docIDs.back();
      }
    }
  }

  // t.Stop();
  // t.Output();

  if (docIDs.empty()) {
    return 0; // 无结果
  }

  // re-build docID's bitmap
  result.SetRange(min_doc, max_doc);
  result.Resize();

  _SetBitmap(docIDs, 0, docIDs.size(), result);

  return static_cast<int>(docIDs.size());
}

template <typename T>
int NumericIndex<T>::_Search(const BlockSkipListIndex<T> *bsl_idx,
                             const T lowerValue, const T upperValue,
                             RangeQueryResultV1 &result) const {
  // std::cout << "[INFO] search block-skiplist index.\n";
  assert(bsl_idx != nullptr);

  if (bsl_idx->size < 1) {
    return -1;
  }

  T min_value = bsl_idx->min_value;
  T max_value = bsl_idx->max_value;

  // if (lowerValue > max_value || upperValue < min_value) {
  //  return 0; // 无结果
  //}
  // if (lowerValue <= min_value && max_value <= upperValue) {
  //  return -1; // 全集
  //}

  std::function<T(int)> getValue = [&](int n) -> T {
    int docid = bsl_idx->docIDs[n]; // for performance, no validity checking
    return getRaw_(docid);
  };

  // calculate [begin, end)
  Long begin = -1, end = -1;
  // [lv, uv] used to optimize bitmap size
  int lv = -1, uv = -1;

  Node<T, int> *node(nullptr);
  Long first = -1, last = -1;

  if (lowerValue < min_value /*&& upperValue <= max_value*/) {
    lv = 0;
    begin = 0;

    node = bsl_idx->index.FindLessOrEqual(upperValue);

    uv = node->value; // last block
    first = bsl_idx->blocks[uv].offset;
    last = first + bsl_idx->blocks[uv].size;

    end = _UpperBound(getValue, first, last, upperValue);

  } else if (/*lowerValue >= min_value &&*/ upperValue > max_value) {
    uv = bsl_idx->blocks.size() - 1;
    end = bsl_idx->size;

    node = bsl_idx->index.FindLessOrEqual(lowerValue);

    lv = node->value; // 1st block
    first = bsl_idx->blocks[lv].offset;
    last = first + bsl_idx->blocks[lv].size;

    begin = _LowerBound(getValue, first, last, lowerValue);

  } else { // min_value <= lowerValue && upperValue <= max_value
    node = bsl_idx->index.FindLessOrEqual(lowerValue);

    lv = node->value; // 1st block
    first = bsl_idx->blocks[lv].offset;
    last = first + bsl_idx->blocks[lv].size;

    // _LowerBound返回的是第一个不小于(即大于或等于)lowerValue的迭代器
    begin = _LowerBound(getValue, first, last, lowerValue);

    node = bsl_idx->index.FindLessOrEqual(upperValue);

    uv = node->value; // last block
    first = bsl_idx->blocks[uv].offset;
    last = first + bsl_idx->blocks[uv].size;

    // _UpperBound返回的是第一个大于upperValue的迭代器
    end = _UpperBound(getValue, first, last, upperValue);
  }

  if (end == begin) {
    return 0; // 无结果
  }

  // assign DocIDs
  // docIDs.insert(docIDs.end(), bsl_idx->docIDs.cbegin() + begin,
  //              bsl_idx->docIDs.cbegin() + end);

  // I want to build a smaller bitmap ...
  int min_doc = bsl_idx->size - 1;
  int max_doc = 0;

  for (auto i = lv; i <= uv; i++) {
    auto &b = bsl_idx->blocks[i];
    min_doc = std::min(min_doc, b.min_doc);
    max_doc = std::max(max_doc, b.max_doc);
  }

  // build docID's bitmap
  result.SetRange(min_doc, max_doc);
  result.Resize();

  _SetBitmap(bsl_idx->docIDs, begin, end, result);

  return (end - begin);
}

class Indexes {
public:
  ~Indexes() {
    for (auto &one : indexes_) {
      delete one.second;
    }
  }

  // search & do intersection *** immediately ***
  int Search(const std::vector<RangeFilter> &filters,
             RangeQueryResultV1 &result) const;

  // search & do intersection *** lazily ***
  int Search(const std::vector<RangeFilter> &filters,
             RangeQueryResult &out) const;

  int Search(const std::string &field, const std::string &lowerValue,
             const std::string &upperValue, RangeQueryResultV1 &result) const {
    Index *index = _GetIndex(field);
    if (nullptr == index) {
      return -1;
    }
    return index->Search(lowerValue, upperValue, result);
  }

  int Search(const std::string &field, const std::string &value,
             RangeQueryResultV1 &result) const {
    return Search(field, value, value, result);
  }

  template <typename T>
  int Indexing(const std::string &field, int n_docs,
               std::function<T(const int)> cb);

  void Add(int docID, const std::string &field, const char *value) {
    Index *index = _GetIndex(field);
    if (index) {
      index->Add(value, docID);
    }
  }

  size_t MemoryUsage() const {
    size_t bytes = 0;
    for (auto &one : indexes_) {
      bytes += one.second->MemoryUsage();
    }
    return bytes;
  }

  void Output() const {
    for (auto &one : indexes_) {
      one.second->Output(one.first);
    }
  }

  int Dump(const std::string &file) {
    return 0; // not implemented
  }
  int Load(const std::string &file) {
    return 0; // not implemented
  }

private:
  Index *_GetIndex(const std::string &field) const {
    auto iter = indexes_.find(field);
    return iter != indexes_.end() ? iter->second : nullptr;
  }

  int _Intersect(const RangeQueryResultV1 *results, int j, int k,
                 RangeQueryResultV1 &out) const;

private:
  static const int kLazyThreshold_ = 10000;

  std::map<std::string, Index *> indexes_;
};

template <typename T>
inline int Indexes::Indexing(const std::string &field, int n_docs,
                             std::function<T(const int)> cb) {
  if (!_GetIndex(field)) {
    auto idx = new (std::nothrow) NumericIndex<T>(field, n_docs);
    if (idx) {
      idx->Set(cb); // set callback before build
      if (idx->Build() < 0) {
        delete idx;
      } else {
        indexes_.insert({field, idx});
        return 0;
      }
    }
  }

  return -1;
}

inline int Indexes::Search(const std::vector<RangeFilter> &filters,
                           RangeQueryResultV1 &out) const {
  out.Clear();
  int fsize = filters.size();

  if (1 == fsize) {
    auto &_ = filters[0];
    return Search(_.field, _.lower_value, _.upper_value, out);
  }

  // Timer t;
  // t.Start("Search");

  RangeQueryResultV1 results[fsize];
  int j = -1;
  // 记录最短的docID列表
  int k = -1, k_size = std::numeric_limits<int>::max();

  for (int i = 0; i < fsize; i++) {
    results[j + 1].SetFlags(out.Flags());

    auto &_ = filters[i];
    int retval = Search(_.field, _.lower_value, _.upper_value, results[j + 1]);
    if (retval < 0) {
      ;
    } else if (retval == 0) {
      return 0; // 有一个无结果，就不必求交了
    } else {
      j += 1;

      if (k_size > retval) {
        k_size = retval;
        k = j;
      }
    }
  }

  if (j < 0) {
    return -1; // 全集
  }

  // t.Stop();
  // t.Output();

  return _Intersect(results, j, k, out);
}

inline int Indexes::Search(const std::vector<RangeFilter> &filters,
                           RangeQueryResult &out) const {
  out.Clear();
  int fsize = filters.size();

  if (1 == fsize) {
    RangeQueryResultV1 tmp(out.Flags());
    auto &_ = filters[0];
    int retval = Search(_.field, _.lower_value, _.upper_value, tmp);
    if (retval > 0) {
      out.Add(tmp);
    }
    return retval;
  }

  // Timer t;
  // t.Start("Search");

  RangeQueryResultV1 results[fsize];
  int j = -1;
  // 记录最短的docID列表
  int k = -1, k_size = std::numeric_limits<int>::max();

  for (int i = 0; i < fsize; i++) {
    results[j + 1].SetFlags(out.Flags());

    auto &_ = filters[i];
    int retval = Search(_.field, _.lower_value, _.upper_value, results[j + 1]);
    if (retval < 0) {
      ;
    } else if (retval == 0) {
      return 0; // 有一个无结果，就不必求交了
    } else {
      j += 1;

      if (k_size > retval) {
        k_size = retval;
        k = j;
      }
    }
  }

  if (j < 0) {
    return -1; // 全集
  }

  // t.Stop();
  // t.Output();

  // 当最短doc链较长时，采用lazy计算，不求交集
  if (k_size > kLazyThreshold_) {
    for (int i = 0; i <= j; i++) {
      out.Add(results[i]);
    }
    return 1; // it's hard to count the return docs
  }

  RangeQueryResultV1 tmp(out.Flags());
  int count = _Intersect(results, j, k, tmp);
  if (count > 0) {
    out.Add(tmp);
  }

  return count;
}

inline int Indexes::_Intersect(const RangeQueryResultV1 *results, int j, int k,
                               RangeQueryResultV1 &out) const {
  assert(results != nullptr && j >= 0);

  // t.Start("Intersect");
  // I want to build a smaller bitmap ...
  int min_doc = results[0].Min();
  int max_doc = results[0].Max();

  for (int i = 1; i <= j; i++) {
    auto &r = results[i];

    // 取最小值中的最大值
    if (r.Min() > min_doc) {
      min_doc = r.Min();
    }
    // 取最大值总的最小值
    if (r.Max() < max_doc) {
      max_doc = r.Max();
    }
  }

#if 0  // too slow
  out.SetRange(min_doc, max_doc);
  out.Resize(true);

  // 求交集
  for (int i = 0; i <= j; i++) {
    auto &r = results[i];

    int x = min_doc - r.Min();
    int y = r.Max() - max_doc;

    std::transform(r.Ref().begin() + x, r.Ref().end() - y, // operand 1
                   out.Ref().begin(),                      // operand 2
                   out.Ref().begin(), std::bit_and<bool>());
  }

  return out.Size();
#endif // too slow

  out.SetRange(min_doc, max_doc);
  out.Resize();

  // 以最短的doc链，求交集
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

//======================================================================
// A Simple SkipList Index Implement
//======================================================================
// Note: 数据分布越稠密，内存浪费越少，检索性能也越好
template <typename T> struct DupNode {
  explicit DupNode(const T _1st) : next(nullptr) {
    std::fill_n(ids, kDupNodeSize, -1);
    ids[0] = _1st;
  }

  T ids[kDupNodeSize];
  DupNode *next;
};

template <typename T> struct DupList {
  DupList() : head(nullptr), size_(0) {}

  DupNode<T> *head;

  bool Add(T x) {
    int pos = size_ % kDupNodeSize + 1;
    if (pos == kDupNodeSize) {
      return false; // require to Insert() a new node
    }

    Tail()->ids[pos] = x;
    size_++;
    return true;
  }

  void Insert(DupNode<T> *x) {
    // Note: 遍历时只遍历到Tail()，故next可被安全修改
    Tail()->next = x;
    SetTail(x);
    size_++;
  }

  void SetTail(DupNode<T> *x) { tail_.store(x, std::memory_order_release); }
  DupNode<T> *Tail() { return tail_.load(std::memory_order_acquire); }

private:
  std::atomic<DupNode<T> *> tail_;
  int size_; // 仅在rt.Insert线程(单线程)中使用，用于定位插入的slot
};

template <typename K, typename V> struct Node {
  Node(int level, K k, V v) : level(level), key(k), value(v) {}

  int level; // used for _OutputNode

  K key;
  V value;

  Node *Next(int n) {
    assert(n >= 0);
    return (next_[n].load(std::memory_order_acquire));
  }
  void SetNext(int n, Node *x) {
    assert(n >= 0);
    next_[n].store(x, std::memory_order_release);
  }
  Node *NoBarrier_Next(int n) {
    assert(n >= 0);
    return (next_[n].load(std::memory_order_relaxed));
  }
  void NoBarrier_SetNext(int n, Node *x) {
    assert(n >= 0);
    next_[n].store(x, std::memory_order_relaxed);
  }

  void SetDup(DupList<V> *x) { dup_.store(x, std::memory_order_release); }
  DupList<V> *Dup() { return (dup_.load(std::memory_order_acquire)); }

  void NoBarrier_SetDup(DupList<V> *x) {
    dup_.store(x, std::memory_order_relaxed);
  }

private:
  std::atomic<DupList<V> *> dup_; // store duplicates if any

  // array of length equal to the node level + 1. next_[0] is lowest level link.
  std::atomic<Node *> next_[1];
};

template <typename K, typename V> class SkipList {
public:
  SkipList()
      : allow_dup_(false), head_(nullptr), tail_(nullptr), rnd_(0x12345678),
        alloc_bytes_(0) {
    K tail_key = std::numeric_limits<K>::max();
    _CreateList(tail_key);
  }

  ~SkipList() { _FreeList(); }

  // 声明成Node<K,V>而不是Node
  Node<K, V> *Search(const K key) const;

  Node<K, V> *FindGreaterOrEqual(const K key) const;
  Node<K, V> *FindLessOrEqual(const K key) const;

  bool Insert(K key, V value);
  bool Remove(K key, V &value);

  int Size() const {
    return (size_.load(std::memory_order_relaxed)); //
  }
  int Level() const {
    return (level_.load(std::memory_order_relaxed)); //
  }

  // 第一个有值的节点
  Node<K, V> *First() const { return head_->Next(0); }
  // 最后一个有值的节点，tail_前面一个
  Node<K, V> *Last() const { return head_->Next(kMaxHeight_); }

  void Reset() {
    _FreeList();

    K tail_key = std::numeric_limits<K>::max();
    _CreateList(tail_key);
  }

  void SetAllowDup(bool value) { allow_dup_ = value; }

  void Output(const std::string &tag) {
    std::cout << "Num of nodes: " << Size() << ", level: " << Level()
              << ", memory usage: " << MemoryUsage() << " bytes.\n";
    std::cout << "---------- only list the first 10 nodes -----------\n";
    _OutputAllNodes(tag, 10);
  }

  size_t MemoryUsage() const { return alloc_bytes_; }

private:
  void _CreateList(K tail_key);
  void _FreeList();

  Node<K, V> *_CreateNode(int level, K key, V value);
  Node<K, V> *_CreateNode(int level) {
    return _CreateNode(level, 0, 0); // k & v always 0, it's ok
  }

  int _RandomLevel(); // 随机生成一个level

  DupList<V> *_CreateDupList(V value);
  DupNode<V> *_CreateDupNode(V value);

private:
  // No copying allowed
  SkipList(const SkipList &);
  void operator=(const SkipList &);

private:
  // TODO use Comparator
  // Comparator _compare;
  // bool Equal(const K &a, const K &b) const { return (_compare(a, b) == 0); }
  // bool LessThan(const K &a, const K &b) const { return (_compare(a, b) < 0);
  // }

private:
  static const int kMaxHeight_ = 16;

  bool allow_dup_; // 是否允许Insert重复的key

  Node<K, V> *head_;
  Node<K, V> *tail_;

  std::atomic<int> level_; // Height of the entrie list
  std::atomic<size_t> size_;

  Random rnd_;

private:
  // TODO use memory allocator，统计内存使用等
  std::vector<char *> mems_;
  size_t alloc_bytes_;

  char *_Allocate(size_t bytes) {
    char *mem = static_cast<char *>(malloc(bytes));
    assert(mem != nullptr);
    mems_.emplace_back(mem);
    alloc_bytes_ += bytes;
    return mem;
  }

private:
  void _OutputAllNodes(const std::string &tag, int num = -1);
  void _OutputNode(Node<K, V> *node);
};

template <typename K, typename V> void SkipList<K, V>::_CreateList(K tail_key) {
  tail_ = _CreateNode(0);

  tail_->key = tail_key;
  tail_->SetNext(0, nullptr);

  // 设置头结
  head_ = _CreateNode(kMaxHeight_);

  for (int i = 0; i < kMaxHeight_ + 1; ++i) {
    head_->SetNext(i, tail_);
  }

  level_ = 0;
  size_ = 0;
}

template <typename K, typename V>
Node<K, V> *SkipList<K, V>::_CreateNode(int level, K key, V value) {
  char *mem = _Allocate(sizeof(Node<K, V>) +
                        (1 + level) * sizeof(std::atomic<Node<K, V> *>));
  return new (mem) Node<K, V>(level, key, value);
}

template <typename K, typename V>
DupList<V> *SkipList<K, V>::_CreateDupList(V value) {
  char *mem = _Allocate(sizeof(DupList<V>));
  DupList<V> *lst = new (mem) DupList<V>();

  // set head & tail of dup list
  lst->head = _CreateDupNode(value);
  lst->SetTail(lst->head);
  return lst;
}

template <typename K, typename V>
DupNode<V> *SkipList<K, V>::_CreateDupNode(V value) {
  char *mem = _Allocate(sizeof(DupNode<V>));
  return new (mem) DupNode<V>(value);
}

template <typename K, typename V> void SkipList<K, V>::_FreeList() {
  for (auto &x : mems_) {
    free(x);
  }
  mems_.clear();
}

template <typename K, typename V> int SkipList<K, V>::_RandomLevel() {
  int level = static_cast<int>(rnd_.Uniform(kMaxHeight_));
  return (level == 0) ? 1 : level;
}

template <typename K, typename V>
Node<K, V> *SkipList<K, V>::Search(const K key) const {
  Node<K, V> *node = head_;
  for (int i = Level(); i >= 0; --i) {
    Node<K, V> *next = node->Next(i);

    while (next->key < key) {
      node = next;
      next = node->Next(i);
    }
  }

  node = node->Next(0);
  if (node->key == key) {
    return node;
  } else {
    return nullptr;
  }
}

template <typename K, typename V>
Node<K, V> *SkipList<K, V>::FindLessOrEqual(const K key) const {
  Node<K, V> *node = head_;
  for (int i = Level(); i >= 0; --i) {
    Node<K, V> *next = node->Next(i);

    while (next->key < key) {
      node = next;
      next = node->Next(i);
    }
  }

  Node<K, V> *prev = node; // 最后一个小于key的node

  node = node->Next(0);
  if (node->key == key) {
    return node;
  } else {
    return prev;
  }
}

template <typename K, typename V>
Node<K, V> *SkipList<K, V>::FindGreaterOrEqual(const K key) const {
  Node<K, V> *node = head_;
  for (int i = Level(); i >= 0; --i) {
    Node<K, V> *next = node->Next(i);

    while (next->key < key) {
      node = next;
      next = node->Next(i);
    }
  }

  node = node->Next(0);
  if (node->key == key) {
    return node;
  } else {
    return node;
  }
}

template <typename K, typename V> bool SkipList<K, V>::Insert(K key, V value) {
  Node<K, V> *update[kMaxHeight_];

  Node<K, V> *node = head_;

  for (int i = Level(); i >= 0; --i) {
    Node<K, V> *next = node->Next(i);

    while (next->key < key) {
      node = next;
      next = node->Next(i);
    }

    update[i] = node;
  }

  // 首个结点插入时，node->Next(0)其实就是tail
  node = node->Next(0);

  // 如果key已存在，则直接返回false
  if (node->key == key) {
    if (not allow_dup_)
      return false;

    auto dup = node->Dup();
    if (dup) {
      if (not dup->Add(value)) {
        auto x = _CreateDupNode(value);
        dup->Insert(x);
      }
    } else {
      auto x = _CreateDupList(value);
      node->SetDup(x);
    }

    return true;
  }

  int nodeLevel = _RandomLevel();

  if (nodeLevel > Level()) {
    nodeLevel = ++level_;
    update[nodeLevel] = head_;
  }

  // 创建新结点
  Node<K, V> *x = _CreateNode(nodeLevel, key, value);

  // 初始化为无重复值的节点
  x->NoBarrier_SetDup(nullptr);

  // 调整next指针
  for (int i = nodeLevel; i >= 0; --i) {
    node = update[i];

    x->NoBarrier_SetNext(i, node->NoBarrier_Next(i));
    node->SetNext(i, x);
  }

  // 插入的是最后一个有效节点，更新last指针,
  // TODO no barrier ???
  if (x->Next(0) == tail_) {
    head_->SetNext(kMaxHeight_, x);
  }

  ++size_;

#ifdef DEBUG
  _OutputAllNodes(__func__);
#endif

  return true;
}

template <typename K, typename V> bool SkipList<K, V>::Remove(K key, V &value) {
  Node<K, V> *update[kMaxHeight_];
  Node<K, V> *node = head_;
  for (int i = Level(); i >= 0; --i) {
    Node<K, V> *next = node->Next(i);

    while (next->key < key) {
      node = next;
      next = node->Next(i);
    }

    update[i] = node;
  }

  node = node->Next(0);

  // 如果结点不存在就返回false
  if (node->key != key) {
    return false;
  }

  value = node->value;

  for (int i = 0; i <= Level(); ++i) {
    if (update[i]->Next(i) != node) {
      break;
    }
    update[i]->SetNext(i, node->Next(i));
  }

  // 释放结点
  node->~Node();

  // 更新level的值
  // 因为有可能在移除一个结点之后，level值会发生变化，
  // 及时移除可避免造成空间浪费
  while (level_ > 0 && head_->Next(level_) == tail_) {
    --level_;
  }

  --size_;

#ifdef DEBUG
  _OutputAllNodes(__func__);
#endif

  return true;
}

template <typename K, typename V>
void SkipList<K, V>::_OutputAllNodes(const std::string &tag, int num) {
  std::cout << tag << " => ";
  Node<K, V> *tmp = head_;

  Node<K, V> *next = tmp->Next(0);
  while (num-- && next != tail_) {
    tmp = next;
    next = tmp->Next(0);

    _OutputNode(tmp);
    std::cout << "\t-------------------------------------\n";
  }

  std::cout << "\n";
}

template <typename K, typename V>
void SkipList<K, V>::_OutputNode(Node<K, V> *node) {
  if (node == nullptr) {
    return;
  }

  std::cout << "\tnode->key:" << node->key << ",node->value:" << node->value
            << "\n";

  // 注意是i<=level而不是i<level
  for (int i = 0; i <= node->level; ++i) {
    Node<K, V> *tmp = node->Next(i);

    std::cout << "\t\tforward[" << i << "]:"
              << "key:" << tmp->key << ",value:" << tmp->value << "\n";
  }
}

} // namespace NI
} // namespace tig_gamma

#endif // SRC_SEARCHER_INDEX_NUMERIC_INDEX_H_
