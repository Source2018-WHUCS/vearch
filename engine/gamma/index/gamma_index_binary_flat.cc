#include "gamma_index_binary_flat.h"
#include <faiss/utils/hamming.h>

namespace tig_gamma {
namespace gamma_hamming {
GammaHammingFlatIndex::GammaHammingFlatIndex(size_t d,
                                             const char *docids_bitmap,
                                             RawVector *raw_vec)
    : GammaIndex(d, docids_bitmap, raw_vec) {}

bool GammaHammingFlatIndex::Add(int n, const uint8_t *vec) {
  std::vector<uint8_t> data(n);
  
  for (int i = 0; i < n; ++i) {
    data[i] = vec[i];
  }

  data_.emplace_back(std::move(data));
  return true;
}

int GammaHammingFlatIndex::BinarySearch(const GammaBinaryQuery &q,
                                        const VectorQuery *query,
                                        GammaSearchCondition *condition,
                                        VectorResult &result) {

  int k = 1;
  //int nq = q.vec_num;
  int nq  = q.n;
#pragma omp parallel for
  for (int i = 0; i < nq; ++i) {
    uint8_t *xa = reinterpret_cast<uint8_t *>(q.xa[i]->value);
    uint8_t *xb = reinterpret_cast<uint8_t *>(q.xb[i]->value);
    int distance = 0;
    long label = 0;
    int d = q.xa[i]->len;
    faiss::hammings_knn_mc(xa, xb, 1, 1, 1, d, &distance, &label);

    result.dists[i] = 1 - ((float)distance) / (2 * d);
    result.docids[i] = i;
    result.topn = nq;
    result.sources[i] = new char[10];
    result.source_lens[i] = 10;

  }
 
  return 0;
}
}
}
