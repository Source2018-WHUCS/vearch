#ifndef UTILS_H_
#define UTILS_H_

#include "gamma_api.h"
#include "numeric_index.h"
#include <cassert>
#include <functional>
#include <string>
#include <vector>

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
};

struct GammaQuery {
  GammaQuery() {
    vec_query = nullptr;
    vec_num = 0;
    condition = nullptr;
  }

  ~GammaQuery() {}
  VectorQuery **vec_query;
  int vec_num;
  GammaSearchCondition *condition;
};

struct VectorDocField {
  std::string name;
  double score;
  char *source;
  int source_len;
};

struct VectorDoc {
  VectorDoc() {}

  ~VectorDoc() {
    if (fields) {
      delete[] fields;
      fields = nullptr;
    }
  }

  bool init(std::string *vec_names, int vec_num) {
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
  // char *source;
  // int source_len;
  struct VectorDocField *fields;
  int fields_len;
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

namespace utils {

long get_file_size(const char *path);

std::vector<std::string> split(const std::string &p_str,
                               const std::string &p_separator);

int count_lines(const char *filename);

double elapsed();

double getmillisecs();

int isFolderExist(const char *path);

int remove_dir(const char *dir);

#ifdef _WIN32

inline char file_sepator() { return '\\'; }
#else

inline char file_sepator() { return '/'; }
#endif

using file_filter_type = std::function<bool(const char *, const char *)>;

std::vector<std::string> for_each_file(const std::string &dir_name,
                                       file_filter_type filter,
                                       bool sub = false);

std::vector<std::string> for_each_folder(const std::string &dir_name,
                                         file_filter_type filter,
                                         bool sub = false);

std::vector<std::string> ls(const std::string &dir_name, bool sub = false);

std::vector<std::string> ls_folder(const std::string &dir_name,
                                   bool sub = false);

ssize_t write_n(int fd, const char *buf, ssize_t nbyte, int retry);

ByteArray *string_to_bytearray(const std::string &str);

std::string float_array_to_string(float *data, int len);
std::string VectorQueryToString(VectorQuery *vector_query);
std::string RequestToString(const Request *request);

template <class T> inline T *NewArray(int len, const char *msg) {
  assert(len > 0);
  T *data = new (std::nothrow) T[len];
  if (data == nullptr) {
    throw std::runtime_error("new array error, " + std::string(msg));
  }
  return data;
}

typedef struct MEM_PACKED {
  char name[20];
  unsigned long total;
  char name2[20];
} MEM_OCCUPY;

typedef struct MEM_PACK {
  double total, used_rate;
} MEM_PACK;

MEM_PACK *get_memoccupy();

} // namespace utils

#endif /* UTILS_H_ */
