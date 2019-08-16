/**
 * Copyright (c) The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

#include "gamma_engine.h"

#include "log.h"
#include <chrono>
#include <cstring>
#include <fcntl.h>
#include <mutex>
#include <sys/mman.h>
#include <thread>
#include <vector>

#include "bitmap.h"
#include "cJSON.h"
#include "gamma_common_data.h"
#include "utils.h"

using std::string;

namespace tig_gamma {

#ifdef DEBUG
static string float_array_to_string(float *data, int len) {
  if (data == nullptr)
    return "";
  std::stringstream ss;
  ss << "[";
  for (int i = 0; i < len; i++) {
    ss << data[i];
    if (i != len - 1) {
      ss << ",";
    }
  }
  ss << "]";
  return ss.str();
}

static string VectorQueryToString(VectorQuery *vector_query) {
  std::stringstream ss;
  ss << "name:"
     << std::string(vector_query->name->value, vector_query->name->len)
     << " min score:" << vector_query->min_score
     << " max score:" << vector_query->max_score
     << " boost:" << vector_query->boost
     << " has boost:" << vector_query->has_boost << " value:"
     << float_array_to_string((float *)vector_query->value->value,
                              vector_query->value->len / sizeof(float));
  return ss.str();
}

static string RequestToString(const Request *request) {
  std::stringstream ss;
  ss << "{req_num:" << request->req_num << " topn:" << request->topn
     << " has_rank:" << request->has_rank
     << " vec_num:" << request->vec_fields_num;
  for (int i = 0; i < request->vec_fields_num; i++) {
    ss << " vec_id:" << i << " [" << VectorQueryToString(request->vec_fields[i])
       << "]";
  }
  ss << "}";
  return ss.str();
}
#endif // DEBUG

GammaEngine::GammaEngine(const string &index_root_path)
    : index_root_path_(index_root_path) {
  char ch = index_root_path_.back();
  if (ch != '/') {
    index_root_path_ += "/";
  }

  docids_bitmap_ = nullptr;
  profile_ = nullptr;
  vec_manager_ = nullptr;
  numeric_index_ = nullptr;
  index_status_ = IndexStatus::UNINDEXED;
  delete_num = 0;
  b_running_ = false;
#ifdef PERFORMANCE_TESTING
  search_num_ = 0;
#endif
}

GammaEngine::~GammaEngine() {
  if (b_running_) {
    b_running_ = false;
    std::mutex running_mutex;
    std::unique_lock<std::mutex> lk(running_mutex);
    running_cv_.wait(lk);
  }

  if (vec_manager_) {
    delete vec_manager_;
    vec_manager_ = nullptr;
  }
  if (profile_) {
    delete profile_;
    profile_ = nullptr;
  }
  if (docids_bitmap_) {
    delete docids_bitmap_;
    docids_bitmap_ = nullptr;
  }
  if (numeric_index_) {
    delete numeric_index_;
    numeric_index_ = nullptr;
  }
}

GammaEngine *GammaEngine::GetInstance(const string &index_root_path,
                                      int max_doc_size) {
  GammaEngine *engine = new GammaEngine(index_root_path);
  int ret = engine->Setup(max_doc_size);
  if (ret < 0) {
    LOG(ERROR) << "BuildSearchEngine [" << index_root_path << "] error!";
    return nullptr;
  }
  return engine;
}

int GammaEngine::Setup(int max_doc_size) {
  if (max_doc_size < 1) {
    return -1;
  }
  max_doc_size_ = max_doc_size;

  if (!docids_bitmap_) {
    int bitmap_bytes_size = 0;
    if (bitmap::create(docids_bitmap_, bitmap_bytes_size, max_doc_size) != 0) {
      LOG(ERROR) << "Cannot create bitmap!";
      return -1;
    }
  }

  if (!profile_) {
    profile_ = new Profile(max_doc_size);
    if (!profile_) {
      LOG(ERROR) << "Cannot create profile!";
      return -2;
    }
  }

  if (!vec_manager_) {
    vec_manager_ =
        new VectorManager(IVFPQ, MemoryOnly, docids_bitmap_, max_doc_size);
    if (!vec_manager_) {
      LOG(ERROR) << "Cannot create vec_manager!";
      return -3;
    }
  } // namespace tig_gamma

  max_docid_ = 0;
  LOG(INFO) << "GammaEngine setup successed!";
  return 0;
} // namespace tig_gamma

Response *GammaEngine::Search(const Request *request) {
#ifdef PERFORMANCE_TESTING
  double start = utils::getmillisecs();
  std::stringstream ss;
#endif

#ifdef DEBUG
  LOG(INFO) << "search request:" << RequestToString(request);
#endif

  int ret = 0;
  Response *response_results =
      static_cast<Response *>(malloc(sizeof(Response)));
  memset(response_results, 0, sizeof(Response));
  response_results->req_num = request->req_num;

  response_results->results = static_cast<SearchResult **>(
      malloc(response_results->req_num * sizeof(SearchResult *)));
  for (int i = 0; i < response_results->req_num; i++) {
    SearchResult *result =
        static_cast<SearchResult *>(malloc(sizeof(SearchResult)));
    result->total = 0;
    result->result_num = 0;
    result->result_items = nullptr;
    result->msg = nullptr;
    response_results->results[i] = result;
  }

  response_results->online_log_message = nullptr;

  if (request->req_num <= 0) {
    string msg = "req_num should not less than 0";
    for (int i = 0; i < response_results->req_num; i++) {
      response_results->results[i]->msg =
          MakeByteArray(msg.c_str(), msg.length());
      response_results->results[i]->result_code =
          SearchResultCode::SEARCH_ERROR;
    }
    return response_results;
  }

  std::string online_log_level;
  if (request->online_log_level) {
    online_log_level.assign(request->online_log_level->value,
                            request->online_log_level->len);
  }

  OnlineLogger logger;
  if (0 != logger.Init(online_log_level)) {
    LOG(WARNING) << "init online logger error!";
  }

  OLOG(&logger, INFO, "online log level: " << online_log_level);

  bool use_direct_search = ((request->direct_search_type == 1) ||
                            ((request->direct_search_type == 0) &&
                             (index_status_ != IndexStatus::INDEXED)));

  if ((not use_direct_search) && (index_status_ != IndexStatus::INDEXED)) {
    string msg = "index not trained!";
    for (int i = 0; i < response_results->req_num; i++) {
      response_results->results[i]->msg =
          MakeByteArray(msg.c_str(), msg.length());
      response_results->results[i]->result_code =
          SearchResultCode::INDEX_NOT_TRAINED;
    }
    return response_results;
  }

  GammaQuery gamma_query;
  gamma_query.logger = &logger;
  gamma_query.vec_query = request->vec_fields;
  gamma_query.vec_num = request->vec_fields_num;
  GammaSearchCondition condition;
  condition.topn = request->topn;
  condition.parallel_mode = 1; // default to parallelize over inverted list
  condition.recall_num = request->topn; // TODO: recall number should be
                                        // transmitted from search request
  condition.has_rank = request->has_rank == 1 ? true : false;
  condition.use_direct_search = use_direct_search;

  NI::RangeQueryResult numeric_filter_result; // Note its scope
  if (request->range_filters_num > 0) {
    std::vector<NI::RangeFilter> range_filters;
    for (int i = 0; i < request->range_filters_num; i++) {
      auto c = request->range_filters[i];
      range_filters.emplace_back(
          NI::RangeFilter{string(c->field->value, c->field->len),
                          string(c->lower_value->value, c->lower_value->len),
                          string(c->upper_value->value, c->upper_value->len)});
    }

    int retval = numeric_index_->Search(range_filters, numeric_filter_result);
    if (retval == 0) {
      string msg = "No result: numeric filter return 0 result";
      for (int i = 0; i < response_results->req_num; i++) {
        response_results->results[i]->msg =
            MakeByteArray(msg.c_str(), msg.length());
        response_results->results[i]->result_code = SearchResultCode::SUCCESS;
      }

      const char *log_message = logger.Data();
      if (log_message) {
        response_results->online_log_message =
            MakeByteArray(log_message, logger.Length());
      }
      return response_results;
    }

    if (retval < 0) {
      condition.numeric_results = nullptr;
    } else {
      condition.numeric_results = &numeric_filter_result;
    }
  }
#ifdef PERFORMANCE_TESTING
  double numeric_filter_time = utils::getmillisecs();
  ss << "numeric filter cost [" << numeric_filter_time - start << "]ms, ";
#endif

  /*
  NI::RangeQueryResult *numeric_filter_result = new NI::RangeQueryResult();
  if (request->range_filters_num > 0) {
    std::vector<RangeFilter *> range_filters;
    for (int i = 0; i < request->range_filters_num; i++) {
      range_filters.push_back(request->range_filters[i]);
    }
    int retval = numeric_index_->Search(range_filters, numeric_filter_result);
    if (retval == 0) {
      LOG(INFO) << "No result";
      return response_results;
    }
  }*/

  // condition.min_dist = request->vec_fields[0]->min_score;
  // condition.max_dist = request->vec_fields[0]->max_score;

  gamma_query.condition = &condition;
  if (request->vec_fields_num > 0) {
    GammaResult gamma_results[request->req_num];
    for (int i = 0; i < request->req_num; ++i) {
      gamma_results[i].total = this->GetDocsNum();
    }
    ret = vec_manager_->Search(gamma_query, gamma_results);
    if (ret != 0) {
      string msg = "search error [" + std::to_string(ret) + "]";
      for (int i = 0; i < response_results->req_num; i++) {
        response_results->results[i]->msg =
            MakeByteArray(msg.c_str(), msg.length());
        response_results->results[i]->result_code =
            SearchResultCode::SEARCH_ERROR;
      }

      const char *log_message = logger.Data();
      if (log_message) {
        response_results->online_log_message =
            MakeByteArray(log_message, logger.Length());
      }

      return response_results;
    }

    PackResults(gamma_results, response_results);
  } else {
    GammaResult gamma_result;
    gamma_result.topn = request->topn;
    gamma_result.init(request->topn, nullptr, 0);
    for (int docid = 0; docid < max_docid_; docid++) {
      if (numeric_filter_result.Has(docid) &&
          !bitmap::test(docids_bitmap_, docid)) {
        ++gamma_result.total;
        if (gamma_result.results_count < request->topn) {
          gamma_result.docs[gamma_result.results_count++].docid = docid;
        }
      }
    }
    response_results->req_num = 1; // only one result
    PackResults(&gamma_result, response_results);
  }

#ifdef PERFORMANCE_TESTING
  double search_time = utils::getmillisecs();
  if (++search_num_ % 1000 == 0) {
    ss << "search cost [" << search_time - numeric_filter_time
       << "]ms, total cost [" << search_time - start << "]ms";
    LOG(INFO) << ss.str();
  }
#endif

  const char *log_message = logger.Data();
  if (log_message) {
    response_results->online_log_message =
        MakeByteArray(log_message, logger.Length());
  }

  return response_results;
}

int GammaEngine::CreateTable(const Table *table) {
  if (!vec_manager_ || !profile_) {
    LOG(ERROR) << "vector and profile should not be null!";
    return -1;
  }
  int ret_vec = vec_manager_->CreateVectorTable(
      table->vectors_info, table->vectors_num, table->ivfpq_param);
  int ret_profile = profile_->CreateTable(table);

  if (ret_vec != 0 || ret_profile != 0) {
    LOG(ERROR) << "Cannot create table!";
    return -2;
  }

  numeric_index_ = new (std::nothrow) NI::Indexes();
  if ((nullptr == numeric_index_) || (AddNumIndexFields() < 0)) {
    LOG(ERROR) << "add numeric index fields error!";
    return -3;
  }

  LOG(INFO) << "create table="
            << std::string(table->name->value, table->name->len) << "success!";

  return 0;
}

int GammaEngine::Add(const Doc *doc) {
  if (max_docid_ >= max_doc_size_) {
    LOG(ERROR) << "Doc size reached upper size [" << max_docid_ << "]";
    return -1;
  }
  std::vector<Field *> fields_profile;
  std::vector<Field *> fields_vec;
  for (int i = 0; i < doc->fields_num; i++) {
    if (doc->fields[i]->data_type != VECTOR) {
      fields_profile.push_back(doc->fields[i]);
    } else {
      fields_vec.push_back(doc->fields[i]);
    }
  }
  // add fields into profile
  if (profile_->Add(fields_profile, max_docid_, false) != 0) {
    return -1;
  }

  for (int i = 0; i < doc->fields_num; ++i) {
    numeric_index_->Add(
        max_docid_,
        string(doc->fields[i]->name->value, doc->fields[i]->name->len),
        doc->fields[i]->value->value);
  }

  // add vectors by VectorManager
  if (vec_manager_->AddToStore(max_docid_, fields_vec) != 0) {
    return -2;
  }
  max_docid_++;

  return 0;
}

int GammaEngine::AddOrUpdate(const Doc *doc) {
  if (max_docid_ >= max_doc_size_) {
    LOG(ERROR) << "Doc size reached upper size [" << max_docid_ << "]";
    return -1;
  }
  std::vector<Field *> fields_profile;
  std::vector<Field *> fields_vec;
  string key;
  for (int i = 0; i < doc->fields_num; i++) {
    if (doc->fields[i]->data_type != VECTOR) {
      fields_profile.push_back(doc->fields[i]);
      const string &name =
          string(doc->fields[i]->name->value, doc->fields[i]->name->len);
      if (name == "_id") {
        key = string(doc->fields[i]->value->value, doc->fields[i]->value->len);
      }
    } else {
      fields_vec.push_back(doc->fields[i]);
    }
  }
  // add fields into profile
  int docid = -1;
  profile_->GetDocIDByKey(key, docid);
  if (docid == -1) {
    int ret = profile_->Add(fields_profile, max_docid_, false);
    if (ret != 0)
      return -1;
  } else {
    Del(key);
    int ret = profile_->Add(fields_profile, max_docid_, true);
    if (ret != 0)
      return -1;
  }

  for (int i = 0; i < doc->fields_num; ++i) {
    numeric_index_->Add(
        max_docid_,
        string(doc->fields[i]->name->value, doc->fields[i]->name->len),
        doc->fields[i]->value->value);
  }

  // add vectors by VectorManager
  if (vec_manager_->AddToStore(max_docid_, fields_vec) != 0) {
    return -2;
  }
  max_docid_++;

  return 0;
}

int GammaEngine::Update(const Doc *doc) {
  string key;
  std::vector<Field *> fields_profile;
  std::vector<Field *> fields_vec;
  for (int i = 0; i < doc->fields_num; i++) {
    if (doc->fields[i]->data_type != VECTOR) {
      if (strncmp(doc->fields[i]->name->value, "_id", 3) == 0) {
        key = string(doc->fields[i]->value->value, doc->fields[i]->value->len);
      }
      fields_profile.push_back(doc->fields[i]);
    } else {
      fields_vec.push_back(doc->fields[i]);
    }
  }

  int doc_id = -1;
  if (profile_->GetDocIDByKey(key, doc_id) < 0) {
    return -1;
  }

  if (Del(key) != 0) {
    return -1;
  }

  if (profile_->Add(fields_profile, max_docid_, true) != 0) {
    return -1;
  }

  for (int i = 0; i < doc->fields_num; ++i) {
    numeric_index_->Add(
        max_docid_,
        string(doc->fields[i]->name->value, doc->fields[i]->name->len),
        doc->fields[i]->value->value);
  }

  // add vectors by VectorManager
  if (vec_manager_->AddToStore(max_docid_, fields_vec) != 0) {
    return -2;
  }
  max_docid_++;
  return 0;
}

int GammaEngine::Del(const std::string &key) {
  int docid = -1, ret = 0;
  ret = profile_->GetDocIDByKey(key, docid);
  if (ret != 0 || docid < 0)
    return -1;

  if (bitmap::test(docids_bitmap_, docid)) {
    return ret;
  }
  ++delete_num;
  bitmap::set(docids_bitmap_, docid);

  return ret;
}

int GammaEngine::DelDocByQuery(Request *request) {
#ifdef DEBUG
  LOG(INFO) << "delete by query request:" << RequestToString(request);
#endif

  if (request->range_filters_num <= 0) {
    LOG(ERROR) << "no range filter";
    return 1;
  }
  NI::RangeQueryResult numeric_filter_result; // Note its scope
  std::vector<NI::RangeFilter> range_filters;
  for (int i = 0; i < request->range_filters_num; i++) {
    auto c = request->range_filters[i];
    range_filters.emplace_back(
        NI::RangeFilter{string(c->field->value, c->field->len),
                        string(c->lower_value->value, c->lower_value->len),
                        string(c->upper_value->value, c->upper_value->len)});
  }

  int retval = numeric_index_->Search(range_filters, numeric_filter_result);
  if (retval == 0) {
    LOG(ERROR) << "numeric index search error, ret=" << retval;
    return 1;
  }

  std::vector<int> doc_ids = numeric_filter_result.ToDocs();
  for (size_t i = 0; i < doc_ids.size(); ++i) {
    int docid = doc_ids[i];
    if (bitmap::test(docids_bitmap_, docid)) {
      continue;
    }
    ++delete_num;
    bitmap::set(docids_bitmap_, docid);
  }
  return 0;
}

Doc *GammaEngine::GetDoc(const std::string &id) {
  int docid = -1, ret = 0;
  ret = profile_->GetDocIDByKey(id, docid);
  if (ret != 0 || docid < 0) {
    LOG(INFO) << "GetDocIDbyKey error!";
    return nullptr;
  }

  if (bitmap::test(docids_bitmap_, docid)) {
    LOG(INFO) << "bitmap set error!";
    return nullptr;
  }
  return profile_->Get(id);
}

int GammaEngine::BuildIndex() {
  if (vec_manager_->Indexing() != 0) {
    LOG(ERROR) << "Create index failed!";
    return -1;
  }

  // WARNING: use max_docid_ instead of GetDocsNum()
  if (numeric_index_->Indexing(max_docid_) < 0) {
    LOG(ERROR) << "Indexing Numeric Fields Error!";
    return -2;
  }

  b_running_ = true;
  int ret = 0;
  while (b_running_) {
    if (vec_manager_->AddRTVecsToIndex() != 0) {
      LOG(ERROR) << "Add real time vectors to index error!";
      ret = -3;
      break;
    }
    index_status_ = IndexStatus::INDEXED;
    usleep(5000 * 1000); // sleep 5000ms
  }
  running_cv_.notify_one();
  return ret;
}

int GammaEngine::GetDocsNum() { return max_docid_ - delete_num; }

long GammaEngine::GetMemoryBytes() {
  long profile_mem_bytes = profile_->GetMemoryBytes();
  long num_mem_bytes = numeric_index_->MemoryUsage();
  long vec_mem_bytes = vec_manager_->GetTotalMemBytes();

  long total_mem_bytes = profile_mem_bytes + num_mem_bytes + vec_mem_bytes;
  LOG(INFO) << "total_mem_bytes: " << total_mem_bytes
            << ", profile_mem_bytes: " << profile_mem_bytes
            << ", num_mem_bytes: " << num_mem_bytes
            << ", vec_mem_bytes: " << vec_mem_bytes;
  return total_mem_bytes;
}

int GammaEngine::GetIndexStatus() { return index_status_; }

int GammaEngine::Dump() {
  int ret = profile_->Dump(index_root_path_, max_docid_);
  if (ret != 0) {
    LOG(ERROR) << "dump profile error, ret=" << ret;
    return -1;
  }
  ret = vec_manager_->Dump(index_root_path_);
  if (ret != 0) {
    LOG(ERROR) << "dump vector error, ret=" << ret;
    return -1;
  }
  return ret;
}

int GammaEngine::Load() {
  int ret = profile_->Load(index_root_path_, max_docid_);
  if (ret != 0) {
    LOG(ERROR) << "load profile error, ret=" << ret;
    return -1;
  }
  ret = vec_manager_->Load(index_root_path_);
  if (ret != 0) {
    LOG(ERROR) << "load vector error, ret=" << ret;
    return -1;
  }

  numeric_index_ = new (std::nothrow) NI::Indexes();
  if ((nullptr == numeric_index_) || (AddNumIndexFields() < 0)) {
    LOG(ERROR) << "add numeric index fields error!";
    return -1;
  }

  return ret;
}

int GammaEngine::AddNumIndexFields() {
  int retvals = 0;
  std::map<std::string, enum DataType> attr_type;
  retvals = profile_->GetAttrType(attr_type);

  std::map<std::string, int> attr_index;
  retvals = profile_->GetAttrIsIndex(attr_index);
  for (const auto &it : attr_type) {
    string field_name = it.first;
    const auto &attr_index_it = attr_index.find(field_name);
    if (attr_index_it == attr_index.end()) {
      LOG(ERROR) << "Cannot find field [" << field_name << "]";
      continue;
    }
    int is_index = attr_index_it->second;
    if (is_index == 0) {
      continue;
    }
    int retval = 0;
    switch (it.second) {
    case DataType::INT:
      retval = AddNumIndexField<int>(it.first);
      break;
    case DataType::LONG:
      retval = AddNumIndexField<long>(it.first);
      break;
    case DataType::FLOAT:
      retval = AddNumIndexField<float>(it.first);
      break;
    case DataType::DOUBLE:
      retval = AddNumIndexField<double>(it.first);
      break;
    default:
      break;
    }
    retvals += retval;
  }
  return retvals;
}

template <typename T>
int GammaEngine::AddNumIndexField(const std::string &field) {
  assert(numeric_index_ != nullptr);

  int field_id = profile_->GetAttrIdx(field);
  if (field_id < 0) {
    return -1;
  }

  std::function<T(const int)> getField = [&, field_id](const int docid) -> T {
    T value(0);
    if (!profile_->GetField<T>(docid, field_id, value)) {
      std::cout << "error: getField(" << docid << ", " << field_id << ").\n";
    }
    return value;
  };

  return numeric_index_->Add<T>(field, getField);
}

// TODO: handle malloc error and NULL pointer errors
void GammaEngine::PackResults(const GammaResult *gamma_results,
                              Response *response_results) {
  for (int i = 0; i < response_results->req_num; i++) {
    SearchResult *result = response_results->results[i];
    result->total = gamma_results[i].total;
    result->result_num = gamma_results[i].results_count;
    result->result_items = new ResultItem *[gamma_results[i].results_count];
    for (int j = 0; j < gamma_results[i].results_count; j++) {
      ResultItem *result_item = new ResultItem;
      result->result_items[j] = result_item;
      VectorDoc *vec_doc = gamma_results[i].docs + j;
      result->result_items[j]->score = vec_doc->score;

      int docid = vec_doc->docid;
      result->result_items[j]->doc = profile_->Get(docid);

      /*
      Doc *doc_ptr = result->result_items[j]->doc;
      doc_ptr->fields[doc_ptr->fields_num - 1]->value =
          static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
      doc_ptr->fields[doc_ptr->fields_num - 1]->value->value =
          static_cast<char *>(
              malloc(gamma_results[i].docs[j].source_len * sizeof(char)));

      memcpy(doc_ptr->fields[doc_ptr->fields_num - 1]->value->value,
             gamma_results[i].docs[j].source,
             gamma_results[i].docs[j].source_len);

      doc_ptr->fields[doc_ptr->fields_num - 1]->value->len =
          gamma_results[i].docs[j].source_len;
      */

      cJSON *extra_json = cJSON_CreateObject();
      cJSON *vec_result_json = cJSON_CreateArray();
      cJSON_AddItemToObject(extra_json, EXTRA_VECTOR_RESULT.c_str(),
                            vec_result_json);
      for (int i = 0; i < vec_doc->fields_len; i++) {
        VectorDocField *vec_field = vec_doc->fields + i;
        cJSON *vec_field_json = cJSON_CreateObject();

        cJSON_AddStringToObject(vec_field_json, EXTRA_VECTOR_FIELD_NAME.c_str(),
                                vec_field->name.c_str());
        string source = string(vec_field->source, vec_field->source_len);
        cJSON_AddStringToObject(
            vec_field_json, EXTRA_VECTOR_FIELD_SOURCE.c_str(), source.c_str());
        cJSON_AddNumberToObject(
            vec_field_json, EXTRA_VECTOR_FIELD_SCORE.c_str(), vec_field->score);
        cJSON_AddItemToArray(vec_result_json, vec_field_json);
      }
      char *extra_data = cJSON_PrintUnformatted(extra_json);
      result_item->extra = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
      result_item->extra->len = std::strlen(extra_data);
      result_item->extra->value =
          static_cast<char *>(malloc(result_item->extra->len));
      memcpy(result_item->extra->value, extra_data, result_item->extra->len);
      free(extra_data);
      cJSON_Delete(extra_json);
    }
    string msg = "Success";
    result->msg = MakeByteArray(msg.c_str(), msg.length());
    result->result_code = SearchResultCode::SUCCESS;
  }

  return;
}
} // namespace tig_gamma
