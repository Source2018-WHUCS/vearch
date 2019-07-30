#ifndef BUFFER_H
#define BUFFER_H

#include "utils.h"
#include <atomic>
#include <fcntl.h>
#include <glog/logging.h>
#include <memory>
#include <sys/mman.h>
#include <thread>
#include <vector>

namespace tig_gamma {

template <class T> class Buffer {
public:
  typedef struct _buf {
    T *buffer;
    long size;

    _buf() {
      size = 0;
      buffer = nullptr;
    }
  } BUF;

  Buffer(const std::string &file, const int offset = 0) : file_(file) {
    offset_ = offset;
    curr_idx_ = 0;
    buffers_.emplace_back(std::make_shared<BUF>());
    buffers_.emplace_back(std::make_shared<BUF>());
  }

  Buffer(const Buffer &orig) {}

  virtual ~Buffer() {
    int prepare = 1 - curr_idx_.load();
    while (buffers_[prepare].use_count() > 1) {
      std::this_thread::yield();
    }
    std::shared_ptr<BUF> buffer = buffers_[prepare];
    if (buffer->size > 0)
      munmap(buffer->buffer, buffer->size);
    curr_idx_ = prepare;
    prepare = 1 - curr_idx_.load();
    while (buffers_[prepare].use_count() > 1) {
      std::this_thread::yield();
    }
    buffer = buffers_[prepare];
    if (buffer->size > 0)
      munmap(buffer->buffer, buffer->size);
    curr_idx_ = prepare;
  }

  std::shared_ptr<BUF> getBuffer() {
    std::shared_ptr<BUF> buf = buffers_[curr_idx_.load()];
    return buf;
  }

  void mmap() {
    LOG(INFO) << "Buffer mmap " << file_;
    int prepare = 1 - curr_idx_.load();
    while (buffers_[prepare].use_count() > 1) {
      std::this_thread::yield();
    }
    std::shared_ptr<BUF> buffer = buffers_[prepare];
    if (buffer->size > 0)
      munmap(buffer->buffer, buffer->size);
    buffer->size = utils::get_file_size(file_.c_str());
    int fd = open(file_.c_str(), O_RDONLY, 0);
    auto buf = ::mmap(NULL, buffer->size, PROT_READ, MAP_SHARED, fd, 0);
    buffer->buffer = static_cast<T *>(buf + offset_);
    int ret = madvise(static_cast<void *>(buf), buffer->size,
                      MADV_WILLNEED | MADV_RANDOM);
    if (ret != 0) {
      LOG(ERROR) << "madvise error : " << ret;
    }
    close(fd);
    curr_idx_ = prepare;
  }

private:
  std::vector<std::shared_ptr<BUF>> buffers_;
  std::atomic_int curr_idx_;
  std::string file_;
  int offset_;
};

} // namespace tig_gamma

#endif /* BUFFER_H */
