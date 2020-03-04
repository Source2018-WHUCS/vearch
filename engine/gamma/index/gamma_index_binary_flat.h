/**
 * Copyright 2019 The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

#pragma once

#include "faiss/Index.h"
#include "gamma_index.h"
#include "log.h"
#include "raw_vector.h"
#include "utils.h"
#include <faiss/IndexBinaryFlat.h>

namespace tig_gamma {
namespace gamma_hamming {

class GammaHammingFlatIndex : public GammaIndex {
 public:
  GammaHammingFlatIndex(size_t d, const char *docids_bitmap,
                        RawVector *raw_vec);
  ~GammaHammingFlatIndex() {};

  int Indexing() override{};

  int AddRTVecsToIndex() override{};

  bool Add(int n, const float *vec) override{};

  bool Add(int n, const uint8_t *vec) override;

  int Search(const VectorQuery *query, GammaSearchCondition *condition,
             VectorResult &result) {
    return 0;
  };

  int BinarySearch(const GammaBinaryQuery &q, const VectorQuery *query,
                   GammaSearchCondition *condition,
                   VectorResult &result) override;

  ByteArray *GetBinaryVector(int vec_id) {
    ByteArray *arr = new ByteArray;
    std::vector<uint8_t> &data = data_[vec_id];
    arr->value = new char[data.size()];
    for (int i = 0; i < data.size(); ++i) {
      arr->value[i] = (char)data[i];
    }
    
    arr->len = data.size();
    return arr;
  }

  long GetTotalMemBytes() override {
    return 0;
  };

  int Dump(const std::string &dir, int max_vid) override{};
  int Load(const std::vector<std::string> &index_dirs) override{};

 private:
  std::vector<std::vector<uint8_t>> data_;
  int indexed_vec_count_;
  size_t nlist_;
  size_t M_;
  size_t nbits_per_idx_;
  int nprobe_;
  int search_idx_;

  faiss::IndexBinaryFlat *index_;

  int tmp_mem_num_;

  bool b_exited_;

  bool is_indexed_;
};

}  // namespace gamma_hamming
}  // namespace tig_gamma
