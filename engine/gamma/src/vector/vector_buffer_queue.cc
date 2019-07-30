#include "vector_buffer_queue.h"
#include "thread_util.h"
#include <cassert>
#include <glog/logging.h>
#include <iostream>
#include <stdexcept>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

using namespace std;

VectorBufferQueue::VectorBufferQueue(int max_size, int resident_size,
                                     int dimension) {
  max_size_ = max_size;
  resident_size_ = resident_size;
  dimension_ = dimension;
  poll_index_ = add_index_ = peek_index_ = 0;
  total_mem_bytes_ = 0;
}

VectorBufferQueue::~VectorBufferQueue() {
  if (buffer_ != NULL) {
    free(buffer_);
    buffer_ = nullptr;
  }
  int ret = pthread_rwlock_destroy(&shared_mutex_);
  if (0 != ret) {
    LOG(ERROR) << "destory read write lock error, ret=" << ret;
  }
}

int VectorBufferQueue::Init(int init_index) {
  if (dimension_ <= 0) {
    return 1;
  }
  feature_byte_size_ = sizeof(float) * dimension_;
  cout << "buffer byte size=" << max_size_ * feature_byte_size_
       << ", buffer vector size=" << max_size_ << endl;
  buffer_ = (float *)calloc(max_size_, feature_byte_size_);
  if (buffer_ == NULL) {
    cerr << "malloc buffer failed" << endl;
    return 2;
  }
  total_mem_bytes_ += max_size_ * feature_byte_size_;
  poll_index_ = add_index_ = peek_index_ = init_index;

  int ret = pthread_rwlock_init(&shared_mutex_, NULL);
  if (ret != 0) {
    LOG(ERROR) << "init read-write lock error, ret=" << ret;
    return 2;
  }
  return 0;
}
int VectorBufferQueue::Add(const float *v, int dim, int timeout) {
  if (v == NULL || dim != dimension_)
    return 1;

  if (!WaitFor(timeout, 1, 1)) {
    return 3; // timeout
  }

  memcpy((void *)(buffer_ + add_index_ % max_size_ * dimension_), (void *)v,
         feature_byte_size_);
  add_index_++;
  return 0;
}
int VectorBufferQueue::Add(const float *v, int dim, int num, int timeout) {
  if (v == NULL || dim != dimension_ || num <= 0)
    return 1;

  if (!WaitFor(timeout, 1, num)) {
    return 3; // timeout
  }

  memcpy((void *)(buffer_ + add_index_ % max_size_ * dimension_), (void *)v,
         feature_byte_size_ * num);
  add_index_ += num;
  return 0;
}
int VectorBufferQueue::Poll(float *v, int *dim, int timeout) {
  if (v == NULL || dim == NULL)
    return 1;

  if (!WaitFor(timeout, 2, 1)) {
    return 3; // timeout
  }

  memcpy((void *)v, (void *)(buffer_ + poll_index_ % max_size_ * dimension_),
         feature_byte_size_);
  poll_index_++;
  *dim = dimension_;
  return 0;
}
int VectorBufferQueue::Poll(float *v, int *dim, int num, int timeout) {
  if (v == NULL || dim == NULL || num <= 0)
    return 1;

  if (!WaitFor(timeout, 2, num)) {
    return 3; // timeout
  }
  memcpy((void *)v, (void *)(buffer_ + poll_index_ % max_size_ * dimension_),
         feature_byte_size_ * num);
  poll_index_ += num;
  *dim = dimension_;
  return 0;
}

int VectorBufferQueue::GetVector(int id, float *v, int dim) {
  if (v == nullptr || dim != dimension_)
    return 1;

  ReadThreadLock read_lock(shared_mutex_);
  if ((std::uint64_t)id < poll_index_ || (std::uint64_t)id >= add_index_) {
    return 4; // no existed
  }

  memcpy((void *)v, (void *)(buffer_ + id % max_size_ * dimension_),
         feature_byte_size_);
  return 0;
}

int VectorBufferQueue::Peek(float *v, int dim, int num) const {
  if (v == NULL || dim != dimension_ || num <= 0)
    return 1;

  if (add_index_ - peek_index_ < (std::uint64_t)num) {
    return 4;
  }

  for (int i = 0; i < num; i++) {
    memcpy((void *)(v + i * dim),
           (void *)(buffer_ + (peek_index_ + i) % max_size_ * dimension_),
           feature_byte_size_);
  }
  return 0;
}

int VectorBufferQueue::PeekSize() const { return add_index_ - peek_index_; }

int VectorBufferQueue::Size() const { return add_index_ - poll_index_; }

void VectorBufferQueue::MovePollIndex(int inc) {
  WriteThreadLock write_lock(shared_mutex_);
  if (poll_index_ + inc > peek_index_) {
    poll_index_ = peek_index_;
    return;
  }
  poll_index_ += inc;
}

void VectorBufferQueue::MovePeekIndex(int inc) { peek_index_ += inc; }

bool VectorBufferQueue::WaitFor(int timeout, int type, int num) {
  int cost = 0;
  while (timeout == -1 || cost < timeout) {
    bool status = false;
    switch (type) {
    case 1: // if it can add num vector
      status = max_size_ - (add_index_ - poll_index_) >= (std::uint64_t)num;
      break;
    case 2: // if it can poll num vector
      status = add_index_ - poll_index_ >= (std::uint64_t)num;
      break;
    default:
      throw std::invalid_argument("invalid waiting type=" +
                                  std::to_string(type));
    }
    if (status)
      return true;
    usleep(100000); // wait 100ms
    cost += 100;
  }
  return false;
}
