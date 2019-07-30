#include "memory_raw_vector.h"
#include <glog/logging.h>
#include <string.h>

using namespace std;

namespace tig_gamma {

MemoryRawVector::MemoryRawVector(const std::string name, int dimension,
                                 int max_doc_size)
    : RawVector(name, dimension, max_doc_size) {
  vector_mem_ = nullptr;
  str_mem_ptr_ = nullptr;
  total_mem_bytes_ = 0;
}

MemoryRawVector::~MemoryRawVector() { Close(); }

int MemoryRawVector::Init() {
  if (vector_mem_ == nullptr) {
    delete[] vector_mem_;
  }
  uint64_t len = (uint64_t)max_doc_size_ * dimension_;
  vector_mem_ = new (std::nothrow) float[len];
  if (vector_mem_ == nullptr) {
    LOG(ERROR) << "vector memory alloc err , max_doc_size_ [" << max_doc_size_
               << "], dimension_ [" << dimension_ << "]";
    return 2;  // internal error
  }
  total_mem_bytes_ += len * sizeof(float);
  /*
  FILE *fp_vector = fopen(file_path_.c_str(), "rb");
  if (fp_vector == nullptr) {
    LOG(ERROR) << "Cannot open file " << file_path_;
    return 1;
  }
  long file_size = utils::get_file_size(file_path_.c_str());
  fread((void *)vector_mem_, sizeof(char), file_size - offset_, fp_vector);
  */

  len = (uint64_t)max_doc_size_ * 100;
  str_mem_ptr_ = new (std::nothrow) char[len];
  total_mem_bytes_ += len;

  vid2docid_.resize(max_doc_size_, -1);
  total_mem_bytes_ += max_doc_size_ * sizeof(int);
  //docid2vid_.resize(max_doc_size_); // TODO: calculate memory usage

  source_mem_pos_.resize(max_doc_size_);
  source_mem_pos_.assign(max_doc_size_, 0);
  total_mem_bytes_ += max_doc_size_ * sizeof(long);

  docid2vid_.resize(max_doc_size_, nullptr);
  total_mem_bytes_ += max_doc_size_ * sizeof(docid2vid_[0]);

  return 0;
}

void MemoryRawVector::Close() {
  if (vector_mem_ != nullptr) {
    delete[] vector_mem_;
  }
  if (str_mem_ptr_) {
    delete[] str_mem_ptr_;
  }
  vid2docid_.clear();
  source_mem_pos_.clear();
  ntotal_ = 0;
  total_mem_bytes_ = 0;
  for (size_t i = 0; i < docid2vid_.size(); i++) {
    if (docid2vid_[i] != nullptr) {
      delete[] docid2vid_[i];
      docid2vid_[i] = nullptr;
    }
  }
  docid2vid_.clear();
}

const float *MemoryRawVector::GetVectors(int expected_doc_num) {
  return vector_mem_;
}

const float *MemoryRawVector::GetVector(long doc_id) const {
  if (doc_id == -1) return nullptr;
  return vector_mem_ + (uint64_t)doc_id * dimension_;
}
void MemoryRawVector::Get(std::vector<int> &doc_id,
                          std::vector<const float *> &vec) {}

void MemoryRawVector::DestroyVector(const float *vector) {}

int MemoryRawVector::AddWithIds(int n, const float *x, const int *xids,
                                int timeout) {
  if (ntotal_ + n > max_doc_size_) {
    return -1;
  }
  memcpy((void *)(vector_mem_ + (uint64_t)ntotal_ * dimension_), (void *)x,
         dimension_ * sizeof(float) * n);
  vid2docid_.insert(vid2docid_.end(), xids, xids + n);
  ntotal_ += n;
  return 0;
}

int MemoryRawVector::Add(int docid, int n, const float *x) {
  if (ntotal_ >= max_doc_size_) {
    return -1;
  }
  memcpy((void *)(vector_mem_ + (uint64_t)ntotal_ * dimension_), (void *)x,
         dimension_ * sizeof(float) * n);

  for (int i = 0; i < n; i++) {
    vid2docid_[ntotal_] = docid;
    // put to docid2vid_;
    ntotal_++;
  }

  return 0;
}

int MemoryRawVector::Add(int docid, Field *&field) {
  if (ntotal_ >= max_doc_size_) {
    return -1;
  }
  memcpy((void *)(vector_mem_ + (uint64_t)ntotal_ * dimension_),
         (void *)(field->value->value), dimension_ * sizeof(float));

  int len = field->source ? field->source->len : 0;
  if (len > 0) {
    memcpy(str_mem_ptr_ + source_mem_pos_[ntotal_], field->source->value,
           len * sizeof(char));
    source_mem_pos_[ntotal_ + 1] = source_mem_pos_[ntotal_] + len;
  } else {
    source_mem_pos_[ntotal_ + 1] = source_mem_pos_[ntotal_];
  }
  vid2docid_[ntotal_] = docid;
  if (docid2vid_[docid] == nullptr) {
    docid2vid_[docid] = utils::NewArray<int>(MAX_VECTOR_NUM_PER_DOC + 1, "init_vid_list");
    total_mem_bytes_ += (MAX_VECTOR_NUM_PER_DOC + 1) * sizeof(int);
    docid2vid_[docid][0] = 1;
    docid2vid_[docid][1] = ntotal_;
  } else {
    int *vid_list = docid2vid_[docid];
    if (vid_list[0] + 1 > MAX_VECTOR_NUM_PER_DOC) {
      return -1;
    }
    vid_list[vid_list[0]] = ntotal_;
    vid_list[0]++;
  }
  ntotal_++;
  return 0;
}

int MemoryRawVector::GetSource(int vid, char *&str, int &len) {
  if (vid >= ntotal_)
    return -1;
  else {
    len = source_mem_pos_[vid + 1] - source_mem_pos_[vid];
    str = str_mem_ptr_ + source_mem_pos_[vid];
  }
  return 0;
}

float *MemoryRawVector::GetVectorHeader() { return vector_mem_; }

int MemoryRawVector::Gets(int k, long *ids_list,
                          std::vector<const float *> &results) const {
  if (results.size() != (size_t)k) {
    return -1;
  }
  for (int i = 0; i < k; i++) {
    results[i] = GetVector(ids_list[i]);
  }
  return 0;
}

int MemoryRawVector::Dump(const string &path, int nprobe) {
  string fet_file_path = path + "/" + vector_name_ + ".fet";
  string src_file_path = path + "/" + vector_name_ + ".src";

  FILE *fet_fp = fopen(fet_file_path.c_str(), "wb");
  FILE *src_fp = fopen(src_file_path.c_str(), "wb");
  if (fet_fp == nullptr) {
    LOG(ERROR) << "open feature file error, file path=" << fet_file_path;
    return -1;
  }
  if (src_fp == nullptr) {
    LOG(ERROR) << "open source file error, file path=" << src_file_path;
    return -1;
  }
  int vec_type = static_cast<int>(MemoryOnly);
  assert(1 == fwrite((void *)&nprobe, sizeof(int), 1, fet_fp));
  assert(1 == fwrite((void *)&vec_type, sizeof(int), 1, fet_fp));
  assert(1 == fwrite((void *)&dimension_, sizeof(int), 1, fet_fp));
  assert(1 == fwrite((void *)&ntotal_, sizeof(int), 1, fet_fp));
  assert((size_t)ntotal_ ==
         fwrite((void *)vid2docid_.data(), sizeof(int), ntotal_, fet_fp));
  assert((size_t)ntotal_ == fwrite((void *)vector_mem_, sizeof(float) * dimension_,
                           ntotal_, fet_fp));

  assert(1 == fwrite((void *)&ntotal_, sizeof(int), 1, src_fp));
  assert((size_t)ntotal_ + 1 ==
         fwrite((void *)source_mem_pos_.data(), sizeof(long), ntotal_ + 1, src_fp));
  assert((size_t)source_mem_pos_[ntotal_] == fwrite((void *)str_mem_ptr_, sizeof(char),
                                            source_mem_pos_[ntotal_], src_fp));

  fclose(fet_fp);
  fclose(src_fp);

  LOG(INFO) << "dump feature file path=" << fet_file_path
            << ", source file path=" << src_file_path << ", ntotal=" << ntotal_
            << ", dimension=" << dimension_
            << ", source data size=" << source_mem_pos_[ntotal_];

  return 0;
}

int MemoryRawVector::Load(const string &path) {
  Close();
  if (0 != Init()) {
    LOG(INFO) << "init error";
    return -1;
  }
  string fet_file_path = path + "/" + vector_name_ + ".fet";
  string src_file_path = path + "/" + vector_name_ + ".src";

  FILE *fet_fp = fopen(fet_file_path.c_str(), "rb");
  FILE *src_fp = fopen(src_file_path.c_str(), "rb");
  if (fet_fp == nullptr) {
    LOG(ERROR) << "open feature file error, file path=" << fet_file_path;
    return -1;
  }
  if (src_fp == nullptr) {
    LOG(ERROR) << "open source file error, file path=" << src_file_path;
    return -1;
  }

  long fet_file_size = utils::get_file_size(fet_file_path.c_str());
  long head_len = 0;
  int vec_type = -1, dimension = 0, nprobe = 0;
  assert(1 == fread((void *)&nprobe, sizeof(int), 1, fet_fp));
  head_len += sizeof(int);
  assert(1 == fread((void *)&vec_type, sizeof(int), 1, fet_fp));
  head_len += sizeof(int);
  assert(1 == fread((void *)&dimension, sizeof(int), 1, fet_fp));
  head_len += sizeof(int);
  assert(vec_type == static_cast<int>(MemoryOnly));
  assert(dimension == dimension_);
  assert(1 == fread((void *)&ntotal_, sizeof(int), 1, fet_fp));
  head_len += sizeof(int);
  assert((size_t)ntotal_ ==
         fread((void *)vid2docid_.data(), sizeof(int), ntotal_, fet_fp));
  head_len += sizeof(int) * ntotal_;
  if ((fet_file_size - head_len) != ((long)sizeof(float)) * dimension_ * ntotal_) {
    LOG(ERROR) << "invalid feature file size=" << fet_file_size
               << ", ntotal=" << ntotal_;
    return -1;
  }
  assert((size_t)ntotal_ == fread((void *)vector_mem_, sizeof(float) * dimension_,
                          ntotal_, fet_fp));

  // create docid2vid_ from vid2docid_
  for (int vid = 0; vid < ntotal_; vid++) {
    int docid = vid2docid_[vid];
    if (docid2vid_[docid] == nullptr) {
      docid2vid_[docid] =
          utils::NewArray<int>(MAX_VECTOR_NUM_PER_DOC + 1, "load_init_vid_list");
      total_mem_bytes_ += (MAX_VECTOR_NUM_PER_DOC + 1) * sizeof(int);
      docid2vid_[docid][0] = 1;
      docid2vid_[docid][1] = vid;
    } else {
      int *vid_list = docid2vid_[docid];
      if (vid_list[0] + 1 > MAX_VECTOR_NUM_PER_DOC) {
        LOG(ERROR) << "vid list size=" << vid_list[0] + 1 << " > "
                   << MAX_VECTOR_NUM_PER_DOC << ", vid=" << vid;
        return -1;
      }
      vid_list[vid_list[0]] = vid;
      vid_list[0]++;
    }
  }

  long src_file_size = utils::get_file_size(src_file_path.c_str());
  head_len = 0;
  int ntotal = 0;
  assert(1 == fread((void *)&ntotal, sizeof(int), 1, src_fp));
  if (ntotal != ntotal_) {
    LOG(ERROR) << "source ntotal=" << ntotal
               << " is not equal to feature ntotal=" << ntotal_;
    return -1;
  }
  head_len += sizeof(int);
  assert((size_t)ntotal_ + 1 ==
         fread((void *)source_mem_pos_.data(), sizeof(long), ntotal_ + 1, src_fp));
  head_len += sizeof(long) * (ntotal_ + 1);
  if (src_file_size - head_len != source_mem_pos_[ntotal_]) {
    LOG(ERROR) << "invalid source file size=" << src_file_size
               << ", source data size=" << source_mem_pos_[ntotal_];
    return -1;
  }
  assert((size_t)source_mem_pos_[ntotal_] == fread((void *)str_mem_ptr_, sizeof(char),
                                           source_mem_pos_[ntotal_], src_fp));

  fclose(fet_fp);
  fclose(src_fp);

  LOG(INFO) << "load feature file path=" << fet_file_path
            << ", source file path=" << src_file_path << ", ntotal=" << ntotal_
            << ", dimension=" << dimension_
            << ", source data size=" << source_mem_pos_[ntotal_]
            << ", nprobe=" << nprobe;

  return 0;
}

}  // namespace tig_gamma
