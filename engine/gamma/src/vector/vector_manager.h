#ifndef VECTOR_MANAGER_H_
#define VECTOR_MANAGER_H_

#include <map>
#include <string>
#include <glog/logging.h>

#include "gamma_api.h"
#include "gamma_index.h"
#include "raw_vector.h"
#include "utils.h"

namespace tig_gamma {

class VectorManager {
public:
  VectorManager(const RetrievalModel &model, const RawVectorType &store_type,
                const char *docids_bitmap, int max_doc_size);

  int CreateVectorTable(VectorInfo **vectors_info, int vectors_num, int nprobe);

  int AddToStore(int docid, std::vector<Field *> &fields);

  int Indexing();

  int AddRTVecsToIndex();

  // int Add(int docid, const std::vector<Field *> &field_vecs);
  int Search(const GammaQuery &query, GammaResult *results);

  long GetTotalMemBytes() {
    long index_total_mem_bytes = 0;
    for (auto iter = vector_indexes_.begin(); iter != vector_indexes_.end(); iter++) {
      index_total_mem_bytes += iter->second->GetTotalMemBytes();
    }

    long vector_total_mem_bytes = 0;
    for (auto iter = raw_vectors_.begin(); iter != raw_vectors_.end(); ++iter) {
      vector_total_mem_bytes += iter->second->GetTotalMemBytes();
    }

    return index_total_mem_bytes + vector_total_mem_bytes;
  }

  int Dump(const std::string &path);
  int Load(const std::string &path);

private:
  RetrievalModel default_model_;
  RawVectorType default_store_type_;
  const char *docids_bitmap_;
  int max_doc_size_;
  bool table_created_;
  int nprobe;

  std::map<std::string, RawVector *> raw_vectors_;
  std::map<std::string, GammaIndex *> vector_indexes_;
};

} // namespace tig_gamma

#endif
