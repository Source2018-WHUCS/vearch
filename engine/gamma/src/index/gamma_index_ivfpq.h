#ifndef GAMMA_INDEX_IVFPQ_H_
#define GAMMA_INDEX_IVFPQ_H_

#include "log.h"
#include "gamma_common_data.h"
#include "gamma_index.h"
#include "numeric_index.h"
#include "raw_vector.h"
#include "realtime_invert_index.h"

#include "faiss/AuxIndexStructures.h"
#include "faiss/FaissAssert.h"
#include "faiss/Heap.h"
#include "faiss/IndexIVF.h"
#include "faiss/IndexIVFPQ.h"
#include "faiss/InvertedLists.h"
#include "faiss/hamming.h"
#include "faiss/utils.h"

namespace tig_gamma {

/// statistics are robust to internal threading, but not if
/// IndexIVFPQ::search_preassigned is called by multiple threads
struct IndexIVFPQStats {
  size_t nrefine; // nb of refines (IVFPQR)

  size_t n_hamming_pass;
  // nb of passed Hamming distance tests (for polysemous)

  // timings measured with the CPU RTC
  // on all threads
  size_t search_cycles;
  size_t refine_cycles; // only for IVFPQR

  IndexIVFPQStats() { reset(); }
  void reset(){};
};

// global var that collects them all
extern IndexIVFPQStats indexIVFPQ_stats;

// namespace {

using idx_t = faiss::Index::idx_t;

static uint64_t get_cycles() {
#ifdef __x86_64__
  uint32_t high, low;
  asm volatile("rdtsc \n\t" : "=a"(low), "=d"(high));
  return ((uint64_t)high << 32) | (low);
#else
  return 0;
#endif
}

#define TIC t0 = get_cycles()
#define TOC get_cycles() - t0

/** QueryTables manages the various ways of searching an
 * IndexIVFPQ. The code contains a lot of branches, depending on:
 * - metric_type: are we computing L2 or Inner product similarity?
 * - by_residual: do we encode raw vectors or residuals?
 * - use_precomputed_table: are x_R|x_C tables precomputed?
 * - polysemous_ht: are we filtering with polysemous codes?
 */
struct QueryTables {

  /*****************************************************
   * General data from the IVFPQ
   *****************************************************/

  const faiss::IndexIVFPQ &ivfpq;
  const faiss::IVFSearchParameters *params;

  // copied from IndexIVFPQ for easier access
  int d;
  const faiss::ProductQuantizer &pq;
  faiss::MetricType metric_type;
  bool by_residual;
  int use_precomputed_table;
  int polysemous_ht;

  // pre-allocated data buffers
  float *sim_table, *sim_table_2;
  float *residual_vec, *decoded_vec;

  // single data buffer
  std::vector<float> mem;

  // for table pointers
  std::vector<const float *> sim_table_ptrs;

  explicit QueryTables(const faiss::IndexIVFPQ &ivfpq,
                       const faiss::IVFSearchParameters *params)
      : ivfpq(ivfpq), d(ivfpq.d), pq(ivfpq.pq), metric_type(ivfpq.metric_type),
        by_residual(ivfpq.by_residual),
        use_precomputed_table(ivfpq.use_precomputed_table) {
    mem.resize(pq.ksub * pq.M * 2 + d * 2);
    sim_table = mem.data();
    sim_table_2 = sim_table + pq.ksub * pq.M;
    residual_vec = sim_table_2 + pq.ksub * pq.M;
    decoded_vec = residual_vec + d;

    // for polysemous
    polysemous_ht = ivfpq.polysemous_ht;
    if (auto ivfpq_params =
            dynamic_cast<const faiss::IVFPQSearchParameters *>(params)) {
      polysemous_ht = ivfpq_params->polysemous_ht;
    }
    if (polysemous_ht != 0) {
      q_code.resize(pq.code_size);
    }
    init_list_cycles = 0;
    sim_table_ptrs.resize(pq.M);
  }

  /*****************************************************
   * What we do when query is known
   *****************************************************/

  // field specific to query
  const float *qi;

  // query-specific intialization
  void init_query(const float *qi) {
    this->qi = qi;
    if (metric_type == faiss::METRIC_INNER_PRODUCT)
      init_query_IP();
    else
      init_query_L2();
    if (!by_residual && polysemous_ht != 0)
      pq.compute_code(qi, q_code.data());
  }

  void init_query_IP() {
    // precompute some tables specific to the query qi
    pq.compute_inner_prod_table(qi, sim_table);
  }

  void init_query_L2() {
    if (!by_residual) {
      pq.compute_distance_table(qi, sim_table);
    } else if (use_precomputed_table) {
      pq.compute_inner_prod_table(qi, sim_table_2);
    }
  }

  /*****************************************************
   * When inverted list is known: prepare computations
   *****************************************************/

  // fields specific to list
  long key;
  float coarse_dis;
  std::vector<uint8_t> q_code;

  uint64_t init_list_cycles;

  /// once we know the query and the centroid, we can prepare the
  /// sim_table that will be used for accumulation
  /// and dis0, the initial value
  float precompute_list_tables() {
    float dis0 = 0;
    uint64_t t0;
    TIC;
    if (by_residual) {
      if (metric_type == faiss::METRIC_INNER_PRODUCT)
        dis0 = precompute_list_tables_IP();
      else
        dis0 = precompute_list_tables_L2();
    }
    init_list_cycles += TOC;
    return dis0;
  }

  float precompute_list_table_pointers() {
    float dis0 = 0;
    uint64_t t0;
    TIC;
    if (by_residual) {
      if (metric_type == faiss::METRIC_INNER_PRODUCT)
        FAISS_THROW_MSG("not implemented");
      else
        dis0 = precompute_list_table_pointers_L2();
    }
    init_list_cycles += TOC;
    return dis0;
  }

  /*****************************************************
   * compute tables for inner prod
   *****************************************************/

  float precompute_list_tables_IP() {
    // prepare the sim_table that will be used for accumulation
    // and dis0, the initial value
    ivfpq.quantizer->reconstruct(key, decoded_vec);
    // decoded_vec = centroid
    float dis0 = faiss::fvec_inner_product(qi, decoded_vec, d);

    if (polysemous_ht) {
      for (int i = 0; i < d; i++) {
        residual_vec[i] = qi[i] - decoded_vec[i];
      }
      pq.compute_code(residual_vec, q_code.data());
    }
    return dis0;
  }

  /*****************************************************
   * compute tables for L2 distance
   *****************************************************/

  float precompute_list_tables_L2() {
    float dis0 = 0;

    if (use_precomputed_table == 0 || use_precomputed_table == -1) {
      ivfpq.quantizer->compute_residual(qi, residual_vec, key);
      pq.compute_distance_table(residual_vec, sim_table);

      if (polysemous_ht != 0) {
        pq.compute_code(residual_vec, q_code.data());
      }

    } else if (use_precomputed_table == 1) {
      dis0 = coarse_dis;

      faiss::fvec_madd(pq.M * pq.ksub,
                       &ivfpq.precomputed_table[key * pq.ksub * pq.M], -2.0,
                       sim_table_2, sim_table);

      if (polysemous_ht != 0) {
        ivfpq.quantizer->compute_residual(qi, residual_vec, key);
        pq.compute_code(residual_vec, q_code.data());
      }

    } else if (use_precomputed_table == 2) {
      dis0 = coarse_dis;

      const faiss::MultiIndexQuantizer *miq =
          dynamic_cast<const faiss::MultiIndexQuantizer *>(ivfpq.quantizer);
      FAISS_THROW_IF_NOT(miq);
      const faiss::ProductQuantizer &cpq = miq->pq;
      int Mf = pq.M / cpq.M;

      const float *qtab = sim_table_2; // query-specific table
      float *ltab = sim_table;         // (output) list-specific table

      long k = key;
      for (size_t cm = 0; cm < cpq.M; cm++) {
        // compute PQ index
        int ki = k & ((uint64_t(1) << cpq.nbits) - 1);
        k >>= cpq.nbits;

        // get corresponding table
        const float *pc =
            &ivfpq.precomputed_table[(ki * pq.M + cm * Mf) * pq.ksub];

        if (polysemous_ht == 0) {

          // sum up with query-specific table
          faiss::fvec_madd(Mf * pq.ksub, pc, -2.0, qtab, ltab);
          ltab += Mf * pq.ksub;
          qtab += Mf * pq.ksub;
        } else {
          for (size_t m = cm * Mf; m < (cm + 1) * Mf; m++) {
            q_code[m] =
                faiss::fvec_madd_and_argmin(pq.ksub, pc, -2, qtab, ltab);
            pc += pq.ksub;
            ltab += pq.ksub;
            qtab += pq.ksub;
          }
        }
      }
    }

    return dis0;
  }

  float precompute_list_table_pointers_L2() {
    float dis0 = 0;

    if (use_precomputed_table == 1) {
      dis0 = coarse_dis;

      const float *s = &ivfpq.precomputed_table[key * pq.ksub * pq.M];
      for (size_t m = 0; m < pq.M; m++) {
        sim_table_ptrs[m] = s;
        s += pq.ksub;
      }
    } else if (use_precomputed_table == 2) {
      dis0 = coarse_dis;

      const faiss::MultiIndexQuantizer *miq =
          dynamic_cast<const faiss::MultiIndexQuantizer *>(ivfpq.quantizer);
      FAISS_THROW_IF_NOT(miq);
      const faiss::ProductQuantizer &cpq = miq->pq;
      int Mf = pq.M / cpq.M;

      long k = key;
      int m0 = 0;
      for (size_t cm = 0; cm < cpq.M; cm++) {
        int ki = k & ((uint64_t(1) << cpq.nbits) - 1);
        k >>= cpq.nbits;

        const float *pc =
            &ivfpq.precomputed_table[(ki * pq.M + cm * Mf) * pq.ksub];

        for (int m = m0; m < m0 + Mf; m++) {
          sim_table_ptrs[m] = pc;
          pc += pq.ksub;
        }
        m0 += Mf;
      }
    } else {
      FAISS_THROW_MSG("need precomputed tables");
    }

    if (polysemous_ht) {
      FAISS_THROW_MSG("not implemented");
      // Not clear that it makes sense to implemente this,
      // because it costs M * ksub, which is what we wanted to
      // avoid with the tables pointers.
    }

    return dis0;
  }
};

/*****************************************************
 * Scaning the codes.
 * The scanning functions call their favorite precompute_*
 * function to precompute the tables they need.
 *****************************************************/
template <typename IDType, bool store_pairs, class C,
          faiss::MetricType METRIC_TYPE>
struct IVFPQScannerT : QueryTables {

  const uint8_t *list_codes;
  const IDType *list_ids;
  size_t list_size;

  explicit IVFPQScannerT(const faiss::IndexIVFPQ &ivfpq,
                         const faiss::IVFSearchParameters *params)
      : QueryTables(ivfpq, params) {
    FAISS_THROW_IF_NOT(pq.nbits == 8);
    assert(METRIC_TYPE == metric_type);
  }

  float dis0;

  void init_list(long list_no, float coarse_dis, int mode) {
    this->key = list_no;
    this->coarse_dis = coarse_dis;

    if (mode == 2) {
      dis0 = precompute_list_tables();
    } else if (mode == 1) {
      dis0 = precompute_list_table_pointers();
    }
  }

  /// tables are not precomputed, but pointers are provided to the
  /// relevant X_c|x_r tables
  size_t scan_list_with_pointer(size_t ncode, const uint8_t *codes,
                                const long *ids, size_t k, float *heap_sim,
                                long *heap_ids) const {
    size_t nup = 0;
    for (size_t j = 0; j < ncode; j++) {

      float dis = dis0;
      const float *tab = sim_table_2;

      for (size_t m = 0; m < pq.M; m++) {
        int ci = *codes++;
        dis += sim_table_ptrs[m][ci] - 2 * tab[ci];
        tab += pq.ksub;
      }

      if (C::cmp(heap_sim[0], dis)) {
        faiss::heap_pop<C>(k, heap_sim, heap_ids);
        long id = store_pairs ? (key << 32 | j) : ids[j];
        faiss::heap_push<C>(k, heap_sim, heap_ids, dis, id);
        nup++;
      }
    }
    return nup;
  }

  /// nothing is precomputed: access residuals on-the-fly
  size_t scan_on_the_fly_dist(size_t ncode, const uint8_t *codes,
                              const long *ids, size_t k, float *heap_sim,
                              long *heap_ids) const {
    const float *dvec;
    float dis0 = 0;
    size_t nup = 0;
    if (by_residual) {
      if (METRIC_TYPE == faiss::METRIC_INNER_PRODUCT) {
        ivfpq.quantizer->reconstruct(key, residual_vec);
        dis0 = faiss::fvec_inner_product(residual_vec, qi, d);
      } else {
        ivfpq.quantizer->compute_residual(qi, residual_vec, key);
      }
      dvec = residual_vec;
    } else {
      dvec = qi;
      dis0 = 0;
    }

    for (size_t j = 0; j < ncode; j++) {

      pq.decode(codes, decoded_vec);
      codes += pq.code_size;

      float dis;
      if (METRIC_TYPE == faiss::METRIC_INNER_PRODUCT) {
        dis = dis0 + faiss::fvec_inner_product(decoded_vec, qi, d);
      } else {
        dis = faiss::fvec_L2sqr(decoded_vec, dvec, d);
      }

      if (C::cmp(heap_sim[0], dis)) {
        faiss::heap_pop<C>(k, heap_sim, heap_ids);
        long id = store_pairs ? (key << 32 | j) : ids[j];
        faiss::heap_push<C>(k, heap_sim, heap_ids, dis, id);
        nup++;
      }
    }
    return nup;
  }

  /*****************************************************
   * Scanning codes with polysemous filtering
   *****************************************************/

  template <class HammingComputer>
  size_t scan_list_polysemous_hc(size_t ncode, const uint8_t *codes,
                                 const idx_t *ids, size_t k, float *heap_sim,
                                 long *heap_ids) const {
    int ht = ivfpq.polysemous_ht;
    size_t n_hamming_pass = 0, nup = 0;

    int code_size = pq.code_size;

    HammingComputer hc(q_code.data(), code_size);

    for (size_t j = 0; j < ncode; j++) {
      const uint8_t *b_code = codes;
      int hd = hc.hamming(b_code);
      if (hd < ht) {
        n_hamming_pass++;

        float dis = dis0;
        const float *tab = sim_table;

        for (size_t m = 0; m < pq.M; m++) {
          dis += tab[*b_code++];
          tab += pq.ksub;
        }

        if (C::cmp(heap_sim[0], dis)) {
          faiss::heap_pop<C>(k, heap_sim, heap_ids);
          long id = store_pairs ? (key << 32 | j) : ids[j];
          faiss::heap_push<C>(k, heap_sim, heap_ids, dis, id);
          nup++;
        }
      }
      codes += code_size;
    }
#pragma omp critical
    { indexIVFPQ_stats.n_hamming_pass += n_hamming_pass; }
    return nup;
  }

  size_t scan_list_polysemous(size_t ncode, const uint8_t *codes,
                              const long *ids, size_t k, float *heap_sim,
                              long *heap_ids) const {
    switch (pq.code_size) {
#define HANDLE_CODE_SIZE(cs)                                                   \
  case cs:                                                                     \
    return scan_list_polysemous_hc<faiss::HammingComputer##cs>(                \
        ncode, codes, ids, k, heap_sim, heap_ids);                             \
    break
      HANDLE_CODE_SIZE(4);
      HANDLE_CODE_SIZE(8);
      HANDLE_CODE_SIZE(16);
      HANDLE_CODE_SIZE(20);
      HANDLE_CODE_SIZE(32);
      HANDLE_CODE_SIZE(64);
#undef HANDLE_CODE_SIZE
    default:
      if (pq.code_size % 8 == 0)
        return scan_list_polysemous_hc<faiss::HammingComputerM8>(
            ncode, codes, ids, k, heap_sim, heap_ids);
      else
        return scan_list_polysemous_hc<faiss::HammingComputerM4>(
            ncode, codes, ids, k, heap_sim, heap_ids);
      break;
    }
  }
};

template <faiss::MetricType METRIC_TYPE, bool store_pairs, class C,
          int precompute_mode>
struct GammaIndexScanner : IVFPQScannerT<idx_t, store_pairs, C, METRIC_TYPE>,
                           faiss::InvertedListScanner {

  GammaIndexScanner(const faiss::IndexIVFPQ &ivfpq)
      : IVFPQScannerT<idx_t, store_pairs, C, METRIC_TYPE>(ivfpq, nullptr) {
    docids_bitmap_ = nullptr;
    numeric_index_ptr_ = nullptr;
    raw_vec_ = nullptr;
  }

  size_t scan_list_with_table(size_t ncode, const uint8_t *codes,
                              const long *ids, size_t k, float *heap_sim,
                              long *heap_ids) const {
    assert(this->pq.M % 4 == 0);

    // set filter func
    std::function<bool(int)> is_filterable;

    if (numeric_index_ptr_ != nullptr) {
      is_filterable = [this](int doc_id) -> bool {
        return (bitmap::test(docids_bitmap_, doc_id) ||
                (not numeric_index_ptr_->Has(doc_id)));
      };
    } else {
      is_filterable = [this](int doc_id) -> bool {
        return (bitmap::test(docids_bitmap_, doc_id));
      };
    }

    // set compute distance func
    std::function<float(const uint8_t *)> calc_dis;

    if (this->pq.M % 4 == 0) {
      calc_dis = [this](const uint8_t *codes) -> float {
        float dis = this->dis0;
        const float *tab = this->sim_table;

        for (size_t m = 0; m < this->pq.M; m += 4) {
          dis += tab[*codes++], tab += this->pq.ksub;
          dis += tab[*codes++], tab += this->pq.ksub;
          dis += tab[*codes++], tab += this->pq.ksub;
          dis += tab[*codes++], tab += this->pq.ksub;
        }

        return dis;
      };
    } else {
      calc_dis = [this](const uint8_t *codes) -> float {
        float dis = this->dis0;
        const float *tab = this->sim_table;

        for (size_t m = 0; m < this->pq.M; m++) {
          dis += tab[*codes++], tab += this->pq.ksub;
        }

        return dis;
      };
    }

    int nup = 0;

#define HANDLE_ONE                                                             \
  do {                                                                         \
    int doc_id = raw_vec_->vid2docid_[ids[j]];                                 \
    if ((numeric_index_ptr_ != nullptr &&                                      \
         (not numeric_index_ptr_->Has(doc_id))) ||                             \
        bitmap::test(docids_bitmap_, doc_id)) {                                \
      codes += this->pq.M; /* increment pointer */                             \
      j++;                 /* increment j*/                                    \
      continue;                                                                \
    }                                                                          \
                                                                               \
    float dis = this->dis0;                                                    \
    const float *tab = this->sim_table;                                        \
    for (size_t m = 0; m < this->pq.M; m += 4) {                               \
      dis += tab[*codes++], tab += this->pq.ksub;                              \
      dis += tab[*codes++], tab += this->pq.ksub;                              \
      dis += tab[*codes++], tab += this->pq.ksub;                              \
      dis += tab[*codes++], tab += this->pq.ksub;                              \
    }                                                                          \
                                                                               \
    if (C::cmp(heap_sim[0], dis)) {                                            \
      faiss::heap_pop<C>(k, heap_sim, heap_ids);                               \
      long id = ids[j];                                                        \
      faiss::heap_push<C>(k, heap_sim, heap_ids, dis, id);                     \
      nup++;                                                                   \
    }                                                                          \
                                                                               \
    j++; /* increment j */                                                     \
  } while (0)
    size_t j = 0;
    size_t loops = ncode / 8;
    for (size_t i = 0; i < loops; i++) {
      HANDLE_ONE; // 1
      HANDLE_ONE; // 2
      HANDLE_ONE; // 3
      HANDLE_ONE; // 4
      HANDLE_ONE; // 5
      HANDLE_ONE; // 6
      HANDLE_ONE; // 7
      HANDLE_ONE; // 8
    }

    switch (ncode % 8) {
    case 7:
      HANDLE_ONE;
    case 6:
      HANDLE_ONE;
    case 5:
      HANDLE_ONE;
    case 4:
      HANDLE_ONE;
    case 3:
      HANDLE_ONE;
    case 2:
      HANDLE_ONE;
    case 1:
      HANDLE_ONE;
    }

    assert(j == ncode);

#undef HANDLE_ONE
    return nup;
  }

  size_t scan_list_with_table(size_t ncode, const uint8_t **codes,
                              const long *ids, size_t k, float *heap_sim,
                              long *heap_ids) const {
    int nup = 0;
    assert(this->pq.M % 4 == 0);

#define HANDLE_ONE                                                             \
  do {                                                                         \
    float dis = this->dis0;                                                    \
    const float *tab = this->sim_table;                                        \
    const uint8_t *code = codes[j];                                            \
    for (size_t m = 0; m < this->pq.M; m += 4) {                               \
      dis += tab[*code++], tab += this->pq.ksub;                               \
      dis += tab[*code++], tab += this->pq.ksub;                               \
      dis += tab[*code++], tab += this->pq.ksub;                               \
      dis += tab[*code++], tab += this->pq.ksub;                               \
    }                                                                          \
                                                                               \
    if (C::cmp(heap_sim[0], dis)) {                                            \
      faiss::heap_pop<C>(k, heap_sim, heap_ids);                               \
      long id = ids[j];                                                        \
      faiss::heap_push<C>(k, heap_sim, heap_ids, dis, id);                     \
      nup++;                                                                   \
    }                                                                          \
                                                                               \
    j++; /* increment j */                                                     \
  } while (0)

    size_t j = 0;
    size_t loops = ncode / 8;
    for (size_t i = 0; i < loops; i++) {
      HANDLE_ONE; // 1
      HANDLE_ONE; // 2
      HANDLE_ONE; // 3
      HANDLE_ONE; // 4
      HANDLE_ONE; // 5
      HANDLE_ONE; // 6
      HANDLE_ONE; // 7
      HANDLE_ONE; // 8
    }

    switch (ncode % 8) {
    case 7:
      HANDLE_ONE;
    case 6:
      HANDLE_ONE;
    case 5:
      HANDLE_ONE;
    case 4:
      HANDLE_ONE;
    case 3:
      HANDLE_ONE;
    case 2:
      HANDLE_ONE;
    case 1:
      HANDLE_ONE;
    }

    assert(j == ncode);

#undef HANDLE_ONE
    return nup;
  }

  inline void set_query(const float *query) override {
    this->init_query(query);
  }

  inline void set_search_condition(const GammaSearchCondition *condition) {
    this->numeric_index_ptr_ = condition->numeric_results;
  }

  inline void set_list(long list_no, float coarse_dis) override {
    this->init_list(list_no, coarse_dis, precompute_mode);
  }

  inline float distance_to_code(const uint8_t *code) const override {
    assert(precompute_mode == 2);
    float dis = this->dis0;
    const float *tab = this->sim_table;

    for (size_t m = 0; m < this->pq.M; m++) {
      dis += tab[*code++];
      tab += this->pq.ksub;
    }
    return dis;
  }

  inline size_t scan_codes(size_t ncode, const uint8_t *codes, const long *ids,
                           float *heap_sim, long *heap_ids,
                           size_t k) const override {
    if (this->polysemous_ht > 0) {
      assert(precompute_mode == 2);
      this->scan_list_polysemous(ncode, codes, ids, k, heap_sim, heap_ids);
    } else if (precompute_mode == 2) {
      this->scan_list_with_table(ncode, codes, ids, k, heap_sim, heap_ids);
    } else if (precompute_mode == 1) {
      this->scan_list_with_pointer(ncode, codes, ids, k, heap_sim, heap_ids);
    } else if (precompute_mode == 0) {
      this->scan_on_the_fly_dist(ncode, codes, ids, k, heap_sim, heap_ids);
    } else {
      FAISS_THROW_MSG("bad precomp mode");
    }
    return 0;
  }

  inline size_t scan_codes(size_t ncode, const uint8_t **codes, const long *ids,
                           float *heap_sim, long *heap_ids, size_t k) {
    if (precompute_mode == 2) {
      this->scan_list_with_table(ncode, codes, ids, k, heap_sim, heap_ids);
    } else {
      FAISS_THROW_MSG("bad precomp mode");
    }
    return 0;
  }

  inline void SetVecFilter(const char *docids_bitmap,
                           const RawVector *raw_vec) {
    if (docids_bitmap == nullptr) {
      LOG(ERROR) << "docids_bitmap is NULL!";
      return;
    }

    if (!docids_bitmap_) {
      docids_bitmap_ = docids_bitmap;
    }

    if (!raw_vec_) {
      raw_vec_ = raw_vec;
    }

    return;
  }

  const char *docids_bitmap_;
  NI::RangeQueryResult *numeric_index_ptr_;
  const RawVector *raw_vec_;
};

//} // anonymous namespace

struct RTInvertedLists : faiss::InvertedLists {

  using idx_t = int;

  RTInvertedLists(realtime::RTInvertIndex *rt_invert_index_ptr, size_t nlist,
                  size_t code_size);

  /*************************
   *  Read only functions */

  // get the size of a list
  size_t list_size(size_t list_no) const override;

  /** get the codes for an inverted list
   * must be released by release_codes
   *
   * @return codes    size list_size * code_size
   */
  const uint8_t *get_codes(size_t list_no) const override;

  /** get the ids for an inverted list
   * must be released by release_ids
   *
   * @return ids      size list_size
   */
  const long *get_ids(size_t list_no) const override;

  /*************************
   * writing functions     */

  size_t add_entries(size_t list_no, size_t n_entry, const long *ids,
                     const uint8_t *code) override;

  void resize(size_t list_no, size_t new_size) override;

  void update_entries(size_t list_no, size_t offset, size_t n_entry,
                      const long *ids_in, const uint8_t *codes_in) override;

  realtime::RTInvertIndex *rt_invert_index_ptr_;
};

struct GammaIVFPQIndex : GammaIndex, faiss::IndexIVFPQ {
  GammaIVFPQIndex(faiss::Index *quantizer, size_t d, size_t nlist, size_t M,
                  size_t nbits_per_idx, const char *docids_bitmap,
                  RawVector *raw_vec, int nprobe);

  faiss::InvertedListScanner *
  get_InvertedListScanner(bool store_pairs) const override;

  int Indexing() override;

  int AddRTVecsToIndex() override;

  bool Add(int n, const float *vec) override;

  int Search(const VectorQuery *query, const GammaSearchCondition *condition,
             VectorResult &result) override;

  void search_preassigned(int n, const float *x,
                          const GammaSearchCondition *condition,
                          const long *assign, const float *centroid_dis,
                          float *distances, long *labels, int *total,
                          bool store_pairs,
                          const faiss::IVFSearchParameters *params = nullptr);

  // assign the vectors, then call search_preassign
  void SearchIVFPQ(int n, const float *x, const GammaSearchCondition *condition,
                   float *distances, long *labels, int *total);

  void SearchDirectly(int n, const float *x,
                      const GammaSearchCondition *condition, float *distances,
                      long *labels, int *total);

  long GetTotalMemBytes() override {
    if (!rt_invert_index_ptr_) {
      return 0;
    }
    return rt_invert_index_ptr_->getTotalMemBytes();
  }

  int indexed_vec_count_;
  realtime::RTInvertIndex *rt_invert_index_ptr_;
};

} // namespace tig_gamma

#endif
