/**
 * Copyright (c) The Gamma Authors.
 *
 * This source code is licensed under the Apache License, Version 2.0 license
 * found in the LICENSE file in the root directory of this source tree.
 */

#ifndef UTILS_H_
#define UTILS_H_

#include <cassert>
#include <functional>
#include <string>
#include <vector>

namespace utils {

long get_file_size(const char *path);

std::vector<std::string> split(const std::string &p_str,
                               const std::string &p_separator);

int count_lines(const char *filename);

double elapsed();

double getmillisecs();

int isFolderExist(const char *path);

int remove_dir(const char *dir);

#ifdef _WIN32

inline char file_sepator() { return '\\'; }
#else

inline char file_sepator() { return '/'; }
#endif

using file_filter_type = std::function<bool(const char *, const char *)>;

std::vector<std::string> for_each_file(const std::string &dir_name,
                                       file_filter_type filter,
                                       bool sub = false);

std::vector<std::string> for_each_folder(const std::string &dir_name,
                                         file_filter_type filter,
                                         bool sub = false);

std::vector<std::string> ls(const std::string &dir_name, bool sub = false);

std::vector<std::string> ls_folder(const std::string &dir_name,
                                   bool sub = false);

ssize_t write_n(int fd, const char *buf, ssize_t nbyte, int retry);

template <class T> inline T *NewArray(int len, const char *msg) {
  assert(len > 0);
  T *data = new (std::nothrow) T[len];
  if (data == nullptr) {
    throw std::runtime_error("new array error, " + std::string(msg));
  }
  return data;
}

typedef struct MEM_PACKED {
  char name[20];
  unsigned long total;
  char name2[20];
} MEM_OCCUPY;

typedef struct MEM_PACK {
  double total, used_rate;
} MEM_PACK;

MEM_PACK *get_memoccupy();

} // namespace utils

#endif /* UTILS_H_ */
