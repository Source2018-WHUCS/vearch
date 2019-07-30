#include "vector_manager.h"
#include "gamma_index_factory.h"
#include "raw_vector_factory.h"

namespace tig_gamma {

bool InnerProductCmp(const VectorDoc &a, const VectorDoc &b) {
  return a.score > b.score;
}

bool L2Cmp(const VectorDoc &a, const VectorDoc &b) { return a.score < b.score; }

VectorManager::VectorManager(const RetrievalModel &model,
                             const RawVectorType &store_type,
                             const char *docids_bitmap, int max_doc_size)
    : default_model_(model), default_store_type_(store_type),
      docids_bitmap_(docids_bitmap), max_doc_size_(max_doc_size) {
  table_created_ = false;
  nprobe = 0;
}

int VectorManager::CreateVectorTable(VectorInfo **vectors_info,
                                     int vectors_num, int nprobe) {
  if (table_created_)
    return -1;

  for (int i = 0; i < vectors_num; i++) {
    std::string vec_name(vectors_info[i]->name->value,
                         vectors_info[i]->name->len);
    int dimension = vectors_info[i]->dimension;

    std::string store_type_str(vectors_info[i]->store_type->value,
                               vectors_info[i]->store_type->len);

    RawVectorType store_type = default_store_type_;
    if (!strcasecmp("MemoryOnly", store_type_str.c_str())) {
      store_type = RawVectorType::MemoryOnly;
    } else if (!strcasecmp("MemoryWithDisk", store_type_str.c_str())) {
      store_type = RawVectorType::MemoryWithDisk;
    } else {
      LOG(WARNING) << "NO support for store type " << store_type_str
                   << ", default to " << default_store_type_;
    }

    RawVector *vec = RawVectorFactory::Create(store_type, vec_name, dimension,
                                              max_doc_size_);
    int ret = vec->Init();
    if (ret != 0) {
      LOG(ERROR) << "Raw vector " << vec_name << " init error, code [" << ret
                 << "]!";
      continue;
    }

    raw_vectors_[vec_name] = vec;

    std::string retrieval_type_str(vectors_info[i]->retrieval_type->value,
                                   vectors_info[i]->retrieval_type->len);

    RetrievalModel model = default_model_;
    if (!strcasecmp("IVFPQ", retrieval_type_str.c_str())) {
      model = RetrievalModel::IVFPQ;
    } else if (!strcasecmp("SPTAG", retrieval_type_str.c_str())) {
      model = RetrievalModel::SPTAG;
    } else if (!strcasecmp("PACINS", retrieval_type_str.c_str())) {
      model = RetrievalModel::PACINS;
    } else {
      LOG(WARNING) << "NO support for retrieval type " << retrieval_type_str
                   << ", default to " << default_model_;
    }

    GammaIndex *index = GammaIndexFactory::Create(model, dimension,
                                                  docids_bitmap_, vec, nprobe);
    if (index == nullptr) {
      LOG(ERROR) << "create gamma index " << vec_name << " error!";
      continue;
    }
    this->nprobe = nprobe;

    vector_indexes_[vec_name] = index;
  }
  table_created_ = true;
  return 0;
}

int VectorManager::AddToStore(int docid, std::vector<Field *> &fields) {
  for (unsigned int i = 0; i < fields.size(); i++) {
    std::string name =
        std::string(fields[i]->name->value, fields[i]->name->len);
    if (raw_vectors_.find(name) == raw_vectors_.end()) {
      LOG(ERROR) << "Cannot find raw vector [" << name << "]";
      continue;
    }
    raw_vectors_[name]->Add(docid, fields[i]);
  }
  return 0;
}

int VectorManager::Indexing() {
  int ret = 0;
  std::map<std::string, GammaIndex *>::iterator iter = vector_indexes_.begin();
  for (; iter != vector_indexes_.end(); iter++) {
    if (0 != iter->second->Indexing()) {
      ret = -1;
      LOG(ERROR) << "vector table " << iter->first << " indexing failed!";
    }
  }
  return ret;
}

int VectorManager::AddRTVecsToIndex() {
  int ret = 0;
  std::map<std::string, GammaIndex *>::iterator iter = vector_indexes_.begin();
  for (; iter != vector_indexes_.end(); iter++) {
    if (0 != iter->second->AddRTVecsToIndex()) {
      ret = -1;
      LOG(ERROR) << "vector table " << iter->first
                 << " add real time vectors failed!";
    }
  }
  return ret;
}

int VectorManager::Search(const GammaQuery &query, GammaResult *results) {
  int ret = 0, n = 0;

  VectorResult all_vector_results[query.vec_num];

  query.condition->sort_by_docid = query.vec_num > 1 ? true : false;
  std::string vec_names[query.vec_num];
  for (int i = 0; i < query.vec_num; i++) {
    std::string name = std::string(query.vec_query[i]->name->value,
                                   query.vec_query[i]->name->len);
    vec_names[i] = name;
    std::map<std::string, GammaIndex *>::iterator iter =
        vector_indexes_.find(name);
    if (iter == vector_indexes_.end()) {
      LOG(ERROR) << "Query name " << name
                 << " not exist in created vector table";
      continue;
    }

    n = query.vec_query[i]->value->len / (sizeof(float) * iter->second->d_);
    if (!all_vector_results[i].init(n, query.condition->topn)) {
      continue;
    }

    GammaSearchCondition condition(query.condition);
    condition.min_dist = query.vec_query[i]->min_score;
    condition.max_dist = query.vec_query[i]->max_score;
    int ret_vec = iter->second->Search(query.vec_query[i], &condition,
                                       all_vector_results[i]);
    if (ret_vec != 0) {
      ret = ret_vec;
    }
  }

  if (query.condition->sort_by_docid) {
    for (int i = 0; i < n; i++) {
      int start_docid = 0, common_docid_count = 0, common_idx = 0;
      double score = 0;
      bool has_common_docid = true;
      if (!results[i].init(query.condition->topn, vec_names, query.vec_num)) {
        continue;
      }
      while (start_docid < INT_MAX) {
        for (int j = 0; j < query.vec_num; j++) {
          float vec_dist = 0;
          char *source = nullptr;
          int source_len = 0;
          int cur_docid = all_vector_results[j].seek(i, start_docid, vec_dist,
                                                     source, source_len);
          if (cur_docid == start_docid) {
            common_docid_count++;
            double field_score = query.vec_query[j]->has_boost == 1
                                     ? (vec_dist * query.vec_query[j]->boost)
                                     : vec_dist;
            score += field_score;
            results[i].docs[common_idx].fields[j].score = field_score;
            results[i].docs[common_idx].fields[j].source = source;
            results[i].docs[common_idx].fields[j].source_len = source_len;
            if (common_docid_count == query.vec_num) {
              results[i].docs[common_idx].docid = start_docid;
              results[i].docs[common_idx++].score = score;
              int total = results[i].total;
              if (total > all_vector_results[j].total[i]) {
                results[i].total = all_vector_results[j].total[i];
              }

              start_docid++;
              common_docid_count = 0;
              score = 0;
            }
          } else if (cur_docid > start_docid) {
            common_docid_count = 0;
            start_docid = cur_docid;
            score = 0;
          } else {
            has_common_docid = false;
            break;
          }
        }
        if (!has_common_docid)
          break;
      }
      results[i].results_count = common_idx;
      if (query.condition->has_rank) {
        std::sort(results[i].docs, results[i].docs + common_idx,
                  InnerProductCmp);
      }
    }
  } else {

    for (int i = 0; i < n; i++) {
      // double score = 0;
      if (!results[i].init(query.condition->topn, vec_names, query.vec_num)) {
        continue;
      }
      results[i].total = all_vector_results[0].total[i];
      int pos = 0, topn = all_vector_results[0].topn;
      for (int j = 0; j < topn; j++) {
        int real_pos = i * topn + j;
        if (all_vector_results[0].docids[real_pos] == -1)
          continue;
        results[i].docs[pos].docid = all_vector_results[0].docids[real_pos];

        results[i].docs[pos].fields[0].source =
            all_vector_results[0].sources[real_pos];
        results[i].docs[pos].fields[0].source_len =
            all_vector_results[0].source_lens[real_pos];

        double score = all_vector_results[0].dists[real_pos];

        score = query.vec_query[0]->has_boost == 1
                    ? (score * query.vec_query[0]->boost)
                    : score;

        results[i].docs[pos].fields[0].score = score;
        results[i].docs[pos].score = score;
        pos++;
      }
      results[i].results_count = pos;
    }
  }

  return ret;
}

int VectorManager::Dump(const string &path) {
  std::map<std::string, RawVector *>::iterator iter = raw_vectors_.begin();
  for (; iter != raw_vectors_.end(); iter++) {
    if (0 != iter->second->Dump(path, this->nprobe)) {
      LOG(ERROR) << "vector table " << iter->first << " dump failed!";
      return -1;
    }
  }
  return 0;
}

int VectorManager::Load(const string &path) {
  if (raw_vectors_.size() > 0) {
    std::map<std::string, RawVector *>::iterator iter = raw_vectors_.begin();
    for (; iter != raw_vectors_.end(); iter++) {
      if (iter->second != nullptr) {
        delete iter->second;
      }
    }
  }

  if (vector_indexes_.size() > 0) {
    std::map<std::string, GammaIndex *>::iterator iter = vector_indexes_.begin();
    for (; iter != vector_indexes_.end(); iter++) {
      if (iter->second != nullptr) {
        delete iter->second;
      }
    }
  }

  const std::vector<string> files = utils::ls(path);
  for (const auto file : files) {
    auto strs = utils::split(file, ".");
    if (strs.size() == 2 && strs[1] == "fet") {
      string vec_name = strs[0];
      strs = utils::split(vec_name, "/");
      vec_name = strs[strs.size() -1];
      FILE *fet_fp = fopen(file.c_str(), "rb");
      if (fet_fp == nullptr) {
        LOG(ERROR) << "open error: feature file=" << file.c_str();
        return -1;
      }
      int dimension = 0;
      int type;
      int nprobe = 0;
      assert(1 == fread((void *)&nprobe, sizeof(nprobe), 1, fet_fp));
      assert(1 == fread((void *)&type, sizeof(type), 1, fet_fp));
      assert(1 == fread((void *)&dimension, sizeof(dimension), 1, fet_fp));
      fclose(fet_fp);

      RawVectorType vec_type = static_cast<RawVectorType>(type);
      RawVector *raw_vec = RawVectorFactory::Create(vec_type, vec_name,
                                                    dimension, max_doc_size_);
      if (raw_vec == nullptr) {
        LOG(ERROR) << "create raw vector error";
        return -1;
      }
      int ret = raw_vec->Load(path);
      if (ret != 0) {
        LOG(ERROR) << "load error: vector name=" << vec_name
                   << ", vector type=" << vec_type
                   << ", dimension=" << dimension << ", ret=" << ret;
        return -1;
      }
      if (nprobe <= 0 || (this->nprobe != 0 && this->nprobe != nprobe)) {
        LOG(ERROR) << "load error: invalid current nprobe=" << nprobe
                   << ", pre nprobe=" << this->nprobe;
        return -1;
      }
      if (this->nprobe == 0) {
        this->nprobe = nprobe;
      }
      raw_vectors_[vec_name] = raw_vec;

      RetrievalModel model = default_model_; // it should be retrieved from file
      GammaIndex *index =
        GammaIndexFactory::Create(model, dimension, docids_bitmap_, raw_vec, nprobe);
      if (index == nullptr) {
        LOG(ERROR) << "Load: create gamma index " << vec_name << " error!";
        return -1;
      }
      LOG(ERROR) << "load vector success: vector name=" << vec_name
                 << ", vector type=" << vec_type
                 << ", dimension=" << dimension << ", ret=" << ret
                 << ", nprobe=" << nprobe;
      vector_indexes_[vec_name] = index;
    }
  }
  return 0;
}

} // namespace tig_gamma
