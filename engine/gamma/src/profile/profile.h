#ifndef PROFILE_H_
#define PROFILE_H_

#include "gamma_api.h"
#include "mem_cache.h"
#include <glog/logging.h>
#include <map>
#include <string>
#include <vector>

namespace tig_gamma {

/** profile, support add, update, delete, dump and load.
 */
class Profile {
public:
  explicit Profile(const int max_doc_size);

  ~Profile();

  /** create table
   *
   * @param table  table definition
   * @return 0 if successed
   */
  int CreateTable(const Table *table);

  /** add a doc to table
   *
   * @param doc     doc to add
   * @param doc_idx doc index number
   * @return 0 if successed
   */
  int AddDoc(const std::vector<Field *> &fields, int doc_idx);

  /** add a doc to table, if doc existed, update it
   *
   * @param doc     doc to add
   * @param doc_idx doc index number
   * @return 0 if successed
   */
  int AddOrUpdateDoc(const std::vector<Field *> &fields, int doc_idx);

  /** get docid by key
   *
   * @param key key to get
   * @param doc_id output, the docid to key
   * @return 0 if successed, -1 key not found
   */
  int GetDocIDbyKey(const std::string &key, int &doc_id);

  /** dump datas to disk
   *
   * @return ResultCode
   */
  // ResultCode Dump();
  int Dump(const string &path, int doc_num);

  long GetMemoryBytes();

  Doc *GetDocByID(const std::string &id);
  Doc *GetDocByDocid(const int &docid);

  template <typename T>
  bool GetField(const int docid, const int field_id, T &value) const {
    if ((docid < 0) or
        (field_id < 0 || field_id >= field_num_))
      return false;

    size_t offset = docid * item_length_ + idx_attr_offset_[field_id];
    memcpy(&value, mem_ + offset, sizeof(T));
    return true;
  }

  template <typename T>
  void GetField(int docid, const std::string &field, T &value) const {
    const auto &iter = attr_idx_map_.find(field);
    if (iter == attr_idx_map_.end()) {
      return;
    }
    GetField<T>(docid, iter->second, value);
  }

  int GetField(int docid, const std::string &field, char **value) const;

  std::map<std::string, enum DataType> &getAttrType();

  std::map<std::string, int> &getAttrIsIndex();

  int GetAttrIdx(const std::string &field) const;

  int Load(const string &path, int &doc_num);

private:
  int FTypeSize(enum DataType fType);

  void SetFieldValue(int docid, const std::string &field, const char *value,
                     uint16_t len);

  int AddField(const string &name, enum DataType ftype, int is_index);

  std::string name_;  // table name
  std::string path_;  // datas files path
  int item_length_;   // every doc item length
  int head_length_;   // profile file head length
  uint8_t field_num_; // field number
  int key_idx_;       // key postion

  std::map<int, std::string> idx_attr_map_;
  std::map<std::string, int> attr_idx_map_;
  std::map<std::string, enum DataType> attr_type_map_;
  std::map<std::string, int> attr_is_index_map_;
  std::vector<int> idx_attr_offset_;
  std::vector<enum DataType> attrs_;
  int *docid_list_ptr_;
  MemCache *item_to_docid_;

  char *mem_;
  char *str_mem_;
  uint64_t max_profile_size_;
  uint64_t max_str_size_;
  uint64_t str_offset_;

  bool table_created_;
};

inline struct ByteArray *StringToByteArray(const std::string &str) {
  struct ByteArray *ba =
      static_cast<struct ByteArray *>(malloc(sizeof(struct ByteArray)));
  ba->len = str.length();
  ba->value = static_cast<char *>(malloc((str.length()) * sizeof(char)));
  memset(ba->value, 0, str.length());
  memcpy(ba->value, str.data(), str.length());
  return ba;
}
} // namespace tig_gamma

#endif
