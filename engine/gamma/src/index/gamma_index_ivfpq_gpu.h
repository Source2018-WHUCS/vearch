/**
 * Copyright(C) JD.COM, all rights reserved.
 * Author: Chen Jianyu (chenjianyu@jd.com)
 */
#ifndef SRC_SEARCHER_INDEX_GAMMA_INDEX_IVFPQ_GPU_H_
#define SRC_SEARCHER_INDEX_GAMMA_INDEX_IVFPQ_GPU_H_

#include "faiss/Index.h"
#include "faiss/gpu/GpuIndex.h"
#include "gamma_index.h"
#include "numeric_index.h"
#include "raw_vector.h"
#include "utils.h"
#include <glog/logging.h>

namespace tig_gamma {

struct GammaIVFPQGPUIndex : GammaIndex {
  GammaIVFPQGPUIndex(GammaIndex *cpu_index, size_t d, const char *docids_bitmap,
                     RawVector *raw_vec)
      : GammaIndex(d, docids_bitmap, raw_vec), cpu_index_(cpu_index),
        gpu_index_(nullptr) {}
  ~GammaIVFPQGPUIndex(){};

  int Init();
  int Indexing() override { return cpu_index_ ? cpu_index_->Indexing() : -1; }

  int AddRTVecsToIndex() override {
    return cpu_index_ ? cpu_index_->AddRTVecsToIndex() : -1;
  }

  bool Add(int n, const float *vec) override {
    return cpu_index_ ? cpu_index_->Add(n, vec) : false;
  }

  int Search(const VectorQuery *query, const GammaSearchCondition *condition,
             VectorResult &result) override;

  long GetTotalMemBytes() override {
    return cpu_index_ ? cpu_index_->GetTotalMemBytes() : 0;
  }

  int _Search(int n, const float *x, int k, float *distances, long *labels,
              const GammaSearchCondition *);

  GammaIndex *cpu_index_;
  faiss::Index *gpu_index_;
};

} // namespace tig_gamma

#endif // SRC_SEARCHER_INDEX_GAMMA_INDEX_IVFPQ_GPU_H_
