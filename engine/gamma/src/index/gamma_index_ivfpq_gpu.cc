#include "gamma_index_ivfpq_gpu.h"

#include "bitmap.h"
#include "faiss/Heap.h"
#include "faiss/gpu/GpuAutoTune.h"
#include "faiss/gpu/GpuIndexIVF.h"
#include "faiss/gpu/GpuIndexIVFPQ.h"
#include "faiss/gpu/StandardGpuResources.h"
#include "faiss/gpu/utils/DeviceUtils.h"
#include "faiss/utils.h"
#include <algorithm>
#include <vector>

using std::string;
using std::vector;

namespace tig_gamma {

int GammaIVFPQGPUIndex::Init() {
  int ngpus = faiss::gpu::getNumDevices();
  LOG(INFO) << "Number of GPUs: " << ngpus;

  // TODO check ngpus

  vector<faiss::gpu::GpuResources *> res;
  vector<int> devs;

  for (int i = 0; i < ngpus; i++) {
    res.push_back(new faiss::gpu::StandardGpuResources);
    devs.push_back(i);
  }

  faiss::gpu::GpuMultipleClonerOptions *options =
      new faiss::gpu::GpuMultipleClonerOptions();

  options->indicesOptions = faiss::gpu::INDICES_64_BIT;
  options->useFloat16CoarseQuantizer = false;
  options->useFloat16 = false;
  options->usePrecomputed = false;
  options->reserveVecs = 0;
  options->storeTransposed = false;
  options->verbose = true;

  // shard the index across GPUs
  options->shard = true;
  options->shard_type = 1;

  gpu_index_ = faiss::gpu::index_cpu_to_gpu_multiple(
      res, devs, dynamic_cast<faiss::Index *>(cpu_index_));

  // override
  if (auto *ix = dynamic_cast<faiss::gpu::GpuIndexIVF *>(gpu_index_)) {
    ix->setNumProbes(50);
  }

  if (auto *ix = dynamic_cast<faiss::gpu::GpuIndexIVFPQ *>(gpu_index_)) {
    ix->setPrecomputedCodes(true);
  }

  return 0;
}

int GammaIVFPQGPUIndex::Search(const VectorQuery *query,
                               const GammaSearchCondition *condition,
                               VectorResult &result) {
  assert(query != nullptr && condition != nullptr);

  if (gpu_index_ == nullptr) {
    Init();
  }

  // for now, GPU does NOT support IP, use cpu instead.
  // if (InnerProduct == condition->metric_type) {
  //   assert(cpu_index_ != nullptr);
  //   return cpu_index_->Search(query, condition, result);
  // } else {
  //   assert(gpu_index_ != nullptr);
  //   // TODO check
  // }

  float *xq = reinterpret_cast<float *>(query->value->value);
  int n = query->value->len / (d_ * sizeof(float));
  int k = condition->topn;

  _Search(n, xq, k, result.dists, result.docids, condition);

  for (int i = 0; i < n; ++i) {
    int pos = 0;

    std::set<int> uniq_docids;

    for (int j = 0; j < k; ++j) {
      long docid = result.docids[i * k + j];
      if (docid < 0) {
        continue;
      }

      int vid = static_cast<int>(docid);
      int real_docid = this->raw_vec_->vid2docid_[vid];

      if (uniq_docids.count(real_docid)) {
        continue;
      } else {
        uniq_docids.insert(real_docid);
      }

      int real_pos = i * k + pos;

      if (0 != raw_vec_->GetSource(vid, result.sources[real_pos],
                                   result.source_lens[real_pos])) {
        result.sources[real_pos] = nullptr;
        result.source_lens[real_pos] = 0;
      }

      result.docids[real_pos] = real_docid;
      result.dists[real_pos] = result.dists[i * k + j];

      pos++;
    }

    result.total[i] = pos; // ??

    if (pos > 0) {
      result.idx[i] = 0; // init start id of seeking
    }

    for (; pos < k; pos++) {
      result.docids[i * k + pos] = -1;
      result.dists[i * k + pos] = -1;
    }
  }

  return 0;
}

int GammaIVFPQGPUIndex::_Search(int n, const float *x, int k, float *distances,
                                long *labels,
                                const GammaSearchCondition *condition) {
  auto kk = condition->recall_num;
  assert(kk <= 1024);

  vector<float> D(n * kk);
  vector<long> I(n * kk);

  gpu_index_->search(n, x, kk, D.data(), I.data());

  using HeapForL2 = faiss::CMax<float, long>;
  using HeapForInner = faiss::CMin<float, long>;

#pragma omp parallel
  {
    // set filter
    auto is_filterable = [this, condition](long docid) -> bool {
      auto *num = condition->numeric_results;

      return bitmap::test(docids_bitmap_, docid) ||
             (num && not num->Has(docid));
    };

    bool check_dist = (condition->min_dist >= 0 && condition->max_dist >= 0);

#pragma omp for // parallelize over queries
    for (int i = 0; i < n; i++) {
      const float *xi = x + i * d_; // query

      float *simi = distances + i * k;
      long *idxi = labels + i * k;

      faiss::heap_heapify<HeapForInner>(k, simi, idxi); // intialize result heap

      for (int j = 0; j < kk; j++) {
        auto vid = I[i * kk + j];
        if (vid < 0) {
          continue;
        }

        auto docid = raw_vec_->vid2docid_[vid];
        if (is_filterable(docid)) {
          continue;
        }

        // auto dist = faiss::fvec_L2sqr(xi,
        //                               raw_vec_->GetVector(vid), // vector id
        //                               d_);
        auto dist = faiss::fvec_inner_product(xi,
                                      raw_vec_->GetVector(vid), // vector id
                                      d_);
        if (check_dist &&
            (dist < condition->min_dist || dist > condition->max_dist)) {
          continue;
        }

        // add vid to result heap
        if (HeapForInner::cmp(simi[0], dist)) {
          faiss::heap_pop<HeapForInner>(k, simi, idxi);
          faiss::heap_push<HeapForInner>(k, simi, idxi, dist, vid);
        }
      }

      if (condition->sort_by_docid) {
        std::vector<std::pair<long, float>> id_sim_pairs(k);
        for (int z = 0; z < k; z++) {
          id_sim_pairs[z] = std::make_pair(idxi[z], simi[z]);
        }
        std::sort(id_sim_pairs.begin(), id_sim_pairs.end());
        for (int z = 0; z < k; z++) {
          idxi[z] = id_sim_pairs[z].first;
          simi[z] = id_sim_pairs[z].second;
        }
      } else {
        faiss::heap_reorder<HeapForInner>(k, simi, idxi); // reorder result heap
      }
    }
  } // parallel

  return 0;
}

} // namespace tig_gamma
