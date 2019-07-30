#include "memory_raw_vector.h"
#include <string>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <cassert>
#include <cmath>
#include <unistd.h>
#include <iostream>
#include "utils.h"
#include <string.h>
#include <cstdlib>
#include <ctime>
#include <thread>

using namespace std;
using namespace tig_gamma;

float *buildVector(int dim, float offset) {
  float *v = new float[dim];
  for (int i = 0; i < dim; i++) {
    v[i] = i + offset;
  }
  return v;
}

float *buildMultiVectors(int n, int dim, float offset) {
  float *v = new float[n * dim];
  for (int j = 0; j < n; j++) {
    for (int i = 0; i < dim; i++) {
      v[j * dim + i] = i + offset;
    }
  }
  return v;
}

bool floatArrayEquals(const float *a, int m, const float *b, int n) {
  if (m != n) return false;
  for (int i = 0; i < m; i++) {
    if (std::fabs(a[i] - b[i]) > 0.0001f) {
      return false;
    }
  }
  return true;
}

void TestNormal(string file_path = "test_memory_raw_vector.fet") {
  int max_doc_size = 100000;
  int dimension = 512;
  string head = "headlines";

  int fd = open(file_path.c_str(), O_WRONLY | O_CREAT | O_TRUNC, 00777);
  assert(-1 != fd);
  assert(head.length() == write(fd, head.c_str(), head.length()));
  close(fd);

  MemoryRawVector *raw_vector =
      new MemoryRawVector("test_memory_raw_vector", max_doc_size, dimension);
  raw_vector->SetFilePath(file_path);
  raw_vector->SetOffset(head.length());
  assert(0 == raw_vector->Init());

  // batch add
  int batch_num = 10000;
  int batch_offset = 1;
  float *batch_vector = buildMultiVectors(batch_num, dimension, batch_offset);
  int xids[batch_num];
  for (int i = 0; i < batch_num; i++) {xids[i] = i;}
  raw_vector->AddWithIds(batch_num, batch_vector, xids, -1);
  for (int i = 0; i < batch_num; i++) {xids[i] = i + batch_num;}
  raw_vector->AddWithIds(batch_num, batch_vector, xids, -1);

  // add one by one
  int ids[1];
  for (int i = batch_num * 2; i < max_doc_size; i++) {
    ids[0] = i;
    float *vector = buildVector(dimension, i);
    raw_vector->AddWithIds(1, vector, ids, -1);
  }

  assert(max_doc_size == raw_vector->GetVectorNum());
  const float *all_features = raw_vector->GetVectors(raw_vector->GetVectorNum());
  assert(nullptr != all_features);

  // check all features
  for (int i = 0; i < max_doc_size; i++) {
    int offset = batch_offset;
    if (i >= batch_num * 2) {
      offset = i;
    }
    float *expect = buildVector(dimension, offset);
    const float *feature = all_features + i * dimension;
    cerr << "i=" << i << ", feature[0]=" << feature[0] << ", expect[0]=" << expect[0] << endl;
    assert(floatArrayEquals(expect, dimension, feature, dimension));
    delete expect;
  }

  // random get feature
  std::srand(std::time(nullptr));
  int get_num = max_doc_size;
  for (int i = 0; i < get_num; i++) {
    int feature_id = std::rand() % max_doc_size;
    int offset = batch_offset;
    if (feature_id >= batch_num * 2) {
      offset = feature_id;
    }
    float *expect = buildVector(dimension, offset);
    const float *feature = raw_vector->GetVector(feature_id);
    cerr << "random get i=" << i << ", feature id=" << feature_id << ", feature[0]=" << feature[0] << ", expect[0]=" << expect[0] << endl;
    raw_vector->DestroyVector(feature);
    delete [] expect;
  }

  assert(nullptr == raw_vector->GetVector(-1));
  assert(nullptr == raw_vector->GetVector(max_doc_size));
  delete raw_vector;
}

int main(int argc, char **argv) {
  if (argc != 2) {
    return 0;
  }
  string file_path = string(argv[1]);
  TestNormal(file_path);
  return 0;
}
