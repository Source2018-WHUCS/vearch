/**
 * Copyright(C) JD.COM, all rights reserved.
 * Author: Chen Jianyu (chenjianyu@jd.com)
 * Complie with:
 *   g++ -std=c++11 -fPIC -m64 -Wall -O3 -msse4 -mpopcnt -fopenmp
 * -Wno-sign-compare test.cpp Heap.cpp
 */
#ifndef SRC_INDEX_PACINS_INDEX_H_
#define SRC_INDEX_PACINS_INDEX_H_

#include <fcntl.h>
#include <math.h>
#include <omp.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/time.h>
#include <sys/types.h>
#include <unistd.h>

#include "faiss/Heap.h"
#include "faiss/utils.h"

#include <limits>
#include <map>
#include <string>
#include <vector>

#include <glog/logging.h>

#pragma GCC diagnostic ignored "-Wunused-variable"

#include "gamma_index.h"

using std::map;
using std::string;
using std::vector;

using faiss::float_minheap_array_t;
using faiss::minheap_heapify;
using faiss::minheap_pop;
using faiss::minheap_push;
using faiss::minheap_reorder;

namespace tig_gamma {
namespace pacins {

constexpr int NUM_THREADS = 15; // 检索线程数

using keep_func_t = std::function<bool(int)>;

//==============================================================
// utility
//==============================================================
static inline __m256 masked_read256(int d, const float *x) {
  assert(0 <= d && d < 8);
  __attribute__((__aligned__(16))) float buf[8] = {0, 0, 0, 0, 0, 0, 0, 0};
  switch (d) {
  case 7:
    buf[6] = x[6];
  case 6:
    buf[5] = x[5];
  case 5:
    buf[4] = x[4];
  case 4:
    buf[3] = x[3];
  case 3:
    buf[2] = x[2];
  case 2:
    buf[1] = x[1];
  case 1:
    buf[0] = x[0];
  }
  return _mm256_load_ps(buf);
}

float fvec_inner_product_avx2(const float *x, const float *y, size_t d) {
  __m256 mx, my;
  __m256 msum1 = _mm256_setzero_ps();

  while (d >= 8) {
    mx = _mm256_loadu_ps(x);
    x += 8;
    my = _mm256_loadu_ps(y);
    y += 8;
    msum1 = _mm256_add_ps(msum1, _mm256_mul_ps(mx, my));
    d -= 8;
  }

  if (d != 0) {
    // add the last 1~7 values
    mx = masked_read256(d, x);
    my = masked_read256(d, y);
    __m256 prod = _mm256_mul_ps(mx, my);

    msum1 = _mm256_add_ps(msum1, prod);
  }

  msum1 = _mm256_hadd_ps(msum1, msum1);
  msum1 = _mm256_add_ps(msum1, _mm256_permute2f128_ps(msum1, msum1, 0x1));
  msum1 = _mm256_hadd_ps(msum1, msum1);
  return _mm_cvtss_f32(_mm256_castps256_ps128(msum1));
}

//==============================================================
// 核心计算代码
//==============================================================
typedef long idx_t;

float fvec_inner_product_by_idx(
    const float *x,                   // 向量x
    const float *y,                   // 向量y
    const std::vector<uint16_t> &ids) // 需要计算内积的下标
{

  float dot = 0;

  // for (size_t i = 0; i < ids.size(); ++i) {
  //  int j = ids[i];
  //  dot += x[j] * y[j];
  //}

  // BLOCK: optimized code of the above
  if (not ids.empty()) {
#define INNER_PRODUCT                                                          \
  do {                                                                         \
    int j = ids[i];                                                            \
    dot += x[j] * y[j];                                                        \
    i++;                                                                       \
  } while (0)

    register int count = static_cast<int>(ids.size());
    register int n = (count + 7) / 8;
    register int i = 0;

    switch (count % 8) {
    case 0:
      do {
        INNER_PRODUCT;
      case 7:
        INNER_PRODUCT;
      case 6:
        INNER_PRODUCT;
      case 5:
        INNER_PRODUCT;
      case 4:
        INNER_PRODUCT;
      case 3:
        INNER_PRODUCT;
      case 2:
        INNER_PRODUCT;
      case 1:
        INNER_PRODUCT;
      } while (--n > 0);
    }
#undef INNER_PRODUCT
  }

  return dot;
}

void _knn_cosine(const float *x,             // 需要查询的向量
                 const float *y,             // 索引库
                 size_t d,                   // 向量维度
                 size_t nx,                  // 需要查询向量的个数
                 size_t ny,                  // 索引库的向量总数
                 float_minheap_array_t *res, // 用于存放返回结果的堆
                 int offset,                 // docid起始偏移
                 keep_func_t keep_func,      // 过滤函数
                 const std::vector<uint16_t> *idxs, // x中非0值的下标
                 int *ntotal)                       // 计算量
{
  size_t k = res->k;

#pragma omp parallel for
  for (size_t i = 0; i < nx; i++) {
    const float *x_ = x + i * d;
    const float *y_ = y;
    float *__restrict simi = res->get_val(i);
    long *__restrict idxi = res->get_ids(i);
    auto &idxs_ = idxs[i];

    ntotal[i] = 0;

    minheap_heapify(k, simi, idxi);
    for (size_t j = 0; j < ny; j++) {
      int docid = offset + j;
      if (not keep_func(docid)) {
        y_ += d;
        continue;
      }

      ntotal[i]++;

      float disij = fvec_inner_product_by_idx(x_, y_, idxs_);
      if (disij > simi[0]) {
        minheap_pop(k, simi, idxi);
        minheap_push(k, simi, idxi, disij, docid);
      }
      y_ += d;
    }
    minheap_reorder(k, simi, idxi);
  }
}

void __Search(idx_t nx, // 输入查询向量的个数(从x这个数组中取)
              const float *x,        // 查询向量数组
              idx_t k,               // 每个查询返回个数
              float *distances,      // 返回的距离数组(长度为n*k)
              idx_t *labels,         // 返回命中id数组(长度为n*k)
              size_t d,              // 向量维度
              const float *y,        // 索引库
              size_t ny,             // 索引库的向量总数
              int offset,            // docid起始偏移
              keep_func_t keep_func, // 过滤函数
              const std::vector<uint16_t> *idxs, // x中非0值的下标
              int *ntotal)                       // 计算量
{
  float_minheap_array_t res = {
      size_t(nx), // heap个数，每个heap对应一个查询的返回
      size_t(k),  // 每个heap(每个查询返回)大小
      labels, // 返回id数组，大小为nh*k，即查询个数*每个查询返回大小
      distances // 返回距离数组，大小为nh*k，即查询个数*每个查询返回大小
  };

  _knn_cosine(x, y, d, nx, ny, &res, offset, keep_func, idxs, ntotal);
}

struct IDScore_t {
  IDScore_t() : id(-1), score(0) {}

  idx_t id;
  float score;

  static bool Greater(const IDScore_t &x, const IDScore_t &y) {
    return x.score > y.score;
  }
};

void _Search(idx_t nx, // 输入查询向量的个数(从x这个数组中取)
             const float *x,        // 查询向量数组
             idx_t k,               // 每个查询返回个数
             float *distances,      // 返回的距离数组(长度为n*k)
             idx_t *labels,         // 返回命中id数组(长度为n*k)
             size_t d,              // 向量维度
             const float *y,        // 索引库
             size_t ny,             // 索引库的向量总数
             keep_func_t keep_func, // 过滤函数
             int *ntotal)           // 总计算量
{
  // 用于计算query向量和index向量中非0值的交集
  std::vector<uint16_t> idxs[nx];

  for (long i = 0; i < nx; ++i) {
    const float *x_ = x + i * d;
    for (size_t j = 0; j < d; ++j) {
      if (x_[j] > std::numeric_limits<float>::epsilon()) {
        idxs[i].push_back(j);
      }
    }
  }

  int all_ntotals[NUM_THREADS][nx];
  float all_distances[NUM_THREADS][nx * k];
  idx_t all_labels[NUM_THREADS][nx * k];

  size_t ny_per_thread = ny / NUM_THREADS;

  omp_set_num_threads(NUM_THREADS);

#pragma omp parallel for
  for (int i = 0; i < NUM_THREADS; ++i) {
    int *ntotal = all_ntotals[i];
    float *distances_ = all_distances[i];
    idx_t *labels_ = all_labels[i];

    const float *y_ = y + i * ny_per_thread * d;
    size_t ny_ = ny_per_thread;

    // 余数
    if (i == NUM_THREADS - 1) {
      ny_ = ny_per_thread + ny % NUM_THREADS;
    }

    int offset = i * ny_per_thread;
    __Search(nx, x, k, distances_, labels_, d, y_, ny_, offset, keep_func, idxs,
             ntotal);
  }

  // 合并、取topK
  for (long i = 0; i < nx; ++i) {
    std::vector<IDScore_t> tmp(NUM_THREADS * k);
    ntotal[i] = 0;

    for (long j = 0; j < NUM_THREADS; ++j) {
      float *distances_ = all_distances[j] + i * k;
      idx_t *labels_ = all_labels[j] + i * k;

      for (long k_ = 0; k_ < k; k_++) {
        tmp[j * k + k_].score = distances_[k_];
        tmp[j * k + k_].id = labels_[k_];
      }

      ntotal[i] += *(all_ntotals[j] + i);
    }

    std::stable_sort(tmp.begin(), tmp.end(), IDScore_t::Greater);

    // 取topK
    for (long k_ = 0; k_ < k; k_++) {
      distances[i * k + k_] = tmp[k_].score;
      labels[i * k + k_] = tmp[k_].id;
    }
  }
}

//==============================================================
// PacinsIndex
//==============================================================
class PacinsIndex : public GammaIndex {
public:
  PacinsIndex(size_t d, const char *docids_bitmap, RawVector *raw_vec)
      : GammaIndex(d, docids_bitmap, raw_vec) {}

public:
  int Indexing() override { return 0; }

  int AddRTVecsToIndex() override { return 0; }
  bool Add(int n, const float *vec) override { return true; }

public:
  int Search(const VectorQuery *query, const GammaSearchCondition *condition,
             VectorResult &result) override;

  // todo
  long GetTotalMemBytes() override { return 0; }

private:
  struct ResultItem {
    ResultItem() : doc_id(-1), sign(nullptr), sign_size(0), score(-1) {}

    size_t doc_id;
    char *sign;
    int sign_size;
    float score;
  };
};

int PacinsIndex::Search(const VectorQuery *query,
                        const GammaSearchCondition *condition,
                        VectorResult &result) {
  assert(query != nullptr);

  float *queries = reinterpret_cast<float *>(query->value->value);
  int n = query->value->len / (d_ * sizeof(float));
  int k = (condition->topn > 0) ? condition->topn : 10;

  int total[n];
  float distances[n * k];
  long labels[n * k];

  // filter
  auto keep_func = [this, condition](int doc) {
    bool is_del = bitmap::test(docids_bitmap_, doc);
    return (not is_del) &&
           (condition->numeric_results ? condition->numeric_results->Has(doc)
                                       : true);
  };

  _Search(n, queries, k, distances, labels, d_, raw_vec_->GetVector(0),
          raw_vec_->GetVectorNum(), keep_func, total);

  for (int i = 0; i < n; ++i) {
    ResultItem items[k]; // k items buffer
    int real_count = 0;  // used

    for (int j = 0; j < k; ++j) {
      int pos = j + i * k;

      int docid = labels[pos];
      if (docid < 0) {
        continue;
      }

      raw_vec_->GetSource(docid,
                          items[real_count].sign, // source
                          items[real_count].sign_size);

      items[real_count].doc_id = docid;
      items[real_count].score = distances[pos];
      real_count++;
    }

    // assign to result
    for (int j = 0; j < k; ++j) {
      int pos = j + i * k;

      result.docids[pos] = items[j].doc_id;
      result.sources[pos] = items[j].sign;
      result.source_lens[pos] = items[j].sign_size;
      result.dists[pos] = items[j].score;
    }

    result.total[i] = total[i];
  }

  return 0;
}

} // namespace pacins
} // namespace tig_gamma

#endif // SRC_INDEX_PACINS_INDEX_H_
