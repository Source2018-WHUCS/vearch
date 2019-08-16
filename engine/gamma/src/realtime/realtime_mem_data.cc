/**
 * Copyright (c) The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

#include "realtime_mem_data.h"
#include "log.h"
#include <stdio.h>
#include <string.h>
#include <unistd.h>

namespace tig_gamma {
namespace realtime {

RTInvertBucketData::RTInvertBucketData(long **idx_array, int *retrieve_idx_pos,
                                       int *cur_bucket_keys,
                                       uint8_t **codes_array)
    : _idx_array(idx_array), _retrieve_idx_pos(retrieve_idx_pos),
      _cur_bucket_keys(cur_bucket_keys), _codes_array(codes_array) {}

RTInvertBucketData::RTInvertBucketData() {
  _idx_array = NULL;
  _retrieve_idx_pos = NULL;
  _cur_bucket_keys = NULL;
  _codes_array = NULL;
}

RTInvertBucketData::~RTInvertBucketData() {}

bool RTInvertBucketData::init(const size_t &buckets_num,
                              const size_t &bucket_keys,
                              const size_t &code_bytes_per_vec,
                              long &total_mem_bytes) {
  _idx_array = new (std::nothrow) long *[buckets_num];
  _codes_array = new (std::nothrow) uint8_t *[buckets_num];
  _cur_bucket_keys = new (std::nothrow) int[buckets_num];
  if (_idx_array == NULL || _codes_array == NULL)
    return false;
  for (size_t i = 0; i < buckets_num; i++) {
    _idx_array[i] = new (std::nothrow) long[bucket_keys];
    _codes_array[i] =
        new (std::nothrow) uint8_t[bucket_keys * code_bytes_per_vec];
    if (_idx_array[i] == NULL || _codes_array[i] == NULL)
      return false;
    _cur_bucket_keys[i] = bucket_keys;
  }

  total_mem_bytes += buckets_num * bucket_keys * sizeof(long);
  total_mem_bytes +=
      buckets_num * bucket_keys * code_bytes_per_vec * sizeof(uint8_t);
  total_mem_bytes += buckets_num * sizeof(int);

  _retrieve_idx_pos = new (std::nothrow) int[buckets_num];
  if (_retrieve_idx_pos == NULL)
    return false;
  memset(_retrieve_idx_pos, 0, buckets_num * sizeof(int));
  total_mem_bytes += buckets_num * sizeof(int);
  LOG(INFO) << "===init total_mem_bytes is " << total_mem_bytes << "===";
  return true;
}

bool RTInvertBucketData::extendBucketMem(const size_t &bucket_no,
                                         const size_t &code_bytes_per_vec,
                                         long &total_mem_bytes) {
  int extend_size = _cur_bucket_keys[bucket_no] * 2;

  uint8_t *extend_code_bytes_array =
      new (std::nothrow) uint8_t[extend_size * code_bytes_per_vec];
  if (extend_code_bytes_array == nullptr) {
    LOG(ERROR) << "memory extend_code_bytes_array alloc error!";
    return false;
  }
  memcpy((void *)extend_code_bytes_array, (void *)_codes_array[bucket_no],
         sizeof(uint8_t) * _cur_bucket_keys[bucket_no] * code_bytes_per_vec);
  _codes_array[bucket_no] = extend_code_bytes_array;
  total_mem_bytes += extend_size * code_bytes_per_vec * sizeof(uint8_t);

  long *extend_idx_array = new (std::nothrow) long[extend_size];
  if (extend_idx_array == nullptr) {
    LOG(ERROR) << "memory extend_idx_array alloc error!";
    return false;
  }
  memcpy((void *)extend_idx_array, (void *)_idx_array[bucket_no],
         sizeof(long) * _cur_bucket_keys[bucket_no]);
  _idx_array[bucket_no] = extend_idx_array;
  total_mem_bytes += extend_size * sizeof(long);

  _cur_bucket_keys[bucket_no] = extend_size;
  return true;
}

bool RTInvertBucketData::releaseBucketMem(const size_t &bucket_no,
                                          const size_t &code_bytes_per_vec,
                                          long &total_mem_bytes) {
  if (_idx_array[bucket_no]) {
    delete[] _idx_array[bucket_no];
    _idx_array[bucket_no] = NULL;
    total_mem_bytes -= _cur_bucket_keys[bucket_no] * sizeof(long);
  }
  if (_codes_array[bucket_no]) {
    delete[] _codes_array[bucket_no];
    _codes_array[bucket_no] = NULL;
    total_mem_bytes -=
        _cur_bucket_keys[bucket_no] * code_bytes_per_vec * sizeof(uint8_t);
  }
  return true;
}

bool RTInvertBucketData::destroyMem() {
  if (_idx_array) {
    delete[] _idx_array;
    _idx_array = NULL;
  }
  if (_retrieve_idx_pos) {
    delete _retrieve_idx_pos;
    _retrieve_idx_pos = NULL;
  }
  if (_cur_bucket_keys) {
    delete _cur_bucket_keys;
    _cur_bucket_keys = NULL;
  }
  if (_codes_array) {
    delete[] _codes_array;
    _codes_array = NULL;
  }
  return true;
}

bool RTInvertBucketData::getBucketMemInfo(const size_t &bucket_no,
                                          std::string &mem_info) {
  return false;
}

RealTimeMemData::RealTimeMemData(size_t buckets_num, long max_vec_size,
                                 size_t bucket_keys, size_t code_bytes_per_vec)
    : _buckets_num(buckets_num), _bucket_keys(bucket_keys),
      _code_bytes_per_vec(code_bytes_per_vec), _max_vec_size(max_vec_size) {
  _cur_invert_ptr = new (std::nothrow) RTInvertBucketData();
  _extend_invert_ptr = NULL;
  _total_mem_bytes = 0;
}

RealTimeMemData::~RealTimeMemData() {
  if (_cur_invert_ptr) {
    delete _cur_invert_ptr;
    _cur_invert_ptr = NULL;
  }
  if (_extend_invert_ptr) {
    delete _extend_invert_ptr;
    _extend_invert_ptr = NULL;
  }
}

bool RealTimeMemData::init() {
  // fprintf(stderr, "%u\n", _total_keys);
  // fprintf(stderr, "%u\n", _code_bytes_per_vec);
  // fprintf(stderr, "%u\n", _buckets_num);

  _vid_bucket_no_pos.resize(_max_vec_size, -1);

  return _cur_invert_ptr &&
         _cur_invert_ptr->init(_buckets_num, _bucket_keys, _code_bytes_per_vec,
                               _total_mem_bytes);
}

bool RealTimeMemData::addKeys(size_t list_no, size_t n, std::vector<long> &keys,
                              std::vector<uint8_t> &keys_codes) {
  if (keys.size() * _code_bytes_per_vec != keys_codes.size()) {
    LOG(ERROR) << "number of key and key codes not match!";
    return false;
  }
  int retrive_pos = _cur_invert_ptr->_retrieve_idx_pos[list_no];
  // copy new added idx to idx buffer

  if (NULL == _cur_invert_ptr->_idx_array[list_no]) {
    LOG(ERROR) << "-------idx_array is NULL!--------";
  }
  memcpy((void *)(_cur_invert_ptr->_idx_array[list_no] + retrive_pos),
         (void *)(keys.data()), sizeof(long) * keys.size());

  // copy new added codes to codes buffer
  memcpy((void *)(_cur_invert_ptr->_codes_array[list_no] +
                  retrive_pos * _code_bytes_per_vec),
         (void *)(keys_codes.data()), sizeof(uint8_t) * keys_codes.size());

  for (size_t i = 0; i < keys.size(); i++) {
    if (keys[i] >= _max_vec_size) {
      return false;
    }
    _vid_bucket_no_pos[keys[i]] = list_no << 32 | retrive_pos;
    retrive_pos++;
  }

  // atomic switch retriving pos of list_no
  _cur_invert_ptr->_retrieve_idx_pos[list_no] = retrive_pos;
  return true;
}

bool RealTimeMemData::extendBucketMem(const size_t &bucket_no) {
  _extend_invert_ptr = new (std::nothrow) RTInvertBucketData(
      _cur_invert_ptr->_idx_array, _cur_invert_ptr->_retrieve_idx_pos,
      _cur_invert_ptr->_cur_bucket_keys, _cur_invert_ptr->_codes_array);
  if (!_extend_invert_ptr) {
    LOG(ERROR) << "memory _extend_invert_ptr alloc error!";
    return false;
  }

  long *old_idx_array = _cur_invert_ptr->_idx_array[bucket_no];
  uint8_t *old_codes_array = _cur_invert_ptr->_codes_array[bucket_no];
  int old_keys = _cur_invert_ptr->_cur_bucket_keys[bucket_no];

  // WARNING:
  // the above _idx_array and _codes_array pointer would be changed by
  // extendBucketMem()
  if (!_extend_invert_ptr->extendBucketMem(bucket_no, _code_bytes_per_vec,
                                           _total_mem_bytes)) {
    LOG(ERROR) << "extendBucketMem error!";
    return false;
  }

  RTInvertBucketData *old_invert_ptr = _cur_invert_ptr;
  _cur_invert_ptr = _extend_invert_ptr;

  sleep(1);

  if (old_idx_array) {
    delete old_idx_array;
    old_idx_array = NULL;
    _total_mem_bytes -= old_keys * sizeof(long);
  }

  if (old_codes_array) {
    delete old_codes_array;
    old_codes_array = NULL;
    _total_mem_bytes -= old_keys * _code_bytes_per_vec * sizeof(uint8_t);
  }

  delete old_invert_ptr;
  old_invert_ptr = NULL;
  _extend_invert_ptr = NULL;

  return true;
}

bool RealTimeMemData::getIvtList(const size_t &bucket_no, long *&ivt_list,
                                 uint8_t *&ivt_codes_list) {
  ivt_list = _cur_invert_ptr->_idx_array[bucket_no];
  ivt_codes_list = (uint8_t *)(_cur_invert_ptr->_codes_array[bucket_no]);

  return true;
}

int RealTimeMemData::RetrieveCodes(
    int *vids, size_t vid_size,
    std::vector<std::vector<const uint8_t *>> &bucket_codes,
    std::vector<std::vector<long>> &bucket_vids) {
  bucket_codes.resize(_buckets_num);
  bucket_vids.resize(_buckets_num);
  for (size_t i = 0; i < _buckets_num; i++) {
    bucket_codes[i].reserve(vid_size / _buckets_num);
    bucket_vids[i].reserve(vid_size / _buckets_num);
  }

  for (size_t i = 0; i < vid_size; i++) {
    if (_vid_bucket_no_pos[vids[i]] != -1) {
      int bucket_no = _vid_bucket_no_pos[vids[i]] >> 32;
      int pos = _vid_bucket_no_pos[vids[i]] & 0xffffffff;
      bucket_codes[bucket_no].push_back(
          _cur_invert_ptr->_codes_array[bucket_no] + pos * _code_bytes_per_vec);
      bucket_vids[bucket_no].push_back(vids[i]);
    }
  }

  return 0;
}

int RealTimeMemData::RetrieveCodes(
    int **vids_list, size_t vids_list_size,
    std::vector<std::vector<const uint8_t *>> &bucket_codes,
    std::vector<std::vector<long>> &bucket_vids) {
  bucket_codes.resize(_buckets_num);
  bucket_vids.resize(_buckets_num);
  for (size_t i = 0; i < _buckets_num; i++) {
    bucket_codes[i].reserve(vids_list_size / _buckets_num);
    bucket_vids[i].reserve(vids_list_size / _buckets_num);
  }

  for (size_t i = 0; i < vids_list_size; i++) {
    for (int j = 1; j <= vids_list[i][0]; j++) {
      int vid = vids_list[i][j];
      if (_vid_bucket_no_pos[vid] != -1) {
        int bucket_no = _vid_bucket_no_pos[vid] >> 32;
        int pos = _vid_bucket_no_pos[vid] & 0xffffffff;
        bucket_codes[bucket_no].push_back(
            _cur_invert_ptr->_codes_array[bucket_no] +
            pos * _code_bytes_per_vec);
        bucket_vids[bucket_no].push_back(vid);
      }
    }
  }

  return 0;
}

} // namespace realtime

} // namespace tig_gamma
