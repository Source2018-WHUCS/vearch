#ifndef MEMORY_RAW_VECTOR_H_
#define MEMORY_RAW_VECTOR_H_

#include "raw_vector.h"

namespace tig_gamma {

class MemoryRawVector : public RawVector {
public:
  MemoryRawVector(const std::string name, int dimension, int max_doc_size);
  virtual ~MemoryRawVector();

  int Init();
  void Close();
  const float *GetVectors(int expected_doc_num);
  const float *GetVector(long doc_id) const;
  void Get(std::vector<int> &doc_id, std::vector<const float *> &vec);
  void DestroyVector(const float *vector);
  int AddWithIds(int n, const float *x, const int *xids, int timeout);

  int Add(int docid, int n, const float *x);
  int Add(int docid, Field *&field);

  float *GetVectorHeader();

  int Gets(int k, long *ids_list, std::vector<const float *> &results) const;
  int GetSource(int vid, char *&str, int &len);

  int Dump(const std::string &path, int nprobe);
  int Load(const std::string &path);

private:
  float *vector_mem_; // vector memory
  char *str_mem_ptr_;

  std::vector<long> source_mem_pos_;
};

} // namespace tig_gamma
#endif /* MEMORY_RAW_VECTOR_H_ */
