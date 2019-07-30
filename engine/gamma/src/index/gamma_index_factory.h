#ifndef GAMMA_INDEX_FACTORY_H_
#define GAMMA_INDEX_FACTORY_H_

#include "gamma_index_ivfpq.h"
#include "gamma_index_ivfpq_gpu.h"
#include "pacins_index.h"
#include "raw_vector.h"
#include "utils.h"

#include "faiss/IndexFlat.h"

namespace tig_gamma {

class GammaIndexFactory {
public:
  static GammaIndex *Create(RetrievalModel model, size_t dimension,
                            const char *docids_bitmap, RawVector *raw_vec,
                            int nprobe) {
    if (docids_bitmap == nullptr) {
      LOG(ERROR) << "docids_bitmap is NULL!";
      return nullptr;
    }
    switch (model) {
    case IVFPQ: {
      faiss::IndexFlatL2 *coarse_quantizer = new faiss::IndexFlatL2(dimension);
      int ncentroids = 256;
      return (GammaIndex *)new GammaIVFPQIndex(coarse_quantizer, dimension,
                                               ncentroids, 32, 8, docids_bitmap,
                                               raw_vec, nprobe);
      break;
    }
    case GPU_IVFPQ: {
      faiss::IndexFlatL2 *coarse_quantizer = new faiss::IndexFlatL2(dimension);
      int ncentroids = 256;
      GammaIVFPQIndex *cpu_index =
          new GammaIVFPQIndex(coarse_quantizer, dimension, ncentroids, 32, 8,
                              docids_bitmap, raw_vec, nprobe);
      GammaIVFPQGPUIndex *gpu_index =
          new GammaIVFPQGPUIndex(cpu_index, dimension, docids_bitmap,
                                    raw_vec);
      return (GammaIndex *)gpu_index;
      break;
    }

    case SPTAG: {
      ;
      break;
    }
      // return (RawVector *)new MemoryDiskRawVector(name, dimension, 100000,
      //                                            100000 * 2, max_doc_size);

    case PACINS: {
      return (new pacins::PacinsIndex(dimension, docids_bitmap, raw_vec));
    }
    default: {
      throw std::invalid_argument("invalid raw feature type");
      break;
    }
    }

    return nullptr;
  }
};
} // namespace tig_gamma

#endif // GAMMA_INDEX_FACTORY_H_
