/**
 * Copyright 2019 The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

#ifndef FIELD_RANGE_INDEX_H_
#define FIELD_RANGE_INDEX_H_

#include <map>
#include <string>
#include <vector>
#include <chrono>
#include <condition_variable>
#include "gamma_api.h"
#include "profile.h"
#include "range_query_result.h"
#include "concurrentqueue/blockingconcurrentqueue.h"

namespace tig_gamma {

typedef struct {
  int field;
  std::string lower_value;
  std::string upper_value;
  int is_union;
} FilterInfo;

class ResourceToRecovery {
 public:
  ResourceToRecovery(void *data, int after = 1) {
    deadline_ = std::chrono::system_clock::now() + std::chrono::seconds(after);
    data_ = data;
  }

  ~ResourceToRecovery() {
    free(data_);
    data_ = nullptr;
  }

  std::chrono::time_point<std::chrono::system_clock> Deadline() {
    return deadline_;
  }

  void *Data() { return data_; }

 private:
  std::chrono::time_point<std::chrono::system_clock> deadline_;
  void *data_;
};

typedef moodycamel::BlockingConcurrentQueue<ResourceToRecovery *> ResourceQueue;

class FieldRangeIndex;
class MultiFieldsRangeIndex {
 public:
  MultiFieldsRangeIndex(std::string &path, Profile *profile);
  ~MultiFieldsRangeIndex();

  int Add(int docid, int field);

  int AddField(int field, enum DataType field_type);

  int Search(const std::vector<FilterInfo> &origin_filters,
             MultiRangeQueryResults *out);

 private:
  int Intersect(RangeQueryResult **results, int j, int k,
                RangeQueryResult *out);
  void ResourceRecoveryWorker();
  std::vector<FieldRangeIndex *> fields_;
  Profile *profile_;
  std::string path_;
  bool b_running_;
  std::condition_variable running_cv_;
  ResourceQueue *resource_recovery_q;
};

}  // namespace tig_gamma

#endif
