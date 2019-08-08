#ifndef GAMMA_COMMON_DATA_H_
#define GAMMA_COMMON_DATA_H_

#include "gamma_api.h"
#include "numeric_index.h"
#include "online_logger.h"

namespace tig_gamma {

const std::string EXTRA_VECTOR_FIELD_SOURCE = "source";
const std::string EXTRA_VECTOR_FIELD_SCORE = "score";
const std::string EXTRA_VECTOR_FIELD_NAME = "field";
const std::string EXTRA_VECTOR_RESULT = "vector_result";

const float GAMMA_INDEX_RECALL_RATIO = 1.0f;

enum class ResultCode : std::uint16_t {
#define DefineResultCode(Name, Value) Name = Value,
#include "definition_list.h"
#undef DefineResultCode
  Undefined
};

enum RawVectorType { MemoryOnly, MemoryWithDisk };
enum RetrievalModel { IVFPQ, GPU_IVFPQ, SPTAG, PACINS };

struct VectorDocField {
  std::string name;
  double score;
  char *source;
  int source_len;
};

struct VectorDoc {
  VectorDoc() {
    docid = -1;
    score = 0.0f;
  }

  ~VectorDoc() {
    if (fields) {
      delete[] fields;
      fields = nullptr;
    }
  }

  bool init(std::string *vec_names, int vec_num) {
    if (vec_num <= 0) {
      fields = nullptr;
      fields_len = 0;
      return true;
    }
    fields = new (std::nothrow) VectorDocField[vec_num];
    if (fields == nullptr) {
      return false;
    }
    for (int i = 0; i < vec_num; i++) {
      fields[i].name = vec_names[i];
    }
    fields_len = vec_num;
    return true;
  }

  int docid;
  double score;
  struct VectorDocField *fields;
  int fields_len;
};

struct GammaSearchCondition {
  GammaSearchCondition() {
    numeric_results = nullptr;
    topn = 0;
    has_rank = false;
    metric_type = InnerProduct;
    sort_by_docid = false;
    min_dist = -1;
    max_dist = -1;
    recall_num = 0;
    parallel_mode = 1; // default to parallelize over inverted list
    use_direct_search = false;
  }

  GammaSearchCondition(GammaSearchCondition *condition) {
    numeric_results = condition->numeric_results;
    topn = condition->topn;
    has_rank = condition->has_rank;
    metric_type = condition->metric_type;
    sort_by_docid = condition->sort_by_docid;
    min_dist = condition->min_dist;
    max_dist = condition->max_dist;
    recall_num = condition->recall_num;
    parallel_mode = condition->parallel_mode;
    use_direct_search = condition->use_direct_search;
  }

  ~GammaSearchCondition() {
    numeric_results = nullptr; // should not delete
  }

  NI::RangeQueryResult *numeric_results;

  int topn;
  bool has_rank;
  DistanceMetricType metric_type;
  bool sort_by_docid;
  float min_dist;
  float max_dist;
  int recall_num;
  int parallel_mode;
  bool use_direct_search;
};

struct GammaQuery {
  GammaQuery() {
    vec_query = nullptr;
    vec_num = 0;
    condition = nullptr;
    logger = nullptr;
  }

  ~GammaQuery() {}
  VectorQuery **vec_query;
  int vec_num;
  GammaSearchCondition *condition;
  OnlineLogger *logger;
};

struct GammaResult {
  GammaResult() {
    topn = 0;
    total = 0;
    results_count = 0;
    docs = nullptr;
  }
  ~GammaResult() {
    if (docs) {
      delete[] docs;
      docs = nullptr;
    }
  }

  bool init(int n, std::string *vec_names, int vec_num) {
    topn = n;
    docs = new (std::nothrow) VectorDoc[topn];
    if (!docs) {
      // LOG(ERROR) << "docs in CommonDocs init error!";
      return false;
    }
    for (int i = 0; i < n; i++) {
      if (!docs[i].init(vec_names, vec_num)) {
        return false;
      }
    }
    return true;
  }

  int topn;
  int total;
  int results_count;

  VectorDoc *docs;
};

} // namespace tig_gamma

#endif
