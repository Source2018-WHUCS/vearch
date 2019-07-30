#include "vector_file_mapper.h"
#include "utils.h"
#include <errno.h>
#include <fcntl.h>
#include <glog/logging.h>
#include <sys/stat.h>
#include <sys/types.h>

namespace tig_gamma {

VectorFileMapper::VectorFileMapper(std::string file_path, int offset,
                                   int max_size, int dimension)
    : file_path_(file_path), offset_(offset), max_size_(max_size),
      dimension_(dimension) {
  map_byte_size_ = (size_t)max_size * dimension * sizeof(float) + offset;
  buf_ = nullptr;
  vectors_ = nullptr;
}

VectorFileMapper::~VectorFileMapper() {
  if (buf_ != nullptr) {
    Unmap();
  }
}

int VectorFileMapper::Map() {
  int fd = open(file_path_.c_str(), O_RDONLY, 0);
  if (-1 == fd) {
    LOG(ERROR) << "open vector file error, path=" << file_path_;
    return -1;
  }
  buf_ = mmap(NULL, map_byte_size_, PROT_READ, MAP_SHARED, fd, 0);
  if (buf_ == MAP_FAILED) {
    LOG(ERROR) << "mmap error:" << strerror(errno)
               << ", max byte size=" << map_byte_size_
               << ", file path=" << file_path_ << ", offset=" << offset_;
    close(fd);
    return -1;
  }
  close(fd);
  vectors_ = (float *)((char *)buf_ + offset_);

  long file_size =
      utils::get_file_size(file_path_.c_str()); // TODO: file size may be int64
  mapped_num_ = ((file_size - offset_) / sizeof(float)) /
                dimension_; // TODO: check if file_size is valid

  int ret = madvise(static_cast<void *>(buf_), map_byte_size_,
                    MADV_WILLNEED | MADV_RANDOM);
  if (ret != 0) {
    LOG(ERROR) << "madvise error : " << ret;
    return -1;
  }
  LOG(INFO) << "map success!"
            << "max byte size=" << map_byte_size_
            << ", file path=" << file_path_ << ", offset=" << offset_
            << ", mapped vector number=" << mapped_num_;
  return 0;
}

int VectorFileMapper::Unmap() {
  if (buf_ == nullptr) {
    return 0;
  }

  int ret = munmap(buf_, map_byte_size_);
  if (ret != 0) {
    LOG(ERROR) << "munmap error";
    return -1;
  }
  buf_ = nullptr;
  vectors_ = nullptr;
  return 0;
}

const float *VectorFileMapper::GetVector(int id) {
  if (id < 0 || id >= max_size_)
    return nullptr;
  return vectors_ + ((long)id) * dimension_;
}

const float *VectorFileMapper::GetVectors() { return vectors_; }

} // namespace tig_gamma
