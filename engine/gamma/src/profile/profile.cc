/**
 * Copyright (c) The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

#include "profile.h"

#include <fcntl.h>
#include <sys/mman.h>

#include <fstream>
#include <string>

#include "utils.h"

using std::move;
using std::string;
using std::vector;

namespace tig_gamma {

Profile::Profile(const int max_doc_size) {
  item_length_ = 0;
  head_length_ = 0;
  field_num_ = 0;
  key_idx_ = -1;
  mem_ = nullptr;
  str_mem_ = nullptr;
  max_profile_size_ = max_doc_size;
  max_str_size_ = max_profile_size_ * 128;
  str_offset_ = 0;

  if (!item_to_docid_.reserve(max_doc_size)) {
    LOG(ERROR) << "item_to_docid reserve failed!";
  }

  table_created_ = false;
  name_ = "test";
  LOG(INFO) << "Profile created success!";
}

Profile::~Profile() {
  if (mem_ != nullptr) {
    delete[] mem_;
  }

  if (str_mem_ != nullptr) {
    delete[] str_mem_;
  }
}

int Profile::Load(const string &path, int &doc_num) {
  const std::vector<string> files = utils::ls(path);
  for (const auto file : files) {
    auto strs = utils::split(file, ".");
    if (strs.size() == 2 && strs[1] == "prf") {
      name_ = strs[0];
      strs = utils::split(name_, "/");
      name_ = strs[strs.size() - 1];
      break;
    }
  }
  LOG(INFO) << "name_ " << name_;
  const string prf_name = path + name_ + ".prf";

  FILE *fp_prf = fopen(prf_name.c_str(), "rb");
  if (fp_prf == nullptr) {
    LOG(ERROR) << "Cannot open file " << prf_name;
    return -1;
  }

  fread((void *)(&field_num_), sizeof(uint8_t), 1, fp_prf);
  LOG(INFO) << "field_num = " << static_cast<int>(field_num_);
  head_length_ += 1;
  for (int i = 0; i < field_num_; ++i) {
    idx_attr_offset_.push_back(item_length_);
    uint8_t a = 0;
    fread((void *)(&a), sizeof(uint8_t), 1, fp_prf);

    uint8_t type = a & 0x07;
    enum DataType ftype = static_cast<enum DataType>(type);
    attrs_.push_back(ftype);

    int8_t is_index = -1;
    fread((void *)(&is_index), sizeof(int8_t), 1, fp_prf);

    uint8_t len = ((a >> 3) & 0x1F) + 1;
    char name_c[len];
    fread((void *)(name_c), sizeof(char), len, fp_prf);

    string name = string(name_c, len);
    item_length_ += FTypeSize(ftype);
    idx_attr_map_.insert(std::pair<int, string>(i, name));
    attr_idx_map_.insert(std::pair<string, int>(name, i));
    attr_type_map_.insert(std::pair<string, enum DataType>(name, ftype));
    attr_is_index_map_.insert(std::pair<string, int>(name, is_index));

    head_length_ += 2 + len;
    LOG(INFO) << "attr name : " << name
              << ", type : " << static_cast<int>(type);
  }

  int file_size = utils::get_file_size(prf_name.c_str());
  if (mem_ != nullptr) {
    delete mem_;
  }
  mem_ = new char[max_profile_size_ * item_length_];
  fread((void *)mem_, sizeof(char), file_size - head_length_, fp_prf);
  fclose(fp_prf);

  doc_num = (file_size - head_length_) / item_length_;

  const string prf_str_name = path + name_ + ".str.prf";
  FILE *fp_str_output = fopen(prf_str_name.c_str(), "rb");
  if (fp_str_output == nullptr) {
    LOG(ERROR) << "Cannot open file " << prf_str_name;
    return -2;
  }

  file_size = utils::get_file_size(prf_str_name.c_str());
  if (str_mem_ != nullptr) {
    delete str_mem_;
  }

  str_mem_ = new char[max_str_size_];
  fread((void *)str_mem_, sizeof(char), file_size, fp_str_output);

  fclose(fp_str_output);

  table_created_ = true;
  LOG(INFO) << "Read table " << path_ << " " << name_
            << " success, doc_num : " << doc_num
            << ", item_length_ : " << item_length_;
  return 0;
}

int Profile::CreateTable(const Table *table) {
  if (table_created_) {
    return -10;
  }
  name_ = std::string(table->name->value, table->name->len);
  for (int i = 0; i < table->fields_num; ++i) {
    const string name =
        string(table->fields[i]->name->value, table->fields[i]->name->len);
    enum DataType ftype = table->fields[i]->data_type;
    int is_index = table->fields[i]->is_index;
    LOG(INFO) << "Add field name [" << name << "], type [" << ftype
              << "], index [" << is_index << "]";
    int ret = AddField(name, ftype, is_index);
    if (ret != 0) {
      return ret;
    }
  }

  if (key_idx_ == -1) {
    LOG(ERROR) << "No field _id! ";
    return -1;
  }

  if (mem_) {
    delete[] mem_;
  }
  if (str_mem_) {
    delete[] str_mem_;
  }

  mem_ = new char[max_profile_size_ * item_length_];
  str_mem_ = new char[max_str_size_];

  table_created_ = true;
  LOG(INFO) << "Create table " << name_ << " success!";
  return 0;
}

int Profile::FTypeSize(DataType fType) {
  int length = 0;
  if (fType == DataType::INT) {
    length = sizeof(int32_t);
  } else if (fType == DataType::LONG) {
    length = sizeof(int64_t);
  } else if (fType == DataType::FLOAT) {
    length = sizeof(float);
  } else if (fType == DataType::DOUBLE) {
    length = sizeof(double);
  } else if (fType == DataType::STRING) {
    length = sizeof(uint64_t) + sizeof(uint16_t);
  }
  return length;
}

void Profile::SetFieldValue(int docid, const std::string &field,
                            const char *value, uint16_t len) {
  const auto &iter = attr_idx_map_.find(field);
  if (iter == attr_idx_map_.end()) {
    LOG(ERROR) << "Cannot find field [" << field << "]";
    return;
  }
  int idx = iter->second;
  size_t offset = (uint64_t)docid * item_length_ + idx_attr_offset_[idx];
  enum DataType attr = attrs_[idx];
  if (attr == DataType::INT) {
    memcpy(mem_ + offset, value, sizeof(int32_t));
  } else if (attr == DataType::LONG) {
    memcpy(mem_ + offset, value, sizeof(int64_t));
  } else if (attr == DataType::FLOAT) {
    memcpy(mem_ + offset, value, sizeof(float));
  } else if (attr == DataType::DOUBLE) {
    memcpy(mem_ + offset, value, sizeof(double));
  } else if (attr == DataType::STRING) {
    int ofst = 0;
    ofst += sizeof(uint64_t);
    if ((str_offset_ + len) >= max_str_size_) {
      LOG(ERROR) << "Str memory reached max size [" << max_str_size_ << "]";
      return;
    }
    memcpy(mem_ + offset, &str_offset_, sizeof(uint64_t));
    memcpy(mem_ + offset + ofst, &len, sizeof(uint16_t));
    memcpy(str_mem_ + str_offset_, value, sizeof(char) * len);
    str_offset_ += len;
  }
}

int Profile::AddField(const string &name, enum DataType ftype, int is_index) {
  if (attr_idx_map_.find(name) != attr_idx_map_.end()) {
    LOG(ERROR) << "Duplicate field " << name;
    return -1;
  }
  if (name == "_id") {
    key_idx_ = field_num_;
  }
  idx_attr_offset_.push_back(item_length_);
  item_length_ += FTypeSize(ftype);
  attrs_.push_back(ftype);
  idx_attr_map_.insert(std::pair<int, string>(field_num_, name));
  attr_idx_map_.insert(std::pair<string, int>(name, field_num_));
  attr_type_map_.insert(std::pair<string, enum DataType>(name, ftype));
  attr_is_index_map_.insert(std::pair<string, int>(name, is_index));
  ++field_num_;
  return 0;
}

int Profile::GetDocIDByKey(const std::string &key, int &doc_id) {
  if (item_to_docid_.find(key, doc_id)) {
    return 0;
  }

  return -1;
}

int Profile::Add(const std::vector<Field *> &fields, int doc_id,
                    bool is_existed) {
  if (doc_id >= static_cast<int>(max_profile_size_)) {
    LOG(ERROR) << "Doc num reached upper limit [" << max_profile_size_ << "]";
    return -1;
  }
  string key;
  for (size_t i = 0; i < fields.size(); ++i) {
    const auto field_value = fields[i];
    const string &name =
        std::string(field_value->name->value, field_value->name->len);
    if (name == "_id") {
      key = string(field_value->value->value, field_value->value->len);
      break;
    }
  }
#ifdef DEBUG__
  printDoc(doc);
#endif
  if (key.empty()) {
    LOG(ERROR) << "Add item error : _id is null!";
    return -1;
  }

  if (is_existed) {
    item_to_docid_.erase(key);
  }

  item_to_docid_.insert(key, doc_id);

  for (size_t i = 0; i < fields.size(); ++i) {
    const auto field_value = fields[i];
    const string &name =
        std::string(field_value->name->value, field_value->name->len);

    auto it = attr_idx_map_.find(name);
    if (it == attr_idx_map_.end()) {
      LOG(ERROR) << "Cannot find field name : " << name;
      continue;
    }
    SetFieldValue(doc_id, name.c_str(), field_value->value->value,
                  field_value->value->len);
  }

  if (doc_id % 10000 == 0) {
    LOG(INFO) << "Add item _id = " << key << ", num " << doc_id;
  }
  return 0;
}

int Profile::Dump(const string &path, int doc_num) {
  const string prf_name = path + name_ + ".prf";
  const string prf_str_name = path + name_ + ".str.prf";
  FILE *fp_output = fopen(prf_name.c_str(), "wb");
  if (fp_output == nullptr) {
    LOG(ERROR) << "Cannot write file " << prf_name;
    return -1;
  }

  FILE *fp_str_output = fopen(prf_str_name.c_str(), "wb");
  if (fp_str_output == nullptr) {
    LOG(ERROR) << "Cannot write file " << prf_str_name;
    return -1;
  }

  fwrite((void *)(&field_num_), sizeof(uint8_t), 1, fp_output);
  head_length_ += 1;
  for (const auto &it : idx_attr_map_) {
    int idx = it.first;
    string name = it.second;
    uint8_t type = static_cast<uint8_t>(attrs_[idx]);
    const char *n = name.data();
    uint8_t len = name.length() - 1;
    uint8_t a = (len << 3) | type;
    fwrite((void *)(&a), sizeof(uint8_t), 1, fp_output);

    int8_t is_index = attr_is_index_map_[name];
    fwrite((void *)(&is_index), sizeof(uint8_t), 1, fp_output);
    fwrite((void *)(n), sizeof(char), name.length(), fp_output);
    head_length_ += 1 + name.length();
  }

  LOG(INFO) << "head_length = " << head_length_
            << " item_length = " << item_length_;

  fwrite((void *)(mem_), sizeof(char), doc_num * item_length_, fp_output);
  fwrite((void *)(str_mem_), sizeof(char), str_offset_, fp_str_output);

  fclose(fp_output);
  fclose(fp_str_output);

  return 0;
}

long Profile::GetMemoryBytes() {
  return max_profile_size_ * item_length_ + max_str_size_;
}

Doc *Profile::Get(const int docid) {
  Doc *doc = static_cast<Doc *>(malloc(sizeof(Doc)));

  doc->fields_num = attr_type_map_.size();
  doc->fields =
      static_cast<Field **>(malloc(doc->fields_num * sizeof(Field *)));
  memset(doc->fields, 0, doc->fields_num * sizeof(Field *));
  int i = 0;
  for (const auto &it : attr_type_map_) {
    enum DataType type = it.second;
    const string &attr = it.first;
    Field *field = static_cast<Field *>(malloc(sizeof(Field)));
    memset(field, 0, sizeof(Field));
    field->name = StringToByteArray(attr);
    field->value = static_cast<ByteArray *>(malloc(sizeof(ByteArray)));
    if (type == DataType::INT) {
      int value = 0;
      GetField<int>(docid, attr, value);
      field->value->len = sizeof(int);
      field->value->value = static_cast<char *>(malloc(field->value->len));
      memcpy(field->value->value, &value, field->value->len);
    } else if (type == DataType::LONG) {
      long value = 0;
      GetField<long>(docid, attr, value);
      field->value->len = sizeof(long);
      field->value->value = static_cast<char *>(malloc(field->value->len));
      memcpy(field->value->value, &value, field->value->len);
    } else if (type == DataType::FLOAT) {
      float value = 0;
      GetField<float>(docid, attr, value);
      field->value->len = sizeof(float);
      field->value->value = static_cast<char *>(malloc(field->value->len));
      memcpy(field->value->value, &value, field->value->len);
    } else if (type == DataType::DOUBLE) {
      double value = 0;
      GetField<double>(docid, attr, value);
      field->value->len = sizeof(double);
      field->value->value = static_cast<char *>(malloc(field->value->len));
      memcpy(field->value->value, &value, field->value->len);
    } else if (type == DataType::STRING) {
      char *value;
      field->value->len = GetField(docid, attr, &value);
      field->value->value = static_cast<char *>(malloc(field->value->len));
      memcpy(field->value->value, value, field->value->len);
    }
    field->data_type = type;
    doc->fields[i] = field;
    ++i;
  }

  return doc;
}

Doc *Profile::Get(const std::string &key) {
  int doc_id = 0;
  int ret = GetDocIDByKey(key, doc_id);
  if (ret < 0) {
    return nullptr;
  }
  Doc *doc = Get(doc_id);
  return doc;
}

int Profile::GetField(int docid, const std::string &field, char **value) const {
  const auto &iter = attr_idx_map_.find(field);
  if (iter == attr_idx_map_.end()) {
    LOG(ERROR) << "docid " << docid << " field " << field;
    return -1;
  }
  int idx = iter->second;
  size_t offset = (uint64_t)docid * item_length_ + idx_attr_offset_[idx];
  size_t str_offset = 0;
  memcpy(&str_offset, mem_ + offset, sizeof(size_t));
  unsigned short len;
  memcpy(&len, mem_ + offset + sizeof(size_t), sizeof(unsigned short));
  offset += sizeof(unsigned short);
  *value = str_mem_ + str_offset;
  return len;
}

int Profile::GetAttrType(std::map<std::string, enum DataType> &attr_type_map) {
  for (const auto attr_type : attr_type_map_) {
    attr_type_map.insert(attr_type);
  }
  return 0;
}

int Profile::GetAttrIsIndex(std::map<std::string, int> &attr_is_index_map) {
  for (const auto attr_is_index : attr_is_index_map_) {
    attr_is_index_map.insert(attr_is_index);
  }
  return 0;
}

int Profile::GetAttrIdx(const std::string &field) const {
  const auto &iter = attr_idx_map_.find(field.c_str());
  return (iter != attr_idx_map_.end()) ? iter->second : -1;
}

} // namespace tig_gamma
