/**
 * Copyright (c) The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

#ifndef MEMORY_RAW_VECTOR_H_
#define MEMORY_RAW_VECTOR_H_

#include "raw_vector.h"

namespace tig_gamma {

class MemoryRawVector : public RawVector {
public:
  MemoryRawVector(const std::string &name, int dimension, int max_doc_size,
                  const std::string &root_path);
  virtual ~MemoryRawVector();

  int Init() override;
  const float *GetVector(long vid) const override;
  int AddToStore(float *v, int len) override;
  const float *GetVectorHeader(int start, int end) override;

  int Dump(const std::string &path, int dump_docid, int max_docid) override;
  int Load(const std::vector<std::string> &path) override;

private:
  float *vector_mem_; // vector memory
};

} // namespace tig_gamma
#endif /* MEMORY_RAW_VECTOR_H_ */
