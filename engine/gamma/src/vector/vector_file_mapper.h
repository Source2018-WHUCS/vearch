#ifndef VECTOR_FILE_MAPPER_H_
#define VECTOR_FILE_MAPPER_H_
#include <string>
#include <sys/mman.h>

namespace tig_gamma {

class VectorFileMapper {
public:
  VectorFileMapper(std::string file_path, int offset, int max_size,
                   int dimension);
  ~VectorFileMapper();
  int Map();
  int Unmap();
  const float *GetVector(int id);
  const float *GetVectors();
  int GetMappedNum() const { return mapped_num_; };

private:
  void *buf_;
  float *vectors_;
  std::string file_path_;
  int offset_;
  int max_size_;
  int dimension_;
  size_t map_byte_size_;
  int mapped_num_;
};

} // namespace tig_gamma

#endif
