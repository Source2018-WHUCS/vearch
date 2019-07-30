#ifndef RAW_VECTOR_H_
#define RAW_VECTOR_H_

#include "gamma_api.h"
#include "utils.h"
#include <string>
#include <vector>
#include "mem_cache.h"

namespace tig_gamma {

const static int MAX_VECTOR_NUM_PER_DOC = 10;

class RawVector {
public:
  RawVector(const std::string name, int dimension, int max_doc_size)
      : vector_name_(name), dimension_(dimension), max_doc_size_(max_doc_size),
    ntotal_(0), total_mem_bytes_(0){};
  virtual ~RawVector(){};

  virtual int Init() = 0;
  virtual void Close() = 0;
  virtual const float *GetVectors(int expected_doc_num) = 0;
  virtual const float *GetVector(long doc_id) const = 0;
  virtual void Get(std::vector<int> &doc_id, std::vector<const float *> &vec) {
    return;
  };
  virtual void DestroyVector(const float *vec) = 0;

  virtual int AddWithIds(int n, const float *x, const int *xids,
                         int timeout) = 0;

  virtual int Add(int docid, int n, const float *x) = 0;
  virtual int Add(int docid, Field *&field) = 0;

  virtual float *GetVectorHeader() = 0;

  virtual int GetSource(int vid, char *&str, int &len) = 0;

  virtual int Dump(const std::string &path, int nprobe) = 0;
  virtual int Load(const std::string &path) = 0;

  long GetTotalMemBytes() {return total_mem_bytes_;};

  const std::string &GetName() { return vector_name_; };

  void SetName(const std::string &name) { vector_name_ = name; };

  int GetDimension() { return dimension_; };

  void SetDimension(int dimension) { dimension_ = dimension; };

  DataType GetDataType() { return data_type_; };

  void SetDataType(DataType data_type) { data_type_ = data_type; };

  virtual int Gets(int k, long *ids_list,
                   std::vector<const float *> &resultss) const = 0;

  std::vector<int> &GetIds() { return vid2docid_; };

  int GetVectorNum() const { return ntotal_; };
  int GetMaxDocSize() const { return max_doc_size_; }

  void SetFilePath(const std::string file_path) { file_path_ = file_path; };
  void SetOffset(int len) { offset_ = len; };

  std::vector<int> vid2docid_; // vector id to doc id
  std::vector<int *> docid2vid_;  // doc id to vector id list
protected:
  std::string vector_name_; // vector name
  int dimension_;           // vector dimension
  int max_doc_size_;
  DataType data_type_; // vector data type, float only supported now
  int ntotal_;         // vector num
  std::string file_path_;
  int offset_;
  long total_mem_bytes_;
};
} // namespace tig_gamma
#endif /* RAW_VECTOR_H_ */
