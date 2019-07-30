#ifndef MEMORY_DISK_RAW_VECTOR_H_
#define MEMORY_DISK_RAW_VECTOR_H_

#include "raw_vector.h"
#include "vector_buffer_queue.h"
#include "vector_file_mapper.h"
#include <string>
#include <thread>

namespace tig_gamma {

class MemoryDiskRawVector : public RawVector {
public:
  MemoryDiskRawVector(const std::string name, int dimension,
                      int resident_buffer_size, int max_buffer_size,
                      int max_doc_size);
  ~MemoryDiskRawVector();
  int Init(); // malloc memory and mmap file, if file is not existed, create it
              // and write header to it
  void Close(); // free resource
  const float *GetVectors(int expected_doc_num);
  const float *GetVector(long doc_id) const;
  void Get(std::vector<int> &doc_id, std::vector<const float *> &vec);
  void DestroyVector(const float *vector);
  int AddWithIds(int n, const float *x, const int *xids, int timeout);
  int Add(int docid, int n, const float *x);
  int Add(int docid, Field *&field);
  float *GetVectorHeader();
  int GetSource(int vid, char *&str, int &len);
  int Gets(int k, long *ids_list, std::vector<const float *> &results) const;

  int Dump(const std::string &path, int nprobe) {return -1;}
  int Load(const std::string &path) {return -1;}

  // int AddVector(float *vector, int dimension, int timeout);
  int FlushAll(int expected_doc_num);
  int Flush();
  // int GetVector(int id, float *vector, int dimension);

private:
  int FlushBatch(int peek_size);

private:
  VectorBufferQueue *vector_buffer_queue_;
  VectorFileMapper *vector_file_mapper_;
  int resident_buffer_size_;
  int max_buffer_size_;
  int flush_batch_size_;
  int flush_write_retry_;
  std::thread *flush_thread_;
  int init_vector_num_;
  int flushed_vector_num_;
  bool closed_;
  int flush_interval_; // ms
  float *flush_batch_vectors_;
  int vector_byte_size_;
};

} // namespace tig_gamma

#endif
