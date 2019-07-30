#include "memory_disk_raw_vector.h"
#include "utils.h"
#include <errno.h>
#include <exception>
#include <fcntl.h>
#include <glog/logging.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

using namespace std;

namespace tig_gamma {

void FlushHandler(MemoryDiskRawVector *mixed_vector_house) {
  int ret = mixed_vector_house->Flush();
  if (ret != 0) {
    LOG(ERROR)
        << "flush thread of mixed vector house is exited unexpectedly, ret="
        << ret;
  } else {
    LOG(INFO) << "flush thread of mixed vector house is exited successfully";
  }
}

int MemoryDiskRawVector::Flush() {
  int peek_no_data_count = 0;
  while (!closed_) {
    try {
      int peek_size = vector_buffer_queue_->PeekSize();
      if (peek_size > 0) {
        peek_no_data_count = 0;
        int ret = FlushBatch(peek_size);
        if (-1 == ret) {
          LOG(ERROR) << "flush to vector file error, truncate it to be length "
                        "before flushing, ret="
                     << ret;
          if (0 !=
              truncate(file_path_.c_str(),
                       (uint64_t)flushed_vector_num_ * vector_byte_size_ + offset_)) {
            LOG(ERROR) << "fatal error: truncate vector file error, break the "
                          "flushing thread";
            return -1;
          }
        } else if (-2 == ret) {
          LOG(ERROR) << "open vector file error, file path" << file_path_;
        } else if (ret >= 0) {
          flushed_vector_num_ += ret;
          LOG(INFO) << "flush one batch vectors to disk success! batch size="
                    << flush_batch_size_
                    << ", max flushed vector id=" << flushed_vector_num_;
        } else {
          LOG(ERROR) << "unknown return=" << ret; // TODO: return ?
        }
      } else {
        peek_no_data_count++;
        if (peek_no_data_count >= 100) {
          LOG(INFO) << "no vector need to flush, buffer queue size="
                    << vector_buffer_queue_->Size();
          peek_no_data_count = 0;
        }
      }

      int size = vector_buffer_queue_->Size();
      if (size > resident_buffer_size_) {
        vector_buffer_queue_->MovePollIndex(size - resident_buffer_size_);
      }

      std::this_thread::sleep_for(std::chrono::milliseconds(flush_interval_));
    } catch (const std::exception &e) {
      LOG(ERROR) << "Flush exception: " << e.what();
      std::this_thread::sleep_for(std::chrono::milliseconds(flush_interval_));
    }
  }
  return 0;
}

int MemoryDiskRawVector::FlushBatch(int peek_size) {
  int fd = open(file_path_.c_str(), O_WRONLY | O_APPEND);
  if (fd == -1) {
    LOG(ERROR) << "open file error:" << strerror(errno)
               << ", file path=" << file_path_;
    return -2;
  }
  int num = 0, remain = peek_size;
  LOG(INFO) << "remain=" << remain
            << ", flush batch size=" << flush_batch_size_;
  try {
    while (remain > 0) {
      num = remain > flush_batch_size_ ? flush_batch_size_ : remain;
      LOG(INFO) << "num=" << num;
      vector_buffer_queue_->Peek(flush_batch_vectors_, dimension_,
                                 num); // error handle
      ssize_t write_size = (ssize_t)num * vector_byte_size_;
      ssize_t ret = utils::write_n(fd, (char *)flush_batch_vectors_, write_size,
                                   flush_write_retry_);
      if (ret != write_size) {
        LOG(ERROR) << "write_n error:" << strerror(errno)
                   << ", batch number=" << num << ", write size=" << write_size
                   << ", retry=" << flush_write_retry_;
        return -1;
      }
      LOG(INFO) << "write_n success, batch number=" << num
                << ", write size=" << write_size
                << ", retry=" << flush_write_retry_;
      vector_buffer_queue_->MovePeekIndex(num);
      remain -= num;
    }
  } catch (const std::exception &e) {
    LOG(ERROR) << "Flush batch exception: " << e.what();
    close(fd);
    return -1;
  }
  close(fd);
  return peek_size - remain; // return successed number
}

MemoryDiskRawVector::MemoryDiskRawVector(const std::string name, int dimension,
                                         int resident_buffer_size,
                                         int max_buffer_size, int max_doc_size)
    : RawVector(name, dimension, max_doc_size) {
  resident_buffer_size_ = resident_buffer_size;
  max_buffer_size_ = max_buffer_size;
  flush_batch_size_ = 1000;
  init_vector_num_ = 0;
  flushed_vector_num_ = 0;
  vector_byte_size_ = sizeof(float) * dimension;
  flush_interval_ = 100; // 100ms
  flush_write_retry_ = 5;
}

MemoryDiskRawVector::~MemoryDiskRawVector() {
  if (!closed_) {
    Close();
  }
}

int MemoryDiskRawVector::Init() {
  cerr << "MemoryDiskRawVector init dimension=" << dimension_ << endl;
  vector_buffer_queue_ = new VectorBufferQueue(
      max_buffer_size_, resident_buffer_size_, dimension_);
  vector_file_mapper_ =
      new VectorFileMapper(file_path_, offset_, max_doc_size_, dimension_);
  int ret = vector_file_mapper_->Map();
  if (0 != ret) {
    LOG(ERROR) << "vector file mapper map error, ret=" << ret;
    return -1;
  }
  init_vector_num_ = ntotal_ = flushed_vector_num_ =
      vector_file_mapper_->GetMappedNum();

  ret = vector_buffer_queue_->Init(init_vector_num_);
  if (0 != ret) {
    LOG(ERROR) << "init vector buffer queue error, ret=" << ret;
    return -1;
  }
  total_mem_bytes_ += vector_buffer_queue_->GetTotalMemBytes();

  flush_batch_vectors_ = new float[(uint64_t)flush_batch_size_ * dimension_];
  total_mem_bytes_ += (uint64_t)flush_batch_size_ * dimension_ * sizeof(float);

  // if (flush_thread_ != nullptr)

  flush_thread_ = new std::thread(FlushHandler, this);
  LOG(INFO) << "init success! vector byte size=" << vector_byte_size_
            << ", flush batch size=" << flush_batch_size_;
  return 0;
}

void MemoryDiskRawVector::Close() {
  closed_ = true;
  flush_thread_->join();
  if (vector_buffer_queue_ != nullptr) {
    delete vector_buffer_queue_;
  }

  if (vector_file_mapper_ != nullptr) {
    delete vector_file_mapper_;
  }

  if (flush_batch_vectors_ != nullptr) {
    delete[] flush_batch_vectors_;
  }
}

int MemoryDiskRawVector::AddWithIds(int n, const float *x, const int *xids,
                                    int timeout) {
  for (int i = 0; i < n; ++i) {
    const float *vector = x + i * dimension_;
    int ret = vector_buffer_queue_->Add(vector, dimension_, timeout);
    if (ret != 0)
      return ret;
    // direct_map_.push_back(xids[i]);
    ++ntotal_;
  }
  return 0;
}

int MemoryDiskRawVector::Add(int docid, int n, const float *x) { return -1; }

int MemoryDiskRawVector::Add(int docid, Field *&field) { return -1; }

float *MemoryDiskRawVector::GetVectorHeader() { return nullptr; }

int MemoryDiskRawVector::GetSource(int vid, char *&str, int &len) { return -1; }

int MemoryDiskRawVector::Gets(int k, long *ids_list,
                              std::vector<const float *> &results) const {
  return -1;
}

int MemoryDiskRawVector::FlushAll(int expected_doc_num) {
  int num = expected_doc_num;
  while (flushed_vector_num_ < num) {
    LOG(INFO) << "raw vector name=" << vector_name_
              << ", waiting......, expected doc num=" << expected_doc_num
              << ", flushed doc num=" << flushed_vector_num_;
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
  }
  return 0;
}

const float *MemoryDiskRawVector::GetVectors(int expected_doc_num) {
  FlushAll(expected_doc_num);
  return vector_file_mapper_->GetVectors();
}

void MemoryDiskRawVector::DestroyVector(const float *vector) {
  if (vector != nullptr) {
    delete[] vector;
  }
}

const float *MemoryDiskRawVector::GetVector(long doc_id) const {
  if (doc_id >= ntotal_ || doc_id < 0) {
    return nullptr;
  };

  float *vector = new float[dimension_];
  int ret = vector_buffer_queue_->GetVector(doc_id, vector, dimension_);
  if (ret == 0) {
    return vector;
  } else if (ret == 4) { // it's not in buffer queue
    const float *fea = vector_file_mapper_->GetVector(doc_id);
    if (fea == NULL) {
      return nullptr; // not existed
    } else {
      memcpy((void *)vector, (void *)fea, vector_byte_size_);
      return vector;
    }
  } else {
    return nullptr;
  }
}

void MemoryDiskRawVector::Get(std::vector<int> &doc_id,
                              std::vector<const float *> &vec) {}

} // namespace tig_gamma
